# collections/shelf-books Specification

## Purpose

Lets a reader put books on their shelves, where the three default shelves act as a single reading status and custom shelves are free tags.

## Requirements

### Requirement: Add and remove a book
`POST /api/v1/shelves/{id}/books` `{book_id}` SHALL add a published book to the caller's shelf (201); an unpublished or unknown book MUST return 404 and a book already on that shelf 409. `DELETE /api/v1/shelves/{id}/books/{bookId}` removes it (200); a book not on the shelf returns 404.

#### Scenario: Adding twice
- **WHEN** a reader adds a book to a shelf it is already on
- **THEN** the response is 409

### Requirement: One reading status
A book SHALL be on at most one of the default shelves of a user. Adding a book to a default shelf MUST, in the same transaction, remove it from the user's other two default shelves. Custom shelves are independent: a book can be on any number of them and on a default shelf at the same time.

#### Scenario: Moving to «Прочитано»
- **WHEN** a book on «Читаю» is added to «Прочитано»
- **THEN** it is on «Прочитано» only among the default shelves

#### Scenario: Custom shelf alongside a status
- **WHEN** a book on «Читаю» is added to the custom shelf «Отпуск»
- **THEN** it stays on «Читаю» and is also on «Отпуск»

### Requirement: Books of a shelf
`GET /api/v1/shelves/{id}` SHALL return `{shelf, items, total, page, limit}` with the shelf's published books, most recently added first (limit default 20, max 100). A book that is on the shelf but not published is kept on the shelf and reappears when it is published again, but MUST NOT be listed while unpublished.

#### Scenario: Book unpublished after shelving
- **WHEN** a book on a reader's shelf is rejected by a moderator
- **THEN** the shelf's book list no longer shows it, and it comes back if the book is published again

### Requirement: Shelving from the book page
The book page and book cards SHALL let a signed-in reader pick the reading status («Хочу прочитать», «Читаю», «Прочитано»; picking the current one again removes it) and toggle custom shelves («Добавить на полку»). Changes apply immediately, and a failure shows «Ошибка обновления полки».

#### Scenario: Changing the status
- **WHEN** a reader whose book is on «Хочу прочитать» picks «Читаю»
- **THEN** the book moves to «Читаю» and «Хочу прочитать» is no longer marked
