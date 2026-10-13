# catalog-admin/catalog-lists Specification

## Purpose

Lets an admin find any book, author or genre, whatever its status, and jump to editing it.

## Requirements

### Requirement: Admin list endpoints
`GET /api/v1/admin/books`, `/authors` and `/genres` SHALL require the `admin` role and return `{items, total, page, limit}` with records of every status (`page` defaults to 1, `limit` to 20, at most 100). `status` filters by a valid status, otherwise 400. Books are listed newest first; authors and genres by name. Merged genres are not listed.

#### Scenario: Invalid status filter
- **WHEN** an admin lists books with `status=hidden`
- **THEN** the response is 400

### Requirement: Search and filters
Book search SHALL be a Russian full-text match on the title, ISBN and description; author search a full-text match on the name and biography; genre search a case-insensitive substring of the name. Books can also be filtered by `author_id`, `genre_id` and `source` (`manual` for books without an external source, `harvested` for books with one); a malformed `author_id` or `genre_id` is ignored. All filters combine.

#### Scenario: Harvested pending books
- **WHEN** an admin lists books with `status=pending&source=harvested`
- **THEN** only pending books that came from a harvest are listed

#### Scenario: Word forms in book search
- **WHEN** an admin searches books for «войны» and «Война и мир» exists
- **THEN** «Война и мир» is listed

### Requirement: Catalog list pages
The admin app SHALL provide «Книги» (`/catalog/books`), «Авторы» (`/catalog/authors`) and «Жанры» (`/catalog/genres`), 20 rows per page, with a search box («Поиск по названию, ISBN, описанию», «Поиск по имени», «Поиск по названию»), a status filter («Все статусы») and a link «Очередь модерации». The book list adds the filters «Автор», «Жанр» (each offering the first 100 by name) and «Источник» («Все», «Добавлено вручную», «Импортировано»), and marks harvested books «Импортировано». The author and genre lists show the Wikidata QID. Every row has «Редактировать», which keeps the list state for the way back, and published rows have «Снять с публикации» (see `catalog-moderation/entity-status`). Search, filters and page are kept in the address. Empty results show «Книги не найдены.», «Авторы не найдены.» or «Жанры не найдены.». The catalog offers no «add» action (see `catalog-import/admin-import-section`).

#### Scenario: Back to the filtered list
- **WHEN** an admin filters books by «Импортировано», opens «Редактировать» and saves
- **THEN** they return to the book list with the same filter and page
