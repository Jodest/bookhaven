## Context

Epic #413 adds BookHaven-curated ranked collections. Feature 1 (this change) builds the entity, the public pages and admin management of **manual** collections. Later features reuse it: bulk paste with catalog matching (2), automatic collections built by a worker from a Bayesian rating and stored as a snapshot (3), the «№N в …» badge and a home block (4), AI drafting (5). Readers' lists (`lists`/`list_items`, module `lists`) stay untouched.

## Decisions

1. **Separate module `backend/internal/modules/curated`** (domain / repository / service / transport/http), not an extension of `lists`: no user owner, ranked places, size, source, visibility. Name `curated` avoids clashing with the OpenSpec domain name `collections`.
2. **Places are computed at read time** from the stored admin order (`position` 1..k, dense) and `books.status = 'published'`. Nothing is rewritten when a book's status changes. Public place = row number among published items ordered by `position`, capped at `size`.
3. **Pending books are accepted in a collection** (rejected are not). The admin can queue a book that is still in moderation; it is `hidden` until published. Rationale: harvested books land as `pending`, and making the admin come back later is the friction the reserve is meant to remove. A book that becomes rejected later simply stays `hidden` and is visible to the admin as such.
4. **`kind` column now, only `manual` allowed** (`CHECK (kind IN ('manual'))`). Feature 3 widens the check with a forward migration and reuses `curated_collection_items` for the snapshot (its `position` will equal the computed rank). Admin book-mutation endpoints will reject non-manual collections then; today all are manual. API responses already expose `kind`.
5. **Reordering = move one book to a position** (`PUT …/books/{bookId}/position`), not a full-order PATCH. It is atomic, cheap for 1000 items and matches the UI (up/down and «Переместить на»). No drag-and-drop: the apps have no DnD library and adding one is not worth it now.
6. **Concurrency**: every item mutation runs in a transaction that first locks the collection row (`SELECT … FOR UPDATE`), then shifts positions, so positions stay dense under concurrent edits. There is deliberately no unique index on `(collection_id, position)` (only an ordinary one), so a plain `UPDATE … SET position = position ± 1 WHERE position BETWEEN …` followed by setting the moved row is enough, with no transient-duplicate failures.
7. **Limits**: `size` 1..1000; at most 1000 books per collection (ranked + reserve); public page `limit` default 50, max 100.
8. **Slug**: optional on create; empty → derived from the title. Extract the admin service's `slugify` into `backend/internal/pkg/slug` (`slug.Make`) and make `admin/service` call it — identical behavior, existing admin tests must stay green. Validate `^[a-z0-9]+(-[a-z0-9]+)*$`, ≤ 100 chars; if derivation yields an empty string → 400 `slug is required`. Uniqueness → 409.
9. **Index order**: published collections newest first by `published_at` (set on the first publish, never cleared), ties by title. No manual ordering of collections in v1.
10. **Admin nav**: own sidebar section «Подборки» (id `collections`, icon `ListOrdered`, admin only) instead of a tab under «Каталог», so the catalog-lists rule «the catalog offers no add action» stays true.
11. **Genre**: optional FK `ON DELETE SET NULL`; accepted when the genre exists, is not rejected and is not merged (`merged_into IS NULL`). Public output shows the genre only when published. Link target on web: `/books?genre_id={id}` (existing catalog filter).

## API

Public (no auth middleware; register in `RegisterRoutes`):

- `GET /api/v1/collections` → `200 {items: CollectionSummary[], total}`
  - `CollectionSummary = {id, slug, title, description, kind, size, shown_count, source|null, source_url|null, genre: {id,name,slug}|null, cover_urls: string[]}`
  - `shown_count = min(size, count(items where book published))`; `cover_urls` = non-null covers among the first 4 placed books (one query per collection is fine; collections are few — or a single window-function query).
