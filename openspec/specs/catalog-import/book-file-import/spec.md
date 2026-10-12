# catalog-import/book-file-import Specification

## Purpose

Lets an admin create a batch of books in one operation by uploading a CSV or JSON file (for example a partner export), with each row processed independently and a per-row report returned.

## Requirements

### Requirement: Admin-only upload endpoint
The system SHALL accept a book file upload at `POST /api/v1/admin/books/import` as `multipart/form-data` with a required `file` field. The endpoint MUST require the `admin` role and a bearer token, like every other state-changing admin route.

#### Scenario: Moderator is refused
- **WHEN** a user with the `moderator` role uploads a file
- **THEN** the response is 403 and nothing is created

#### Scenario: No file part
- **WHEN** an admin sends the request without a `file` part
- **THEN** the response is 400 `file is required`

### Requirement: File format detection
The system SHALL take the format from the optional `format` field (`csv` or `json`). When the field is absent it MUST infer the format from the uploaded filename's extension (`.csv` or `.json`). A format that cannot be determined MUST be rejected with 400.

#### Scenario: Format inferred from extension
- **WHEN** an admin uploads `partners.json` without a `format` field
- **THEN** the file is parsed as JSON

#### Scenario: Unknown extension
- **WHEN** an admin uploads `books.txt` without a `format` field
- **THEN** the response is 400 and no row is processed

### Requirement: File schema
A CSV file MUST have a header row; columns SHALL be matched by name, case-insensitively and in any order. Columns: `title` (required per row), `authors` (`;`-separated names), `genres` (optional, `;`-separated names), and the optional `isbn`, `description`, `cover_url`, `published_year`, `page_count`, `language`, `slug`. A JSON file MUST be an array of objects with the same snake_case field names, where `authors` and `genres` are string arrays. A column missing from the CSV header SHALL behave as an empty value on every row, not as a file error.

#### Scenario: Column order does not matter
- **WHEN** a CSV header is `Authors,TITLE,isbn`
- **THEN** each row's title and authors are read from the matching columns

#### Scenario: Missing authors column
- **WHEN** a CSV file has no `authors` column at all
- **THEN** the file is accepted and every row is reported as a row error for having no authors

### Requirement: Whole-file rejection limits
The system SHALL reject a whole file, processing no rows, when:
- the request body exceeds 2 MiB (413, before any parsing);
- the file cannot be parsed at all (CSV that cannot be read, JSON that is not an array of objects) (400);
- the file has more than 500 data rows (422, with a message pointing to the harvest or seed tools for larger loads).

#### Scenario: Too many rows
- **WHEN** an admin uploads a CSV with 501 data rows
- **THEN** the response is 422 and no book is created

#### Scenario: Broken JSON
- **WHEN** an admin uploads a `.json` file with a syntax error
- **THEN** the response is 400 and no book is created

### Requirement: Independent per-row processing with a report
Each row SHALL be validated and created independently: a failing row MUST NOT prevent the other rows from being processed. The response MUST be HTTP 200 with the report `{processed, created, skipped_existing, errors: [{row, title, message}], created_authors: [{id, name, row}], created_genres: [{id, name, row}]}`, where `row` is 1-based over data rows (the header is not counted), even when some or all rows failed. An empty file (header only, or `[]`) MUST return a zero report, not an error.

#### Scenario: Mixed batch
- **WHEN** a file has 3 rows and the second has an invalid ISBN
- **THEN** the response is 200 with `processed: 3`, `created: 2` and one error for `row: 2`

#### Scenario: Empty file
- **WHEN** an admin uploads `[]`
- **THEN** the response is 200 with `processed: 0, created: 0, skipped_existing: 0, errors: [], created_authors: [], created_genres: []`

### Requirement: Row validation matches single-book creation
A row SHALL be validated with exactly the same rules as creating a single book in the admin (title required, ISBN checksum, year and page-count bounds, supported language, Russian-language content, slug safety). On top of that, a row with no resolvable author name MUST be a row error. An omitted `language` MUST default to `ru`. A non-numeric `published_year` or `page_count` SHALL be treated as not provided.

#### Scenario: Row without authors
- **WHEN** a row has a title but an empty `authors` value
- **THEN** that row is reported as an error and the other rows are processed

#### Scenario: Language default
- **WHEN** a row omits `language`
- **THEN** the book is created with language `ru`

### Requirement: Create-only, duplicates are skipped
The import SHALL only create books. A row whose explicit or generated slug already belongs to a book MUST be counted in `skipped_existing` and MUST NOT modify the existing book. A second row with the same title in one file is therefore skipped too.

