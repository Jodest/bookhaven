# Genre Moderation Queue with Merge

Status: done

Issue: #302. Epic #290 "Genre moderation and Wikidata import", feature 2 of 3 (phase 7 of [admin-navigation-restructure.md](admin-navigation-restructure.md)). Builds on [genre-status.md](genre-status.md) (#301: `genres.status`, `wikidata_qid`, harvest resolver, public published-only reads). Modeled on [author-moderation-queue.md](author-moderation-queue.md) and [duplicate-author-merge.md](duplicate-author-merge.md). The merge mechanism **deliberately differs from authors**: the merged genre becomes an alias row instead of being deleted (Decision below).

## Goal

Give admins one page, `/moderation/genres`, to process `pending` genres from harvest and file import. Each row shows the context needed to decide: QID, linked books and likely duplicates. From there the admin can publish, reject or merge a genre into another one. Publish and reject work per row or for selected rows. A merged genre must keep mapping future harvests and file imports to its target, both by QID and by slug.

## Out of scope

- #303 Wikidata genre import (`/import/genres`).
- Un-merge or alias management UI. A merge cannot be undone from the UI. Recovery is manual SQL by the human.
- Automatic merging or rejecting. Hints are display-only.
- pg_trgm or any other fuzzy-matching infrastructure.
- Deleting genres. There is no admin genre delete today, and none is added.
- Per-harvest-job filtering (same reasoning as for authors).
- Widening anything to moderators. Everything here is admin-only.

## Decision: merge = alias row (`merged_into_id`), not delete

There were two options:

**(a) Transfer the QID and delete the source (the author model).** This loses data in two cases:
- **Slug:** after the delete, the source slug ("романы") matches nothing. The next file import or QID-less harvest of that label recreates a `pending` duplicate. Slug matching is the only path for file import and for QID-less records, so this happens regularly.
- **QID:** a genre can hold only one QID (unique index). When both genres have a QID, the source QID is dropped, and the next harvest of it recreates the duplicate. Genre-status rule 5 made "same ru label, different QID" link to one genre, so two QIDs for one concept is the expected case for genres, not an edge case.

**(b) Keep the source as an alias row (chosen).** The source row keeps its name and slug, gets `merged_into_id = target`, and keeps its QID (moved to the target only when the target has none). Resolvers that find an alias by QID or slug follow it to the target. This is the only option that keeps **both** the QID and the slug of every merged genre resolving to the target, including after chained merges.

The alias status is forced to `rejected`, and a DB CHECK enforces it. This gives two guarantees:
1. **Public safety comes from the DB.** Every public read already filters `status = 'published'` (the 8 paths in genre-status.md), so alias rows can never surface publicly. No public code changes are needed.
2. **A resolver path that forgets to follow the alias fails safe.** It hits the existing rejected-tombstone skip: no link and no duplicate genre. It never creates junk.

Admin reads hide aliases explicitly with `merged_into_id IS NULL`, so an alias never appears in the rejected tab, the catalog list or a picker.

## User Stories

- As an admin, I open Модерация → Жанры and see pending genres, newest first. Each row shows its QID (as a link), its linked books (total and visible, plus a few titles linking to the book edit page) and "possibly the same genre" hints.
- As an admin, I publish or reject a genre, or select rows and publish or reject them in bulk. In the rejected tab I can return a genre to moderation or publish it.
- As an admin, I merge "романы" into "роман" in two clicks from a hint. Its books move to "роман", and the next harvest or file import of "романы", or of its QID, links "роман".
- As an admin, the sidebar badge Модерация and the tab Жанры show how many genres await moderation, and the Overview tile links to the queue.

## Business Rules

