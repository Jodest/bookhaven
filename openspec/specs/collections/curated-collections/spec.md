# collections/curated-collections Specification

## Purpose
Lets anyone browse ranked book collections curated by BookHaven itself («подборки», e.g. «Топ-100 книг всех времён») — distinct from readers' own lists — with places computed from the admin's order and the books' publication status.

## Requirements

### Requirement: Curated collection
A curated collection SHALL belong to BookHaven, not to a user, and MUST have a title (2-255 characters), a unique slug (lowercase Latin letters, digits and single hyphens, at most 100 characters), a description (may be empty), a size N (1-1000 ranked places), a visibility (draft or published, draft by default) and a kind. Optionally it MAY name a source (free text up to 255 characters, e.g. «по версии BBC Big Read, 2003») with an `http`/`https` URL, and link one catalog genre. The kind SHALL be `manual` (books and order chosen by an admin) or `auto` (built from site data by a rule, see «Automatic collection rule»). It is chosen when the collection is created, `manual` when none is given, and MUST NOT change afterwards. Only automatic collections have a rule. Their linked genre also acts as the rule's genre filter.

#### Scenario: New collection is a draft
- **WHEN** an admin creates a collection without choosing its visibility
- **THEN** it is a draft and does not appear publicly

#### Scenario: Kind of a new collection
- **WHEN** a collection is created without a kind
- **THEN** its kind is `manual`

#### Scenario: Automatic collection
- **WHEN** an admin creates a collection of kind `auto` with the metric `weighted_rating`
- **THEN** its kind is `auto` and its books come from the rule, not from an admin list

### Requirement: Ranked places and reserve
A manual collection SHALL hold one ordered list of distinct catalog books that may be longer than its size N. Its public places MUST be computed at read time: take the books in the admin's order, keep only books whose status is `published`, and number the first N of them 1..N. Books after the N-th published one form the reserve and MUST NOT be shown publicly. A book that is not published MUST never be shown and MUST NOT take a place. Status changes of books MUST NOT rewrite the stored order. When fewer than N published books exist, the collection SHALL be shown with fewer places.

#### Scenario: Unpublished book shifts the rest up
- **WHEN** a collection of size 3 holds A, B, C, D in that order, all published, and B is unpublished
- **THEN** the public places are 1 A, 2 C, 3 D

#### Scenario: Republished book returns to its place
- **WHEN** B from the previous scenario is published again
- **THEN** the public places are 1 A, 2 B, 3 C and D is back in the reserve

#### Scenario: Reserve exhausted
- **WHEN** a collection of size 100 holds 98 published books and 4 pending books
- **THEN** the public collection shows 98 places, numbered 1..98

### Requirement: Public collection index API
`GET /api/v1/collections` SHALL be open to everyone without a token and return `{items, total}` with every published collection, manual or automatic, newest first by the time it was first published. Each item MUST carry:
- `id`, `slug`, `title`, `description`, `kind` and `size`;
- `shown_count`: the number of public places, at most `size`;
- `source` and `source_url`, null when absent;
- `genre`: `{id, name, slug}` only when the linked genre is published, otherwise null;
- `cover_urls`: covers of up to the first 4 placed books that have one;
- `rule`: `{metric, period_days, min_ratings, confidence}` for an automatic collection, null for a manual one;
- `computed_at`: the time of the current snapshot of an automatic collection, null for a manual one or before the first computation.

Draft collections MUST NOT be listed. An optional `limit` query parameter (1-50) SHALL cut `items` to that many collections, taken from the start of the same order, while `total` still counts every published collection. A `limit` that is missing, not a number or below 1 SHALL be ignored; a `limit` above 50 SHALL be treated as 50.

#### Scenario: Draft collection is not listed
- **WHEN** a visitor requests the index while one collection is published and one is a draft
- **THEN** only the published collection is returned

#### Scenario: Automatic collection is listed
- **WHEN** a visitor requests the index while a published automatic collection exists
- **THEN** it is listed like a manual one, with its `rule` and `computed_at`

#### Scenario: Limited index
- **WHEN** a visitor requests the index with `limit=2` while 5 collections are published
- **THEN** `items` holds the 2 most recently first-published collections and `total` is 5

