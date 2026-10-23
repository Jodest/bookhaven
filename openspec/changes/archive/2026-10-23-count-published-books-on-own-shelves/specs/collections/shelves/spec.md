## MODIFIED Requirements

### Requirement: Listing my shelves
`GET /api/v1/shelves` SHALL return the caller's shelves as `{items, total}`: the default shelves first in the order want to read, reading, read, then custom shelves oldest first. Private custom shelves are included. Each item is `{id, name, type, description, is_public, books_count, created_at}`, where `description` is an empty string when the shelf has none and `books_count` counts the published books on the shelf, the same books the shelf page shows. With `?book_id=…` each item also carries `has_book`, telling whether that book is on the shelf. The shelf in `GET /api/v1/shelves/{id}` also carries `description` and `is_public`.

#### Scenario: Shelf picker for a book
- **WHEN** a reader lists their shelves with `?book_id` of a book on «Читаю»
- **THEN** «Читаю» has `has_book: true` and the other shelves `false`

#### Scenario: Private shelf in my list
- **WHEN** a reader with a private custom shelf «Подарки» lists their shelves
- **THEN** «Подарки» is listed with `is_public: false`

#### Scenario: Unpublished books are not counted
- **WHEN** a reader's «Читаю» holds one published, one pending and one rejected book and they list their shelves
- **THEN** «Читаю» has `books_count: 1`