1. **Queue membership is by `status` only**, and alias rows are always excluded. The default view is `status=pending`; `published` and `rejected` are the other tabs. Ordering: `created_at DESC, name ASC, id`.
2. **Status actions:** Publish sets `published`, Reject sets `rejected` (tombstone), Return to moderation sets `pending`. Any transition is allowed, and a no-op counts as success. Reject keeps the `book_genres` links: a rejected genre is already invisible publicly (genre-status rule 7), and un-reject restores everything.
3. **Bulk status** works on an explicit list of IDs, max 100 per request. Duplicate IDs are removed. Each ID is processed on its own: an unknown ID or an alias ID gives the row error `genre not found`, and the other IDs still proceed. Only `status` changes.
4. **Merge `source → target`** (`POST /admin/genres/:id/merge`, `:id` = source). The whole merge runs in **one transaction**:
   1. Move the links: `UPDATE book_genres SET genre_id = target WHERE genre_id = source AND book_id NOT IN (SELECT book_id FROM book_genres WHERE genre_id = target)`. The affected row count is `books_reassigned`.
   2. Dedupe: `DELETE FROM book_genres WHERE genre_id = source`. These are books that already had the target.
   3. Re-point existing aliases: `UPDATE genres SET merged_into_id = target WHERE merged_into_id = source`. Alias chains therefore always stay one hop deep.
   4. QID: if the source has a QID and the target has none, clear it on the source, then set it on the target. The order matters because of the unique index. Otherwise the source keeps its own QID, and the alias resolves it.
   5. Turn the source into an alias: `UPDATE genres SET merged_into_id = target, status = 'rejected' WHERE id = source`. Name, slug and any remaining QID stay.
   - The target's name, slug and status are never changed. Its QID changes only through step 4.
   - Errors, with nothing changed: `source == target` gives 400 (`ErrCannotMergeGenreIntoSelf`). A missing source or target, or one that is already an alias, gives 404 (`ErrGenreNotFound`).
   - Any status is allowed for the source and the target, including a `rejected` target. In that case the UI warns that the books end up in a hidden genre and that future harvests of the source skip it.
5. **Alias resolution.** The alias is followed at most one hop. If the target is unexpectedly an alias too, the resolver returns an error and logs both IDs, so the invariant breach is visible.
   - **Harvest** (`resolveHarvestGenre`): the QID lookup and the slug lookup each still return alias rows. When the match is an alias, load its target with `GetGenreByID` and apply the normal outcome to the target: a rejected target gives `GenreOutcomeRejected` (skip), anything else gives `GenreOutcomeExisting` (link the target).
     - The alias check happens **before** the rejected check.
     - No QID backfill happens on an alias-followed match: neither the alias nor the target is written.
     - `GenreResolution` gains `ViaAlias *catalogDomain.Genre`, the matched alias, so #303 can report "merged into X".
   - **File import** (`ResolveOrCreateGenreIDsByNames`): a slug match on an alias links the target, whatever the target's status (genre-status rule 6).
   - Dedupe by resolved ID still applies. For example, the refs "роман" and "романы" both resolve to one link.
6. **Alias rows are reserved but invisible in admin.**
   - `GetGenreBySlug`, `GetGenreByQID` and `GenreSlugExists` stay unfiltered. As a result, an alias slug or QID cannot be reused: creating or renaming to it gives 409 `ErrDuplicateSlug`, and setting it as a QID gives 409 `ErrDuplicateGenreQID`.
   - These reads filter `merged_into_id IS NULL`:
     - `ListGenres`: `/catalog/genres`, the book-form picker and the merge dialog search
     - the moderation list and the hint candidates
     - `GetGenreByID`: PATCH, merge and the resolver target load
     - `SetGenreStatus`
     - `GenresExist`: saving a book with an alias ID gives 400, as for an unknown genre
     - `DashboardCounts.Genres`
   - `PendingGenres` needs no change, because aliases are `rejected`.
7. **Similar-name hints** are suggestions only, computed per page and never persisted.
   - `normalizeGenreName`: lowercase, `ё→е`, every char that is not a letter or digit becomes a space, spaces collapse, trim.
   - `genreStem(token)`: strip the longest matching suffix from this fixed list, but only if at least 4 runes remain; otherwise keep the token: `ами ями ого его ому ему ыми ими ая яя ое ее ые ие ый ий ой ов ев ей ам ям ах ях ы и а я е о у ю`.
   - Match kinds, highest first:
     - `exact_name`: equal normalized names
     - `same_stem`: equal sequences of token stems ("роман"/"романы", "научная фантастика"/"научной фантастики")
     - `shared_word`: the two names share at least one token stem of 5 or more runes ("фантастика"/"научная фантастика")
   - Candidates are all genres with `status <> 'rejected' AND merged_into_id IS NULL`, plus the page's own rows. The row itself is excluded. Genres are a small vocabulary, so **one query loads all candidates** (id, name, slug, status, wikidata_qid) and Go does the matching. No SQL prefilter is needed.
   - At most 5 hints per row, ordered by kind, then published first, then name, then id. The result is always `[]`, never null.
8. **Book context:**
   - `books_total`: all linked books
   - `books_visible`: linked books with `is_hidden = false`
   - `sample_books`: up to 3 books `{id, title, is_hidden}`, ordered by title then id, from one query for the whole page
