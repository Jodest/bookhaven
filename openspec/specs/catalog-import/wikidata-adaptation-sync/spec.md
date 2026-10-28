# catalog-import/wikidata-adaptation-sync Specification

## Purpose
Finds screen adaptations of catalog books in Wikidata on an admin's request or after a harvest. It links them to the books and publishes them at once. It never duplicates a work, never overwrites data edited by hand and never brings back a link an admin hid.

## Requirements

### Requirement: Adaptation sync endpoint
`POST /api/v1/admin/books/sync-adaptations` `{ids}` SHALL require the `admin` role and a bearer header. It syncs 1 to 50 distinct books of any status and returns 200 with `{processed, updated, added, posters, items}`. There is one item per book: `{book_id, title, outcome, found, added, posters, notes, message}`. `found` counts the adaptations Wikidata lists for the book after filtering. `added` counts the links this request created. `posters` counts the works of the book that got a poster in this request (see «Adaptation posters»). The top-level `updated` counts the books with outcome `added`, and the top-level `added` and `posters` are the sums of the items' values. Duplicate ids count once. Empty `ids`, more than 50 ids or a malformed id MUST return 400 and change nothing. A moderator gets 403. Without a configured Wikidata source the response is 503. When every Wikidata request of the call failed, it is 502.

#### Scenario: Too many books
- **WHEN** an admin sends 51 book ids
- **THEN** the response is 400 and nothing is changed

#### Scenario: Moderator
- **WHEN** a moderator calls the sync endpoint
- **THEN** the response is 403

#### Scenario: Wikidata down
- **WHEN** an admin syncs two books with a QID and every Wikidata request fails
- **THEN** the response is 502 and no link is created

#### Scenario: Poster counter
- **WHEN** an admin syncs a book whose two already linked adaptations get a poster and no new adaptation is found
- **THEN** its item has `outcome: "unchanged"`, `added: 0` and `posters: 2`, and the top-level `posters` is 2

### Requirement: Sync outcomes
Each book SHALL get exactly one outcome:
- `added`: at least one new link was created.
- `unchanged`: Wikidata lists no adaptation that is not linked yet, including when every listed one is already linked or hidden.
- `skipped`: the book has no usable Wikidata QID. Note `no_qid`.
- `error`: the book is unknown (message «book not found»), Wikidata is unavailable for the request that covered it (note `wikidata_unavailable`), or saving failed.

A failure on one book MUST NOT abort the others.

#### Scenario: Book without a QID
- **WHEN** an admin syncs a book that was added by hand and has no Wikidata QID
- **THEN** its outcome is `skipped` with the note `no_qid` and Wikidata is not asked about it

#### Scenario: Book without adaptations
- **WHEN** an admin syncs a book whose work has no adaptation in Wikidata
- **THEN** its outcome is `unchanged` with `found: 0` and `added: 0`

### Requirement: Which Wikidata items are adaptations
An adaptation of a book SHALL be a Wikidata item that is "based on" (P144) the book's QID and is an instance of an allowed class. Each class maps to a type:
- teleplay: `teleplay`;
- animated series: `animated_series`;
- television series or miniseries: `series`;
- animated film: `animated_film`;
- television film: `tv_film`;
- film: `film`.

An item with several allowed classes takes the first type in this order. An item that is also a film series, a video game, a board game, a list article or a television episode MUST be excluded even if it has an allowed class. Documentaries and items without an allowed class are not adaptations. Each item MUST appear at most once per book, whatever the number of its statements.

#### Scenario: Film series excluded
- **WHEN** a book is the basis of a film series item and of the three films in that series
- **THEN** the three films are linked and the film series is not

#### Scenario: Game and list article excluded
- **WHEN** Wikidata lists a video game and the list article «film and television adaptations of The Hobbit» as based on a book
- **THEN** neither is linked

#### Scenario: Animated series
- **WHEN** an adaptation is an instance of animated series
- **THEN** it is linked with type `animated_series`

### Requirement: Adaptation data
For each adaptation the sync SHALL take:
- **Title.** The Russian label, else the English label, else the original title (P1476). An item without any of them is not linked and does not count in `found`.
- **Original title.** The original title (P1476) when it differs from the stored title.
- **Year.** The year of the earliest publication date (P577). A year outside 1880 to 2100 is ignored.
- **Directors.** Up to three director names (P57), each by its Russian label, else its English label, sorted by name and joined with «, ».
- **IMDb id.** The first IMDb id (P345) of the form `tt` followed by digits.
- **Kinopoisk id.** The first Kinopoisk film id (P2603) made only of digits.
- **Poster hints.** One film poster file (P3383), one image file (P18), one TMDB movie id (P4947) and one TMDB TV series id (P4983). The TMDB ids are digits only. They are only used to find a poster (see «Adaptation posters»). Only the one TMDB id whose poster the work actually gets is kept, together with that poster, so that the poster can be re-checked later (see `screen-works/poster-copies`). It is not shown anywhere.

