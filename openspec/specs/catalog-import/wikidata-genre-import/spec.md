# catalog-import/wikidata-genre-import Specification

## Purpose

Lets an admin find literary genres in Wikidata and import one into the catalog as a pending genre, without duplicating genres (including merged ones) or reviving rejected ones.

## Requirements

### Requirement: Literary genre classification
A Wikidata item SHALL count as a literary genre when its class path reaches literary genre (`Q223393`) or literary form (`Q4263830`), or when it reaches fiction genre (`Q108465955`) without also reaching music genre (`Q188451`) or film genre (`Q201658`). Classification MUST walk upward from the given items only, and every classification call MUST be bounded by an 8-second deadline.

#### Scenario: Fiction genre
- **WHEN** «детектив» is typed only as a genre of fiction
- **THEN** it is classified as a literary genre

#### Scenario: Music genre
- **WHEN** «рок» reaches fiction genre and music genre
- **THEN** it is not classified as a literary genre

### Requirement: Genre search
The system SHALL expose `GET /api/v1/admin/genres/wikidata-search?q=`. An empty `q` MUST return 400. The system searches Wikidata for up to 20 items, classifies them in one batch, drops the items that are not literary genres, and returns the first 10 as `{qid, label, description, ru_label, literary_genre, existing}` with `class_check: "ok"`. When classification fails, the system MUST log a warning and return the unfiltered candidates with `literary_genre` and `ru_label` set to `null` and `class_check: "unavailable"`. Zero search results MUST return 200 with `items: []`. The endpoint MUST require the `admin` role.

#### Scenario: Classification unavailable
- **WHEN** the classification query times out
- **THEN** the search still returns 200 with unfiltered candidates and `class_check: "unavailable"`

### Requirement: Existing markers in search
Each candidate's `existing` SHALL be the catalog genre owning that QID, merged genres included: an alias MUST be shown as its canonical genre `{id, name, slug, status}` with `via_alias` set. A marker lookup failure MUST be logged and give `existing: null` without failing the search.

#### Scenario: QID owned by a merged genre
- **WHEN** a candidate's QID belongs to a genre merged into «фэнтези»
- **THEN** `existing` is «фэнтези» and `via_alias` names the merged genre

### Requirement: Genre import
The system SHALL accept `POST /api/v1/admin/genres/import-wikidata` with `{"qid"}` and answer 201 for `created` and 200 for `existing` or `rejected`, with `{outcome, match, qid_backfilled, qid_mismatch, via_alias, genre}`. The QID handling MUST follow the author import rules: normalization (400 if invalid), a catalog lookup by QID first with no Wikidata call (aliases followed), 404 for a missing entity, 502 for an unavailable Wikidata, canonical QID after a redirect, a Russian label required (422 `no_ru_label`), one retry on a concurrent duplicate and then 409, and 503 if not configured. The endpoint MUST require the `admin` role and a bearer token.

#### Scenario: Known QID
- **WHEN** an admin imports a QID that a published genre already has
- **THEN** the response is 200 `existing` with `match: "qid"` and Wikidata is not called

### Requirement: Authoritative class check on import
Import SHALL check that the item is a literary genre, first from the fetched claims and otherwise with the classification query. A non-literary item MUST return 422 with code `not_a_literary_genre`. A failed classification MUST return 502: import never skips the check.

#### Scenario: Film genre
- **WHEN** an admin imports a QID that is a film genre
- **THEN** the response is 422 `not_a_literary_genre` and no genre is created

### Requirement: Matching and creation
The import SHALL resolve `{Russian name, canonical QID}` with the genre rules of `catalog-import/wikidata-matching`. A slug match with a different QID MUST return `existing` with `qid_mismatch` set to the genre's QID and create no genre. A created genre MUST be `pending` with the QID and appear in the genre moderation queue.

#### Scenario: New literary genre
- **WHEN** an admin imports a literary genre unknown to the catalog
- **THEN** the response is 201 `created` with a pending genre carrying the QID

### Requirement: Genre import panel
The genre import SHALL live at `/import/genres/wikidata`. Candidates show the Russian name (or the label marked «нет русского названия»), a description, a Wikidata link and the existing marker. The action is «Импортировать» for a candidate not yet in the catalog; «Уже в каталоге», «Отклонён ранее» or «Нет русского названия» (no button) otherwise. `class_check: "unavailable"` MUST show a warning that the check will run at import time. Every outcome and error code MUST produce a readable Russian message with links to the genre or the moderation queue.

#### Scenario: Unavailable classification warning
- **WHEN** the search returns `class_check: "unavailable"`
- **THEN** a warning above the list says that the literary-genre check will be done during import
