# Catalog Unified Lists

Status: done

Issue: #316 (F3 of epic #310, admin catalog/import/moderation unification). Frontend only (`apps/admin`). No backend or api-client changes. All needed endpoints already exist. Depends on F1 [import-by-entity.md](import-by-entity.md), F2 [moderation-queue-widget.md](moderation-queue-widget.md), B1 [book-status-model.md](book-status-model.md), B2 [admin-get-author-genre.md](admin-get-author-genre.md). Out of scope: book moderation UI (F4 #317), harvesters.

## Goal

Каталог has exactly three lists: Книги / Авторы / Жанры. They share one layout: filters in the URL, a table, row actions «Редактировать» and «Снять с публикации». Editing happens on a separate full page per entity, never inline. Every edit page has «Назад» and «Отмена», and both restore the list exactly as the admin left it. Nothing is deleted permanently. «Снять с публикации» sets `status → pending`, which sends the entity back to moderation.

## User Stories

- As an admin, I filter the books, authors or genres list (search, status, plus author/genre/source for books). I open an entity to edit it, and «Назад», «Отмена» or a save brings me back to the same filtered page.
- As an admin, I unpublish a published entity from the catalog after a confirm. It disappears from the public site and shows up in the moderation queue.
- As an admin, the three catalog lists look and behave the same.

## Decisions (open points from the request)

1. **Shared widget: yes, `widgets/catalog-list`.** All three views would otherwise repeat the same ~150 lines: URL state, search form, status select, feedback banner with flash, unpublish flow, edit link with `back`, loading/empty/error states, table frame, pagination. This is the same reasoning as F2. The widget is separate from `widgets/moderation-queue`, because widgets may not import each other and the two behave differently (no selection or bulk bar, an "all" status, free-form filters). The views keep only config: columns, filters, copy, entity row actions.
2. **Return URL: a `?back=` query param**, plus a fixed per-entity fallback. History back is not used. The list builds the edit link as `/catalog/<entity>/<id>/edit?back=<encodeURIComponent(pathname + '?' + qs)>`. The edit page passes `back` through `safeBackHref(raw, allowedPaths, fallback)` (Business Rule 6). Anything that fails validation becomes the fallback `/catalog/<entity>`. Open redirect is impossible. «Назад», «Отмена» and post-save navigation all use the same resolved href.
3. **After save: return to the list and show a success banner there.** This matches BookForm's current redirect after save. It works through a one-shot flash in `sessionStorage` (`shared/lib/flash.ts`). The flash is not a URL param, so the restored list URL stays clean. The behaviour is the same for all three entities.
4. **«Опубликовать» is not offered in the catalog.** Publishing belongs to Модерация only, which keeps the catalog simple. Pending and rejected rows get only «Редактировать». Known side door: the existing status field in the edit forms can still set any status. That is the current behaviour and stays as is.
5. **«Обогатить из Wikidata» is a row action in the catalog authors list.** It appears only when `external_id` is set and works the same way as the moderation row action (`useEnrichAuthors`, `enrichItemText`, `enrichErrorMessage`). It is deliberately not on the edit page. Server-side enrichment while the form has unsaved edits would need form re-sync logic.
6. **Author merge leaves the catalog and lives in Модерация only.** `/moderation/authors` already offers «Слить» with duplicate hints on the pending and published tabs. The catalog authors list stops importing `features/merge-author`.
7. **Moderation queues: no name links to the new author/genre edit pages in F3.** This keeps the F2 view tests (`span.font-medium` selectors) unchanged. It can be a follow-up. Book links (`/catalog/books/{id}/edit`) in moderation stay as they are. Without `back` they fall back to `/catalog/books`. The allowlist (Business Rule 6) already accepts `/moderation/<entity>`, so F4 can pass `back` later without touching F3 code.
8. **The inner `h2` («Редактировать автора» / «Новый автор») is removed from AuthorForm.** The page `h1` (edit) or `PageSection title="Вручную"` (Import) already gives the heading. GenreForm has no heading. AuthorForm's `successMessage` and `onDismissSuccess` props are also removed, since both remaining callers pass `null`.
9. **Book unpublish uses a new `features/moderate-book` `useBookStatusMutation()`.** It matches the F2 sketch for F4. F4 reuses it, and `views/book-moderation` is not touched here.

## Acceptance Criteria

