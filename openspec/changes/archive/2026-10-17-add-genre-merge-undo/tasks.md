# Tasks

## 1. Backend: journal and merge

- [x] 1.1 Add `backend/migrations/000033_genre_merge_journal.up.sql` / `.down.sql` exactly as in design.md (do not edit older migrations; never run `migrate down`/`force`). Verify by applying with the usual `migrate up` against the local DB only if needed.
- [x] 1.2 Add `repository/genre_merge_journal.go`: per-source snapshot helper (status, QID, `book_moved`, `book_linked`, `alias`) and the batched journal writer; add `MergeID` to `GenreMergeResult`.
- [x] 1.3 In `genre_moderation.go` change only `mergeGenres` and the two public wrappers: take `actorID *uuid.UUID`, snapshot each source right before `mergeGenreInTx`, add collections/`qid_carried` after it, write the journal after the loop in the same transaction. Update the repository interface and every fake.
- [x] 1.4 Service: add `WithActor` / `actorFromContext` (new file); `MergeGenres` returns `mergeID` as 5th value; `GenreBatchMergeOutcome.MergeID`; both pass the actor from context. `MergeGenresBatch` signature unchanged.
- [x] 1.5 Handlers: wrap the context with the actor from `user_id` in `MergeGenres` / `MergeGenresBatch`; add `merge_id` to both responses; update swagger text (drop «Необратимо.»).
- [x] 1.6 SQLite tests: extend the fixture DDL with the journal tables (+ `users` if missing); single merge writes one entry with correct items, actor and `qid_carried`; batch writes one entry, sources in order, shared book is `book_moved` for the first source and `book_linked` for the second; a 404 merge writes nothing. Service/handler tests: `merge_id` in responses, actor passed from context. Run `go build ./...`, `go vet ./...`, `go test ./internal/modules/admin/...` from `backend/`.

## 2. Backend: history, preview, undo

- [x] 2.1 Add `repository/genre_merge_undo.go`: `ListGenreMerges`, `PreviewGenreMergeUndo`, `UndoGenreMerge(ctx, mergeID, actorID)` and `GenreMergeUndoBlockedError{Code}` per design.md (lock helper without alias filter, check order, reverse-order apply, later-merge protection, `AutoCollectionIDs`).
- [x] 2.2 Service (new file): `ErrGenreMergeNotFound`, blocked error with code, `ListGenreMerges(page, limit)` with default 20 / max 100, `PreviewGenreMergeUndo`, `UndoGenreMerge` that recomputes switched-back auto collections after commit and re-reads the target.
- [x] 2.3 Handlers in new `transport/http/handler_genre_merge_undo.go` (400/404/409 `{error, code}`/500), Russian swagger godoc; register `GET /genres/merges`, `GET /genres/merges/:id/undo-preview`, `POST /genres/merges/:id/undo` with `requireBearer` in the admin-only genres group in `router.go`; regenerate swagger (additive).
- [x] 2.4 SQLite repo tests (new file): full single round trip (status, QID, `merged_into_id`, moved books back and off the target, linked books keep both links, aliases back, collections back); batch round trip with a shared book; carried QID returns; partial changes skipped (moved book unlinked from target, book deleted, collection re-pointed, collection deleted); later merge relying on a moved book keeps the target link; each block code (`already_undone`, `target_missing`, `target_merged`, `source_missing`, `source_not_alias`, `qid_conflict`) leaves everything unchanged; auto collection ids returned; preview counts equal the undo result and preview changes nothing; history order, pagination, name/e-mail fallback, `undone_at/by`.
- [x] 2.5 Service tests (fake repo): not-found and blocked mapping, recompute called only after success with auto ids. Handler tests: 200 shapes for all three, 400 bad id, 404, 409 with `code`; moderator gets 403 on all three routes (same pattern as the existing merge-batch moderator test). Run go build/vet/test from `backend/`.

## 3. Frontend: api-client and admin

- [x] 3.1 `packages/api-client`: new types, `merge_id` on merge results, `merges` / `undoMergePreview` / `undoMerge` methods; expose them in `apps/admin/src/entities/genre/api/index.ts`.
- [x] 3.2 New feature `apps/admin/src/features/undo-genre-merge`: `UndoGenreMergeDialog` + preview summary + `undoErrorMessage`, exact Russian texts from the spec. Vitest: loading text, returned/relinked lines (relinked only when > 0), collection lines, button disabled until preview loads, each 409 code text, 404 and generic text, success callback and query invalidation.
- [x] 3.3 New view `apps/admin/src/views/genre-merge-history` and route `apps/admin/src/app/moderation/genres/merges/page.tsx`. Vitest: rows and «Слияние» format, «—» performer, «Активно»/«Отменено {date}», no button on undone rows, empty and load-error texts, `?undo=<id>` opens the dialog, success banner with and without the collections suffix.
- [x] 3.4 `widgets/moderation-queue`: optional `action` link on `ModerationFeedback` (vitest: rendered when set, absent otherwise).
- [x] 3.5 `views/genre-moderation`: toolbar link «История слияний»; merge success messages carry the «Отменить» action to `/moderation/genres/merges?undo=<merge_id>`. `features/merge-genre`: `onMerged` passes `mergeId`; replace «Действие необратимо.» with «Слияние можно будет отменить в «Истории слияний».» in both dialogs; do not touch `merge-preview-summary.tsx`. Update the existing vitest suites accordingly.
- [x] 3.6 Run `tsc --noEmit` and `next lint` for `apps/admin` and typecheck `packages/api-client`, plus the admin vitest suite. No native `next build` — the CI frontend job is the typedRoutes gate.

## 4. Docs

- [x] 4.1 `docs/admin-guide/duplicates.md` (Russian): the irreversibility note applies to authors only; genre merge steps say the merge can be undone and mention the «Отменить» link; new section «Отмена слияния жанров» (where the history is, columns, the dialog and preview texts, what is restored and what is skipped, every block message, merges made before the feature cannot be undone).
- [x] 4.2 Add a merge-order note to the PR description: if this PR lands before #482, #482's migration must be renumbered to 000034 before it merges.
