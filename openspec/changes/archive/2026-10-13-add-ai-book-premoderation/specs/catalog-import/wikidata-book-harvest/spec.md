## MODIFIED Requirements

### Requirement: Scheduled harvest of flagged authors
When scheduled harvesting is switched on, the worker SHALL periodically harvest, on a configurable schedule (daily by default), authors that are flagged for automatic harvesting, are `published` and have a Wikidata QID. It is switched off by default and is switched on only by deployment configuration. Each run MUST pick at most a configured batch of authors (10 by default, never more than 50). Authors never auto-harvested come first, then the least recently auto-harvested. An author auto-harvested less than 30 days ago MUST NOT be picked. The picked authors MUST be harvested as one ordinary harvest job with the default result limit. That job follows every rule of this capability: serialized processing, retries, create-only idempotency, `pending` books, matching and the report. A picked author whose stored QID does not match `^Q[0-9]+$` MUST be left out of the job and logged. Its last-auto-harvest time MUST still be recorded, so one bad QID never blocks the batch or keeps the run failing. When every picked author has a malformed QID, no job is queued, and those authors' last-auto-harvest times are still recorded. A picked author with a valid QID MUST have its last-auto-harvest time recorded only after its job was queued successfully. A run MUST be skipped without picking anyone while an earlier scheduled job is still pending, active or retrying. A run that finds no eligible author queues nothing. Scheduled harvesting MUST NOT publish anything and MUST NOT change existing books. Manual harvests do not change an author's last-auto-harvest time.

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

#### Scenario: Flagged author with a malformed QID
- **WHEN** a run picks three flagged authors and one of them has the stored QID `7200`
- **THEN** one harvest job for the other two authors is queued, the malformed QID is logged, and all three authors get their last-auto-harvest time set

#### Scenario: Only malformed QIDs
- **WHEN** every author a run picks has a malformed QID
- **THEN** no harvest job is queued, the run does not fail, and those authors get their last-auto-harvest time set
