## MODIFIED Requirements

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
