# Design

## Context
Features 1-3 of epic #413 shipped manual and automatic curated collections (`backend/internal/modules/curated`, tables `curated_collections` and `curated_collection_items`). Both kinds keep their order in `curated_collection_items.position`: the admin order for manual collections and the snapshot order for automatic ones. Both derive public places the same way at read time. The places are the published books ordered by `(position, book_id)`, numbered 1..N, where N is the collection's `size` (see `PublishedPage` and `Covers` in `repository/repository.go`). Migration 000025 already created `idx_curated_collection_items_book (book_id)` and `idx_curated_collection_items_order (collection_id, position)`. This change needs no migration.

## Decisions

1. **New endpoint, not an extension of the public book endpoint.** We add `GET /api/v1/books/{id}/collections`, served by the `curated` handler.
   - Extending `GET /books/{slug}` would make the `catalog` module query curated tables, either through a cross-module import or a new port. Every consumer of the book payload would then pay for the places query.
   - The new endpoint follows the existing `GET /books/{id}/similar` pattern: per-book, keyed by id, owned by one module.
   - Places are never cached. They are computed per request, so a badge always equals what `/top/{slug}` shows at that moment, which is the spec requirement. The query is cheap (Decision 2), so a cache would only add staleness.

2. **One query for all places of a book.** The query lives in the repository:
   ```sql
   SELECT * FROM (
     SELECT c.id, c.slug, c.title, c.kind, c.size, c.published_at,
            1 + (SELECT COUNT(*)
                 FROM curated_collection_items j
                 JOIN books b ON b.id = j.book_id
                 WHERE j.collection_id = i.collection_id
                   AND b.status = 'published'
                   AND (j.position < i.position
                        OR (j.position = i.position AND j.book_id < i.book_id))) AS place
     FROM curated_collection_items i
     JOIN curated_collections c ON c.id = i.collection_id
     WHERE i.book_id = ? AND c.is_published = ?
   ) t
   WHERE t.place <= t.size
   ORDER BY t.place ASC, t.size ASC, t.published_at DESC, t.title ASC
   ```
   - The outer query uses `idx_curated_collection_items_book`. It returns one row per collection that holds the book, which is a handful in practice.
   - Each correlated count is a range scan on `idx_curated_collection_items_order` over at most `position - 1` rows (at most 999), each joined to `books` by primary key. The worst case per request is about 1000 × (collections holding the book) cheap lookups.
   - The tie-break `(position, book_id)` mirrors the `PublishedPage` order exactly, so the number matches `/top/{slug}` even in the theoretical case of equal positions.
   - The SQL is portable (no window functions over the whole table, no Postgres-only syntax), so the SQLite repository tests run it unchanged. Pass the status as a parameter (`catalogDomain.BookStatusPublished`), as `Counts` does.
   - A never-computed automatic collection has no items, so it is excluded naturally.
   - **Rejected alternative:** `ROW_NUMBER() OVER (PARTITION BY collection_id …)` over all items of every collection containing the book. It reads every item of those collections instead of only the rows ahead of the book.

3. **Book visibility.** The service first loads the book through the existing `repo.GetBook`.
   - It returns `ErrBookNotFound` (404 «book not found») when the book is missing or its status is not `published`. This mirrors the public book page, which 404s non-published books, and the `similar` endpoint.
   - This also guarantees that a hidden book never gets a "place", because the count only counts books ahead of it.

4. **Badge ordering and limit.** Order: place ascending, then smaller size, then `published_at DESC`, then title.
   - The API returns all items, bounded by the number of published collections, so no pagination is needed.
   - The UI shows 3 badges and «Ещё {n}» expands the rest. This keeps the header compact without hiding data or needing a plural helper.

5. **Book page data flow.** `BookDetailView` is a server component. It loads the book by slug and then calls `collectionsApi.forBook(book.id).catch(() => null)`.
   - The call is sequential because the slug-to-id step is needed.
   - We render on the server rather than in a client widget like `SimilarBooks`. The badges sit right under the title, above the fold, so server rendering avoids layout shift and keeps them in the SSR HTML for SEO.
   - A failure yields no badges, never an error page.

