# Tasks

## 1. Shared helpers and UI primitive

- [x] 1.1 Add `pluralRu(n, forms)` in `apps/web/src/shared/lib/plural-ru.ts` with `plural-ru.test.ts`. Verify that 1, 2, 5, 11, 12, 14, 21, 22, 25, 111 and 0 give the right forms for «оценка/оценки/оценок».
- [x] 1.2 Add `languageNameRu(code)` in `apps/web/src/shared/lib/language-name.ts` with a test. Verify: `ru` gives «Русский», `en` gives «Английский», an unknown code comes back as is, and an empty code gives null.
- [x] 1.3 Add the typed `loginHref(redirect): Route` wrapper in `apps/web/src/shared/lib/safe-redirect.ts` and extend `safe-redirect.test.ts`. Verify that `/books/abc` gives `/login?redirect=%2Fbooks%2Fabc` and `/` gives `/login?redirect=%2F`.
- [x] 1.4 Add `@radix-ui/react-popover` to `packages/ui` (run `pnpm install` from the repo root). Create `packages/ui/src/components/ui/popover.tsx` (`Popover`, `PopoverTrigger`, `PopoverAnchor`, `PopoverContent`) and export it from `packages/ui/src/index.ts`. Add a `ResizeObserver` stub to `apps/web/vitest.setup.ts`. Verify with `tsc --noEmit` and lint in `packages/ui`.

## 2. Quick fixes in existing features and widgets

- [x] 2.1 `features/rate-book/ui/book-rating.tsx`:
  - replace the local `useId` with `React.useId()` (colons stripped);
  - remove the average and count display and `StarRating`;
  - render «Моя оценка: N» once rated;
  - call `router.refresh()` after a successful save;
  - use `loginHref` for both guest links.

  Add `book-rating.test.tsx`: clicking «Оценить на 4» calls the set API and shows «Моя оценка: 4»; a failure restores the previous value and shows «Ошибка сохранения оценки»; the guest link has an encoded redirect; the star `clipPath` ids are stable across two renders (no `Math.random`).
- [x] 2.2 `widgets/reviews-section/ui/reviews-section.tsx`:
  - add `id="reviews"` and `scroll-mt-24` to the section;
  - change the h2 to `font-display text-xl font-semibold` and drop `mt-10`;
  - use `loginHref` for «Войдите, чтобы написать рецензию».

  Update or add its test for the encoded href.
- [x] 2.3 `features/add-to-list/ui/add-to-list-button.tsx`: change ONLY the guest link to `loginHref(\`/books/${slug}\`)`. No other change to lists (user decision).

## 3. Shelves: hook, skeleton, book-page actions

- [x] 3.1 Extract `useBookShelves(bookId)` into `features/shelve-book/model/use-book-shelves.ts`. Keep the existing optimistic update and rollback logic. Start in `loading` when authenticated, and add `clearStatus()`.
- [x] 3.2 Refactor `ShelveBookButtons` onto the hook. Keep the props and behaviour unchanged; add the loading skeleton (`aria-busy`, sr-only «Загрузка полок»); use `loginHref`. Update `shelve-book-buttons.test.tsx`: the hrefs become encoded, and add a loading-skeleton case. `widgets/book-of-the-day` tests must still pass unchanged.
- [x] 3.3 Add `BookShelfActions` in `features/shelve-book/ui/book-shelf-actions.tsx` and export it from the feature index. It has the status split control (Popover), «На полку» (Popover with the checklist «Добавить на полку»), the skeleton, the error box and the guest link. Add `book-shelf-actions.test.tsx` covering:
  - with no status, «Хочу прочитать» adds to want_to_read;
  - «Выбрать статус», then «Читаю», moves the status and closes the popover;
  - with a status, «Убрать статус» removes it;
  - picking the current status removes it;
  - «На полку» opens the checklist and a toggle keeps it open;
  - «На полку» is absent without custom shelves;
  - a failure rolls back and shows «Ошибка обновления полки»;
  - loading shows the skeleton;
  - the guest sees one encoded login link and no «На полку».

## 4. Book page view

- [x] 4.1 Add `views/book-detail/api/get-book.ts` (`getBook`, wrapped in React `cache` with an identity fallback).
- [x] 4.2 Add `views/book-detail/lib/book-seo.ts` (`buildBookMetadata`, `buildBookJsonLd`) and `book-seo.test.ts`. Cover:
  - the title with and without authors;
  - the description cut at about 160 characters with «…»;
  - the fallback text when there is only an AI description;
  - og image only for an absolute `coverUrl`;
  - `aggregateRating` only when `ratingsCount > 0`;
  - no `description` from `aiDescription`.
