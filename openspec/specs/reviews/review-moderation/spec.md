# reviews/review-moderation Specification

## Purpose

Lets moderators and admins hide inappropriate reviews from the site and restore them, without deleting the review or its author's rating.

## Requirements

### Requirement: Review list for moderation
`GET /api/v1/admin/reviews` SHALL require the `admin` or `moderator` role and return all reviews of all books, newest first, with the book title and the author, filtered by `visibility=all|visible|hidden` (default all). `limit` defaults to 20 and is capped at 100.

#### Scenario: Hidden reviews only
- **WHEN** a moderator requests `?visibility=hidden`
- **THEN** only hidden reviews are listed

#### Scenario: Reader is refused
- **WHEN** a user with the role `user` requests the list
- **THEN** the response is 403

### Requirement: Hide and restore
`PATCH /api/v1/admin/reviews/{id}` with `{is_hidden}` SHALL require the `admin` or `moderator` role and a bearer token, and return 200 with the review (404 when it does not exist). Hiding or restoring MUST change only the review's visibility and recalculate the book's visible review count in the same transaction; setting the visibility it already has changes nothing. Hiding MUST NOT delete the review, its likes, its notifications or its author's rating.

#### Scenario: Hiding updates the count
- **WHEN** a moderator hides one of a book's three visible reviews
- **THEN** the book's `reviews_count` becomes 2 and the reviewer's rating still counts in the book's average

### Requirement: Effects of a hidden review
A hidden review SHALL NOT appear in the book's public review list or on the home page. Nobody can like it (404). Its author cannot edit it (409 «review is hidden»), but MUST be able to delete it. Deleting keeps the author's rating and removes the review's likes and notifications, exactly like deleting a visible review. While the hidden review exists, its author cannot write a new review of the book (409); after deleting it, they can. The author still receives it as `my_review` when listing the book's reviews, marked with `is_hidden: true`, and the book page shows it to them as hidden.

#### Scenario: Restoring brings it back
- **WHEN** a moderator restores a hidden review
- **THEN** it appears again in the book's public reviews and counts in `reviews_count`

#### Scenario: Author deletes the hidden review
- **WHEN** the author deletes their hidden review
- **THEN** it disappears from the moderation list, the author's rating still counts in the book's average, and the author can write a new review of the book

### Requirement: Review moderation page
The admin section «Модерация» SHALL have the tab «Отзывы» (`/moderation/reviews`) for admins and moderators, titled «Модерация отзывов», with the filter «Все» / «Видимые» / «Скрытые», the number of hidden reviews, and a table with the book, the user, the rating, the review text, the status («Видим» / «Скрыт») and an action «Скрыть» or «Восстановить». Each action MUST ask «Вы уверены, что хотите скрыть этот отзыв?» (or «восстановить») and then report «Отзыв скрыт. Счетчик отзывов книги обновлен.» or «Отзыв восстановлен. Счетчик отзывов книги обновлен.»; a failure shows «Не удалось обновить отзыв.». For a moderator this is the admin app's home page.

#### Scenario: Moderator signs in
- **WHEN** a moderator signs in to the admin app
- **THEN** they land on the review moderation page