### Requirement: Public collection API
`GET /api/v1/collections/{slug}` SHALL be open to everyone without a token and return `{collection, items, total, page, limit}` for a published collection.
- `collection` has the index fields, and `total` equals `shown_count`.
- `items` are `{place, movement, adaptation, book}` in place order.
- `book` carries `id`, `title`, `slug`, `cover_url`, `published_year`, `rating`, `ratings_count`, `authors` and published `genres`, in the same shape as books on a public list.
- `movement` is null for a manual collection and for an automatic collection without a movement baseline. Otherwise it is `{direction, by}`. `direction` is `up`, `down`, `same` or `new` (see «Position movement»). `by` is the number of positions moved, 0 for `same` and `new`.
- `adaptation` is set only for the verdict metrics, and only while the source adaptation's link is visible. It is `{id, title, year, kind, votes_count, share}`. These are the source adaptation's id, title, year (null when unknown) and kind, plus the counted votes and the raw share (0-1) of the metric's value stored by the snapshot. Otherwise `adaptation` is null.
- `page` defaults to 1 and `limit` to 50 (at most 100). Places keep their global numbers across pages.
- An unknown slug or a draft collection MUST return 404 «collection not found», for admins too.

#### Scenario: Second page keeps numbering
- **WHEN** a visitor requests page 2 with `limit=50` of a collection with 100 places
- **THEN** the items are places 51..100

#### Scenario: Draft collection link
- **WHEN** anyone requests the slug of a draft collection
- **THEN** the response is 404

#### Scenario: Manual collection has no movement
- **WHEN** a visitor requests a published manual collection
- **THEN** every item has `movement: null`

#### Scenario: Verdict collection names the adaptation
- **WHEN** a visitor requests a published `book_beats_adaptation` collection whose snapshot scored «Война и мир» by its 1967 film with 40 votes, 36 of them «Книга лучше»
- **THEN** the item has `adaptation` with that film's id, title, `year: 1967`, `kind: "film"`, `votes_count: 40` and `share: 0.9`

#### Scenario: Other metrics have no adaptation
- **WHEN** a visitor requests a published `weighted_rating` collection or a manual collection
- **THEN** every item has `adaptation: null`

### Requirement: Public collection pages
The web app SHALL provide two pages.
- `/top` («Подборки») lists published collections, manual and automatic alike, as cards with the title, up to 4 covers, «{shown_count} книг» and the source line. When there are none it shows «Подборок пока нет».
- `/top/{slug}` shows the title, the description, the source line («Источник: {source}», linked when a URL exists), the genre linked to the catalog filtered by it, and the ranked places. Each place shows the place number, the cover, the title linked to the book page, the authors (linked when published) and the rating.
- For an automatic collection, `/top/{slug}` SHALL also show «Составлена автоматически по {basis}{period}.», followed by «Обновлено {date}.» once it has been computed. {basis} is:
  - «оценкам читателей BookHaven» for `weighted_rating`;
  - «отметкам «Прочитано»» for `most_read`;
  - «отметкам «Хочу прочитать»» for `most_wanted`;
  - «активности читателей» for `trending`;
  - «голосам читателей «Книга или экранизация?»» for `book_beats_adaptation` and `adaptation_beats_book`.

  {period} is « за последние {period_days} дн.» when the rule has a period. {date} is the day of `computed_at`, e.g. «14 октября 2026».
- For a place with an `adaptation`, the page SHALL show a line under the rating. It reads «Книга лучше экранизации «{title}» ({year}): {P}% · {N} {голос|голоса|голосов}» for `book_beats_adaptation`, and «Экранизация «{title}» ({year}) лучше книги: {P}% · {N} {голос|голоса|голосов}» for `adaptation_beats_book`. P is the share as a whole percent and N is `votes_count`. « ({year})» is left out when the year is unknown. A place without an `adaptation` shows no such line.
- Next to each place number the page SHALL show the movement: «↑{by}» for `up`, «↓{by}» for `down`, «Новинка» for `new`, and nothing for `same` or a null movement.
- An unknown or draft collection shows «Подборка не найдена».
- Both pages MUST open for everyone, signed in or not.
- The site header SHALL show a «Подборки» link to `/top` to every visitor.

