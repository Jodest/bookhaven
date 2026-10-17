# Catalog Pagination

Status: completed

## Goal

Make catalog pagination on `/books` correct and pleasant: stable, duplicate-free pages from the backend; one shared numbered paginator for web and admin; and a "Показать ещё" + numbered pager hybrid in the public catalog.

The public catalog has no OpenSpec domain (it is not covered by `catalog-admin` / `catalog-moderation` / `home`), so this plan is a Feature Contract. The API contract of `GET /api/v1/books` does not change.

Work is split into three independently mergeable parts, in this order:

- **Part 1 — backend**: deterministic `ListBooks` ordering and duplicate-free genre/author filters.
- **Part 2 — frontend / packages/ui**: a shared `Pagination` component plus a page-range function; admin moves onto it.
- **Part 3 — frontend / apps/web**: catalog view gets "Показать ещё", link-based numbered pagination, scrolling and dimming.

Part 3 depends on Part 2. Part 1 is independent.

## User Stories

1. As a reader paging through the catalog, I never see the same book twice on different pages and I never miss a book.
2. As a reader filtering by several genres, I see each book once and the total/page count is correct.
3. As a reader, I can press «Показать ещё 24» to append the next page without losing my scroll position.
4. As a reader, I can jump to any page by number, the view scrolls to the top of the grid, and I can open a page in a new tab or share its URL.
5. As an admin, every paginated list has the same numbered paginator.

## Acceptance Criteria

### Part 1 — backend

- [ ] Every `ListBooks` sort ends with a unique tiebreaker `books.id ASC`. Under any sort, paging through all pages with any limit returns each matching book exactly once.
- [ ] A genre filter with several IDs returns a book that has 2+ of the selected genres **once**, and `total` counts it once.
- [ ] Combining the author filter with the genre filter produces no duplicates and gives a correct `total`.
- [ ] The existing published-genre and published-author rules stay unchanged. Existing sqlite tests (`genre_status_sqlite_test.go`, `author_status_sqlite_test.go`, `book_status_sqlite_test.go`) pass without edits.
- [ ] HTTP contract unchanged: same params, the same `limit` default of 20 and cap of 100, the same response shape.

### Part 2 — shared paginator

- [ ] `@bookhaven/ui` exports `Pagination` and a pure `getPaginationRange` function.
- [ ] Desktop (`sm` and wider) shows `‹ 1 … 4 [5] 6 … 42 ›`: first and last page always, a sibling window around the current page, and `…` for gaps. The output never has a `…` that hides exactly one page; that page is shown instead.
- [ ] Mobile (narrower than `sm`) uses compact mode `‹ 5 / 42 ›`.
- [ ] Prev/next are **disabled** at the edges. They do not disappear, so the layout does not shift.
- [ ] `<nav aria-label="Пагинация">`. The current page has `aria-current="page"` and is not a link. Each page item has `aria-label="Страница N"`. Prev/next have `aria-label="Предыдущая страница"` / `"Следующая страница"`. Ellipses are `aria-hidden`.
- [ ] The component works in link mode (`getHref` + `renderLink`) and in button mode (`onPageChange`).
- [ ] Renders nothing when `totalPages <= 1`.
- [ ] Every admin view that uses `Pagination` (catalog-list, users, moderation-queue, screen-works, ai-description-moderation, reviews) works unchanged, through the `@/shared/ui` re-export.
- [ ] `packages/ui` has unit tests for `getPaginationRange` and component tests for `Pagination`.

### Part 3 — web catalog

