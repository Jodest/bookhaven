## ADDED Requirements

### Requirement: Films of created authors are synced
After the enrichment of a `created` author, the import SHALL sync the films about that author with the rules of `catalog-import/wikidata-author-film-sync`, on a best-effort basis, when a Wikidata source is configured. A sync failure MUST be logged only. It MUST NOT fail the import or change its response. An `existing` or `rejected` outcome does not sync films. This applies to every flow that uses the direct import, including the AI author curation page.

#### Scenario: Created author with a biopic
- **WHEN** an admin imports a new author whose Wikidata entity is the main subject of a biographical film
- **THEN** the response is 201 `created` and the film is linked to the pending author

#### Scenario: Film sync fails
- **WHEN** an admin imports a new author and Wikidata fails during the film sync
- **THEN** the response is still 201 `created` with the author and its enrichment

#### Scenario: Existing author
- **WHEN** an admin imports the QID of an author already in the catalog
- **THEN** no film sync runs for that author
