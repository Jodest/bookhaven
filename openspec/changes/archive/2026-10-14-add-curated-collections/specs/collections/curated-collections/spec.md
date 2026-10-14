## Purpose

Lets anyone browse ranked book collections curated by BookHaven itself («подборки», e.g. «Топ-100 книг всех времён») — distinct from readers' own lists — with places computed from the admin's order and the books' publication status.

## ADDED Requirements

### Requirement: Curated collection
A curated collection SHALL belong to BookHaven, not to a user, and MUST have a title (2-255 characters), a unique slug (lowercase Latin letters, digits and single hyphens, at most 100 characters), a description (may be empty), a size N (1-1000 ranked places), a visibility (draft or published, draft by default) and a kind. Optionally it MAY name a source (free text up to 255 characters, e.g. «по версии BBC Big Read, 2003») with an `http`/`https` URL, and link one catalog genre. Every collection created now SHALL have the kind `manual`; other kinds are reserved for later and MUST NOT be creatable.

#### Scenario: New collection is a draft
- **WHEN** an admin creates a collection without choosing its visibility
- **THEN** it is a draft and does not appear publicly

#### Scenario: Kind of a new collection
- **WHEN** any collection is created
- **THEN** its kind is `manual`

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
`GET /api/v1/collections` SHALL be open to everyone without a token and return `{items, total}` with every published collection, newest first by the time it was first published. Each item MUST carry `id`, `slug`, `title`, `description`, `kind`, `size`, `shown_count` (the number of public places, at most `size`), `source` and `source_url` (null when absent), `genre` (`{id, name, slug}` only when the linked genre is published, otherwise null) and `cover_urls` (covers of up to the first 4 placed books that have one). Draft collections MUST NOT be listed.

#### Scenario: Draft collection is not listed
- **WHEN** a visitor requests the index while one collection is published and one is a draft
- **THEN** only the published collection is returned

### Requirement: Public collection API
`GET /api/v1/collections/{slug}` SHALL be open to everyone without a token and return `{collection, items, total, page, limit}` for a published collection, where `collection` has the index fields, `total` equals `shown_count`, and `items` are `{place, book}` in place order with `book` carrying `id`, `title`, `slug`, `cover_url`, `published_year`, `rating`, `ratings_count`, `authors` and published `genres`, in the same shape as books on a public list. `page` defaults to 1 and `limit` to 50 (at most 100); places keep their global numbers across pages. An unknown slug or a draft collection MUST return 404 «collection not found», for admins too.

#### Scenario: Second page keeps numbering
- **WHEN** a visitor requests page 2 with `limit=50` of a collection with 100 places
- **THEN** the items are places 51..100

#### Scenario: Draft collection link
- **WHEN** anyone requests the slug of a draft collection
- **THEN** the response is 404

### Requirement: Public collection pages
The web app SHALL provide `/top` («Подборки») listing published collections as cards with the title, up to 4 covers, «{shown_count} книг» and the source line, or «Подборок пока нет» when there are none; and `/top/{slug}` showing the title, description, the source line («Источник: {source}», linked when a URL exists), the genre linked to the catalog filtered by it, and the ranked places with the place number, cover, title linked to the book page, authors (linked when published) and rating. An unknown or draft collection shows «Подборка не найдена». Both pages MUST open for everyone, signed in or not. The site header SHALL show a «Подборки» link to `/top` to every visitor.

#### Scenario: Anonymous visitor opens a collection
- **WHEN** a visitor without session cookies opens `/top/{slug}` of a published collection
- **THEN** the ranked places are shown without a redirect to login

#### Scenario: Header link
- **WHEN** an anonymous visitor looks at the site header
- **THEN** it has a «Подборки» link to `/top`
