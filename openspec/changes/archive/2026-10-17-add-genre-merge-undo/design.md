# Design

## Context

Genre merges (`POST /admin/genres/{id}/merge`, `POST /admin/genres/merge-batch`) run in one transaction in `backend/internal/modules/admin/repository/genre_moderation.go` (`mergeGenres` -> per source `mergeGenreInTx`), orchestrated by `service/merge_genre.go` and `service/merge_genre_batch.go`. Nothing is recorded, so a merge cannot be reverted. This change adds a merge journal written inside the merge transaction and an undo that replays the journal backwards, skipping whatever no longer matches.

Constraints:
- PR #482 (`feat/ai-genre-suggestions`, not in main) also edits `genre_moderation.go`, calls the service `MergeGenresBatch`, adds migration 000032 and moves `features/merge-genre/ui/merge-preview-summary.tsx` to `entities/genre`. Keep edits to those files small and put new code in new files. Do NOT touch `merge-preview-summary.tsx`.
- Relevant schema: `genres.slug` is UNIQUE over all rows (aliases included); `genres.wikidata_qid` has a partial UNIQUE index; `chk_genres_alias_rejected` (alias => status rejected); `genres.merged_into_id` FK ON DELETE RESTRICT; `book_genres` PK (book_id, genre_id), FKs ON DELETE CASCADE; `curated_collections.genre_id` FK ON DELETE SET NULL.

## Decisions

