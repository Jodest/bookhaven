## MODIFIED Requirements

### Requirement: Job list and job details
The system SHALL expose `GET /api/v1/admin/harvest/jobs` with recent jobs grouped by state (active, retry, pending, completed, archived), at most 50 per state, and `GET /api/v1/admin/harvest/jobs/:id`, which returns 404 for a job that is unknown or no longer retained. Both MUST require the `admin` role. Every job MUST carry `trigger` (`manual` or `scheduled`) and `author_qids` (the harvested author QIDs, possibly empty). A job queued before this field existed, or queued from the command line, reports `manual` with an empty list. Every job MUST also carry `kind`: `harvest` for a harvest job and `adaptation_sync` for a catalog adaptation sync job (see `catalog-import/wikidata-adaptation-sync`). A harvest job carries its report in `result` and `adaptation_sync: null`. A catalog adaptation sync job carries `author_qids: []`, `result: null` and its report in `adaptation_sync`. Job history is limited to what the queue still retains; there is no separate history table. Cancelling or retrying a job from the admin is not supported.

#### Scenario: Job aged out
- **WHEN** an admin requests a job that completed more than 24 hours ago
- **THEN** the response is 404

#### Scenario: Scheduled job in the list
- **WHEN** an admin lists jobs after a scheduled run queued a job for `Q7200` and `Q1183296`
- **THEN** that job has `trigger: "scheduled"` and `author_qids: ["Q7200", "Q1183296"]`

#### Scenario: Manual job in the list
- **WHEN** an admin lists jobs after triggering a harvest for `Q7200` from the admin
- **THEN** that job has `trigger: "manual"` and `author_qids: ["Q7200"]`

#### Scenario: Catalog sync job in the list
- **WHEN** an admin lists jobs after starting a catalog adaptation sync from the admin
- **THEN** that job has `kind: "adaptation_sync"`, `trigger: "manual"`, `author_qids: []`, `result: null` and an `adaptation_sync` report, and every harvest job has `kind: "harvest"`

### Requirement: Jobs page
The admin SHALL show the jobs at `/import/jobs`, refreshed every 5 seconds: a state badge per job, a trigger badge per job («Авто» for scheduled jobs, «Вручную» otherwise) with the number of harvested authors when known, a progress bar for active jobs, and for finished jobs a summary that expands into the list of errors with reasons and the list of skipped titles. A catalog adaptation sync job shows «Поиск экранизаций» under its trigger badge instead of the number of authors, a progress bar of processed books out of `total` while active, and when completed the summary «Книг: <processed> из <total>, обновлено: <updated>, добавлено экранизаций: <added>, ошибок: <failed>». When it has errors, its summary expands into the listed errors with the book title and the reason «Wikidata недоступна» (`wikidata_unavailable`) or «Не удалось сохранить экранизации» (`save_failed`), followed by «Ещё ошибок: <n>» when `failed` exceeds the listed errors.

#### Scenario: Expanding a report
- **WHEN** an admin expands a completed job with 2 errors and 3 skips
- **THEN** the 2 errors with messages and the 3 skipped titles are listed

#### Scenario: Scheduled job badge
- **WHEN** the list contains a scheduled job for 10 authors
- **THEN** its row shows the badge «Авто» and «Авторов: 10»

#### Scenario: Completed catalog sync row
- **WHEN** the list contains a completed manual catalog sync job with `total: 683`, `processed: 683`, `updated: 40`, `added: 75` and `failed: 0`
- **THEN** its row shows «Вручную», «Поиск экранизаций» and «Книг: 683 из 683, обновлено: 40, добавлено экранизаций: 75, ошибок: 0»

#### Scenario: Catalog sync errors beyond the list
- **WHEN** an admin expands a completed catalog sync job with `failed: 120` and 50 listed errors
- **THEN** the 50 errors are shown with their reasons, followed by «Ещё ошибок: 70»
