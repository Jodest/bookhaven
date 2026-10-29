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
When a work lacks a cover, description, page count or ISBN, the job SHALL try Open Library and fill only the fields Wikidata left empty. It tries, in this order: by ISBN, then by the work's Open Library IDs (cover only), then by the work's FantLab work IDs (cover only, see "Cover from a FantLab edition"), then by title and first author. Wikidata values MUST NOT be overwritten. A non-Russian Open Library description MUST be dropped silently instead of failing the book.

**Cover by Open Library ID.** When the work still has no cover after the ISBN step, the job SHALL use the Open Library IDs that Wikidata lists for the work (property P648):
- Only work IDs (`OL<digits>W`) and edition IDs (`OL<digits>M`) are used. Other values are ignored.
- Duplicates are removed. Work IDs are tried before edition IDs, each kind in ascending numeric order. At most 3 IDs are tried per work.
- For each ID the job reads the Open Library work or edition record and takes the first cover ID in its cover list that is greater than zero. Zero and negative cover IDs are skipped.
- The first ID that gives a cover ID wins, and no further IDs are requested. The cover link is `https://covers.openlibrary.org/b/id/<cover ID>-L.jpg`.
- An ID whose record is missing, has no usable cover ID, or whose request fails moves on to the next ID. Such failures MUST NOT fail the book.
- This step MUST fill only the cover. Description, ISBN, page count and title are never taken from these records.
- A cover found this way is used even when it belongs to an edition in another language. The book shows no marker for it.

The title search still runs as before when the ISBN step found nothing. A cover from the title search is used only when the work still has no cover after the Open Library ID and FantLab steps.

