# Design

## Context

Single genre merge exists: handler `MergeGenres` (`backend/internal/modules/admin/transport/http/handler.go` ~1794), service `backend/internal/modules/admin/service/merge_genre.go`, repo `GormRepository.MergeGenres` (`backend/internal/modules/admin/repository/genre_moderation.go:197`, one transaction: move `book_genres` with dedup, delete leftovers, re-point aliases, carry QID, turn source into rejected alias). `curated_collections.genre_id` (migration 000025, FK `ON DELETE SET NULL`, no index) is never touched by it, so an auto collection filtered by a merged genre ranks nothing on its next recompute (`curated/repository/auto.go:46` joins `book_genres` on the linked genre).

There is no Redis/cache of curated collections: the home cache (`home/service/service.go`) holds only the home selection (no collections), and public collection pages read the DB. The only "stale" state is the auto collection snapshot (`curated_collection_items`), handled below.

## Decisions

1. **One transactional core for both merges.** Extract the body of `MergeGenres` into an unexported helper `mergeGenreInTx(tx, sourceID, targetID uuid.UUID, targetQID *string, sourceQID *string) (booksReassigned int64, carried bool, err error)` operating on an open `*gorm.DB` tx. Add `curated_collections` re-pointing to it:
   `UPDATE curated_collections SET genre_id = ?, updated_at = ? WHERE genre_id = ?` (target, now, source) — capture affected collection ids + kind beforehand with `SELECT id, kind FROM curated_collections WHERE genre_id = ?`.
   - `MergeGenres(ctx, source, target)` = lock both rows, call helper once.
   - `MergeGenresBatch(ctx, sourceIDs, targetID)` = lock all rows (`SELECT id, wikidata_qid FROM genres WHERE id IN ? AND merged_into_id IS NULL ORDER BY id` + ` FOR UPDATE` on Postgres — sorted lock order avoids deadlocks), require `found == len(sources)+1` else `gorm.ErrRecordNotFound`, then call the helper for each source **in request order**, passing the current target QID (updated in memory after a carry). This makes "first source with a QID in request order wins" fall out naturally; later QID sources keep theirs on the alias.
   - Raw SQL on `curated_collections` inside the admin repository is the existing pattern (admin repo already writes `book_genres`, `books`, `genres`); the curated module is not imported by the admin repository.
2. **Repository result type.** Change the repo signature to return a struct (both methods):
   ```go
   type GenreMergeResult struct {
       BooksReassigned     int64
       SourcesMerged       int
       CollectionsSwitched int
       AutoCollectionIDs   []uuid.UUID // switched collections with kind = 'auto'
       CarriedQIDFrom      *uuid.UUID  // source whose QID moved to the target
       KeptQIDs            map[uuid.UUID]string // source id -> QID kept on its alias
   }
   ```
   `books_reassigned` = sum of per-step `RowsAffected`; with sequential steps this equals the distinct-books definition of the spec (a book in two sources moves in step 1 and is already in the target for step 2).
3. **Preview** is a read-only repo method `PreviewGenreMerge(ctx, sourceIDs, targetID) (GenreMergePreview, error)`:
   - existence check: `SELECT id FROM genres WHERE id IN ? AND merged_into_id IS NULL` must find all (else `gorm.ErrRecordNotFound`);
   - books: `SELECT COUNT(DISTINCT book_id) FROM book_genres WHERE genre_id IN (sources) AND book_id NOT IN (SELECT book_id FROM book_genres WHERE genre_id = target)`;
   - collections: `SELECT c.id, c.title, c.slug, c.kind, c.is_published, g.id, g.name FROM curated_collections c JOIN genres g ON g.id = c.genre_id WHERE c.genre_id IN (sources) ORDER BY c.title, c.id`.
   Portable SQL (SQLite tests run it).
