## Context

The public book page is the route `apps/web/src/app/books/[slug]/page.tsx`, which renders the server component `apps/web/src/views/book-detail/ui/book-detail-view.tsx`. Today that view is a `md:grid-cols-[300px_1fr]` grid: the cover in the left column and every block stacked in the right one. `SimilarBooks` follows the grid.

This change is frontend only, in apps/web plus one primitive in packages/ui. There is no API or DB change; every field used (`language`, `reviewsCount`, `aiDescription`, `genres[].id`) is already on `Book` in `@bookhaven/api-client`.

Stack: Next 14.2.6 (App Router, `experimental.typedRoutes: true`), React 18.3, Tailwind 3.4. `@bookhaven/ui` is consumed from source (`main: ./src/index.ts`) and currently ships only `@radix-ui/react-slot`.

**Out of scope (user decision).** `features/add-to-list` (`AddToListButton`) keeps its behaviour and markup. It is only moved into the actions column, and the single change inside it is the login href (decision 7). Lists will be redesigned separately.

## Decisions

1. **Layout via CSS grid areas plus `display: contents` on mobile.** DOM order is: `<aside>` (cover + actions), then `<div>` (header: title / authors / summary), then `<div>` (body: badges / description / details / genres / sections).
   - **Desktop (`md:`).** The grid is `md:grid-cols-[260px_minmax(0,1fr)]` with `md:grid-rows-[auto_1fr]`. The aside is `md:sticky md:top-20 md:self-start md:row-span-2 md:max-h-[calc(100vh-6rem)] md:overflow-y-auto`. Header and body stack in column 2. The body runs to the end of the page (it holds the sections), so the sticky aside stays pinned while the whole page scrolls.
   - **Mobile.** The grid is `grid-cols-[minmax(0,2fr)_minmax(0,3fr)]` (cover ≈ 40%). The aside becomes `max-md:contents`, so the cover goes to row 1 / col 1 and the header to row 1 / col 2. The actions take row 2 across `col-span-2`, the body row 3 across `col-span-2`. No sticky.
   - **Cover.** `next/image` gets `sizes="(max-width: 767px) 40vw, 260px"`.
   - **Title.** The h1 is `text-2xl sm:text-3xl md:text-4xl`.
   - **Sections.** Inside the body (`min-w-0`, so «Похожие книги» scrolls horizontally inside the column), in a `space-y-10` wrapper: `<BookAdaptations>`, `<SimilarBooks>`, `<ReviewsSection>`. Adaptations are a `grid gap-3 sm:grid-cols-2 xl:grid-cols-3` of compact cards.
   - **Actions column.** `BookShelfActions`, then `AddToListPopover`, then `BookRating`. `AddToListPopover` (features/add-to-list) is a thin wrapper: for a guest the same login link as `AddToListButton`; for a signed-in reader a «В список ▾» outline button that opens the unchanged `AddToListButton` in a `PopoverContent` (`w-72 max-h-[60vh] overflow-y-auto`), so the tall list UI does not stretch the pinned column.
   - **Status control.** With a status set, the trigger is a `secondary` button «✓ status ▾» with an explicit ChevronDown, so it reads as a dropdown.
   - **Status icons.** Bookmark (want to read), BookOpen (reading), BookCheck (read), X («Убрать статус», muted, below a divider). The trigger shows the current status icon; the selected menu item is `ghost` with a Check at the right (`aria-pressed` kept).
   - **Lists popover content.** `AddToListButton` is now used only inside the popover, so its look is menu-like: no bordered box, a divider, and a full-width ghost row «+ Новый список» that turns into an autofocused input (Enter creates, Escape cancels the field and does not close the popover; empty input is not sent). Its loading/toggle/create logic is unchanged.
   - **Rating control.** Large stars (`h-8 w-8`), the label «Оцените книгу» / «Моя оценка: N» on its own line, hover preview (`hoverValue`) and `focus-visible` ring; centred on `md:` in the actions column, left-aligned on mobile.
   - **Fallback.** If `contents` misbehaves with `sticky`, an acceptable fallback is to render the actions twice (`md:hidden` / `hidden md:block`). This is not preferred, because each copy would fetch the shelves twice.
