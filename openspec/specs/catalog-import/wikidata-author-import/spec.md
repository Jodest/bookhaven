# catalog-import/wikidata-author-import Specification

## Purpose

Lets an admin bring one specific author into the catalog from Wikidata without running a harvest first, optionally starting a harvest of that author's books, without duplicating authors or reviving rejected ones.

## Requirements

### Requirement: Import endpoint
The system SHALL accept `POST /api/v1/admin/authors/import-wikidata` with `{"qid": "Q7200"}`. It MUST answer 201 for `created` and 200 for `existing` or `rejected`, with `{outcome, match: "qid"|"slug"|null, qid_backfilled, namesake_of, author, enrichment}`. The endpoint MUST require the `admin` role and a bearer token.

#### Scenario: New author
- **WHEN** an admin imports a QID that matches nothing in the catalog
- **THEN** the response is 201 with `outcome: "created"`, `match: null` and a pending author

### Requirement: QID validation and database-first lookup
The QID SHALL be trimmed and uppercased and MUST match `^Q[0-9]+$`, otherwise the response is 400. The system MUST check the catalog by QID before calling Wikidata: a match returns `existing` (or `rejected` for a rejected author) with `match: "qid"` and makes no Wikidata call, so this works even while Wikidata is down.

#### Scenario: Known QID while Wikidata is down
- **WHEN** an admin imports the QID of an existing published author and Wikidata is unavailable
- **THEN** the response is 200 `existing` with `match: "qid"`

### Requirement: Wikidata entity checks
For a QID not found in the catalog, the system SHALL fetch the entity from Wikidata. A missing entity MUST return 404, and a rate-limited or unavailable Wikidata MUST return 502. If Wikidata redirects to another QID, the canonical QID MUST be used and checked against the catalog again. The author name MUST be the entity's Russian label; a missing or non-Russian label MUST return 422 with code `no_ru_label` (no fallback to the search label). The entity MUST be a human, pseudonym or collective pseudonym (instance of `Q5`, `Q61002` or `Q16017119`), otherwise 422 with code `not_a_person`. If import is not configured on the server, the response is 503.

#### Scenario: No Russian label
- **WHEN** the Wikidata entity has only an English label
- **THEN** the response is 422 with code `no_ru_label` and no author is created

#### Scenario: Collective pseudonym
- **WHEN** an admin imports «Козьма Прутков»
- **THEN** the person check passes

### Requirement: Matching and creation
After the checks, the system SHALL resolve `{Russian name, canonical QID}` with the rules of `catalog-import/wikidata-matching`, reporting `match: "slug"`, `qid_backfilled` and `namesake_of` when they apply. A newly created author MUST be `pending` and carry the QID, so it appears in the author moderation queue. If creation hits a concurrent duplicate, the system MUST retry the resolution once and then answer 409.

#### Scenario: Name match without QID
- **WHEN** the catalog has «Лев Толстой» without a QID and an admin imports `Q7200`
- **THEN** the response is 200 `existing` with `match: "slug"` and `qid_backfilled: true`

#### Scenario: Previously rejected author
- **WHEN** the imported author matches a rejected author
- **THEN** the response is 200 `rejected` and no author is created

### Requirement: Enrichment of created authors
A `created` author SHALL be enriched on a best-effort basis from the same Wikidata fetch (dates and a Commons photo with attribution), and the response `enrichment` reports what was applied. An enrichment failure MUST NOT fail the import.

#### Scenario: Photo available
- **WHEN** the imported entity has a Commons image
- **THEN** the created author gets the photo and `enrichment` reports it

### Requirement: Import with books
The import panel SHALL offer, for a candidate not yet in the catalog, «Импортировать автора» and «Импортировать с книгами»; for an existing pending or published author, «Импортировать книги»; for a rejected author, no import action («Отклонён ранее»). The book actions MUST ask for confirmation first. They import the author, then trigger a harvest for the stored QID only if the outcome is `created` or `existing`. A rejected outcome or an import error MUST NOT start a harvest. If the harvest call fails after a successful import, the admin is told to press «Импортировать книги» again.

#### Scenario: Import with books
- **WHEN** an admin confirms «Импортировать с книгами» for a new author
- **THEN** a pending author is created and a harvest job for that author's QID is queued

#### Scenario: Rejected author with books
- **WHEN** the import outcome is `rejected`
- **THEN** no harvest is started and the message says the books were not imported

### Requirement: Import panel
The author import SHALL live at `/import/authors/wikidata` and reuse the harvest author search. Each candidate shows its label, its description (or «без описания»), a Wikidata link opening in a new tab and, when it is already in the catalog, the author's status with a link to open it. Every outcome and error code (`no_ru_label`, `not_a_person`, 502) MUST produce a readable Russian message. After an import, author and dashboard data and the current search results MUST refresh.

#### Scenario: Empty search
- **WHEN** a search returns no candidates
- **THEN** the hint suggests creating the author manually and links to `/import/authors/manual`
