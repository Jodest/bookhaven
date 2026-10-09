# Book Status Model (pending / published / rejected)

Status: done

Issue: #311. Epic #310 "Admin catalog/import/moderation unification", task B1. This is the book version of [genre-status.md](genre-status.md) and [author-status.md](author-status.md).

## Goal

Books use the same `status` lifecycle as authors and genres. `books.is_hidden` is **removed completely**. There is no compatibility shim, no generated column and no `is_hidden` left in any book API. Every public read shows only `published` books. Admin create, update, import, harvest and seed set the status explicitly. `POST /admin/books/bulk-status` replaces `POST /admin/books/bulk-publish`.

**Hard constraint:** `reviews.is_hidden` and all review hiding logic stay **untouched**. Only book predicates change.

## Out of scope

- A book moderation queue endpoint with context (B3, #313). The existing `/moderation/books` view only gets the minimal adaptation described below.
- Restructuring the import section (F1), redesigning the catalog (F3/F4). Admin UI changes here are minimal adaptations only.
- Any change to review hiding (`reviews.is_hidden`, review moderation, `ReviewFilter.Visibility`, `/moderation/reviews?visibility=`).
- Linking book publication to author or genre status.

## User Stories

- As a reader, I see only books an admin has published, in every public surface: catalog, book page, search, home blocks, similar books, shelves, lists, ratings and reviews.
- As an admin, I see each book's status, I can filter by it, and I can set it in the book form.
- As an admin, I can publish or reject pending books in bulk from `/moderation/books`.
- As an admin, harvested books always arrive as `pending`. When importing a file, I choose the status for the whole batch.

## Business Rules

1. `books.status` is one of `pending`, `published`, `rejected`. It is NOT NULL, has a CHECK constraint and a DB default of `pending`, the same as genres: an unknown insert counts as unvetted. Go code must always set `Status` explicitly.
2. Migrating existing rows: `is_hidden = false` becomes `published`. Anything else becomes `pending`. Per the user, existing records need no special care.
3. Creation paths:
   | Path | Status |
   |---|---|
   | `POST /admin/books` | `status` from the body if given (must be valid, otherwise 400). If omitted, `published`: manual creation is vetted, like author/genre rule 2. |
   | `PATCH /admin/books/:id` | `status` is optional: omitted means unchanged, invalid means 400. Any value can change to any value. |
   | File import (`POST /admin/books/import`) | Form field `status` (`pending` or `published`) applies to the whole batch. If empty or absent, `published` (keeps today's default of "visible"). Anything else, including `rejected`, returns 400. |
   | Harvest (`harvest/service/handler.go`) | always `pending` |
   | Seed (`cmd/seed`) | `published`, set explicitly in Go |
4. A book is publicly visible **only** when `status = 'published'`. A `pending` or `rejected` book is a 404 on detail pages. It is also excluded from lists, search, home, similar books, shelves and lists, and it cannot be rated or reviewed (the `BookExists` checks). Public response shapes do not change, and no `status` field appears publicly.
5. Admin reads see every status. `AdminBook.status` is always present.
6. The admin list filter `?status=pending|published|rejected` (empty means all; invalid means 400) replaces `?visibility=`. The old `pending_review` predicate (`is_hidden AND external_source IS NOT NULL`) becomes simply `status = 'pending'`, which **now includes manually created pending books**. The `source` filter stays independent and is ANDed with status, so "pending harvested" is `status=pending&source=harvested`. The `visibility` param is removed. If sent, it is silently ignored, like any unknown param.
7. `POST /admin/books/bulk-status` mirrors `/admin/authors/bulk-status` and `/admin/genres/bulk-status` exactly:
   - Body: `{ids: string[], status}`. An invalid status, empty `ids` or a non-UUID id returns 400.
   - At most 100 distinct ids after deduplication; more returns 400 (`ErrBulkStatusTooLarge`).
   - Each id is processed independently. An unknown id becomes the row error "book not found".
   - Responds 200 with `{processed, updated, errors[{book_id, title, message}]}`.
   - Only `status` (and `updated_at`) changes. No other field is touched and no content re-validation runs. Today's bulk publish only re-ran `UpdateBook` with unchanged fields, so it behaved the same in practice: the title check is grandfathered for an unchanged title.
   - Requires the `adminRole` group plus `requireBearer`. Moderators get 403.
8. `POST /admin/books/bulk-publish` (the search-based "publish all pending") is **removed**, along with `BulkPublishBooks`, `BulkPublishReport`, `ErrBulkPublishTooLarge` and `bulkPublishCap`.
9. In the author and genre moderation queues, `books_visible` keeps its JSON name but now counts linked books with `status = 'published'`. `sample_books[].is_hidden` is replaced by `sample_books[].status`.
10. Dashboard changes:
    - `hidden_books` and `pending_import_review` are removed.
    - `pending_books` (count of `status = 'pending'`) and `rejected_books` (count of `status = 'rejected'`) are added.
    - `books` still counts all statuses.
    - `pending_books` takes over every role of `pending_import_review`: the moderation tile, the `moderation.books` nav tab badge and the sidebar moderation total.

## Acceptance Criteria

DB
- [ ] Migration `000019_books_status` adds `status` with its CHECK constraint, default `pending` and the rule-2 backfill. It drops `idx_books_is_hidden` and `books.is_hidden`, and adds the partial index. The down migration restores them (provided per convention; **never run by agents**).

Public (book predicates only)
- [ ] Every non-review `is_hidden` occurrence in the classification table below now uses `status = 'published'` (table-qualified where a join exists).
- [ ] A `pending` or `rejected` book is 404 on `GET /books/:slug`. It is absent from `GET /books` (list and search), the author page books, similar books (live and precomputed job), every home block, random book, the review feed, shelves and lists (own, public, profile) and their counts. It cannot be rated, reviewed, shelved or added to a list.
- [ ] Review hiding behaves exactly as before. Every `reviews.is_hidden` / `r.is_hidden` predicate and `Review.IsHidden` is unchanged.

Admin API
- [ ] `AdminBook` has `status`. `is_hidden` is gone from every book-related admin response and request.
- [ ] `GET /admin/books?status=` filters by status; an invalid value returns 400. `visibility` is gone.
- [ ] `POST /admin/books` accepts an optional `status` (defaults to `published`; invalid returns 400). `PATCH` accepts an optional `status` (omitted means unchanged; invalid returns 400).
- [ ] `POST /admin/books/bulk-status` follows rule 7. `/bulk-publish` returns 404/405 (the route is removed).
- [ ] `POST /admin/books/import` follows rule 3 for the `status` form field. The `is_hidden` form field is gone.
- [ ] Harvested books are created `pending`. Seed books are `published`.
- [ ] The author and genre moderation queues follow rule 9.
- [ ] `GET /admin/dashboard` follows rule 10.
- [ ] Swagger is regenerated. `bulk-publish` and the book `is_hidden` fields are gone from `backend/docs`. The review `is_hidden` fields remain.

Admin UI
- [ ] The book form has a status select (Опубликована / На модерации / Отклонена) instead of the visibility select. New books default to "Опубликована". The existing confirm on a visibility change becomes a confirm on a status change.
- [ ] The books list has a status filter bound to `?status=` and a status badge column. The legacy `?visibility=` param is ignored. The "Импортировано" badge stays.
- [ ] `/moderation/books` lists `status=pending` (all sources). Per-row "Опубликовать" and "Отклонить" buttons and a header "Опубликовать все на странице (N)" all call `bulkStatus`.
- [ ] The import view has a batch status select (Опубликованы / На модерации) instead of the hidden select.
- [ ] The dashboard has a moderation tile "Книги на модерации" = `pending_books` -> `/moderation/books`. The catalog tile "Отклонённых книг" = `rejected_books` -> `/catalog/books?status=rejected` replaces "Скрытых книг".
- [ ] The nav counters and `pendingModerationTotal` use `pending_books`.
- [ ] The author and genre moderation sample books show a non-published marker based on `status`.

## API

Admin (`admin/transport/http/handler.go`):
- `bookResponse`: replace `IsHidden bool json:"is_hidden"` with `Status string json:"status"`.
- `createUpdateBookRequest`: replace `IsHidden bool` with `Status *string json:"status,omitempty"`. Validate it in the handler (400 `ErrInvalidBookStatus`) and again in the service.
- `GET /admin/books`: add `@Param status`, replacing `@Param visibility`. Validate with `catalogDomain.IsValidBookStatus` (empty allowed), otherwise 400.
- New `bulkBookStatusRequest{IDs []string; Status string}` and `BulkSetBookStatus` handler, a copy of `BulkSetGenreStatus` (lines ~1468-1517). Route: `books.POST("/bulk-status", adminHandler.BulkSetBookStatus, requireBearer)` in `internal/transport/http/router.go`. Remove the `/bulk-publish` route, `bulkPublishBooksRequest` and the `BulkPublishBooks` handler. Update the `Service` interface (line ~49).
- Import handler (line ~1743): `status := strings.TrimSpace(c.FormValue("status"))`. Empty becomes `published`. Anything other than `pending`/`published` returns 400. Update the `@Description`.
- `authorSampleBookResponse` and `genreSampleBookResponse`: `IsHidden` is replaced by `Status string json:"status"`.
- `dashboardResponse`: remove `HiddenBooks` and `PendingImportReview`; add `PendingBooks json:"pending_books"` and `RejectedBooks json:"rejected_books"`.
- **Do not touch** `moderateReviewRequest`, the review response `IsHidden`, `ListReviews` or the reviews `visibility` param.

Public: no shape changes.

## Database

`backend/migrations/000019_books_status.up.sql`:
```sql
ALTER TABLE books ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'pending';
ALTER TABLE books ADD CONSTRAINT chk_books_status CHECK (status IN ('pending','published','rejected'));
UPDATE books SET status = 'published' WHERE is_hidden = FALSE;
DROP INDEX IF EXISTS idx_books_is_hidden;
ALTER TABLE books DROP COLUMN is_hidden;
CREATE INDEX idx_books_status_not_published ON books(status) WHERE status <> 'published';
```

`000019_books_status.down.sql`, in reverse order:
```sql
DROP INDEX IF EXISTS idx_books_status_not_published;
ALTER TABLE books ADD COLUMN is_hidden BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE books SET is_hidden = (status <> 'published');
CREATE INDEX idx_books_is_hidden ON books(is_hidden);
ALTER TABLE books DROP CONSTRAINT IF EXISTS chk_books_status;
ALTER TABLE books DROP COLUMN status;
```

- Index rationale: public reads hit `status = 'published'`, which matches the vast majority of rows. The old `idx_books_is_hidden` was a low-selectivity btree that brought no benefit there either. The partial index serves admin counts, the admin list filter and the queue (`pending`/`rejected`), using the same pattern as `idx_authors_status_not_published` and `idx_genres_status_not_published`. Do not add a full `status` index.
- `idx_reviews_is_hidden` (from migration 000007) and `reviews.is_hidden` are **not touched**.
- Domain `catalog/domain/book.go`:
  - replace `IsHidden` with `Status string gorm:"type:varchar(20);not null;default:'pending'"`
  - add constants `BookStatusPending/Published/Rejected`, `func (b Book) IsPublished() bool` and `func IsValidBookStatus(string) bool`
  - copy the shape from `author.go` and `genre.go`
- **Agents must NEVER run `migrate down`, drop or truncate tables, reset volumes or run any other destructive DB command.** `migrate down` without a count rolls back ALL migrations and wipes the local DB. Apply with `migrate up` only, or leave applying to the human.

## Backend: `is_hidden` classification (non-test code)

**Column ambiguity:** `authors.status` and `genres.status` exist, and both tables are joined in book queries. Every new book predicate inside a join must be table-qualified (`books.status` or `b.status`). Bind the value as a parameter or use the literal `'published'`, matching the surrounding style.

| File | Line(s) | Kind | Change |
|---|---|---|---|
| `catalog/domain/book.go` | 22 | BOOK | field -> `Status` (see Database) |
| `catalog/repository/repository.go` | 81, 161, 172 (author page `Books` preload), 229, 243, 333 (`b.`), 395 | BOOK | `status = 'published'` |
| `catalog/repository/similarity.go` | 106 (`b.`), 123 | BOOK | same |
| `home/repository/repository.go` | 74 (comment), 165, 214, 253, 279, 304, 315, 329, 348, 366, 382, 411, 455 | BOOK | `books.status` / `b.status = 'published'` |
| `home/repository/repository.go` | 330, 349 (`r.is_hidden`) | REVIEW | **keep** |
| `shelves/repository/repository.go` | 114 (comment), 126, 240, 272 | BOOK | same |
| `lists/repository/repository.go` | 176, 208 | BOOK | same |
| `ratings/repository/repository.go` | 102 | BOOK | same |
| `reviews/repository/repository.go` | 262 (`BookExists`; also fix its doc comment) | BOOK | same |
| `reviews/repository/repository.go` | 56, 69, 100, 108, 126, 302 | REVIEW | **keep** |
| `reviews/service/service.go` | 149, 189, 208 | REVIEW | **keep** |
| `reviews/domain/review.go` | 18 | REVIEW | **keep** |
| `cmd/seed/demo_readers.go` | 191 | REVIEW | **keep** |
| `admin/repository/repository.go` | 164-168 (`BookFilter.Visibility` -> `Status`), 288-295 (filter switch -> `WHERE status = ?` when set), 247 and 260 (dashboard -> `PendingBooks`/`RejectedBooks`), 206 (comment), 217 (`SampleBook.IsHidden` -> `Status string`), 320-333 (the `maxBookListLimit` comment refers to bulk publish; reword it or keep the cap at 100) | BOOK | change |
| `admin/repository/repository.go` | 721-747, 758, 774, 801, 829-840, 257 (`PendingReviews`) | REVIEW | **keep** |
| `admin/repository/author_moderation.go` | 17 (`SUM(CASE WHEN b.status = 'published' THEN 1 ELSE 0 END)`), 137-157 (select `b.status`) | BOOK | change |
| `admin/repository/genre_moderation.go` | 27, 39, 130-150 | BOOK | same as authors |
| `admin/service/service.go` | 183, 261, 353 | BOOK | `BookInput.Status *string`. Create: nil means `published`, invalid means `ErrInvalidBookStatus`. Update: nil means unchanged. |
| `admin/service/import.go` | 73 (comment), 132; the `ImportBooks(ctx, rows, defaultHidden bool)` signature | BOOK | becomes `defaultStatus string` |
| `admin/service/bulk_publish.go` | whole file | BOOK | **delete**; add `BulkSetBookStatus` (see below) |
| `admin/transport/http/handler.go` | 132, 175, 216, 264, 462, 671, 739, 1018, 1444, 1692, 1743, 1749-1780, 313-320, 551, 568 | BOOK | see API |
| `admin/transport/http/handler.go` | 309, 376, 480, 1577, 1590, 1642 | REVIEW | **keep** |
| `harvest/service/handler.go` | 13 (comment), 345 | BOOK | `IsHidden: true` -> `Status: &pending` (`BookInput.Status`) |
| `cmd/harvest/main.go` | 18 | BOOK | comment only |
| `migrations/000001`, `000007` | - | history | do not edit |
| `backend/docs/*` | - | generated | regenerate |

The developer must re-grep `is_hidden|IsHidden` across `backend/` when finished. Only review occurrences, historical migrations and tests of review behavior may remain.

### Service

- New file `admin/service/book_moderation.go` (or replace `bulk_publish.go`):
  - `bulkBookStatusCap = 100`
  - `BookBulkStatusReport{Processed, Updated int; Errors []BookBulkStatusRowError}` with `BookBulkStatusRowError{BookID json:"book_id", Title json:"title", Message}`
  - `BulkSetBookStatus(ctx, ids, status)`, a copy of `BulkSetGenreStatus`: dedupe, cap -> `ErrBulkStatusTooLarge`, per-id `repo.SetBookStatus(ctx, id, status) (found bool, err error)`
  - unknown id -> `ErrBookNotFound.Error()` as the row message
- New `ErrInvalidBookStatus`.
- `ListBooks` service and handler validate `BookFilter.Status`.
- `DashboardStats` drops `HiddenBooks` and `PendingImportReview`, and gains `PendingBooks` and `RejectedBooks`.
- Repository: add `SetBookStatus` (`UPDATE books SET status = ?, updated_at = now() WHERE id = ?`; found = rows affected > 0) to the interface. Update every fake implementing the repository or service interfaces:
  - `admin/transport/http/handler_test.go`
  - `admin/service/service_test.go`
  - `harvest/service/handler_test.go`
  - any other fake the compiler flags

Seed (`cmd/seed/main.go` ~293-545): add `Status: catalogDomain.BookStatusPublished` to every seed book. `FirstOrCreate` leaves existing rows alone, and the migration backfills those to `published`.

## Frontend

`packages/api-client/src/index.ts`
- `export type AdminBookStatus = 'pending' | 'published' | 'rejected'`.
- `AdminBook`: `is_hidden` is replaced by `status: AdminBookStatus`.
- `AdminBookInput`: `is_hidden` is replaced by `status?: AdminBookStatus` (create: omitted means published; update: omitted means unchanged).
- `admin.books.list` params: `visibility` is replaced by `status?: AdminBookStatus | ''`.
- `admin.books.import(file, { status?: 'pending' | 'published' })` sends form field `status`.
- `bulkPublish` and `BulkPublishReport`/`BulkPublishRowError` are replaced by `bulkStatus(ids, status)` returning `AdminBookBulkStatusReport {processed, updated, errors: {book_id, title, message}[]}`, with the same shape as the genre version.
- `AdminAuthorSampleBook` and the genre sample book type: `is_hidden` is replaced by `status: AdminBookStatus`. Fix the "Linked books with is_hidden = false" doc comments (~396, ~443) to say "status = published".
- `AdminDashboard`: remove `hidden_books` and `pending_import_review`; add `pending_books` and `rejected_books`.
- **Do not touch** `AdminReview.is_hidden`, `admin.reviews.list({visibility})` or `moderate(id, isHidden)`.

`apps/admin` (minimal, no redesign)
- `entities/book/ui/book-status-badge.tsx` (`BookStatusBadge`, `BOOK_STATUS_LABELS`): copy `AuthorStatusBadge`/`GenreStatusBadge`. Export it from `entities/book`. `entities/book/api/index.ts`: `bulkPublish` is replaced by `bulkStatus`.
- `features/manage-book/ui/book-form.tsx` (~60-67, 169-170, 335-343): a `status` select. The default for create is `published`. The confirm text changes to "сменить статус на «…»".
- `views/books-list/ui/books-list-view.tsx`: `?status=` param (allow only valid values), in the query key, page reset; a `BookStatusBadge` in the Status column.
- `views/book-moderation/ui/book-moderation-view.tsx`: `status: 'pending'`. Row actions call `bulkStatus([id], 'published' | 'rejected')`. The header button "Опубликовать все на странице (N)" calls `bulkStatus(pageIds, 'published')` (page size 20 < cap 100). Update the doc comment, since search no longer defines the bulk scope. Show the report's `errors` like today.
- `views/book-import/ui/book-import-view.tsx`: batch status select, default `published`.
- `views/dashboard/ui/dashboard-view.tsx`: apply rule 10's tile changes.
- `features/nav-counters/lib/build-nav-counters.ts` and `entities/dashboard/lib/pending-moderation.ts`: `pending_import_review` is replaced by `pending_books`.
- `views/author-moderation/ui/author-moderation-view.tsx:519` and `views/genre-moderation/ui/genre-moderation-view.tsx:443`: show the marker when `b.status !== 'published'`, with the label from `BOOK_STATUS_LABELS` (e.g. "(на модерации)" / "(отклонена)").
- Leave `reviews-view.tsx` and `moderate-review-button.tsx` untouched.

`apps/web`: none. A grep confirmed it uses no book `is_hidden` and no admin book types.

## Existing Patterns

- Migrations `000015_authors_status_external_id`, `000017_genres_status_wikidata_qid` (CHECK, partial index).
- `catalog/domain/genre.go` / `author.go` status helpers.
- `admin/service/genre_moderation.go` `BulkSetGenreStatus`, `handler.go` `BulkSetGenreStatus`, `SetGenreStatus` repo method, `handler_genre_moderation_test.go`.
- `entities/genre/ui/genre-status-badge.tsx`, the `views/genres` status filter, the genre form status select.

## Tests

Backend: run in Docker (`docker compose run ...`), changed packages only. Run `gofmt` on changed files only, **never `gofmt -w` the whole tree**. Tests apply no migrations.
- Update every SQLite DDL in tests that creates `books` with `is_hidden` to use `status`: `catalogStatusDDL` in the catalog repository tests, admin/home repository SQLite tests and the moderation SQLite tests. Keep `reviews.is_hidden` in the review DDLs.
- Catalog repository (SQLite):
  - `ListBooks`, `GetBySlug`, the author page books, `GetBooksByIDs`, `GetSimilarSource`/`ListSimilarCandidates` and the similarity job pool all exclude `pending` and `rejected` books
- Home repository:
  - random, new, popular, top, review feed and similar blocks exclude non-published books (SQLite where feasible, otherwise SQL-text checks)
  - the review feed still excludes hidden reviews
- Shelves, lists, ratings, reviews:
  - `BookExists`/add-item paths reject pending and rejected books
  - shelf and list items and counts exclude them
  - a review regression test: a hidden review on a published book stays hidden
- Admin repository:
  - status filter
  - `SetBookStatus` found/not found
  - dashboard `PendingBooks`/`RejectedBooks`
  - the author and genre moderation `books_visible` and sample `status`
- Admin service:
  - create default `published`, explicit status, invalid status
  - update omitted, changed and invalid
  - `BulkSetBookStatus`: dedupe, cap 101 -> error, unknown id row error, invalid status
  - import default `published`, `pending` batch
- Admin handler:
  - `status` in the response
  - `?status=` invalid -> 400
  - create/update invalid status -> 400
  - bulk-status: 400 cases, 200 report, Bearer required
  - import `status` field (invalid -> 400)
  - dashboard fields
  - delete `handler_bulk_publish_test.go` and `bulk_publish_test.go`
- Harvest: a new book is `pending`.
- Domain: `IsValidBookStatus` and `IsPublished`.

Frontend (admin vitest, native):
- book form status select (create default, update sends status)
- books list filter and badge
- book moderation (list param, row publish/reject, page bulk)
- import status select
- dashboard tiles
- nav counters / pending-moderation
- author/genre moderation sample markers
- update every `AdminDashboard` and `AdminBook` fixture

`tsc`/vitest run natively. The CI frontend build is the typedRoutes gate. Do not run a native `next build`.

## Documentation

- This contract: set `Status: done` and add Implementation Notes when finished.
- `docs/features/harvest-bulk-publish.md`: add a "Superseded by book-status-model.md (bulk-status)" note.
- `docs/features/harvest-review-queue.md` and `admin-book-list-filters.md`: note that `visibility`/`pending_review` is replaced by `status`.
- `docs/features/admin-navigation-restructure.md`: `/moderation/books` now lists `status=pending`.
- Regenerate Swagger: `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal` (in Docker).

## Risks

- **A missed book predicate** leaks unpublished books publicly, or a review predicate gets changed by mistake. This is mitigated by the classification table and the final grep. Reviewers should diff-check that no `r.is_hidden` / `reviews.is_hidden` line changed.
- **Ambiguous `status` column** in joined queries (authors and genres have `status` too) causes SQL errors or a wrong filter. Always qualify it.
- **Breaking API change**: `is_hidden`, `visibility`, `bulk-publish`, `hidden_books` and `pending_import_review` are removed. Admin is the only consumer and it is updated in the same PR. There is no prod.
- **Scope change of `/moderation/books`**: it now includes manually created pending books. The "publish all matching search" action becomes "publish this page". This is intended. B3 (#313) redesigns the queue.
- **Bulk-status skips content validation.** This is equivalent to today's behavior because the title check is grandfathered, but publishing no longer re-validates the description language. This is accepted for parity with authors and genres.
- **Precomputed similarities and cached home selections** reflect status changes only after the recompute or the cache TTL. Book reads re-check status per request, as with `is_hidden`.
- **The migration is irreversible for nuance**: the down migration maps `rejected` to hidden, which is lossy. Agents never run down anyway.

## Implementation Notes

- Migration `000019_books_status` (up + down) follows the contract SQL exactly. The developer agent applied it to the local DB with `migrate up` only (all 156 local books became `published`); the down migration was never run.
- `domain.Book`: `IsHidden` replaced by `Status`; constants `BookStatusPending/Published/Rejected`, `Book.IsPublished()`, `IsValidBookStatus()`.
- Public predicates: every non-review `is_hidden` occurrence from the classification table now filters `status = 'published'`. Predicates inside joins are qualified (`books.status`, `b.status`); single-table raw queries (`ListNewBookIDs`, `ListTopRatedBookIDs`, the anniversary query, `ListVisibleBookStats`, the `BookExists` helpers) use bare `status`. All `reviews.is_hidden` / `r.is_hidden` predicates, `Review.IsHidden`, `ReviewFilter.Visibility` and the review moderation code are unchanged.
- Admin service: `BookInput.Status *string` (create: nil = `published`; update: nil = unchanged; invalid = `ErrInvalidBookStatus`, mapped to 400 in the handlers, which also validate before calling the service). `ImportBooks(ctx, rows, defaultStatus string)`: `""` falls back to the create default (`published`). `ListBooks` validates `BookFilter.Status`.
- `admin/service/book_moderation.go` replaces `bulk_publish.go`: `BulkSetBookStatus` is a copy of `BulkSetGenreStatus` (dedupe, `bulkBookStatusCap = 100` -> shared `ErrBulkStatusTooLarge`, per-id `repo.SetBookStatus`, unknown id -> "book not found"). The row error `title` is always present but empty, the same as the genre report's `name` (the status-only update does not load the book).
- `repo.SetBookStatus` writes only `status` and `updated_at` (explicit column map, not `Save`).
- `maxBookListLimit` in the admin `ListBooks` is now 100: its only reason to be 1000 was bulk publish.
- Harvest sets `Status: &pending`; seed sets `Status: catalogDomain.BookStatusPublished` on all 19 seed books.
- Test fakes for the public modules (catalog, home, shelves, lists, reviews services/handlers) treat a fixture with an empty `Status` as published (`Status == "" || IsPublished()`), so the many existing zero-value fixtures keep meaning "visible"; only an explicit `pending`/`rejected` hides a book there. Real-SQL SQLite tests cover the actual predicates: `catalog/repository/book_status_sqlite_test.go`, `home/repository/book_status_sqlite_test.go` (plus SQL-text checks for the Postgres-only queries), `shelves|lists|ratings|reviews/repository/book_status_sqlite_test.go`, `admin/repository/book_status_sqlite_test.go`.
- Admin UI: `entities/book` exports `BookStatusBadge` / `BOOK_STATUS_LABELS` (Опубликована / На модерации / Отклонена). `/moderation/books` keeps its title "Книги на проверке"; row "Опубликовать"/"Отклонить" and the header "Опубликовать все на странице (N)" all call `bulkStatus` (with a confirm each) and show the report summary and the first 3 row errors (`title || book_id`). The author/genre moderation sample books show `(на модерации)` / `(отклонена)` from `BOOK_STATUS_LABELS`.
- Swagger regenerated in Docker; `bulk-publish`, `hidden_books`, `pending_import_review` and the book `is_hidden` fields are gone, the review `is_hidden` fields and the reviews `visibility` param remain.
