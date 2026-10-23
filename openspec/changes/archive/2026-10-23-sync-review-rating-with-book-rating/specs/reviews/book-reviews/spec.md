## MODIFIED Requirements

### Requirement: A review carries the reader's rating
The reviewer's rating of the book SHALL be the single source of truth for the rating shown on their review: the rating returned with a review (in lists, as `my_review`, and anywhere else a review is shown) MUST always equal the reviewer's current rating of the book. Creating or editing a review SHALL set the reviewer's rating of the book to the review's rating (creating the rating if needed) and recalculate the book's rating, rating count and review count in the same transaction. Rating the book directly later changes the rating and, in the same transaction, the rating shown on the review (see `reviews/book-ratings`). Deleting a review MUST keep the reviewer's rating; a rating cannot be removed, so a review is never left without one.

#### Scenario: Review creates a rating
- **WHEN** a reader who never rated a book reviews it with rating 5
- **THEN** the reader's rating of the book is 5 and the book's `ratings_count` grows by one

#### Scenario: Deleting a review keeps the rating
- **WHEN** a reader deletes their review rated 3
- **THEN** their rating of the book stays 3 and the book's review count drops by one

#### Scenario: Re-rating the book updates the review
- **WHEN** a reader whose review is rated 5 rates the book 2 with the stars
- **THEN** the book's reviews list shows their review with rating 2 and the book's `rating` and `ratings_count` count their rating once, as 2

#### Scenario: Editing the review updates the rating
- **WHEN** a reader who rated a book 4 edits their review of it to rating 1
- **THEN** `GET /api/v1/books/{id}/ratings/me` returns `{value: 1}`

### Requirement: Reviews on the book page
The book page SHALL show «Рецензии» with the visible reviews. A signed-in reader gets «Написать рецензию», or «Редактировать рецензию» when they already have one; the form requires a rating («Выберите оценку») and text («Рецензия не может быть пустой», «Рецензия слишком длинная») and saves with «Опубликовать» or «Сохранить». The author can delete their review after confirming «Удалить рецензию? Ваша оценка книги сохранится.», which tells them their rating of the book stays. An anonymous visitor sees «Войдите, чтобы написать рецензию».

#### Scenario: Anonymous visitor
- **WHEN** a visitor who is not signed in opens a book page
- **THEN** the reviews are shown with «Войдите, чтобы написать рецензию» instead of the form

#### Scenario: Delete confirmation mentions the rating
- **WHEN** the author clicks delete on their review
- **THEN** a confirmation «Удалить рецензию? Ваша оценка книги сохранится.» is shown, and after confirming the review disappears while «Моя оценка: N» stays on the page

#### Scenario: Cancelling the deletion
- **WHEN** the author dismisses the delete confirmation
- **THEN** the review and the rating are unchanged