- `GET /api/v1/collections/:slug?page&limit` → `200 {collection: CollectionSummary, items: [{place, book: bookListItemResponse}], total, page, limit}`; `404 {"error":"collection not found"}` for unknown or draft.
  - Query: `SELECT books.* … JOIN curated_collection_items i … WHERE i.collection_id = ? AND books.status = 'published' ORDER BY i.position, i.book_id LIMIT ? OFFSET ?` with `LIMIT = max(0, min(limit, size - offset))`; `place = offset + idx + 1`. Preload Authors and Genres. Reuse the `bookListItemResponse` / `authorResponse` mapping rules of `lists/transport/http` (non-published author → name only, slug null; published genres only) — copy into the module (no cross-module transport import).

Admin (`collections := admin.Group("/collections", adminRole)`; `requireBearer` on every non-GET):

| Method | Path | Body | Success | Errors |
|---|---|---|---|---|
| GET | `/admin/collections` | — | 200 `{items:[{id,slug,title,kind,size,is_published,books_count,shown_count,updated_at}], total}` | — |
| POST | `/admin/collections` | `{title, size, slug?, description?, source?, source_url?, genre_id?, is_published?}` | 201 `AdminCollection` | 400 field, 409 `slug already exists` |
| GET | `/admin/collections/:id` | — | 200 `AdminCollectionDetail` | 400 bad uuid, 404 `collection not found` |
| PATCH | `/admin/collections/:id` | same fields, all optional; `genre_id: null` clears, `source`/`source_url` empty string clears | 200 `AdminCollection` | 400, 404, 409 |
| DELETE | `/admin/collections/:id` | — | 200 `{status:"ok"}` | 404 |
| POST | `/admin/collections/:id/books` | `{book_id, position?}` | 201 `{status:"ok"}` | 400, 404 `collection not found` / `book not found`, 409 `book already in collection`, 422 `book is rejected` / `collection is full` |
| DELETE | `/admin/collections/:id/books/:bookId` | — | 200 | 404 `book not in collection` |
| PUT | `/admin/collections/:id/books/:bookId/position` | `{position}` | 200 | 400 `invalid position`, 404 |

- `AdminCollection = {id, slug, title, description, kind, size, source, source_url, genre: {id,name,slug,status}|null, is_published, published_at|null, created_at, updated_at}`
- `AdminCollectionDetail = AdminCollection + {books_count, shown_count, items: [{position, place|null, state: "ranked"|"reserve"|"hidden", book: {id,title,slug,status,cover_url,authors:[{id,name}]}}]}` — no pagination (≤ 1000 rows, admin only). State is computed in the service from the ordered rows.
- Validation: title 2..255 after trim; description ≤ 5000; size integer 1..1000; source ≤ 255; source_url ≤ 500 and `http(s)://` with a host; `genre_id` uuid. Errors are `{"error": "<field>: <reason>"}` like other admin handlers.
- Swagger annotations in Russian like the rest of the handlers; regenerate `backend/docs` the usual way if the repo keeps it generated.

## Database

Forward-only migration `backend/migrations/000025_curated_collections.{up,down}.sql`:

```sql
CREATE TABLE curated_collections (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         VARCHAR(100) NOT NULL UNIQUE,
    title        VARCHAR(255) NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    kind         VARCHAR(20) NOT NULL DEFAULT 'manual' CHECK (kind IN ('manual')),
    size         INTEGER NOT NULL CHECK (size BETWEEN 1 AND 1000),
    source       VARCHAR(255),
    source_url   VARCHAR(500),
    genre_id     UUID REFERENCES genres(id) ON DELETE SET NULL,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_curated_collections_published ON curated_collections (is_published, published_at DESC);

CREATE TABLE curated_collection_items (
    collection_id UUID NOT NULL REFERENCES curated_collections(id) ON DELETE CASCADE,
    book_id       UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    position      INTEGER NOT NULL CHECK (position >= 1),
    added_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (collection_id, book_id)
);
CREATE INDEX idx_curated_collection_items_order ON curated_collection_items (collection_id, position);
CREATE INDEX idx_curated_collection_items_book ON curated_collection_items (book_id);
```

The down file drops both tables (exists by convention; never run by agents). The `book_id` index serves feature 4 («№N в …» badge).

## Frontend