4. **Service validation** (new file `backend/internal/modules/admin/service/merge_genre_batch.go`): shared `validateGenreMergeInput(sourceIDs, targetID)` used by batch and preview — empty → `ErrGenreMergeNoSources`, duplicate → `ErrGenreMergeDuplicateSource`, `> 100` → `ErrGenreMergeTooLarge` (reuse the 100 cap constant used by `ErrBulkStatusTooLarge`; a separate error so the message names merge), target in sources → `ErrCannotMergeGenreIntoSelf`. All map to 400 in the handler; `ErrGenreNotFound` → 404. Duplicates are rejected (not collapsed like bulk-status) because a merge is irreversible and a duplicate signals a client bug.
5. **Response genre.** Single merge keeps the in-memory QID mirroring. Batch re-reads the target with `GetGenreByID` after commit (simpler than mirroring N steps). `KeptQIDs` are logged with `s.log.Info` as the single merge does.
6. **Auto collection recompute after commit.** Spec rule from `collections/curated-collections`: changing an auto collection's genre recomputes it at once without baseline. Admin service gets an optional dependency, following the `WithAuthorFilmSyncer(screenWorksService)` pattern:
   ```go
   type CollectionRecomputer interface {
       RecomputeCollections(ctx context.Context, ids []uuid.UUID)
   }
   func WithCollectionRecomputer(r CollectionRecomputer) Option
   ```
   Curated service gets `RecomputeCollections(ctx, ids)` that calls the existing `recomputeLogged` (reset baseline, failures only logged) per id. Called by both merge service methods after a successful commit, only with `AutoCollectionIDs`; nil dependency → skipped. In `backend/cmd/api/main.go` construct `curatedService` before `adminService` (currently line 247 vs 127) and pass it. The worker's admin service needs no recomputer (it never merges).
7. **Legacy data fix.** Migration `000031_curated_collections_unalias_genres` (data only, forward-only): `UPDATE curated_collections c SET genre_id = g.merged_into_id, updated_at = NOW() FROM genres g WHERE c.genre_id = g.id AND g.merged_into_id IS NOT NULL;` — `down.sql` is a no-op comment (the old link is not restorable and was wrong). Switched auto collections there are picked up by the next scheduled recompute.

## API

All under the existing `genres := admin.Group("/genres", adminRole)` in `backend/internal/transport/http/router.go`, both with `requireBearer`, registered next to `/bulk-status` (static paths; no clash with `/:id/merge`):

- `POST /api/v1/admin/genres/merge-batch`
  - body `mergeGenreBatchRequest{ SourceIDs []string json:"source_ids"; TargetID string json:"target_id" }`
  - 200 `mergeGenreBatchResponse{ Genre genreResponse; SourcesMerged int json:"sources_merged"; BooksReassigned int64 json:"books_reassigned"; CollectionsSwitched int json:"collections_switched" }`
  - 400 `{"error": "invalid request" | "invalid genre id: \"x\"" | "invalid target_id" | "source_ids must not be empty" | "duplicate source id" | "too many source ids: at most 100 per request" | "cannot merge a genre into itself"}`; 404 `{"error":"genre not found"}`; 500 `{"error":"failed to merge genres"}`.
- `POST /api/v1/admin/genres/merge-preview` — same body and 400/404; 200 `mergeGenrePreviewResponse{ BooksReassigned int64 json:"books_reassigned"; Collections []genreMergeCollection json:"collections" }` with `genreMergeCollection{ID, Title, Slug, Kind, IsPublished json:"is_published", SourceGenre {ID, Name} json:"source_genre"}`; `collections` is `[]` not null. 500 `{"error":"failed to preview genre merge"}`.
- `POST /api/v1/admin/genres/{id}/merge` — response gains `collections_switched` (additive). Errors unchanged.

Swagger godoc in Russian on both new handlers (admin, Bearer, read-only note on preview); regenerate with `make swagger` equivalent (`swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`) — additive diff only.

Handler interface in `handler.go` (line ~44): `MergeGenres` return type changes to carry `collectionsSwitched`; add `MergeGenresBatch(ctx, sourceIDs []uuid.UUID, targetID uuid.UUID) (*GenreBatchMergeOutcome, error)` and `PreviewGenreMerge(ctx, sourceIDs []uuid.UUID, targetID uuid.UUID) (*repository.GenreMergePreview, error)`. Update the handler test fakes.

## Database

- No schema change. One data migration `000031` (Decision 7).
- Admin repo SQLite test schema (`genre_moderation_sqlite_test.go`) gains a `curated_collections` table (copy the column list used in `curated/repository/repository_sqlite_test.go`).