A result of the title-and-author search MUST be used only when it really is the same work. The job checks the first 5 results in Open Library's order and uses the first one that passes both rules below. Titles and names are compared after normalization: lower case, «ё» read as «е», every character that is not a letter, digit or space removed, runs of spaces collapsed.
- **Title.** The result's title equals the work's Russian label or its English Wikidata label. A result title followed by a subtitle also matches: the part before the first «:», «(», « — » or « - » is compared instead. An empty normalized title never matches.
- **Author.** The last word of one of the result's author names equals the last word of the Russian or English Wikidata label of one of the work's authors. A result without author names never matches.

When no result passes, or when the search answer reports more than 10 000 matching books (Open Library ignored the query), the job MUST take nothing from the search: no cover, description, ISBN or page count. The work is still created from its Wikidata data. The English labels, the Open Library IDs and the FantLab work IDs are used only for these lookups and are not stored.

#### Scenario: English excerpt
- **WHEN** Open Library returns an English description for a work
- **THEN** the book is created without a description

#### Scenario: Unrelated search result
- **WHEN** the title search for «История Петра I» by «Александр Сергеевич Пушкин» returns "I, Robot" by Isaac Asimov first, and the work has neither an Open Library ID nor a FantLab work ID in Wikidata
- **THEN** the book is created without a cover, description or ISBN from Open Library

#### Scenario: Same author, other work
- **WHEN** the title search for «Капитанская дочка» returns only a different Pushkin work, and the work has neither an Open Library ID nor a FantLab work ID in Wikidata
- **THEN** that result is not used and the book gets no cover from Open Library

#### Scenario: Match by English label
- **WHEN** a work labelled «Белый Клык» in Russian and "White Fang" in English, by «Джек Лондон» / "Jack London", has neither an Open Library ID nor a FantLab work ID in Wikidata, and its second search result is titled "White Fang" by "Jack London" while the first result is unrelated
- **THEN** the cover of the second result is used

#### Scenario: Title with a subtitle
- **WHEN** the search result is titled «Зов предков: повесть» by «Джек Лондон» for the work «Зов предков»
- **THEN** the result is used

#### Scenario: Query ignored by Open Library
- **WHEN** the search answer reports 907 849 matching books
- **THEN** no result of that answer is used, even one whose title and author match

#### Scenario: Cover from the Open Library work ID
- **WHEN** a work without an ISBN has the Open Library ID `OL27479W` in Wikidata, and that work's cover list is `[-1, 8406786]`
- **THEN** the book is created with the cover link `https://covers.openlibrary.org/b/id/8406786-L.jpg`, and neither a FantLab cover nor a cover from the title search is used

#### Scenario: Several Open Library IDs
- **WHEN** a work has the Open Library IDs `OL27516W`, `OL27479W` and `OL27455W`, the work `OL27455W` has no covers, and `OL27479W` has covers
- **THEN** `OL27455W` is read first, then `OL27479W`, its first positive cover ID is used, and `OL27516W` is not requested

#### Scenario: Edition ID only
- **WHEN** a work's only Open Library ID is the edition ID `OL7353617M` and that edition has a cover
- **THEN** the book gets the cover of that edition

#### Scenario: English edition cover
- **WHEN** the only cover found by a work's Open Library ID is from an English edition
- **THEN** the book is created with that cover and no marker

#### Scenario: Open Library ID without a cover
- **WHEN** a work's only Open Library ID has an empty cover list, the work has no FantLab work ID, and the title search finds a matching result with a cover
- **THEN** the cover of the matching title-search result is used

#### Scenario: Description not taken from the ID record
- **WHEN** a work gets its cover through its Open Library work ID and that work record has a Russian description
- **THEN** the description is not taken from that record, and the title search may still fill it

#### Scenario: Open Library down for the ID lookup
- **WHEN** reading the record behind a work's Open Library ID times out, and the work has no FantLab work ID
- **THEN** the book is still created, and the title search is still tried for its cover

#### Scenario: FantLab cover before the title search
- **WHEN** a work has no Open Library cover by ID, gets a FantLab cover, and the title search also finds a matching result with a cover and a Russian description
- **THEN** the book gets the FantLab cover link and the description from the title search, and the title-search cover is not used

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

### Requirement: Adaptations are synced after the job
After the author enrichment step, the job SHALL sync screen adaptations of the books it created with the rules of `catalog-import/wikidata-adaptation-sync`, on a best-effort basis, when a Wikidata source is configured. The result MUST report `adaptations_added`, the number of links created, and `adaptation_sync_errors`, the number of created books whose sync ended in an error. A sync failure MUST only be counted and MUST NOT fail the job. Books the job skipped as already existing are not synced. This applies to manual, scheduled and import-triggered harvests alike.

#### Scenario: New book with adaptations
- **WHEN** a harvest creates a pending book whose work has two film adaptations in Wikidata
- **THEN** the job result has `adaptations_added: 2` and both adaptations appear on the book page once the book is published

#### Scenario: Sync failure
- **WHEN** Wikidata fails while the job syncs adaptations of three created books
- **THEN** the job still completes with its books created and reports `adaptation_sync_errors: 3`

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

### Requirement: Audit of covers from the title search
The system SHALL provide a maintenance command, run from the command line by an operator with database access, that finds harvested books whose cover link may have come from a wrong Open Library title search result. It has no HTTP endpoint and no admin UI. The candidates are books with Wikidata as their source whose cover link is an Open Library cover-ID link. For each candidate the command checks:
- `shared_cover`: another book has a link to the same Open Library cover ID;
- `no_match`: the title search for the book's title and author names, checked with the rules of the Open Library enrichment requirement (Russian title and names only), finds no matching result;
- `other_match`: that check finds a matching result with a different cover.

By default the command MUST change nothing. It prints every candidate with at least one reason: book ID, status, title, authors, cover link, ISBN, whether it has a description, and its reasons. It also prints the number of candidates that passed every check. Only when run with an explicit apply option and an explicit list of book IDs does it clear the cover link and the ISBN of exactly those books. It MUST NOT change any book outside that list and MUST NOT change descriptions or any other field. Clearing MUST go through the normal book save, so the stored cover copy follows `catalog/book-covers`. An ID that is unknown, or is not a candidate, MUST be reported and skipped. An Open Library failure for one book MUST be reported for that book and MUST NOT stop the run.

#### Scenario: Report only
- **WHEN** an operator runs the audit without the apply option while «Белый Клык» and «Зов предков» share the cover ID `9157583`
- **THEN** both books are listed with the reason `shared_cover` and no book changes

#### Scenario: Clearing the reviewed books
- **WHEN** an operator runs the audit with the apply option and the ID of «Капитанская дочка»
- **THEN** only that book loses its cover link and ISBN, and it shows «Нет обложки» on the site

#### Scenario: ID that is not a candidate
- **WHEN** the apply list contains the ID of a book whose cover link is not an Open Library cover-ID link
- **THEN** that ID is reported as skipped and the book does not change

#### Scenario: Open Library down during the audit
- **WHEN** Open Library times out for one candidate
- **THEN** that candidate is reported with the error, and the other candidates are still checked

### Requirement: Cover backfill by Open Library ID
The system SHALL provide a maintenance command that gives existing harvested books a cover through their Open Library IDs and, when those give none, through their FantLab work IDs. An operator with database access runs it from the command line. It has no HTTP endpoint and no admin UI.

The candidates are books that:
- have Wikidata as their source;
- have a stored Wikidata QID matching `^Q[0-9]+$`;
- have no cover link;
- are not `rejected`.

The command reads the candidates' Open Library IDs (P648) and FantLab work IDs (P7439) from Wikidata in batches of at most 50 QIDs. To each candidate with Open Library IDs it applies the rule of "Cover by Open Library ID" in the Open Library enrichment requirement. To each candidate that got no cover that way (no Open Library IDs, no usable cover, or an Open Library error) and that has FantLab work IDs, it applies the rule of "Cover from a FantLab edition", with the book's stored title as its Russian title. FantLab requests follow the pacing of that requirement. The per-job request cap does not apply, because the command is not a harvest job.

By default the command MUST change nothing. It prints:
- each candidate that got a cover: book ID, status, title, QID, the source ID used (the Open Library ID, or `fantlab:<work ID>/<edition ID>`) and the cover link;
- each candidate that has IDs but got no cover: book ID, title, QID and its IDs;
- the totals of candidates, candidates with Open Library IDs, candidates with FantLab work IDs, covers found from Open Library, covers found from FantLab, and errors.

Only with an explicit apply option does it write. It then sets the cover link of every candidate that got a cover. When an explicit list of book IDs is also given, it writes only those books. A listed ID that is unknown or is not a candidate MUST be reported and skipped.

Writing MUST go through the normal book save, so the cover copy follows `catalog/book-covers` (a FantLab link becomes `linked` and is never downloaded). Each write MUST re-read the book first and skip it when it has a cover link by then. Only the cover link changes; the ISBN, description, status and every other field stay as they are. Each write reports `updated`, `skipped: …` or `error: …`.

A Wikidata failure for a batch MUST be reported for the books of that batch. An Open Library or FantLab failure for one book MUST be reported for that book. Neither stops the run. The command never clears or deletes anything. A second run therefore finds only the books that are still without a cover.

#### Scenario: Backfill report only
- **WHEN** an operator runs the backfill without the apply option, and a pending book from Wikidata without a cover has the Open Library ID `OL27479W` whose work has covers
- **THEN** that book is listed with `OL27479W` and its cover link, and no book changes

#### Scenario: Backfill applied
- **WHEN** an operator runs the backfill with the apply option
- **THEN** every listed book gets its cover link through the normal save, the cover copy follows `catalog/book-covers` for it, and no other field of those books changes

#### Scenario: Cover set in the meantime
- **WHEN** an admin set a cover link on a candidate after the report was built and before the write
- **THEN** that book is reported as skipped and keeps the admin's cover

#### Scenario: Rejected book
- **WHEN** a rejected book from Wikidata has no cover link but has an Open Library ID with a cover
- **THEN** it is not a candidate and does not change

#### Scenario: Wikidata down for one batch
- **WHEN** Wikidata fails for one batch of 50 QIDs
- **THEN** those 50 books are reported with the error, and the other batches are still processed

#### Scenario: Backfill from FantLab
- **WHEN** an operator runs the backfill without the apply option, and a book «Пикник на обочине» without a cover has no Open Library ID but has the FantLab work ID `7193`, whose Russian paper edition `16469` is named «Пикник на обочине» and has an image
- **THEN** the book is listed with `fantlab:7193/16469` and the cover link `https://fantlab.ru/images/editions/big/16469`, and no book changes

#### Scenario: Open Library cover wins in the backfill
- **WHEN** a candidate has both an Open Library ID with a cover and a FantLab work ID
- **THEN** the Open Library cover is listed and FantLab is not requested for that book

#### Scenario: FantLab down for one book
- **WHEN** FantLab answers 503 for one candidate's work
- **THEN** that candidate is reported with the error, and the other candidates are still checked

### Requirement: Cover from a FantLab edition
When a work still has no cover after the Open Library ID step, the job SHALL try the FantLab work IDs that Wikidata lists for the work (property P7439). The rule is strict: no cover is better than a wrong one.
- **IDs.** The numeric FantLab work ID is taken from each value, which can be written as `work<digits>` or `<digits>`. Other values are ignored. Duplicates are removed, and the IDs are tried in ascending numeric order. At most 2 IDs are tried per work.
- **Work.** For each ID the job reads the FantLab work with its list of editions. A work whose normalized FantLab name is not equal to the normalized Russian title of the book is not used.
- **Editions.** Only paper editions with the language `ru` are considered. Audio editions, magazine publications and planned editions are ignored. An edition qualifies only when its normalized name equals the normalized Russian title of the book. Normalization is the same as for the title search, but subtitles are never cut, so a collection under another name never qualifies.
- **Choice.** Qualifying editions are tried newest year first. Editions without a year come last, and ties go to the lower edition ID. At most 3 editions are checked per work. The first edition that FantLab reports as having an image wins. Its cover link is `https://fantlab.ru/images/editions/big/<edition ID>`. The image itself is never requested by the harvest.
- **Failures.** A missing work, a work without a qualifying edition with an image, or a failed request moves on to the next ID. None of these fails the book.
- **Scope.** This step MUST fill only the cover. Description, ISBN, page count, year and title are never taken from FantLab.
- **Pacing.** Requests to FantLab MUST be at least 1.2 seconds apart. One work makes at most 4 FantLab requests. One harvest job makes at most 250 FantLab requests. After that, the remaining works of that job skip this step, and the backfill can cover them later.

#### Scenario: Self-titled Russian edition
- **WHEN** a work «Пикник на обочине» without an Open Library cover has the FantLab work ID `7193`, and FantLab lists the Russian paper edition `16469` named «Пикник на обочине» with an image
- **THEN** the book is created with the cover link `https://fantlab.ru/images/editions/big/16469`

#### Scenario: Only a collection edition
- **WHEN** the only Russian paper editions of a work «Улитка на склоне» are named «Улитка на склоне. Повести» and «Избранное»
- **THEN** the book gets no cover from FantLab

#### Scenario: Wrong FantLab work
- **WHEN** a work «Трудно быть богом» has a FantLab work ID whose FantLab work is named «Понедельник начинается в субботу»
- **THEN** none of that work's editions is used

#### Scenario: Newest edition with an image
- **WHEN** three Russian paper editions named like the work exist, from 2015 without an image, from 2008 with an image, and from 1990 with an image
- **THEN** the 2015 edition is checked first, and the cover of the 2008 edition is used

#### Scenario: Audio and foreign editions ignored
- **WHEN** the only editions named like the work are an audiobook and an English paper edition
- **THEN** the book gets no cover from FantLab

#### Scenario: FantLab unavailable
- **WHEN** FantLab times out for a work's FantLab ID
- **THEN** the book is still created, and the title search is still tried for its cover

#### Scenario: Request cap reached
- **WHEN** a harvest job has already made 250 FantLab requests
- **THEN** the job's remaining works are created without a FantLab lookup

#### Scenario: Open Library cover by ID wins
- **WHEN** a work gets a cover through its Open Library ID and also has a FantLab work ID
- **THEN** no FantLab request is made for that work
