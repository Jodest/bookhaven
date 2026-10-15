## Purpose

Finds screen adaptations of catalog books in Wikidata on an admin's request or after a harvest. It links them to the books and publishes them at once. It never duplicates a work, never overwrites data edited by hand and never brings back a link an admin hid.

## ADDED Requirements

### Requirement: Adaptation sync endpoint
`POST /api/v1/admin/books/sync-adaptations` `{ids}` SHALL require the `admin` role and a bearer header. It syncs 1 to 50 distinct books of any status and returns 200 with `{processed, updated, added, items}`. There is one item per book: `{book_id, title, outcome, found, added, notes, message}`. `found` counts the adaptations Wikidata lists for the book after filtering. `added` counts the links this request created. The top-level `updated` counts the books with outcome `added`, and the top-level `added` is the sum of the items' `added`. Duplicate ids count once. Empty `ids`, more than 50 ids or a malformed id MUST return 400 and change nothing. A moderator gets 403. Without a configured Wikidata source the response is 503. When every Wikidata request of the call failed, it is 502.

#### Scenario: Too many books
- **WHEN** an admin sends 51 book ids
- **THEN** the response is 400 and nothing is changed

#### Scenario: Moderator
- **WHEN** a moderator calls the sync endpoint
- **THEN** the response is 403

#### Scenario: Wikidata down
- **WHEN** an admin syncs two books with a QID and every Wikidata request fails
- **THEN** the response is 502 and no link is created

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

A missing or malformed value leaves that field empty without failing the item. Poster images MUST NOT be fetched or stored.

#### Scenario: Several release dates
- **WHEN** an adaptation has the publication dates 1967 and 1972
- **THEN** its year is 1967 and it is linked once

#### Scenario: No Russian label
- **WHEN** an adaptation has only an English label «The Master and Margarita»
- **THEN** it is linked with that English title

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

The bulk result reads «Обновлено книг: <n> из <m>, добавлено экранизаций: <k>.». It adds «Без QID пропущено: <j>.» when selected rows were not sent, and up to three per-book error or skip details. Failed calls show «Wikidata недоступна, попробуйте позже.» (502), «Поиск экранизаций не настроен.» (503) or «Не удалось найти экранизации.». The button is disabled while a sync is running.

#### Scenario: Bulk sync with books lacking a QID
- **WHEN** an admin selects five books in the moderation queue, two of them without a QID, and clicks «Найти экранизации в Wikidata (3)»
- **THEN** three books are sent and the banner includes «Без QID пропущено: 2.»

#### Scenario: Book without a QID in the list
- **WHEN** a book in the book list has no Wikidata QID
- **THEN** its row has no «Найти экранизации в Wikidata» action