6. **Home block uses the existing index API, not the `/home` payload.** Adding collections to `GET /home` would make the `home` module query curated tables and enter the 10-minute selection cache. Instead, `HomeView` fetches `collectionsApi.list({ limit: 6 })` in its existing `Promise.all`, next to `genresApi.list()`.
   - It keeps `items.filter(c => c.shown_count > 0).slice(0, 3)`.
   - The block reuses the `/top` card shape as is, and no `home` backend code changes.
   - The block is independent of the home payload: it renders even when `/home` fails.
   - **Why `limit=6` and not 3.** It leaves room to skip empty collections (e.g. a published automatic collection whose rule matches nothing yet). It still bounds the per-render cover queries.

7. **Index `limit` implementation.** `ListPublic(ctx, limit)` still loads all published collections (one query) and their `Counts` (one query). It then slices to `limit` *before* computing summaries, because `Covers` is the per-collection query. The handler sets `total` to the full count.
   - `limit` ≤ 0, missing or unparsable → no limit; > 50 → 50.
   - `/top` keeps calling it without `limit`.

8. **No «Показывать на главной» flag.** The home row shows the newest published collections, the same order as `/top`. That needs no admin UI or migration. A featured flag can be added later if the admin needs control. Note: re-publishing does not move a collection forward, because `published_at` is kept.

## API

### `GET /api/v1/books/{id}/collections` (public, no token)
- **Route.** Register `api.GET("/books/:id/collections", curatedHandler.ListBookPlaces)` in `backend/internal/transport/http/router.go`, next to `/books/:id/similar`. Echo already resolves `/books/:slug` vs `/books/:id/similar`, so the same applies here; keep the param name `:id` like `similar`.
- **200 response:**
  ```json
  {
    "items": [
      { "place": 3,
        "collection": { "id": "uuid", "slug": "top-100", "title": "Топ-100 книг всех времён", "kind": "manual", "size": 100 } }
    ],
    "total": 1
  }
  ```
- **Errors:**
  - 400 `{"error":"invalid book id"}` for a non-UUID id;
  - 404 `{"error":"book not found"}` for an unknown or non-published book;
  - 500 `{"error":"failed to get book collections"}`.
- **Swagger.** Add a Russian annotation (`@Summary Места книги в подборках`, `@Tags collections`, `@Router /books/{id}/collections [get]`) and regenerate `backend/docs`.

### Layers
- **Repository:** `BookPlaces(ctx, bookID uuid.UUID) ([]BookPlace, error)`, where `BookPlace{Collection domain.Collection (id, slug, title, kind, size, published_at), Place int}`. It runs the query from Decision 2.
- **Service:** `ListBookPlaces(ctx, bookID) ([]repository.BookPlace, error)`, which checks the book via `GetBook` and maps a missing or non-published book to a new `ErrBookNotFound` in `curated/service`, unless an equivalent already exists. Check `mapError`.
- **Handler:** maps to the response above.

### `GET /api/v1/collections?limit=`
- Optional, 1-50; see Decision 7.
- Response shape unchanged.
- `total` is the count of all published collections. Today it equals `len(items)`, so this is not breaking.

## Frontend

### `packages/api-client`
- `collections.list(params?: { limit?: number })` appends `?limit=` when given.
- `collections.forBook(bookId: string)` calls `fetcher<{ items: BookCollectionPlace[]; total: number }>(\`/api/v1/books/${bookId}/collections\`)`.
- Type: `BookCollectionPlace { place: number; collection: { id: string; slug: string; title: string; kind: CuratedCollectionKind; size: number } }`, reusing the existing kind type if there is one. Keep snake_case as the other curated types do. Export the type.

### `apps/web/src/entities/curated-collection`
- `api/index.ts`: add `forBook`.
- `lib/place-href.ts`: `collectionPlaceHref(slug, place)` returns `/top/{slug}` for `place <= COLLECTION_PAGE_SIZE`, else `/top/{slug}?page={ceil(place / COLLECTION_PAGE_SIZE)}`. Import `COLLECTION_PAGE_SIZE` from `model/use-curated-collections.ts`, or move the constant to `lib`/`config` inside the entity to avoid a `lib → model` import. Write unit tests for places 1, 50, 51 and 73.
- `ui/collection-place-badges.tsx` (`'use client'`, state for the expansion):
  - props `{ places: BookCollectionPlace[] }`; it renders nothing for an empty array;
  - each badge is a `Link` with the text `№{place} в „{title}“`;
  - the first 3 are shown, then a button «Ещё {n}» (`n = places.length - 3`) that reveals the rest and hides itself;
  - a wrapper `<div aria-label="Места в подборках">`;
  - reuse `Badge` from `@bookhaven/ui` (variant `secondary` or `outline`) inside the link.