#### Scenario: Book already exists
- **WHEN** a row's generated slug equals an existing book's slug
- **THEN** the row is counted as `skipped_existing` and the existing book is unchanged

### Requirement: Batch status
The optional `status` form field SHALL set the status of every created book: `published` (the default) or `pending`. Any other value MUST be rejected with 400. There is no per-row status.

#### Scenario: Import for moderation
- **WHEN** an admin uploads a file with `status=pending`
- **THEN** every created book is pending and appears in the book moderation queue

#### Scenario: Invalid status
- **WHEN** an admin sends `status=rejected`
- **THEN** the response is 400

### Requirement: Author and genre resolution by name
Authors SHALL be matched by exact normalized name slug, with no fuzzy matching. A matched author MUST be linked whatever its status, and a missing author MUST be created as `published` (admin-curated input). Genres SHALL be matched by name slug, following a merged genre to its target, and linked whatever their status. A missing genre MUST be created as `pending`, because free-text labels need review.

#### Scenario: Unknown genre label
- **WHEN** a row lists the genre «киберпанк» that does not exist
- **THEN** a pending genre «киберпанк» is created and linked, and it stays hidden on the public site until published

### Requirement: File import is not harvesting
Books created by a file import SHALL have no external source or external ID, so they are never treated as harvested. The import MUST NOT call any external network service.

#### Scenario: Source filter
- **WHEN** an admin filters the book list by the harvested source
- **THEN** file-imported books are not listed

### Requirement: File import page
The admin SHALL provide the file import at `/import/books/file`. The page has a `.csv`/`.json` file picker, a batch status select («Опубликованы» / «На модерации»), a submit button, and a result view with the counts and a per-row error table. 400, 413 and 422 responses MUST be shown as readable messages. When the report lists created authors or genres, the result view MUST show a «Созданные авторы (N)» and/or a «Созданные жанры (N)» list. Each entry shows the name, linked to the entity's edit page (`/catalog/authors/{id}/edit` or `/catalog/genres/{id}/edit`), and its row number («строка N»). The genres list MUST carry the note «Новые жанры созданы на модерации и не видны на сайте до публикации.» A list with no entries MUST NOT be rendered.

#### Scenario: Oversized upload message
- **WHEN** the server answers 413
- **THEN** the page shows a message that the file is too large instead of a generic error

#### Scenario: Created entities shown
- **WHEN** the report has two created authors and one created genre
- **THEN** the result view shows «Созданные авторы (2)» with two linked names and «Созданные жанры (1)» with one linked name and the pending note

#### Scenario: Nothing created
- **WHEN** the report has empty `created_authors` and `created_genres`
- **THEN** neither list is shown

### Requirement: Created authors and genres in the report
The report SHALL list every author and every genre that the import created, in `created_authors` and `created_genres`. Each entry is `{id, name, row}`: the created entity's ID, its name as stored, and the 1-based data row whose processing created it. Entries MUST be in creation order. Each entity MUST appear at most once, because a later row with the same name matches the entity already created. An author or genre that already existed before the import, including one reached through a merged genre, MUST NOT be listed. An entity created while processing a row MUST be listed even when that row then ended as a row error or was counted in `skipped_existing`, because the entity stays in the catalog. Both arrays MUST always be present, empty when nothing was created. Because the report is only returned by the upload endpoint, it is visible only to callers with the `admin` role and a bearer token.

#### Scenario: New author and new genre
- **WHEN** a file has one row with author «Иван Новиков» and genre «киберпанк», and neither exists
- **THEN** the report has `created_authors: [{id, name: "Иван Новиков", row: 1}]` and `created_genres: [{id, name: "киберпанк", row: 1}]`

#### Scenario: Existing entities are not listed
- **WHEN** every author and genre named in the file already exists
- **THEN** `created_authors` and `created_genres` are both `[]`

#### Scenario: Same new author on several rows
- **WHEN** rows 2 and 5 both name the same author that did not exist before the import
- **THEN** that author is listed once, with `row: 2`

#### Scenario: Entity created by a failed row
- **WHEN** row 3 names a new author and is then rejected for an invalid ISBN
- **THEN** the author is listed in `created_authors` with `row: 3`, and row 3 appears in `errors`

#### Scenario: Entity created by a skipped row
- **WHEN** a row's book already exists, so the row is counted in `skipped_existing`, and the row names a new genre
- **THEN** the genre is listed in `created_genres` with that row's number

#### Scenario: Merged genre
- **WHEN** a row names a genre that was merged into another genre
- **THEN** the target genre is linked and nothing is added to `created_genres`
