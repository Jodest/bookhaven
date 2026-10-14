# home/catalog-rows Specification

## Purpose

Gives visitors three quick ways into the catalog: what is new, what readers rate highest, and what readers are writing about right now.

## Requirements

### Requirement: New books
«Новое в каталоге» SHALL show up to 10 published books in the order defined by `catalog-moderation/entity-status` (first-publication time, newest first). Its link «Весь каталог» opens `/books?sort=new`, which sorts by publication year instead (see `catalog-moderation/entity-status`).

#### Scenario: Row size
- **WHEN** 25 books are published
- **THEN** «Новое в каталоге» shows 10 of them

### Requirement: Top rated
«Высоко оценённые» SHALL show up to 10 published books with a rating of at least 4.0 and at least 3 ratings, by rating, then by number of ratings. Its link «Смотреть все» opens `/books?sort=rating`.

#### Scenario: Too few ratings
- **WHEN** a published book has a 5.0 rating from two readers
- **THEN** it is not in «Высоко оценённые»

### Requirement: Recent reviews
«Свежие рецензии» SHALL show the 3 newest visible reviews of at least 40 characters on published books, at most one per book (the newest). Each shows an excerpt of at most 280 characters cut at a word boundary with «…» when cut, the rating as stars, the review author's name (or «Аноним») and the book with its cover and author names. The API also returns the likes count and author slugs (`null` for a non-published author); the web block does not show them. «Читать полностью» and the book links open the book page.

#### Scenario: Two reviews of the same book
- **WHEN** the two newest long reviews are both on one book
- **THEN** only the newer of them is shown and the next slot goes to another book

#### Scenario: Short review
- **WHEN** the newest review says «ок, норм»
- **THEN** it is not shown

### Requirement: Curated collections row
«Подборки» SHALL show BookHaven's curated collections (see `collections/curated-collections`).
- **Which collections.** Take the 6 published collections that were first published most recently, in the order of the public collection index. Keep those with at least one public place, and show the first 3 of them.
- **Cards.** The cards are the same as on `/top`: the title, up to 4 covers, «{shown_count} книг» and the source line. Each card links to `/top/{slug}`.
- **Link.** The block has the link «Все подборки» to `/top`.
- **Hidden block.** The block MUST render nothing when no collection qualifies or when the collections cannot be loaded.
- **Freshness.** It is not part of the home payload's selection cache: a collection that is published or unpublished shows up or disappears on the next page load.
- No admin setting chooses the collections.

#### Scenario: Three newest collections
- **WHEN** 5 published collections with places exist
- **THEN** «Подборки» shows the 3 most recently first-published ones and the link «Все подборки»

#### Scenario: Empty collection skipped
- **WHEN** the most recently published collection is automatic and has no places yet, and two older published collections have places
- **THEN** «Подборки» shows the two older collections only

#### Scenario: No published collections
- **WHEN** no collection is published
- **THEN** the home page has no «Подборки» block
