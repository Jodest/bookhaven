# Proposal

Backlog proposal #382 — "Author of a hidden review cannot edit, delete or re-write it".

## Why

When a moderator hides a review, its author is stuck. The book page shows the review like a normal one, with «Редактировать рецензию». Editing returns 404, the author has no way to delete the review, and writing a new one returns 409. The product decision is "SEES + DELETES": the author sees that the review is hidden and can delete it. Editing stays blocked so it cannot be used to get around moderation.

## What Changes

- Every review object in the reviews API gets a new field, `is_hidden` (boolean). It can be true only on the caller's own `my_review`, because other readers never receive hidden reviews.
- `DELETE /api/v1/reviews/{id}` is allowed for the author of a hidden review. It works exactly like a normal delete: the rating is kept, likes and notifications are removed, and `reviews_count` is recounted from visible reviews only, so the count does not change.
- **BREAKING** (small): `PATCH /api/v1/reviews/{id}` on the author's own hidden review now returns 409 «review is hidden» instead of 404. The author can see the review and delete it, so "not found" would be contradictory. The review stays unchanged.
- After deleting the hidden review, the author can write a new review as usual. `POST` still returns 409 while the hidden review exists.
- Book page (apps/web): the author's hidden review is shown as their own card with the mark «Скрыт модератором» and an explanation. The card has only «Удалить»: no edit button and no like button. The «Написать рецензию» and «Редактировать рецензию» buttons are hidden while the hidden review exists.
- Other users are not affected: hidden reviews stay invisible to them and cannot be liked. A non-author still gets 403 on edit and delete.

## Capabilities

### New Capabilities

### Modified Capabilities

- `reviews/book-reviews`: `is_hidden` on reviews; deleting a hidden review is allowed for its author; editing one returns 409; the book page shows the hidden own review.
- `reviews/review-moderation`: the effects of a hidden review now allow deletion by its author and a fresh review after that.

## Impact

- backend: reviews module (service delete/update rules, the review response shape, swagger annotations and the generated swagger files).
- frontend: `packages/api-client` (`Review.is_hidden`), apps/web review card entity and reviews-section widget, plus tests.
- devops: none.
- docs: none beyond the specs and swagger.
