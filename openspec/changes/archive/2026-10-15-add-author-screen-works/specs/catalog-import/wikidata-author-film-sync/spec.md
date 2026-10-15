## Purpose
Finds films about catalog authors in Wikidata, both biopics and documentaries, on an admin's request or after an author import or a harvest. It links them to the authors and publishes them at once. It never duplicates a work, never overwrites data edited by hand and never brings back a link an admin hid.

## ADDED Requirements

### Requirement: Author film sync endpoint
`POST /api/v1/admin/authors/sync-screen-works` `{ids}` SHALL require the `admin` role and a bearer header. It syncs 1 to 50 distinct authors of any status and returns 200 with `{processed, updated, added, items}`. There is one item per author: `{author_id, name, outcome, found, added, notes, message}`.
- `found` counts the films Wikidata lists for the author after filtering.
- `added` counts the links this request created.
- The top-level `updated` counts the authors with outcome `added`.
- The top-level `added` is the sum of the items' `added`.

Duplicate ids count once. Empty `ids`, more than 50 ids or a malformed id MUST return 400 and change nothing. A moderator gets 403. Without a configured Wikidata source the response is 503. When every Wikidata request of the call failed, it is 502.

#### Scenario: Too many authors
- **WHEN** an admin sends 51 author ids
- **THEN** the response is 400 and nothing is changed

#### Scenario: Moderator
- **WHEN** a moderator calls the author film sync endpoint
- **THEN** the response is 403

#### Scenario: Wikidata down
- **WHEN** an admin syncs two authors with a QID and every Wikidata request fails
- **THEN** the response is 502 and no link is created

### Requirement: Author film sync outcomes
Each author SHALL get exactly one outcome:
- `added`: at least one new link was created.
- `unchanged`: Wikidata lists no film that is not linked yet, including when every listed one is already linked or hidden.
- `skipped`: the author has no usable Wikidata QID. Note `no_qid`.
- `error`: the author is unknown (message «author not found»), Wikidata is unavailable for the request that covered it (note `wikidata_unavailable`), or saving failed.

A failure on one author MUST NOT abort the others.

#### Scenario: Author without a QID
- **WHEN** an admin syncs an author that was added by hand and has no Wikidata QID
- **THEN** its outcome is `skipped` with the note `no_qid` and Wikidata is not asked about it

#### Scenario: Author without films
- **WHEN** an admin syncs an author that no Wikidata video work has as its main subject
- **THEN** its outcome is `unchanged` with `found: 0` and `added: 0`

### Requirement: Which Wikidata items are films about an author
A film about an author SHALL be a Wikidata item whose main subject (P921) is the author's QID and that is an instance of an allowed class. Other properties, such as "depicts" or "characters", MUST NOT make an item a film about the author.

The allowed classes and their types are those of book adaptations (see `catalog-import/wikidata-adaptation-sync`), with the same priority order. In addition:
- documentary film and biographical film give `film`;
- television documentary gives `tv_film`;
- documentary television series gives `series`.

An item that is also a film series, a video game, a board game, a list article, a television episode or a series of creative works MUST be excluded even if it has an allowed class. No genre is required: any allowed video work with the author as main subject is linked. Each item MUST appear at most once per author, whatever the number of its statements.

#### Scenario: Biographical film
- **WHEN** a film has Tolkien as its main subject and the genre biographical film
- **THEN** it is linked to Tolkien as a `biopic`

#### Scenario: Television biopic
- **WHEN** a television film has Rowling as its main subject
- **THEN** it is linked to Rowling with type `tv_film` as a `biopic`

#### Scenario: Film without a genre
- **WHEN** a film has the author as its main subject and no genre in Wikidata
- **THEN** it is linked as a `biopic`

#### Scenario: Documentary series episode excluded
- **WHEN** an episode of a documentary television series has the author as its main subject
- **THEN** the episode is not linked

#### Scenario: Book and article about the author
- **WHEN** a biography book and a scholarly article have the author as their main subject
- **THEN** neither is linked

### Requirement: Biopic or documentary
A film about an author SHALL be linked as `documentary` when one of its classes or one of its genres (P136) is documentary film or a documentary television class. Otherwise it is linked as `biopic`. The relation is set when the link is created, and a later sync MUST NOT change it.

#### Scenario: Documentary by class
- **WHEN** an item is an instance of documentary film and has the author as its main subject
- **THEN** it is linked with type `film` as a `documentary`