- [x] `/catalog/books`, `/catalog/authors` and `/catalog/genres` render through `widgets/catalog-list`, with the same layout and the same status filter: all / published / pending / rejected.
- [x] All filters live in the URL: `search`, `status`, `page`, plus `author_id`, `genre_id` and `source` for books. A reload or «Назад» restores the list exactly, including the search input.
- [x] Each row has «Редактировать», a link to the edit page carrying `back`. Published rows also have «Снять с публикации». Authors with a QID also have «Обогатить из Wikidata».
- [x] «Снять с публикации» flow: confirm → bulk-status `[id]` → `pending` → banner → list refetch, plus invalidation of the moderation queue and dashboard/nav counters. Cancelling the confirm does nothing.
- [x] New routes `/catalog/authors/[id]/edit` and `/catalog/genres/[id]/edit`. They load via `get`, render AuthorForm/GenreForm, save via PATCH, show errors inline (genres: `genreMutationErrorText`, for example the 409 QID conflict), and show separate not-found (404/400) and load-error states, each with «Назад».
- [x] `/catalog/books/[id]/edit` gets the same header («Назад» + h1). BookForm's «Отмена» and post-save navigation go to the resolved back href, and the list shows the success banner.
- [x] Inline edit forms and the merge dialog are gone from the catalog list views.
- [x] The import success banners (F1) link to `/catalog/authors/{id}/edit` and `/catalog/genres/{id}/edit`.
- [x] Dashboard links (`/catalog/books?status=rejected` etc.) still work. Nav counters update after an unpublish.
- [x] FSD boundaries hold, and `tsc --noEmit`, `./node_modules/.bin/next lint` and `vitest run` pass in `apps/admin`. No native `next build`.
- [x] Dynamic-route smoke check (Tests section) done after restarting the admin dev container.

## Business Rules

1. **List URL state** (owned by the widget):
   - `status`: one of `published|pending|rejected`, else `undefined` (all). The legacy `visibility` param is ignored.
   - `search`: trimmed string, empty → absent.
   - `page`: `max(1, parseInt(page) || 1)`.
   - Entity filters are each configured by URL `param`. The widget passes the raw value through, and the view's `buildParams` sanitises it (books: `source` must be `manual|harvested`, else dropped). The author and genre options load asynchronously, so the widget does not validate against options the way the moderation queue does.
   - `limit` is 20.
2. **URL updates:**
   - Copy the current params and delete empty values.
   - Changing the filter, status or search resets `page` (deletes it; today it is set to `'1'`, and either is fine).
   - Pagination sets `page`.
   - `router.push(\`${pathname}?${qs}\` as any)`. Keep the `as any` cast (typedRoutes).
   - The search input re-syncs from the URL when `searchParams` change, including on back navigation.
3. **Query keys:**
   - Lists keep `['admin', '<entity>s', params]`.
   - Edit pages use `['admin', '<entity>s', id]`.
   - Mutations invalidate the `['admin', '<entity>s']` prefix, which also covers moderation queues. Status-affecting mutations (unpublish, and saves) also invalidate `['admin', 'dashboard']`, which feeds the nav counters.
4. **Unpublish:**
   - Shown only when `status === 'published'`.
   - Flow: `confirm(copy.unpublishConfirm(name))` → `statusMutation.mutate({ids: [id], status: 'pending'})`.
   - Success: if `report.errors.length > 0`, error banner `Не удалось снять с публикации «${name}»: ${errors[0].message}.`; otherwise success `copy.unpublishSuccess(name)`.
   - On error: `ApiError` message, or `copy.statusErrorFallback`.
   - The button is disabled while the mutation is pending.
   - Copy:
     - Book: confirm `Снять книгу «X» с публикации? Она вернётся в очередь модерации.`, success `Книга «X» снята с публикации и возвращена на модерацию.`
     - Author: `Снять автора «X» с публикации? …` / `Автор «X» снят с публикации …`
     - Genre: `Снять жанр «X» с публикации? …` / `Жанр «X» снят с публикации …`
