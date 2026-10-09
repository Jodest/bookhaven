# catalog-import/wikidata-book-harvest Specification

## Purpose

Lets an admin harvest the bibliography of chosen authors from Wikidata (enriched from Open Library) as an asynchronous job, with harvested books landing in moderation instead of going public.

## Requirements

### Requirement: Trigger a harvest by author QIDs
The system SHALL accept `POST /api/v1/admin/harvest` with `{"author_qids": string[], "limit"?: number}` and enqueue one harvest job, answering 201 `{task_id, queue, state: "pending"}`. An empty list or a QID not matching `^Q[0-9]+$` MUST return 400. A failure to enqueue MUST return 502. The endpoint MUST require the `admin` role and a bearer token. Raw SPARQL MUST NOT be accepted from the admin UI or API; arbitrary queries stay a command-line-only tool.

#### Scenario: Valid trigger
- **WHEN** an admin submits `{"author_qids": ["Q7200"]}`
- **THEN** the response is 201 with a task ID and the job is queued

#### Scenario: Malformed QID
- **WHEN** an admin submits `{"author_qids": ["7200"]}`
- **THEN** the response is 400 and nothing is queued

### Requirement: Result size limit
`limit` SHALL bound the number of works the job's query returns, not the number of authors. A missing or non-positive limit MUST default to 500, and a limit above 5000 MUST be clamped to 5000 rather than rejected.

#### Scenario: Oversized limit
- **WHEN** an admin submits `limit: 20000`
- **THEN** the job runs with a limit of 5000

### Requirement: What is harvested
A job SHALL select literary works authored by the given QIDs that have a Russian Wikidata label. The Russian label becomes the book title.

#### Scenario: Work without a Russian label
- **WHEN** an author's work has only an English label in Wikidata
- **THEN** that work is not part of the job's results

### Requirement: Author search to pick QIDs
The system SHALL expose `GET /api/v1/admin/harvest/authors/search?q=` returning Wikidata candidates `{qid, label, description, existing}`, where `existing` is the catalog author with that QID (`{id, name, slug, status}`) or `null`. An empty `q` MUST return 400, zero matches MUST return 200 with `items: []`, and Wikidata being unavailable MUST return 502. The system MUST NOT auto-pick a candidate: the admin always chooses explicitly.

#### Scenario: Ambiguous surname
- **WHEN** an admin searches «Толстой»
- **THEN** several candidates with descriptions are returned and none is selected automatically

#### Scenario: Candidate already in the catalog
- **WHEN** a candidate's QID belongs to a pending author
- **THEN** its `existing` field carries that author with status `pending`

### Requirement: Harvest form input modes
The harvest page (`/import/books/wikidata`) SHALL offer two mutually exclusive ways to build the QID list for one submission: picking candidates found by name search, or typing QIDs manually. One submission MUST send QIDs from exactly one mode, never a merge of both. The optional limit field is passed through.

#### Scenario: Manual QID mode
- **WHEN** an admin switches to manual mode and enters `Q7200, Q1183296`
- **THEN** only those two QIDs are submitted

### Requirement: Asynchronous, serialized processing
Harvest jobs SHALL run in a background worker, one job at a time. A job MUST be retried at most 3 times, MUST time out after 30 minutes, and its result MUST stay inspectable for 24 hours after it finishes.

#### Scenario: Two jobs queued
- **WHEN** an admin triggers two harvests in a row
- **THEN** the second job starts only after the first one finishes

### Requirement: Idempotent, create-only harvesting
A work whose Wikidata QID is already stored on a book SHALL be skipped and MUST NOT update that book. Harvesting never edits existing books.

#### Scenario: Re-harvesting the same author
- **WHEN** an author is harvested a second time
- **THEN** books from the first run are reported as skipped and remain unchanged

### Requirement: Harvested books go to moderation
Every book created by a harvest SHALL be `pending` and record Wikidata as its source together with the work's QID. The Russian-title check is not applied to harvested titles, because the title is the Russian label by definition, even when written in Latin script. The description MUST still be Russian. A missing language MUST default to `ru`.

#### Scenario: Latin-script Russian title
- **WHEN** a work's Russian label is «S.N.U.F.F.»
- **THEN** the book is created pending with that title

### Requirement: Open Library enrichment fills gaps only
When a work lacks a cover, description, page count or ISBN, the job SHALL try Open Library (by ISBN, then by title and first author) and fill only the fields Wikidata left empty. Wikidata values MUST NOT be overwritten. A non-Russian Open Library description MUST be dropped silently instead of failing the book.

#### Scenario: English excerpt
- **WHEN** Open Library returns an English description for a work
- **THEN** the book is created without a description

### Requirement: Author and genre matching
Each work's authors and genres SHALL be resolved with the rules of `catalog-import/wikidata-matching`. A reference that resolves to a rejected tombstone MUST be skipped, and the book is still created without it.

#### Scenario: Rejected author
- **WHEN** a work's only author is a rejected tombstone
- **THEN** the book is created pending with no author linked

### Requirement: New authors are enriched after the job
After the per-work loop, the job SHALL enrich the authors it created with a QID (dates and photo from Wikidata) on a best-effort basis. Enrichment failures MUST only be counted (`authors_enriched`, `author_enrichment_errors`) and MUST NOT fail the job.

#### Scenario: Enrichment failure
- **WHEN** enriching one created author fails
- **THEN** the job still completes and reports one enrichment error

### Requirement: Per-record report and progress
A failure on one work SHALL NOT abort the job. The job result MUST contain `total`, `processed`, `created`, `skipped_existing`, `errors: [{index, title, message}]` and `skipped_records: [{index, title}]`, with exactly one skipped record per skip. While the job is active, the result MUST be updated after every work so the progress can be shown. A progress-write failure MUST NOT fail the job.

#### Scenario: Progress of an active job
- **WHEN** an admin views a job that has processed 37 of 200 works
- **THEN** the job shows `processed: 37` and `total: 200`

### Requirement: Job list and job details
The system SHALL expose `GET /api/v1/admin/harvest/jobs` with recent jobs grouped by state (active, retry, pending, completed, archived), at most 50 per state, and `GET /api/v1/admin/harvest/jobs/:id`, which returns 404 for a job that is unknown or no longer retained. Both MUST require the `admin` role. Job history is limited to what the queue still retains; there is no separate history table. Cancelling or retrying a job from the admin is not supported.

#### Scenario: Job aged out
- **WHEN** an admin requests a job that completed more than 24 hours ago
- **THEN** the response is 404

### Requirement: Jobs page
The admin SHALL show the jobs at `/import/jobs`, refreshed every 5 seconds: a state badge per job, a progress bar for active jobs, and for finished jobs a summary that expands into the list of errors with reasons and the list of skipped titles.

#### Scenario: Expanding a report
- **WHEN** an admin expands a completed job with 2 errors and 3 skips
- **THEN** the 2 errors with messages and the 3 skipped titles are listed
