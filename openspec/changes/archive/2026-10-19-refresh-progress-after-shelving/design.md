# Design

## Context

- `features/shelve-book/model/use-book-shelves.ts` keeps shelves in `useState`, calls `shelvesApi.addBook/removeBook` directly and never touches the React Query cache.
- `widgets/personal-home/model/use-personal-home.ts` caches `['home','me',userId]` and `['reading-challenges',year,userId]` with `staleTime: 60_000`, `refetchOnWindowFocus: false` (global defaults in `shared/api/query-client.ts` are the same). Client-side navigation back to `/` remounts the widget, but the data is still fresh, so nothing is refetched for up to 60 s.
- `views/shelf-detail/ui/shelf-detail-view.tsx` (`onRemoveBook`, `onDelete`) and `views/shelves/ui/shelves-view.tsx` (`onDelete`) change shelf contents the same way.
- `views/challenge` fetches without the cache (always fresh) — leave it as is.
- `features/manage-reading-challenge/ui/manage-reading-challenge-goal.tsx:80` already invalidates with the literal `['reading-challenges', year]`.
- Backend computes `books_read` live; no backend change.

## Decisions

1. **Key factories in entities** (pattern: `CURATED_COLLECTIONS_KEY` in `entities/curated-collection/model/use-curated-collections.ts`). New files `model/query-keys.ts`, re-exported from each entity `index.ts`:
   ```ts
   // entities/shelf
   export const shelfKeys = {
     all: ['shelves'] as const,
     user: (userId?: string) => [...shelfKeys.all, userId] as const,
     list: (userId?: string) => [...shelfKeys.user(userId), 'list'] as const,
     forBook: (userId: string | undefined, bookId: string) => [...shelfKeys.user(userId), 'book', bookId] as const,
     detail: (userId: string | undefined, shelfId: string) => [...shelfKeys.user(userId), 'detail', shelfId] as const,
     detailPage: (userId: string | undefined, shelfId: string, page: number, limit: number) =>
       [...shelfKeys.detail(userId, shelfId), { page, limit }] as const,
   };
   // entities/reading-challenge
   export const readingChallengeKeys = {
     all: ['reading-challenges'] as const,
     year: (year: number) => [...readingChallengeKeys.all, year] as const,
     detail: (year: number, userId?: string) => [...readingChallengeKeys.year(year), userId] as const,
   };
   // entities/home
   export const homeKeys = {
     all: ['home'] as const,
     me: (userId?: string) => [...homeKeys.all, 'me', userId] as const,
   };
   ```
   Resulting keys for home and challenge are identical to today's literals (`['home','me',id]`, `['reading-challenges',year,id]`), so nothing else changes. Entities do not import each other.
2. **One invalidation helper** in `features/shelve-book/model/invalidate-shelf-caches.ts`, exported from `features/shelve-book/index.ts` (views may import features; features may import entities):
   - `invalidateShelfContents(queryClient)` → `invalidateQueries` for `shelfKeys.all`, `readingChallengeKeys.all`, `homeKeys.all`. Used by: book add/remove (default or custom), «Убрать с полки», shelf deletion (deleting a custom shelf changes which books are "on a shelf", which affects recommendations).
   - `invalidateShelfMeta(queryClient)` → `shelfKeys.all` only. Used by: create, rename, description edit, privacy toggle.
   Invalidation is prefix-based on `all`, which also covers other accounts' entries harmlessly; keys still carry `userId` so a different account never reads another account's cached data.
