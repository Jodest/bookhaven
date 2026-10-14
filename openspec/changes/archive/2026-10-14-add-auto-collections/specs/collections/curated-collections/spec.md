## MODIFIED Requirements

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

### Requirement: Public collection index API
`GET /api/v1/collections` SHALL be open to everyone without a token and return `{items, total}` with every published collection, manual or automatic, newest first by the time it was first published. Each item MUST carry:
- `id`, `slug`, `title`, `description`, `kind` and `size`;
- `shown_count`: the number of public places, at most `size`;
- `source` and `source_url`, null when absent;
- `genre`: `{id, name, slug}` only when the linked genre is published, otherwise null;
- `cover_urls`: covers of up to the first 4 placed books that have one;
- `rule`: `{metric, period_days, min_ratings, confidence}` for an automatic collection, null for a manual one;
- `computed_at`: the time of the current snapshot of an automatic collection, null for a manual one or before the first computation.

Draft collections MUST NOT be listed.

#### Scenario: Draft collection is not listed
- **WHEN** a visitor requests the index while one collection is published and one is a draft
- **THEN** only the published collection is returned

#### Scenario: Automatic collection is listed
- **WHEN** a visitor requests the index while a published automatic collection exists
- **THEN** it is listed like a manual one, with its `rule` and `computed_at`

### Requirement: Public collection API
`GET /api/v1/collections/{slug}` SHALL be open to everyone without a token and return `{collection, items, total, page, limit}` for a published collection.
- `collection` has the index fields, and `total` equals `shown_count`.
- `items` are `{place, movement, book}` in place order.
- `book` carries `id`, `title`, `slug`, `cover_url`, `published_year`, `rating`, `ratings_count`, `authors` and published `genres`, in the same shape as books on a public list.
- `movement` is null for a manual collection and for an automatic collection without a movement baseline. Otherwise it is `{direction, by}`. `direction` is `up`, `down`, `same` or `new` (see «Position movement»). `by` is the number of positions moved, 0 for `same` and `new`.
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

### Requirement: Public collection pages
The web app SHALL provide two pages.
- `/top` («Подборки») lists published collections, manual and automatic alike, as cards with the title, up to 4 covers, «{shown_count} книг» and the source line. When there are none it shows «Подборок пока нет».
- `/top/{slug}` shows the title, the description, the source line («Источник: {source}», linked when a URL exists), the genre linked to the catalog filtered by it, and the ranked places. Each place shows the place number, the cover, the title linked to the book page, the authors (linked when published) and the rating.
- For an automatic collection, `/top/{slug}` SHALL also show «Составлена автоматически по {basis}{period}.», followed by «Обновлено {date}.» once it has been computed. {basis} is:
  - «оценкам читателей BookHaven» for `weighted_rating`;
  - «отметкам «Прочитано»» for `most_read`;
  - «отметкам «Хочу прочитать»» for `most_wanted`;
  - «активности читателей» for `trending`.

  {period} is « за последние {period_days} дн.» when the rule has a period. {date} is the day of `computed_at`, e.g. «14 октября 2026».
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

## ADDED Requirements

### Requirement: Automatic collection rule
An automatic collection SHALL rank published catalog books by the metric of its rule.
- **Genre filter.** When the collection links a genre, only books that have that genre are ranked.
- **Period.** The rule MAY have a period of 1-3650 days. A period limits the counted signals to those given within the last that many days before the computation. A rating counts by the time it was set or last changed; a shelf mark counts by the time the book was put on the shelf. Without a period, all signals count.
- **Metrics.** The v1 metrics are:
  - `weighted_rating`: the weighted rating described in «Weighted rating».
  - `most_read`: the number of readers who have the book on their «Прочитано» shelf.
  - `most_wanted`: the number of readers who have the book on their «Хочу прочитать» shelf.
  - `trending`: the number of ratings set or changed, plus the number of books put on a reading-status shelf («Хочу прочитать», «Читаю», «Прочитано»), within the period. Its period is 30 days when none is given.
- **Eligibility.** For `most_read`, `most_wanted` and `trending`, a book MUST have a score of at least 1 to be ranked.
- **Order and ties.** Books SHALL be ordered by score, highest first. Ties are broken by the book's total number of ratings (more first), then by title alphabetically, then by book id.
- **Signal sources.** Only ratings and reading-status shelves count. Custom shelves, readers' lists and review likes MUST NOT count. Pending and rejected books MUST NOT be ranked, and their signals MUST NOT count.

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
- **What a computation stores.** Each computation stores the ranked published books in score order. That is up to `size + 20` books, at most 1000, so that the books after the N-th form a reserve. The computation also stores each book's score and the computation time.
- **Places between computations.** Public places SHALL be derived from the snapshot exactly like the places of a manual collection from the admin's order. A book that is no longer published leaves its place and the following books shift up, and the reserve fills place N. A book published again gets its place back. New ratings and shelf marks MUST NOT change the places until the next computation.
- **Before the first computation.** An automatic collection that has never been computed has no places.

#### Scenario: Places are stable between computations
- **WHEN** readers rate books after the snapshot of a `weighted_rating` collection was computed
- **THEN** the public places stay the same until the next computation

#### Scenario: Unpublished between computations
- **WHEN** a book at place 2 of an automatic collection of size 3 is unpublished before the next computation
- **THEN** the former places 3 and 4 become places 2 and 3, and the next computation does not include the book

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
