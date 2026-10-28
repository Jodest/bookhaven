# Design

## Context
This change is frontend-only in `apps/web`, with no API or DB change. It reverts the tabbed lower part of the book page from `add-adaptation-posters-and-tabs` to stacked sections, and it shrinks the adaptation/film cards to a horizontal compact layout. Several FSD layers are touched: views/book-detail, entities/screen-work, widgets/similar-books, widgets/reviews-section and views/author-detail.

## Decisions
1. **Delete the tab machinery outright.**
   - Remove `views/book-detail/ui/book-tabs.tsx`, `book-tabs.test.tsx`, `book-tab-link.tsx` and `lib/book-tabs.ts`, plus the `BOOK_TAB_EVENT` window event.
   - `BookDetailView` loses the `tab` prop and renders the three sections directly inside the right column `space-y-6` block, keeping the `pt-4` spacer before the first section: `BookAdaptations` → `SimilarBooks` → `ReviewsSection`.
2. **Page route.**
   - `app/books/[slug]/page.tsx` drops `searchParams` entirely.
   - Rendering mode check: the page stays dynamic anyway, because `getBook` and the side requests go through the API fetcher with `no-store`. Removing `searchParams` therefore does not make it static, and that is fine. Do not add `export const dynamic` or `revalidate`.
   - A leftover `?tab=` is simply ignored.
   - `book-seo.ts` keeps the canonical `${site}/books/${slug}`. Only update its comment, which mentions `?tab=`.
3. **Anchors instead of tab links.**
   - `RatingSummary` uses a plain `<a href="#reviews">`. `#reviews` already exists on `ReviewsSection` with `scroll-mt-24`, and that section is always rendered, so the native jump works.
   - The header link becomes `<a href="#adaptations">` with the clapperboard icon and plural text.
   - `BookAdaptations` keeps `id="adaptations"` on its `<section>` and gets `scroll-mt-24`.
   - No JS scrolling is needed. If smooth scrolling is wanted, rely on CSS `scroll-behavior` respecting `prefers-reduced-motion`. Do not add a click handler.
4. **Shared compact card: `entities/screen-work/ui/screen-work-card.tsx`** (server component, exported only inside the slice).
   - Props: `title`, `kind`, `year`, `poster`, `metaParts: ReactNode[]` (or explicit `director`, `kinopoiskUrl`, `imdbUrl`, `documentary?`), `children` (the extra slot for the verdict and vote).
   - Layout: `<div className="flex gap-3">`. On the left, `<div className="w-[72px] shrink-0"><ScreenWorkPoster …/></div>`. On the right, `<div className="min-w-0 flex-1">` with the title (`font-medium leading-tight`, `line-clamp-2`), the `·`-separated wrapping metadata line (`text-sm text-muted-foreground`, built the way `BookAdaptations` builds it today), `{children}`, and `<PosterAttribution className="mt-1 text-[11px] leading-tight" />` last.
   - `BookAdaptations` and `AuthorFilms` both render their `<li>`s through it. `AuthorFilms` folds year, kind, «Документальный», «Режиссёр» and the links into that single metadata line.
5. **`ScreenWorkPoster` at thumbnail size.**
   - The frame stays `aspect-[2/3] w-full` and the parent sets the width to 72 px.
   - Use `sizes="72px"`. For site paths, next/image then serves at most a 2x DPR file.
   - The placeholder icon goes from `h-10 w-10` to `h-6 w-6`, with the year in `text-xs`, `gap-1` and `p-1`.
   - Absolute Commons/TMDB URLs stay `unoptimized`.
6. **Grid.**
   - Book page: `grid grid-cols-1 gap-4 lg:grid-cols-2`. The right column at `md` is only about 400 px wide, so a second column starts at `lg`.
   - Author page (full-width section): `grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3`.
