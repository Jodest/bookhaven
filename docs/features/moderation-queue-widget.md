# Moderation Queue Widget

Status: done

Issue: #315 (F2 of epic #310, admin catalog/import/moderation unification). Frontend only (`apps/admin`). No backend or api-client changes. Next consumer: F4 (#317, book moderation on `GET /admin/books/moderation`, see [book-moderation-queue.md](book-moderation-queue.md)).

## Goal

`views/author-moderation/ui/author-moderation-view.tsx` (~600 lines) and `views/genre-moderation/ui/genre-moderation-view.tsx` (~520 lines) are near-copies. Move the shared shell into one widget, `widgets/moderation-queue`. Each view becomes a thin composition that passes in only the entity-specific parts. Behaviour, copy, URL params and DOM structure stay the same. The generic API must also cover F4 (books) without any widget changes.

## User Stories

- As an admin, the author and genre moderation pages work exactly as they do today.
- As a developer building F4, I get a book queue by writing a config (columns, copy, data hook, mutation). I do not copy another 500-line view.

## Acceptance Criteria

- [x] New slice `apps/admin/src/widgets/moderation-queue/` with public `index.ts`, exporting `ModerationQueue`, `moderationCreatedAtColumn`, and the public types (API section).
- [x] `AuthorModerationView` and `GenreModerationView` render through `ModerationQueue`. Each view file holds only entity config, entity cells, entity row/bulk extras and its merge dialog. Target: under about 250 lines each. No duplicated shell logic (URL parsing, selection, feedback banner, bulk bar, table frame, pagination, states).
- [x] The existing `author-moderation-view.test.tsx` (14 tests) and `genre-moderation-view.test.tsx` (12 tests) pass **unchanged**. If a structural change truly forces an edit, only the setup or selectors may change, and every behavioural assertion (texts, confirm strings, API call args, pushed URLs, checked state, invalidated keys) stays.
- [x] New widget-level test `widgets/moderation-queue/ui/moderation-queue.test.tsx` (Tests section).
- [x] FSD: the widget imports only `shared`, `entities`, `features`, `@bookhaven/*`, and its own slice. It never imports `views` or another widget. Views import the widget only through `@/widgets/moderation-queue`. `next lint` is clean (boundaries plugin).
- [x] `tsc --noEmit`, `next lint` (via `./node_modules/.bin/next lint`) and `vitest` for `apps/admin` all pass. Do **not** run a native `next build`. The CI frontend job is the typedRoutes gate.
- [x] `views/book-moderation` is untouched (F4).

## Business Rules (behaviour that must be preserved, now owned by the widget)

1. **URL state.** Read from `useSearchParams()`:
   - `status`: `published`/`rejected`; anything else → `pending`.
   - `search`: string; empty → `undefined`.
   - `page`: `max(1, parseInt(page) || 1)`.
   - Each configured filter `param`: kept only if it equals one of that filter's non-empty option values, else `undefined`. This matches today's `parseQid` and `parseBooks`.
   - `limit` is a constant 20 (prop `limit`, default 20).
2. **URL updates** (`updateParams`): copy the current params, delete keys whose value is `undefined` or `''`, set the rest, clear the selection, then `router.push(\`${pathname}?${next}\` as any)`. Keep the `as any` cast (typedRoutes).
   - Status tab click: `{status: tab === 'pending' ? undefined : tab, page: undefined}`.
   - Search form submit: `{search: input.trim(), page: undefined}`.
   - Filter select change: `{[param]: value, page: undefined}`.
   - Pagination: `{page: String(p)}`.
   - Resulting URLs must match today's, e.g. `/moderation/authors?qid=without`, `?status=rejected`, `?search=%D0%A2...`.
3. **Selection.** Selection is a `Set<string>` of row ids and belongs to the current page and filters only. It is cleared:
   - by `useEffect` on `searchParams.toString()`;
   - in `updateParams`;
   - after a successful bulk status change;
   - through `controls.clearSelection()` (merge success, bulk enrich success).
   `selectedItems = items.filter(i => selected.has(getId(i)))`. "Select all on page" toggles between all ids on the page and none. It is checked only when `items.length > 0` and every row is selected.
