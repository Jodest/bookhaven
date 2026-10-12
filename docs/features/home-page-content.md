# Home Page: Book of the Day, Literary Calendar and Personal Blocks

Status: implemented (PR 1 merged in #235/#236, PR 2 merged in #237/#238)

GitHub issue: #229 "Главная страница: книга дня, литературный календарь и персональные блоки". Branch `feat/home-page-content` from `main`.

## Goal

Turn the home page into a living page built only from BookHaven's own data. Today it has a hero, genre chips, and "Популярные книги", which is just the first 10 books of `GET /books`. Guests get a book of the day, a literary calendar, a "Что почитать?" roulette, new books, top-rated books and fresh reviews. Logged-in users also get "Продолжить чтение", reading-challenge progress, "Вам может понравиться" and one pick from their "Хочу прочитать" shelf.

## Scope boundaries

**In scope:**
- A new backend `home` module with three endpoints: public payload, random pick, personal payload.
- A Redis cache for the public payload.
- An optional "Хочу прочитать" mode for the existing shelve buttons.
- New web widgets and the api-client additions.
- A small dev-seed extension, tests and docs.

**Out of scope** (as in the issue):
- External new releases (ЛитРес feed etc.), RSS literary news.
- Editorial collections in the admin.
- "Популярно на этой неделе".
- Spoiler handling (#207).
- Re-rolling the want-to-read pick.
- Per-user caching.
- Any schema change.
- SEO metadata (#199).

**Delivery:** one contract, two PRs, merged in order:
- **PR 1 (public):** backend module, `GET /home`, `GET /home/random`, public widgets, the hero/genres rework.
- **PR 2 (personal):** `GET /home/me`, personal widgets, the `-demo-readers` extension.

PR 1 is shippable on its own. Each PR ticks its own criteria below.

## User Stories

- As a guest, I land on a page that changes every day: one featured book, the authors born or who died today, and books celebrating a round anniversary this year.
- As a guest who doesn't know what to read, I press "Что почитать?" (optionally choosing a genre) and get a good random book. I can press again for another one.
- As a guest, I see what is new in the catalog, what readers rate highest, and what people are writing in reviews right now.
- As a reader, I see the books I'm reading, my progress toward this year's goal, books similar to what I liked, and a reminder of one book I wanted to read.

## Acceptance Criteria

### PR 1: public

- [x] `GET /api/v1/home` (no auth) returns the payload under API. Every block follows its rule in Business Rules. Hidden books (including pending-review harvested books, which are `is_hidden = true`) and hidden reviews never appear.
- [x] The book of the day is the same all day for every visitor. The day is the Moscow date (UTC+3). It is not the same book on two consecutive days unless the pool has a single book. The fallback tiers apply when the strict pool is empty.
- [x] The calendar matches `authors.birth_date`/`death_date` by month and day of the Moscow date. In non-leap years, authors born or died on Feb 29 appear on Feb 28. With no events today, the nearest day with events in the next 7 days is shown with `is_today = false`. Otherwise `events` is empty. Anniversaries are books whose `current_year - published_year` is a positive multiple of 25.
- [x] The public payload is cached in Redis as ID lists only. Books and reviews are always read live, so a book or review hidden after caching disappears on the next request. A Redis outage only costs latency, never a 500.
- [x] A failure in one block is logged and that block comes back empty (`null`/`[]`). The response is still 200. A response with a failed block is not cached. If every block fails, the response is 500.
- [x] `GET /api/v1/home/random?genre_id=&exclude_id=` returns one random visible book from the pool, or `{"book": null}`. A malformed UUID returns 400.
- [x] The web home page is server-rendered with the public blocks. It no longer calls `booksApi.list({limit: 10})`. If `GET /home` fails, the page still renders the hero and genre chips.
- [x] Empty blocks are not rendered. There are no empty headings.
- [x] The book of the day has a "Хочу прочитать" toggle that works for logged-in users. Guests get a link to `/login?redirect=/`. (Component-tested and guest link checked in SSR HTML; logged-in browser check pending, to record in the PR.)
- [x] Dates are in Russian ("4 октября"). Year counts use correct plural forms ("1 год", "2 года", "5 лет").
- [ ] The layout works at 360px width: rows scroll horizontally, and the book of the day and the calendar stack. (Implemented with responsive classes; manual check at 360px/1280px pending, to record in the PR.)
- [x] Backend and web tests listed under Tests pass. Swagger is regenerated.

### PR 2: personal

- [x] `GET /api/v1/home/me` (`authMiddleware`) returns only the caller's data, using the user ID from the token and no ID parameter. It returns 401 without a session.
- [x] Logged-in users see these blocks, each hidden when empty: "Продолжить чтение", the challenge card, "Вам может понравиться" and "Из вашего «Хочу прочитать»". Guests see none of them, and no `/home/me` request is made for them. (Component-tested, guest SSR HTML checked and `/home/me` checked via the API with demo data; logged-in browser check pending, to record in the PR.)
- [x] The challenge card reuses `GET /api/v1/reading-challenges/{year}` (no new endpoint). It shows "Прочитано N из M в <год>" with a progress bar, or a "Поставить цель" CTA linking to `/challenge` when `goal` is null.
- [x] Recommendations never include a book on any of the user's shelves (custom shelves included) or a book the user has rated.
- [x] The tests listed under Tests pass.

## Business Rules

All rules read **visible books only** (`books.is_hidden = false`). Constants are exported from `home/service` so tests and docs refer to the names:

| Constant | Value | Meaning |
|---|---|---|
| `GoodRatingMin` | 4.0 | "well-rated" threshold |
| `GoodRatingsCountMin` | 3 | minimum ratings for "well-rated" |
| `RowLimit` | 10 | books in the "new" and "top-rated" rows |
| `CalendarMaxAuthors` | 6 | |
| `CalendarBooksPerAuthor` | 3 | |
| `CalendarLookaheadDays` | 7 | |
| `AnniversaryStep` | 25 | |
| `AnniversaryLimit` | 6 | |
| `RecentReviewsLimit` | 3 | |
| `ReviewExcerptRunes` | 280 | |
| `DescriptionExcerptRunes` | 300 | |
| `ContinueReadingLimit` | 6 | |
| `RecommendationsLimit` | 10 | |
| `RecommendationSeeds` | 50 | |
| `HomeCacheTTL` | 10 min | |

### Day and timezone

"Today" is the civil date in Moscow, `time.Now().In(moscow)` with `moscow = time.FixedZone("MSK", 3*60*60)`. Moscow has had no DST since 2014. A fixed zone avoids a tzdata dependency in the container. The date is injected into the service as `now func() time.Time` so it can be tested.

### Book of the day

- **Pool, strict:** visible, `cover_url IS NOT NULL`, `rating >= GoodRatingMin`, `ratings_count >= GoodRatingsCountMin`.
- **Fallback tiers:** each tier is used only when the previous one is empty.
  1. Visible with a cover and `ratings_count >= 1`.
  2. Visible with a cover.
  3. Any visible book.
  4. Nothing: `book_of_the_day = null`.
- **Pick:** a pure function `pickDaily(pool []uuid.UUID, day civil date, salt string) uuid.UUID`.
  - Each ID gets the key `fnv64a(salt + "|" + YYYY-MM-DD + "|" + id)`. The ID with the smallest key wins.
  - If the winner equals `pickDaily(pool, day-1, salt)` and `len(pool) > 1`, the second-smallest wins instead. This means no repeat on consecutive days against the same pool.
  - Salt is `"botd"`.
  - The pool is loaded as IDs only, ordered by `id` (the order is irrelevant to the result, but keeps the input deterministic).
- **Payload:** the book list item plus `description_excerpt`. The excerpt is `excerpt(description, DescriptionExcerptRunes)`: cut at the last whitespace before the limit, trailing punctuation trimmed, `…` appended only when the text was cut. It is null when there is no description.

### Literary calendar

- **Match days:** a pure function `calendarDays(d civil date) []monthDay` returns `[(m, d)]`. In a non-leap year on Feb 28 it returns `[(2,28), (2,29)]`. Feb 29 itself only exists in leap years and returns `[(2,29)]`.
- **Events:** the query covers authors with `EXTRACT(MONTH FROM birth_date)` and `EXTRACT(DAY ...)` in the match days (event `birth`), and the same for `death_date` (event `death`).
  - Only authors with at least one visible book are included.
  - An author whose birth and death both match yields two events.
  - `years` = event year minus the date's year, for example "212 лет со дня рождения". `years <= 0` is dropped. A future birth date would be bad data.
- **Order:** "round" events first (`years % 25 == 0`), then visible-book count DESC, then name ASC. Truncate to `CalendarMaxAuthors` events.
- **Books per author:** top `CalendarBooksPerAuthor` visible books by `rating DESC, ratings_count DESC, title ASC`.
- **Empty-day fallback:** if today has no events, try `d+1 … d+CalendarLookaheadDays` in order and use the first day that has events. Return it with `is_today = false` and its `date`. The web heading becomes "Скоро в календаре · 15 октября". If no day in the window has events, `events = []`.
  - Recommended over hiding the calendar: harvested authors are sparse, so empty days will be common.
  - Recommended over "nearest ever": a date far in the future looks broken.
  - At most 8 cheap author queries in the worst case. The result is cached.
- **Anniversaries (year-long, not daily):** `published_year` has no day. Visible books with `age = current_year - published_year`, `age > 0` and `age % AnniversaryStep == 0`. Order: `age % 100 == 0` first, then `age % 50 == 0`, then `rating DESC, ratings_count DESC, title ASC`. Limit `AnniversaryLimit`. Each item carries `years`.
- **Block visibility:** `calendar` is `null` when both `events` and `anniversaries` are empty.

### "Что почитать?" (random)

- **Pool:** the strict "well-rated" pool. It ignores the cover requirement but requires `ratings_count >= GoodRatingsCountMin` and `rating >= GoodRatingMin`. It is restricted to `genre_id` when given. `exclude_id` (the book currently shown) is excluded.
- **Fallback:** if empty, visible books with `ratings_count >= 1` (same genre and exclude filters). Then any visible book in the genre. Then `null`.
- **Query:** `ORDER BY random() LIMIT 1`. This is fine at the current catalog size. Scaling path: count, then a random `OFFSET`.
- **Caching:** none. Each press is a new pick, so consecutive presses may repeat only through `exclude_id`'s limited memory. That is accepted.
- **Validation:** an unknown `genre_id` gives `null`, not 404.

### New in catalog

Visible, `ORDER BY published_at DESC NULLS LAST, id DESC LIMIT RowLimit` (#245, OpenSpec change `track-book-publish-time`). `books.published_at` is the first time a book became `published`: set once on any path (manual create, import, single or bulk status change, seed) and never cleared, so unpublishing and republishing does not bump a book. A harvested book published from the review queue now ranks by its publish time, not its harvest time. Books published before migration `000020` were backfilled with `created_at`. `NULLS LAST` keeps raw-SQL fixtures without a time at the end.

### Top rated

Visible, `rating >= GoodRatingMin AND ratings_count >= GoodRatingsCountMin`, `ORDER BY rating DESC, ratings_count DESC, id ASC LIMIT RowLimit`. There is no fallback: an empty result hides the block.

### Fresh reviews

- **Filter:** `reviews.is_hidden = false` on visible books, `char_length(content) >= 40` (skips "ок, норм").
- **Selection:** at most one review per book: the newest review per book (`DISTINCT ON (book_id) ... ORDER BY book_id, created_at DESC`), then the newest `RecentReviewsLimit` of those.
- **Payload:** `excerpt(content, ReviewExcerptRunes)`, rating, `likes_count`, `created_at`, `user {id, name, avatar_url}` (null name renders as "Аноним", as in `ReviewCard`), and `book {id, slug, title, cover_url, authors[name, slug]}`. The full content is never sent.
- **Spoilers:** not handled because #207 is not implemented. Mitigated by the short excerpt plus a link to the book page; see Risks.

### Continue reading (personal)

Books on the caller's `reading` system shelf, visible, `ORDER BY shelf_items.added_at DESC LIMIT ContinueReadingLimit`. Also returns `shelf_id` and `total` (visible count) for an "Все (N)" link to `/shelves/{shelf_id}`.

### Challenge progress (personal)

Reuse `GET /api/v1/reading-challenges/{year}` with `year = new Date().getFullYear()`. The backend counts `read`-shelf additions by UTC year; the New Year's Eve hour mismatch is accepted. No backend change.

### You may like (personal)

- **Seeds:** the caller's positive books, defined as in [similar-books-stage2.md](similar-books-stage2.md#interactions) but for this user only.
  - Included: books on a `read`/`reading` shelf, or rated `>= 4`.
  - Excluded: books rated `<= 2`, and hidden books.
  - Up to `RecommendationSeeds`, ordered by latest activity (`added_at` / `ratings.updated_at`).
- **Exclusions:** every book on any of the caller's shelves (custom shelves included), every book the caller rated, and the seeds themselves.
- **Tier 1 (precomputed):**
  - Query: `SELECT similar_book_id, SUM(score) AS s, COUNT(*) AS c FROM book_similarities WHERE book_id IN seeds GROUP BY 1`.
  - Apply the exclusions and visibility.
  - Order: `s DESC, c DESC, id ASC`, limit `RecommendationsLimit`.
- **Tier 2:** runs only if tier 1 returns fewer than 6.
  - For the 3 most recent seeds, call catalog `Service.ListSimilarBooks(ctx, seed, 20)`. That call is precomputed, then the stage-1 Redis cache, then live, so new or niche books still get content-based neighbours.
  - Merge round-robin by seed recency, dedupe against tier 1 and the exclusions, and fill up to the limit.
  - A `ErrBookNotFound` from a seed hidden mid-request is skipped.
- **No seeds:** the block is hidden (`[]`).
  - Padding with top-rated was rejected because it would duplicate the public "Высоко оценённые" row on the same page.
- **Ordering:** a pure function `mergeRecommendations(tier1 []uuid.UUID, perSeed [][]uuid.UUID, exclude set, limit int) []uuid.UUID` handles the merge and is tested.

### From your want-to-read (personal)

- One visible book from the caller's `want_to_read` shelf, chosen by `pickDaily(shelfBookIDs, today, "wtr:" + userID)`. It is stable across reloads within a day and changes daily.
- No re-roll in this scope.
- Returns `null` when the shelf has no visible books, plus `shelf_id` and `total` for a "Вся полка" link.

## API

All responses are snake_case. Books use the existing `bookListItemResponse` shape (id, title, slug, cover_url, published_year, rating, ratings_count, authors, genres), exported from the catalog transport (see Backend).

### `GET /api/v1/home` — public, no auth

```json
{
  "date": "2026-10-04",
  "book_of_the_day": { "book": { /* bookListItem */ }, "description_excerpt": "…" } ,
  "calendar": {
    "date": "2026-10-15",
    "is_today": false,
    "events": [
      { "type": "birth", "years": 212,
        "author": { "id": "…", "name": "Михаил Лермонтов", "slug": "mikhail-lermontov", "photo_url": "…", "birth_date": "1814-10-15", "death_date": "1841-07-27" },
        "books": [ /* bookListItem, max 3 */ ] }
    ],
    "anniversaries": [ { "years": 100, "book": { /* bookListItem */ } } ]
  },
  "new_books": [ /* bookListItem */ ],
  "top_rated": [ /* bookListItem */ ],
  "recent_reviews": [
    { "id": "…", "excerpt": "…", "rating": 5, "likes_count": 2, "created_at": "…",
      "user": { "id": "…", "name": "Анна", "avatar_url": null },
      "book": { "id": "…", "slug": "…", "title": "…", "cover_url": "…", "authors": [ { "name": "…", "slug": "…" } ] } }
  ]
}
```

- `book_of_the_day` and `calendar` are nullable. The arrays are never null.
- Status codes: 200, or 500 only when every block failed.

### `GET /api/v1/home/random?genre_id=<uuid>&exclude_id=<uuid>` — public, no auth

- Response: `{ "book": { /* bookListItem */ } | null }`.
- 400 on a malformed UUID.
- It lives under `/home`, not `/books/random`. `/books/:slug` would shadow a book whose slug is `random`, or be shadowed by it, because Echo's static route wins.

### `GET /api/v1/home/me` — `authMiddleware`

```json
{
  "continue_reading": { "shelf_id": "…", "total": 2, "books": [ /* bookListItem */ ] },
  "recommendations": [ /* bookListItem */ ],
  "want_to_read_pick": { "shelf_id": "…", "total": 14, "book": { /* bookListItem */ } | null }
}
```

- `continue_reading` and `want_to_read_pick` are `null` if the system shelf does not exist yet. Shelves are materialized lazily; do not create them here.
- An unverified or expired session gets 401 from the existing middleware.
- The challenge is **not** part of this response. Use the existing endpoint.

**Why aggregated endpoints:**
- One request per audience gives one SSR fetch and one cache entry keyed by date.
- Blocks share the "today" value and the hydration query (all book IDs are hydrated in one `GetBooksByIDs`).
- Partial failure is handled per block inside the handler, so the page doesn't need N error states.
- Random is separate because it is user-triggered and uncached.
- Personal is separate because it needs auth and must never pass through the public cache.
- Existing endpoints are reused where they already fit, which is the challenge. "Continue reading" is folded into `/home/me` rather than reusing `GET /shelves` + `GET /shelves/:id`, because that would be a two-request waterfall, and `/home/me` already reads the user's shelves for the exclusions.

Add swag annotations to the three handlers, the same as in the catalog handler, and regenerate `backend/docs`.

## Database

No migration in the original feature. Migration `000020_books_published_at` (#245) later added `books.published_at` and the partial index `idx_books_published_at (published_at DESC NULLS LAST, id DESC) WHERE status = 'published'` for "Новое в каталоге". Existing indexes cover the other queries: `idx_books_created_at`, `idx_books_rating`, `idx_books_is_hidden`, `idx_reviews_is_hidden`, `idx_shelves_user_id`, the `book_similarities` PK prefix. The calendar's `EXTRACT` on `authors` and the reviews `DISTINCT ON` are sequential scans of small tables, and the payload is cached.

Scaling path, not built now:
- an expression index on `(EXTRACT(MONTH FROM birth_date), EXTRACT(DAY FROM birth_date))`, and the same for `death_date`;
- a partial index `reviews(created_at DESC) WHERE is_hidden = false`.

## Backend

### Module placement: new `backend/internal/modules/home`

`service/`, `repository/`, `transport/http/`. There is no `domain/` package because the module owns no tables; small value types live in `service`.

It is a read-only composition module. The `home` repository reads `books`, `authors`, `book_authors`, `reviews`, `users`, `shelves`, `shelf_items`, `ratings` and `book_similarities` in raw SQL (`db.Raw(...).Scan`) and returns **IDs and scalars only**. This follows the read-only-aggregate precedent of `catalog/repository.ListInteractions`. Add a comment listing the tables read, so schema changes in those modules find it.

Putting it in `catalog` was rejected. Catalog would gain review and shelf knowledge, and its `Repository` interface would grow again; it has three hand-written fakes.

**Cross-module dependencies.** These go through small interfaces declared in `home/service`, the same as `usershttp.NewHandler` depending on other modules' services:

```go
type BookLoader interface { // satisfied by catalog repository.Repository
    GetBooksByIDs(ctx context.Context, ids []uuid.UUID) ([]catalogdomain.Book, error)
}
type SimilarLister interface { // satisfied by *catalogservice.Service
    ListSimilarBooks(ctx context.Context, bookID uuid.UUID, limit int) ([]catalogdomain.Book, error)
}
type Cache interface { // fail-fast; errors = miss / ignored
    Get(ctx context.Context, key string) ([]byte, bool, error)
    Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
}
```

Hydration: collect every book ID of the public payload, call `GetBooksByIDs` once, and reorder per block. Drop IDs that are no longer visible, the same as `ListSimilarBooks`. Calendar authors come back from the repository as full rows; there are few fields and they are not sensitive.

### Repository: `home/repository`

```go
type Repository interface {
    ListBookOfDayPool(ctx, tier int) ([]uuid.UUID, error)
    ListCalendarEvents(ctx, days []MonthDay) ([]CalendarEventRow, error)                 // author fields + type + event date + visible_books count
    ListTopBooksByAuthors(ctx, authorIDs []uuid.UUID, perAuthor int) (map[uuid.UUID][]uuid.UUID, error) // ROW_NUMBER() OVER (PARTITION BY author_id ...)
    ListAnniversaryBooks(ctx, year, step, limit int) ([]AnniversaryRow, error)          // (current_year - published_year) % step = 0 AND published_year < year
    ListNewBookIDs(ctx, limit int) ([]uuid.UUID, error)
    ListTopRatedBookIDs(ctx, minRating float64, minCount, limit int) ([]uuid.UUID, error)
    ListRecentReviewIDs(ctx, minLen, limit int) ([]uuid.UUID, error)
    GetReviewsByIDs(ctx, ids []uuid.UUID) ([]ReviewRow, error)                          // live, is_hidden=false, visible book, joins user + book + authors
    PickRandomBook(ctx, f RandomFilter) (*uuid.UUID, error)                             // applies the tier chain in one call or per tier
    // personal (PR 2)
    GetSystemShelf(ctx, userID uuid.UUID, shelfType string) (*ShelfRef, error)          // id + visible total; nil if not materialized
    ListShelfBookIDs(ctx, shelfID uuid.UUID, limit int) ([]uuid.UUID, error)            // added_at DESC, visible; limit 0 = all
    ListRecommendationSeeds(ctx, userID uuid.UUID, limit int) ([]uuid.UUID, error)
    ListUserExcludedBookIDs(ctx, userID uuid.UUID) (map[uuid.UUID]struct{}, error)      // any shelf + any rating
    AggregatePrecomputedSimilar(ctx, seeds []uuid.UUID, limit int) ([]ScoredID, error)
}
```

The implementer may merge or split methods. The contract is IDs-in, IDs-out, with the visibility filter in SQL.

### Service: `home/service`

**Pure functions** (in `rules.go`, all unit-tested):
- `pickDaily`
- `calendarDays`
- `isLeap`
- `eventYears`
- `orderCalendarEvents`
- `isAnniversary(age, step)`
- `anniversaryRank`
- `excerpt`
- `mergeRecommendations`

**`GetHome(ctx)`:**
1. Compute `today` (MSK).
2. Cache key `home:v1:<YYYY-MM-DD>`. The value is JSON of the **ID-level** selection: book-of-day ID plus excerpt, the calendar structure with author rows and book IDs, anniversary `{years, id}`, the new/top ID lists, and review IDs.
3. On a miss, compute each block independently. Collect errors per block and log them with zap and the block name.
4. Cache only when no block failed. A failed cache read or write never fails the request; use a 200ms per-op timeout, like `RedisSimilarCache`.
5. Hydrate live: one `GetBooksByIDs` and one `GetReviewsByIDs`.
6. If every block errored, return an error, which becomes a 500.

**`RandomBook(ctx, genreID, excludeID *uuid.UUID)`:** applies the tier chain, then hydrates the one book.

**`GetPersonalHome(ctx, userID)`:**
- No cache.
- The blocks are computed independently with the same per-block error policy.
- Every block failing gives a 500.

Constructor: `NewService(repo, books BookLoader, opts ...Option)` with `WithCache`, `WithSimilarLister`, `WithLogger` and `WithClock`.

### Transport: `home/transport/http`

`Handler` has `GetHome`, `RandomBook` and `GetMyHome`. Route registration goes in `internal/transport/http/router.go`:

```go
api.GET("/home", homeHandler.GetHome)
api.GET("/home/random", homeHandler.RandomBook)
api.GET("/home/me", homeHandler.GetMyHome, authMiddleware)
```

Add a `homeService` parameter to `RegisterRoutes`. User ID comes from the context, the same way as in the shelves handler.

**Book mapping:** export from `catalog/transport/http`:
- `BookListItemResponse` and `ToBookListItemResponse`, renamed from the unexported versions;
- `AuthorResponse` and `GenreResponse`, if needed for the rename to compile.

The catalog JSON stays the same. The home transport imports them, so the shape cannot drift. The calendar author uses its own type with `death_date`.

### Wiring: `cmd/api/main.go`

```go
homeService := homeservice.NewService(homerepository.NewGormRepository(db), catalogRepo,
    homeservice.WithCache(homerepository.NewRedisCache(redisClient)),
    homeservice.WithSimilarLister(catalogService),
    homeservice.WithLogger(logger))
```

## Frontend (apps/web, FSD)

### Rendering model

**Public blocks are server-rendered; personal blocks are client-side.**
- `api-client`'s `fetcher` on the server does not forward the browser's cookies. No code uses `next/headers` `cookies()`, and middleware only refreshes tokens ([middleware-session-persistence.md](middleware-session-persistence.md)). Server components therefore cannot make authenticated calls, and introducing cookie forwarding is out of scope.
- Personal widgets use `useAuthStore` and React Query, the same as `ShelveBookButtons` and `SimilarBooks`, and get the silent-refresh behavior of `fetcher` for free.
- Public blocks being server-rendered gives SEO and fast first paint, and keeps `force-dynamic` on `app/page.tsx`.

### api-client (`packages/api-client/src/index.ts`)

- Types `HomePayload`, `HomeCalendar`, `HomeCalendarEvent`, `HomeAnniversary`, `HomeReview`, `PersonalHome`. All are camelCase: `bookOfTheDay.descriptionExcerpt`, `isToday`, `photoUrl`, `birthDate`, `deathDate`, `likesCount`, `createdAt`, `avatarUrl`, `coverUrl`, `shelfId`, `continueReading`, `wantToReadPick`, and so on.
- `Raw*` counterparts, and normalizers that use the existing `normalizeBookListItem` for every book.
- **Normalize everything**, nested review user/book included. #127/#128 (past bugs #129/#131) came from returning snake_case data typed as camelCase.
- `api.home = { get(), random({ genreId?, excludeId? }), me() }`.

### Slices

| Slice | Type | Content |
|---|---|---|
| `entities/home` | api | `homeApi = { get, random, me }` re-exported from api-client (same pattern as `entities/book/api`) |
| `entities/book/ui/book-row.tsx` | ui | `BookRow({ title, books, href?, linkLabel? })`: horizontal snap-scroll `BookCard` row, the same markup as `SimilarBooks`. Returns `null` for an empty list. `SimilarBooks` is not refactored. |
| `shared/lib/format-date-ru.ts` | lib | `formatDayMonth('2026-10-04')` gives "4 октября", via `Intl.DateTimeFormat('ru-RU', {day:'numeric', month:'long', timeZone:'UTC'})` on `YYYY-MM-DDT00:00:00Z`, so server and client agree. `pluralRu(n, ['год','года','лет'])`. `formatYears(n)` gives "212 лет". |
| `widgets/book-of-the-day` | server ui | Large cover, title, authors, rating with count, excerpt, "Подробнее" link, `ShelveBookButtons` in want-to-read-only mode |
| `widgets/literary-calendar` | server ui | Heading "Литературный календарь · 4 октября", or "Скоро в календаре · 15 октября" when `!isToday`. Events read "Родился 212 лет назад" / "Умер …", with author avatar, name linking to `/authors/{slug}`, and up to 3 small book links. A "Юбилеи года" sub-list reads "«Мастер и Маргарита» — 100 лет". |
| `features/pick-random-book` | client | "Что почитать?" card: genre `<select>` (genres passed from SSR; "Любой жанр" default) and a "Подобрать" button. The result shows as a compact horizontal card with an "Ещё вариант" button that passes `excludeId`. `useMutation`. Shows a "Не нашли подходящую книгу" message on `null`, and an inline error on failure. |
| `widgets/recent-reviews` | server ui | Up to 3 cards: book cover thumbnail plus title link, stars, a quoted excerpt with a "Читать полностью" link to `/books/{slug}`, avatar plus name, and date. Read-only; `ReviewCard` is not reused because it carries owner/like actions. |
| `widgets/personal-home` | client | Renders `null` unless `isAuthenticated`. Renders nothing while `isLoading`, to avoid a guest-to-user flash. `useQuery(['home','me'])` with `staleTime` 60s and `refetchOnWindowFocus: false`, plus `useQuery(['reading-challenges', year])` in parallel. Sub-blocks: `ContinueReading` (`BookRow`, link "Все (N)"), `ChallengeProgress` (card with bar, same markup as `manage-reading-challenge-goal`, or CTA), `Recommendations` (`BookRow` "Вам может понравиться"), `WantToReadPick` (single card "Из вашего «Хочу прочитать»" with a "Вся полка (N)" link). Skeletons as in `SimilarBooks` while pending. On error the block is hidden. |

`views/home/ui/home-view.tsx` stays an async server component:
- It runs `Promise.all([homeApi.get().catch(() => null), genresApi.list().catch(() => ({ items: [] }))])`.
- It composes the widgets. Remove "Популярные книги" and `booksApi.list`.

### `ShelveBookButtons` (additive props, defaults keep the book page unchanged)

- `types?: ('want_to_read'|'reading'|'read')[]`, default all three.
- `showCustomShelves?: boolean`, default `true`.
- `loginRedirect?: string`, default `/books/{slug}`.

The book of the day uses `types={['want_to_read']}`, `showCustomShelves={false}` and `loginRedirect="/"`.

### Layout (top to bottom)

1. **Hero:** keep it, compacted (`py-8 md:py-10`), with the same title, subtitle, CTAs and genre chips. Recommendation: keep, because it is the only brand and search entry on the page. Remove only the old "Популярные книги" section.
2. **Personal section** (logged-in only, client): `ContinueReading` at full width. Then a two-column row at `md`+ with `ChallengeProgress` and `WantToReadPick`, stacked on mobile.
3. **Book of the day plus calendar:** `lg:grid-cols-[3fr_2fr]`, stacked below `lg`. The cover is about 200px on mobile and 260px on desktop.
4. Recommendations (logged-in only).
5. "Что почитать?"
6. "Новое в каталоге", with a link "Весь каталог" to `/books?sort=new`. Note that `sort=new` orders by `published_year`; the link is a browse aid, not the same list.
7. "Высоко оценённые", with a link to `/books?sort=rating`.
8. "Свежие рецензии": 1 column on mobile, 3 at `lg`.

Empty or `null` blocks are not rendered. If `home` is `null`, only the hero, genres and personal section render.

### Copy

All copy is in Russian, as quoted above. Rating is shown as "4,6 · 12 оценок". Use `toFixed(1)` with a comma via `toLocaleString('ru-RU')`, and plural forms "оценка/оценки/оценок".

## Existing Patterns

- Redis fail-fast cache with a 200ms timeout and a versioned key prefix: `backend/internal/modules/catalog/repository/similar_cache.go`.
- Option-based `NewService`, live visibility re-read after cached IDs, and hydration plus reorder: `catalog/service/similar.go` (`ListSimilarBooks`).
- Read-only cross-table raw SQL aggregate: `catalog/repository/similarity.go` (`ListInteractions`).
- Route registration and `authMiddleware`: `backend/internal/transport/http/router.go`. Cross-module service dependencies in a handler: `usershttp.NewHandler`.
- Client widget with skeleton and "render nothing on empty/error": `apps/web/src/widgets/similar-books/ui/similar-books.tsx` and its test.
- Auth-aware client button with a guest login link: `apps/web/src/features/shelve-book/ui/shelve-book-buttons.tsx`.
- Challenge progress bar markup: `apps/web/src/features/manage-reading-challenge/ui/manage-reading-challenge-goal.tsx`. API: `entities/reading-challenge`.
- api-client normalization: `normalizeBookListItem`, `normalizeReadingChallenge`.

## Tests

Backend: `go test ./...` via docker compose. The tests are mock-based with no Postgres, the same as catalog.

**`home/service/rules_test.go` (pure):**
- `pickDaily`:
  - Deterministic for the same inputs.
  - Different for different salts in general.
  - Never equal to yesterday's pick when the pool has 2 or more books, checked across 60 consecutive days on pools of size 2, 3 and 50.
  - Single-element pool returns that element.
  - Empty pool returns `uuid.Nil`/false.
- `calendarDays`:
  - Normal day.
  - Feb 28 in a non-leap year (2027) includes Feb 29.
  - Feb 28 in a leap year (2028) does not.
  - Feb 29 in 2028.
  - Mar 1 does not include Feb 29.
- Today in MSK versus UTC: `2026-10-03T22:30Z` is Oct 4 in MSK.
- `eventYears`, plus `years <= 0` dropped.
- `orderCalendarEvents`: round anniversaries first, then book count, then name.
- `isAnniversary`/`anniversaryRank`: 25, 50, 75, 100 and 150 match. 0, 10 and negative do not. 100 ranks above 50, which ranks above 25.
- `excerpt`:
  - Shorter than the limit is returned unchanged with no ellipsis.
  - Cut at a word boundary.
  - Cyrillic counted in runes, not bytes.
  - A single long word is hard-cut.
  - Trailing punctuation trimmed before `…`.
- `mergeRecommendations`:
  - Tier-1 order kept.
  - Round-robin fill.
  - Exclusions and duplicates removed.
  - Limit honored.
  - Empty in, empty out.

**Service with fake repo, loader and cache:**
- Fallback tiers for the book of the day.
- Calendar lookahead: today empty and day+3 has events gives `is_today=false` with that date. All 8 days empty and no anniversaries gives `calendar=nil`.
- A cache hit skips the selection queries but still hydrates. A hidden book in cached IDs is dropped.
- A cache error behaves like a miss.
- One block's error gives the others intact, that block empty, and no cache `Set`.
- All blocks erroring gives an error.
- Random: genre/exclude are passed through, the tier chain is applied, and nothing found gives nil.
- Personal:
  - No seeds gives no recommendations.
  - Tier 2 is called only when tier 1 returns fewer than 6.
  - `ErrBookNotFound` from the similar lister is skipped.
  - Excluded IDs never appear.
  - A missing shelf gives a nil block.
  - `want_to_read` pick is stable for the same day and user.

**`home/transport/http/handler_test.go`:**
- 200 shapes are snake_case, with `null` for empty `book_of_the_day`/`calendar` and `[]` for arrays.
- `/home/random` returns 400 on a bad UUID.
- `/home/me` returns 401 without a user in context, and passes the context user ID to the service (no ID from the query).

**catalog:** the existing handler tests pass after the export rename.

Web (vitest + testing-library, alongside `similar-books.test.tsx`):
- `packages/api-client` (or `entities/home/api/index.test.ts`, following `entities/shelf/api/index.test.ts`): `home.get`, `random` and `me` normalize every nested field to camelCase, including review user/book and calendar author dates.
- `shared/lib/format-date-ru.test.ts`: "4 октября", "1 января" and "29 февраля", independent of the host timezone. `pluralRu` for 1, 2, 5, 11, 21, 22 and 25.
- `widgets/literary-calendar`: the today versus "Скоро" heading. Birth and death wording. The anniversaries list. Renders nothing for `null`.
- `widgets/book-of-the-day`: excerpt and rating render; renders nothing for `null`.
- `features/pick-random-book`: a click calls `random` with the genre. "Ещё вариант" passes `excludeId`. The `null` message. The error message.
- `widgets/personal-home`: a guest renders nothing and makes no API calls. A logged-in user gets skeletons, then blocks. Empty blocks are hidden. A null challenge goal shows the CTA.
- `features/shelve-book`: the `types`/`showCustomShelves`/`loginRedirect` props. The defaults are unchanged.

Manual (record in the PR):
- `curl /api/v1/home` twice: the second call hits the `home:v1:<date>` key.
- Hiding the book of the day in the admin removes it on the next request.
- Logged-in versus guest page in the browser at 360px and 1280px.

## Dev data

- Seed authors have `birth_date` and no `death_date`. On 2026-10-04 no seed author matches today. The lookahead finds nothing either: Lermontov (Oct 15) is 11 days out, and Dostoevsky/Turgenev are in November. No seed book has a 25-multiple age in 2026. So the calendar block will be empty locally on most days.
- Cheap fix: in `cmd/seed`, add `DeathDate` to the seed authors (real dates, for example Tolstoy 1910-11-20, Chekhov 1904-07-15, Pushkin 1837-02-10).
- Note that the seed uses `FirstOrCreate` by slug, so an existing dev DB won't get them. Document an `UPDATE` one-liner in the PR, or let the implementer set them in the admin.
- To see the calendar on a given day, verify via the unit tests. Temporarily editing an author's dates in the admin is fine.
- Do not add a public `?date=` override, since it would multiply cache keys.
- Top-rated, book-of-the-day and fresh reviews need ratings and reviews. Extend `cmd/seed -demo-readers` (PR 2, still behind the production guard):
  - Ratings of 4–5 so that at least 6 books reach `ratings_count >= 3`.
  - 3–4 reviews of 40 characters or more.
  - Recalculate `books.rating`/`ratings_count`/`reviews_count` as the existing demo-readers code does.
  - Until then, the fallback tiers keep the book of the day non-empty, and the top-rated block is simply hidden.
- **Do not run `migrate down`** or any destructive DB command on the dev database; this feature has no migration.

## Documentation

- This contract: set Status to `in progress` when implementation starts, then `implemented` with ticked criteria and implementation notes.
- `docs/ROADMAP.md`: add under "Up Next" a "Home page content (#229)" item marked in progress (see this contract), with the PR 1/PR 2 split. Move it to Completed on merge.
- Swagger regenerated for the three endpoints.
- No DEPLOYMENT changes. The new Redis keys are `home:v1:*` with a 10-minute TTL, which is self-expiring.

## Risks

- **Spoilers in fresh reviews (#207 not implemented):** a review quote on the most-visited page can spoil a book. Mitigation: a 280-rune excerpt, the minimum length rule, and moderation via the existing review hiding (which takes effect immediately because review rows are hydrated live). Revisit when #207 lands: exclude spoiler-marked reviews.
- **Sparse data:** with few ratings, top-rated and the strict book-of-the-day pool are empty. The fallback tiers and empty-block hiding keep the page sane, but the "well-rated" promise is weak until the user base grows. The thresholds are constants, so tune them later.
- **Julian/Gregorian dates:** birth dates of 18th–19th century Russian authors from harvest sources may be stored in either calendar, so a calendar event can be off by 11–13 days. This is a data-quality issue and is accepted. Admins can correct dates.
- **`created_at` for harvested books (resolved by #245):** books published from the review queue used to show up in "Новое в каталоге" by harvest time. The row now orders by `published_at`. Books published from the queue before migration `000020` keep their harvest-time position (backfill = `created_at`, no status history).
- **Up to 10 minutes of staleness** for which books are selected, for new books or rating changes. Visibility is always live, so this is accepted.
- **Cross-module raw SQL** in the `home` repository over other modules' tables: schema changes in shelves, ratings, reviews or similarities must update it. Mitigated by the comment listing the tables read, and by tests on the service contract.
- **Tier-2 recommendation cost:** up to 3 `ListSimilarBooks` calls per `/home/me` request when precomputed data is sparse. Stage 1 is Redis-cached for 6h, so this is cheap in steady state. The personal response is not cached.
- **Auth flash:** the personal section appears after `fetchUser` resolves. Rendering nothing while loading avoids flashing guest content, but it can shift layout. It sits right below the hero, which is accepted.
- **Catalog transport export rename:** this is a mechanical rename and must not change the catalog JSON. The existing catalog handler tests guard it.

## Implementation notes

### PR 1 (public) — #231, #232

- **`pickDaily` no-repeat rule.** The contract's "second-smallest if equal to `pickDaily(day-1)`" is recursive without a base case (yesterday's pick depends on the day before, and so on), and comparing only against yesterday's *raw* smallest key does repeat (e.g. a 2-book pool). Implemented a stateless variant that keeps "smallest key wins" and guarantees no repeat on consecutive days against the same pool: even days (days since 1970-01-01) take the smallest key; odd days take the smallest key excluding the picks of both neighbouring even days; a two-element pool alternates by day parity in a salted order. Key, salt (`"botd"`) and the id-ordered input are as specified; the input order does not affect the result. Tested over 60 days for pools of 2, 3 and 50.
- **Cache contents.** `home:v1:<date>` holds the ID-level selection (book-of-the-day ID, calendar date/is_today/author rows/book IDs, anniversary `{years, id}`, new/top ID lists, review IDs). The description excerpt is computed from the live hydrated book instead of being cached, so it can never be stale.
- **Review hydration.** `GetReviewsByIDs` returns the review row with user fields and `book_id`; the review's book card comes from the same single `GetBooksByIDs` call as every other block (one hydration query), so a review whose book was hidden after caching is dropped.
- **Hydration failure.** A failed `GetBooksByIDs` is a 500 (every block depends on it). A failed `GetReviewsByIDs` empties only `recent_reviews`.
- **Pool cover rule** treats an empty `cover_url` like `NULL`.
- **Repository** (`home/repository`) has the PR 1 methods only: `ListPoolIDs`/`PickRandomBookID` share one `PoolFilter` (cover, min rating, min count, genre, exclude), plus the calendar, anniversary, new, top-rated and review queries. The calendar match uses `month*100 + day IN (...)`. The personal methods, `WithSimilarLister`, `GetMyHome` and `mergeRecommendations` arrive with PR 2.
- **Handler** depends on a small `HomeService` interface for tests. The calendar author, review user and review book fields are always present (`null` when empty), not `omitempty`.
- **Catalog export rename:** `BookListItemResponse`, `ToBookListItemResponse`, `AuthorResponse`, `GenreResponse`; JSON unchanged, catalog tests pass. Swagger now shows these under the fully qualified package name because both transport packages are named `http`.
- **Web.** `homeApi = { get, random }` (`me` comes with PR 2). `BookRow` lives in `entities/book`. `shared/lib/format-date-ru.ts` also has `formatTimestampDayMonth` (review dates, Moscow time) and `formatRatingSummary` ("4,6 · 12 оценок"). `PickRandomBook` keeps the last result visible while the next pick loads. If `GET /home` fails, the page renders only the hero and genre chips (the roulette is hidden too). When only one of the book of the day and the calendar is present, it takes the full width. The top-rated row links to `/books?sort=rating` ("Смотреть все").
- **Dev data.** `cmd/seed` now sets real `DeathDate`s for the 11 seed authors. Existing dev databases need this one-off update (applied to the local dev DB):

  ```sql
  UPDATE authors a SET death_date = v.d::date FROM (VALUES ('lev-tolstoy','1910-11-20'),('fyodor-dostoevsky','1881-02-09'),('george-orwell','1950-01-21'),('mikhail-bulgakov','1940-03-10'),('nikolai-gogol','1852-03-04'),('ivan-turgenev','1883-09-03'),('anton-chekhov','1904-07-15'),('mikhail-lermontov','1841-07-27'),('aleksandr-pushkin','1837-02-10'),('aldous-huxley','1963-11-22'),('erich-maria-remarque','1970-09-25')) AS v(slug, d) WHERE a.slug = v.slug AND a.death_date IS NULL;
  ```

  None of these dates falls in early October, so locally the calendar is usually empty (verified by temporarily moving an author's date, then reverting). Ratings/reviews seed data came with PR 2 (`-demo-readers`, below).

### PR 2 (personal) — #233, #234

- **Repository.** Five personal methods as in the contract: `GetSystemShelf` (id + visible count, `nil` when not materialized; never creates shelves), `ListShelfBookIDs` (`added_at DESC`, visible, `limit <= 0` = all), `ListRecommendationSeeds` (the stage-2 positive-interaction rule for one user, ordered by the latest `added_at`/`ratings.updated_at`), `ListUserExcludedBookIDs` (any shelf, custom included, plus any rating) and `AggregatePrecomputedSimilar`. The aggregate takes the user ID and applies visibility, the seeds and the user's shelf/rating exclusions **in SQL**, so its `LIMIT` never cuts valid candidates; `mergeRecommendations` still filters against the same exclusion set, which also covers tier 2.
- **Service.** `GetPersonalHome(ctx, userID)` computes three independent blocks (`continue_reading`, `recommendations`, `want_to_read_pick`) with the public per-block error policy (`ErrAllPersonalBlocksFailed` → 500 only when all three fail) and hydrates every book in one `GetBooksByIDs`. No cache. Tier 2 runs when tier 1 has fewer than `RecommendationsTopUpBelow` (6) books: `ListSimilarBooks(seed, RecommendationsTopUpPerSeed = 20)` for the `RecommendationsTopUpSeeds` (3) most recent seeds. `ErrBookNotFound` is skipped silently; any other lister error is logged and only drops that seed's top-up (tier 1 is kept) rather than failing the block. The want-to-read pick is `pickDaily(visible shelf IDs, today MSK, "wtr:" + userID)`; an empty shelf gives the block with `book: null`.
- **Handler/route.** `GET /api/v1/home/me` behind `authMiddleware`; the user ID comes only from the `user_id` context value (401 when missing or malformed, a `user_id` query parameter is ignored). Wired with `WithSimilarLister(catalogService)` in `cmd/api/main.go`.
- **Web.** `homeApi.me` / `api.home.me` with `PersonalHome` (camelCase, all nested books normalized). `widgets/personal-home` exports `PersonalHome` (continue reading + challenge card + want-to-read pick, mounted right below the hero) and `PersonalRecommendations` (mounted between the book of the day/calendar and "Что почитать?"; if `GET /home` failed it still renders below the personal section). Both share one `useQuery` (`['home', 'me', userId]`; the user ID is in the key so another account never sees a cached payload, `staleTime` 60s, `refetchOnWindowFocus: false`); the challenge uses `['reading-challenges', year, userId]` in parallel. Queries are disabled for guests and while the session resolves, so guests make no `/home/me` request. Skeletons while pending; failed or empty blocks render nothing, and the whole personal section renders nothing when every block is empty. The challenge card reads "Прочитано N из M в <год>" with a progress bar and a "Подробнее" link, or "Поставить цель" → `/challenge` when `goal` is null.
- **Dev data.** `cmd/seed -demo-readers` now adds 4-5 ratings of books the demo readers already read, so 8 seed books reach `ratings_count >= 3` and `rating >= 4.0` (war-and-peace stays above 4.0 despite reader06's 1), plus 4 reviews (40+ characters, one per book, rating = the reader's rating) on war-and-peace, master-and-margarita, crime-and-punishment and 1984. It recalculates `rating`/`ratings_count`/`reviews_count`; existing reviews of the same reader and book are left untouched. A fixture test (`cmd/seed/demo_readers_test.go`) guards these numbers. Re-run `go run ./cmd/seed -demo-readers` and then `go run ./cmd/similarity` on an existing dev DB.