#### Scenario: Anonymous visitor opens a collection
- **WHEN** a visitor without session cookies opens `/top/{slug}` of a published collection
- **THEN** the ranked places are shown without a redirect to login

#### Scenario: Header link
- **WHEN** an anonymous visitor looks at the site header
- **THEN** it has a «Подборки» link to `/top`

#### Scenario: Movement indicators
- **WHEN** a visitor opens an automatic collection whose places have the movements `up` by 3, `down` by 2, `new` and `same`
- **THEN** the places show «↑3», «↓2», «Новинка» and no indicator respectively

#### Scenario: Automatic collection caption
- **WHEN** a visitor opens an automatic `trending` collection with a 30-day period computed on 14 October 2026
- **THEN** the page shows «Составлена автоматически по активности читателей за последние 30 дн. Обновлено 14 октября 2026.»

#### Scenario: Verdict collection caption
- **WHEN** a visitor opens an automatic `adaptation_beats_book` collection without a period, computed on 21 October 2026
- **THEN** the page shows «Составлена автоматически по голосам читателей «Книга или экранизация?». Обновлено 21 октября 2026.»

#### Scenario: Source adaptation line
- **WHEN** a visitor opens a `book_beats_adaptation` collection whose place 1 has the adaptation «Война и мир» (1967) with `votes_count` 40 and `share` 0.9
- **THEN** place 1 shows «Книга лучше экранизации «Война и мир» (1967): 90% · 40 голосов»

#### Scenario: Adaptation without a year
- **WHEN** a place of an `adaptation_beats_book` collection has an adaptation «Сталкер» with no year, `votes_count` 21 and `share` 0.62
- **THEN** it shows «Экранизация «Сталкер» лучше книги: 62% · 21 голос»

### Requirement: Automatic collection rule
An automatic collection SHALL rank published catalog books by the metric of its rule.
- **Genre filter.** When the collection links a genre, only books that have that genre are ranked.
- **Period.** The rule MAY have a period of 1-3650 days. A period limits the counted signals to those given within the last that many days before the computation. A rating counts by the time it was set or last changed. A shelf mark counts by the time the book was put on the shelf. An adaptation vote counts by the time its value was set or last changed; repeating the same value does not count as a change. Without a period, all signals count.
- **Metrics.** The metrics are:
  - `weighted_rating`: the weighted rating described in «Weighted rating».
  - `most_read`: the number of readers who have the book on their «Прочитано» shelf.
  - `most_wanted`: the number of readers who have the book on their «Хочу прочитать» shelf.
  - `trending`: the number of ratings set or changed, plus the number of books put on a reading-status shelf («Хочу прочитать», «Читаю», «Прочитано»), within the period. Its period is 30 days when none is given.
  - `book_beats_adaptation`: the share of «Книга лучше» votes on the book's source adaptation, described in «Verdict score».
  - `adaptation_beats_book`: the share of «Экранизация лучше» votes on the book's source adaptation, described in «Verdict score».
- **Eligibility.** For `most_read`, `most_wanted` and `trending`, a book MUST have a score of at least 1 to be ranked. For the verdict metrics, the eligibility rules of «Verdict score» apply.
- **Order and ties.** Books SHALL be ordered by score, highest first. Ties are broken by the book's total number of ratings (more first), then by title alphabetically, then by book id. For the verdict metrics, the source adaptation's counted votes (more first) break a tie before the number of ratings.
- **Signal sources.** Only ratings, reading-status shelves and adaptation votes count, and each metric counts only its own signals. Custom shelves, readers' lists and review likes MUST NOT count. Pending and rejected books MUST NOT be ranked, and their signals MUST NOT count.

#### Scenario: Most read
- **WHEN** a `most_read` collection is computed while book A is on 5 readers' «Прочитано» shelves, book B on 2, and book C only on «Хочу прочитать» shelves
- **THEN** the order is A, B, and C is not ranked

#### Scenario: Genre filter
- **WHEN** a `most_read` collection linked to the genre «Фантастика» is computed
- **THEN** only books with the genre «Фантастика» are ranked