- [ ] The catalog requests `limit: 24`, and `totalPages = ceil(total / 24)`.
- [ ] The «Показать ещё N» button is placed above the numbered paginator. It appends the next page to the end of the grid and updates `?page=` in the URL without scrolling or a new history entry. It is hidden when the last loaded page is the last page.
- [ ] Paginator items are real `<a href="/books?…&page=N">` links built from the current URL. All other params (including `author_name`) are preserved. Page 1 has no `page` param.
- [ ] Clicking a page number, prev or next scrolls to the top of the grid, not to the top of the page.
- [ ] Skeletons appear only on the first load, when there are no books yet. Any later fetch keeps the old grid with reduced opacity and sets `aria-busy="true"`.
- [ ] Stale responses never overwrite newer ones (for example, fast clicks or a filter change during load).
- [ ] A direct visit or reload of `?page=N` shows page N only.
- [ ] The #459 regression stays fixed: URL changes, including the `page` change from «Показать ещё», do not make SearchBar reset `page`. `search-bar.test.tsx` stays green.

## Business Rules

1. **Sort orders** (all add `books.id ASC` last):
   - `rating` (default): `books.rating DESC, books.created_at DESC, books.id ASC`
   - `new`: `books.published_year DESC NULLS LAST, books.created_at DESC, books.id ASC`
   - `title`: `books.title ASC, books.id ASC`
2. **Genre filter semantics stay ANY (OR)**: a book matches if it has at least one selected genre that is published. Multiple matching genres never multiply the row.
3. **Author filter**: the book has the given author, and that author is published.
4. **Loaded range model (web catalog)**: the view holds a contiguous range of loaded pages `[startPage, endPage]`. The grid shows those pages in order. The URL `?page=` always equals `endPage`.
   - Initial load or reload of `?page=N`: `startPage = endPage = N`.
   - «Показать ещё»: fetch `endPage + 1` and append it, then `endPage += 1`. The URL is updated with `history.replaceState`, so there is no new history entry and Back leaves the current catalog state as before. The page does not scroll.
   - Navigation by paginator link (number, prev, next): the range resets to that single page and the grid is replaced (dimmed while loading). This adds a normal history entry.
   - A filter or sort change resets to page 1 with a single-page range. This is existing behaviour.
5. **Paginator state with a multi-page range**:
   - Current (highlighted, `aria-current`) = `endPage`.
   - `‹` (prev) leads to `startPage - 1`, the page before the first loaded one. It is disabled when `startPage === 1`.
   - `›` (next) leads to `endPage + 1`. It is disabled when `endPage === totalPages`.
   - Numbers inside `[startPage, endPage - 1]` are ordinary links. Clicking one shows that page alone. They get no special styling.
   - The window and ellipses are computed around `endPage`.
6. **Button label**: «Показать ещё N», where `N = min(24, total - endPage * 24)`. For example, the last partial page reads «Показать ещё 7».
7. **Out-of-range page** (`?page=N` with `N > totalPages` and `total > 0`): show the message «На этой странице книг нет» with a link «Перейти на последнюю страницу» (`?page=totalPages`) instead of the "nothing found" EmptyState. `total === 0` keeps the existing EmptyState.
8. **Invalid page** (`page < 1` or not a number): treated as 1. This is existing behaviour.

## UX

- **Grid container**: carries a `ref` and `scroll-mt-6`. While a replacing fetch is in flight it gets `opacity-50 transition-opacity pointer-events-none` and `aria-busy="true"`.
- **«Показать ещё»**: an outline `Button`, full-width on mobile and centered on desktop, with `mt-8` under the grid. While loading it is disabled, shows the label «Загрузка…» and sets `aria-busy`. Appended cards appear without a skeleton.
  - If loading more fails, an inline error «Не удалось загрузить книги» appears next to the button. The already loaded books stay, and the button can be pressed again to retry.
- **Paginator**: `mt-6` under the button, centered.
- **Scroll on page navigation**: `gridRef.current.scrollIntoView({ block: 'start', behavior })`, where `behavior` is `'smooth'` unless `prefers-reduced-motion: reduce` is set, in which case it is `'auto'`. The scroll happens on click, together with the dimming. The old books stay visible while dimmed, so there is no blank jump.

## API

No changes. `GET /api/v1/books?page=&limit=24&…` is already supported (`limit` cap 100).

## Database

No schema changes, no migrations.

## Backend (Part 1)

File: `backend/internal/modules/catalog/repository/repository.go`, `ListBooks`.

