## MODIFIED Requirements

### Requirement: Only the status changes
A bulk change SHALL write only the status (and the update time where the entity has one), plus, for a book published for the first time, its first-publication time (see `catalog-moderation/entity-status`). A book that already has a first-publication time MUST keep it, whatever the new status. Names, slugs, Wikidata QIDs, other fields and the links between books, authors and genres MUST stay unchanged, and no content re-validation runs.

#### Scenario: Rejecting a genre keeps its links
- **WHEN** an admin rejects a genre linked to five books
- **THEN** the five links remain and the genre is only hidden from public book pages

#### Scenario: Bulk publish sets the first-publication time
- **WHEN** an admin bulk-publishes two pending books, one never published and one published before and later unpublished
- **THEN** the first book gets the current time as its first-publication time and the second keeps its earlier time