#### Scenario: Period limits signals
- **WHEN** a `most_wanted` collection with a 7-day period is computed and a book was put on «Хочу прочитать» 10 days ago by its only reader
- **THEN** that book is not ranked

#### Scenario: Trending counts ratings and shelvings
- **WHEN** a `trending` collection with a 30-day period is computed and, within the period, a book got 2 ratings and was put on «Читаю» by 3 readers
- **THEN** its score is 5

#### Scenario: Verdict metrics rank opposite sides
- **WHEN** a book's source adaptation has 10 votes, 8 of them «Экранизация лучше»
- **THEN** the book can be ranked in an `adaptation_beats_book` collection and is not ranked in a `book_beats_adaptation` collection

#### Scenario: Votes do not feed other metrics
- **WHEN** a `trending` collection is computed and, within the period, a book got 7 adaptation votes and nothing else
- **THEN** that book is not ranked

### Requirement: Weighted rating
The `weighted_rating` score of a book SHALL be `(v·R + m·C) / (v + m)`, where:
- R is the average of the book's counted ratings;
- v is their number;
- C is the average of all counted ratings in the rule's scope (published books, the genre filter and the period), including ratings of books that are below the minimum rating count;
- m is the rule's confidence threshold.

A book MUST have at least the rule's minimum rating count (min ratings) of counted ratings to be ranked. Defaults: m = 10 and min ratings = 3; each may be set to 1-10000. The collection SHALL be computed the same way whatever the number of rated books or readers. There is no special handling of a young catalog: a collection simply holds fewer places when fewer books qualify. Only ratings from accounts that can sign in count. Accounts with an unconfirmed email cannot sign in and so cannot rate (see `auth/email-verification`). This invariant is relied on as the protection against throwaway accounts; ranking adds no other filter.

#### Scenario: Many ratings beat a few perfect ones
- **WHEN** a `weighted_rating` collection with m = 10 and min ratings = 1 is computed over book A (10 ratings, average 4.8), book B (2 ratings, both 5) and book C (8 ratings, average 3.0), so C = 4.1
- **THEN** the scores are A 4.45, B 4.25 and C 3.61, and the order is A, B, C

#### Scenario: Below the minimum rating count
- **WHEN** the same collection has min ratings = 3
- **THEN** B is not ranked, its ratings still count in the site average, and the order is A, C

### Requirement: Automatic snapshot
The places of an automatic collection SHALL come from a stored snapshot, never computed per request.
- **What a computation stores.** Each computation stores the ranked published books in score order. That is up to `size + 20` books, at most 1000, so that the books after the N-th form a reserve. The computation also stores each book's score and the computation time. For the verdict metrics it also stores each book's source adaptation, that adaptation's counted votes, and the raw share of the metric's value among them (k / v).
- **Places between computations.** Public places SHALL be derived from the snapshot exactly like the places of a manual collection from the admin's order. A book that is no longer published leaves its place and the following books shift up, and the reserve fills place N. A book published again gets its place back. New ratings, shelf marks and adaptation votes MUST NOT change the places, scores or stored vote figures until the next computation.
- **Source adaptation between computations.** When the source adaptation's link is hidden or deleted after the computation, the book keeps its place, but its source adaptation is not shown until the next computation.
- **Before the first computation.** An automatic collection that has never been computed has no places.

#### Scenario: Places are stable between computations
- **WHEN** readers rate books after the snapshot of a `weighted_rating` collection was computed
- **THEN** the public places stay the same until the next computation

#### Scenario: Unpublished between computations
- **WHEN** a book at place 2 of an automatic collection of size 3 is unpublished before the next computation
- **THEN** the former places 3 and 4 become places 2 and 3, and the next computation does not include the book

#### Scenario: Source adaptation hidden after the computation
- **WHEN** an admin hides the source adaptation of the book at place 1 of a `book_beats_adaptation` collection
- **THEN** the book stays at place 1 with `adaptation: null` until the next computation

### Requirement: Scheduled recomputation
The worker SHALL recompute every automatic collection, draft or published, on a schedule: daily at 01:00 UTC by default. The operator MAY change the time or switch the schedule off. When the schedule is off, automatic collections change only through admin actions. A computation that fails for one collection MUST keep that collection's current snapshot and MUST NOT stop the others. A computation and an admin edit of the same collection MUST NOT interleave: the computation uses the rule saved at the time it starts writing.

