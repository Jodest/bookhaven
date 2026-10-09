# Book Moderation UI

Status: done

Issue: #317 (F4, the last task of epic #310). Frontend only (`apps/admin`). No backend or api-client changes.

Builds on:
- F2 `widgets/moderation-queue` ([moderation-queue-widget.md](moderation-queue-widget.md), "F4 validation sketch");
- B3 `GET /admin/books/moderation` ([book-moderation-queue.md](book-moderation-queue.md));
- F3 `features/moderate-book` `useBookStatusMutation`, `shared/lib` `buildEditHref`/`safeBackHref`/flash ([catalog-unified-lists.md](catalog-unified-lists.md)).

## Goal

Rebuild `/moderation/books` (`views/book-moderation`) as a thin composition over `ModerationQueue`. The book queue then matches the author and genre queues: status tabs, `source` filter, selection with bulk actions, per-row errors, and "possibly the same book" hints. Also fix the F3 stale-flash bug: a "book saved" flash that is never consumed after the edit page returns to the moderation queue.

## User Stories

- As an admin, I review pending books (manual and harvested) with their authors, genres, source and duplicate hints, and publish or reject them one by one or in bulk.
- As an admin, I look through published or rejected books, filtered by source, and move them between statuses.
- As an admin, I open a book (or a hinted duplicate) for editing. After saving or cancelling, I land back in the queue with the same tab, filters, search and page, and I see the "saved" message there and nowhere else.

## Acceptance Criteria

- [x] `views/book-moderation/ui/book-moderation-view.tsx` renders through `ModerationQueue` from `@/widgets/moderation-queue`. It has no shell logic of its own: no URL parsing, selection, feedback, table frame or pagination. Target size is under about 200 lines.
- [x] Data comes from `adminBooksApi.moderationQueue` with `queryKey ['admin','books','moderation']`. `adminBooksApi.list` is no longer used by this view.
- [x] `statusMutation = useBookStatusMutation()` from `@/features/moderate-book`. No new mutation hook.
- [x] Status tabs, search, the `source` filter (`Источник: любой` / `Вручную` / `Харвестер`), selection, bulk and row status buttons, feedback banner, pagination and empty/error states all come from the widget (Business Rules).
- [x] Columns as in the Frontend section. The title links to `/catalog/books/{id}/edit?back=<current queue URL>`, and the hint titles link the same way.
- [x] `h1` is `Модерация книг`. The header button "Опубликовать все на странице (N)" is **removed**: select-all plus `Опубликовать выбранные (N)` replaces it.
- [x] Flash fix: after saving a book whose edit page returns to `/moderation/books`, the queue shows `Книга «X» сохранена.` once. The message no longer stays in sessionStorage to pop up later in `/catalog/books`. Author and genre queues behave exactly as before.
- [x] Existing `author-moderation-view.test.tsx`, `genre-moderation-view.test.tsx` and `moderation-queue.test.tsx` stay unchanged and pass. New widget tests are added only for the flash prop.
- [x] Dashboard «Книги на модерации», nav counters (`moderation.books` = `pending_books`), the shell/section-tabs `Модерация` link, the harvest-jobs links and the books-list «Очередь модерации» link still point to `/moderation/books`. Their tests are unchanged.
- [x] `tsc --noEmit`, `./node_modules/.bin/next lint` (FSD boundaries) and `vitest run` pass in `apps/admin`. Do **not** run a native `next build`; CI is the typedRoutes gate.

## Business Rules