9. **Filters:**
   - `status`: pending, published or rejected; invalid gives 400
   - `qid`: `with`, `without` or empty; invalid gives 400
   - `search`: `name ILIKE %s%`, same as `ListGenres`
   - `page`, `limit`: default 20, clamped to 50
10. **Authorization:** everything lives in the existing `genres := admin.Group("/genres", adminRole)` group, and the POST routes add `requireBearer`. Moderators get 403, anonymous users get 401. The frontend nav counters stay admin-only: `useNavCounters` sends no dashboard request for moderators (already the case).
11. **Effect on public reads:** status changes and merges take effect immediately, except for cached home selections and precomputed similarities (genre-status.md risks).

## Acceptance Criteria

DB
- [x] Migration `000018_genres_merged_into` adds `merged_into_id` (self FK, `ON DELETE RESTRICT`), a not-self CHECK, an "alias ⇒ rejected" CHECK and a partial index. It is created but **not applied** by agents.

Backend
- [x] `GET /api/v1/admin/genres/moderation` returns `{items, total, page, limit}`, with items `{genre, created_at, books_total, books_visible, sample_books, similar}`. It applies the defaults, filters, 400s and ordering from rules 1, 7, 8 and 9, and never lists alias rows.
- [x] Sample books and hint candidates take one query each per page (no N+1).
- [x] Hints follow rule 7: `exact_name`, `same_stem`, `shared_word`, self and rejected excluded, aliases excluded, at most 5, ordering, `[]` when there are none.
- [x] `POST /api/v1/admin/genres/bulk-status` with `{"ids": [...], "status": "..."}` returns 200 `{processed, updated, errors: [{genre_id, name, message}]}`. It gives 400 for an invalid status, empty ids, more than 100 ids, malformed ids or a bad body. An unknown or alias ID becomes a row error. `book_genres`, name, slug and QID are untouched.
- [x] `POST /api/v1/admin/genres/:id/merge` with `{"canonical_id": "uuid"}` returns 200 `{genre: <target genreResponse>, books_reassigned}`, with 400 and 404 as in rule 4.
- [x] Merge cases:
  - books linked to both genres end up with exactly one link, to the target
  - the source becomes `rejected` with `merged_into_id = target` and keeps its name and slug
  - existing aliases of the source are re-pointed to the target
  - QID moves to the target when the target has none, and stays on the source when both have one
  - the whole merge is atomic: a forced failure leaves nothing changed
- [x] Harvest resolution follows aliases:
  - an alias found by QID links the target
  - an alias found by slug, with or without a ref QID, links the target, and nothing is backfilled
  - an alias whose target is rejected is skipped
  - the alias is checked before the rejected check
  - no new genre is created in any alias case
- [x] File import: a slug match on an alias links the target.
- [x] Admin `GET /admin/genres`, `PATCH /admin/genres/:id` (404 for an alias), the dashboard `genres` count and book save with an alias genre ID (400) all exclude aliases. Creating a genre with an alias slug gives 409, and setting an alias QID gives 409.
- [x] Public: an alias row is never returned by `GET /genres` or embedded in a book, and it is guaranteed `rejected` by the CHECK.
- [x] 401 and 403 on all three new endpoints. A POST without a bearer token gives 401.

Frontend (`apps/admin`)
- [x] `/moderation/genres` renders `GenreModerationView` and replaces the placeholder.
- [x] Status tabs (На модерации, the default and not written to the URL / Опубликованные / Отклонённые), a QID select (Все / С QID / Без QID) and search. All of them are URL params (`status`, `qid`, `search`, `page`).
- [x] Each row shows:
  - a checkbox
  - the name with a `GenreStatusBadge`, and the slug
  - the QID as a link to `https://www.wikidata.org/wiki/<QID>` (`target="_blank" rel="noopener noreferrer"`), or "—"
  - the "Добавлен" date
  - books as "N (видимых M)" plus up to 3 titles linking to `/catalog/books/<id>/edit`, with hidden books marked
  - hints: name, match label, status badge, QID, a "разные QID" warning when both genres have different QIDs, and an "Это он — слить" button
- [x] Per-row actions depend on the status:
  - pending: Опубликовать / Отклонить / Слить
  - published: Отклонить / Слить
  - rejected: Вернуть на модерацию / Опубликовать / Слить
