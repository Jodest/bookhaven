# catalog-moderation/author-merge Specification

## Purpose

Lets an admin consolidate a duplicate author into the author that stays, moving all of the duplicate's books and deleting the duplicate, without losing its Wikidata identity.

## Requirements

### Requirement: Merge endpoint
The system SHALL accept `POST /api/v1/admin/authors/{id}/merge` with `{"canonical_id"}`, where `{id}` is the duplicate. On success it MUST return 200 with `{author, books_reassigned}`: the canonical author and the number of books moved to it. A non-UUID id or `canonical_id` MUST return 400, merging an author into itself MUST return 400, and a missing duplicate or canonical author MUST return 404; in every error case nothing changes. The endpoint MUST require the `admin` role and a bearer token.

#### Scenario: Merge into itself
- **WHEN** an admin sends the duplicate's own id as `canonical_id`
- **THEN** the response is 400 and nothing changes

### Requirement: Books move and the duplicate is deleted
In one transaction, every book linked to the duplicate SHALL be linked to the canonical author instead; a book already linked to both keeps a single link to the canonical author and is not counted in `books_reassigned`. The duplicate author MUST then be deleted. The merge is irreversible.

#### Scenario: Book linked to both
- **WHEN** the duplicate has books A and B and the canonical author already has book B
- **THEN** both books are linked to the canonical author once, `books_reassigned` is 1 and the duplicate no longer exists

### Requirement: Canonical author is not edited
The merge SHALL NOT change the canonical author's name, slug, biography, photo, dates or status, whatever the statuses of the two authors.

#### Scenario: Pending duplicate into published author
- **WHEN** a pending duplicate is merged into a published author
- **THEN** the canonical author stays published with its own name and biography

### Requirement: Wikidata identity carry-over
When the canonical author has no Wikidata QID and the duplicate has one, the QID SHALL move to the canonical author in the same transaction, so later imports match it by QID instead of recreating the author. When both have a QID, the canonical author MUST keep its own and the duplicate's is dropped.

#### Scenario: QID moves to the canonical author
- **WHEN** a duplicate with QID `Q7200` is merged into an author without a QID
- **THEN** the canonical author now has QID `Q7200`, and a later harvest of `Q7200` links to it

### Requirement: Merge dialog
The author moderation queue SHALL offer «Слить» on every non-rejected row and «Это он — слить» on each hint (which preselects that hint as the canonical author). The dialog shows the duplicate («будет удалён»), lets the admin search for and change the canonical author, and warns that the action is irreversible: all the duplicate's books move to the canonical author and the duplicate is deleted. The confirm button reads «Слить и удалить «{name}»». A 400 MUST show «Нельзя слить автора с самим собой.», a 404 MUST say that one of the authors was not found and ask to refresh the list, and any other error shows «Не удалось выполнить слияние. Попробуйте позже.».

#### Scenario: Merge from a hint
- **WHEN** an admin clicks «Это он — слить» on a hint
- **THEN** the dialog opens with the queue row as the duplicate and the hint as the canonical author