A missing or malformed value leaves that field empty without failing the item.

#### Scenario: Several release dates
- **WHEN** an adaptation has the publication dates 1967 and 1972
- **THEN** its year is 1967 and it is linked once

#### Scenario: No Russian label
- **WHEN** an adaptation has only an English label «The Master and Margarita»
- **THEN** it is linked with that English title

#### Scenario: Malformed TMDB id
- **WHEN** an adaptation's TMDB movie id is not made of digits
- **THEN** it is linked as usual and TMDB is not asked about it

### Requirement: Idempotent, additive sync
A work SHALL be stored once per Wikidata QID, and the same work MAY be linked to several books. A repeated sync MUST NOT create a second work or a second link. The sync only adds works and links: it never removes a link or a work, even when Wikidata no longer lists it. For a work that already exists, the sync MUST NOT change its title or type. It fills an empty year, original title, directors, IMDb id or Kinopoisk id. It MUST NOT change a work that was edited by hand.

#### Scenario: Re-sync
- **WHEN** an admin syncs the same book twice and Wikidata did not change
- **THEN** the second outcome is `unchanged` with the same `found` and `added: 0`, and no work is duplicated

#### Scenario: Work shared by two books
- **WHEN** a film is based on two catalog books and both are synced
- **THEN** one work exists and it is linked to both books

#### Scenario: Missing year filled later
- **WHEN** a work was stored without a year and Wikidata later has a publication date for it
- **THEN** the next sync fills the year and keeps the title

### Requirement: Hidden links stay hidden
A link an admin hid SHALL stay hidden on every later sync: the sync MUST NOT unhide it, create it again or count it in `added`.

#### Scenario: Re-sync after hiding
- **WHEN** an admin hides a wrong adaptation of a book and then syncs that book again
- **THEN** the link is still hidden, the outcome is `unchanged` and the public book page does not show it

### Requirement: Immediate publication
A link created by the sync SHALL be visible on the public book page at once, without moderation, whenever the book is published (see `screen-works/book-adaptations`). A link created for a pending book becomes visible when the book is published.

#### Scenario: Pending book
- **WHEN** adaptations are synced for a pending book that is later published
- **THEN** they appear on its public page from the moment of publication

### Requirement: Sync actions in the admin
The admin offers «Найти экранизации в Wikidata» in three places, only on books with a Wikidata QID:
- on book list rows;
- on rows of every tab of the book moderation queue, with «Найти экранизации в Wikidata (<n>)» for the selected rows, where <n> counts only rows with a QID and only those are sent;
- on the book edit page.

The single result SHALL read:
- «Книга «<title>»: добавлено экранизаций — <k>.»;
- «Книга «<title>»: новых экранизаций нет.»;
- «Книга «<title>»: пропущена — нет QID Wikidata.»;
- «Книга «<title>»: не удалось найти экранизации — <reason>.», where the reason for `wikidata_unavailable` is «Wikidata недоступна».

The first two add « Добавлено постеров: <p>.» when the book's `posters` is above 0.

The bulk result reads «Обновлено книг: <n> из <m>, добавлено экранизаций: <k>.». It adds «Добавлено постеров: <p>.» when the top-level `posters` is above 0, «Без QID пропущено: <j>.» when selected rows were not sent, and up to three per-book error or skip details. Failed calls show «Wikidata недоступна, попробуйте позже.» (502), «Поиск экранизаций не настроен.» (503) or «Не удалось найти экранизации.». The button is disabled while a sync is running.

#### Scenario: Bulk sync with books lacking a QID
- **WHEN** an admin selects five books in the moderation queue, two of them without a QID, and clicks «Найти экранизации в Wikidata (3)»
- **THEN** three books are sent and the banner includes «Без QID пропущено: 2.»

#### Scenario: Book without a QID in the list
- **WHEN** a book in the book list has no Wikidata QID
- **THEN** its row has no «Найти экранизации в Wikidata» action

#### Scenario: Posters backfilled from the edit page
- **WHEN** an admin syncs a book on its edit page and no new adaptation is found but two posters are filled
- **THEN** the banner reads «Книга «<title>»: новых экранизаций нет. Добавлено постеров: 2.»

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
The job result SHALL be `{total, processed, updated, added, posters, unchanged, skipped, failed, errors}`:
- `total` is the number of books the job selected and `processed` the number already handled;
- `updated`, `unchanged`, `skipped` and `failed` count the books with the outcome `added`, `unchanged`, `skipped` and `error`;
- `added` is the number of links created;
- `posters` is the number of works that got a poster;
- `errors` lists `{book_id, title, reason}` for failed books, where `reason` is `wikidata_unavailable` or `save_failed`, at most the first 50 of them; `failed` still counts all.