- Replace the genre `Joins(...)` + `IN` with:
  `Where("EXISTS (SELECT 1 FROM book_genres bg JOIN genres g ON g.id = bg.genre_id WHERE bg.book_id = books.id AND bg.genre_id IN ? AND g.status = ?)", filter.GenreID, domain.GenreStatusPublished)`
- Replace the author join with:
  `Where("EXISTS (SELECT 1 FROM book_authors ba JOIN authors a ON a.id = ba.author_id WHERE ba.book_id = books.id AND ba.author_id = ? AND a.status = ?)", *filter.AuthorID, domain.AuthorStatusPublished)`
- With the joins gone, the `Count` needs no `DISTINCT`. Qualify columns in `ORDER BY` and filters with `books.` where ambiguity is possible. Add `books.id ASC` to all three orders (see Business Rules 1).
- Leave other call sites (for example, the `GetAuthorBySlug` preload order) untouched. They are out of scope.

## Frontend

### Part 2 — `packages/ui`

- `packages/ui/src/lib/pagination-range.ts`:
  - `export type PaginationItem = { type: 'page'; page: number } | { type: 'ellipsis'; key: 'start' | 'end' }`
  - `export function getPaginationRange(current: number, total: number, siblings = 1): PaginationItem[]`
  - Behaviour: `total <= 0` returns `[]`. `current` is clamped to `[1, total]`. First and last pages are always included. Ellipses are only used to replace 2 or more pages. When `total <= 2*siblings + 5`, all pages are listed.
  - Examples: `(5, 42)` gives `1 … 4 5 6 … 42`. `(1, 42)` gives `1 2 3 4 5 … 42`. `(42, 42)` gives `1 … 38 39 40 41 42`. `(3, 7)` gives `1 2 3 4 5 6 7`.
  - The function is pure and has no React dependency.
- `packages/ui/src/components/ui/pagination.tsx`, `export function Pagination(props)`:
  - `page: number`, `totalPages: number`
  - `prevPage?: number` / `nextPage?: number`: targets for `‹` / `›`, defaulting to `page - 1` / `page + 1`. The web catalog passes `startPage - 1` and `endPage + 1`. A target outside `[1, totalPages]` renders the control disabled.
  - `getHref?: (page: number) => string`
  - `renderLink?: (props: { href: string; className: string; 'aria-label': string; children: ReactNode; onClick?: MouseEventHandler<HTMLAnchorElement> }) => ReactNode`. It defaults to a plain `<a>`.
  - `onPageChange?: (page: number) => void`
  - `siblings?: number`, `className?: string`
  - Mode: if `getHref` is set, items are links (rendered via `renderLink`) and `onPageChange`, if also set, is called from the link `onClick`. That is how the web catalog triggers its scroll. Otherwise items are `<button type="button">` that call `onPageChange`.
  - Disabled controls are `<span aria-disabled="true">` in link mode and `<button disabled>` in button mode.
  - Desktop list `hidden sm:flex`; compact `flex sm:hidden` with `‹ {page} / {totalPages} ›`. The compact text has `aria-label="Страница {page} из {totalPages}"`.
  - Visible prev/next glyphs: `ChevronLeft` / `ChevronRight` from `lucide-react`, which is already a dependency. Styling uses `buttonVariants` (`outline` / `ghost`, `size: 'sm'`) and `cn`.
  - No `next` dependency. A Next `Link` is injected via `renderLink`, which avoids `asChild` / Slot complexity and the typedRoutes type mismatch of a `linkComponent` prop.
- Export both from `packages/ui/src/index.ts`.
- Test infra for `packages/ui`:
  - Add `vitest`, `jsdom`, `@testing-library/react`, `@testing-library/jest-dom` and `@vitejs/plugin-react` as devDependencies. Match the major versions already used in `apps/web/package.json`.
  - Add `vitest.config.ts` (jsdom) and `vitest.setup.ts`.
  - Add the script `"test": "vitest run"`. CI's `pnpm run test` → turbo picks it up automatically.