## Frontend

`packages/api-client/src/index.ts`:
- `AdminGenreMergeResult` gains `collections_switched: number`.
- New types `AdminGenreMergeBatchResult { genre; sources_merged; books_reassigned; collections_switched }`, `AdminGenreMergePreview { books_reassigned; collections: AdminGenreMergeCollection[] }`, `AdminGenreMergeCollection { id; title; slug; kind: 'manual' | 'auto'; is_published; source_genre: { id; name } }`.
- `adminGenresApi.mergeBatch(sourceIds, targetId)` and `adminGenresApi.mergePreview(sourceIds, targetId)` (POST, body `{source_ids, target_id}`), with the error-code comments like `merge`.

`apps/admin/src/features/merge-genre` (one slice, so internal sharing is allowed):
- `ui/merge-preview-summary.tsx` — internal component: `useQuery(['admin','genres','merge-preview', [...sourceIds].sort(), targetId], …)`, renders «Считаем последствия…», «Будет перенесено книг: N.» and one line per collection «Подборка «{title}» будет переключена на жанр «{target}».»; reports `ready` to the parent (e.g. `onReadyChange`) so the confirm button can be gated; errors rendered with the dialog's error mapper. `staleTime: 0`.
- `merge-genre-dialog.tsx` — renders the summary once a canonical is chosen; confirm disabled until ready; on success also invalidate `COLLECTIONS_QUERY_KEY` (import from `@/entities/curated-collection`, an entity — allowed) and pass `collections_switched` to `onMerged(booksReassigned, collectionsSwitched)`.
- `ui/batch-merge-genres-dialog.tsx` — props `{ genres: AdminGenre[] (selection in queue order); onClose; onMerged(result: AdminGenreMergeBatchResult, targetName) }`. Radio group (`name="merge-target"`) over `genres` plus «Другой жанр…»; the latter shows the search identical to the single dialog (candidates exclude every selected id). Sources = `genres` minus target, preserving order. Same explanation, rejected warning, summary, error mapping (400 → «Некорректный выбор жанров для слияния.»). Confirm text «Слить {sources.length} в «{target}»», pending «Слияние...». Invalidate `['admin','genres']`, `['admin','books']`, `DASHBOARD_QUERY_KEY`, `COLLECTIONS_QUERY_KEY`.
- export both from `index.ts`.

`apps/admin/src/views/genre-moderation/ui/genre-moderation-view.tsx`:
- `renderBulkActions={(selected, controls) => <Button …>Слить выбранные ({selected.length})</Button>}` (disabled while `statusPending`), opening the batch dialog with `selected.map(i => i.genre)`; overlay state becomes a union (`single | batch`).
- Success notifications per spec; `controls.clearSelection()` after either merge.

## Existing Patterns

- Transaction + row lock: `GormRepository.MergeGenres` (`genre_moderation.go:197`).
- Bulk id validation / 100 cap: `BulkSetGenreStatus` handler (`handler.go:1741`) and service (`service/genre_moderation.go:172`).
- Optional cross-module service dependency: `adminservice.WithAuthorFilmSyncer(screenWorksService)` in `cmd/api/main.go:132`.
- Bulk action slot: `renderBulkActions` in `views/author-moderation/ui/author-moderation-view.tsx:268`.
- Dialog + tests: `features/merge-genre/ui/merge-genre-dialog.tsx` and its `.test.tsx`.

## Risks

- **Lock scope**: a batch of up to 100 genres locks those rows plus moves many `book_genres` rows in one tx; acceptable at current data size (a queue page is ≤ 50 rows anyway).
- **Preview vs merge drift**: counts can change between preview and confirm; the merge result is authoritative and is what the success message shows.
- **Recompute after commit is not atomic** with the merge: a failure leaves the old snapshot (logged) until the scheduled recompute — accepted, same as genre edit.
- **Rejected target**: a published collection switched to a rejected genre shows no genre publicly (existing `genre: null` rule); the dialog's rejected-target warning already covers the intent.
- **main.go reorder**: moving `curatedService` construction earlier must not change its other consumers; it only depends on `db` and `logger`.
- `docs/features/genre-moderation-queue.md` is frozen history — do not edit it.
