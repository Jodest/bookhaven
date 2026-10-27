## MODIFIED Requirements

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

### Requirement: Adaptation data
For each adaptation the sync SHALL take:
- **Title.** The Russian label, else the English label, else the original title (P1476). An item without any of them is not linked and does not count in `found`.
- **Original title.** The original title (P1476) when it differs from the stored title.
- **Year.** The year of the earliest publication date (P577). A year outside 1880 to 2100 is ignored.
- **Directors.** Up to three director names (P57), each by its Russian label, else its English label, sorted by name and joined with «, ».
- **IMDb id.** The first IMDb id (P345) of the form `tt` followed by digits.
- **Kinopoisk id.** The first Kinopoisk film id (P2603) made only of digits.
- **Poster hints.** One film poster file (P3383), one image file (P18), one TMDB movie id (P4947) and one TMDB TV series id (P4983). The TMDB ids are digits only. They are only used to find a poster (see «Adaptation posters») and are not stored.

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

## ADDED Requirements

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

   The poster keeps the author, the licence, the licence URL and the file page. The image is linked from Wikimedia and not copied.
2. **TMDB**, only when a TMDB API key is configured. It takes the poster of the TMDB movie id (P4947) or of the TMDB TV series id (P4983); the TV id is tried first for series and animated series. The poster is requested for the Russian language. The image is linked from TMDB's image server at width 342 and not copied. Without a key this step is skipped silently, and TMDB is never called.
3. Otherwise the work stays without a poster, and a later sync tries again.

The sync covers works that are already linked, so re-running it fills the posters of existing adaptations. This applies to a single book, a bulk sync, the catalog sync job and the sync after a harvest. A Commons or TMDB failure MUST NOT change a book's outcome, its `added` or the HTTP status, and the affected work stays without a poster. One chunk of up to 50 books makes at most 100 TMDB lookups; the remaining works wait for a later sync. The film sync of authors does not fetch posters.

#### Scenario: Commons film poster
- **WHEN** a synced adaptation has a public-domain film poster file on Commons
- **THEN** the work gets a poster with `source: "commons"` linking a Wikimedia thumbnail, and its item counts it in `posters`

#### Scenario: Non-free image falls back to TMDB
- **WHEN** an adaptation's only Commons image is not free, it has a TMDB movie id and the TMDB key is configured
- **THEN** the work gets a poster with `source: "tmdb"`

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