#### Scenario: One collection fails
- **WHEN** the scheduled recomputation fails for one of three automatic collections
- **THEN** the other two get new snapshots and the failed one keeps its previous snapshot

### Requirement: Position movement
An automatic collection SHALL remember, for every book in its snapshot, the book's position in the previous snapshot. The previous snapshot is its movement baseline. The movement of a place compares positions in the two snapshots, not public places:
- `up` by k when the book moved k positions higher;
- `down` by k when it moved k positions lower;
- `same` when its position is unchanged;
- `new` when the book was not in the previous snapshot, including its reserve.

When the collection is created, and when its rule, linked genre or size changes, it SHALL be recomputed at once without a baseline. Every movement is then null until the next computation. Every later computation, scheduled or requested by an admin, SHALL use the snapshot it replaces as the baseline.

#### Scenario: Book climbs
- **WHEN** a book was at position 7 in the previous snapshot and is at position 4 now
- **THEN** its movement is `up` by 3

#### Scenario: Book enters the collection
- **WHEN** a book is at position 10 now and was not in the previous snapshot
- **THEN** its movement is `new`

#### Scenario: Rule changed
- **WHEN** an admin changes the metric of an automatic collection
- **THEN** it is recomputed and every place has `movement: null` until the next computation

### Requirement: Book collection places API
`GET /api/v1/books/{id}/collections` SHALL be open to everyone without a token. It returns `{items, total}` with every public place the book holds in a published collection, manual or automatic.
- **Item shape.** Each item is `{place, collection}`, where `collection` is `{id, slug, title, kind, size}`. `total` is the number of items. There is no pagination.
- **Same places as the collection page.** The place MUST be exactly the place that `/api/v1/collections/{slug}` shows for the book at the same moment. It is computed at read time by the rules of «Ranked places and reserve» and «Automatic snapshot».
- **Excluded collections.**
  - Draft collections MUST NOT be listed.
  - A collection where the book is in the reserve (beyond place N) MUST NOT be listed.
  - An automatic collection that has never been computed MUST NOT be listed.
- **Order.** Items SHALL be ordered by:
  1. place, best first;
  2. then the smaller collection size;
  3. then newest first by the time the collection was first published;
  4. then title.
- **Errors.**
  - An id that is not a UUID MUST return 400 «invalid book id».
  - An unknown book, or a book whose status is not `published`, MUST return 404 «book not found».
- A published book in no collection returns 200 with `{items: [], total: 0}`.

#### Scenario: Place counts only published books
- **WHEN** a published collection of size 3 holds A, B, C, D in that order, B is pending, and a visitor requests the places of C
- **THEN** the response has one item with `place` 2 for that collection

#### Scenario: Reserve book has no place
- **WHEN** a published collection of size 2 holds published A, B, C and a visitor requests the places of C
- **THEN** that collection is not in the response

#### Scenario: Draft collection is not counted
- **WHEN** a book holds place 1 in a draft collection and in no other collection
- **THEN** the response is 200 with `{items: [], total: 0}`

#### Scenario: Automatic collection place
- **WHEN** a book is at snapshot position 4 of a published automatic collection of size 10 and one book ahead of it is no longer published
- **THEN** its item has `place` 3

#### Scenario: Best place first
- **WHEN** a book is №5 in a collection of size 100, №1 in a collection of size 1000 and №1 in a collection of size 50
- **THEN** the items come in the order: №1 of size 50, №1 of size 1000, №5 of size 100

#### Scenario: Unpublished book
- **WHEN** anyone requests the places of a pending book
- **THEN** the response is 404 «book not found»

### Requirement: Collection badges on the book page
The public book page SHALL show the book's places in published collections, as returned by the book collection places API, under the book's authors.
- **Badge.** Each place is a badge «№{place} в „{title}“».
- **Link.** The badge links to `/top/{slug}` when the place is on the collection's first page of 50 places. Otherwise it links to `/top/{slug}?page={p}`, the page that shows the place.
- **More than three.** The first 3 badges SHALL be visible. When there are more, a button «Ещё {n}» reveals the remaining n badges and then disappears.
- **Nothing to show.** When the book has no places, or the places cannot be loaded, the page shows no badge area and no error. The rest of the book page MUST still render.
- **Access.** Badges MUST be shown to everyone, signed in or not.