#### Scenario: Documentary by genre
- **WHEN** an item is an instance of film with the genre documentary film and has the author as its main subject
- **THEN** it is linked as a `documentary`

### Requirement: Author film data
For each film the sync SHALL take the title, original title, year, directors, IMDb id and Kinopoisk id with the rules of book adaptations (see `catalog-import/wikidata-adaptation-sync`):
- **Title.** The Russian label, else the English label, else the original title (P1476). An item without any of them is not linked and does not count in `found`.
- **Year.** The year of the earliest publication date (P577). A year outside 1880 to 2100 is ignored.

A missing or malformed value leaves that field empty without failing the item. Poster images MUST NOT be fetched or stored.

#### Scenario: Several release dates
- **WHEN** a film about an author has the publication dates 2019 and 2020
- **THEN** its year is 2019 and it is linked once

### Requirement: Idempotent, additive author film sync
A work SHALL be stored once per Wikidata QID and shared with book adaptations: a work already stored as an adaptation is linked to the author without creating a second work. The same work MAY be linked to several authors. A repeated sync MUST NOT create a second work or a second link. The sync only adds works and links. It never removes a link or a work, even when Wikidata no longer lists it. For a work that already exists, the sync MUST NOT change its title or type. It fills an empty year, original title, directors, IMDb id or Kinopoisk id. It MUST NOT change a work that was edited by hand.

#### Scenario: Re-sync
- **WHEN** an admin syncs the same author twice and Wikidata did not change
- **THEN** the second outcome is `unchanged` with the same `found` and `added: 0`, and no work is duplicated

#### Scenario: Work already stored as an adaptation
- **WHEN** a work stored as a book adaptation also has an author as its main subject and that author is synced
- **THEN** one work exists and it is linked to both the book and the author

#### Scenario: Documentary about two authors
- **WHEN** a documentary has two catalog authors as main subjects and both are synced
- **THEN** one work exists and it is linked to both authors

### Requirement: Hidden author film links stay hidden
A film link an admin hid SHALL stay hidden on every later sync: the sync MUST NOT unhide it, create it again or count it in `added`.

#### Scenario: Re-sync after hiding
- **WHEN** an admin hides a wrong film of an author and then syncs that author again
- **THEN** the link is still hidden, the outcome is `unchanged` and the public author page does not show it

### Requirement: Immediate publication of author films
A link created by the sync SHALL be visible on the public author page at once, without moderation, whenever the author is published (see `screen-works/author-films`). A link created for a pending author becomes visible when the author is published.

#### Scenario: Pending author
- **WHEN** films are synced for a pending author that is later published
- **THEN** they appear on the author's public page from the moment of publication

### Requirement: Author film sync actions in the admin
The admin SHALL offer «Найти фильмы об авторе в Wikidata» in three places, only on authors with a Wikidata QID:
- on author list rows;
- on rows of every tab of the author moderation queue, with «Найти фильмы об авторе в Wikidata (<n>)» for the selected rows, where <n> counts only rows with a QID and only those are sent;
- on the author edit page.

The single result reads:
- «Автор «<name>»: добавлено фильмов — <k>.»;
- «Автор «<name>»: новых фильмов нет.»;
- «Автор «<name>»: пропущен — нет QID Wikidata.»;
- «Автор «<name>»: не удалось найти фильмы — <reason>.», where the reason for `wikidata_unavailable` is «Wikidata недоступна».

The bulk result reads «Обновлено авторов: <n> из <m>, добавлено фильмов: <k>.». It adds «Без QID пропущено: <j>.» when selected rows were not sent, and up to three per-author error or skip details. Failed calls show:
- «Wikidata недоступна, попробуйте позже.» for 502;
- «Поиск фильмов об авторах не настроен.» for 503;
- «Не удалось найти фильмы об авторе.» otherwise.

The button is disabled while a sync is running.

#### Scenario: Bulk sync with authors lacking a QID
- **WHEN** an admin selects five authors in the moderation queue, two of them without a QID, and clicks «Найти фильмы об авторе в Wikidata (3)»
- **THEN** three authors are sent and the banner includes «Без QID пропущено: 2.»

#### Scenario: Author without a QID in the list
- **WHEN** an author in the author list has no Wikidata QID
- **THEN** its row has no «Найти фильмы об авторе в Wikidata» action
