## ADDED Requirements

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

## MODIFIED Requirements

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
