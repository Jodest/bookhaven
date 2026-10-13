## ADDED Requirements

### Requirement: Scheduled harvest of flagged authors
When scheduled harvesting is switched on, the worker SHALL periodically harvest, on a configurable schedule (daily by default), authors that are flagged for automatic harvesting, are `published` and have a Wikidata QID. It is switched off by default and is switched on only by deployment configuration. Each run MUST pick at most a configured batch of authors (10 by default, never more than 50). Authors never auto-harvested come first, then the least recently auto-harvested. An author auto-harvested less than 30 days ago MUST NOT be picked. The picked authors MUST be harvested as one ordinary harvest job with the default result limit. That job follows every rule of this capability: serialized processing, retries, create-only idempotency, `pending` books, matching and the report. A picked author's last-auto-harvest time MUST be recorded only after its job was queued successfully. A run MUST be skipped without picking anyone while an earlier scheduled job is still pending, active or retrying. A run that finds no eligible author queues nothing. Scheduled harvesting MUST NOT publish anything and MUST NOT change existing books. Manual harvests do not change an author's last-auto-harvest time.

#### Scenario: Batch of flagged authors
- **WHEN** scheduled harvesting is on with a batch of 10 and 25 eligible flagged authors exist
- **THEN** one harvest job for the 10 authors that were never auto-harvested or were auto-harvested longest ago is queued, and their last-auto-harvest time is set

#### Scenario: Recently harvested author
- **WHEN** a flagged published author was auto-harvested 5 days ago
- **THEN** the next scheduled run does not pick that author

#### Scenario: Ineligible authors
- **WHEN** a flagged author is `pending`, or `rejected`, or has no Wikidata QID
- **THEN** no scheduled run picks that author

#### Scenario: Previous scheduled job still running
- **WHEN** a scheduled run starts while the previous scheduled job is still active
- **THEN** no new job is queued and no author's last-auto-harvest time changes

#### Scenario: New work of a flagged author
- **WHEN** a scheduled job harvests an author whose Wikidata bibliography gained a work since the last run
- **THEN** that work is created as a `pending` book in the moderation queue, and the author's books already in the catalog, including rejected ones, are reported as skipped and stay unchanged

#### Scenario: Switched off
- **WHEN** scheduled harvesting is not switched on in the configuration
- **THEN** no scheduled harvest job is ever queued

## MODIFIED Requirements

### Requirement: Asynchronous, serialized processing
Harvest jobs SHALL run in a background worker, one job at a time, on every host regardless of its CPU count. Manual and scheduled jobs share the same single slot. A job MUST be retried at most 3 times, MUST time out after 30 minutes, and its result MUST stay inspectable for 24 hours after it finishes.

#### Scenario: Two jobs queued
- **WHEN** an admin triggers two harvests in a row
- **THEN** the second job starts only after the first one finishes

#### Scenario: Multi-core host
- **WHEN** the worker runs on a host with 8 CPUs and three harvest jobs are queued
- **THEN** exactly one of them is active at any moment

#### Scenario: Manual job during a scheduled job
- **WHEN** an admin triggers a harvest while a scheduled harvest job is active
- **THEN** the manual job stays pending until the scheduled job finishes

### Requirement: Job list and job details
The system SHALL expose `GET /api/v1/admin/harvest/jobs` with recent jobs grouped by state (active, retry, pending, completed, archived), at most 50 per state, and `GET /api/v1/admin/harvest/jobs/:id`, which returns 404 for a job that is unknown or no longer retained. Both MUST require the `admin` role. Every job MUST carry `trigger` (`manual` or `scheduled`) and `author_qids` (the harvested author QIDs, possibly empty). A job queued before this field existed, or queued from the command line, reports `manual` with an empty list. Job history is limited to what the queue still retains; there is no separate history table. Cancelling or retrying a job from the admin is not supported.

#### Scenario: Job aged out
- **WHEN** an admin requests a job that completed more than 24 hours ago
- **THEN** the response is 404

#### Scenario: Scheduled job in the list
- **WHEN** an admin lists jobs after a scheduled run queued a job for `Q7200` and `Q1183296`
- **THEN** that job has `trigger: "scheduled"` and `author_qids: ["Q7200", "Q1183296"]`

#### Scenario: Manual job in the list
- **WHEN** an admin lists jobs after triggering a harvest for `Q7200` from the admin
- **THEN** that job has `trigger: "manual"` and `author_qids: ["Q7200"]`

### Requirement: Jobs page
The admin SHALL show the jobs at `/import/jobs`, refreshed every 5 seconds: a state badge per job, a trigger badge per job («Авто» for scheduled jobs, «Вручную» otherwise) with the number of harvested authors when known, a progress bar for active jobs, and for finished jobs a summary that expands into the list of errors with reasons and the list of skipped titles.

#### Scenario: Expanding a report
- **WHEN** an admin expands a completed job with 2 errors and 3 skips
- **THEN** the 2 errors with messages and the 3 skipped titles are listed

#### Scenario: Scheduled job badge
- **WHEN** the list contains a scheduled job for 10 authors
- **THEN** its row shows the badge «Авто» and «Авторов: 10»