1. **Shell behaviour** comes from widget rules 1–12 ([moderation-queue-widget.md](moderation-queue-widget.md)): URL state, tabs, selection, bulk/row flows, banner and states. The book view only supplies config.
2. **Source filter.** `{param: 'source', label: 'Источник', options: [{'' ,'Источник: любой'}, {'manual','Вручную'}, {'harvested','Харвестер'}]}`. Any other URL value is dropped by the widget, so the backend never receives it (it would return 400).
3. **buildParams** returns `{status, source: filters.source as AdminBookQueueParams['source'], search, page, limit}` (typed cast, as in the author view).
4. **Identity.** `getId = i => i.book.id`, `getName = i => i.book.title`, `getStatus = i => i.book.status`.
5. **Copy.** The helpers live in `features/moderate-book/lib/confirm-text.ts` and are exported from the slice index, mirroring `features/moderate-author`.
   - `bookStatusConfirmText(status, title)`:
     - published: `Опубликовать книгу «X»?` (unchanged from the old page);
     - rejected: `Отклонить книгу «X»?` (unchanged);
     - pending: `Вернуть книгу «X» на модерацию?`.
   - `bulkBookStatusConfirmText(status, count)`:
     - `Опубликовать выбранные книги (N)?`;
     - `Отклонить выбранные книги (N)?`;
     - `Вернуть выбранные книги на модерацию (N)?`.
   - `bulkBookStatusSummary(report, names)`: `Обновлено X из N.`, plus ` Ошибок: K (<sample>).` when there are errors. The sample is the first 3 errors as `«${e.title || names[e.book_id] || e.book_id}»: ${e.message}`, joined with `; `. This is the same shape as `bulkAuthorStatusSummary`.
   - Row success messages (in the view):
     - `Книга «X» опубликована.`
     - `Книга «X» отклонена.`
     - `Книга «X» возвращена на модерацию.`
   - title `Модерация книг`;
   - placeholder `Поиск по названию или ISBN`;
   - loadError `Не удалось загрузить очередь книг.`;
   - emptyPending `Нет книг на модерации`;
   - emptyFiltered `Книги не найдены`;
   - statusErrorFallback `Не удалось изменить статус книги.`
6. **No merge, no bulk extras, no overlay.** Books cannot be merged. `renderBulkActions` and `renderOverlay` are omitted. `renderRowActions` is omitted too, because the title link replaces the old «Редактировать» button. Row actions are therefore only the widget's status buttons.
7. **Edit link with back.**
   - The view calls `usePathname()`/`useSearchParams()` itself. Views may do this, and the widget does not expose them.
   - Edit href: `buildEditHref(\`/catalog/books/${id}/edit\`, pathname, searchParams)`, rendered as `Link href={… as any}`. Keep the typedRoutes cast.
   - `book-edit-view` already allows `/moderation/books` in `ALLOWED_BACK`. No change is needed there.
8. **Flash fix (generic, minimal).**
   - Add an optional prop to `ModerationQueue`: `flashKey?: string`. When it is set, a mount effect (`[flashKey]`) calls `consumeFlash(flashKey)` and shows the message as a `success` banner. This is the same code as `widgets/catalog-list`.
   - When it is unset, nothing is read, so author/genre stay as they are: they don't pass it.
   - The book view passes `flashKey="catalog.books"`. The key now means "book-saved flash, consumed by whichever book list the edit page returns to". `book-edit-view` and `flash.ts` are unchanged.
   - Rejected alternatives:
     - keying the flash by target path: this needs changes to every edit view and the catalog-list;
     - skipping `setFlash` for non-catalog back URLs: the moderation queue would then lose the saved message.
9. **Authorization.** Unchanged: `/moderation/*` is admin-only through the existing route guard and shell. The backend enforces admin on both endpoints.

## UX

Widget layout (h1, tabs, search + `Источник` select, banner, bulk bar, error, table + pagination). Table columns, in order:

| Header | Content |
|---|---|
| `Книга` | Title as a `Link` with `className` including `font-medium` (edit href with back), then `BookStatusBadge`. Below it, a muted line `ISBN · год` (only the parts that exist). No cover thumbnail (no `<img>` pattern in admin yet; `next lint` `no-img-element`). |
| `Авторы` | One entry per author: name, plus `AuthorStatusBadge` when `status !== 'published'`. `—` (muted span) when empty. Plain text, no links. |
| `Жанры` | Same as authors, with `GenreStatusBadge`. |
| `Источник` | `external_source == null` → `Вручную`. Otherwise `external_source` plus `external_id` (`font-mono text-xs`, when present) below it. |
| `Добавлен` | `moderationCreatedAtColumn(i => i.created_at)` |
| `Возможно, та же книга` | `—` when `similar` is empty. Otherwise a list: hint title `Link` (edit href with back), `(тот же ISBN)` / `(то же название и автор)`, `BookStatusBadge` (all statuses, since a rejected duplicate is informative), and ISBN/year muted when present. |

Cell classes follow the author view (`align-top`, `align-top text-sm`).

## API

