## MODIFIED Requirements

### Requirement: Queuing a copy
Every save that writes a non-empty cover link SHALL queue a copy of that book once the save is committed. This covers:
- an admin creating or updating a book;
- each row of a CSV/JSON book import;
- a book created by a Wikidata harvest, including a cover filled in from Open Library by ISBN, by Open Library ID or by the title search;
- a cover link set by the cover backfill by Open Library ID (see `catalog-import/wikidata-book-harvest`).

An admin update that keeps the same link queues a copy only when no copy of it is stored. A copy queued by a save is attempted even after an earlier failure of that link. Queuing MUST NOT delay or fail the save: if it fails, the save still succeeds and the periodic sweep makes the copy later.

#### Scenario: File import
- **WHEN** an admin imports a CSV file whose three created rows have cover links
- **THEN** a copy is queued for each of the three books after its row is saved

#### Scenario: Queue unavailable
- **WHEN** an admin saves a book with a cover link while the task queue is down
- **THEN** the save succeeds, the book shows the source link, and a later sweep stores the copy

#### Scenario: Re-saving a failed cover
- **WHEN** an admin saves a book again, unchanged, after its cover copy failed
- **THEN** the copy is attempted again

#### Scenario: Backfilled cover
- **WHEN** the cover backfill sets the Open Library cover link of a book that had none
- **THEN** a copy is queued for that book, the book shows the Open Library link until the copy is stored, and a `/covers/…` path afterwards