2. **Parallel side requests.** `Promise.all([collectionsApi.forBook(id).catch(() => null), screenWorksApi.forBook(id).catch(() => null)])`. Each request keeps its own `.catch`, so one failure never rejects the other.
3. **One book fetch for metadata and view.** `fetcher` uses `cache: 'no-store'`, so Next's fetch dedup does not apply. Add `views/book-detail/api/get-book.ts`:
   ```ts
   const memo = typeof cache === 'function' ? cache : <T,>(fn: T) => fn;
   export const getBook = memo((slug: string) => booksApi.get(slug).catch(() => null));
   ```
   `cache` comes from `react`. The identity fallback keeps vitest working: the non-RSC React build may not export `cache`. If `@types/react` lacks `cache`, `import * as React` and read `(React as { cache?: … }).cache`. Both `BookDetailView` and `generateBookMetadata` call `getBook`.
4. **Metadata and JSON-LD live in the view slice.**
   - `views/book-detail/lib/book-seo.ts` holds pure functions `buildBookMetadata(book): Metadata` and `buildBookJsonLd(book): object`, both unit-tested.
   - `views/book-detail/index.ts` exports `generateBookMetadata({ slug })`. It returns `{ title: 'Книга не найдена — BookHaven' }` when `getBook` is null.
   - `app/books/[slug]/page.tsx` adds `export async function generateMetadata({ params }: PageProps) { return generateBookMetadata(params.slug); }`. Params are a plain object in Next 14.
   - **Open Graph.** Next's `openGraph` gets `type: 'book'`, `isbn` and `authors` (names), and `images: [{ url: coverUrl }]` only when `coverUrl` is absolute (`^https?://`). No `metadataBase` exists, and relative URLs would warn.
   - **JSON-LD.** It is rendered in the view as `<script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(ld).replace(/</g, '\\u003c') }} />`. The `<` escaping is mandatory (XSS via the title or description). Omit the `url` field: there is no site-URL env.
   - **Fields.** `@context`, `@type: 'Book'`, `name`, `author: [{ '@type': 'Person', name }]`, `isbn?`, `numberOfPages?`, `datePublished?` (String(year)), `inLanguage?`, `image?`, `description?` (own description only, never `aiDescription`). `aggregateRating` (`@type: 'AggregateRating'`, `ratingValue`, `ratingCount`, `bestRating: 5`, `worstRating: 1`) is set only when `ratingsCount > 0`.
   - **Meta description.** Own description, whitespace collapsed, cut at about 160 characters on a word boundary plus «…». The fallback is «{title}: оценки, рецензии и экранизации на BookHaven.».
5. **Rating summary is server-rendered in the view.** It lives in a new `views/book-detail/ui/rating-summary.tsx`, which uses the lucide `Star`, `rating.toFixed(1)` and `pluralRu`. Its reviews link is `<a href="#reviews">`, a same-page anchor; a plain `<a>` avoids typedRoutes issues.
   - **Reviews anchor.** `ReviewsSection` gets `id="reviews"` and `scroll-mt-24` on its `<section>`.
   - **Live update after rating.** `BookRating` calls `router.refresh()` (`useRouter` from `next/navigation`) after a successful `api.ratings.set`. The server summary then re-renders, and client state such as shelves and reviews is preserved.
   - **What `BookRating` drops.** It loses its own average and count display and the read-only `StarRating`, because the summary owns them now. `BookRating` is used only on the book page (checked).
   - **Explicit text.** `BookRating` renders the text «Моя оценка: N» once rated, «Моя оценка» before.
6. **Hydration fix.** In `features/rate-book/ui/book-rating.tsx` delete the local `useId` (which uses `Math.random`) and use `React.useId()`. Its output contains `:` characters, so strip them for SVG ids: `useId().replace(/:/g, '')`. This avoids `url(#…)` issues.
7. **Login href helper.** Reuse the existing `loginHrefWithRedirect` in `shared/lib/safe-redirect.ts` (it already encodes via `URLSearchParams`). Add a typed wrapper next to it:
   ```ts
   export function loginHref(redirect: string): Route {
     return loginHrefWithRedirect(redirect) as Route;
   }
   ```
   `Route` comes from `import type { Route } from 'next'`. typedRoutes rejects a plain `string` in `<Link href>`, and `tsc` does not catch it; only CI `next build` does.
   - **Call sites.** Replace the four copies:
     - `book-rating.tsx` (×2);
     - `widgets/reviews-section/ui/reviews-section.tsx`;
     - `features/shelve-book/ui/shelve-book-buttons.tsx`: `loginHref(loginRedirect ?? \`/books/${slug}\`)`. The book-of-the-day card passes `/`, which becomes `/login?redirect=%2F`;
     - `features/add-to-list/ui/add-to-list-button.tsx`: only this line.