- `ui/collection-card.tsx`: add an optional `headingLevel?: 'h2' | 'h3'` (default `h2`), so that cards under the home section's `h2` use `h3`.
- Export the new members from `index.ts`.
- Typed routes: build the href as a template string and cast `as Route` (or `as any`, like `collection-detail-view`) only if `typedRoutes` rejects the query string.

### `apps/web/src/views/book-detail/ui/book-detail-view.tsx`
After `booksApi.get`, run `const places = await collectionsApi.forBook(book.id).catch(() => null);`. Render `<CollectionPlaceBadges places={places?.items ?? []} />` in a `mt-4` block right after the authors' block. A view may import entities.

### `apps/web/src/widgets/curated-collections-row` (new)
- `ui/curated-collections-row.tsx`: a server-compatible component (no hooks), props `{ collections: CuratedCollection[] }`.
- It returns `null` for an empty array.
- It renders a `<section>` with the `h2` «Подборки» and the link «Все подборки» → `/top`, matching the `BookRow` header layout (check `entities/book` `BookRow` for the heading/link classes).
- Below the header: a grid `sm:grid-cols-2 lg:grid-cols-3` of `CollectionCard headingLevel="h3"`.
- `index.ts` exports it.

### `apps/web/src/views/home/ui/home-view.tsx`
- Add `collectionsApi.list({ limit: 6 }).catch(() => null)` to the `Promise.all`.
- Compute `const homeCollections = (collections?.items ?? []).filter((c) => c.shown_count > 0).slice(0, 3);`.
- Render `<CuratedCollectionsRow collections={homeCollections} />` after «Высоко оценённые» and before `<RecentReviews>` when `home` is loaded.
- When `home` is null, render it in the fallback container next to `PersonalRecommendations`, so the block is shown regardless of the home payload.
- Update the doc comment (the spec reference is `openspec home/*`).

### FSD
- The views import entities and widgets.
- The widget imports only the `curated-collection` entity and `@bookhaven/ui`.
- No widget imports another widget, and no view imports a view.

## Existing Patterns
- Per-book public sub-resource: `GET /books/:id/similar` (`catalog/transport/http/handler.go` `ListSimilarBooks`) for the id parsing, the 400/404 texts and route placement.
- Read-time ranking: `curated/repository` `PublishedPage`, `Covers` and `Counts` (published filter, `(position, book_id)` order, status as a parameter).
- Index cards: `entities/curated-collection/ui/collection-card.tsx`.
- Home server fetch with `.catch(() => null)` and empty-block-renders-nothing: `views/home/ui/home-view.tsx`, `widgets/recent-reviews`.
- Route tests: `backend/internal/transport/http/router_curated_test.go`. Add the anonymous 200/404 checks there.

## Risks
- **Place drift.** If someone later changes the public ordering in `PublishedPage` (e.g. a different tie-break) without updating `BookPlaces`, badges and `/top` disagree. Mitigation: a repository test asserts that for every book in a mixed fixture (pending in the middle, reserve, an automatic snapshot), `BookPlaces` equals the place from `PublishedPage`. Add a comment in both functions pointing to each other.
- **Cost of the correlated count.** It is bounded by 1000 rows per collection holding the book. That is fine at the current scale. If collections grow into the hundreds per book, cache per book in Redis with a short TTL.
- **Extra request on the book page.** It adds one sequential server request after the book fetch, a few ms against the local API. If latency matters, move the badges to a client widget.
- **Home row cost.** Each home render calls the index: 2 queries plus at most 6 cover queries. This is acceptable. The `/home` selection cache does not cover it.
- **Typed routes.** A query-string href (`?page=`) may need a cast. CI is the gate, so do not run a native `next build`.