- [x] Every status action uses `confirm()`. The reject text names the genre and, when `books_visible > 0`, says it disappears from N published books. The actions call `bulkStatus({ids, status})`.
- [x] A bulk toolbar appears when at least 1 row is selected, with buttons that depend on the tab (same as authors). It confirms first, then shows "Обновлено X из N" plus the first errors. The selection clears on success and on page or filter change. A header checkbox selects the whole page.
- [x] "Слить" opens `MergeGenreDialog` with the row as `source`. "Это он — слить" opens it with `initialCanonical` set to the hint. The dialog text explains the alias behavior and warns when the target is rejected. On success, the feedback shows the reassigned count.
- [x] Nav counters: `pendingModerationTotal` = `pending_import_review + pending_authors + pending_genres`, and `tabs['moderation.genres'] = pending_genres`. Moderators send no admin requests.
- [x] The Overview tile "Жанры на модерации" links to `/moderation/genres`.
- [x] `/catalog/genres` gets a header link "Очередь модерации" pointing to `/moderation/genres`.
- [x] Invalidations: after a status change, `['admin','genres']` (prefix) and the dashboard key; after a merge, also `['admin','books']`.

## UX

- A table and a feedback banner in the style of `author-moderation-view.tsx`: green or destructive banner, no toasts. The only dialog is the new merge dialog.
- Empty states: "Нет жанров на модерации" on the pending tab, "Жанры не найдены" otherwise.
- Merge dialog copy (Russian):
  - title: "Слить жанр «X» в …"
  - body: "Книги жанра «X» будут перенесены в «Y». «X» исчезнет из списков, но его название и QID продолжат указывать на «Y» при будущих импортах. Действие необратимо."
  - confirm button: "Слить «X» в «Y»"
  - when the target is rejected: "Целевой жанр отклонён — книги окажутся в скрытом жанре."
- A 409 on create or QID in `/catalog/genres` keeps its existing message. Extend the QID message with "(возможно, у слитого жанра)".

## API

`GET /api/v1/admin/genres/moderation?status=&qid=&search=&page=&limit=` (admin)
```json
{
  "items": [{
    "genre": { "id": "uuid", "name": "романы", "slug": "romany", "status": "pending", "wikidata_qid": "Q123" },
    "created_at": "2026-10-08T10:00:00Z",
    "books_total": 3,
    "books_visible": 1,
    "sample_books": [{ "id": "uuid", "title": "…", "is_hidden": true }],
    "similar": [{ "genre": { "...genreResponse": "" }, "match": "exact_name|same_stem|shared_word" }]
  }],
  "total": 12, "page": 1, "limit": 20
}
```
`POST /api/v1/admin/genres/bulk-status` (admin + bearer). Body `{"ids": ["uuid"], "status": "published"}`. Returns 200 `{"processed": 2, "updated": 1, "errors": [{"genre_id": "uuid", "name": "", "message": "genre not found"}]}`.

`POST /api/v1/admin/genres/:id/merge` (admin + bearer). Body `{"canonical_id": "uuid"}`. Returns 200 `{"genre": {...target}, "books_reassigned": 2}`. Errors: 400 for a bad id, bad body or self-merge; 404 for a missing genre or an alias.

Register `GET /moderation` and `POST /bulk-status` next to `genres.GET("")` in `internal/transport/http/router.go`, and `POST /:id/merge` with the other `/:id` routes. `genreResponse` is unchanged: `merged_into_id` is never exposed, because aliases are never returned. Swagger annotations on all three handlers, then regenerate (`swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`).

## Database

`backend/migrations/000018_genres_merged_into.up.sql`:
```sql
ALTER TABLE genres ADD COLUMN merged_into_id UUID REFERENCES genres(id) ON DELETE RESTRICT;
ALTER TABLE genres ADD CONSTRAINT chk_genres_merged_into_not_self CHECK (merged_into_id IS NULL OR merged_into_id <> id);
ALTER TABLE genres ADD CONSTRAINT chk_genres_alias_rejected CHECK (merged_into_id IS NULL OR status = 'rejected');
CREATE INDEX idx_genres_merged_into_id ON genres(merged_into_id) WHERE merged_into_id IS NOT NULL;
```
`.down.sql` drops the index, both constraints and the column, in reverse order.

- `ON DELETE RESTRICT` is chosen because there is no genre delete. If a delete feature is added later, it must decide what happens to aliases explicitly; silently cascading them away would lose the tombstones.
- `genres` has no `updated_at` column, so do not write one.
- **Agents never run `migrate down`**, drop or truncate tables, reset volumes or run any other destructive DB command. Leave applying (`migrate up`) to the human.

