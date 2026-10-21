# Tasks

## 1. Backend

- [x] 1.1 Add forward-only migration `backend/migrations/000035_adaptation_votes.{up,down}.sql` as in design.md (composite FK to `book_screen_works` ON DELETE CASCADE, user FK cascade, value CHECK, link and `updated_at` indexes). Do not edit earlier migrations, and never run `migrate down` or any drop against a database. Verify by applying `up` on the local stack.
- [x] 1.2 Screenworks domain: vote values `book|adaptation|same`, `MinVerdictVotes = 5`, errors for invalid value, not a reader and adaptation not found. Unit-test the value parsing.
- [x] 1.3 Repository: `UpsertVote` (no `updated_at` bump when the value is unchanged), `DeleteVote`, `IsReader` (read shelf via `shelf_items`/`shelves`) and `ListVerdicts` (visible links only, per-value counts and the caller's vote, public order). Add a comment that any future book or work merge must move votes.
- [x] 1.4 Service: `ListVerdicts(bookID, *userID)` (book published, otherwise `ErrBookNotFound`; counts nulled below the minimum; `can_vote`), `Vote(userID, bookID, workID, value)` (book published → visible link → reader → upsert → return item) and `Unvote` (idempotent). Unit-test every spec scenario: first vote, change, not a reader (403), hidden link (404), unpublished book, below the minimum, hidden-then-shown keeps counts, vote kept after the reader leaves «Прочитано».
- [x] 1.5 Transport: three handlers per design.md API, with the exact error strings from the spec. Register `GET /books/:id/adaptations/verdicts` (optionalAuth), and `PUT`/`DELETE /books/:id/adaptations/:screen_work_id/vote` (authMiddleware). Add handler tests for 400/401/403/404/200/204, swagger annotations, and regenerate `backend/docs`.
- [x] 1.6 Repository test (or service test with a fake repo plus one SQL-level test, following existing screenworks tests) proving that deleting a manual link removes its votes, and that hiding does not.
- [x] 1.7 Verify from `backend/`: `go build ./...`, `go vet ./...`, `go test ./...`. Run gofmt only on the changed files.

## 2. Frontend web

- [x] 2.1 `packages/api-client`: types `AdaptationVoteValue`, `AdaptationVerdict`, `AdaptationVerdicts`, and methods `adaptationVerdicts`, `voteAdaptation` and `unvoteAdaptation` under `api.books`.
- [x] 2.2 `entities/screen-work`: `screenWorkKeys`, api additions, the pure `verdictLabel` helper with tests (78%/23 голоса, tie «Мнения разделились», below the minimum, 1/2/5 plural forms), and the `renderExtra` slot on `BookAdaptations`, with an existing test extended to cover it.
- [x] 2.3 New `features/vote-adaptation`: verdicts query hook plus vote/unvote mutations, and the `AdaptationVerdict` client component per the spec. Component tests cover guest prompt, not-read prompt, reader buttons with the selected state, clicking the selected button to remove, the kept vote with «Убрать голос», the error text «Не удалось сохранить голос.», and rendering nothing on query error.
- [x] 2.4 `views/book-detail`: pass `renderExtra` with `AdaptationVerdict`, and update `book-detail-view.test.tsx`. Leave `AuthorFilms` without a verdict.
- [x] 2.5 `features/shelve-book/model/invalidate-shelf-caches.ts`: also invalidate `screenWorkKeys.all`, and add a test for it.
- [x] 2.6 Verify in `apps/web`: `tsc --noEmit`, `next lint` and the web unit tests, plus `tsc --noEmit` for `packages/api-client`. No native `next build`: CI is the typedRoutes gate.

## 3. Docs

- [x] 3.1 `docs/admin-guide/screen-works.md`: hiding a link keeps its readers' votes (shown again when unhidden), and deleting a manual link deletes its votes. There is no admin vote UI.
- [x] 3.2 `docs/ROADMAP.md`: mark #509 in progress or done per the pipeline convention.
