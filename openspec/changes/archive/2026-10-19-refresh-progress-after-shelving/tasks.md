# Tasks

Frontend only (`apps/web`). Do not touch `backend/`. Verify with `pnpm --filter @bookhaven/web exec tsc --noEmit`, `pnpm --filter @bookhaven/web lint` (next lint) and `pnpm --filter @bookhaven/web test` (vitest run) — never a native `next build` (CI is the typedRoutes gate).

## 1. Frontend: query-key factories

- [x] 1.1 Add `entities/shelf/model/query-keys.ts` (`shelfKeys`), `entities/reading-challenge/model/query-keys.ts` (`readingChallengeKeys`), `entities/home/model/query-keys.ts` (`homeKeys`) exactly as in design.md Decision 1; re-export from each entity `index.ts`. Verify: tsc passes.
- [x] 1.2 Switch `widgets/personal-home/model/use-personal-home.ts` to `homeKeys.me(user?.id)` and `readingChallengeKeys.detail(year, user?.id)` (keys unchanged in value). Switch `features/manage-reading-challenge/ui/manage-reading-challenge-goal.tsx` invalidation to `readingChallengeKeys.year(year)` and drop the comment about the literal key. Verify: `personal-home.test.tsx` and the challenge tests pass (update their entity mocks to keep the real key exports via `importOriginal`).
- [x] 1.3 Grep `apps/web/src` for remaining literal `['shelves'`, `['home'`, `['reading-challenges'` keys — none must remain outside the factories.

## 2. Frontend: shelve-book feature on React Query

- [x] 2.1 Add `features/shelve-book/model/invalidate-shelf-caches.ts` with `invalidateShelfContents(queryClient)` (shelfKeys.all + readingChallengeKeys.all + homeKeys.all) and `invalidateShelfMeta(queryClient)` (shelfKeys.all); export both from `features/shelve-book/index.ts`.
- [x] 2.2 Rewrite `features/shelve-book/model/use-book-shelves.ts` per design.md Decision 3 (useQuery + one useMutation, optimistic `onMutate`, targeted rollback in `onError`, guarded invalidation in `onSettled`, same return shape and status semantics). `book-shelf-actions.tsx` and `shelve-book-buttons.tsx` must need no changes beyond imports, if any.
- [x] 2.3 Update `book-shelf-actions.test.tsx` and `shelve-book-buttons.test.tsx`: render inside a fresh `QueryClientProvider` per test (`retry: false`), `useAuthStore` mock returns `user: { id: 'u1' }`, entity mock keeps real `shelfKeys`. All existing cases must pass unchanged in intent.
- [x] 2.4 Add tests (e.g. `features/shelve-book/model/use-book-shelves.test.tsx` or in the actions test): (a) picking «Прочитано» calls `addBook('s3','b1')` and afterwards the client has invalidated `['reading-challenges', …]` and `['home','me', …]` (seed those keys with `setQueryData`, assert `getQueryState(key)?.isInvalidated === true`, or spy on `invalidateQueries`); (b) a failing `addBook` on «Прочитано» while the book is on «Читаю» restores «Читаю» as marked and shows the error; (c) a failing custom toggle restores the checkbox and leaves the status untouched; (d) an unauthenticated user triggers no `list` call and status is `'ready'`.

## 3. Frontend: shelf views on React Query

- [x] 3.1 Rewrite `views/shelves/ui/shelves-view.tsx` per design.md Decision 4 (list query, create/update/remove mutations, same error texts and UI). Remove uses `invalidateShelfContents`; create/update use `invalidateShelfMeta`.
- [x] 3.2 Rewrite `views/shelf-detail/ui/shelf-detail-view.tsx` per design.md Decision 5 (paged query with `keepPreviousData`, update/remove/removeBook mutations, `removeQueries` for the deleted shelf before navigating, same error texts and UI).
- [x] 3.3 Update `shelves-view.test.tsx` and `shelf-detail-view.test.tsx` with a fresh `QueryClientProvider` per test and `importOriginal`-based entity mocks; add to `shelf-detail-view.test.tsx` a test that «Убрать с полки» calls `removeBook` and invalidates the reading-challenge and home keys, and a test that pagination «Вперёд» requests page 2.
- [x] 3.4 Leave `views/public-shelf-detail` unchanged.

## 4. Verification and docs

- [x] 4.1 Run tsc, lint and vitest for `apps/web`; all green.
- [x] 4.2 Manual check on the local stack deferred to the human reviewer: listed as an unchecked item in the dev PR checklist (open `/`, note «Прочитано n из goal», mark a book «Прочитано», go back without F5 — card shows n+1; «Убрать с полки» on «Прочитано» — n−1; «Читаю» — «Продолжить чтение»). Not performed by the pipeline.
- [x] 4.3 No docs beyond this change; specs are updated when the change is archived.