#### Scenario: Book with one place
- **WHEN** a visitor opens the page of a book that is №3 in the published collection «Топ-100 книг всех времён»
- **THEN** the page shows the badge «№3 в „Топ-100 книг всех времён“» linking to that collection's `/top/{slug}`

#### Scenario: Place on a later page
- **WHEN** a book is №73 in a published collection
- **THEN** its badge links to `/top/{slug}?page=2`

#### Scenario: Five places
- **WHEN** a book holds places in 5 published collections
- **THEN** the page shows the 3 best badges and «Ещё 2», and pressing it shows all 5 badges

#### Scenario: Places fail to load
- **WHEN** the book collection places request fails
- **THEN** the book page renders without badges and without an error message

### Requirement: Verdict score
The metrics `book_beats_adaptation` and `adaptation_beats_book` SHALL score a book by readers' «Книга или экранизация?» votes on one of its adaptations, the source adaptation. The value of the metric is `book` («Книга лучше») for `book_beats_adaptation` and `adaptation` («Экранизация лучше») for `adaptation_beats_book`.
- **Counted votes.** A vote counts only when the adaptation link is visible, that is not hidden by an admin, and the book is published and in the rule's scope (the genre filter and the period). Votes on hidden links MUST NOT count, not even in the site share C.
- **Source adaptation.** The source adaptation is the book's visible adaptation with the most counted votes. When several have the same number of votes, the one listed first on the book page wins: the oldest year first, works without a year last, then by title, then by id.
- **Minimum.** The source adaptation MUST have at least the rule's minimum vote count of counted votes, or the book is not ranked. Other adaptations of the book are not tried instead. The default minimum is 5, the same as the verdict endpoint's `min_votes`.
- **Leader.** The metric's value MUST have strictly more votes on the source adaptation than each of the other two values («Книга лучше», «Экранизация лучше», «Одинаково»). Otherwise the book is not ranked.
- **Score.** The score is `(k + m·C) / (v + m)`, where:
  - v is the source adaptation's counted votes and k is how many of them have the metric's value;
  - C is the share of the metric's value among all counted votes in the rule's scope, including votes on non-source adaptations and on books below the minimum;
  - m is the rule's confidence threshold, 10 by default.
- **Accounts.** Only accounts that can sign in vote, the same invariant as for ratings.

#### Scenario: Most-voted adaptation is the source
- **WHEN** a `book_beats_adaptation` collection with m = 10 is computed. Its only voted book is «Война и мир». The 1967 adaptation has 40 votes: 36 «Книга лучше», 3 «Экранизация лучше», 1 «Одинаково». The 2016 adaptation has 10 votes: 5, 4 and 1.
- **THEN** C is 41/50 = 0.82. The source adaptation is the 1967 one, and the book's score is (36 + 8.2) / 50 = 0.884.

#### Scenario: Equal vote counts
- **WHEN** a book's adaptations from 1967 and 2016 each have 12 counted votes
- **THEN** the 1967 adaptation is the source

#### Scenario: Hidden adaptation does not count
- **WHEN** a book's hidden adaptation has 50 votes and its visible adaptation has 8
- **THEN** the visible adaptation is the source, and the 50 votes do not count in C

#### Scenario: Source below the minimum
- **WHEN** a book's most-voted visible adaptation has 4 counted votes and the rule's minimum is 5
- **THEN** the book is not ranked

#### Scenario: Value does not lead
- **WHEN** the source adaptation has 10 votes: 4 «Книга лучше», 4 «Экранизация лучше» and 2 «Одинаково»
- **THEN** the book is ranked in neither `book_beats_adaptation` nor `adaptation_beats_book`

#### Scenario: Period counts recent votes
- **WHEN** a `book_beats_adaptation` collection with a 30-day period is computed. One adaptation has 20 votes, all set more than 30 days ago. Another has 6 votes set within the period.
- **THEN** the adaptation with 6 votes is the source