Domain changes in `catalog/domain/genre.go`:
- `MergedIntoID *uuid.UUID` with `gorm:"type:uuid;column:merged_into_id"`
- `func (g Genre) IsAlias() bool`

`createGenre` leaves `MergedIntoID` nil. Update the SQLite test DDLs that create `genres` (admin repository tests, and `catalogStatusDDL` in the catalog tests) to add `merged_into_id`.

## Backend

`admin/repository`, new file `genre_moderation.go`:
- `GenreModerationFilter{Status, QID, Search string; Limit, Offset int}`
- `ListGenresForModeration(ctx, f) ([]GenreModerationRow, int64, error)`: `genres g LEFT JOIN (SELECT bg.genre_id, COUNT(*) total, SUM(CASE WHEN b.is_hidden THEN 0 ELSE 1 END) visible FROM book_genres bg JOIN books b ON b.id = bg.book_id GROUP BY bg.genre_id) s`, filtered by `g.merged_into_id IS NULL`. Scan into a flat struct, not the GORM model. For SQLite portability, use `CASE` and avoid `FILTER`.
- `ListGenreSampleBooks(ctx, ids) (map[uuid.UUID][]SampleBook, error)`: a `ROW_NUMBER()` query with `rn <= 3`, like authors. Reuse the `SampleBook` type.
- `ListGenreHintCandidates(ctx) ([]catalogDomain.Genre, error)`: `status <> 'rejected' AND merged_into_id IS NULL`.
- `SetGenreStatus(ctx, id, status) (found bool, err error)`: `Updates(map{status})` with `WHERE id = ? AND merged_into_id IS NULL`.
- `MergeGenres(ctx, sourceID, targetID) (int64, error)`: runs rule 4 in one transaction. It re-reads both rows inside the transaction and returns `gorm.ErrRecordNotFound` if either is missing or already an alias.

`admin/repository/repository.go`:
- add `merged_into_id IS NULL` to `ListGenres`, `GetGenreByID`, `GenresExist` and the `DashboardCounts` genres count
- keep `GetGenreBySlug`, `GetGenreByQID` and `GenreSlugExists` unfiltered (rule 6)
- add the new methods to the `Repository` interface
- update every fake: `admin/transport/http/handler_test.go`, `admin/service/service_test.go` and the other admin service test fakes, and `harvest/service/handler_test.go`

`admin/service`:
- `genre_moderation.go`:
  - `ListGenreModerationQueue`: validates filters with `ErrInvalidModerationFilter`, clamps the limit, runs the list, samples and candidates, and builds the hints
  - `BulkSetGenreStatus`: `ErrInvalidGenreStatus`, dedupe, a cap of 100 reusing `ErrBulkStatusTooLarge`, row message `genre not found`
  - pure helpers `normalizeGenreName`, `genreStem`, `genreMatchKind`
- `merge_genre.go`: `MergeGenres(ctx, sourceID, targetID) (*catalogDomain.Genre, int64, error)`. It returns `ErrCannotMergeGenreIntoSelf` or `ErrGenreNotFound` (reuse it if it exists, otherwise add it), and returns the target with any carried QID applied in memory, like `MergeAuthors`. Log when the source keeps its QID because the target has its own.
- `harvest_resolve.go`:
  - add `followGenreAlias(ctx, g) (target, alias *Genre, err)`
  - call it in `resolveHarvestGenre`, on both the QID path and the slug path, before the status checks and before any backfill
  - call it in `ResolveOrCreateGenreIDsByNames`
  - add `GenreResolution.ViaAlias`
  - update the doc comments with rule 5

`admin/transport/http/handler.go`: the handlers `ListGenreModerationQueue`, `BulkSetGenreStatus` and `MergeGenres`, plus the response types `genreQueueItemResponse` (reusing `toGenreResponse`) and `mergeGenreResponse`. The handler validates the status, ids and UUIDs before calling the service, like `BulkSetAuthorStatus`.

## Frontend

`packages/api-client/src/index.ts`:
- types:
  - `AdminGenreQueueItem`
  - `AdminGenreHint { genre: AdminGenre; match: 'exact_name' | 'same_stem' | 'shared_word' }`
  - `AdminGenreSampleBook` (or reuse the author sample book type)
  - `AdminGenreQueueParams { status?, qid?: 'with' | 'without', search?, page?, limit? }`
  - `AdminGenreBulkStatusReport`
  - `AdminGenreMergeResult { genre: AdminGenre; books_reassigned: number }`