- [x] 4.3 Add `ui/rating-summary.tsx` («★ 4.3 · 21 оценка · 3 рецензии», «Пока нет оценок», «Написать рецензию», link `#reviews`) and `ui/book-description.tsx` (`whitespace-pre-line`, the long-text heuristic with `line-clamp-6`, «Ещё»/«Свернуть» with `aria-expanded`, the AI notice).
- [x] 4.4 Rewrite `ui/book-detail-view.tsx`:
  - the layout per design decision 1 (sticky aside on desktop; 40% cover beside the header on mobile; no cover rating badge);
  - `Promise.all` for collection places and adaptations;
  - the actions column order: `BookShelfActions`, `AddToListButton`, `BookRating`;
  - the `<dl>` with «Год издания», «Страниц», «Язык», «ISBN»;
  - genre links to `/books?genre_id=<id>`;
  - full-width «Экранизации», then `SimilarBooks`, then `ReviewsSection`;
  - the JSON-LD script with `<` escaped.

  Export `generateBookMetadata` from `views/book-detail/index.ts`.
- [x] 4.5 `app/books/[slug]/page.tsx`: add `generateMetadata` delegating to `generateBookMetadata(params.slug)`.
- [x] 4.6 Update `book-detail-view.test.tsx`. Mock `BookShelfActions`, and keep the existing badge, author, description and adaptation cases passing. Add cases for:
  - both side requests failing, with the page still rendering;
  - both side requests being started before either resolves (parallel);
  - no rating badge on the cover;
  - the summary plural forms and the `#reviews` link;
  - the `<dl>` containing «Язык» «Русский» and omitting missing terms;
  - the genre link href `/books?genre_id=g1`;
  - the long description showing «Ещё», with expand and collapse;
  - paragraphs kept (a `whitespace-pre-line` class on the text);
  - the AI notice still shown;
  - the section order «Экранизации» → similar → «Рецензии» (DOM order via `compareDocumentPosition`);
  - JSON-LD present with `@type: Book` and no `aggregateRating` when there are 0 ratings.

## 5. Verification and docs

- [x] 5.1 From `apps/web`: `pnpm exec tsc --noEmit`, `pnpm exec next lint`, `pnpm exec vitest run`. From `packages/ui`: `pnpm exec tsc --noEmit` and lint. Do NOT run a native `next build`; typedRoutes is gated by the CI build, so note that the first CI run must be watched.
- [x] 5.2 Manually check `/books/<slug>` in the dev server at about 375 px and about 1280 px:
  - the sticky aside;
  - the mobile cover beside the title;
  - both popovers (open, Escape, outside click);
  - no hydration warning in the console;
  - the page source has the `<title>`, `og:image` and the JSON-LD.
- [x] 5.3 Docs:
  - add `catalog` to the "Existing domains" line in `openspec/config.yaml` `context`;
  - in `docs/ROADMAP.md`, note on item #199 that book-page metadata is done (author and profile pages, sitemap and robots remain).

## 6. Review feedback iteration

- [x] 6.1 Status trigger keeps a visible ChevronDown when a status is set (`secondary` variant); test added.
- [x] 6.2 Adaptations as a compact card grid (`sm:grid-cols-2 xl:grid-cols-3`).
- [x] 6.3 «Моя оценка»: large stars, «Оцените книгу» label, hover preview, focus ring; test added.
- [x] 6.4 Two columns to the end of the page: sections move into the right column, the left column is sticky with its own max height; `min-w-0` on the content column; view tests for placement and order.
- [x] 6.5 `AddToListPopover` («В список ▾») in the sticky column, wrapping the unchanged `AddToListButton`; tests for guest link and signed-in popover.
- [x] 6.6 JSON-LD escaping fixed (`<`, U+2028, U+2029) with a test.- [x] 6.7 Lists popover: «+ Новый список» row with inline input (Enter / Escape), no bordered box; tests in `add-to-list-button.test.tsx`.
- [x] 6.8 Status icons on the trigger and menu items, check mark on the selected item, muted «Убрать статус» under a divider; test added.
- [x] 6.9 «Моя оценка» centred on `md:`.
