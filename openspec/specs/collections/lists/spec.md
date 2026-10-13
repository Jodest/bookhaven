# collections/lists Specification

## Purpose

Lets a reader curate titled, ordered book lists (for example «Лучшее за год»), public by default and optionally private.

## Requirements

### Requirement: Create, edit and delete a list
A user SHALL create a list with `POST /api/v1/lists` `{title, description?, is_public?}` (201), edit it with `PATCH /api/v1/lists/{id}` (any of the three fields) and delete it with `DELETE /api/v1/lists/{id}`. The title is trimmed and MUST be 2–255 characters, the description at most 2,000 characters, otherwise 400. `is_public` defaults to `true`. Deleting a list removes its items, not the books.

#### Scenario: Default visibility
- **WHEN** a reader creates a list without `is_public`
- **THEN** the list is public

#### Scenario: Too long a description
- **WHEN** a reader saves a 2,001-character description
- **THEN** the response is 400 and the list is unchanged

### Requirement: Listing my lists
`GET /api/v1/lists` SHALL return `{items, total}` with the caller's lists, newest first, each `{id, title, description, is_public, books_count, created_at}`, where `books_count` counts every item. With `?book_id=…` the response also carries `book_in_lists`, the ids of the caller's lists that contain that book.

#### Scenario: Add-to-list picker
- **WHEN** a reader lists their lists with `?book_id` of a book on «Лучшее»
- **THEN** `book_in_lists` contains the id of «Лучшее»

### Requirement: Owner-only access
All `/api/v1/lists` endpoints SHALL require authentication. Reading or changing another user's list through them MUST return 403, and an unknown list 404, whether the list is public or private. Others see public lists only through the public page (see `collections/public-collections`).

#### Scenario: Someone else's public list
- **WHEN** a reader requests `GET /api/v1/lists/{id}` for another reader's public list
- **THEN** the response is 403

### Requirement: Lists pages
The web app SHALL provide `/lists` (the reader's lists marked «Публичный» or «Приватный», creating a list with a title and «Описание (необязательно)», deleting after «Удалить список? Книги будут убраны из него.») and `/lists/{id}` (editing the title and description, «Сделать приватным» / «Сделать публичным», removing books, and for a public list «Поделиться», which copies the public link: «Скопировано!»). Field errors show «Минимум 2 символа», «Максимум 255 символов» and «Максимум 2000 символов».

#### Scenario: Sharing a list
- **WHEN** the owner of a public list clicks «Поделиться»
- **THEN** the link to `/lists/public/{id}` is copied and «Скопировано!» is shown
