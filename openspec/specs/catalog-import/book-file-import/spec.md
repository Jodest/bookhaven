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
The report SHALL list every author and every genre that the import created, in `created_authors` and `created_genres`. Each entry is `{id, name, row}`: the created entity's ID, its name as stored, and the 1-based data row whose processing created it. Entries MUST be in creation order. Each entity MUST appear at most once, because a later row with the same name matches the entity already created. An author or genre that already existed before the import, including one reached through a merged genre, MUST NOT be listed. Only rows whose book was created create authors or genres, so a row that ended as a row error or was counted in `skipped_existing` MUST NOT contribute any entry. An entity whose creation was rolled back with its row MUST NOT be listed. When a row that would have created an entity fails or is skipped, a later row that names the same entity and creates its book creates the entity, and the entry MUST carry that later row's number. Both arrays MUST always be present, empty when nothing was created. Because the report is only returned by the upload endpoint, it is visible only to callers with the `admin` role and a bearer token.

#### Scenario: New author and new genre
- **WHEN** a file has one row with author «Иван Новиков» and genre «киберпанк», and neither exists
- **THEN** the report has `created_authors: [{id, name: "Иван Новиков", row: 1}]` and `created_genres: [{id, name: "киберпанк", row: 1}]`

#### Scenario: Existing entities are not listed
- **WHEN** every author and genre named in the file already exists
- **THEN** `created_authors` and `created_genres` are both `[]`

#### Scenario: Same new author on several rows
- **WHEN** rows 2 and 5 both name the same author that did not exist before the import, and both rows create their books
- **THEN** that author is listed once, with `row: 2`

#### Scenario: Entity created by a failed row
- **WHEN** row 3 names a new author and is rejected for an invalid ISBN
- **THEN** row 3 appears in `errors`, the author is not created, and `created_authors` has no entry for it

#### Scenario: Entity created by a skipped row
- **WHEN** a row's book already exists, so the row is counted in `skipped_existing`, and the row names a new genre
- **THEN** the genre is not created and `created_genres` has no entry for it

#### Scenario: New author first named on a failed row
- **WHEN** row 2 names a new author and fails validation, and row 5 names the same author and creates its book
- **THEN** the author is listed once in `created_authors`, with `row: 5`

#### Scenario: Rolled-back entity is not listed
- **WHEN** a row creates a new author and its book creation then fails, so the row's changes are rolled back
- **THEN** `created_authors` has no entry for that author

#### Scenario: Merged genre
- **WHEN** a row names a genre that was merged into another genre
- **THEN** the target genre is linked and nothing is added to `created_genres`

### Requirement: A row changes the catalog only when its book is created
Before creating any author or genre for a row, the system MUST validate the row and run its duplicate check. Validation covers every rule of "Row validation matches single-book creation", including the rule that the row has at least one non-blank author name. The duplicate check is the slug-existence check of "Create-only, duplicates are skipped". A row that ends as a row error or is counted in `skipped_existing` MUST NOT create any author or genre. The row's author creation, genre creation and book creation SHALL be atomic: when any step fails after the checks, nothing created for that row MUST remain in the catalog. Examples are a failed author or genre creation, a book whose slug was taken concurrently, and a database error. A row whose book slug was taken concurrently MUST be counted in `skipped_existing`. Any other failure MUST be a row error. Each row's outcome (created, `skipped_existing`, or the error reported) SHALL be the same as without this rule, with one exception: because the checks now run first, a validation error or an existing slug takes precedence over an author or genre resolution failure on the same row. Apart from that, only the side effects of failed and skipped rows change. The endpoint's authorization is unchanged: the `admin` role and a bearer token.

#### Scenario: Invalid row creates nothing
- **WHEN** a row names an author and a genre that do not exist and has an invalid ISBN
- **THEN** the row is reported in `errors`, and neither the author nor the genre exists in the catalog after the import

#### Scenario: Skipped row creates nothing
- **WHEN** a row's book slug already belongs to a book and the row names a genre that does not exist
- **THEN** the row is counted in `skipped_existing`, the existing book is unchanged, and the genre does not exist in the catalog after the import

#### Scenario: Row without authors creates no genre
- **WHEN** a row has a blank `authors` value and names a genre that does not exist
- **THEN** the row is reported as an error for having no authors, and the genre is not created

#### Scenario: Book creation fails after the authors were created
- **WHEN** a valid row names a new author, and the book creation fails with a database error after the author was created
- **THEN** the row is reported in `errors`, and the author does not exist in the catalog after the import

#### Scenario: Book taken concurrently
- **WHEN** a valid row names a new genre, and another request creates a book with the same slug between the row's duplicate check and its book creation
- **THEN** the row is counted in `skipped_existing`, and the genre does not exist in the catalog after the import

#### Scenario: Second author creation fails
- **WHEN** a valid row names two new authors, «Иван Новиков» is created first, and creating the second author fails
- **THEN** the row is reported in `errors`, and «Иван Новиков» does not exist in the catalog after the import

#### Scenario: Other rows are unaffected
- **WHEN** row 2 fails validation and rows 1 and 3 are valid and name new authors
- **THEN** the authors of rows 1 and 3 are created and their books exist, and nothing from row 2 is created
