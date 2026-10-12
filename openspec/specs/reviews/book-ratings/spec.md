# reviews/book-ratings Specification

## Purpose

Lets a signed-in reader rate a book from 1 to 5 and keeps each book's average rating and rating count up to date.

## Requirements

### Requirement: Set a rating
The system SHALL accept `POST /api/v1/books/{id}/ratings` with `{value}` from an authenticated user. `value` MUST be an integer from 1 to 5, otherwise 400. The book MUST be published, otherwise 404. A user has at most one rating per book: the first call creates it (201), later calls replace its value (200). The response is `{book_id, value, rating, ratings_count}` with the book's updated average and count.

#### Scenario: First rating
- **WHEN** a reader rates a published book 4 for the first time
- **THEN** the response is 201 with `value: 4` and the book's new average and count

#### Scenario: Changing a rating
- **WHEN** the reader rates the same book 2
- **THEN** the response is 200, the reader still has one rating, and it is 2

#### Scenario: Pending book
- **WHEN** a reader rates a book that is not published
- **THEN** the response is 404

### Requirement: Read my rating
`GET /api/v1/books/{id}/ratings/me` SHALL return `{value}` of the authenticated user's rating for the book, or 404 when the user has not rated it.

#### Scenario: Not rated yet
- **WHEN** a reader who has not rated a book asks for their rating
- **THEN** the response is 404

### Requirement: Book aggregates
A book's `rating` SHALL be the average of all its users' ratings rounded to 2 decimals (0 when there are none), and `ratings_count` their number. Both MUST be recalculated in the same transaction as every rating change, including ratings written through reviews (see `reviews/book-reviews`). Hiding a review does not remove its author's rating from the aggregates.

#### Scenario: Average
- **WHEN** a book has ratings 5 and 4
- **THEN** its `rating` is 4.5 and `ratings_count` is 2

### Requirement: Rating widget
The book page SHALL show signed-in readers a 1–5 star control (each star labelled «Оценить на N») with «Моя оценка: N» once rated; changing it saves immediately, and a failure shows «Ошибка сохранения оценки».

#### Scenario: Rating from the book page
- **WHEN** a signed-in reader clicks the fourth star
- **THEN** the rating 4 is saved and shown as «Моя оценка: 4»
