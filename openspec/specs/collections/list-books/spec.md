# collections/list-books Specification

## Purpose

Lets a reader add books to their lists in a deliberate order and change that order.

## Requirements

### Requirement: Add and remove a book
`POST /api/v1/lists/{id}/books` `{book_id}` SHALL add a published book at the end of the caller's list (201); an unpublished or unknown book MUST return 404 and a book already on the list 409. `DELETE /api/v1/lists/{id}/books/{bookId}` removes it (200); a book not on the list returns 404. A book can be on any number of lists.

#### Scenario: New book goes last
- **WHEN** a reader adds a book to a list with three books
- **THEN** it is listed fourth

### Requirement: Reorder
`PATCH /api/v1/lists/{id}/books/reorder` `{book_ids}` SHALL set the order of the given books to their order in `book_ids`, in one transaction (200). Ids that are not on the list are ignored, and books left out keep their previous position value. The web app does not offer reordering yet; the endpoint is used directly.

#### Scenario: Swapping two books
- **WHEN** a list holds A, B, C and the owner sends `book_ids: [B, A, C]`
- **THEN** the list is shown as B, A, C

### Requirement: Books of a list
`GET /api/v1/lists/{id}` SHALL return the list with its published books in list order (position, then time added), paginated (limit default 20, max 100). A book that is on the list but not published stays on it and is not shown until published again.

#### Scenario: Unpublished book on a list
- **WHEN** a book on a list is returned to moderation
- **THEN** the list's page no longer shows it

### Requirement: Adding from the book page
The book page SHALL let a signed-in reader toggle the book on each of their lists and create a new list on the spot («Добавить в список», «Название нового списка»). Failures show «Ошибка изменения списка» or «Ошибка создания списка».

#### Scenario: New list from a book
- **WHEN** a reader creates a list from the book page
- **THEN** the list is created and can be ticked for the book immediately
