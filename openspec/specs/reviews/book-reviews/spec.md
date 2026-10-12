# reviews/book-reviews Specification

## Purpose

Lets a signed-in reader write one review with a rating per book, edit or delete it, and lets everyone read a book's visible reviews.

## Requirements

### Requirement: List a book's reviews
`GET /api/v1/books/{id}/reviews` SHALL be public (optional authentication) and return `{items, total, page, limit}` with the book's visible reviews, newest first; `limit` defaults to 20 and is capped at 100. Each review is `{id, book_id, user_id, user: {id, name, avatar_url}, rating, content, likes_count, liked_by_me, created_at, updated_at}`; `liked_by_me` is false for anonymous callers. For a signed-in caller who has reviewed the book, the response MUST also carry `my_review`, even when that review is hidden. A book that is not published MUST return 404.

#### Scenario: Anonymous reader
- **WHEN** a visitor lists the reviews of a published book
- **THEN** only visible reviews are returned and none is marked as liked

#### Scenario: Own review is returned separately
- **WHEN** a signed-in reader who reviewed the book lists its reviews
- **THEN** the response includes `my_review` with their review

### Requirement: Write a review
`POST /api/v1/books/{id}/reviews` SHALL require authentication and accept `{rating, content}`: `rating` 1–5 and `content` 1–10,000 characters after trimming, otherwise 400. The book MUST be published (404 otherwise). A user can review a book only once: a second review MUST return 409 «already reviewed», also when the first one is hidden. On success the response is 201 with the review.

#### Scenario: Second review
- **WHEN** a reader who already reviewed a book submits another review of it
- **THEN** the response is 409

#### Scenario: Whitespace-only text
- **WHEN** a reader submits a review whose text is only spaces
- **THEN** the response is 400 and nothing is saved

### Requirement: A review carries the reader's rating
Creating or editing a review SHALL set the reviewer's rating of the book to the review's rating (creating the rating if needed) and recalculate the book's rating, rating count and review count in the same transaction. Rating the book directly later changes the rating but not the rating shown on the review. Deleting a review MUST keep the reviewer's rating.

#### Scenario: Review creates a rating
- **WHEN** a reader who never rated a book reviews it with rating 5
- **THEN** the reader's rating of the book is 5 and the book's `ratings_count` grows by one

#### Scenario: Deleting a review keeps the rating
- **WHEN** a reader deletes their review rated 3
- **THEN** their rating of the book stays 3 and the book's review count drops by one

### Requirement: Edit and delete own review
`PATCH /api/v1/reviews/{id}` (with `rating` and/or `content`, the same rules; neither given is 400) and `DELETE /api/v1/reviews/{id}` SHALL require authentication and be allowed only to the review's author: another user MUST get 403, and an unknown or hidden review 404. A deleted review's likes and the notifications about it are removed with it.

#### Scenario: Editing someone else's review
- **WHEN** a reader tries to edit another reader's review
- **THEN** the response is 403 and the review is unchanged

#### Scenario: Editing a hidden review
- **WHEN** the author tries to edit their review after a moderator hid it
- **THEN** the response is 404

### Requirement: Review count
A book's `reviews_count` SHALL count its visible reviews only, recalculated when a review is created, edited, deleted, hidden or restored.

#### Scenario: Hidden review is not counted
- **WHEN** a book has three reviews and one is hidden
- **THEN** its `reviews_count` is 2

### Requirement: Reviews on the book page
The book page SHALL show «Рецензии» with the visible reviews. A signed-in reader gets «Написать рецензию», or «Редактировать рецензию» when they already have one; the form requires a rating («Выберите оценку») and text («Рецензия не может быть пустой», «Рецензия слишком длинная») and saves with «Опубликовать» or «Сохранить». The author can delete their review after confirming «Удалить рецензию?». An anonymous visitor sees «Войдите, чтобы написать рецензию».

#### Scenario: Anonymous visitor
- **WHEN** a visitor who is not signed in opens a book page
- **THEN** the reviews are shown with «Войдите, чтобы написать рецензию» instead of the form
