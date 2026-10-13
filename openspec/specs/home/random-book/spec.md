# home/random-book Specification

## Purpose

Helps an undecided reader pick something to read with one click, optionally within a genre.

## Requirements

### Requirement: Random book endpoint
`GET /api/v1/home/random` SHALL be available to anyone and return `{book}` with one random published book from the first non-empty pool among: rating of at least 4.0 with at least 3 ratings; at least one rating; any published book. `genre_id` limits every pool to books of that genre and matches nothing when the genre is not published; `exclude_id` leaves one book out. `book` is `null` when nothing matches. A malformed `genre_id` or `exclude_id` MUST return 400. The result is never cached.

#### Scenario: Another option
- **WHEN** the only well-rated book of a genre is excluded with `exclude_id`
- **THEN** the pick falls back to the genre's books with at least one rating

#### Scenario: Unpublished genre
- **WHEN** a client asks for a random book in a genre that is not published
- **THEN** the response is `{"book": null}`

### Requirement: «Что почитать?» block
The home page SHALL offer «Что почитать?» («Подберём случайную книгу с хорошими оценками читателей.») with a genre selector («Жанр», «Любой жанр») and «Подобрать» («Подбираем…» while loading). A picked book shows its cover, title and authors linking to the book page, and «Ещё вариант» picks again without the book shown. It shows «Не нашли подходящую книгу.» when nothing matches and «Не удалось подобрать книгу. Попробуйте ещё раз.» on failure.

#### Scenario: Nothing in the genre
- **WHEN** a visitor picks a genre with no published books and clicks «Подобрать»
- **THEN** «Не нашли подходящую книгу.» is shown