- methods on `adminGenresApi`: `moderationQueue(params)`, `bulkStatus(ids, status)` and `merge(sourceId, canonicalId)`

`apps/admin` (FSD):
- `app/moderation/genres/page.tsx`: a thin page rendering `GenreModerationView`. Remove the `PlaceholderPage` usage. `PlaceholderPage` stays for `/import/genres`.
- `views/genre-moderation/` (`ui/genre-moderation-view.tsx`, `index.ts`): filters, table, selection, bulk toolbar, feedback and merge-dialog state. Query key `['admin','genres','moderation', params]`. Mirror `views/author-moderation`.
- `features/moderate-genre/`:
  - `useGenreStatusMutation`: wraps `bulkStatus` and invalidates `['admin','genres']` plus the dashboard key
  - the confirm and summary text builders
- `features/merge-genre/ui/merge-genre-dialog.tsx`: modeled on `MergeAuthorDialog`.
  - Props: `source`, `initialCanonical?`, `onClose`, `onMerged`.
  - Candidate search uses `adminGenresApi.list` with key `['admin','genres','merge-candidates', q]` and excludes the source.
  - On success it invalidates `['admin','genres']`, `['admin','books']` and the dashboard key.
  - Show the candidate status badge.
- `entities/genre/lib/moderation-href.ts`: `genreModerationHref({name, status})`, which returns `/moderation/genres?status=…&search=…`. Export it from `entities/genre`. #303 will use it; no caller is required now.
- `entities/dashboard/lib/pending-moderation.ts`: add `pending_genres`, and update the doc comment.
- `features/nav-counters/lib/build-nav-counters.ts`: add `tabs['moderation.genres'] = stats.pending_genres`.
- `views/dashboard/ui/dashboard-view.tsx`: point the tile href to `/moderation/genres`.
- `views/genres/ui/genres-view.tsx`: add the header link "Очередь модерации".
- `shared/config/navigation.ts`: no change, the tab already exists. Update the header comment only if it mentions placeholders.
- typedRoutes: follow the existing `as any` href convention. The CI frontend build is the gate. Do not run a native `next build`.

## Existing Patterns

- Backend:
  - `admin/service/author_moderation.go` and `admin/repository/author_moderation.go`: queue, samples, bulk status, report
  - `admin/service/merge_author.go` and `GormRepository.MergeAuthors`: transaction structure and dedupe SQL
  - `admin/service/harvest_resolve.go` `resolveHarvestGenre`: where the alias hook goes
- Frontend:
  - `views/author-moderation`, `features/moderate-author`, `features/merge-author`
  - `entities/author/lib/moderation-href.ts`
  - `features/nav-counters`, `entities/dashboard/lib/pending-moderation.ts`
- Tests:
  - `admin/repository/author_moderation_sqlite_test.go`, `merge_qid_sqlite_test.go`, `genre_status_sqlite_test.go`: real-SQL SQLite templates
  - `handler_author_moderation_test.go`: auth chain 401/403/bearer

## Tests

Backend. Run via `docker compose run`, changed packages only. Run gofmt only on the changed files, never `-w` on the whole tree. Never run `migrate down` or reset the DB.
- `admin/service`, pure helpers, table-driven: `normalizeGenreName`, `genreStem` (suffix list, the 4-rune floor), match kinds, ordering, the cap of 5, self, rejected and alias excluded, `[]` when empty.
- `admin/service` queue and bulk status, with fakes: filter 400 sentinels, limit clamp, unknown ID as a row error, dedupe, more than 100, invalid status.
- `admin/service` `MergeGenres`: self merge, not found, alias, the returned target with the carried QID.
- `admin/service` resolvers:
  - harvest: QID match on an alias links the target; slug match on an alias links the target with no backfill (with and without a ref QID); an alias with a rejected target is skipped; refs for both the alias and the target produce one ID; a broken chain returns an error
  - file import: a slug match on an alias links the target
- `admin/repository` SQLite (new `genre_moderation_sqlite_test.go`):
  - list filters (status, qid with and without), aliases excluded, `books_total` and `books_visible` (0 books, only hidden, mixed), at most 3 samples
  - `SetGenreStatus`: changes only the status, keeps the links, an alias counts as not found
  - `MergeGenres`:
    - links are moved, and a book linked to both genres ends up with one link
    - the source becomes a rejected alias with its slug kept
    - existing aliases are re-pointed
    - QID transfer when the target has none, and no transfer when both have one
    - atomicity, for example by making the target missing inside the transaction, or by a failing step on a closed transaction
  - `GetGenreBySlug` and `GetGenreByQID` return alias rows
  - `ListGenres`, `GetGenreByID`, `GenresExist` and `DashboardCounts.Genres` exclude aliases
