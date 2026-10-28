# catalog-import/book-lookup Specification

## Purpose

Lets an admin pre-fill the manual book creation form from Open Library by ISBN or by title and author, so known data does not have to be retyped.

## Requirements

### Requirement: Lookup endpoint
The system SHALL expose `GET /api/v1/admin/books/lookup` with the query parameters `isbn`, or `q` (title) plus an optional `author`. It MUST return 200 with a normalized record (`title`, `authors[]`, `genres[]`, `isbn`, `description`, `cover_url`, `published_year`, `page_count`, `language`, `source_name`, `source_id`) when Open Library has a match.

#### Scenario: Lookup by ISBN
- **WHEN** an admin requests `?isbn=9785170900000` and Open Library knows the book
- **THEN** the response is 200 with the normalized record

#### Scenario: Lookup by title and author
- **WHEN** an admin requests `?q=Война и мир&author=Толстой`
- **THEN** the system searches by title and author and returns the best match

### Requirement: Lookup errors
The lookup MUST return 400 when neither `isbn` nor `q` is given, 404 when the source has no match, and 502 when Open Library is rate-limited or unavailable. A title search answer that reports more than 10 000 matching books MUST be treated as no match (404), because it means Open Library ignored the query. The lookup does not otherwise check that the returned title or author matches the request: the record is a prefill the admin reviews.

#### Scenario: Source unavailable
- **WHEN** Open Library times out after retries
- **THEN** the response is 502 with a message to retry or enter the data manually

#### Scenario: Query ignored by Open Library
- **WHEN** an admin requests `?q=История Петра I&author=Александр Сергеевич Пушкин` and Open Library answers with 907 849 matching books, the first being "I, Robot"
- **THEN** the response is 404 and the form shows the lookup-miss message

### Requirement: Lookup is read-only and admin-only
The lookup SHALL NOT write to the database or create books, authors or genres. It MUST require the `admin` role. Being a read, it accepts cookie or bearer auth.

#### Scenario: Moderator
- **WHEN** a moderator calls the lookup
- **THEN** the response is 403

### Requirement: Form pre-fill
The manual book form SHALL offer a lookup action that uses the typed ISBN or title. On success it MUST overwrite only the scalar fields the record actually returned (title, description, ISBN, cover URL, year, page count, language), never clearing a field with an empty value. Authors and genres MUST NOT be selected automatically; the returned names are shown as hints so the admin picks existing records. Lookup errors MUST be shown inline and MUST NOT block manual entry or saving.

#### Scenario: Partial record
- **WHEN** the lookup returns a record without a cover URL
- **THEN** the form's cover URL keeps whatever the admin had typed

#### Scenario: Lookup miss
- **WHEN** the lookup returns 404
- **THEN** an inline message appears and the admin can still fill and save the form
