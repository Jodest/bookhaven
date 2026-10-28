# Tasks

## 1. Frontend: compact screen-work card (apps/web, entities/screen-work)
- [x] 1.1 Change `ScreenWorkPoster` to thumbnail size: `sizes="72px"`, a smaller placeholder icon and year. Verify that the existing poster and placeholder tests in `book-adaptations.test.tsx` and `author-films.test.tsx` still pass after their expectations are updated.
- [x] 1.2 Add the shared horizontal `ScreenWorkCard`: a 72 px thumbnail on the left, and on the right the title, the `·` metadata line, the extra slot and a compact `PosterAttribution`. Verify with a unit test for the Commons caption, the links, the placeholder and alt text.
- [x] 1.3 Add the client `CollapsibleCardList` (first 4 visible, «Показать все (N)», `aria-expanded`/`aria-controls`, focus moves to the 5th card, button removed). Verify with tests: 6 items show the button «Показать все (6)», and clicking it shows all 6 and focuses the 5th. 4 items have no button. All 6 titles are in the initial render output.
- [x] 1.4 Rework `BookAdaptations`: visible heading, `scroll-mt-24` on `#adaptations`, `intro` under the heading, the cards through `ScreenWorkCard` + `CollapsibleCardList`, grid `grid-cols-1 lg:grid-cols-2`, and no `headingHidden`. Verify `book-adaptations.test.tsx`.
- [x] 1.5 Rework `AuthorFilms` to `ScreenWorkCard` with year/kind/«Документальный»/director/links on one line, grid `md:grid-cols-2 xl:grid-cols-3`, and no show-all button. Verify `author-films.test.tsx` and `author-detail-view.test.tsx`.

## 2. Frontend: similar books row (apps/web, widgets/similar-books)
- [x] 2.1 Replace `BookCard` in the row with a compact item (a `w-24 sm:w-28` 2:3 cover, a two-line title, a link to the book) and resize the skeleton to match. Remove `emptyFallback` and `headingHidden`, and return `null` on empty or error.
- [x] 2.2 Make the `<ul>` focusable: `tabIndex={0}`, `aria-labelledby` the heading, a visible focus ring. Verify `similar-books.test.tsx`: it renders a list with the heading «Похожие книги», the list is focusable, an empty result or error renders nothing, and the text «Похожих книг пока нет.» is gone.

## 3. Frontend: book page without tabs (apps/web, views/book-detail + app route)
- [x] 3.1 Delete `book-tabs.tsx`, `book-tabs.test.tsx`, `book-tab-link.tsx` and `lib/book-tabs.ts`. Remove `headingHidden` from `ReviewsSection`.
- [x] 3.2 `BookDetailView`: drop the `tab` prop and render `BookAdaptations` → `SimilarBooks` → `ReviewsSection` stacked. The header link is `<a href="#adaptations">` with the icon and plural text. `RatingSummary` links to `#reviews`.
- [x] 3.3 `app/books/[slug]/page.tsx`: remove `searchParams`. Confirm nothing else reads `tab`, using `grep "tab"` in views/book-detail and app/books. Do not add `dynamic` or `revalidate`, because the page stays dynamic through its `no-store` fetches. Update the `book-seo.ts` comment, and keep the canonical test asserting the book URL without query.
- [x] 3.4 Update `book-detail-view.test.tsx`: the three sections are present in order with visible headings and no `role="tablist"`. The header link has `href="#adaptations"` and is absent without adaptations. The review count link has `href="#reviews"`.
- [x] 3.5 Run `pnpm --filter web exec tsc --noEmit`, `pnpm --filter web lint` and the web vitest suite for the touched folders. No native `next build`, because CI is the typedRoutes gate.

## 4. Docs
- [x] 4.1 Update the adaptations epic entry in `docs/ROADMAP.md`. It currently describes the tabs (`?tab=`). Replace that with stacked sections, compact horizontal adaptation cards with «Показать все (N)», the similar-books row and the `#adaptations` header anchor, and reference the change `restack-book-page-sections`.
- [x] 4.2 Run `npx -y @fission-ai/openspec@1.13.2 validate restack-book-page-sections --strict` and confirm it passes.
