# Tasks

## 1. Backend

- [x] 1.1 In the reviews service, `DeleteReview` stops refusing hidden reviews: unknown is 404, not the author is 403 (hidden or not), the author is allowed. Verify with new service unit tests: the author deletes a hidden review (succeeds, repo `Delete` is called), another user deletes a hidden review (`ErrForbidden`).
- [x] 1.2 Add `ErrReviewHidden` ("review is hidden"). `UpdateReview` returns it for the author's hidden review (after the ownership check, before nothing-to-update and field validation), and the handler maps it to 409 `{"error":"review is hidden"}`. Verify with a service test (the author gets `ErrReviewHidden`, the review is unchanged and `Update` is not called; another user still gets `ErrForbidden`) and a handler test, or extend the existing handler test pattern if there is one, for the 409. _Handler test: `transport/http/update_hidden_test.go` (409 for the author, 403 for another user)._
- [x] 1.3 Add `IsHidden bool \`json:"is_hidden"\`` to `reviewResponse` and fill it in `toReviewResponse`. Verify with a test: `ListReviews` for an author with a hidden review returns `my_review.is_hidden == true` and no hidden review in `items`.
- [x] 1.4 Repository test (the existing sqlite/repository test style): deleting a hidden review keeps the reviewer's `ratings` row, keeps `reviews_count` equal to the number of visible reviews, and removes its `review_likes` and `notifications` rows. If the sqlite harness does not enforce the FK cascade, assert only what it supports and note it. _Not done as a repository test: `Delete` → `recalcInTx` uses Postgres-only SQL that the sqlite harness cannot run. Rating kept and `reviews_count` recounted are covered by service tests with a fake repo; the likes/notifications cascade relies on the existing FKs (no migration here) and is untested._
- [x] 1.5 Swagger: add `@Failure 409` to the PATCH godoc. Update `backend/docs/swagger.json` and `backend/docs/swagger.yaml` for `is_hidden` on the review response and the PATCH 409. Change only those definitions.
- [x] 1.6 Verify from `backend/`: `go build ./...`, `go vet ./...`, `go test ./internal/modules/reviews/...`. Run gofmt only on the changed files.

## 2. Frontend

- [x] 2.1 `packages/api-client`: add `is_hidden?: boolean` to `Review`.
- [x] 2.2 `entities/review` `ReviewCard`: for `review.is_hidden`, show «Скрыт модератором» and the note «Модератор скрыл эту рецензию — другие читатели её не видят. Вы можете удалить её и написать новую.», hide the like button and «Редактировать», and keep «Удалить» for the owner.
- [x] 2.3 `widgets/reviews-section`: when `my_review.is_hidden`, render the own hidden card (with `onDelete={handleDelete}`) in place of the «Написать рецензию»/«Редактировать рецензию» button, and never open the form. After a confirmed delete, the refetch shows «Написать рецензию».
- [x] 2.4 Extend `reviews-section.test.tsx`. Hidden `my_review`: the badge and the note are shown, there is no «Редактировать рецензию», «Написать рецензию», «Редактировать» or like button, and «Удалить» is present. Confirming the delete calls `reviewsApi.remove` and then shows «Написать рецензию». Dismissing the confirm keeps the card. A visible `my_review` behaves as before.
- [x] 2.5 Verify: `tsc --noEmit` and `next lint` for apps/web, the api-client type check, and the apps/web unit tests for the touched files. No native `next build`: CI is the gate.

## 3. Docs

- [x] 3.1 No separate docs. The spec deltas in this change are the documentation, and they are archived into `openspec/specs/reviews/` by the orchestrator. Confirm the deltas still match the implementation, including the copy and the status codes.