7. **«Показать все (N)» with all cards in the SSR HTML.**
   - `BookAdaptations` stays a server component, because `renderExtra` is a function prop and cannot cross into a client component. It renders every `<li>` and wraps the `<ul>` in a small client component `entities/screen-work/ui/collapsible-card-list.tsx` that receives `children` (the server-rendered `<li>` nodes), `total` and `initialVisible = 4`.
   - The client component owns `expanded` state and renders `<ul id=… data-expanded={expanded} className="group …">`. Items with index ≥ 4 get, on the server, the class `hidden group-data-[expanded=true]:flex` (or an equivalent nth-child arbitrary variant on the `<ul>`).
   - When `total > 4 && !expanded` it renders `<Button variant="outline" size="sm" aria-expanded={false} aria-controls={listId}>Показать все ({total})</Button>`.
   - On click, set `expanded` and focus the 5th `<li>`. Give the `<li>` items `tabIndex={-1}` and `outline-none`, or focus its title, to avoid focus loss when the button unmounts.
   - The hidden cards are `display:none` but present in the HTML, which satisfies the SEO requirement. Without JS the button does nothing, which is acceptable.
   - `AuthorFilms` does not use the collapsible list.
8. **Heading and intro.**
   - Remove `headingHidden` from `BookAdaptations`, `SimilarBooks` and `ReviewsSection`, because all headings are visible again with the shared `font-display text-xl font-semibold` style.
   - `intro` (`AdaptationVotePrompt`) stays right under the heading, above the list.
   - `renderExtra` (`AdaptationVerdict`) goes into the card's `children` slot.
9. **Similar books row (`widgets/similar-books`).**
   - Remove the `emptyFallback` and `headingHidden` props. Empty results and errors return `null`, as before the tabs.
   - Replace `BookCard`, which is large and has a rating badge and genres, with an inline compact item: a `Link` to `/books/{slug}` containing a `w-24 sm:w-28` 2:3 cover (`next/image`, `sizes="112px"`, «Нет обложки» fallback in `text-xs`) and a `line-clamp-2 text-sm font-medium` title.
   - Keep `BookCard` itself unchanged, because other pages use it.
   - Make the `<ul>` `flex gap-3 overflow-x-auto snap-x pb-2`, plus `tabIndex={0}`, `aria-labelledby={HEADING_ID}` and a visible `focus-visible:ring`. The axe rule scrollable-region-focusable is then satisfied, and the arrow keys scroll it.
   - Match the skeleton to the new item size to limit layout shift.
   - Keep `SIMILAR_LIMIT = 8` and the query key.

## Frontend
- `views/book-detail`: `book-detail-view.tsx` (sections, header anchor), `rating-summary.tsx` (`#reviews` anchor), and removal of the tab files. Update the comment in `lib/book-seo.ts`, and the test in `lib/book-seo.test.ts` if it mentions `tab`.
- `app/books/[slug]/page.tsx`: no `searchParams`.
- `entities/screen-work/ui`: the new `screen-work-card.tsx` and `collapsible-card-list.tsx`, plus changes to `book-adaptations.tsx`, `author-films.tsx` and `screen-work-poster.tsx`. The `index.ts` exports are unchanged unless something new is needed outside the slice.
- `widgets/similar-books/ui/similar-books.tsx` and `widgets/reviews-section/ui/reviews-section.tsx` (prop cleanup).
- FSD: the entity gets no feature imports. The verdict and prompt are still injected by the view through `renderExtra` and `intro`.

## Existing Patterns
- The `·`-separated metadata line is in the current `book-adaptations.tsx`.
- The anchor with `scroll-mt-24` is on `reviews-section.tsx`.
- The horizontal snap row is the current `similar-books.tsx`.
- CSS-hidden surplus items that stay in the HTML follow `entities/book/ui/book-row.tsx` (`visibilityClass`).

## Risks
- Layout shift: `SimilarBooks` loads on the client above «Рецензии». When it resolves empty it collapses, so an initial `#reviews` jump can land slightly off. The matched skeleton height keeps the shift to that empty case.
- Tailwind `group-data-[…]` needs Tailwind ≥ 3.2. If it is unavailable, use the nth-child arbitrary variant or a conditional class on the `<ul>`.
- Narrow cards: on 320 px phones the title and metadata wrap to several lines. That is acceptable, because the thumbnail fixes the minimum height of a card.