3. **`useBookShelves(bookId)` on React Query**, same return shape `{ isAuthenticated, shelves, status, error, toggleDefault, toggleCustom, clearStatus }`:
   - `useAuthStore()` → `{ user, isAuthenticated }`; `useQuery({ queryKey: shelfKeys.forBook(user?.id, bookId), queryFn: () => shelvesApi.list({ book_id: bookId }).then(r => r.items), enabled: isAuthenticated })`.
   - `shelves = query.data ?? []`; `status`: `!isAuthenticated` → `'ready'`; `query.isPending` → `'loading'`; `query.isError` → `'error'`; else `'ready'` (a disabled query is pending, so check auth first — preserves the skeleton behaviour).
   - One `useMutation` with variables `{ shelf, wasActive, kind: 'default' | 'custom' }`, `mutationKey: ['shelves','toggle-book']`, `mutationFn`: `wasActive ? removeBook(shelf.id, bookId) : addBook(shelf.id, bookId)`.
   - `onMutate`: `cancelQueries(forBook key)`, then `setQueryData` with the same patch as today: default → every default-type shelf gets `hasBook = s.type === shelf.type ? !wasActive : false`; custom → only `s.id === shelf.id` flips.
   - `onError`: targeted rollback, not a whole-snapshot restore (keeps concurrent toggles of other shelves intact, matching today's logic): default → shelf of `shelf.type` gets `hasBook = wasActive`, others unchanged; custom → that shelf gets `wasActive`. Error text as today: `err instanceof Error ? err.message : 'Ошибка обновления полки'`.
   - `onSettled`: call `invalidateShelfContents(queryClient)`, but only when `queryClient.isMutating({ mutationKey: ['shelves','toggle-book'] }) === 1` (the last in-flight toggle), so a refetch never overwrites optimistic state of a still-running toggle. The refetch of `forBook` also reconciles server-side mutual exclusion.
   - `error` returned = mutation error message ?? query error message ?? `null`, using local `useState` set in `onError` / query error (so the error persists after the mutation settles, like today, and clears on the next toggle).
   - `toggleDefault/toggleCustom(shelf)` stay `async (shelf) => Promise<void>`, no-op when not authenticated, implemented with `mutateAsync(...).catch(() => {})` so callers using `void toggleDefault(...)` never see a rejection. `clearStatus` unchanged (finds current default shelf in `shelves`).
   - Book cards render many `useBookShelves` instances — one request per book as today; after a toggle, `shelfKeys.all` invalidation refetches only *active* `forBook` queries (mounted cards). Acceptable; same order of requests as an F5.
4. **`views/shelves`**: `useQuery(shelfKeys.list(user?.id), () => shelvesApi.list().then(r => r.items))`; `useMutation`s for create / update (rename, privacy) / remove; `onSuccess` → `invalidateShelfMeta` (create, update) or `invalidateShelfContents` (remove). Errors keep the current texts (`'Ошибка создания полки'`, `'Ошибка переименования полки'`, `'Ошибка изменения видимости полки'`, `'Ошибка удаления полки'`, `'Ошибка загрузки полок'`) shown in the same banner; `reset()` after create and `setEditingId(null)` after rename as today. `onRename/onTogglePrivacy/onDelete` remain `Promise<void>` (ShelfCard props unchanged). «Загрузка...» while `isPending`.
5. **`views/shelf-detail`**: `page` in `useState(1)`, `limit = 12`; `useQuery({ queryKey: shelfKeys.detailPage(user?.id, id, page, limit), queryFn: () => shelvesApi.get(id, { page, limit }), placeholderData: keepPreviousData })`. Pagination buttons call `setPage`, disabled while `isFetching`. Form `reset({ name, description })` in a `useEffect` on `data?.shelf` (only when not editing). Mutations: update (edit, privacy) → `invalidateShelfMeta`; removeBook → `invalidateShelfContents`; remove shelf → `removeQueries({ queryKey: shelfKeys.detail(user?.id, id) })` first (avoids a 404 refetch flash of the deleted shelf), then `invalidateShelfContents`, then `router.push('/shelves')`. Error texts unchanged. Initial «Загрузка...» when there is no data yet.
6. **Reading-challenge consumers**: `use-personal-home.ts` uses `homeKeys.me(user?.id)` and `readingChallengeKeys.detail(year, user?.id)` (staleTime/refetchOnWindowFocus unchanged — invalidation, not shorter stale time, is the fix). `manage-reading-challenge-goal.tsx` uses `readingChallengeKeys.year(year)` (its manual `upsert` call can stay as is; converting it to `useMutation` is optional and out of scope).
7. `views/public-shelf-detail` stays on direct fetching (read-only, public, not affected by the bug).

## Frontend

FSD boundaries: entities (`shelf`, `home`, `reading-challenge`) ← feature `shelve-book` (helper + hook) ← views `shelves`, `shelf-detail`. Feature `manage-reading-challenge` and widget `personal-home` import only entities. No view imports another view, no feature imports another feature or a widget.

## Existing Patterns

- Key constant in entity: `apps/web/src/entities/curated-collection/model/use-curated-collections.ts`.
- `useMutation` usage: `apps/web/src/features/pick-random-book/ui/pick-random-book.tsx`.
- Test wrapper with `QueryClient({ defaultOptions: { queries: { retry: false } } })`: `apps/web/src/widgets/personal-home/ui/personal-home.test.tsx`.

## Risks

- **Test mocks**: tests `vi.mock('@/entities/shelf' | '@/entities/home' | '@/entities/reading-challenge', () => ({ ...api only }))` will lose the new key exports, making `shelfKeys` etc. undefined. Use `vi.mock(path, async (importOriginal) => ({ ...(await importOriginal()), shelvesApi: {...} }))` or put the factories in a module that the tests do not mock and import it explicitly in the mock factory.
- **Rapid toggles**: guarded by targeted rollback + the `isMutating === 1` check before invalidation; without the guard a refetch could flash an intermediate state.
- **useAuthStore mocks** in `book-shelf-actions.test.tsx` / `shelve-book-buttons.test.tsx` return only `isAuthenticated`; add `user: { id: 'u1' }` so keys are stable (undefined also works, but be explicit).
- **Each test needs a fresh QueryClient** (no cache leak between tests); `retry: false` for queries and mutations, otherwise rollback tests wait for retries.
- **shelf-detail after deleting the last book on the last page**: behaviour as today (backend returns an empty page); no extra page-clamping logic.
