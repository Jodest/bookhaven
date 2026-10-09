# Book Moderation Queue Endpoint

Status: done

Issue: #313 (B3 of epic #310, admin catalog/import/moderation unification). Builds on [book-status-model.md](book-status-model.md) (#311: `books.status`, `POST /admin/books/bulk-status`). Mirrors [author-moderation-queue.md](author-moderation-queue.md) and [genre-moderation-queue.md](genre-moderation-queue.md) as closely as possible.

## Goal

Give the admin UI one read endpoint, `GET /api/v1/admin/books/moderation`, that lists books by moderation status. Each row carries the context a moderator needs to decide: authors and genres with their own status (a book can reference pending authors/genres), external source/id, ISBN, year, cover, creation date, and "possibly the same book" hints. The endpoint only lists. Status changes keep using `POST /admin/books/bulk-status` (B1).

## Out of scope

- Any admin UI change. `/moderation/books` stays as is in this PR. The UI moves to a shared moderation widget in F4 (#317), which will consume this endpoint.
- Bulk actions, merge of books, deleting books.
- Changes to `GET /admin/books` or `GET /admin/books/:id` (their response shape stays byte-identical; `external_id` is therefore exposed only on the queue item, see API).
- Migrations, new indexes, pg_trgm/fuzzy matching.
- Widening to moderators. Admin-only, like the author/genre queues.

## User Stories

- As an admin (via F4 UI later), I list pending books newest first and see for each one its authors and genres with their status, its source (manual or harvested, with external id), ISBN, year and cover.
- As an admin, I see "possibly the same book" hints (same ISBN, or same title by a shared author), so I can reject duplicates instead of publishing them.
- As an admin, I filter the queue by status tab, by source and by a search string, with pagination.

## Business Rules

1. **Queue membership is by `books.status` only.** Default `status=pending`; `published` and `rejected` are the other tabs. Ordering: `created_at DESC, title ASC, id ASC`.
2. **Filters** (validated in the service, error wraps `ErrInvalidModerationFilter` → HTTP 400 with `{"error": "<message>"}`):
   - `status`: `""` → `pending`; otherwise must satisfy `catalogDomain.IsValidBookStatus`. Message: `invalid moderation filter: status must be pending, published or rejected`.
   - `source`: `""` (any), `manual` (`external_source IS NULL`), `harvested` (`external_source IS NOT NULL`), same semantics as `BookFilter.Source`. Anything else → `invalid moderation filter: source must be manual or harvested`. (Note: the list endpoint silently ignores unknown values; the queue rejects them like the author/genre queues reject bad `qid`.)
   - `search`: the **same full-text condition as `ListBooks`** (`to_tsvector('russian', title || ' ' || COALESCE(isbn,'') || ' ' || COALESCE(description,'')) @@ plainto_tsquery('russian', ?)`), so it hits the existing GIN index `idx_books_search` and covers title and ISBN. Postgres-only, like author search.
   - `page`, `limit`: `parsePagination`; default limit 20, handler clamps to 50, service clamps to 50 (`bookQueueDefaultLimit = 20`, `bookQueueMaxLimit = 50`), repository keeps its defensive 20/100 clamp.
3. **Per-book context:** authors (`authorResponse`, incl. `status`), genres (`genreResponse`, incl. `status`), `isbn`, `published_year`, `cover_url`, `external_source`, `external_id`, `created_at`. Authors and genres are loaded with `Preload("Authors").Preload("Genres")` on the page query (one query each, page-bounded, like `ListBooks`). No author/genre status filtering: rejected/pending linked entities are shown as-is, with status, because that is exactly what the moderator needs to see.
4. **"Possibly the same book" hints** — display-only, recomputed per call, never persisted.
   - Match kinds, strongest first:
     - `same_isbn`: normalized ISBNs are equal. Normalization: strip `-` and spaces, uppercase. Only normalized values of length 10 or 13 participate. An ISBN-10 also matches its `978`-prefixed ISBN-13 and vice versa (the service expands each page ISBN into both forms, recomputing the check digit; `979` ISBN-13s have no ISBN-10 form). Checksum validity is not required.
     - `same_title_author`: normalized titles are equal **and** the two books share at least one author id. Title normalization = the author/genre normalizer (lowercase, `ё→е`, every non-letter/non-digit rune → space, collapse spaces); add `normalizeBookTitle` (may delegate to the shared logic). Empty normalized title → no match. Books without authors can only get `same_isbn` hints. Title-only matches (no shared author) are **not** hints (too noisy: "Стихи", "Избранное").
   - A candidate matching both kinds is reported once, as `same_isbn`.
   - Candidates: **any status** (pending, published, rejected) — unlike authors/genres, a rejected duplicate is useful ("this was already rejected"). The row itself is excluded. The page's own rows are added to the candidate pool so two duplicates on the same page hint at each other.
   - At most 5 hints per row (`maxBookHints = 5`), ordered by match kind, then status `published` < `pending` < `rejected`, then title, then id. Always `[]`, never null.
   - Cost: at most **two extra queries per page**, both bounded by the page:
     1. ISBN candidates: `SELECT id, title, slug, status, isbn, published_year, external_source FROM books WHERE upper(replace(replace(isbn,'-',''),' ','')) IN ? AND id NOT IN ?` (page-derived ISBN set; skipped when empty). No index exists on isbn; one sequential scan per page request is acceptable at current catalog size (Risks).
     2. Title+author candidates: books linked via `book_authors.author_id IN ?` (page author ids, uses `idx_book_authors_author_id`), `id NOT IN ?` page ids, returning `(book fields, author_id)` rows; the repository groups them into candidates with `AuthorIDs`. Safety cap `LIMIT 2000` rows, ordered `b.id, ba.author_id` for determinism (Risks). Skipped when the page has no authors. The Go side does the normalized-title comparison.
   - SQL must be portable (Postgres + SQLite): `replace`, `upper`, `IN`, no `FILTER`, no `ILIKE` in the hint queries.
5. **Read-only.** The endpoint changes nothing.
6. **Authorization:** route lives in the existing `books := admin.Group("/books", adminRole)` group. Anonymous → 401, moderator/user → 403. GET, so no `requireBearer` (same as the author/genre queue GETs).

## Acceptance Criteria

Backend
- [x] `GET /api/v1/admin/books/moderation` returns 200 `{items, total, page, limit}`; items `{book, external_id, created_at, similar}` (API section).
- [x] Defaults: no params → status `pending`, page 1, limit 20, ordering per rule 1. `limit=500` → 50.
- [x] `status=bogus` → 400; `source=bogus` → 400; both with the `invalid moderation filter: …` message.
- [x] `source=manual|harvested` filters as in `GET /admin/books`; combined with status via AND.
- [x] `search` uses the `ListBooks` full-text condition.
- [x] Each item's authors and genres include their `status`; `external_source`, `external_id`, `isbn`, `published_year`, `cover_url`, `created_at` are present (nullable ones as `null`/omitted consistent with `bookResponse`; `external_id` is always present, `null` when none).
- [x] Hints follow rule 4: `same_isbn` (incl. hyphenated vs plain, ISBN-10 vs 978-ISBN-13), `same_title_author` (normalized title + shared author), self excluded, any status included, dedupe with `same_isbn` winning, cap 5, ordering, `[]` when none, page rows hint at each other.
- [x] Per page: count + list + 2 preloads + at most 2 hint queries; no N+1.
- [x] 401 anonymous, 403 moderator.
- [x] `GET /admin/books` and `GET /admin/books/:id` responses unchanged.
- [x] Swagger annotations added and regenerated (additive diff only).

api-client / admin
- [x] Types `AdminBookQueueItem`, `AdminBookHint`, `AdminBookHintBook`, `AdminBookQueueParams`; method `api.admin.books.moderationQueue(params)`.
- [x] `adminBooksApi.moderationQueue` re-exported in `apps/admin/src/entities/book/api/index.ts`. No other admin change.

## API

`GET /api/v1/admin/books/moderation?status=&source=&search=&page=&limit=` (admin)

```json
{
  "items": [{
    "book": { "...bookResponse": "id, title, slug, cover_url, isbn, published_year, status, external_source, authors[{...authorResponse incl. status}], genres[{...genreResponse incl. status}], created_at, ..." },
    "external_id": "OL123W",
    "created_at": "2026-10-08T10:00:00Z",
    "similar": [{
      "book": { "id": "uuid", "title": "…", "slug": "…", "status": "published", "isbn": "9785…", "published_year": 1999, "external_source": null },
      "match": "same_isbn"
    }]
  }],
  "total": 12, "page": 1, "limit": 20
}
```

- `book` reuses `bookResponse`/`toBookResponse` unchanged. `external_id` sits on the item (not inside `book`) so `bookResponse`, and with it `GET /admin/books`, stays unchanged. `created_at` duplicates `book.created_at` to mirror the author/genre item shape.
- Hint `book` is a compact `bookHintBookResponse` (`id, title, slug, status, isbn, published_year, external_source`; `isbn`/`published_year`/`external_source` as `*T` without omitempty → `null`), because hint candidates are not preloaded with authors/genres.
- `match`: `same_isbn | same_title_author`.
- Errors: 400 invalid filter, 401, 403, 500 `{"error": "failed to list book moderation queue"}`.
- Route: `books.GET("/moderation", adminHandler.ListBookModerationQueue)` in `internal/transport/http/router.go`, registered with the other static `/books/...` routes, before `books.GET("/:id", ...)`.
- Swagger: annotations modeled on `ListGenreModerationQueue` (Russian summary "Очередь модерации книг (админ)", params status/source/search/page/limit, `@Success 200 {object} listBookQueueResponse`, 400/401/403/500), then `docker compose exec -T backend swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`.

## Database

No schema changes, no migration.

## Backend

`admin/repository`:
- New file `book_moderation.go`:
  - `BookModerationFilter{Status, Source, Search string; Limit, Offset int}` (doc comment like `GenreModerationFilter`).
  - `ListBooksForModeration(ctx, f) ([]catalogDomain.Book, int64, error)`: `Model(&catalogDomain.Book{}).Preload("Authors").Preload("Genres")`, filters per rules 1–2, count before limit, order `books.created_at DESC, books.title ASC, books.id ASC`, defensive limit clamp 20/100, offset ≥ 0.
  - `BookHintCandidate{ID uuid.UUID; Title, Slug, Status string; ISBN *string; PublishedYear *int; ExternalSource *string; AuthorIDs []uuid.UUID}`.
  - `FindBooksByNormalizedISBNs(ctx, isbns []string, excludeIDs []uuid.UUID) ([]BookHintCandidate, error)` — rule 4 query 1; empty input → empty slice, no query.
  - `FindBooksByAuthorIDs(ctx, authorIDs, excludeIDs []uuid.UUID) ([]BookHintCandidate, error)` — rule 4 query 2, grouped by book with `AuthorIDs`; empty input → empty slice, no query.
- `repository.go`: add the three methods to the `Repository` interface under a "Book moderation queue" comment block.
- Update every fake implementing the admin `Repository` (admin service test fakes, handler test fakes, harvest service fake) — mechanical.

`admin/service/book_moderation.go` (extend the existing file):
- Constants `bookQueueDefaultLimit = 20`, `bookQueueMaxLimit = 50`, `maxBookHints = 5`; match kinds `BookMatchSameISBN = "same_isbn"`, `BookMatchSameTitleAuthor = "same_title_author"`.
- `BookHint{Book repository.BookHintCandidate; Match string}`, `BookQueueItem{Book catalogDomain.Book; Similar []BookHint}`.
- `ListBookModerationQueue(ctx, filter repository.BookModerationFilter, page, limit int) ([]BookQueueItem, int64, error)`: defaults/validation (rule 2, reuse `ErrInvalidModerationFilter`), page/limit clamp, list, early return on empty page, build ISBN set (with 10↔13 expansion) and author-id set from the page, run the two candidate queries, add page rows (converted to `BookHintCandidate` with their author ids) to the pool, `buildBookHints` per row.
- Pure helpers: `normalizeISBN`, `isbnVariants` (10↔978-13), `normalizeBookTitle`, `bookMatchKind(book, candidate)`, `buildBookHints` (rule 4 ordering, cap, never nil).

`admin/transport/http/handler.go`:
- Add `ListBookModerationQueue` to the handler `Service` interface.
- Types `bookHintBookResponse`, `bookHintResponse{Book bookHintBookResponse; Match string}`, `bookQueueItemResponse{Book bookResponse; ExternalID *string; CreatedAt time.Time; Similar []bookHintResponse}`, `listBookQueueResponse{Items, Total, Page, Limit}`.
- Handler `ListBookModerationQueue`: copy of `ListGenreModerationQueue` (parsePagination, clamp 50, filter from query `status`, `source`, `search`, `errors.Is(err, ErrInvalidModerationFilter)` → 400, else 500).

## Frontend

`packages/api-client/src/index.ts` (next to the genre queue types, comment "Book moderation queue (GET /api/v1/admin/books/moderation, docs/features/book-moderation-queue.md)"):
- `AdminBookHintBook { id; title; slug; status: AdminBookStatus; isbn: string | null; published_year: number | null; external_source: string | null }`
- `AdminBookHint { book: AdminBookHintBook; match: 'same_isbn' | 'same_title_author' }`
- `AdminBookQueueItem { book: AdminBook; external_id: string | null; created_at: string; similar: AdminBookHint[] }`
- `AdminBookQueueParams { status?: AdminBookStatus; source?: 'manual' | 'harvested'; search?; page?; limit? }` (status doc: defaults to 'pending' on the server)
- `api.admin.books.moderationQueue(params?)` → `PaginatedResponse<AdminBookQueueItem>`, built like `genres.moderationQueue`. Name is `moderationQueue` (not `moderation`) for consistency with authors/genres.

`apps/admin/src/entities/book/api/index.ts`: add `moderationQueue: api.admin.books.moderationQueue`. Nothing else in `apps/admin`.

## Existing Patterns

- `admin/service/genre_moderation.go`, `author_moderation.go`: filter validation, clamp, candidate pool incl. page rows, hint ordering/cap.
- `admin/repository/author_moderation.go` `FindAuthorsBySurnames`: coarse SQL prefilter + Go re-check, portable SQL.
- `admin/repository/repository.go` `ListBooks`: preload, search condition, source filter.
- `admin/transport/http/handler.go` `ListGenreModerationQueue` and response types.
- Tests: `admin/repository/genre_moderation_sqlite_test.go`, `book_status_sqlite_test.go` (books DDL), `admin/service/genre_moderation_test.go`, `admin/transport/http/handler_genre_moderation_test.go` (auth chain at the bottom).

## Tests

Backend — run only changed packages via `docker compose exec -T backend go test ./internal/modules/admin/...` (and harvest if its fake changed); gofmt only changed files, never `gofmt -w` the whole tree; no destructive DB commands (`migrate down`, drops, volume resets).
- `admin/service` (new `book_moderation_queue_test.go` or extend existing): status default `pending`; invalid status/source → `ErrInvalidModerationFilter`; page/limit defaults and clamp (offset passed to repo); empty page → no candidate queries; `normalizeISBN` / `isbnVariants` table (hyphens, spaces, lowercase `x`, 10→13, 978-13→10, 979 → no 10, wrong length ignored); `normalizeBookTitle`; match kinds; self excluded; any status included; `same_isbn` wins on double match; ordering (kind, published<pending<rejected, title, id); cap 5; `[]` when none; page rows hint at each other; book without authors gets no title hint.
- `admin/repository` SQLite (new `book_moderation_sqlite_test.go`, DDL for books/authors/genres/book_authors/book_genres like the existing files): status and source filters, ordering, total ignores limit/offset, authors/genres preloaded with status; `FindBooksByNormalizedISBNs` matches hyphenated/spaced stored ISBNs and excludes ids, empty input; `FindBooksByAuthorIDs` groups `AuthorIDs`, excludes ids, empty input. Search is Postgres-only and not covered (note it in the file header, like the author/genre tests).
- `admin/transport/http` (new `handler_book_moderation_test.go`): 200 response shape (book with author/genre status, `external_id`, `created_at`, `similar` with `match`); query params forwarded and limit clamped to 50; 400 on `ErrInvalidModerationFilter`; 500 otherwise; auth chain: anonymous 401, moderator 403, admin 200.

Frontend: none required beyond typecheck (types only). CI frontend build is the gate; do not run a native `next build`.

## Documentation

- This file: on completion set `Status: done` and add Implementation Notes.
- `docs/features/book-status-model.md`: in the out-of-scope line about B3 (#313), link here.
- `docs/ROADMAP.md` #310 line: add "#313 book moderation queue endpoint (done: `GET /admin/books/moderation` …, see docs/features/book-moderation-queue.md)" when shipped.
- Swagger regenerated (`backend/docs/*`).

## Risks

- **ISBN hint query has no index** (seq scan with an expression per request). Fine at current catalog size; if it shows up in profiling, add an expression index on the normalized ISBN in a later migration (not in this PR).
- **Prolific authors**: the author-overlap query can return many rows for authors with hundreds of books; the 2000-row cap keeps it bounded but may drop some `same_title_author` hints on such pages. Acceptable for display-only hints.
- **Hint gaps**: different editions with different ISBNs and slightly different titles ("Война и мир. Том 1") are missed; same-title books by the same author that are genuinely different editions are flagged. Hints are suggestions; the moderator decides.
- **Unknown `source` is 400 here but ignored in `GET /admin/books`** — intentional consistency with the other queues; F4 must only send valid values.
- **Fake churn**: three new repository methods and one handler-service method touch every admin repository/service fake (mechanical).
- **Search is Postgres-only** and not covered by SQLite tests (same as authors/genres).

## Implementation Notes

- Repository (`admin/repository/book_moderation.go`): `BookModerationFilter`, `ListBooksForModeration` (`Model(&Book{}).Preload("Authors").Preload("Genres")`, status/source/search per rules 1-2, count before limit, `books.created_at DESC, books.title ASC, books.id ASC`, defensive 20/100 clamp), `BookHintCandidate`, `FindBooksByNormalizedISBNs` (`upper(replace(replace(isbn,'-',''),' ','')) IN ?`, `id NOT IN ?`), `FindBooksByAuthorIDs` (`books b JOIN book_authors ba`, `ba.author_id IN ?`, `b.id NOT IN ?`, ordered `b.id, ba.author_id`, `LIMIT 2000`, grouped into one candidate per book with the matched `AuthorIDs`). Both hint queries return `[]` without querying on empty input. The three methods are on the `Repository` interface; every fake (admin service fakes, harvest fake) got stubs.
- Service (`admin/service/book_moderation.go`): `ListBookModerationQueue` validates status/source (wrapping `ErrInvalidModerationFilter`), clamps page/limit (20/50), returns early on an empty page (no hint queries), builds the page ISBN set (each normalized ISBN plus its 10/13 variant) and author-id set, runs the two hint queries with the page ids excluded, then merges both results and the page rows into one pool (`mergeBookCandidates`: one entry per book id, author ids unioned). Pure helpers: `normalizeISBN`, `isbnVariants` (recomputes the ISBN-13 / ISBN-10 check digit; only digit bodies get a variant), `normalizeBookTitle` (delegates to `normalizeGenreName`), `bookMatchKind`, `buildBookHints` (dedupe by id keeping the strongest match, order kind > status rank > title > id, cap 5, never nil).
- Handler: `ListBookModerationQueue` with `bookHintBookResponse`, `bookHintResponse`, `bookQueueItemResponse` (`external_id` on the item, `created_at` = `book.created_at`), `listBookQueueResponse`. Route `books.GET("/moderation", ...)` sits after `/bulk-status` and before `/:id`. `bookResponse` and `GET /admin/books(/:id)` are unchanged. Swagger regenerated (additive diff only).
- api-client: `AdminBookHintBook`, `AdminBookHint`, `AdminBookQueueItem`, `AdminBookQueueParams`, `api.admin.books.moderationQueue`; re-exported as `adminBooksApi.moderationQueue`. No admin UI change (F4, #317).
- Tests: `admin/repository/book_moderation_sqlite_test.go` (search not covered, Postgres-only), `admin/service/book_moderation_queue_test.go`, `admin/transport/http/handler_book_moderation_test.go`.
