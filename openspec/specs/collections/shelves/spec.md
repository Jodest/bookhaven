# collections/shelves Specification

## Purpose

Gives every reader the three reading-status shelves plus any number of their own named shelves, and lets them manage those shelves.

## Requirements

### Requirement: Default shelves
Every user SHALL have exactly one shelf of each default type: «Хочу прочитать» (`want_to_read`), «Читаю» (`reading`) and «Прочитано» (`read`). They are created on first use, when the user's shelves are listed. Default shelves MUST NOT be renamed or deleted: such a request returns 400.

#### Scenario: New reader
- **WHEN** a reader who has never opened their shelves lists them
- **THEN** the three default shelves exist and are listed, empty

#### Scenario: Deleting a default shelf
- **WHEN** a reader tries to delete their «Прочитано» shelf
- **THEN** the response is 400 and the shelf stays

### Requirement: Custom shelves
A user SHALL create custom shelves with `POST /api/v1/shelves` `{name}` (201), rename them with `PATCH /api/v1/shelves/{id}` `{name}` and delete them with `DELETE /api/v1/shelves/{id}`. The name is trimmed and MUST be 2–100 characters, otherwise 400. Deleting a shelf removes its books from it, not the books themselves.

#### Scenario: Too short a name
- **WHEN** a reader creates a shelf named «А»
- **THEN** the response is 400 and no shelf is created

### Requirement: Listing my shelves
`GET /api/v1/shelves` SHALL return the caller's shelves as `{items, total}`: the default shelves first in the order want to read, reading, read, then custom shelves oldest first. Each item is `{id, name, type, books_count, created_at}`, where `books_count` counts every book on the shelf. With `?book_id=…` each item also carries `has_book`, telling whether that book is on the shelf.

#### Scenario: Shelf picker for a book
- **WHEN** a reader lists their shelves with `?book_id` of a book on «Читаю»
- **THEN** «Читаю» has `has_book: true` and the other shelves `false`

### Requirement: Owner-only access
All `/api/v1/shelves` endpoints SHALL require authentication. Reading or changing another user's shelf through them MUST return 403, and an unknown shelf 404. Others can only view shelves through the public pages (see `collections/public-collections`).

#### Scenario: Someone else's shelf
- **WHEN** a reader requests `GET /api/v1/shelves/{id}` for another reader's shelf
- **THEN** the response is 403

### Requirement: Shelves pages
The web app SHALL provide `/shelves` (the reader's shelves with book counts, creating a shelf with «Название новой полки», renaming and deleting custom shelves after confirming «Удалить полку? Книги будут убраны из неё.») and `/shelves/{id}` (one shelf with its books and «На этой полке пока нет книг» when empty). Name errors show «Минимум 2 символа» / «Максимум 100 символов».

#### Scenario: Deleting a custom shelf
- **WHEN** a reader confirms deleting a custom shelf
- **THEN** the shelf disappears from `/shelves` and its books stay in the catalog
