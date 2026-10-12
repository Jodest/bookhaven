# catalog-moderation/genre-merge Specification

## Purpose

Lets an admin merge a duplicate genre into a target genre. Unlike an author merge, the merged genre is kept as an alias, so imports that name it or carry its Wikidata QID later resolve to the target.

## Requirements

### Requirement: Merge endpoint
The system SHALL accept `POST /api/v1/admin/genres/{id}/merge` with `{"canonical_id"}`, where `{id}` is the source genre and `canonical_id` the target. On success it MUST return 200 with `{genre, books_reassigned}`: the target genre and the number of books moved to it. A non-UUID id or `canonical_id` MUST return 400, merging a genre into itself MUST return 400, and a missing source or target, or one that is already merged (an alias), MUST return 404; in every error case nothing changes. The endpoint MUST require the `admin` role and a bearer token.

#### Scenario: Source already merged
- **WHEN** an admin merges a genre that was already merged into another genre
- **THEN** the response is 404 and nothing changes

### Requirement: Books move and the source becomes an alias
In one transaction, every book linked to the source SHALL be linked to the target instead; a book already linked to both keeps a single link to the target and is not counted in `books_reassigned`. The source MUST become an alias of the target: it keeps its name and slug, its status becomes `rejected`, and it no longer appears in admin lists, queues, hints or the public site. Aliases that pointed at the source MUST be re-pointed at the target, so an alias always leads directly to a non-alias genre. The merge is irreversible.

#### Scenario: Merging a genre that already has an alias
- **WHEN** «романы» was merged into «роман», and then «роман» is merged into «проза»
- **THEN** both «романы» and «роман» are aliases of «проза»

#### Scenario: Imports follow the alias
- **WHEN** after merging «романы» into «роман» a file import row names the genre «романы»
- **THEN** the book is linked to «роман» and no genre is created

### Requirement: Target genre is not edited
The merge SHALL NOT change the target's name, slug or status. A merge into a rejected target is allowed.

#### Scenario: Pending source into published target
- **WHEN** a pending genre is merged into a published genre
- **THEN** the target stays published with its own name

### Requirement: Wikidata QID carry-over
When the source has a Wikidata QID and the target has none, the QID SHALL move to the target. When both have one, the target keeps its own and the source alias keeps its QID, so that QID still resolves to the target.

#### Scenario: Both have QIDs
- **WHEN** a source with QID `Q1` is merged into a target with QID `Q2`
- **THEN** the target keeps `Q2`, and a later import of `Q1` resolves to the target through the alias

### Requirement: Merge dialog
The genre moderation queue SHALL offer «Слить» on every row and «Это он — слить» on each hint (which preselects that hint as the target). The dialog titled «Слить жанр «{source}» в «{target}»» lets the admin search for and change the target, explains that the books move to the target and that the source disappears from the lists while its name and QID keep pointing to the target in future imports, and says the action is irreversible. When the chosen target is rejected it MUST warn «Целевой жанр отклонён — книги окажутся в скрытом жанре.». A 400 MUST show «Нельзя слить жанр с самим собой.», a 404 MUST say that one of the genres was not found and may already be merged, and any other error shows «Не удалось выполнить слияние. Попробуйте позже.».

#### Scenario: Rejected target warning
- **WHEN** an admin picks a rejected genre as the target
- **THEN** the dialog warns that the books will end up in a hidden genre
