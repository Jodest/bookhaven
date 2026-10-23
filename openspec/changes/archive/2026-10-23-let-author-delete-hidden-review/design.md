## Context

Today the reviews service refuses `UpdateReview` and `DeleteReview` with `ErrReviewNotFound` (404) when `review.IsHidden` is set, after the ownership check (`backend/internal/modules/reviews/service/service.go`). `ListReviews` already returns the hidden own review as `my_review` (`GetByUserAndBook` has no visibility filter). However, `reviewResponse` in `transport/http/handler.go` has no hidden flag.

In apps web, `widgets/reviews-section` renders only `items` as cards, so a hidden `my_review` is never shown as a card. The only effect is that the top button reads «Редактировать рецензию». That button opens the form, and PATCH then fails with 404. The author has no way to delete the review.

Two existing mechanisms already give the right behavior once a hidden review is deleted:
- `Repository.Delete` runs `recalcInTx`, which recounts `reviews_count` from `is_hidden = false` rows. It does not decrement, so deleting a hidden review leaves the count correct with no new code.
- `review_likes` and `notifications` reference `reviews(id) ON DELETE CASCADE`, so likes and notifications go away with the row.

## Decisions

1. **Delete is allowed for the author of a hidden review.** In `DeleteReview`, remove the `if review.IsHidden { return ErrReviewNotFound }` branch. The order becomes: not found (404), then not the author (403), then delete. No repository change.
2. **PATCH on the author's own hidden review returns 409 «review is hidden»**, not 404. The author receives the review in `my_review` and can delete it by the same id, so "not found" would contradict what they see. 403 is already used for "not your review". 409 (the resource is in a state that forbids the action) is the precise status, and it does not reveal anything new, because only the author reaches this branch. Add `ErrReviewHidden = errors.New("review is hidden")` in the service. In `UpdateReview`, return it in place of `ErrReviewNotFound` in the existing hidden branch: after the ownership check, before the nothing-to-update and field validation. The handler maps it to `409 {"error": "review is hidden"}`. Request-binding validation in the handler (struct tags) still runs first, so a malformed body is 400 as today.
3. **`is_hidden` goes on every review object** (`reviewResponse`), filled from `domain.Review.IsHidden`. Putting it on every review object, instead of adding a field only to `my_review`, keeps one review type everywhere. It is always false in `items` and in the POST/PATCH responses: POST creates a visible review, and PATCH on a hidden review is refused. Others never see a hidden review, so the field leaks nothing.
4. **`POST` stays 409 «already reviewed»** while the hidden review exists. The UI never offers the form in that state, and the way out is deleting the review.
5. **Rating stars while the review is hidden** keep working as today (`reviews/book-ratings`, scenario "Changing the rating while the review is hidden"). This is a rating, not editing the review's text, so it does not get around moderation and is out of scope.
6. `ToggleLike` keeps its hidden check (404), and so does the public list filter. Admin moderation endpoints are unchanged.

## API

- Review object (list `items`, `my_review`, POST 201, PATCH 200): new field `is_hidden: boolean`.
- `PATCH /api/v1/reviews/{id}`: auth bearer, author only. 401 / 400 invalid id or body / 404 unknown / 403 not the author / **409 `{"error":"review is hidden"}`** for the author's hidden review / 400 field rules / 200.
- `DELETE /api/v1/reviews/{id}`: auth bearer, author only. 404 unknown / 403 not the author (hidden or not) / 200 `{"status":"ok"}`, **now also for the author's hidden review**.
- Update the swagger annotations: add `@Failure 409` on PATCH. Update the generated `backend/docs/swagger.json` and `backend/docs/swagger.yaml` for the `reviewResponse` `is_hidden` property and the PATCH 409, either by hand or by running the repo's usual `swag` command, without touching unrelated definitions.

## Database

None. No migration is needed.

## Frontend

- `packages/api-client/src/index.ts`: `Review` gets `is_hidden?: boolean`. It is optional, like `liked_by_me?`, so existing fixtures and constructed objects keep compiling.
- `apps/web/src/entities/review/ui/review-card.tsx` (`ReviewCard`): when `review.is_hidden` is true:
  - render the badge «Скрыт модератором» next to the date or stars, in muted or destructive style, using an existing `@bookhaven/ui` Badge if one exists, otherwise a small span;
  - render the note «Модератор скрыл эту рецензию — другие читатели её не видят. Вы можете удалить её и написать новую.» under the text;
  - hide the like button and «Редактировать», and keep «Удалить» (only for `isOwner`).
  Non-hidden rendering stays the same. Other users of the card (`recent-reviews`, `notification-bell`) never receive hidden reviews, so they are unaffected.
- `apps/web/src/widgets/reviews-section/ui/reviews-section.tsx`:
  - derive `const myHidden = myReview?.is_hidden === true`;
  - when `myHidden`, render `<ReviewCard review={myReview} isOwner … onDelete={handleDelete} />` in the authenticated block, in place of the «Написать/Редактировать рецензию» button, and never open the form (`showForm` stays false);
  - `handleDelete` is reused unchanged: the same confirm, then `fetchReviews(1, false)`, after which `my_review` is absent and «Написать рецензию» appears. «Моя оценка» is not touched;
  - keep the button label rule «Редактировать рецензию» only for a visible `myReview`.
  The FSD boundaries hold: the widget already imports the entity and the feature.

## Existing Patterns

- Ownership-then-state checks in `reviews/service` (`UpdateReview`, `DeleteReview`, `ToggleLike`).
- Error-to-status switch in `reviews/transport/http/handler.go` (409 is already used for `ErrAlreadyReviewed` in CreateReview).
- `reviews-section.test.tsx` for the widget tests with a mocked `reviewsApi`.

## Risks

- **Small API break:** PATCH on a hidden review changes from 404 to 409. The only client (apps/web) no longer sends it. External callers have no contract.
- The swagger files can drift if regenerated wholesale. Only change the review definitions and the PATCH responses.
- Deleting a hidden review also deletes the likes and notifications that hiding had kept. This is intended and matches a normal delete.