- Admin: `apps/admin/src/shared/ui/pagination.tsx` becomes a thin re-export (`export { Pagination } from '@bookhaven/ui'`). The six view call sites keep the import and props (`page`, `totalPages`, `onPageChange`, button mode).
  - Update `apps/admin/src/shared/ui/pagination.test.tsx` to the new labels: `Предыдущая страница` / `Следующая страница`. Keep all four existing cases: hidden at ≤1, prev disabled on page 1, next disabled on last, `onPageChange(2/4)` from page 3. Add one case: clicking a number calls `onPageChange(n)`.

### Part 3 — `apps/web` catalog

- `apps/web/src/views/catalog/lib/page-href.ts` exports `buildCatalogPageHref(pathname, searchParams, page): string`. It clones the params, deletes `page` when `page <= 1` and otherwise sets it. The function is pure and unit-tested.
- `apps/web/src/views/catalog/ui/catalog-view.tsx`:
  - `const PAGE_SIZE = 24`. Use it in `booksApi.list` and `totalPages`.
  - State: `books`, `total`, `startPage`, `endPage`, `status` (`'initial' | 'replacing' | 'idle'`), `loadingMore`, `loadMoreError`.
  - Replace the current fetch effect:
    - Derive `filterKey` (all params except `page`) and `urlPage` from `searchParams`.
    - A `skipNextUrlSyncRef` is set by «Показать ещё» just before `history.replaceState`. When the effect sees that flag and `urlPage === endPage`, it clears the flag and does **not** refetch.
    - Every other change does a replacing fetch of `urlPage`: `startPage = endPage = urlPage`. Status is `'initial'` if there are no books yet, otherwise `'replacing'`.
    - Guard responses with an incrementing request id (or `AbortController`) and drop stale ones.
  - `loadMore()`: fetch `endPage + 1` with the same filters and append the items. Deduplicate by `book.id` defensively. Set `endPage`, set `skipNextUrlSyncRef`, then call `window.history.replaceState(null, '', buildCatalogPageHref(...))`. Next 14.2 syncs `useSearchParams` from native history.
  - Paginator:
    ```
    <Pagination page={endPage} prevPage={startPage - 1} nextPage={endPage + 1} totalPages={totalPages}
      getHref={(p) => buildCatalogPageHref(pathname, searchParams, p)}
      onPageChange={scrollToGrid}
      renderLink={({ href, ...rest }) => <Link href={href as Route} scroll={false} {...rest} />} />
    ```
    `Route` comes from `next`, the same cast as `personal-home.tsx`. Remove `handlePageChange` and the old Назад/Вперед block.
  - Keep `updateURL` (with `router.push`, `scroll: false`) and all filter handlers as they are. They already drop `page`.
  - The skeleton renders only when `status === 'initial'`.