- `catalog/repository` SQLite: an alias row (`rejected` with `merged_into_id`) is absent from public `ListGenres`.
- `admin/transport/http`: the three handlers (response shapes, 400s, 404, 401/403, POST without bearer), and PATCH on an alias returns 404.
- `harvest/service`: one `processRecord` case where a genre ref hits an alias and the book links the target (fake level).

Frontend (admin vitest):
- `genre-moderation-view.test.tsx`:
  - rows show the QID link, the book summary with `/catalog/books/<id>/edit` links, and the hints with the "разные QID" warning
  - per-row actions per status
  - confirm-gated `bulkStatus` calls; the reject text mentions visible books
  - bulk toolbar and "Обновлено X из N"
  - selection resets on filter change
  - URL params
  - the hint button opens the merge dialog prefilled
- `merge-genre-dialog.test.tsx`: `initialCanonical` preselects, the source is excluded from candidates, the rejected-target warning, 400 and 404 messages, the invalidated keys.
- `build-nav-counters.test.ts` and `pending-moderation.test.ts`: `pending_genres` counts in the total and in `tabs['moderation.genres']`. Flip the #301 fixtures that pinned "not counted".
- `use-nav-counters.test.tsx`: a moderator still sends no requests. `nav-badge` renders on the Жанры tab.
- `dashboard-view.test.tsx`: the tile href is `/moderation/genres`. `genres-view.test.tsx`: the header link.
- `moderation-href` unit test.

## Documentation

- This file: on completion, set `Status: done` and add Implementation Notes.
- `docs/features/genre-status.md` "Hand-off": already noted during planning that #302 chose alias rows over delete. The developer adds a pointer under "Out of scope" when this ships.
- `docs/features/admin-navigation-restructure.md` phase 7, item 2: links here (done during planning). Mark it done when this ships.
- `docs/ROADMAP.md` #290 line: links here (done during planning). Mark #302 done when this ships. Tick the line only after #303 also ships.
- `docs/features/duplicate-author-merge.md`: one line noting that genre merge deliberately uses aliases instead of delete, with a link here.
- Swagger regenerated.

## Risks

