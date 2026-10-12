## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: Independent per-row processing with a report
Each row SHALL be validated and created independently: a failing row MUST NOT prevent the other rows from being processed. The response MUST be HTTP 200 with the report `{processed, created, skipped_existing, errors: [{row, title, message}], created_authors: [{id, name, row}], created_genres: [{id, name, row}]}`, where `row` is 1-based over data rows (the header is not counted), even when some or all rows failed. An empty file (header only, or `[]`) MUST return a zero report, not an error.

#### Scenario: Mixed batch
- **WHEN** a file has 3 rows and the second has an invalid ISBN
- **THEN** the response is 200 with `processed: 3`, `created: 2` and one error for `row: 2`

#### Scenario: Empty file
- **WHEN** an admin uploads `[]`
- **THEN** the response is 200 with `processed: 0, created: 0, skipped_existing: 0, errors: [], created_authors: [], created_genres: []`

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
