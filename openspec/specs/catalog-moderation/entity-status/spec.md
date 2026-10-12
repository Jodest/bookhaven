# catalog-moderation/entity-status Specification

## Purpose

Defines the moderation status that every book, author and genre carries, which status each creation path assigns, how admins change it one entity at a time, and how the status decides what the public site shows.

## Requirements

### Requirement: Status values
Every book, author and genre SHALL have exactly one status: `pending`, `published` or `rejected`. `published` is the only publicly visible status. `rejected` is a tombstone: the entity stays in the database, so automatic imports can recognize it and do not recreate it (see `catalog-import/wikidata-matching`). Admin reads MUST return every status and MUST always include the `status` field; public responses MUST NOT include a `status` field.

#### Scenario: Rejected author is kept
- **WHEN** an admin rejects an author
- **THEN** the author still exists in the admin catalog with status `rejected`

### Requirement: Status on creation
Each creation path SHALL assign a fixed status:
- manual admin creation of an author or genre: `published`; a `status` sent in the body is ignored;
- manual admin creation of a book: the `status` from the body when given (an invalid value is 400), otherwise `published`;
- book file import: the batch status, `published` by default (see `catalog-import/book-file-import`); missing authors are created `published` and missing genres `pending`;
- Wikidata harvest and direct Wikidata author/genre import: `pending`.

#### Scenario: Manual genre ignores a status in the body
- **WHEN** an admin creates a genre with `"status": "pending"` in the body
- **THEN** the genre is created as `published`

#### Scenario: Manual book created for moderation
- **WHEN** an admin creates a book with `"status": "pending"`
- **THEN** the book is created as `pending` and appears in the book moderation queue

### Requirement: Single status change
An admin SHALL change one entity's status through `PATCH /api/v1/admin/books/{id}`, `PATCH /api/v1/admin/authors/{id}` or `PATCH /api/v1/admin/genres/{id}` with an optional `status`. An omitted `status` MUST leave it unchanged, an invalid value MUST return 400, and any status MUST be able to change to any other. A merged genre (alias) MUST be treated as not found (404). These endpoints MUST require the `admin` role and a bearer token.

#### Scenario: Republishing a rejected author
- **WHEN** an admin sets `status: published` on a rejected author
- **THEN** the author becomes published and visible on the public site

#### Scenario: Invalid status
- **WHEN** an admin sends `status: hidden`
- **THEN** the response is 400 and nothing changes

### Requirement: Public visibility of books
A book SHALL be publicly visible only when its status is `published`. A `pending` or `rejected` book MUST return 404 on its public page and MUST be excluded from public lists, search, the home page, similar books, the list of languages, and the contents of shelves and lists. It MUST NOT be possible to add it to a shelf or list, rate it or review it.

#### Scenario: Pending book page
- **WHEN** a visitor opens the public page of a pending book
- **THEN** the response is 404

#### Scenario: Rejected book on a shelf
- **WHEN** a book on a user's shelf is rejected
- **THEN** the shelf no longer shows it

### Requirement: Public visibility of authors
An author SHALL be a public entity (author page, author lists and search, author filter) only when `published`. A book's publication does not depend on its authors: a published book with non-published authors is still shown, and in any public book payload a non-published author MUST appear by name only, without a slug (so it is not a link), photo or birth date. Filtering the public book list by a non-published author MUST return an empty result.

#### Scenario: Book with a pending author
- **WHEN** a published book has one published and one pending author
- **THEN** the public book shows both names, and only the published author links to an author page

#### Scenario: Pending author page
- **WHEN** a visitor opens the public page of a pending author
- **THEN** the response is 404

### Requirement: Public visibility of genres
Only `published` genres SHALL be public. Non-published genres MUST be omitted from the `genres` of every public book payload and from the public genre list. Filtering the public book list by a non-published genre MUST return 200 with an empty result.

#### Scenario: Book with a pending genre
- **WHEN** a published book has a published genre and a pending genre
- **THEN** the public book lists only the published genre

### Requirement: Similar books use published links only
Similar-book ranking SHALL count only published authors as shared authors and only published genres as shared genres.

#### Scenario: Shared pending genre
- **WHEN** two books share only a pending genre
- **THEN** that genre adds nothing to their similarity

### Requirement: Unpublish from the catalog lists
The admin catalog lists of books, authors and genres SHALL offer «Снять с публикации» on published rows only. It MUST ask for confirmation, then set the entity to `pending`, so it returns to its moderation queue (for example «Снять автора «{name}» с публикации? Он вернётся в очередь модерации.»).

#### Scenario: Unpublishing a genre
- **WHEN** an admin confirms «Снять с публикации» on a published genre
- **THEN** the genre becomes pending and appears in the genre moderation queue

### Requirement: Book first-publication time
Every book SHALL carry a first-publication time, which is empty until the book is `published` for the first time. The time MUST be set to the current time on the first transition into `published`, whatever the path: manual creation with status `published` (the default), book file import with batch status `published`, a single status change, or a bulk status change. Once set, it MUST NOT change: unpublishing to `pending` or `rejected` keeps it, and republishing keeps the original time. Editing other book fields MUST NOT change it. A book created `pending` (Wikidata harvest, manual creation or file import with status `pending`) MUST have no first-publication time until it is published. Admin book reads MUST include it as a read-only, nullable `published_at` field; a `published_at` sent in a create or update body MUST be ignored. Public book responses MUST NOT include it.

#### Scenario: Harvested book published from the queue
- **WHEN** an admin publishes a pending harvested book that was created a month ago
- **THEN** its `published_at` is the time of publishing, not its creation time

#### Scenario: Book created published
- **WHEN** an admin creates a book without a `status`
- **THEN** the book is `published` and its `published_at` is set to the creation time

#### Scenario: Pending book has no publication time
- **WHEN** an admin reads a pending book that was never published
- **THEN** its `published_at` is `null`

#### Scenario: Republishing keeps the original time
- **WHEN** an admin unpublishes a book and later publishes it again
- **THEN** its `published_at` still equals the time of its first publication

#### Scenario: Client-sent publication time is ignored
- **WHEN** an admin updates a book with `"published_at": "2000-01-01T00:00:00Z"` in the body
- **THEN** the stored `published_at` is unchanged

### Requirement: New in catalog ordering
The home page row «Новое в каталоге» SHALL list published books ordered by first-publication time, newest first, with ties broken by book id. A book that becomes visible for the first time MUST be able to appear in the row regardless of when it was created. The public book list sorts are not affected.

#### Scenario: Old harvest published today
- **WHEN** a book harvested a month ago is published for the first time today, and no book was published later
- **THEN** it is the first book of «Новое в каталоге» once the home page cache refreshes

#### Scenario: Toggling does not bump a book
- **WHEN** an admin unpublishes and republishes a book first published a year ago
- **THEN** its position in «Новое в каталоге» is the same as before the toggle
