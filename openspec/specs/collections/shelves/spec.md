# collections/shelves Specification

## Purpose

Gives every reader the three reading-status shelves plus any number of their own named shelves, and lets them manage those shelves.

## Requirements

### Requirement: Default shelves
Every user SHALL have exactly one shelf of each default type: «Хочу прочитать» (`want_to_read`), «Читаю» (`reading`), «Прочитано» (`read`) and «Брошено» (`abandoned`). They are created on first use, when the user's shelves are listed. A user who already has the first three gets «Брошено» the next time their shelves are listed. Default shelves are always public and have no description. They MUST NOT be renamed, given a description, made private or deleted: such a request returns 400.

#### Scenario: New reader
- **WHEN** a reader who has never opened their shelves lists them
- **THEN** the four default shelves exist and are listed, empty

#### Scenario: Deleting a default shelf
- **WHEN** a reader tries to delete their «Прочитано» shelf
- **THEN** the response is 400 and the shelf stays

#### Scenario: Making a default shelf private
- **WHEN** a reader sends `PATCH /api/v1/shelves/{id}` with `is_public: false` for their «Читаю» shelf
- **THEN** the response is 400 and the shelf stays public

#### Scenario: Existing reader gets «Брошено»
- **WHEN** a reader who has only the three earlier default shelves lists their shelves
- **THEN** an empty «Брошено» shelf is created and listed after «Прочитано»

#### Scenario: Deleting «Брошено»
- **WHEN** a reader tries to delete their «Брошено» shelf
- **THEN** the response is 400 and the shelf stays

### Requirement: Custom shelves
A user SHALL create custom shelves with `POST /api/v1/shelves` `{name, description?, is_public?}` (201) and delete them with `DELETE /api/v1/shelves/{id}`. `PATCH /api/v1/shelves/{id}` `{name?, description?, is_public?}` changes any of the given fields and leaves the others as they are; a request with none of them returns 400. The name is trimmed and MUST be 2–100 characters, otherwise 400. The description is trimmed and MUST be at most 2,000 characters, otherwise 400; an empty description means the shelf has none. `is_public` defaults to `true`. Lengths count characters, not bytes. Deleting a shelf removes its books from it, not the books themselves.

#### Scenario: Too short a name
- **WHEN** a reader creates a shelf named «А»
- **THEN** the response is 400 and no shelf is created

#### Scenario: Default visibility
- **WHEN** a reader creates a custom shelf without `is_public`
- **THEN** the shelf is public

#### Scenario: Too long a description
- **WHEN** a reader saves a 2,001-character description on a custom shelf
- **THEN** the response is 400 and the shelf is unchanged

#### Scenario: Cyrillic name of 100 characters
- **WHEN** a reader names a custom shelf with 100 Cyrillic letters
- **THEN** the shelf is saved

### Requirement: Listing my shelves
`GET /api/v1/shelves` SHALL return the caller's shelves as `{items, total}`: the default shelves first in the order want to read, reading, read, abandoned, then custom shelves oldest first. Private custom shelves are included. Each item is `{id, name, type, description, is_public, books_count, created_at}`, where `description` is an empty string when the shelf has none and `books_count` counts the published books on the shelf, the same books the shelf page shows. With `?book_id=…` each item also carries `has_book`, telling whether that book is on the shelf, and the response carries the reading summary `reads` (see `collections/reading-journal`). The shelf in `GET /api/v1/shelves/{id}` also carries `description` and `is_public`.

#### Scenario: Shelf picker for a book
- **WHEN** a reader lists their shelves with `?book_id` of a book on «Читаю»
- **THEN** «Читаю» has `has_book: true` and the other shelves `false`

#### Scenario: Private shelf in my list
- **WHEN** a reader with a private custom shelf «Подарки» lists their shelves
- **THEN** «Подарки» is listed with `is_public: false`

#### Scenario: Unpublished books are not counted
- **WHEN** a reader's «Читаю» holds one published, one pending and one rejected book and they list their shelves
- **THEN** «Читаю» has `books_count: 1`

#### Scenario: Default order with «Брошено»
- **WHEN** a reader with one custom shelf lists their shelves
- **THEN** the order is «Хочу прочитать», «Читаю», «Прочитано», «Брошено», then the custom shelf

### Requirement: Owner-only access
All `/api/v1/shelves` endpoints SHALL require authentication. Reading or changing another user's shelf through them MUST return 403, and an unknown shelf 404. Others can only view shelves through the public pages (see `collections/public-collections`).

#### Scenario: Someone else's shelf
- **WHEN** a reader requests `GET /api/v1/shelves/{id}` for another reader's shelf
- **THEN** the response is 403

### Requirement: Shelves pages
The web app SHALL provide `/shelves` and `/shelves/{id}`.
- **`/shelves`.** Lists the reader's shelves with book counts. A shelf is created with «Название новой полки» and an optional «Описание (необязательно)». A custom shelf is renamed, or deleted after confirming «Удалить полку? Книги будут убраны из неё.». A private custom shelf has the badge «Приватная», and every custom shelf offers «Сделать приватной» or «Сделать публичной».
- **`/shelves/{id}`.** Shows one shelf with its description and its books, and «На этой полке пока нет книг» when empty. For a custom shelf it offers editing the name and the description together, «Сделать приватной» / «Сделать публичной» with the badge «Приватная» while private, and deleting.
- **Errors.** Name errors show «Минимум 2 символа» / «Максимум 100 символов», a description error «Максимум 2000 символов». A failed privacy change shows «Ошибка изменения видимости полки».
- **Navigation.** The header and the footer link to the shelves and have no «Мои списки» link.

#### Scenario: Deleting a custom shelf
- **WHEN** a reader confirms deleting a custom shelf
- **THEN** the shelf disappears from `/shelves` and its books stay in the catalog

#### Scenario: Making a shelf private
- **WHEN** a reader presses «Сделать приватной» on a public custom shelf
- **THEN** the shelf shows the badge «Приватная» and the button becomes «Сделать публичной»

#### Scenario: Default shelf has no privacy controls
- **WHEN** a reader opens their «Прочитано» shelf
- **THEN** no «Сделать приватной», no description editing and no deleting are offered

### Requirement: Private shelves stay with the owner
A private custom shelf SHALL be fully available to its owner through the authenticated `/api/v1/shelves` endpoints: listing, viewing, editing, deleting, and adding and removing books. It MUST NOT be shown to anyone else, signed in or not (see `collections/public-collections`). Making a shelf private or public changes nothing about its books.

#### Scenario: Adding a book to a private shelf
- **WHEN** the owner adds a published book to their private shelf
- **THEN** the response is 201 and the book is on the shelf

#### Scenario: Owner opens a private shelf
- **WHEN** the owner requests `GET /api/v1/shelves/{id}` for their private shelf
- **THEN** the shelf and its published books are returned with `is_public: false`