4. **Status tabs** (fixed, widget-owned): `На модерации` / `Опубликованные` / `Отклонённые`. They render inside `role="group" aria-label="Статус"`, active tab has `variant="default"` and `aria-pressed`.
5. **Row status buttons** (widget-owned, depend on `getStatus(item)`; all disabled while `statusMutation.isPending`):
   - `rejected` → `Вернуть на модерацию` (outline), `Опубликовать`.
   - Otherwise: `Опубликовать` (only if `pending`), then `Отклонить` (destructive).
   - Then the `renderRowActions(item, ctx)` slot, in the same `flex flex-col items-end gap-1` container. This keeps today's DOM order:
     - author: `…status, [Слить if not rejected], [Обогатить из Wikidata if QID]`;
     - genre: `…status, Слить` (always, also on rejected rows).
6. **Row status flow.**
   - Clear feedback, then `confirm(copy.rowConfirm(next, item))`. Cancel → nothing.
   - Otherwise `statusMutation.mutate({ids:[id], status: next})`.
   - `onSuccess`: if `report.errors.length > 0`, show error `Не удалось изменить статус «${name}»: ${errors[0].message}.` (widget-owned string). Else show success `copy.rowSuccess(next, name)`.
   - `onError`: `err instanceof ApiError && err.message ? err.message : copy.statusErrorFallback`.
7. **Bulk bar.** Shown only when `selectedItems.length > 0`. Contents in order:
   - `Выбрано: N`.
   - Status buttons, which depend on the **current tab** (not the row status):
     - `rejected` tab: `Вернуть на модерацию (N)` (outline), `Опубликовать выбранные (N)`.
     - Otherwise: `Опубликовать выбранные (N)` (only on `pending`), `Отклонить выбранные (N)` (destructive).
     - All disabled while the mutation is pending.
   - Then the `renderBulkActions(selectedItems, controls)` slot (author: `Обогатить из Wikidata (k)`).
8. **Bulk status flow.**
   - Clear feedback. With an empty selection → return.
   - `confirm(copy.bulkConfirm(next, selectedItems))`. The view computes the `books_visible` sum.
   - Mutate with the selected ids in page order.
   - `onSuccess`: clear the selection, then set feedback to `error` if `report.errors.length > 0`, else `success`. The message is `copy.bulkSummary(report, names)`, where `names = {id: name}` for the selected rows.
   - `onError` as in rule 6.
9. **Feedback banner.** One `{type: 'success'|'error', message}`. Classes: green on success, destructive on error. Has a `Скрыть` button. Rendered after the filter form and before the bulk bar.
10. **Load and empty states.**
    - The `copy.loadError` block shows whenever the query has an error.
    - While `isLoading`: the `h-64 animate-pulse` skeleton.
    - No items and no error: a centered empty box. It shows `copy.emptyPending` when `status === 'pending'` and no search or filter is set; otherwise `copy.emptyFiltered`.
    - Otherwise: the table, then `Pagination` with `totalPages = ceil(total / limit)`.
11. **Query.**
    - `useQuery({ queryKey: [...queryKey, params], queryFn: () => queryFn(params) })`, with `params = buildParams(state)`.
    - Views must keep their keys: `['admin','authors','moderation']` and `['admin','genres','moderation']`. Mutations invalidate by the `['admin','<entity>']` prefix.
12. **Table frame.**
    - Header: checkbox column `TableHead className="w-8"` with `aria-label="Выбрать все на странице"`, then the configured columns, then `Действия` (`text-right`).
    - Row: checkbox cell (`align-top`, `aria-label="Выбрать «${getName(item)}»"`), then the column cells, then the actions cell (`align-top text-right`).
    - The widget adds no other text nodes inside rows. The genre test counts the `—` spans in a row.

## UX

No visible change. The page layout stays `space-y-6`, in this order:
- `h1` (`title`)
- status tabs
- search form, with configured selects after the `Найти` button
- feedback banner
- bulk bar
- load error
- skeleton, empty box, or table + pagination
- overlay slot (dialogs)

## API (widget public interface, `widgets/moderation-queue/index.ts`)