- Do not change `SearchBar`. Add a catalog-view test that proves that a `page`-only URL change does not call `onSearch` / reset the page (#459).

## Existing Patterns

- Sqlite repository tests: `backend/internal/modules/catalog/repository/book_status_sqlite_test.go`, which reuses `catalogStatusDDL` from `author_status_sqlite_test.go`. Full-text `search` cannot run on sqlite, so leave search out of the new tests.
- The `buttonVariants` + `cn` styling in `packages/ui/src/components/ui/button.tsx`.
- `as Route` cast for dynamic hrefs: `apps/web/src/widgets/personal-home/ui/personal-home.tsx`.
- Admin URL-driven pagination: `apps/admin/src/views/screen-works/ui/screen-works-view.tsx` (`pushParams`).

## Tests

### Backend (Part 1)

New file `backend/internal/modules/catalog/repository/list_books_sqlite_test.go`, using `catalogStatusDDL` and a fresh fixture:

- `TestListBooks_MultiGenreNoDuplicates`: one book has genres A and B, both published. Filtering by `[A, B]` returns `total == 1` and `len == 1`.
- `TestListBooks_AuthorAndGenreNoDuplicates`: a book by author X with genres A and B. Filtering by author X and genres `[A, B]` returns 1.
- `TestListBooks_DeterministicPaging` (table over `rating`, `new`, `title`): 7 published books with identical `rating`, `created_at`, `published_year` and `title`. Page through with `Limit 3`, `Offset 0/3/6`. The union must equal all 7 IDs with no repeats, and the order must be ascending by `id`. Also include a variant with one `published_year` NULL for `new`, which must come last. SQLite 3.30+ supports `NULLS LAST`.
- Existing sqlite tests stay unchanged and green.

### Frontend

- `packages/ui`, `pagination-range.test.ts`:
  - edge cases `total` 0 / 1 / 7 / 42
  - `current` at the start, middle and end
  - clamping of `current` out of range
  - no single-page ellipsis
  - `siblings = 2`
- `packages/ui`, `pagination.test.tsx`:
  - link mode renders `<a href>` from `getHref`, and the current page is a non-link with `aria-current="page"`
  - button mode calls `onPageChange`
  - disabled prev/next at the edges, in both modes
  - `prevPage` / `nextPage` overrides
  - compact text `5 / 42` is present
  - `renderLink` is used when provided
  - nothing renders at `totalPages <= 1`
- `apps/admin`: the updated `pagination.test.tsx` (see above).
- `apps/web`, `page-href.test.ts`: preserves repeated `genre_id`, `author_name` and `sort`. Page 1 removes `page`.
- `apps/web`, `catalog-view.test.tsx` (mock `booksApi`, `next/navigation`, `next/link`):
  - first load shows skeletons and calls `limit: 24`
  - «Показать ещё» appends the next page, calls `history.replaceState` with `page=2` and does not refetch page 2 on the following URL sync. The paginator marks 2 as `aria-current`, and prev is disabled when `startPage` is 1.
  - the button is hidden on the last page
  - a URL change to another page dims the grid (`aria-busy="true"`) and shows no skeleton
  - clicking a page link calls `scrollIntoView`
  - a stale response is ignored
  - out-of-range page message
  - #459: changing only `page` does not reset it
- Keep `search-bar.test.tsx` green.
- Frontend checks: `pnpm --filter @bookhaven/ui test`, `pnpm --filter @bookhaven/web test`, `pnpm --filter @bookhaven/admin test`, plus `type-check` / `lint`. Do not run a native `next build`; CI is the gate for typedRoutes.

## Documentation

- This file: set `Status: in-progress` / `completed`.
- `docs/features/search-and-filters.md`:
  - Business Rules: note that sort orders end with `id` as the tiebreaker.
  - Business Rules: note that the web catalog page size is 24 (the API default remains 20).
  - Business Rules: note that the genre filter is duplicate-free ANY.
  - UX "Pagination" line: point to this contract.
- No ROADMAP entry exists for this. If the orchestrator tracks it, add one line under the catalog area.

## Risks

- **URL sync loop**: `replaceState` updates `useSearchParams` and fires the sync effect. Without the skip flag the effect would refetch page N alone and wipe the appended list. Tests must cover this.
- **typedRoutes**: `href as Route` in `renderLink` is required. `tsc` does not catch every typedRoutes error, so wait for CI, and do not run a native `next build` on Windows.
- **packages/ui test deps**: these change `pnpm-lock.yaml`. Keep the versions aligned with the apps so that no duplicate React/testing-library copies appear. The component must keep `react` as a peer only.
- **Admin label change**: the visible text changes from «Назад/Вперед» to chevrons with aria-labels. Any admin e2e or text selectors relying on «Назад»/«Вперед» must be updated (grep `apps/admin` for them).
- **EXISTS performance**: `book_genres(book_id, genre_id)` / `book_authors(book_id, author_id)` composite PKs serve the correlated subqueries. This should be no slower than the JOIN. Verify with `EXPLAIN` only if catalog latency regresses.
- **Back button after «Показать ещё»**: because of `replaceState`, Back does not undo appends. This is deliberate (see Business Rules 4) and should be documented in the PR.
