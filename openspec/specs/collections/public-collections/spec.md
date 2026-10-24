# collections/public-collections Specification

## Purpose

Lets anyone look at a reader's public shelves, from the reader's public profile or by a shared link, without exposing private shelves or unpublished books.

## Requirements

### Requirement: Collections on the public profile
A user's public profile SHALL list their public shelves that hold at least one published book. `books_count` counts published books only, and each shelf carries its `description`. Private shelves MUST NOT be listed, whatever they hold. The profile lists no other user collections.

#### Scenario: Empty shelves are not shown
- **WHEN** a reader's «Хочу прочитать» shelf only holds pending books
- **THEN** the shelf does not appear on their public profile

#### Scenario: Private shelf is not shown
- **WHEN** a reader's private custom shelf holds published books
- **THEN** the shelf does not appear on their public profile

### Requirement: Public pages
The web app SHALL provide `/shelves/public/{id}` («Полка», «На этой полке пока нет книг», «Полка не найдена»), linked from the public profile. It shows the shelf's description when there is one. A private or unknown shelf shows «Полка не найдена». The page MUST open for everyone, signed in or not (see `auth/authorization`). There is no public list page: `/lists/public/{id}` no longer exists and is not redirected.

#### Scenario: Anonymous visitor follows a public list link
- **WHEN** a visitor opens an old `/lists/public/{id}` address
- **THEN** the site's not-found page is shown

#### Scenario: Anonymous visitor follows a public shelf link
- **WHEN** a visitor without session cookies opens `/shelves/public/{id}`
- **THEN** the shelf is shown without a redirect to login

#### Scenario: Private shelf page
- **WHEN** anyone opens `/shelves/public/{id}` of a private shelf
- **THEN** «Полка не найдена» is shown

### Requirement: Public shelf endpoint
`GET /api/v1/shelves/public/{id}` SHALL be open to anyone, signed in or not (optional authentication, see `auth/authorization`). For a public shelf it returns `{shelf, items, total, page, limit}` with only the shelf's published books, most recently added first. `shelf` includes `description` and `is_public`. A private shelf MUST return 404, exactly like an unknown shelf, even when the owner asks. Default shelves are always public.

#### Scenario: Anonymous API call
- **WHEN** a client without a token requests a reader's «Читаю» shelf through the public endpoint
- **THEN** the shelf and its published books are returned

#### Scenario: Private shelf link
- **WHEN** anyone, the owner included, requests the public endpoint of a private shelf
- **THEN** the response is 404

#### Scenario: Shelf made private after sharing
- **WHEN** an owner makes a shared custom shelf private
- **THEN** the shared link stops working

### Requirement: Read counter on public shelves
The public shelf page `/shelves/public/{id}` SHALL show «Прочитано <N> <раз|раза|раз>» under each book the shelf owner finished at least once, using the `reads_count` of the public shelf endpoint and the same plural rules as the book page. Books the owner never finished show no counter. The counter is shown only through a public shelf. A private shelf MUST keep returning 404, so its books' reading history is never exposed. The public profile lists shelves without books and shows no counter.

#### Scenario: Visitor sees a re-read
- **WHEN** a visitor who is not signed in opens a reader's public «Прочитано» shelf holding a book that reader finished three times
- **THEN** the book shows «Прочитано 3 раза»

#### Scenario: Private shelf stays hidden
- **WHEN** anyone opens `/shelves/public/{id}` of a private custom shelf holding re-read books
- **THEN** «Полка не найдена» is shown and no read counts are returned