```ts
export type ModerationStatus = 'pending' | 'published' | 'rejected';
export interface ModerationFeedback { type: 'success' | 'error'; message: string }
export interface ModerationStatusChange { ids: string[]; status: ModerationStatus }
export interface ModerationStatusReport { processed: number; updated: number; errors: { message: string }[] }

export interface ModerationQueueState {
  status: ModerationStatus;
  search?: string;
  page: number;
  limit: number;
  filters: Record<string, string | undefined>; // keyed by filter param, validated
}

export interface ModerationQueueFilter {
  param: string;          // URL key, e.g. 'qid' | 'books' | 'source'
  label: string;          // Select aria-label, e.g. 'QID', 'Книги'
  options: { value: string; label: string }[]; // first option value '' = any
}

export interface ModerationQueueControls {
  notify: (feedback: ModerationFeedback | null) => void;
  clearSelection: () => void;
}
export interface ModerationRowContext extends ModerationQueueControls {
  changeStatus: (next: ModerationStatus) => void; // rule 6 incl. confirm
  statusPending: boolean;
}

export interface ModerationQueueColumn<TItem> {
  id: string;
  header: ReactNode;
  headClassName?: string;
  cellClassName?: string; // full TableCell className, e.g. 'align-top text-sm'
  cell: (item: TItem, ctx: ModerationRowContext) => ReactNode;
}

export interface ModerationQueueCopy<TItem, TReport> {
  title: string;
  searchPlaceholder: string;
  loadError: string;
  emptyPending: string;
  emptyFiltered: string;
  statusErrorFallback: string;
  rowConfirm: (next: ModerationStatus, item: TItem) => string;
  bulkConfirm: (next: ModerationStatus, items: TItem[]) => string;
  rowSuccess: (next: ModerationStatus, name: string) => string;
  bulkSummary: (report: TReport, names: Record<string, string>) => string;
}

export interface ModerationQueueProps<TItem, TParams, TReport extends ModerationStatusReport> {
  queryKey: readonly unknown[];                 // prefix; widget appends params
  buildParams: (state: ModerationQueueState) => TParams;
  queryFn: (params: TParams) => Promise<PaginatedResponse<TItem>>;
  statusMutation: Pick<UseMutationResult<TReport, Error, ModerationStatusChange>, 'mutate' | 'isPending'>;
  getId: (item: TItem) => string;
  getName: (item: TItem) => string;
  getStatus: (item: TItem) => ModerationStatus;
  filters?: ModerationQueueFilter[];
  columns: ModerationQueueColumn<TItem>[];
  renderRowActions?: (item: TItem, ctx: ModerationRowContext) => ReactNode;
  renderBulkActions?: (selected: TItem[], controls: ModerationQueueControls) => ReactNode;
  renderOverlay?: (controls: ModerationQueueControls) => ReactNode;
  copy: ModerationQueueCopy<TItem, TReport>;
  limit?: number; // default 20
}

export function ModerationQueue<TItem, TParams, TReport extends ModerationStatusReport>(props): JSX.Element;
export function moderationCreatedAtColumn<TItem>(get: (item: TItem) => string): ModerationQueueColumn<TItem>;
// header 'Добавлен', cellClassName 'align-top whitespace-nowrap text-sm', toLocaleDateString('ru-RU')
```