None new. This feature uses `api.admin.books.moderationQueue(AdminBookQueueParams)` and `bulkStatus` (via `useBookStatusMutation`). Types: `AdminBookQueueItem`, `AdminBookHint`, `AdminBookQueueParams`, `AdminBookBulkStatusReport`.

## Database

None.

## Frontend

Changed:
- `views/book-moderation/ui/book-moderation-view.tsx`: full rewrite as a composition (rules 2–8). Small internal components `BookPeopleCell` (generic list of `{id,name,status}` + badge render prop), `SourceCell` and `BookHints` may stay in the view file.
- `features/moderate-book/lib/confirm-text.ts` (new) and `features/moderate-book/index.ts`: export `bookStatusConfirmText`, `bulkBookStatusConfirmText`, `bulkBookStatusSummary`. The hook is unchanged.
- `widgets/moderation-queue/model/types.ts`: `flashKey?: string` on `ModerationQueueProps`, with a doc comment.
- `widgets/moderation-queue/ui/moderation-queue.tsx`: the consume-flash effect. Import `consumeFlash` from `@/shared/lib`.
- `entities/book`: add `BOOK_MATCH_LABELS` only if it is needed elsewhere. Otherwise the match labels are a view-local constant (preferred).

Unchanged: the route `app/moderation/books`, `views/book-edit`, `shared/lib/flash.ts`, `widgets/catalog-list`, the author/genre views, nav/dashboard code.

FSD: the view imports `@/widgets/moderation-queue`, `@/features/moderate-book`, `@/entities/{book,author,genre}`, `@/shared/lib`, `@bookhaven/*`. The widget imports `@/shared/lib`, which is allowed.

## Existing Patterns

- `views/author-moderation/ui/author-moderation-view.tsx`: composition shape, `buildParams` cast, `SUCCESS_MESSAGES`, the hints component.
- `features/moderate-author/lib/confirm-text.ts`: the confirm and summary helpers to mirror.
- `widgets/catalog-list/ui/catalog-list.tsx` (lines ~62–68): the consume-flash effect, and `buildEditHref` usage for edit links.

## Tests

**`views/book-moderation/ui/book-moderation-view.test.tsx`: full rewrite.**

Setup:
- Mock `next/navigation` with `pushMock`, `searchParamsValue` and `usePathname → '/moderation/books'`.
- Mock `@/entities/book` partially via `importOriginal` (keep `BookStatusBadge`/`BOOK_STATUS_LABELS`), overriding `adminBooksApi.{moderationQueue,bulkStatus}`.
- Use `QueryClientProvider` with `retry: false`, `confirm` spy, and a `makeItem` factory.
- Clear `sessionStorage` in `beforeEach`.

Cases:
- `h1` is `Модерация книг`. The default call is `moderationQueue({status:'pending', source: undefined, search: undefined, page:1, limit:20})`.
- `?status=rejected&source=harvested&search=x&page=2` is passed through. `source=bogus` becomes `undefined`.
- Changing the source select pushes `/moderation/books?source=manual` (page dropped). Tab click and search submit push as the widget defines.
- Columns:
  - title link href equals `/catalog/books/b1/edit?back=` + `encodeURIComponent('/moderation/books?status=rejected&source=manual')` when those params are set, and `?back=%2Fmoderation%2Fbooks` with no params;
  - a pending author shows the badge, a published one does not;
  - a pending genre shows the badge;
  - the `Вручную` / `wikidata` + `external_id` source;
  - the `Добавлен` date.
- Hints:
  - `same_isbn` → `тот же ISBN` plus the hint title link (with back) and its status badge;
  - `same_title_author` label;
  - empty → `—`.
- Row actions:
  - pending row: `Опубликовать` → confirm `Опубликовать книгу «X»?` → `bulkStatus(['b1'],'published')` → `Книга «X» опубликована.`;
  - rejected tab row: `Вернуть на модерацию` → `pending`;
  - a report with errors gives `Не удалось изменить статус «X»: msg.`;
  - an `ApiError` message is shown.
- Bulk:
  - select-all, then `Опубликовать выбранные (2)`, then confirm `Опубликовать выбранные книги (2)?`, then `bulkStatus` with the ids in page order;
  - a partial report shows the `Ошибок: 1 («T»: msg).` banner and clears the selection.
