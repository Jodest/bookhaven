# collections/public-collections Specification

## Purpose

Lets anyone look at a reader's shelves and public lists, from the reader's public profile or by a shared link, without exposing private lists or unpublished books.

## Requirements

### Requirement: Shelves are always public
Shelves SHALL have no privacy setting. `GET /api/v1/shelves/public/{id}` MUST return any existing shelf to anyone, signed in or not, as `{shelf, items, total, page, limit}` with its published books only; an unknown shelf is 404.

#### Scenario: Anonymous API call
- **WHEN** a client without a token requests a reader's «Читаю» shelf through the public endpoint
- **THEN** the shelf and its published books are returned

### Requirement: Only public lists are public
`GET /api/v1/lists/public/{id}` SHALL return a list to anyone only when it is public, with its published books in list order. A private list MUST return 404, even to its owner, exactly like an unknown list.

#### Scenario: Private list link
- **WHEN** anyone opens the public endpoint of a private list
- **THEN** the response is 404

#### Scenario: List made private after sharing
- **WHEN** an owner makes a shared list private
- **THEN** the shared link stops working

### Requirement: Collections on the public profile
A user's public profile SHALL list their shelves that hold at least one published book, with `books_count` counting published books only, and all their public lists, with `books_count` counting every item.

#### Scenario: Empty shelves are not shown
- **WHEN** a reader's «Хочу прочитать» shelf only holds pending books
- **THEN** the shelf does not appear on their public profile

### Requirement: Public pages
The web app SHALL provide `/shelves/public/{id}` («Полка», «На этой полке пока нет книг», «Полка не найдена») and `/lists/public/{id}` («Публичный список», «В этом списке пока нет книг», «Список не найден или недоступен»), linked from the public profile. Because every page under `/shelves` is guarded (see `auth/authorization`), a visitor without any session cookie who opens `/shelves/public/{id}` is sent to the login page first; `/lists/public/{id}` opens for everyone.

#### Scenario: Anonymous visitor follows a public list link
- **WHEN** a visitor who is not signed in opens `/lists/public/{id}` of a public list
- **THEN** the list is shown

#### Scenario: Anonymous visitor follows a public shelf link
- **WHEN** a visitor without session cookies opens `/shelves/public/{id}`
- **THEN** they are redirected to `/login?redirect=/shelves/public/{id}`