5. **APIs** (unchanged): `adminBooksApi.bulkStatus`, `adminAuthorsApi.bulkStatus`, `adminGenresApi.bulkStatus`. Any transition is allowed server-side (B1 rule 7, author/genre moderation rule 2). Authorization is server-side `admin` on all `/admin/*` routes, and the catalog tabs are `roles: ['admin']`. Nothing changes here.
6. **Return URL safety:** `safeBackHref(raw: string | null, allowed: string[], fallback: string): string` in `shared/lib/back-href.ts`. Accept `raw` only if all of these hold:
   - it is a string starting with exactly one `/` (reject `//…`, `/\…`, and any `\`);
   - it has no ASCII control characters;
   - `new URL(raw, 'http://local')` keeps origin `http://local`;
   - the resulting `pathname` is exactly one of `allowed` (no prefix matching, so `/catalog/books/x/edit` is rejected).

   If accepted, return `pathname + search` (drop the hash). Otherwise return `fallback`.
   - Allowed paths per entity: `['/catalog/<entity>', '/moderation/<entity>']`.
   - Fallback: `/catalog/<entity>`.
   - `buildEditHref(base, pathname, searchParams)` builds the edit link. Put it in the same file or in the widget.
7. **Flash:** `shared/lib/flash.ts` with:
   - `setFlash(key, message)`: writes to `sessionStorage`; if storage throws, it is a no-op.
   - `consumeFlash(key)`: reads and removes the message.

   Keys are `catalog.books`, `catalog.authors` and `catalog.genres`. The widget calls `consumeFlash(flashKey)` once in a mount effect and shows the result as a success banner.
8. **Edit pages:**
   - Read `id` from `useParams()` and `back` from `useSearchParams()`, then resolve with rule 6.
   - Loading shows a skeleton.
   - `ApiError` 404 or 400 → `Автор не найден.` / `Жанр не найден.` / `Книга не найдена.`
   - Any other error → `Не удалось загрузить данные.`
   - Every state shows the «Назад» link.
   - Initialise the form state from the loaded entity once (`authorToInput` / `genreToInput` helpers in the respective `features/manage-*` `lib`, the mapping of today's `startEdit`).
   - Submit runs the PATCH mutation. On success: invalidate the keys (rule 3), `setFlash(key, '<Entity> «name» сохранён.')` using the name from the response, then `router.push(backHref as any)`.
   - On error, show the message inline and stay on the page.
   - «Отмена» runs `router.push(backHref)` and does not save.
   - For books, BookForm does the navigation itself (Frontend section).

## UX

List page (`space-y-6`, same for all three):
- `h1`: `Книги` / `Авторы` / `Жанры`. Genres keep the «Очередь модерации» link in the header row. Add the same link for books and authors (`/moderation/books`, `/moderation/authors`) for uniformity.
- Filter row: search input + `Найти` (icon button as today), the `Статус` select (`Все статусы` / `Опубликован(о)` / `На модерации` / `Отклонён`, labels from `*_STATUS_LABELS`), then the entity selects. Every select has an `aria-label`. Books: `Автор`, `Жанр`, `Источник` (`Все` / `Добавлено вручную` / `Импортировано`).
- Feedback banner: success in green, error in destructive, with «Скрыть».
- Then the load error, skeleton, empty box (`Книги не найдены.` etc.), or table + `Pagination`.
- Table: entity columns, then `Действия` (right). Action order: `Редактировать` (ghost link), `Снять с публикации` (ghost, destructive text), then entity extras.

Columns:
- Books (as today): Название, Авторы, Жанры, Статус (badge + `Импортировано`).
- Authors: Имя, Slug, QID (`WikidataLink` from `@/shared/ui`, using `external_id`), Статус.
- Genres: Название, Slug, Статус, QID (switch to `WikidataLink`).

Edit page:
- Top: a `← Назад` link (`Link` to backHref).
- `h1`: `Редактирование книги` / `Редактирование автора` / `Редактирование жанра`, with the entity name as a muted subtitle once loaded.
- Then the form. Its buttons are `Сохранить` and `Отмена`.

## API

None new. Uses:
- `adminBooksApi.list/get/update/bulkStatus`
- `adminAuthorsApi.list/get/update/bulkStatus/enrichWikidata`
- `adminGenresApi.list/get/update/bulkStatus`

Authors list params are `search, status, page, limit`. There is no `qid` on the plain list (that filter exists only in the moderation queue), so the authors catalog has search + status only.

## Database

None.

## Frontend

New:
- `widgets/catalog-list/`:
  - `index.ts`
  - `model/types.ts`
  - `model/use-catalog-url-state.ts`
  - `ui/catalog-list.tsx`
  - `ui/catalog-list.test.tsx`

  Public API (keep it minimal, no speculative props):
  ```ts
  export type CatalogStatus = 'pending' | 'published' | 'rejected';
  export interface CatalogListState { status?: CatalogStatus; search?: string; page: number; limit: number; filters: Record<string, string | undefined> }
  export interface CatalogListFilter { param: string; label: string; options: { value: string; label: string }[] } // first option value '' = any
  export interface CatalogRowContext { notify: (f: { type: 'success' | 'error'; message: string } | null) => void }
  export interface CatalogListColumn<T> { id: string; header: ReactNode; headClassName?: string; cellClassName?: string; cell: (item: T) => ReactNode }
  export interface CatalogListProps<T, P, R extends { errors: { message: string }[] }> {
    title: string; headerExtra?: ReactNode;
    entityPath: string;            // '/catalog/authors' — edit href = `${entityPath}/${id}/edit?back=…`
    flashKey: string;
    queryKey: readonly unknown[]; buildParams: (s: CatalogListState) => P; queryFn: (p: P) => Promise<PaginatedResponse<T>>;
    statusMutation: Pick<UseMutationResult<R, Error, { ids: string[]; status: CatalogStatus }>, 'mutate' | 'isPending'>;
    getId: (i: T) => string; getName: (i: T) => string; getStatus: (i: T) => CatalogStatus;
    filters?: CatalogListFilter[]; columns: CatalogListColumn<T>[];
    renderRowActions?: (item: T, ctx: CatalogRowContext) => ReactNode;
    copy: { searchPlaceholder: string; loadError: string; empty: string; statusErrorFallback: string;
            unpublishConfirm: (name: string) => string; unpublishSuccess: (name: string) => string };
    limit?: number;
  }
  ```
- `shared/lib/back-href.ts`: `safeBackHref` and `buildEditHref`, with a colocated test, exported from `@/shared/lib`.
- `shared/lib/flash.ts`: `setFlash` and `consumeFlash`, with a colocated test.
- `features/moderate-book/`:
  - `index.ts`
  - `model/use-book-status-mutation.ts`: wraps `adminBooksApi.bulkStatus` and invalidates `['admin','books']` and `['admin','dashboard']`.
- `views/author-edit/`:
  - `index.ts`
  - `ui/author-edit-view.tsx`
  - `ui/author-edit-view.test.tsx`
- `views/genre-edit/`: the same files for genres.
- `app/catalog/authors/[id]/edit/page.tsx` and `app/catalog/genres/[id]/edit/page.tsx`: thin, like the books one.
- `features/manage-author/lib/author-to-input.ts` and `features/manage-genre/lib/genre-to-input.ts`, exported.
- Optional: a `ui/edit-page-header.tsx` (`← Назад` + h1 + subtitle) in `shared/ui`, used by all three edit views, since views may not import each other.

Changed:
- `views/books-list`, `views/authors`, `views/genres`: rewritten as `CatalogList` compositions.
  - Books: filters `author_id` and `genre_id` with options from the existing `['admin','authors','all']` and `['admin','genres','all']` queries (limit 100, unchanged), and `source` with static options.
  - Authors: `renderRowActions` = enrich (when there is a QID), using `useAuthorStatusMutation()` from `features/moderate-author`.
  - Genres: `useGenreStatusMutation()`.
  - Authors and genres lose `AuthorForm`/`GenreForm` and the merge dialog.
- `views/book-edit/ui/book-edit-view.tsx`:
  - Add the header with «Назад».
  - Resolve backHref.
  - Distinguish the 404 state.
  - `handleUpdate` awaits the update, invalidates `['admin','books']` and `['admin','dashboard']`, then calls `setFlash('catalog.books', 'Книга «title» сохранена.')`.
  - Pass `returnHref={backHref}` to BookForm.
- `features/manage-book/ui/book-form.tsx`: new optional prop `returnHref?: string` (default `'/catalog/books'`), used by the post-save `router.push` and by «Отмена». The `stayOnSuccess` behaviour (Import) is unchanged.
- `features/manage-author/ui/author-form.tsx`: remove the `h2`, `successMessage` and `onDismissSuccess`. Update `author-form.test.tsx` and `views/import-authors/ui/author-create-section.tsx`.
- `views/import-authors/ui/author-create-section.tsx` and `views/import-genres/ui/genre-create-section.tsx`: store the created entity (id + name). Link `Открыть в каталоге` → `/catalog/authors/${id}/edit` and `/catalog/genres/${id}/edit` (no `back`, so the fallback applies). Update their tests.
- `shared/config/navigation.test.ts`: add `/catalog/authors/1/edit` → `catalog.authors` and `/catalog/genres/1/edit` → `catalog.genres`. The prefix match should already cover them; verify.

Unchanged:
- `views/*-moderation`
- `widgets/moderation-queue`
- `features/merge-author` (still used by moderation)
- dashboard links
- api-client

FSD: `app → views → widgets → features → entities → shared`. `widgets/catalog-list` imports only shared, entities and `@bookhaven/*`; it receives the status mutation as a prop. Views never import other views.

## Existing Patterns

- `widgets/moderation-queue` (F2): generic shell, config-driven views, `statusMutation` passed as a hook result, `as any` on `router.push`/`Link`, test harness (`next/navigation` mock with `pushMock`/`searchParamsValue`, `QueryClientProvider` with `retry: false`).
- `views/author-moderation`: the enrich row action (copy it for the authors catalog row action).
- `views/book-edit` and `app/catalog/books/[id]/edit/page.tsx`: the edit route and view shape.
- `features/moderate-author|genre/model/use-*-status-mutation.ts`: template for `useBookStatusMutation`.

## Tests

- `shared/lib/back-href.test.ts`:
  - Accepted: `/catalog/books?status=pending&page=2`, and the same value URL-decoded from `back`.
  - Rejected (fallback):
    - `null`, `''`
    - `https://evil.com`, `//evil.com`, `/\evil.com`, `\\evil.com`
    - `javascript:alert(1)`
    - `/catalog/books/../../x` (the pathname normalises to a non-allowed value)
    - `/catalog/booksX`, `/catalog/books/1/edit`, `/login`
    - a value with `%0a` or control characters
  - The hash is dropped.
  - `buildEditHref` round-trip: the produced `back` decodes and passes validation.
- `shared/lib/flash.test.ts`: set, consume, consume again returns null; a throwing `sessionStorage` is handled.
- `widgets/catalog-list/ui/catalog-list.test.tsx` (fake item type):
  - URL parsing: bogus status → all; page `0`/`abc` → 1; filter values passed through.
  - Pushed URLs on status/filter/search changes (page dropped) and on pagination.
  - The edit link href contains the encoded current `pathname?qs`.
  - Unpublish button only on published rows. Confirm cancel → no mutate. Confirm → `mutate({ids:[id], status:'pending'})`.
  - Banners: success, report error, `ApiError` message, fallback. «Скрыть» hides the banner.
  - The flash is consumed and shown once.
  - States: loading, error, empty, table.
  - `renderRowActions` content appears after the built-in actions.
- View tests (update `books-list-view.test.tsx`, `authors-view.test.tsx`, `genres-view.test.tsx`):
  - `buildParams` and API call args, including books `author_id`/`genre_id`/`source` and the invalid `source` being dropped.
  - Authors: the enrich action shows only with `external_id` and calls `enrichWikidata([id])`; there is no «Слить» and no inline form.
  - Genres: there is no inline form.
- `author-edit-view.test.tsx` and `genre-edit-view.test.tsx`:
  - Loading.
  - 404 → not-found text plus a «Назад» link to the fallback.
  - Other error → load error.
  - Loaded → form prefilled.
  - Save → `update(id, input)`, flash set, push to the `back` value.
  - Save error → inline message, no push. Genre 409 → `genreMutationErrorText`.
  - «Отмена» → push back without calling update.
  - A malicious `back` → «Назад» href and pushes go to `/catalog/<entity>`.
- `book-edit` view test (new): header «Назад» href, 404 state, `returnHref` passed. In `book-form.test.tsx`: «Отмена» and post-save push use `returnHref`; the default stays `/catalog/books`; `stayOnSuccess` is unchanged.
- Import create sections: the link points to `/catalog/<entity>/<id>/edit`.
- Commands (native, in `apps/admin`): `tsc --noEmit`, `./node_modules/.bin/next lint`, `vitest run`. **Do not run a native `next build`.** The CI frontend job is the typedRoutes gate.
- **Dynamic-route smoke check (required).** The dockerised Next dev server does not pick up new dynamic segments without a restart; we saw a 404 for exactly this reason before.
  1. Run `docker compose restart admin`.
  2. Wait for it to be ready, then run `curl -s -o /dev/null -w "%{http_code}\n" http://localhost:3001/catalog/authors/00000000-0000-0000-0000-000000000000/edit` and the same for `/catalog/genres/…/edit`. Expect `200` (the route shell). A `404` means the route was not picked up.
  3. Do this before reporting done.

## Documentation

- This file: when done, set `Status: done` and add Implementation Notes.
- `docs/features/import-by-entity.md`: one line saying the catalog links now point to the edit pages (#316).
- `docs/features/merge-author.md` / `enrich-author` docs, if they mention the catalog authors view: note that merge is moderation-only and enrich is a catalog row action (#316).
- `docs/ROADMAP.md` line 53 (#310): add "#316 catalog unified lists (done: `widgets/catalog-list`, standalone edit pages for books/authors/genres, unpublish → pending, see docs/features/catalog-unified-lists.md)".

## Risks

- **Open redirect or path injection via `back`.** This is mitigated by the strict exact-path allowlist plus the URL-origin check (rule 6) and the tests. Never `router.push(raw)`.
- **Dynamic route 404 in dev** if the admin container is not restarted. Covered by the mandatory smoke step.
- **typedRoutes.** New `Link`/`router.push` hrefs with query strings need the existing `as any` casts. Only CI's build validates them.
- **Search input drift:** local `useState` initialised from the URL goes stale on back navigation. Re-sync it in an effect on `searchParams.get('search')`.
- **Async filter options (books):** an `author_id` that is not among the first 100 authors renders as an empty select label but still filters. This is acceptable and is how it works today. Do not drop unknown ids.
- **Lost functionality perception:** catalog merge is removed. Mitigated: moderation already offers merge on the pending and published tabs. Mention it in the PR.
- **Status side door:** the edit forms can still publish. This is accepted (decision 4) and should be documented in the PR.
- **Flash in `sessionStorage`** survives a reload until consumed. It is consumed on the next list mount, keyed per entity, so the impact is low.
- **F4 overlap:** F4 must reuse `features/moderate-book` from here instead of creating its own.

## Implementation Notes

- `widgets/catalog-list` follows the public API above with one addition: a required `statusLabels: Record<CatalogStatus, string>` prop (the entity's `*_STATUS_LABELS`), because book labels are feminine («Опубликована») and author/genre labels masculine. Option order is all / published / pending / rejected for all three lists.
- URL updates delete `page` on status/filter/search changes; an empty query pushes the bare pathname (no trailing `?`). The search value is trimmed when parsed and when submitted.
- «Редактировать» is `Button asChild` wrapping a `Link` (no button nested in an anchor). The edit href is built with `buildEditHref` from `@/shared/lib`.
- `safeBackHref` additionally rejects values longer than 2048 characters and percent-encoded control characters/backslashes (`%00`-`%1F`, `%7F`, `%5C`) anywhere in the value; `searchParams.get('back')` is already decoded once.
- Flash keys are stored as `bookhaven.flash.<key>` in `sessionStorage`.
- `shared/ui/edit-page-header.tsx` (`EditPageHeader`: «← Назад» link + h1 + muted subtitle) is used by all three edit views; every state (loading, not found, load error, loaded) renders it.
- Author/genre edit views initialise the form once from the loaded entity (`authorToInput` / `genreToInput`); saves invalidate `['admin','<entity>s']` and `['admin','dashboard']`, set the flash with the name from the PATCH response and push the resolved back href. The book edit view does the same inside `handleUpdate`, and BookForm navigates via `returnHref`.
- `features/moderate-book` exports `useBookStatusMutation` (+ `BookStatusChange`) for F4 to reuse.
- Import create sections store the created `{ id, name }`; the banner text is unchanged.
- F2 files (`views/*-moderation`, `widgets/moderation-queue`) are untouched.
- Closed in #317 (LOW review note): the book edit page could return to `/moderation/books`, which did not read the flash, so `Книга «X» сохранена.` stayed in sessionStorage and popped up later in `/catalog/books`. The book moderation queue now consumes `catalog.books` via the moderation widget's `flashKey` prop, so the key means "book-saved flash, consumed by whichever book list the edit page returns to" (see [book-moderation-ui.md](book-moderation-ui.md)).
- Checks: `tsc --noEmit`, `next lint`, `vitest run` (51 files, 496 tests) pass in `apps/admin`. Smoke after `docker compose restart admin`: `/catalog/{books,authors,genres}` and `/catalog/{books,authors,genres}/<real id>/edit` return 200, as do the zero-UUID author/genre edit routes.