- No `Опубликовать все на странице` button.
- Empty `Нет книг на модерации`. Filtered empty `Книги не найдены`. Load error `Не удалось загрузить очередь книг.`
- Flash: `setFlash('catalog.books','Книга «X» сохранена.')` before render → banner shown, and `consumeFlash('catalog.books')` returns `null` afterwards.

**`features/moderate-book/lib/confirm-text.test.ts`** (new): the three statuses for row and bulk, the summary with and without errors, and name fallback via `names` / `book_id`.

**`widgets/moderation-queue/ui/moderation-queue.test.tsx`:** add tests only, keep the existing ones as they are:
- with `flashKey` and a stored flash → success banner, and the key is removed;
- without `flashKey` → a stored flash is untouched and no banner is shown.

**Regression:** author/genre view tests, `catalog-list`, `flash.test.ts`, `back-href.test.ts`, dashboard/nav/shell/section-tabs/harvest-jobs/books-list tests are unchanged and pass.

## Documentation

- This file: `Status: done` plus Implementation Notes when finished.
- `docs/features/moderation-queue-widget.md`: a note on the new optional `flashKey` prop (added in #317).
- `docs/features/catalog-unified-lists.md`: close the F3 LOW note. The `catalog.books` flash is now also consumed by `/moderation/books`.
- `docs/features/book-moderation-queue.md`: one line saying the UI is implemented in #317.
- `docs/ROADMAP.md`, #310 line: add "#317 book moderation UI (done, see docs/features/book-moderation-ui.md)". The epic is complete.

## Risks

- **Shared flash key.** Both the catalog books list and the book queue consume `catalog.books`. Only the page the edit view navigates to mounts next, so the right page consumes it. If the user leaves the edit page some other way after saving (not possible today: save always navigates to `backHref`), the next book list shows the message once. That is acceptable.
- **Same latent issue for authors/genres.** `author-edit`/`genre-edit` allow `back=/moderation/{authors,genres}` but nothing links there with `back` today, so the bug cannot occur. Passing `flashKey` there is deliberately out of scope.
- **Test mocking.** Fully mocking `@/entities/book` (as the old test does) would drop `BookStatusBadge` and break rendering. Use a partial mock. `useBookStatusMutation` imports `@/entities/dashboard`; don't mock it away.
- **Strict-mode double effect.** `consumeFlash` removes the key on the first run, so the second run returns `null` and must not clear an already-set banner. Only call `setFeedback` when the message is non-null, as catalog-list does.
- **DOM selectors.** The title link is the first `font-medium` element in the row. Hint titles come later. Keep that order so `rowOf`-style helpers work.
- **typedRoutes.** `Link href={… as any}` is required for the dynamic edit hrefs. Only CI's build validates it.

## Implementation Notes

- `views/book-moderation/ui/book-moderation-view.tsx` (~210 lines incl. the small internal cells) is a pure `ModerationQueue` composition: `FILTERS` (`source`), view-local `MATCH_LABELS` (no `BOOK_MATCH_LABELS` in `entities/book`), `SUCCESS_MESSAGES`, typed `buildParams`, internal `BookPeopleCell` (generic over the status type, badge render prop), `SourceCell`, `BookHints`. The view calls `usePathname()`/`useSearchParams()` for `buildEditHref`; both title and hint `Link`s keep the typedRoutes `as any` cast. `renderRowActions`/`renderBulkActions`/`renderOverlay` are not passed.
- The `ISBN · год` muted line is shown under the title and under each hint only when at least one part exists. `Источник` reads `item.book.external_source` and the item-level `external_id`.
- `features/moderate-book/lib/confirm-text.ts`: `bookStatusConfirmText`, `bulkBookStatusConfirmText`, `bulkBookStatusSummary`, exported from the slice index next to the unchanged hook.
- Widget: optional `flashKey` prop; the mount effect returns early when it is unset and only calls `setFeedback` for a non-null message. The book view passes `flashKey="catalog.books"`.
- Tests: `book-moderation-view.test.tsx` rewritten (17 tests), `confirm-text.test.ts` (4 tests), 2 new flash tests appended to `moderation-queue.test.tsx` (existing 21 unchanged). Author/genre view sources and tests are untouched.
- Checks: `tsc --noEmit`, `next lint`, `vitest run` (52 files, 506 tests) pass in `apps/admin`. Smoke: `/moderation/books` and `/moderation/books?status=published&source=harvested` return 200 with `has_session=1`.
