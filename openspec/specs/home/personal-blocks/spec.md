# home/personal-blocks Specification

## Purpose

Turns the home page into a reader's starting point: what they are reading, what to read next from their own list, recommendations, and their yearly goal.

## Requirements

### Requirement: Personal home payload
`GET /api/v1/home/me` SHALL require authentication and return the caller's `{continue_reading, recommendations, want_to_read_pick}`. It is never cached. Each block is computed independently: a failed block comes back empty, and only when all three fail does the request return 500. Reading this payload never creates shelves.

#### Scenario: Guest
- **WHEN** a request without a token calls `GET /api/v1/home/me`
- **THEN** the response is 401

### Requirement: Continue reading
`continue_reading` SHALL be `{shelf_id, total, books}` for the caller's «Читаю» shelf: up to 6 of its published books, most recently added first, and `total`, the number of its published books. It is `null` when the caller's default shelves have not been created yet.

#### Scenario: Eight books in progress
- **WHEN** a reader has eight published books on «Читаю»
- **THEN** `books` holds the six most recently added and `total` is 8

### Requirement: Want-to-read pick
`want_to_read_pick` SHALL be `{shelf_id, total, book}` with one published book from the caller's «Хочу прочитать» shelf, picked per user and Moscow date by the same daily rule as the book of the day (see `home/book-of-the-day`). `book` is `null` when the shelf has no published books, and the block is `null` when the default shelves do not exist yet.

#### Scenario: Same pick all day
- **WHEN** a reader reloads the home page several times on one day without changing the shelf
- **THEN** the same book is picked each time

### Requirement: Recommendations
`recommendations` SHALL list up to 10 published books similar to the caller's own books. Seeds are the 50 most recent published books the caller has put on «Читаю» or «Прочитано» or rated 4 or 5, minus books they rated 1 or 2. A recommended book MUST NOT be on any of the caller's shelves or rated by them. Precomputed similarities are summed over the seeds first; when that gives fewer than 6 books, the list is topped up in turns from similar books of the 3 most recent seeds. A reader without seeds gets an empty list.

#### Scenario: Already shelved
- **WHEN** a book similar to a reader's favourites is on their «Хочу прочитать» shelf
- **THEN** it is not recommended

#### Scenario: New reader
- **WHEN** a reader has no books on «Читаю» or «Прочитано» and no ratings of 4 or 5
- **THEN** `recommendations` is empty

### Requirement: Personal blocks on the page
For signed-in users the home page SHALL show in the main column (see `home/home-page`): «Продолжить чтение» with «Все (<total>)» linking to the shelf at the top; «Из вашего «Хочу прочитать»» with «Вся полка (<total>)» under «Книга дня»; and further down «Вам может понравиться». The reading-challenge card (see `reading-challenges/progress`) comes first in the sidebar. Guests and visitors whose session is still being checked see none of them; loading blocks show placeholders, and empty or failed blocks are hidden.

#### Scenario: Guest sees no personal blocks
- **WHEN** a guest opens `/`
- **THEN** no personal block is shown and `GET /api/v1/home/me` is not called
