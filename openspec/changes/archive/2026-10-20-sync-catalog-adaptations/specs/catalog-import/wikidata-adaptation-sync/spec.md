## ADDED Requirements

### Requirement: Catalog-wide sync trigger
`POST /api/v1/admin/books/sync-adaptations/catalog` SHALL require the `admin` role and a bearer header. It takes no body and queues one catalog adaptation sync job, answering 201 `{task_id, queue, state: "pending"}`. While a catalog adaptation sync job, manual or scheduled, is pending, active or retrying, the endpoint MUST return 409 «adaptation sync already running» and queue nothing. A failure to inspect or to enqueue MUST return 502 and queue nothing. A moderator gets 403.

#### Scenario: Start a catalog sync
- **WHEN** an admin calls the endpoint and no catalog sync job is queued or running
- **THEN** the response is 201 with a task id and one catalog sync job is queued

#### Scenario: Already running
- **WHEN** an admin calls the endpoint while a catalog sync job is active
- **THEN** the response is 409 and no second job is queued

#### Scenario: Moderator starts a catalog sync
- **WHEN** a moderator calls the catalog sync endpoint
- **THEN** the response is 403 and nothing is queued

### Requirement: Catalog sync job
A catalog sync job SHALL sync every book that is `published` and has a Wikidata QID when the job starts, in chunks of at most 50 books, oldest books first. Every book is synced with the rules of this capability: an existing work is never duplicated, links are only added, a hidden link is never shown, created again or counted, and a work edited by hand is never changed. Books that are pending or rejected, and books without a Wikidata QID, MUST NOT be part of the job. A book whose stored QID is malformed is counted as skipped. A failure on one book or one chunk MUST NOT abort the other books. When the Wikidata request of three chunks in a row fails, the attempt MUST stop and fail, and it is retried later from the start; links created before the failure stay. A job is retried at most 3 times, times out after 30 minutes, and its result stays inspectable for 24 hours after it finishes. The job runs in the single worker slot it shares with harvest jobs: a harvest waits while a catalog sync runs, and the reverse. A catalog sync job MUST NOT count as a scheduled harvest job for the scheduled harvest's skip rule.

#### Scenario: Only published books with a QID
- **WHEN** the catalog has three published books with a QID, one published book without a QID and one pending book with a QID
- **THEN** the job's `total` is 3 and Wikidata is not asked about the other two books

#### Scenario: Hidden link during a catalog sync
- **WHEN** an admin hid a wrong adaptation of a published book and a catalog sync then runs
- **THEN** that link is still hidden and is not counted in `added`

#### Scenario: Wikidata down for the whole run
- **WHEN** the Wikidata request of three chunks in a row fails
- **THEN** the attempt fails, the job goes to retry, and the links created by earlier chunks are kept

#### Scenario: One chunk fails
- **WHEN** the Wikidata request of one chunk of 50 books fails and the next chunks succeed
- **THEN** the job completes, those 50 books are counted in `failed`, and the other books are synced

#### Scenario: Harvest queued during a catalog sync
- **WHEN** an admin triggers a harvest while a catalog sync job is active
- **THEN** the harvest job stays pending until the catalog sync job finishes

### Requirement: Catalog sync report and progress
The job result SHALL be `{total, processed, updated, added, unchanged, skipped, failed, errors}`:
- `total` is the number of books the job selected and `processed` the number already handled;
- `updated`, `unchanged`, `skipped` and `failed` count the books with the outcome `added`, `unchanged`, `skipped` and `error`;
- `added` is the number of links created;
- `errors` lists `{book_id, title, reason}` for failed books, where `reason` is `wikidata_unavailable` or `save_failed`, at most the first 50 of them; `failed` still counts all.

While the job is active, the result MUST be updated after every chunk so the progress can be shown. A progress-write failure MUST NOT fail the job.

#### Scenario: Progress of a catalog sync
- **WHEN** an admin views a catalog sync job that has handled 2 of 14 chunks of a 683-book catalog
- **THEN** the job shows `processed: 100` and `total: 683`

#### Scenario: Many failures
- **WHEN** 120 books of a completed catalog sync failed
- **THEN** the result has `failed: 120` and 50 entries in `errors`

### Requirement: Scheduled catalog sync
When the scheduled catalog sync is switched on, the worker SHALL queue one catalog sync job on a configurable schedule, weekly by default. It is switched off by default and is switched on only by deployment configuration. A scheduled job follows every rule of the catalog sync job and is reported with the trigger `scheduled`. A run MUST queue nothing while a catalog sync job, manual or scheduled, is pending, active or retrying. A failed run is not retried; the next scheduled run tries again. The scheduled catalog sync MUST NOT publish anything and MUST NOT change existing books.

#### Scenario: Scheduled sync switched off
- **WHEN** the scheduled catalog sync is not switched on in the configuration
- **THEN** no catalog sync job is ever queued by the worker

#### Scenario: Scheduled run
- **WHEN** the scheduled catalog sync is on and its schedule fires with no catalog sync job queued or running
- **THEN** one catalog sync job with the trigger `scheduled` is queued

#### Scenario: Manual sync still running
- **WHEN** the schedule fires while a manual catalog sync job is active
- **THEN** no job is queued

### Requirement: Catalog sync action in the admin
The page «Импорт» → «Задачи» (`/import/jobs`) SHALL offer the button «Найти экранизации для всего каталога». It first asks «Найти экранизации в Wikidata для всех опубликованных книг с QID? Задача выполнится в фоне.» with «Запустить» and «Отмена». After «Запустить» it shows:
- «Поиск экранизаций по каталогу поставлен в очередь.» on success, and the job appears in the list;
- «Поиск экранизаций по каталогу уже выполняется.» on 409;
- «Не удалось запустить поиск экранизаций.» on any other failure.

The button is disabled while the request is running and while the job list contains a catalog sync job that is pending, active or retrying. «Отмена» queues nothing.

#### Scenario: Admin starts the catalog sync
- **WHEN** an admin clicks «Найти экранизации для всего каталога» and then «Запустить»
- **THEN** the banner reads «Поиск экранизаций по каталогу поставлен в очередь.» and the new job is listed

#### Scenario: Sync already in the list
- **WHEN** the job list shows an active catalog sync job
- **THEN** the button «Найти экранизации для всего каталога» is disabled

#### Scenario: Cancel the confirmation
- **WHEN** an admin clicks the button and then «Отмена»
- **THEN** no request is sent