Notes:
- `statusMutation` is the **result** of the entity hook (`useAuthorStatusMutation()`), called in the view. Do not pass the hook itself (rules-of-hooks). `AdminAuthorStatus`, `AdminGenreStatus` and `AdminBookStatus` are the same union as `ModerationStatus`, so `AuthorStatusChange` and `GenreStatusChange` are assignable. If TS variance on `mutate` complains, adapt with a typed wrapper in the view. Do not loosen the widget to `any`.
- The view holds its own merge-dialog state. Its overlay renders the dialog and calls `controls.clearSelection()` and `controls.notify({...})` in `onMerged`. Opening merge calls `ctx.notify(null)` first (today's `openMerge` clears feedback).
- The widget owns `useRouter`/`usePathname`/`useSearchParams`, `useQuery`, selection, feedback and confirm. Views own no state except merge targets.

### Author view composition

- Filters:
  - `qid`: label `QID`, options `QID: любой` / `С QID` / `Без QID`.
  - `books`: label `Книги`, options `Книги: любые` / `Без книг`.
- `buildParams`: `{status, qid, books, search, page, limit}`.
- Columns (headers and cellClassName exactly as today):
  - `Автор`: name `span.font-medium` + `AuthorStatusBadge`, slug below.
  - `QID`: `WikidataLink`.
  - `moderationCreatedAtColumn(i => i.created_at)`.
  - `Книги`: `SampleBooksCell`.
  - `Возможно, тот же человек`: hints with `MATCH_LABELS`, the `разные QID — вероятно, тёзки` note, and `Это он — слить` → `openMerge(author, hint.author)`.
- `renderRowActions`:
  - `Слить` (ghost) when status is not `rejected`;
  - `Обогатить из Wikidata` when `external_id`, using `useEnrichAuthors` and the existing `enrichItemText`/`enrichErrorMessage`, through `ctx.notify`.
- `renderBulkActions`: `Обогатить из Wikidata (k)`, as today. It uses selected rows with a QID, `bulkEnrichSummary`, and `clearSelection` on success. Disabled while enrich is pending or k = 0.
- Copy:
  - title `Модерация авторов`, placeholder `Поиск по имени`;
  - load error `Не удалось загрузить очередь авторов.`;
  - empty texts `Нет авторов на модерации` / `Авторы не найдены`;
  - fallback `Не удалось изменить статус автора.`;
  - confirm and summary functions from `features/moderate-author`; bulk confirm sums `books_visible`;
  - success messages from the view's `SUCCESS_MESSAGES`.
- Overlay: `MergeAuthorDialog` with `duplicate` and `initialCanonical`. Success text `Автор «${name}» слит с каноническим автором. Перенесено книг: ${n}.`

### Genre view composition

- Filters: `qid` with options `Все` / `С QID` / `Без QID`.
- Columns:
  - `Жанр`;
  - `QID`;
  - created-at;
  - `Книги`;
  - `Возможно, тот же жанр`: note `разные QID — возможно, разные понятия`.
- `renderRowActions`: `Слить` (ghost) always.
- No bulk extras.
- Copy: genre strings from the current view and `features/moderate-genre`.
- Overlay: `MergeGenreDialog` with `source`. Success text `Жанр «${name}» слит. Перенесено книг: ${n}.`

### F4 (books) validation sketch — not implemented here

- `queryKey: ['admin','books','moderation']`, `queryFn: adminBooksApi.moderationQueue`, `buildParams` → `AdminBookQueueParams` (`status, source, search, page, limit`).
- Filters: `source` with options `Источник: любой` / `Вручную` (`manual`) / `Харвестер` (`harvested`). The backend returns 400 for other values, and rule 1 already drops them.
- `getId: i => i.book.id`, `getName: i => i.book.title`, `getStatus: i => i.book.status`.
- `statusMutation`: a new `features/moderate-book` `useBookStatusMutation()`. It wraps `adminBooksApi.bulkStatus` and invalidates `['admin','books']` and `['admin','dashboard']`. `AdminBookBulkStatusReport` satisfies `ModerationStatusReport`.
- Copy: `bookStatusConfirmText`/`bulkBookStatusConfirmText` (no `books_visible`, so `items` are just counted) and `bulkBookStatusSummary` (keyed by `book_id`/`title`).
- Columns: `Книга` (title + status badge, cover/ISBN/year), `Авторы и жанры` (each with a status badge), `Источник` (`external_source` + `external_id`), created-at, `Возможно, та же книга` (hints with `same_isbn`/`same_title_author` labels; no merge, since books cannot be merged).
- `renderRowActions`: an edit link to `/catalog/books/:id/edit`.
- `renderBulkActions` and `renderOverlay`: none.

Conclusion: every F4 need maps onto existing props. No widget change is expected.

## Database

None.

## Frontend

New:
- `widgets/moderation-queue/index.ts`: public exports.
- `widgets/moderation-queue/ui/moderation-queue.tsx`: the component. It may split internals into `ui/filters-bar.tsx`, `ui/bulk-bar.tsx`, `ui/feedback-banner.tsx`, `model/use-queue-url-state.ts`, `model/use-row-selection.ts` if that helps readability. These stay slice-internal and are not exported.
- `widgets/moderation-queue/model/types.ts`: the types above.
- `widgets/moderation-queue/ui/moderation-queue.test.tsx`.

Deduplicated presentational helpers (identical in both views today):
- `WikidataLink` → `shared/ui/wikidata-link.tsx`, exported from `@/shared/ui`. It has no entity dependency. Markup unchanged: `—` span when no qid; otherwise an `a` with `font-mono text-xs text-primary underline`, `target="_blank"`, `rel="noopener noreferrer"`.
- Books context cell (`N (видимых M)` + sample-book links to `/catalog/books/:id/edit` with a lowercased `BOOK_STATUS_LABELS` suffix for non-published books) → `entities/book/ui/sample-books-cell.tsx` (`SampleBooksCell`), exported from `@/entities/book`. Props are structural: `{ booksTotal: number; booksVisible: number; sampleBooks: { id: string; title: string; status: AdminBookStatus }[] }`. Keep the `as any` cast on the `Link` href.

Changed:
- `views/author-moderation/ui/author-moderation-view.tsx`, `views/genre-moderation/ui/genre-moderation-view.tsx`: rewritten as compositions. Hints cells stay in the views because match labels, warning text and entity shape differ.
- `features/moderate-author`, `features/moderate-genre`, `features/merge-*`, `features/enrich-author`: unchanged.
- `entities/book/index.ts`, `shared/ui/index.ts`: add the new exports.

Unchanged: routes under `app/moderation/*`, `views/book-moderation`, api-client.

## Existing Patterns

- The two views themselves are the reference for exact markup and copy. Copy strings verbatim.
- `widgets/book-harvest`, `widgets/wikidata-author-import`: widget slice layout (`index.ts` + `ui/*.tsx` + colocated test).
- Test setup: the `next/navigation` mock with `pushMock`/`searchParamsValue`, and `QueryClientProvider` with `retry: false`, as in the view tests.

## Tests

- **View tests:** keep `author-moderation-view.test.tsx` and `genre-moderation-view.test.tsx` as they are. Their `vi.mock` calls on `@/entities/author|genre` and `@/features/merge-*` are module-wide, so they still apply when calls go through the widget. These are the regression net for "no behaviour change".
- **Widget test** (`moderation-queue.test.tsx`), using a fake item type (`{id, name, status}`), `queryFn = vi.fn()`, and a `statusMutation` built in a test wrapper with a real `useMutation({ mutationFn: bulkMock })`:
  - Defaults: `buildParams` receives `{status:'pending', page:1, limit:20, filters:{x: undefined}}`. The query key is `[...prefix, params]`.
  - URL parsing: `status=bogus` → pending; an invalid filter value → `undefined`; a valid one passes through; `page=0`/`abc` → 1.
  - Filter select, status tab, search submit and pagination push the expected URLs. The page param is dropped on filter changes.
  - Selection:
    - select-all and per-row toggles;
    - the bulk bar appears only with a selection;
    - the selection clears on a filter change and after a bulk success.
  - Bulk buttons per tab: pending → publish + reject; published → reject only; rejected → return + publish.
  - Confirm cancel → no mutation. Bulk confirm receives the selected items.
  - Bulk success: `bulkSummary` is called with the report and the names map. Banner type is `error` when the report has errors.
  - Row flow:
    - per-status buttons;
    - success uses `rowSuccess`;
    - a report with errors gives `Не удалось изменить статус «X»: msg.`;
    - an `ApiError` message is shown, and `statusErrorFallback` is used for other errors.
  - Slots:
    - `renderRowActions` content appears after the status buttons;
    - `renderBulkActions` gets the selected items;
    - `renderOverlay` controls: `notify` shows the banner, `Скрыть` hides it, `clearSelection` unchecks rows.
  - States: skeleton while loading, `loadError` on rejection, `emptyPending` vs `emptyFiltered` (search or filter set, or a non-pending tab).
- **`SampleBooksCell`** and **`WikidataLink`**: covered through the view tests. A small unit test is optional.
- **Commands** (native, inside `apps/admin`): `tsc --noEmit`, `./node_modules/.bin/next lint`, `vitest run`. No native `next build`.

## Documentation

- This file: when done, set `Status: done` and add Implementation Notes (final prop names, anything that differs from this contract).
- `docs/features/author-moderation-queue.md` and `docs/features/genre-moderation-queue.md`: add a one-line note that the UI now renders through `widgets/moderation-queue` (#315).
- `docs/ROADMAP.md`, #310 line: add "#315 moderation queue widget (done: `widgets/moderation-queue`, author/genre views migrated, see docs/features/moderation-queue-widget.md)".

## Risks

- **Hidden behaviour drift.** Some author/genre differences are easy to flatten by accident:
  - `Слить` is hidden on rejected author rows but shown on rejected genre rows;
  - QID "any" labels differ (`QID: любой` vs `Все`);
  - the author queue has the extra `books` filter, plus enrich row and bulk actions;
  - merge dialog props differ (`duplicate` vs `source`);
  - the different-QID warning texts differ.
  The unchanged view tests catch most of these. Keep them unmodified.
- **DOM-coupled tests.**
  - `rowOf` relies on `span.font-medium` inside a `tr`. The genre test counts exactly two `—` spans per row.
  - Hint lists also use `span.font-medium`, so the primary name must stay the first match. Today's order: primary cell before the hints cell.
  - The widget must not add extra `span.font-medium` or `—` nodes in rows.
- **Generic typing friction.**
  - Generic function components plus TanStack `UseMutationResult` variance: if typing gets awkward, prefer a small view-side adapter over `any` in the widget.
  - The `as any` casts for `router.push` and `Link` hrefs must remain (typedRoutes). Only CI's build validates them.
- **Over-generalisation.** Do not add speculative props (custom status tabs, column sorting, server-side select-all). F4 is validated above with the current set.
- **Test runtime.** Moving logic does not change mocks, but the `useEffect` that clears the selection on params change must stay. Bulk-success tests assert unchecked boxes afterwards.

## Implementation Notes

- Slice layout: `widgets/moderation-queue/index.ts`, `model/types.ts` (public types exactly as in the API section), `model/use-queue-url-state.ts` (URL parsing + push, slice-internal), `ui/moderation-queue.tsx` (shell, incl. the internal generic `QueueRow`), `ui/created-at-column.tsx` (`moderationCreatedAtColumn`), `ui/moderation-queue.test.tsx` (21 tests).
- Prop names match the contract. No extra props were added. `filters` defaults to `[]`, `limit` to 20.
- The widget has no `any` in its generic typing. The only cast is the existing `router.push(... as any)` (typedRoutes). `SampleBooksCell` keeps the `Link` `as any` cast.
- `useAuthorStatusMutation()`/`useGenreStatusMutation()` results are passed directly as `statusMutation`; no adapter was needed (TS accepted the variance).
- Views narrow the validated filter values with a typed cast in `buildParams` (`filters.qid as AdminAuthorQueueParams['qid']`), since the widget already drops values outside the configured options.
- Views keep only the merge-target state. Merge/enrich handlers take the row `ctx` / bulk `controls` and use `notify`/`clearSelection`. Hints cells stay in the views (`AuthorHints`, `GenreHints`).
- `WikidataLink` moved to `shared/ui/wikidata-link.tsx`, `SampleBooksCell` to `entities/book/ui/sample-books-cell.tsx`. The cell renders the cell contents (summary + list); the `TableCell` class comes from the column's `cellClassName`.
- Size: genre view ~215 lines, author view ~295 lines (over the ~250 target because the author-only enrich row/bulk handlers and the second filter stay in the view, as the contract requires).
- The existing author (15 tests) and genre (12 tests) view test files are unchanged and pass.
- Added in #317 (F4, [book-moderation-ui.md](book-moderation-ui.md)): optional `flashKey?: string` prop. When set, a mount effect calls `consumeFlash(flashKey)` from `@/shared/lib` and shows a non-null message as a success banner (same as `widgets/catalog-list`); when unset, sessionStorage is not read. Only the book view passes it (`catalog.books`); author/genre views are unchanged.
