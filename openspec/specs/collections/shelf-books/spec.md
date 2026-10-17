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
- **Status control on the book page.** The book page has a single status control instead of three buttons.
  - **No status.** The control offers «Хочу прочитать», which shelves the book there at once. Next to it, «Выбрать статус» opens a chooser with all three statuses.
  - **With a status.** The control shows the current status as marked, with the icon of that status and a visible dropdown arrow, so it is clear that it opens the status chooser. Every status in the chooser has its own icon, the current one is marked with a check mark on the right, and «Убрать статус» is set apart by a divider. Opening it shows the three statuses, the current one marked, plus «Убрать статус», which removes the book from its status shelf.
  - **After a pick.** Picking closes the chooser.
- **Custom shelves on the book page.** Custom shelves are behind the button «На полку», which opens the checklist «Добавить на полку». Toggling a shelf keeps the checklist open. When the reader has no custom shelves, «На полку» is not shown.
- **Loading.** While the reader's shelves are loading, placeholders of the same size MUST stand in place of the controls, so the controls never disappear and reappear. This applies on the book page and on cards.
- **Not signed in.** A visitor who is not signed in sees «Хочу прочитать» on the book page, leading to sign-in and back to the book. They see no «На полку».
- **Book cards.** Book cards keep their separate status buttons.

#### Scenario: Changing the status
- **WHEN** a reader whose book is on «Хочу прочитать» picks «Читаю»
- **THEN** the book moves to «Читаю» and «Хочу прочитать» is no longer marked

#### Scenario: First status in one click
- **WHEN** a signed-in reader whose book has no status presses «Хочу прочитать» on the book page
- **THEN** the book is on «Хочу прочитать» and the control shows it as the current status

#### Scenario: Removing the status
- **WHEN** a reader whose book is on «Читаю» opens the status control and picks «Убрать статус»
- **THEN** the book is on none of the three default shelves and the control offers «Хочу прочитать» again

#### Scenario: Custom shelves behind «На полку»
- **WHEN** a reader with the custom shelf «Отпуск» opens «На полку» and checks «Отпуск»
- **THEN** the book is added to «Отпуск», its status shelf is unchanged, and the checklist stays open

#### Scenario: Shelves still loading
- **WHEN** a signed-in reader opens a book page and their shelves have not loaded yet
- **THEN** placeholders are shown where the status control and «На полку» will be

#### Scenario: Failure rolls back
- **WHEN** moving a book to «Прочитано» fails
- **THEN** the previous status is marked again and «Ошибка обновления полки» is shown
