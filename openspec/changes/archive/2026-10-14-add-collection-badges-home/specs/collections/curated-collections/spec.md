## MODIFIED Requirements

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

## ADDED Requirements

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