1. **Journal = three tables, no FKs to genres.** The journal is a snapshot; ids of genres/books/collections are plain UUIDs so deleting any of them never fails because of the journal, and undo checks existence itself. Users are FK'd ON DELETE SET NULL.
2. **Snapshot per source, taken right before that source's `mergeGenreInTx`** (inside the same transaction, after the locks). That makes batch semantics exact: a book shared by sources A and B is `book_moved` for A and `book_linked` for B, because by B's turn it is already in the target. The rows are collected in memory and written once after the loop (header row, then sources, then items via `CreateInBatches`, batch ~500).
3. **Actor via context for merges, explicit param for undo.** To keep the service signatures `MergeGenresBatch(ctx, sourceIDs, targetID)` (called by #482) unchanged, the handler wraps the request context with `service.WithActor(ctx, userID)`; the service reads `actorFromContext(ctx)` (nil when absent) and passes it to the repository. New `UndoGenreMerge(ctx, mergeID, actorID)` takes it explicitly. After #482 lands, its apply handler should also call `WithActor` (otherwise `performed_by` is `null` — the journal entry is still written).
4. **Repository merge methods gain `actorID *uuid.UUID`** (`MergeGenres(ctx, sourceID, targetID, actorID)`, `MergeGenresBatch(ctx, sourceIDs, targetID, actorID)`); `GenreMergeResult` gains `MergeID uuid.UUID` (generated in Go with `uuid.New()`, portable to SQLite). Service `MergeGenres` gains a 5th return value `mergeID uuid.UUID`; `GenreBatchMergeOutcome` gains `MergeID`.
5. **Undo check order** (first failure wins, 409 `code`): `already_undone` -> `target_missing` -> `target_merged` (target `merged_into_id IS NOT NULL`) -> `source_missing` -> `source_not_alias` (`merged_into_id <> target`, incl. NULL) -> `qid_conflict` (for a source with recorded `prev_qid`: the current holder of that QID, if any, is neither the source nor — when `qid_carried` — the target). No slug check: alias rows keep their slug and `genres.slug` is unique over all rows, so nobody can take it.
6. **"Later merge" protection.** When undoing a `book_moved` item, the target link is kept if any other entry with the same `target_id`, `undone_at IS NULL` and `created_at` later than this entry has a book item (`book_moved` or `book_linked`) for that book; the source link is restored either way. This keeps later merges that relied on the target link (their dedup) undoable and correct.
7. **No undo of pre-journal merges**: they simply have no entry, so the history never shows them.
8. **Status restore**: one `UPDATE genres SET merged_into_id = NULL, status = prev_status` (both columns in one statement satisfies `chk_genres_alias_rejected`).
9. **Recompute**: undo returns `AutoCollectionIDs` (collections of kind `auto` switched back); the service calls the existing `recomputeSwitchedCollections` after commit, like the merge.

## API

All under the existing `genres := admin.Group("/genres", adminRole)` (admin only; moderator 403), each with `requireBearer`. Register next to `merge-preview`; Echo's static segment `merges` wins over `/:id`.

| Method | Path | Body | Success |
|---|---|---|---|
| GET | `/api/v1/admin/genres/merges?page&limit` | — | 200 `{items: GenreMergeEntry[], total, page, limit}` |
| GET | `/api/v1/admin/genres/merges/{id}/undo-preview` | — | 200 `{merge: GenreMergeEntry, books_returned, books_relinked, collections: genreMergeCollection[]}` |
| POST | `/api/v1/admin/genres/merges/{id}/undo` | — | 200 `{genre, sources_restored, books_returned, books_relinked, collections_switched_back}` |

`GenreMergeEntry` = `{id, created_at, performed_by: {id, name}|null, target: {id, name}, sources: [{id, name}], books_reassigned, collections_switched, undone_at|null, undone_by: {id, name}|null}`. User `name` = `COALESCE(NULLIF(users.name, ''), users.email)`. `limit` default 20, max 100, `page` >= 1. Collections reuse the existing `genreMergeCollection` response type (`source_genre` = the source the collection returns to).

Errors: non-UUID id 400 `{"error":"invalid merge id"}`; unknown id 404 `{"error":"genre merge not found"}`; blocked 409 `{"error":"genre merge cannot be undone","code":"<code>"}` (`ApiError.code` on the client already reads body `code`); 500 otherwise. Preview returns the same 400/404/409.

Existing responses (additive): `mergeGenreResponse` and `mergeGenreBatchResponse` gain `merge_id` (string UUID).

Swagger: godoc blocks in Russian like the neighbours; the existing merge descriptions drop «Необратимо.» and mention the undo. Regenerate the swagger docs the same way previous admin endpoint changes did (additive only).

New handlers go in a new file `backend/internal/modules/admin/transport/http/handler_genre_merge_undo.go` (handler.go is shared with #482). Handler-side service interface gains `ListGenreMerges`, `PreviewGenreMergeUndo`, `UndoGenreMerge`; update the handler test fake.

## Database

Migration `backend/migrations/000033_genre_merge_journal.{up,down}.sql` (additive; down drops the three tables — never run it in tasks):

```sql
CREATE TABLE genre_merges (
    id UUID PRIMARY KEY,
    target_id UUID NOT NULL,
    target_name VARCHAR(255) NOT NULL,
    books_reassigned INTEGER NOT NULL,
    collections_switched INTEGER NOT NULL,
    performed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    undone_at TIMESTAMPTZ,
    undone_by UUID REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX idx_genre_merges_created_at ON genre_merges (created_at DESC, id);
CREATE INDEX idx_genre_merges_target_active ON genre_merges (target_id) WHERE undone_at IS NULL;

CREATE TABLE genre_merge_sources (
    merge_id UUID NOT NULL REFERENCES genre_merges(id) ON DELETE CASCADE,
    source_id UUID NOT NULL,
    position INTEGER NOT NULL,
    source_name VARCHAR(255) NOT NULL,
    prev_status VARCHAR(20) NOT NULL,
    prev_qid VARCHAR(32),
    qid_carried BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (merge_id, source_id)
);

CREATE TABLE genre_merge_items (
    merge_id UUID NOT NULL,
    source_id UUID NOT NULL,
    kind VARCHAR(20) NOT NULL CHECK (kind IN ('book_moved', 'book_linked', 'alias', 'collection')),
    ref_id UUID NOT NULL,
    PRIMARY KEY (merge_id, source_id, kind, ref_id),
    FOREIGN KEY (merge_id, source_id) REFERENCES genre_merge_sources (merge_id, source_id) ON DELETE CASCADE
);
CREATE INDEX idx_genre_merge_items_ref ON genre_merge_items (ref_id);
```

Mirror the three tables (and a minimal `users` table if `genreModerationDDL` lacks one) in the SQLite test DDL in `genre_moderation_sqlite_test.go` (or a new DDL constant in the new test file executed by the fixture).

### Merge write (inside `mergeGenres`, Postgres + SQLite portable SQL)
Before each `mergeGenreInTx(source)`: read source `name, status, wikidata_qid`; `book_moved` = `SELECT book_id FROM book_genres WHERE genre_id = src AND book_id NOT IN (SELECT book_id FROM book_genres WHERE genre_id = target)`; `book_linked` = same with `IN`; `alias` = `SELECT id FROM genres WHERE merged_into_id = src`. After it: `collection` items from the returned `switchedCollection`s, `qid_carried` from `carried`. Read `target_name` once after locking. After the loop insert the journal; `res.MergeID` = header id. Edits in `genre_moderation.go` limited to `mergeGenres` (+ the two public wrappers' signatures and the `GenreMergeResult` field); snapshot/journal helpers live in a new `genre_merge_journal.go`.

### Undo (new `genre_merge_undo.go`, one transaction)
1. Load entry (`FOR UPDATE` on Postgres), not found -> `gorm.ErrRecordNotFound`. Load sources by `position`.
2. Lock target + sources rows (`SELECT id, merged_into_id, wikidata_qid FROM genres WHERE id IN ? ORDER BY id FOR UPDATE`, no alias filter — a new helper, do not change `lockGenresForMerge`).
3. Run the shared check (Decision 5) -> `*GenreMergeUndoBlockedError{Code}`.
4. For each source in reverse `position`:
   - QID: if `prev_qid` set and `qid_carried` and target holds it -> `UPDATE genres SET wikidata_qid = NULL WHERE id = target AND wikidata_qid = prev_qid` first; then if source does not hold it -> `UPDATE genres SET wikidata_qid = prev_qid WHERE id = source`.
   - `UPDATE genres SET merged_into_id = NULL, status = prev_status WHERE id = source`.
   - Aliases: `UPDATE genres SET merged_into_id = source WHERE id IN (alias items) AND merged_into_id = target`.
   - Moved books: candidates = `book_moved` items with an existing `(book, target)` link (book existence implied by the link). Insert `(book, source)` where missing; delete `(book, target)` for candidates not protected by Decision 6. `books_returned += len(candidates)`.
   - Linked books: insert `(book, source)` for `book_linked` items whose book exists and link is missing (`INSERT ... SELECT ... JOIN books ... WHERE NOT EXISTS`). `books_relinked += inserted`.
   - Collections: `SELECT id, kind FROM curated_collections WHERE id IN (collection items) AND genre_id = target`; update those to `genre_id = source, updated_at = now`.
5. `UPDATE genre_merges SET undone_at = now, undone_by = actor WHERE id = ?`.
Preview runs steps 1-3 without locks plus `COUNT` versions of step 4 and the collection list (joined to `genres` for `source_genre.name`, ordered by `title, id`).

### History
`genre_merges LEFT JOIN users pb ON pb.id = performed_by LEFT JOIN users ub ON ub.id = undone_by ORDER BY created_at DESC, id DESC LIMIT/OFFSET`, then one query for `genre_merge_sources WHERE merge_id IN ? ORDER BY merge_id, position`.

## Frontend

- `packages/api-client`: types `AdminGenreMergeEntry`, `AdminGenreMergeUndoPreview`, `AdminGenreMergeUndoResult`, `AdminGenreMergeUndoCode` (union of the six codes); `merge_id: string` on the existing merge/batch-merge result types; methods `api.admin.genres.merges({page, limit})`, `undoMergePreview(id)`, `undoMerge(id)` (append after `mergePreview`).
- `entities/genre/api/index.ts`: expose `merges`, `undoMergePreview`, `undoMerge` on `adminGenresApi` (append-only lines).
- New feature `features/undo-genre-merge`: `UndoGenreMergeDialog({ mergeId, onClose, onUndone(result) })` — preview via `useQuery` (`['admin','genres','merges', id, 'undo-preview']`, `staleTime: 0`), the preview summary inside the feature (its own component, not the merge one), `undoErrorMessage(err)` mapping `ApiError.status`/`code` to the spec texts; on success invalidate `['admin','genres']`, `['admin','books']`, `DASHBOARD_QUERY_KEY`, `COLLECTIONS_QUERY_KEY`. Dialog markup follows `merge-genre-dialog.tsx` (fixed overlay, `role="dialog"`).
- New view `views/genre-merge-history`: `GenreMergeHistoryView` — table (`@bookhaven/ui` table primitives as in `widgets/moderation-queue`), `page` and `undo` URL params (`useSearchParams` + `router.replace`, as `use-queue-url-state.ts` does), pagination like the queue, success/error banner, date formatting like `created-at-column.tsx`. Opens `UndoGenreMergeDialog` for `?undo=<id>` or a row button; closing clears `undo`.
- New route `apps/admin/src/app/moderation/genres/merges/page.tsx` rendering the view (wrap in `Suspense` if the view uses `useSearchParams`, as other pages using URL state do).
- `views/genre-moderation`: `renderToolbar` renders a `Link` «История слияний» to `/moderation/genres/merges`; both `onMerged` handlers pass `action: { label: 'Отменить', href: \`/moderation/genres/merges?undo=${mergeId}\` }` to `notify`.
- `widgets/moderation-queue`: `ModerationFeedback` gains optional `action?: { label: string; href: Route }` rendered as a `next/link` after the message (additive; other queues unaffected).
- `features/merge-genre`: `MergeGenreDialog.onMerged` gets a 3rd arg `mergeId`; in both dialogs replace «Действие необратимо.» with «Слияние можно будет отменить в «Истории слияний».». No other edits; do not touch `merge-preview-summary.tsx`.
- FSD: the view composes features; no feature imports another feature; no view imports a view.

## Existing Patterns

- Transaction + lock style: `mergeGenres` / `lockGenresForMerge` (`FOR UPDATE` only on Postgres).
- Preview endpoint + dialog gating: `PreviewGenreMerge` and `merge-preview-summary.tsx`.
- Post-commit recompute: `recomputeSwitchedCollections` (`WithCollectionRecomputer`).
- Error-code bodies read by `ApiError.code`.
- Actor from `c.Get("user_id").(string)` (e.g. users handler); explicit actor param: admin role change.
- SQLite repository fixtures: `genre_moderation_sqlite_test.go` (`newGenreModRepo`, `genre`, `alias`, `book`, `links`, `load`) and `collection`/`collectionGenre` in `genre_merge_batch_sqlite_test.go`.
- Moderator-403 handler/router test for `merge-batch`.

## Risks

- **Migration number gap.** golang-migrate `up` only applies versions above the DB's current version; it does not detect a missing lower one. If 000033 is applied to a database before #482's 000032 exists there, 000032 would be silently skipped on that DB forever. Fresh databases (CI) apply 31 -> 32 -> 33 in order and are fine; there is no prod yet, only local compose DBs. Mitigation: merge #482 first if possible; if this change lands first, renumber #482's migration to 000034 when rebasing it (checklist item for the merge of #482). Never use `migrate down`/`force` to fix it.
- **Conflicts with #482**: small edits in `mergeGenres` and the repo method signatures (new `actorID` param). If #482 calls the repository merge directly, it needs the extra argument; its service call is unaffected. Its apply handler should add `WithActor` for `performed_by`.
- Journal size grows with genre size (one item per moved book); acceptable for genres, batched inserts keep parameter counts below SQLite/Postgres limits.
- "Later merge" detection relies on `created_at` ordering; two merges into the same target in the same microsecond are not realistic.
- Undoing restores a previously published genre to the public site; public genre pages may be cached until their usual revalidation, as with any status change.
- A merge's undo cannot restore links/collections deleted or changed after the merge — by design (skip, not fail).
- typedRoutes: the `?undo=` href must type-check as `Route`; `tsc` does not catch typedRoutes errors — CI frontend build is the gate.