- **Merges are irreversible from the UI.** An alias cannot be revived, and its slug and QID stay reserved (409 on reuse). A wrong merge needs manual SQL. Mitigation: a confirm dialog with explicit text, and hints that only suggest.
- **Hint noise.** `shared_word` on common words ("литература") produces noisy hints, while rare wordings and synonyms ("фэнтези"/"фентези" pass, "sci-fi"/"фантастика" do not) are missed. Mitigation: the cap of 5, the match labels and the status/QID display. The admin decides.
- **Forgotten alias handling in a future genre lookup** (for example #303). It fails safe through the rejected-tombstone skip and the CHECK constraint, but the record then loses its genre. #303 must use `resolveHarvestGenre` or `followGenreAlias`.
- **A genre delete feature added later** fails on `ON DELETE RESTRICT` while aliases point to the genre. This is intentional, and that feature must handle aliases.
- **Similarities and home caches** reflect a merge only after the daily recompute or the cache TTL.
- **Concurrent moderation:** last write wins. A merged-away ID shows up as a "not found" row error or a 404, which is consistent with authors.
- **A target that is a different concept.** Merging two genres with different QIDs makes both QIDs resolve to the target. This is intended (rule 4) but irreversible. The UI shows the "разные QID" warning.
- **Fake churn.** The new repository methods touch every fake of the admin repository and service interfaces. The changes are mechanical.

## Implementation Notes

- Migration `000018_genres_merged_into` (up/down) is created but **not applied** by the developer agent; apply it with `migrate up` (never `migrate down`). Until it is applied, every admin genre read that filters `merged_into_id IS NULL` fails, so apply it together with the deploy.
- Domain: `Genre.MergedIntoID *uuid.UUID` (`column:merged_into_id`) and `IsAlias()`. `genreResponse` is unchanged; `merged_into_id` is never serialized.
- Repository (`admin/repository/genre_moderation.go`): `ListGenresForModeration` (flat scan, `SUM(CASE ...)` stats join, `created_at DESC, name ASC, id ASC`), `ListGenreSampleBooks` (one `ROW_NUMBER()` query, reuses `SampleBook`/`sampleBooksPerAuthor`), `ListGenreHintCandidates` (one query: `status <> 'rejected' AND merged_into_id IS NULL`, selects only id/name/slug/status/wikidata_qid), `SetGenreStatus` (`Updates(map{status})`, `WHERE id = ? AND merged_into_id IS NULL`), `MergeGenres` (rule 4 in one transaction; both rows are re-read inside it with `merged_into_id IS NULL`, `FOR UPDATE` only on Postgres, ordered by id; a missing/alias row returns `gorm.ErrRecordNotFound`). `ListGenres`, `GetGenreByID`, `GenresExist` and the dashboard `genres` count filter `merged_into_id IS NULL`; `GetGenreBySlug`, `GetGenreByQID` and `GenreSlugExists` stay unfiltered.
- Service: `genre_moderation.go` (`ListGenreModerationQueue`, `BulkSetGenreStatus` returning `GenreBulkStatusReport` with `genre_id` row errors, pure `normalizeGenreName`/`genreStem`/`genreMatchKind`/`buildGenreHints`). `genreStem` strips the longest matching suffix and keeps the token when fewer than 4 runes would remain (no fallback to a shorter suffix). `merge_genre.go`: `MergeGenres` + `ErrCannotMergeGenreIntoSelf` (reuses `ErrGenreNotFound`; a `gorm.ErrRecordNotFound` from the repository merge, i.e. a concurrent merge, also maps to it). `harvest_resolve.go`: `followGenreAlias` (one hop; a missing target or an alias target is logged at error level with both IDs and returned as an error), called on the QID and slug paths of `resolveHarvestGenre` before the status checks and the backfill, and in `ResolveOrCreateGenreIDsByNames`; `GenreResolution.ViaAlias`; `genreMatchOutcome` helper.
- HTTP: `ListGenreModerationQueue`, `BulkSetGenreStatus`, `MergeGenres` handlers in the existing admin-only `genres` group (`requireBearer` on both POSTs); swagger regenerated (additive only).
- Frontend: api-client types + `api.admin.genres.{moderationQueue,bulkStatus,merge}` (exposed via `adminGenresApi`); `views/genre-moderation`, `features/moderate-genre`, `features/merge-genre`, `entities/genre/lib/moderation-href.ts` (exported, no caller yet). Every row status (including rejected) has a "Слить" action. `widgets/section-tabs` gained the `moderation.genres` screen-reader badge label ("ожидают проверки") - without it the new tab badge would have an empty sr label. `/catalog/genres` has the "Очередь модерации" header link and appends "(возможно, у слитого жанра)" to a 409 whose message mentions the QID.
- Tests: backend `admin/repository/genre_moderation_sqlite_test.go` (DDL with both CHECKs; atomicity via a SQLite trigger that aborts the last merge step), `admin/service/genre_moderation_test.go`, `admin/transport/http/handler_genre_moderation_test.go`, `catalog/repository/genre_status_sqlite_test.go` (alias absent publicly), harvest `genre_status_test.go` (`processRecord` alias case); fakes updated in all admin repository/service fakes and the harvest fake (their `GetGenreByID` hides aliases like the repository). Admin vitest: genre moderation view, merge dialog, moderation href, nav counters, pending total, section tabs, dashboard tile, genres view.
- Review follow-up: admin `UpdateGenre` (PATCH) no longer uses `Save`; the repository writes only `name`, `slug`, `status`, `wikidata_qid` via `Updates(map)` with `WHERE id = ? AND merged_into_id IS NULL` (a nil QID still clears the column) and returns `gorm.ErrRecordNotFound` on 0 rows, which the service maps to `ErrGenreNotFound` (404). A PATCH racing a merge therefore can no longer write back `merged_into_id = NULL` and resurrect the alias. `ErrBulkStatusTooLarge`, shared by authors and genres, now says `too many ids: at most 100 per request` (no test or frontend matched the old text). Tests: `TestUpdateGenre_EditableColumnsOnlyAndAliasNotFound` (SQLite) and the `merged concurrently` subtest of `TestUpdateGenre_StatusAndQID`. The fake `UpdateGenre` in `service_test.go` now returns `gorm.ErrRecordNotFound`.
- Not verified: the Postgres-only parts (`ILIKE` search, `FOR UPDATE`, the migration itself) and a manual check on the local stack (migration not applied by the agent).
