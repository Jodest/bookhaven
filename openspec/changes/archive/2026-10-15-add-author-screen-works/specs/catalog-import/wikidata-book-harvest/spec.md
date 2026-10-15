## ADDED Requirements

### Requirement: Films about created authors are synced after the job
After the adaptation sync step, the job SHALL sync the films about the authors it created with the rules of `catalog-import/wikidata-author-film-sync`, on a best-effort basis, when a Wikidata source is configured. These are the same authors the job enriches. The result MUST report:
- `author_films_added`, the number of film links created;
- `author_film_sync_errors`, the number of created authors whose sync ended in an error.

A sync failure MUST only be counted and MUST NOT fail the job. Authors the job only matched to existing catalog authors are not synced. This applies to manual, scheduled and import-triggered harvests alike.

#### Scenario: New author with a documentary
- **WHEN** a harvest creates a pending author that Wikidata lists as the main subject of one documentary
- **THEN** the job result has `author_films_added: 1` and the documentary appears on the author page once the author is published

#### Scenario: Author film sync failure
- **WHEN** Wikidata fails while the job syncs the films of two created authors
- **THEN** the job still completes with its books and authors created and reports `author_film_sync_errors: 2`

#### Scenario: Only existing authors
- **WHEN** a harvest creates books whose authors all already exist in the catalog
- **THEN** no author film sync request is made and the job reports `author_films_added: 0`