8. **Popover primitive in `@bookhaven/ui`.** There is no dropdown or popover today. Add `packages/ui/src/components/ui/popover.tsx`, shadcn-style (`Popover`, `PopoverTrigger`, `PopoverAnchor`, `PopoverContent` with Portal, `align`, `sideOffset=4`, `z-50 rounded-md border bg-popover p-3 shadow-md`), on `@radix-ui/react-popover` (^1.1, React 18-compatible). Export it from `packages/ui/src/index.ts`. Confirm the `bg-popover` token exists in the web Tailwind theme; otherwise use `bg-background`.
   - **Why Radix Popover, not DropdownMenu.** It opens on `click`, so `fireEvent.click` works in tests; DropdownMenu opens on `pointerdown`, which jsdom handles poorly. It handles focus, Escape and outside clicks. One dependency covers both the status chooser and «На полку».
   - **Test setup.** Add a `ResizeObserver` stub to `apps/web/vitest.setup.ts`, and to packages/ui test setup if a primitive test is added there.
   - **Status chooser semantics.** The status chooser is a popover with a list of buttons using `aria-pressed`; there is no `role="menu"`, so no menu keyboard model is owed.
9. **Shelves: shared hook, two presentations.**
   - **Hook.** Extract the state and handlers from `ShelveBookButtons` into `features/shelve-book/model/use-book-shelves.ts`. It owns `shelves`, `status: 'loading' | 'ready' | 'error'`, `error`, `toggleDefault(shelf)`, `toggleCustom(shelf)` and `clearStatus()`, with the existing optimistic update and rollback logic. The initial status is `'loading'` when authenticated, so there is no empty first frame.
   - **Existing component.** `ShelveBookButtons` keeps its props and behaviour (used by `widgets/book-of-the-day`). It only gains the skeleton while loading (`h-10` blocks with `animate-pulse rounded-md bg-muted`, wrapper `aria-busy="true"`, sr-only «Загрузка полок») and the encoded login link.
   - **New component.** Add `features/shelve-book/ui/book-shelf-actions.tsx` (the status popover is anchored to its trigger: wrapping the group in `PopoverAnchor asChild` stopped the click from opening it), `BookShelfActions({ bookId, slug })`, exported from the feature index. It renders:
     - **Status split control.** One controlled `Popover` with a `PopoverAnchor` around a button group.
       - Without a status: a primary `Button` «Хочу прочитать» (calls `toggleDefault(want_to_read)`) plus an icon `PopoverTrigger` (ChevronDown, `aria-label="Выбрать статус"`).
       - With a status: a single default-variant trigger showing a Check icon and the status label (`aria-label="Статус: Читаю. Изменить"` or similar).
       - Content: three buttons with `aria-pressed`. Picking the current one calls `toggleDefault`, which removes it, matching the spec rule. When a status is set, the content also has «Убрать статус». Any pick closes the popover.
     - **«На полку».** A `Popover` with a `Button variant="outline"` trigger «На полку», showing a count suffix when the book is on N > 0 custom shelves, e.g. «На полку · 2». The content has the heading «Добавить на полку» and the checkbox list (it stays open). It is not rendered when there are no custom shelves.
     - **Error.** The same red box as today.
     - **Guest.** One outline link button «Хочу прочитать» to `loginHref('/books/' + slug)`.
     - **Loading.** Two skeleton blocks.
   - **Status removal.** `clearStatus()` removes the current default shelf: `removeBook` on the shelf where `hasBook`.
10. **Description collapse is a client island.** Add `views/book-detail/ui/book-description.tsx` (`'use client'`), with props `text`, `isAI`.
    - **Text.** The `<p>` gets `whitespace-pre-line`.
    - **Long text.** It is long when `text.length > 400 || (text.match(/\n/g)?.length ?? 0) > 4`. This is a deterministic, SSR-safe heuristic: there is no layout measurement, so there is no hydration flash.
    - **Collapsed state.** When long and collapsed the text gets `line-clamp-6`, plus a link-styled button «Ещё» / «Свернуть» with `aria-expanded`.
    - **AI notice.** It is rendered after the text in both states.
