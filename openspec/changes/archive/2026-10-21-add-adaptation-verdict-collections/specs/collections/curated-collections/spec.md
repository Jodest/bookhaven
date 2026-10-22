## ADDED Requirements

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

## MODIFIED Requirements

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
