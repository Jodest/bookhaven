## ADDED Requirements

### Requirement: Adaptations are synced after the job
After the author enrichment step, the job SHALL sync screen adaptations of the books it created with the rules of `catalog-import/wikidata-adaptation-sync`, on a best-effort basis, when a Wikidata source is configured. The result MUST report `adaptations_added`, the number of links created, and `adaptation_sync_errors`, the number of created books whose sync ended in an error. A sync failure MUST only be counted and MUST NOT fail the job. Books the job skipped as already existing are not synced. This applies to manual, scheduled and import-triggered harvests alike.

#### Scenario: New book with adaptations
- **WHEN** a harvest creates a pending book whose work has two film adaptations in Wikidata
- **THEN** the job result has `adaptations_added: 2` and both adaptations appear on the book page once the book is published

#### Scenario: Sync failure
- **WHEN** Wikidata fails while the job syncs adaptations of three created books
- **THEN** the job still completes with its books created and reports `adaptation_sync_errors: 3`
