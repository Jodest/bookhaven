## MODIFIED Requirements

### Requirement: List a book's reviews
`GET /api/v1/books/{id}/reviews` SHALL be public (optional authentication) and return `{items, total, page, limit}` with the book's visible reviews, newest first; `limit` defaults to 20 and is capped at 100. Each review is `{id, book_id, user_id, user: {id, name, avatar_url}, rating, content, likes_count, liked_by_me, is_hidden, created_at, updated_at}`; `liked_by_me` is false for anonymous callers. `is_hidden` tells whether a moderator hid the review; it is always false in `items`, because hidden reviews are never listed there. For a signed-in caller who has reviewed the book, the response MUST also carry `my_review`, even when that review is hidden; in that case `my_review.is_hidden` MUST be true. A book that is not published MUST return 404.

#### Scenario: Anonymous reader
- **WHEN** a visitor lists the reviews of a published book
- **THEN** only visible reviews are returned and none is marked as liked

#### Scenario: Own review is returned separately
- **WHEN** a signed-in reader who reviewed the book lists its reviews
- **THEN** the response includes `my_review` with their review

#### Scenario: Own hidden review is marked
- **WHEN** a signed-in reader whose review of the book was hidden by a moderator lists its reviews
- **THEN** `items` does not contain their review, and `my_review` contains it with `is_hidden: true`

### Requirement: Write a review
`POST /api/v1/books/{id}/reviews` SHALL require authentication and accept `{rating, content}`: `rating` 1–5 and `content` 1–10,000 characters after trimming, otherwise 400. The book MUST be published (404 otherwise). A user can review a book only once: a second review MUST return 409 «already reviewed», also when the first one is hidden. The author of a hidden review can delete it (see "Edit and delete own review") and then write a new review, which is visible until a moderator hides it. On success the response is 201 with the review.

#### Scenario: Second review
- **WHEN** a reader who already reviewed a book submits another review of it
- **THEN** the response is 409

#### Scenario: Whitespace-only text
- **WHEN** a reader submits a review whose text is only spaces
- **THEN** the response is 400 and nothing is saved

#### Scenario: New review after deleting a hidden one
- **WHEN** a reader deletes their hidden review of a book and then submits a new review of it
- **THEN** the response is 201 with `is_hidden: false`, the review appears in the book's public reviews, and the book's `reviews_count` grows by one

### Requirement: Edit and delete own review
`PATCH /api/v1/reviews/{id}` (with `rating` and/or `content`, the same rules; neither given is 400) and `DELETE /api/v1/reviews/{id}` SHALL require authentication (bearer token) and be allowed only to the review's author. For both, an unknown review MUST return 404 and another user MUST get 403, also when the review is hidden. Editing a hidden review MUST be refused for its author with 409 «review is hidden», and the review stays unchanged, so editing cannot get around moderation. Deleting a hidden review MUST be allowed for its author and behave exactly like deleting a visible one: the reviewer's rating is kept and the book's aggregates are recalculated (see "Review count"). A deleted review's likes and the notifications about it are removed with it, whether or not it was hidden.

#### Scenario: Editing someone else's review
- **WHEN** a reader tries to edit another reader's review
- **THEN** the response is 403 and the review is unchanged

#### Scenario: Editing a hidden review
- **WHEN** the author tries to edit their review after a moderator hid it
- **THEN** the response is 409 «review is hidden» and the review's text and rating are unchanged

#### Scenario: Deleting a hidden review
- **WHEN** the author deletes their review rated 4 after a moderator hid it, and the review had likes and like notifications
- **THEN** the response is 200, the review, its likes and their notifications are gone, the author's rating of the book stays 4, and the book's `reviews_count` is unchanged

#### Scenario: Deleting someone else's hidden review
- **WHEN** a reader tries to delete another reader's hidden review
- **THEN** the response is 403 and the review is unchanged

### Requirement: Review count
A book's `reviews_count` SHALL count its visible reviews only, recalculated when a review is created, edited, deleted, hidden or restored.

#### Scenario: Hidden review is not counted
- **WHEN** a book has three reviews and one is hidden
- **THEN** its `reviews_count` is 2

#### Scenario: Deleting a hidden review keeps the count
- **WHEN** a book has two visible reviews and one hidden review, and the author of the hidden review deletes it
- **THEN** its `reviews_count` stays 2

### Requirement: Reviews on the book page
The book page SHALL show «Рецензии» with the visible reviews. A signed-in reader gets «Написать рецензию», or «Редактировать рецензию» when they already have a visible review; the form requires a rating («Выберите оценку») and text («Рецензия не может быть пустой», «Рецензия слишком длинная») and saves with «Опубликовать» or «Сохранить». The author can delete their review after confirming «Удалить рецензию? Ваша оценка книги сохранится.», which tells them their rating of the book stays. An anonymous visitor sees «Войдите, чтобы написать рецензию».

When the signed-in reader's own review is hidden by a moderator, the page MUST show it to them as their own card, above the list, with the mark «Скрыт модератором» and the note «Модератор скрыл эту рецензию — другие читатели её не видят. Вы можете удалить её и написать новую.». That card MUST offer only «Удалить», with the same confirmation, and no edit or like button. Neither «Написать рецензию» nor «Редактировать рецензию» is shown while the hidden review exists. After the deletion, «Написать рецензию» appears.

#### Scenario: Anonymous visitor
- **WHEN** a visitor who is not signed in opens a book page
- **THEN** the reviews are shown with «Войдите, чтобы написать рецензию» instead of the form

#### Scenario: Delete confirmation mentions the rating
- **WHEN** the author clicks delete on their review
- **THEN** a confirmation «Удалить рецензию? Ваша оценка книги сохранится.» is shown, and after confirming the review disappears while «Моя оценка: N» stays on the page

#### Scenario: Cancelling the deletion
- **WHEN** the author dismisses the delete confirmation
- **THEN** the review and the rating are unchanged

#### Scenario: Own hidden review on the book page
- **WHEN** a reader whose review of the book was hidden by a moderator opens the book page
- **THEN** they see their review with «Скрыт модератором» and the explanation, with only «Удалить» and no like button, and neither «Написать рецензию» nor «Редактировать рецензию» is shown

#### Scenario: Deleting the hidden review from the book page
- **WHEN** that reader clicks «Удалить» on their hidden review and confirms «Удалить рецензию? Ваша оценка книги сохранится.»
- **THEN** the hidden card disappears, «Моя оценка: N» stays, and «Написать рецензию» is shown
