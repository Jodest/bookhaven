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
