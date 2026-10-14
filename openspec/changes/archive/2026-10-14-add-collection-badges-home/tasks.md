# Tasks

Ground rules:
- **No destructive DB commands.** Never `migrate down`, a hand-written `DROP`, or a reset of the local DB. This change needs no migration; if one ever becomes necessary, it is `000028_*` and is applied with migrate **up** only.
- **gofmt only the files you changed.** Never the whole tree.
- **No native `next build`.** CI is the typedRoutes gate.

## 1. Backend

- [x] 1.1 Repository (`curated/repository`): add `BookPlace` and `BookPlaces(ctx, bookID)` with the query from design.md Decision 2. Put a cross-reference comment in `BookPlaces` and `PublishedPage`.
  - SQLite tests cover:
    - a pending book ahead → place shifts up (spec «Place counts only published books»);
    - a reserve book → no row;
    - a draft collection → no row;
    - an automatic snapshot with an unpublished book ahead → place 3 (spec «Automatic collection place»);
    - the order (place, size, published_at, title) per spec «Best place first»;
    - a consistency test: for every published book of a mixed fixture, the `BookPlaces` place equals its place from `PublishedPage`.
  - Verify with `go test ./internal/modules/curated/repository/...`.
- [x] 1.2 Service:
  - `ListBookPlaces(ctx, bookID)`: `GetBook`; nil or non-published → `ErrBookNotFound` («book not found»).
  - `ListPublic(ctx, limit)`: slice before `summary`, as in design Decision 7; return the full total.
  - Fake-repository tests cover:
    - a pending book → 404 error;
    - an unknown book;
    - a book with no places → empty;
    - `limit` 2 of 5 → 2 summaries with total 5, and covers requested only for 2;
    - `limit` 0 → all.
- [x] 1.3 Transport and router:
  - `ListBookPlaces` handler with the response shape and error texts from design.md API;
  - `limit` parsing on `ListPublic` (ignore ≤ 0 or unparsable, clamp at 50, `total` = all);
  - the route `api.GET("/books/:id/collections", …)` next to `/books/:id/similar`;
  - Russian swagger annotations, then regenerate `backend/docs` the usual way.
  - Handler tests: 200 shape, 400 «invalid book id», 404 «book not found», 200 empty, the index with `limit=2` (items 2, total 5).
  - `router_curated_test.go`: an anonymous `GET /api/v1/books/{id}/collections` is not 401/403, and an existing `/books/:slug` route still resolves.
- [x] 1.4 Verify from `backend/`: `go build ./...`, `go vet ./...`, `go test ./...`.

## 2. Frontend

- [x] 2.1 `packages/api-client`: `collections.list(params?: { limit?: number })`, `collections.forBook(bookId)` and the exported `BookCollectionPlace` type, per design.md Frontend. Verify with `tsc --noEmit` for the package.
- [x] 2.2 `apps/web` entity `curated-collection`:
  - `forBook` in `api`;
  - `lib/place-href.ts`, with Vitest tests for places 1, 50, 51 and 73 → `/top/s`, `/top/s`, `/top/s?page=2`, `/top/s?page=2`;
  - `ui/collection-place-badges.tsx`;
  - the `headingLevel` prop on `CollectionCard`;
  - exports.
  - RTL tests:
    - the badge text «№3 в „Топ-100 книг всех времён“» and its href;
    - 5 places → 3 badges and «Ещё 2», and a click shows 5 with the button gone;
    - an empty array renders nothing.
- [x] 2.3 `views/book-detail`: fetch `collectionsApi.forBook(book.id)` after the book, with `.catch(() => null)`, and render the badges under the authors. Extend `book-detail-view.test.tsx`:
  - badges render when the API returns places;
  - a rejected `forBook` still renders the page, with no badge and no error text;
  - existing tests mock `forBook` so they stay green.
- [x] 2.4 New `widgets/curated-collections-row` and `views/home`:
  - `limit: 6` in the `Promise.all`;
  - the `shown_count > 0` filter and the slice to 3;
  - placement after «Высоко оценённые»;
  - rendering also when the home payload is null.
  - RTL tests for the widget:
    - 3 cards with `h3` titles and «Все подборки» → `/top`;
    - an empty array renders nothing.
  - If a home-view test harness exists or is easy to add, also test: an empty collection is skipped (spec «Empty collection skipped»), and a null home payload with collections still shows «Подборки».
- [x] 2.5 Verify:
  - `tsc --noEmit` and `next lint` in `apps/web`, and `tsc --noEmit` in `packages/api-client`;
  - the `apps/web` Vitest suite.

  No native `next build`; wait for the CI frontend job.

## 3. Docs

- [x] 3.1 Mark epic #413 feature 4 done in `docs/ROADMAP.md`: book page badges via `GET /books/{id}/collections`, the home «Подборки» row, the index `limit`, no migration, OpenSpec change `add-collection-badges-home`. Swagger is regenerated in 1.3. The orchestrator archives the OpenSpec delta; do not edit `openspec/specs/` directly.
