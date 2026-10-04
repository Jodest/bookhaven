# Similar Books, Stage 1: Content-Based (Genres + Authors)

Status: implemented

GitHub issue: #213 "Похожие книги, этап 1: по жанрам и авторам". This issue also covers #198 (the missing reverse indexes on `book_authors(author_id)`, `book_genres(genre_id)` and `lists(user_id)`). Stage 2 is #217 ("readers also read" plus precomputation). This is separate from personal recommendations (#112). Branch `feat/similar-books-content` from `main`.

Stage 2 (co-reading + precompute): see [similar-books-stage2.md](similar-books-stage2.md).

## Goal

The book page shows a "Похожие книги" block: a horizontal row of book cards, chosen by shared authors and genres. Everyone sees it, including guests, and every viewer gets the same list for a given book. The endpoint is public and cheap. The ranking is computed on request and cached in Redis. No precomputation and no collaborative signal in this stage.

## Scope boundaries

**In scope:** migration `000013` (three reverse indexes from #198), `GET /api/v1/books/:id/similar`, candidate SQL + Go scoring/diversity in the catalog module, Redis cache of the ranked IDs, Swagger, api-client `books.similar`, web widget `similar-books` on the book page, tests, docs.

**Out of scope:** "readers also read" / co-shelving signal and any precomputed tables (#217), embeddings/pgvector, per-user personalization (#112), admin app, explicit cache invalidation hooks in the admin module.

## User Stories

- As a reader (guest or logged in) on a book page, I see up to 8 related books (same author or overlapping genres) and can open one with a single click.
- As a reader of a prolific author, I still get variety: no more than 2 books by any one author.
- As a reader of a book with no related books, I don't see an empty block.

## Acceptance Criteria

- [x] Migration `000013_similar_books_indexes` (up/down) adds `idx_book_authors_author_id`, `idx_book_genres_genre_id`, `idx_lists_user_id` using `CREATE INDEX IF NOT EXISTS`. Down drops them with `DROP INDEX IF EXISTS`.
- [x] `GET /api/v1/books/:id/similar?limit=N` is public and needs no auth middleware. It returns `200 {"items": [bookListItemResponse...]}`, with the same item shape as `GET /books` items.
- [x] `:id` is not a UUID: `400 {"error":"invalid book id"}` (the ratings/reviews handler convention). Unknown or hidden (`is_hidden = true`) book: `404 {"error":"book not found"}`.
- [x] `limit`: default 8. A missing, non-numeric or `<= 0` value becomes 8. A value `> 20` is clamped to 20 (silent clamp, like `ListBooks`). No 400 for `limit`.
- [x] Candidates are visible (`is_hidden = false`) books that share at least one author or genre with the source book. The source book is never included. Hidden books never appear, even from a warm cache.
- [x] Ranking follows the score formula in Business Rules. At most 2 books per author. The order is deterministic.
- [x] A source book with no authors and no genres, or with no candidates, returns `200 {"items": []}`.
- [x] Redis caches the ranked ID list per source book for 6h (key/TTL below). A Redis outage or timeout never fails the request: the request falls through to computing the list, and a failed cache write is ignored.
- [x] Swagger annotation added and `swag init` regenerated (`backend/docs`).
- [x] api-client: `api.books.similar(id, params?)` returns `{ items: Book[] }`, normalized to camelCase (`coverUrl`, `publishedYear`, `ratingsCount`, nested authors/genres). This avoids the #127/#128/#129/#131 class of bug.
- [x] Web: the "Похожие книги" block on `/books/[slug]` shows a skeleton while loading, a horizontal scroll row of the existing `BookCard` on success, and is hidden (renders nothing) on an empty result or an error.
- [x] Tests listed under Tests pass. Docker `go build/vet/test ./...` and web + api-client typecheck/lint (incl. FSD boundaries)/test are clean. Admin still compiles.

## Business Rules

**"Published"** means `books.is_hidden = false`. There is no separate status column. Harvest "pending review" is `is_hidden = true AND external_source IS NOT NULL` (see [harvest-review-queue.md](harvest-review-queue.md)), so it is excluded automatically. Both the source book and the candidates must be visible.

**Inputs per candidate** (from SQL):
- `shared_authors`: number of the source book's authors on the candidate.
- `shared_genres`: number of the source book's genres on the candidate.
- `genre_count`: the candidate's total genre count.
- `rating`: `books.rating`, a denormalized average from 0 to 5.
- `ratings_count`: `books.ratings_count`, denormalized.

Both rating fields are columns on `books`, maintained by the ratings module. No aggregation over `ratings` is needed.

**Score** (pure Go function, constants named and exported within the package so they can be tuned and tested):

```
authorMatch = 1 if shared_authors > 0 else 0            // indicator, not count
jaccard     = shared_genres / (srcGenreCount + genre_count - shared_genres)   // 0 if denominator == 0
ratingBonus = 0.3 * rating / 5                           // 0 .. 0.3
popBonus    = min(0.3, 0.1 * ln(1 + ratings_count))      // 0 .. 0.3
score       = 3*authorMatch + 2*jaccard + ratingBonus + popBonus
```

The quality bonus is at most 0.6, so it reorders books with similar content signal but never lifts a genre-only match above a same-author match (3 > 2 + 0.6). The author term is an indicator, which keeps co-authored books from being over-boosted.

**Sort:** `score DESC`, then `ratings_count DESC`, then `book_id ASC` (string), which makes the order deterministic.

**Diversity (per-author cap = 2):** go through the sorted candidates and take a book only if none of its authors already has 2 selected books. Books with no authors have no cap. The cap also counts the source book's authors in the same way: at most 2 books in the result by the source author. Stop when the selection reaches `MaxSimilarLimit = 20`.

**Response size:** the service always computes and caches the top `MaxSimilarLimit` (20) IDs. The request's `limit` slices that list. Because the greedy cap selection is prefix-stable, `limit=8` equals the first 8 of the 20-item list. This keeps one cache key per book whatever the `limit`.

**Candidate pool bound:** the SQL returns at most 40 author-matched and 100 genre-only candidates (constants `authorPool`, `genrePool`). The pools are separate so that a prolific author (e.g. 150 books) cannot fill the pool and leave only 2 results after the cap. Inside each pool the pre-order is `shared_genres DESC, ratings_count DESC, rating DESC, id`. This approximates the final ranking: Jaccard is computed in Go, and raw `shared_genres` slightly favors books with many genres. That is acceptable for stage 1.

## API

`GET /api/v1/books/{id}/similar?limit=8`. Public, catalog module.

| Case | Response |
|---|---|
| ok | `200 {"items": [ {id,title,slug,cover_url?,published_year?,rating,ratings_count,authors[],genres[]} ... ]}` |
| `id` not a UUID | `400 {"error":"invalid book id"}` |
| book missing or hidden | `404 {"error":"book not found"}` |
| DB error | `500 {"error":"failed to get similar books"}` |

Items use the existing `bookListItemResponse` and `toBookListItemResponse`. Add `type similarBooksResponse struct { Items []bookListItemResponse \`json:"items"\` }`. No `total`, `page` or `limit` fields.

Route: `api.GET("/books/:id/similar", catalogHandler.ListSimilarBooks)` in `backend/internal/transport/http/router.go`, next to the other catalog routes. `/books/:slug` and `/books/:id/...` already coexist (ratings/reviews), so Echo resolves this route.

Swagger: `@Summary Похожие книги`, `@Tags catalog`, `@Param id path string true "ID книги (UUID)"`, `@Param limit query int false "Количество, максимум 20 (по умолчанию 8)"`, `@Success 200 {object} similarBooksResponse`, `@Failure 400/404/500`, `@Router /books/{id}/similar [get]`.

## Database

Migration `backend/migrations/000013_similar_books_indexes`.

up:
```sql
CREATE INDEX IF NOT EXISTS idx_book_authors_author_id ON book_authors(author_id);
CREATE INDEX IF NOT EXISTS idx_book_genres_genre_id ON book_genres(genre_id);
CREATE INDEX IF NOT EXISTS idx_lists_user_id ON lists(user_id);
```
down:
```sql
DROP INDEX IF EXISTS idx_lists_user_id;
DROP INDEX IF EXISTS idx_book_genres_genre_id;
DROP INDEX IF EXISTS idx_book_authors_author_id;
```

I checked these against existing definitions. `book_authors` has `PRIMARY KEY (book_id, author_id)` and `book_genres` has `PRIMARY KEY (book_id, genre_id)`. Both PKs lead with `book_id`, so lookups by `author_id`/`genre_id` currently scan the table. `lists` has only its `id` PK. No later migration (000002–000012) indexes these columns, so nothing is duplicated. `lists(user_id)` is unrelated to similar books but was part of #198, so it is included here. Plain `CREATE INDEX` (not `CONCURRENTLY`) is consistent with 000007 and fine at current catalog size. golang-migrate runs each file in a transaction, and `CONCURRENTLY` would break that.

No new tables or columns.

## Backend design

### Why SQL candidates + Go scoring

Catalog repository tests are pure mocks (`repository_test.go` uses an in-memory `mockRepository`). No catalog test runs against Postgres, and the only SQLite usage in the repo is in auth tests with hand-written DDL. So:

- SQL does only what needs the database: it collects a bounded set of candidate rows with raw features. Keep it free of Postgres-only syntax (no `::` casts, `ANY`, `FILTER`, `DISTINCT ON`). Plain CTEs, `UNION ALL`, `GROUP BY` and `ROW_NUMBER()` also run on SQLite, so a SQLite repository test stays possible later.
- Go does the scoring, sorting, per-author cap and limit in a **pure function** over at most 140 rows. That function is fully unit-testable without a database. It is the actual product logic: the weights, Jaccard, the cap and the tie-breaks.
- The SQL itself is checked manually against the dev Postgres (see Tests). This is the same level of coverage as the existing `ListBooks` SQL.

### Repository (`catalog/repository`)

Add to the `Repository` interface:

```go
// SimilarSource is the visible source book's author/genre IDs.
type SimilarSource struct {
    AuthorIDs []uuid.UUID
    GenreIDs  []uuid.UUID
}

// SimilarCandidate is one raw candidate row (unscored).
type SimilarCandidate struct {
    BookID        uuid.UUID
    SharedAuthors int
    SharedGenres  int
    GenreCount    int
    Rating        float64
    RatingsCount  int
    AuthorIDs     []uuid.UUID // filled by a second query, for the per-author cap
}

GetSimilarSource(ctx context.Context, bookID uuid.UUID) (*SimilarSource, error) // gorm.ErrRecordNotFound if missing/hidden
ListSimilarCandidates(ctx context.Context, bookID uuid.UUID, src SimilarSource, authorPool, genrePool int) ([]SimilarCandidate, error)
GetBooksByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Book, error) // visible only, Preload Authors+Genres; order not guaranteed
```

`GetSimilarSource` runs `SELECT id FROM books WHERE id = ? AND is_hidden = false` (not found becomes `gorm.ErrRecordNotFound`), then `Pluck` of `author_id` from `book_authors` and `genre_id` from `book_genres` for the book.

`ListSimilarCandidates`: if both ID lists are empty, return `nil, nil` without a query. Otherwise build the `matches` CTE from only the non-empty branches. Never send an empty `IN ()`. GORM renders an empty slice as `(NULL)`, which would work but hides intent.

```sql
WITH matches AS (
        SELECT ba.book_id, 1 AS is_author, 0 AS is_genre
        FROM book_authors ba
        WHERE ba.author_id IN @author_ids          -- branch omitted if no authors
    UNION ALL
        SELECT bg.book_id, 0 AS is_author, 1 AS is_genre
        FROM book_genres bg
        WHERE bg.genre_id IN @genre_ids            -- branch omitted if no genres
),
agg AS (
    SELECT book_id,
           SUM(is_author) AS shared_authors,
           SUM(is_genre)  AS shared_genres
    FROM matches
    WHERE book_id <> @book_id
    GROUP BY book_id
),
ranked AS (
    SELECT b.id AS book_id, agg.shared_authors, agg.shared_genres,
           b.rating, b.ratings_count,
           ROW_NUMBER() OVER (
               PARTITION BY CASE WHEN agg.shared_authors > 0 THEN 1 ELSE 0 END
               ORDER BY agg.shared_genres DESC, b.ratings_count DESC, b.rating DESC, b.id
           ) AS rn
    FROM agg
    JOIN books b ON b.id = agg.book_id
    WHERE b.is_hidden = FALSE
)
SELECT r.book_id, r.shared_authors, r.shared_genres, r.rating, r.ratings_count,
       (SELECT COUNT(*) FROM book_genres g WHERE g.book_id = r.book_id) AS genre_count
FROM ranked r
WHERE (r.shared_authors > 0 AND r.rn <= @author_pool)
   OR (r.shared_authors = 0 AND r.rn <= @genre_pool);
```

Use `db.Raw(sql, map[string]interface{}{...}).Scan(&rows)` with GORM named params. The new indexes back the `matches` lookups, and the `genre_count` subquery uses the `book_genres` PK prefix. Second query: `SELECT book_id, author_id FROM book_authors WHERE book_id IN ?` over the candidate IDs, grouped into `AuthorIDs` in Go.

`GetBooksByIDs`: `Preload("Authors").Preload("Genres").Where("id IN ? AND is_hidden = ?", ids, false).Find(&books)`. Return `nil, nil` for empty `ids`.

### Service (`catalog/service`)

- New file `similar.go`:
  - `const DefaultSimilarLimit = 8; MaxSimilarLimit = 20; authorPool = 40; genrePool = 100; perAuthorCap = 2; similarCacheTTL = 6 * time.Hour`
  - Score weights as named consts.
  - Pure `func rankSimilar(srcGenreCount int, srcAuthorIDs []uuid.UUID, cands []repository.SimilarCandidate, max int) []uuid.UUID`: score, then sort, then cap, then truncate.
- Cache abstraction lives on the consumer side (service package):
  ```go
  type SimilarCache interface {
      Get(ctx context.Context, bookID uuid.UUID) (ids []uuid.UUID, ok bool, err error)
      Set(ctx context.Context, bookID uuid.UUID, ids []uuid.UUID, ttl time.Duration) error
  }
  ```
- Constructor stays source-compatible: `NewService(repo repository.Repository, opts ...Option)` with `WithSimilarCache(c SimilarCache)`. Existing callers (`cmd/api/main.go`, `service_test.go`, `handler_test.go`) compile unchanged. A nil cache means no caching.
- `ErrBookNotFound` is reused.
- `func (s *Service) ListSimilarBooks(ctx, bookID uuid.UUID, limit int) ([]domain.Book, error)`:
  1. Normalize `limit`: `<= 0` becomes 8, `> 20` becomes 20. The handler also does this, and the service is defensive.
  2. `src, err := repo.GetSimilarSource(ctx, bookID)`. `gorm.ErrRecordNotFound` becomes `ErrBookNotFound`. This runs on every request, cache hit included, so a book hidden after caching returns 404.
  3. `ids, ok, err := cache.Get(...)`. On `err != nil` or `!ok`: `cands := repo.ListSimilarCandidates(...)`, `ids = rankSimilar(...)`, then `cache.Set(...)` with the error ignored. An empty `ids` list is cached too (`[]`).
  4. `ids = ids[:min(limit, len(ids))]`. Then `books := repo.GetBooksByIDs(ctx, ids)`. Reorder to `ids` order in Go and drop any ID that wasn't returned (hidden since caching).

### Redis adapter

`catalog/repository/similar_cache.go`: `RedisSimilarCache{client *redis.Client, timeout time.Duration}`, with `NewRedisSimilarCache(client *redis.Client) *RedisSimilarCache`.
- Key: `similar:v1:<book_uuid>`. The `v1` segment lets a future scoring change (#217) bump the version instead of flushing.
- Value: JSON array of UUID strings. TTL: 6h (passed by the service).
- `redis.Nil` returns `ok=false, err=nil`. Corrupt JSON returns `ok=false` plus an error, which the service treats as a miss.
- Each op runs under `context.WithTimeout(ctx, 200*time.Millisecond)`. Without this, a down Redis would hold the request for the go-redis dial timeout (5s by default).

Wiring in `cmd/api/main.go`: reuse the existing `redisClient` (the plain go-redis client added for rate limiting in #158). Call `catalogservice.NewService(catalogRepo, catalogservice.WithSimilarCache(catalogrepository.NewRedisSimilarCache(redisClient)))`. Do not add a new client or new config.

### Cache invalidation: TTL only (recommended)

There are no explicit hooks. This is acceptable because:
- Only the **ranked ID list** is cached, never book payloads. Covers, titles, ratings and authors are re-read from the DB on every request, so admin edits and new ratings show up immediately.
- Visibility is checked live, for both the source book (404) and every item (`GetBooksByIDs` filters `is_hidden = false`). A book that gets hidden or unpublished disappears at once and never leaks.
- What stays stale for up to 6h is the ranking/membership itself: newly published books, changed genres/authors, and rating-driven reordering. These are low-impact for a "related books" block.
- Explicit invalidation would have to clear every book whose list *contains* the edited book (a reverse lookup or pattern scan) from the admin module. That is a cross-module dependency, not worth it in stage 1. #217 (precompute) replaces this anyway.

### Handler (`catalog/transport/http`)

`ListSimilarBooks`: `uuid.Parse(c.Param("id"))` (error gives 400), parse/clamp `limit`, call the service, map `ErrBookNotFound` to 404 and other errors to 500. Map items with `toBookListItemResponse` and always return a non-nil `items` slice (`[]`, not `null`).

## Frontend

### api-client (`packages/api-client/src/index.ts`)

- Rename `RawAuthorBookListItem` to `RawBookListItem` and `normalizeAuthorBookListItem` to `normalizeBookListItem`. Update the comment: the shape is the generic `bookListItemResponse`, used by author detail and similar books. Keep the author-detail usage working.
- Add to `api.books`:
  ```ts
  similar: async (id: string, params?: { limit?: number }) => {
    const qs = params?.limit !== undefined ? `?limit=${params.limit}` : '';
    const raw = await fetcher<{ items: RawBookListItem[] }>(`/api/v1/books/${id}/similar${qs}`);
    return { items: raw.items.map(normalizeBookListItem) };
  },
  ```
  This follows the `reviews.list(bookId, params?)` convention of an ID plus an optional params object.
- Do not add `/books/*/similar` to the refresh-excluded paths: it is a public GET, the same as `/books`.

### Web (`apps/web`)

- `entities/book/api/index.ts`: add `similar: api.books.similar` to `booksApi`.
- New widget `apps/web/src/widgets/similar-books/` (`index.ts`, `ui/similar-books.tsx`, `ui/similar-books.test.tsx`). It is a widget because it composes the `entities/book` card plus data fetching, with no user action of its own (FSD: views, then widgets, then entities).
  - `'use client'` and `useQuery({ queryKey: ['books', bookId, 'similar'], queryFn: () => booksApi.similar(bookId, { limit: 8 }), staleTime: 5 * 60_000 })`. React Query is already provided in `app/providers.tsx`, and `notification-bell.tsx` uses the same pattern.
  - Client-side rather than a server component: the main book content renders without waiting for a secondary block, a skeleton state is easy, and it is testable the same way as `notification-bell.test.tsx`.
  - Pending: a `<section>` with heading "Похожие книги" and about 4 skeleton cards. There is no `Skeleton` in `@bookhaven/ui`, so use inline `animate-pulse rounded-xl bg-muted` blocks with `aspect-[2/3]` and the same item width. Add `aria-busy="true"`.
  - Success with items: `<section aria-labelledby=...>` containing `<h2 className="font-display text-xl font-semibold">Похожие книги</h2>` and the row `<ul className="mt-4 flex gap-4 overflow-x-auto pb-2 snap-x snap-mandatory">`. Each item is `<li className="w-40 shrink-0 snap-start sm:w-44">` wrapping the existing `BookCard`.
  - Empty items or error: `return null`. A secondary block must never show an error.
- `BookCard`: add an optional `imageSizes?: string` prop, keeping the current value as the default. The similar row passes `"176px"` so mobile doesn't download 100vw covers. This is presentational only, so other usages are unaffected.
- `views/book-detail/ui/book-detail-view.tsx`: render `<SimilarBooks bookId={book.id} />` full-width **below** the two-column grid, inside `<main>`, with `mt-12`. It does not go in the right column, because the horizontal row needs the full width. The view stays a server component, and the widget is the client island.

## Existing Patterns

- Book list item DTO and mapper: `toBookListItemResponse` in `backend/internal/modules/catalog/transport/http/handler.go`.
- Visibility filter `is_hidden = false`: `catalog/repository/repository.go` (`ListBooks`, `GetBookBySlug`).
- Invalid UUID gives 400 `"invalid book id"`: `ratings/transport/http/handler.go` `parseBookID`.
- Redis client wiring and miniredis tests: `cmd/api/main.go` (`redisClient`) and `internal/pkg/ratelimit/ratelimit_test.go`.
- api-client normalization: `normalizeAuthorBookListItem` (to be renamed) and `api.books.list`.
- Client widget with react-query, and its test: `apps/web/src/widgets/notifications/ui/notification-bell.tsx` and `.test.tsx`.
- Card: `apps/web/src/entities/book/ui/book-card.tsx`.

## Tests

Backend (`go test ./...` via docker compose):
- `catalog/service/similar_test.go`, pure `rankSimilar`:
  - Same-author candidate outranks genre-only even with max quality bonus.
  - Jaccard ordering: 2/2 genres beats 1/3.
  - Rating/popularity bonus breaks ties between equal content signal.
  - Per-author cap: 5 same-author candidates give 2, and a co-authored book is blocked if either author is at the cap.
  - Books without authors are not capped.
  - Truncation to max.
  - Deterministic tie-break.
  - `srcGenreCount = 0` / zero denominator gives no NaN.
- `catalog/service` `ListSimilarBooks` with the fake repo and a fake `SimilarCache`:
  - Not found gives `ErrBookNotFound`.
  - Cache miss computes and calls `Set` with up to 20 IDs.
  - Cache hit skips `ListSimilarCandidates`.
  - Cache `Get` error falls through, and a `Set` error still returns results.
  - Hit containing a now-hidden ID drops it and keeps the order.
  - `limit` slicing: 8 of 20, clamp of 0 and 50.
  - Nil cache works.
  - Source with no authors/genres gives an empty result.
- `catalog/repository/similar_cache_test.go` with miniredis: round-trip, missing key gives `ok=false`, TTL set (`mr.TTL`), corrupt value gives an error, closed server gives an error (not a hang).
- `catalog/transport/http/handler_test.go`: 400 on bad UUID, 404 on unknown/hidden, 200 shape (`items`, snake_case fields, `[]` not `null` when empty), limit default/clamp passed through.
- Update the three fake repositories (`repository_test.go` `mockRepository`, `service_test.go`, `handler_test.go`) for the new interface methods.
- SQL and migration sanity (manual, recorded in the PR): `migrate up` then `\d book_authors`, `\d book_genres`, `\d lists` show the indexes. `migrate down` (one step, per #188) removes them, and `up` again works. On seeded data, `curl /api/v1/books/<id>/similar` returns plausible results with at most 2 per author, and `EXPLAIN` of the candidate query uses the new indexes.

Frontend:
- `widgets/similar-books/ui/similar-books.test.tsx` (vitest + Testing Library, `QueryClientProvider`, mocked `booksApi.similar`): skeleton while pending, heading plus N card links (`/books/<slug>`) on success, renders nothing on `[]`, renders nothing on rejection, calls with `limit: 8`.
- api-client: `similar` builds `/api/v1/books/<id>/similar?limit=8` and normalizes `cover_url` to `coverUrl` and `ratings_count` to `ratingsCount`. Add this if api-client has a test suite; otherwise the typecheck covers it.

## Documentation

- This contract: set Status to `implemented` and tick the criteria on completion.
- `docs/ROADMAP.md`: #213 and #198 are currently not listed in Proposed. Add under "Up Next" a "Similar books" item: "Similar books, stage 1 (content-based) — in progress (see [docs/features/similar-books.md](features/similar-books.md), #213, includes indexes #198)". Add stage 2 "readers also read + precompute (#217)" as the next queued item. On merge, move stage 1 to Completed.
- Swagger regenerated (`backend/docs`).

## Risks

- **Popular-genre scans:** a book in a very large genre aggregates all `book_genres` rows of that genre per cache miss. The new index makes this an index scan, but cost is O(books in genre). With the 6h cache and the current catalog size (thousands of books) this is fine. #217 precompute is the long-term fix. Measure with `EXPLAIN ANALYZE` on seeded data.
- **Redis down = latency:** without the 200ms per-op timeout, every request would wait for the go-redis dial timeout. The timeout is required, not optional.
- **Cache stampede** on a cold popular book: concurrent misses compute in parallel. The query is cheap and bounded, so this is accepted (no singleflight in stage 1).
- **Stale ranking up to 6h** (new books, genre edits). Accepted. Visibility and card data are always fresh because only IDs are cached.
- **Pool pre-order approximation:** genre-only candidates are pre-ranked by raw `shared_genres`, not Jaccard. A strong-Jaccard book with few genres could fall outside the top 100 in a huge genre. This is low impact.
- **Repository interface growth** breaks three hand-written fakes. They must be updated in the same PR.
- **api-client rename** (`RawAuthorBookListItem` to `RawBookListItem`) touches author detail. Keep behavior identical, because author covers broke before (#128).
- #198's exact body could not be fetched during planning (no `gh` access). The index list is taken from #213, which quotes it. Check #198 for any extra items before closing it. Remember that GitHub auto-closes only the first issue in "Closes #213, #198", so close #198 manually after merge.

## Implementation notes

- Backend:
  - Migration: `backend/migrations/000013_similar_books_indexes.{up,down}.sql`.
  - Repository: `GetSimilarSource`, `ListSimilarCandidates`, `GetBooksByIDs` in `catalog/repository/repository.go`.
  - Redis adapter: `catalog/repository/similar_cache.go`.
  - Service: `rankSimilar`, `SimilarCache`, `WithSimilarCache` and `ListSimilarBooks` in `catalog/service/similar.go`. `NewService` is now variadic (`opts ...Option`).
  - Handler: `ListSimilarBooks` in `catalog/transport/http/handler.go`.
  - Route: `GET /api/v1/books/:id/similar` in `internal/transport/http/router.go`.
- `GetSimilarSource` checks visibility with `Pluck("id")` and `Limit(1)` instead of `First`, so a missing book returns `gorm.ErrRecordNotFound` without GORM's record-not-found log noise.
- `rankSimilar` treats a candidate as an author match when `SharedAuthors > 0`, or when its `AuthorIDs` contain a source author. The second check is a defensive fallback, and the SQL already supplies `SharedAuthors`. The per-author cap is applied the same way to every author, including the source book's own authors. The jaccard term is only added when `SharedGenres > 0` and the denominator is positive, so a zero denominator never produces NaN.
- `cmd/api/main.go`: the `redisClient` construction moved above the service construction so that the catalog service can take `WithSimilarCache(NewRedisSimilarCache(redisClient))`. The client and its config are unchanged.
- Web: `SimilarBooks` takes an optional `className`, and the view passes `mt-12`. This way no empty spacer is left behind when the widget renders nothing. The skeleton row uses `overflow-hidden` because it is not scrollable.
- Swagger regenerated with `docker compose exec backend swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`. The only additions are the new path and `similarBooksResponse`.
- api-client has no test suite, so `books.similar` is covered by type-checking and by the mocked widget test.
- Manual verification, against the dev DB with the full seed:
  - `migrate up` applied version 13 and all three indexes exist.
  - The down SQL was checked inside `BEGIN … ROLLBACK`: it dropped all three indexes, re-running the up SQL recreated them, and after the rollback all three were present again. The shared DB was not rolled back.
  - `EXPLAIN` of the candidate query with literal ID lists uses `idx_book_authors_author_id` and `idx_book_genres_genre_id`. This holds even with the default planner on the 19-book seed.
  - `GET /api/v1/books/<brothers-karamazov id>/similar` returns 8 items with 2 Dostoevsky books first. `?limit=50` returns all 16 candidates. A bad id returns 400 and an unknown id returns 404.
  - The Redis key `similar:v1:<id>` has a TTL of about 21600s.
  - Hiding a cached book via SQL removed it from the response immediately, and un-hiding it restored it.
  - The `/books/<slug>` SSR HTML contains the pending skeleton section.
  - The API sends CORS headers for `http://localhost:3000`.