`packages/api-client`: types `CuratedCollection`, `CuratedCollectionDetail`, `AdminCuratedCollection`, `AdminCuratedCollectionDetail`, `AdminCuratedCollectionItem`; `collectionsApi.list()`, `collectionsApi.get(slug, {page, limit})`; `adminCollectionsApi.list/get/create/update/remove/addBook/removeBook/moveBook` (admin writes go through the existing bearer-sending admin fetcher).

`apps/web` (FSD):
- `entities/curated-collection` — query keys, hooks `useCuratedCollections`, `useCuratedCollection(slug, page)`, `formatSource` helper, a `CollectionCard` UI.
- `views/collections` (`/top`) and `views/collection-detail` (`/top/{slug}`); pages `app/top/page.tsx`, `app/top/[slug]/page.tsx` (thin, like `app/lists/public/[id]/page.tsx`). Ranked row: big place number, cover, title → `/books/{slug}`, authors (link `/authors/{slug}` only when slug non-null), rating `★ 4.3 (120)`. Pagination: reuse the page control used by `views/public-list-detail`, `page` in the URL.
- `widgets/header`: add «Подборки» → `/top` next to «Каталог», outside the auth branch. Optionally the footer too.
- No middleware change: `/top` is not a guarded prefix.

`apps/admin` (FSD):
- `shared/config/navigation.ts`: add `NavSectionId` `'collections'`, section «Подборки» (`ListOrdered`), tab `collections.list` (`/collections`, match `['/collections']`, roles `['admin']`). Update `navigation.test.ts` / shell tests.
- `entities/curated-collection` — keys, types re-export, state labels («Резерв», «Скрыта: не опубликована»), `reserveWarning(shown, size)` returning «В подборке видно {shown} из {size} книг — пополните резерв» or null.
- `features/manage-collection` (create/update/delete mutations, form with slug hint), `features/edit-collection-books` (add/remove/move mutations, invalidating the detail query).
- `widgets/collection-books` — the ordered table with the reserve divider, up/down, «Переместить на» input, «Убрать», plus the catalog search box (uses `GET /admin/books?search=&limit=10` through the existing admin books api; shows status badge; «Добавить» disabled for rejected or already present ids).
- `views/collections` (list) and `views/collection-edit` (form + widget; `new` mode renders only the form and redirects to `/collections/{id}` after create). Pages `app/collections/page.tsx`, `app/collections/new/page.tsx`, `app/collections/[id]/page.tsx` (+ layout like other sections if needed).
- Genre select: published genres via the admin genres list (`status=published&limit=100`), same approach as the catalog book filters.
- Error mapping: 409 slug → field error «Такой адрес уже занят»; 409 duplicate book → «Книга уже в подборке»; 422 rejected → «Отклонённую книгу нельзя добавить»; 422 full → «В подборке уже 1000 книг».

## Existing Patterns

- Module layout and handler style: `backend/internal/modules/lists` (domain/repository/service/transport, `bookListItemResponse`, published-only joins, position-ordered items).
- Admin route group, `adminRole`, `requireBearer`: `backend/internal/transport/http/router.go` `RegisterAdminRoutes`; router tests like `router_ai_descriptions_test.go` for 403/401.
- Slug transliteration: `admin/service/service.go` `slugify` (to be extracted).
- SQLite repository tests: `lists/repository/book_status_sqlite_test.go`.
- Admin views/widgets: `views/genre-edit`, `widgets/catalog-list`, nav config `shared/config/navigation.ts`.
- Web public page: `views/public-list-detail`, `app/lists/public/[id]/page.tsx`.

## Risks

- **Large detail payload**: admin detail returns up to 1000 rows with authors — acceptable; preload authors in one query.
- **Position races**: mitigated by the row lock (Decision 6); a SQLite test cannot prove the lock, so cover dense-position invariants in service tests and keep the lock in the repository transaction.
- **Genre merge**: a collection linked to a genre that is later merged keeps pointing at the alias row; public output hides it unless published. Acceptable for v1; merge could repoint `curated_collections.genre_id` later.
- **typedRoutes**: new `/top/[slug]` and admin `/collections/[id]` links must be typed correctly; CI build is the gate (no native `next build`).
- **Feature 3 compatibility**: widening the `kind` CHECK is a forward migration; keep `kind` in every response from day one so clients do not break.
