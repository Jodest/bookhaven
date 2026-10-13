# home/book-of-the-day Specification

## Purpose

Features one good book per day, the same for every visitor, and changes it every day.

## Requirements

### Requirement: Choosing the pool
The book of the day SHALL be picked from published books, using the first non-empty pool among: books with a cover, a rating of at least 4.0 and at least 3 ratings; books with a cover and at least one rating; books with a cover; any published book. When no book is published there is no book of the day.

#### Scenario: Few rated books
- **WHEN** no book with a cover has 3 ratings, but some books with a cover have one rating
- **THEN** the book of the day is one of the covered books with at least one rating

### Requirement: Daily pick
The pick SHALL be deterministic for a Moscow date and a pool: every visitor sees the same book all day. With an unchanged pool of at least two books, the book MUST differ from the previous day's book.

#### Scenario: Next day
- **WHEN** the pool has not changed since yesterday and holds at least two books
- **THEN** today's book of the day is not yesterday's

### Requirement: Book of the day card
The response SHALL carry the book card and a description excerpt of at most 300 characters, cut at a word boundary with «…» appended when cut, or `null` when the book has no description. The web card «Книга дня» shows the cover (or «Нет обложки»), the title, the authors and the publication year, the excerpt and «Подробнее», all linking to the book page.

#### Scenario: Long description
- **WHEN** the book of the day has a 1,000-character description
- **THEN** the excerpt ends with «…» and is at most 300 characters before it
