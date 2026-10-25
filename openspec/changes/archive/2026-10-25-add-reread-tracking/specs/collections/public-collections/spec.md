## ADDED Requirements

### Requirement: Read counter on public shelves
The public shelf page `/shelves/public/{id}` SHALL show «Прочитано <N> <раз|раза|раз>» under each book the shelf owner finished at least once, using the `reads_count` of the public shelf endpoint and the same plural rules as the book page. Books the owner never finished show no counter. The counter is shown only through a public shelf. A private shelf MUST keep returning 404, so its books' reading history is never exposed. The public profile lists shelves without books and shows no counter.

#### Scenario: Visitor sees a re-read
- **WHEN** a visitor who is not signed in opens a reader's public «Прочитано» shelf holding a book that reader finished three times
- **THEN** the book shows «Прочитано 3 раза»

#### Scenario: Private shelf stays hidden
- **WHEN** anyone opens `/shelves/public/{id}` of a private custom shelf holding re-read books
- **THEN** «Полка не найдена» is shown and no read counts are returned