While the job is active, the result MUST be updated after every chunk so the progress can be shown. A progress-write failure MUST NOT fail the job.

#### Scenario: Progress of a catalog sync
- **WHEN** an admin views a catalog sync job that has handled 2 of 14 chunks of a 683-book catalog
- **THEN** the job shows `processed: 100` and `total: 683`

#### Scenario: Many failures
- **WHEN** 120 books of a completed catalog sync failed
- **THEN** the result has `failed: 120` and 50 entries in `errors`

#### Scenario: Catalog-wide poster backfill
- **WHEN** a catalog sync runs over books whose adaptations were linked before posters existed
- **THEN** the result's `posters` counts the works that got a poster, and the books' outcomes are `unchanged` when no new link was created

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

### Requirement: Adaptation posters
For each adaptation Wikidata lists for a synced book, the sync SHALL fill the work's poster only when all of these hold:
- the work has no poster;
- an admin did not remove its automatic poster;
- the work was not edited by hand.

It MUST NOT replace or remove any existing poster, in particular one uploaded by an admin. The sources are tried in this order, and the first usable one wins:
1. **Wikimedia Commons.** The film poster file (P3383), then the image file (P18). A file is usable when:
   - its licence is public domain, CC0, CC BY or CC BY-SA and it is not marked non-free;
   - Commons returns a thumbnail on a Wikimedia upload host;
   - for CC BY and CC BY-SA, its author is known.

   The poster keeps the author, the licence, the licence URL and the file page. The Wikimedia thumbnail is the poster's source link. The image is copied to the site's own storage (see `screen-works/poster-copies`).
2. **TMDB**, only when a TMDB API key is configured. It takes the poster of the TMDB movie id (P4947) or of the TMDB TV series id (P4983); the TV id is tried first for series and animated series. The poster is requested for the Russian language. The poster's source link is on TMDB's image server at width 500. The image is copied to the site's own storage, and the TMDB id it came from is kept for the re-check (see `screen-works/poster-copies`). Without a key this step is skipped silently, and TMDB is never called.
3. Otherwise the work stays without a poster, and a later sync tries again.

Every poster the sync fills queues a copy of it. A failure to queue does not change the sync's result. The sync covers works that are already linked, so re-running it fills the posters of existing adaptations. This applies to a single book, a bulk sync, the catalog sync job and the sync after a harvest. A Commons or TMDB failure MUST NOT change a book's outcome, its `added` or the HTTP status, and the affected work stays without a poster. One chunk of up to 50 books makes at most 100 TMDB lookups; the remaining works wait for a later sync. The film sync of authors does not fetch posters.

#### Scenario: Commons film poster
- **WHEN** a synced adaptation has a public-domain film poster file on Commons
- **THEN** the work gets a poster with `source: "commons"` whose source link is a Wikimedia thumbnail, a copy of it is queued, and its item counts it in `posters`

#### Scenario: Non-free image falls back to TMDB
- **WHEN** an adaptation's only Commons image is not free, it has a TMDB movie id and the TMDB key is configured
- **THEN** the work gets a poster with `source: "tmdb"` whose source link starts with `https://image.tmdb.org/t/p/w500/`

#### Scenario: No TMDB key
- **WHEN** the TMDB key is not configured and an adaptation has only a TMDB movie id
- **THEN** no TMDB request is made, the work has no poster and the book's outcome is unaffected

#### Scenario: Backfill on re-sync
- **WHEN** an admin syncs again a book whose three adaptations were linked before posters existed and Wikidata has posters for two of them
- **THEN** the outcome is `unchanged` with `added: 0` and `posters: 2`

#### Scenario: Uploaded poster kept
- **WHEN** a work has a poster uploaded by an admin and Wikidata has a free film poster for it
- **THEN** the sync keeps the uploaded poster

#### Scenario: Removed poster stays removed
- **WHEN** an admin removed the automatic poster of a work and the book is synced again
- **THEN** the work still has no poster

#### Scenario: Work edited by hand
- **WHEN** an admin fixed the title of a work without a poster and the book is synced again
- **THEN** the work still has no poster and keeps the admin's title

#### Scenario: TMDB unavailable
- **WHEN** every TMDB request of a sync fails
- **THEN** the books get their usual outcomes and links, and the works that needed TMDB stay without a poster