11. **Book details.** A `<dl className="grid grid-cols-2 gap-4 sm:grid-cols-4">` whose children are `<div><dt/><dd/></div>` groups (valid HTML: `div` is allowed inside `dl`). The language comes from a new `shared/lib/language-name.ts`, `languageNameRu(code)`:
    ```ts
    new Intl.DisplayNames(['ru'], { type: 'language' }).of(code)
    ```
    It uppercases the first letter, wraps the call in try/catch, returns the code itself on failure or an empty result, and returns null for an empty code (the row is then omitted). The function is called in the server component, so there is no hydration risk.
12. **Genres.** `<Link href={\`/books?genre_id=${genre.id}\`}>` wrapping `<Badge variant="outline" className="hover:bg-muted">`. The footer already uses this exact href form, which passes typedRoutes.
13. **Plural helper.** Add `shared/lib/plural-ru.ts`, `pluralRu(n, ['оценка', 'оценки', 'оценок'])`, using the standard mod-10 / mod-100 rule. Do not refactor the existing local `pluralizeBooks` copies (out of scope).
14. **Heading style.** Change the `ReviewsSection` h2 from `text-2xl font-bold` to `font-display text-xl font-semibold` and remove its `mt-10`, because spacing now comes from the wrapper. Remove `BookAdaptations`' outer `mt-8` only if spacing doubles; the adaptations test should not depend on it.

## Frontend

FSD placement:
- **app.** `app/books/[slug]/page.tsx` gets `generateMetadata`.
- **views.** `views/book-detail`:
  - `ui/book-detail-view.tsx` (layout, JSON-LD script);
  - `ui/rating-summary.tsx`;
  - `ui/book-description.tsx`;
  - `lib/book-seo.ts`;
  - `api/get-book.ts`;
  - `index.ts` exports `BookDetailView` and `generateBookMetadata`.
- **widgets.** `reviews-section` gets the id, the heading style and the `loginHref`.
- **features.**
  - `shelve-book`: the hook, `BookShelfActions`, the `ShelveBookButtons` skeleton and `loginHref`.
  - `rate-book`: `useId`, removal of the summary, `router.refresh` and `loginHref`.
  - `add-to-list`: the login-href line only.
- **shared.** `lib/plural-ru.ts`, `lib/language-name.ts`, and `loginHref` in `lib/safe-redirect.ts`.
- **packages/ui.** `Popover`.

No cross-feature imports: the view composes `BookShelfActions`, `AddToListButton` and `BookRating` side by side.

## Existing Patterns

- Server view with per-request `.catch(() => null)` degradation: the current `book-detail-view.tsx`.
- A same-shape view test that mocks `@/entities/book`, `next/image` and the client features: `book-detail-view.test.tsx`.
- Genre-filter href: `widgets/footer/ui/footer.tsx` and `views/collection-detail/ui/collection-detail-view.tsx`.
- Encoded login href: `widgets/header/ui/header-login-link.tsx` and `shared/lib/safe-redirect.ts`.
- shadcn-style primitive wrapping Radix: `packages/ui/src/components/ui/button.tsx` (Slot).
- Section heading style: `entities/screen-work/ui/book-adaptations.tsx` and `widgets/similar-books/ui/similar-books.tsx`.

## Risks

- **typedRoutes.** `tsc` does not catch `<Link href>` route-type errors, and native `next build` is forbidden locally. Keep every new href either a template literal on a known route or cast to `Route`. The CI build is the gate; since this work stays local, flag it for the first CI run.
- **`cache` from `react`.** Typing or availability may differ between Next's bundled React and vitest; decision 3 has a fallback. Worst case is two book fetches per request (acceptable).
- **Radix in jsdom.** It needs a `ResizeObserver` stub; portal content renders into `document.body`, so query with `screen`, not `within(container)`.
- **`display: contents` + grid + sticky.** Check visually at 375 px and 1280 px. The fallback is in decision 1.
- **Auth store hydration.** `isAuthenticated` starts false before the store hydrates, so guests' «Хочу прочитать» can flash for a signed-in user. This is pre-existing and not fixed here; the skeleton covers only the post-auth fetch.
- **Behaviour change in `shelve-book-buttons.test.tsx`.** The expected login hrefs become encoded (`%2Fbooks%2Fbook-1`, `%2F`), and the loading state now renders a skeleton.
- **Lockfile.** Adding `@radix-ui/react-popover` changes `pnpm-lock.yaml`. Run `pnpm install` from the repo root only.
