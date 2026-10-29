## MODIFIED Requirements

### Requirement: Open Library enrichment fills gaps only
When a work lacks a cover, description, page count or ISBN, the job SHALL try Open Library (by ISBN, then by title and first author) and fill only the fields Wikidata left empty. Wikidata values MUST NOT be overwritten. A non-Russian Open Library description MUST be dropped silently instead of failing the book.

A result of the title-and-author search MUST be used only when it really is the same work. The job checks the first 5 results in Open Library's order and uses the first one that passes both rules below. Titles and names are compared after normalization: lower case, «ё» read as «е», every character that is not a letter, digit or space removed, runs of spaces collapsed.
- **Title.** The result's title equals the work's Russian label or its English Wikidata label. A result title followed by a subtitle also matches: the part before the first «:», «(», « — » or « - » is compared instead. An empty normalized title never matches.
- **Author.** The last word of one of the result's author names equals the last word of the Russian or English Wikidata label of one of the work's authors. A result without author names never matches.

When no result passes, or when the search answer reports more than 10 000 matching books (Open Library ignored the query), the job MUST take nothing from the search: no cover, description, ISBN or page count. The work is still created from its Wikidata data. The English labels are used only for this check and are not stored.

#### Scenario: English excerpt
- **WHEN** Open Library returns an English description for a work
- **THEN** the book is created without a description

#### Scenario: Unrelated search result
- **WHEN** the title search for «История Петра I» by «Александр Сергеевич Пушкин» returns "I, Robot" by Isaac Asimov first
- **THEN** the book is created without a cover, description or ISBN from Open Library

#### Scenario: Same author, other work
- **WHEN** the title search for «Капитанская дочка» returns only a different Pushkin work
- **THEN** that result is not used and the book gets no cover from Open Library

#### Scenario: Match by English label
- **WHEN** a work labelled «Белый Клык» in Russian and "White Fang" in English, by «Джек Лондон» / "Jack London", gets a second search result titled "White Fang" by "Jack London" and an unrelated first result
- **THEN** the cover of the second result is used

#### Scenario: Title with a subtitle
- **WHEN** the search result is titled «Зов предков: повесть» by «Джек Лондон» for the work «Зов предков»
- **THEN** the result is used

#### Scenario: Query ignored by Open Library
- **WHEN** the search answer reports 907 849 matching books
- **THEN** no result of that answer is used, even one whose title and author match

## ADDED Requirements

### Requirement: Audit of covers from the title search
The system SHALL provide a maintenance command, run from the command line by an operator with database access, that finds harvested books whose cover link may have come from a wrong Open Library title search result. It has no HTTP endpoint and no admin UI. The candidates are books with Wikidata as their source whose cover link is an Open Library cover-ID link. For each candidate the command checks:
- `shared_cover`: another book has a link to the same Open Library cover ID;
- `no_match`: the title search for the book's title and author names, checked with the rules of the Open Library enrichment requirement (Russian title and names only), finds no matching result;
- `other_match`: that check finds a matching result with a different cover.

By default the command MUST change nothing. It prints every candidate with at least one reason: book ID, status, title, authors, cover link, ISBN, whether it has a description, and its reasons. It also prints the number of candidates that passed every check. Only when run with an explicit apply option and an explicit list of book IDs does it clear the cover link and the ISBN of exactly those books. It MUST NOT change any book outside that list and MUST NOT change descriptions or any other field. Clearing MUST go through the normal book save, so the stored cover copy follows `catalog/book-covers`. An ID that is unknown, or is not a candidate, MUST be reported and skipped. An Open Library failure for one book MUST be reported for that book and MUST NOT stop the run.

#### Scenario: Report only
- **WHEN** an operator runs the audit without the apply option while «Белый Клык» and «Зов предков» share the cover ID `9157583`
- **THEN** both books are listed with the reason `shared_cover` and no book changes

#### Scenario: Clearing the reviewed books
- **WHEN** an operator runs the audit with the apply option and the ID of «Капитанская дочка»
- **THEN** only that book loses its cover link and ISBN, and it shows «Нет обложки» on the site

#### Scenario: ID that is not a candidate
- **WHEN** the apply list contains the ID of a book whose cover link is not an Open Library cover-ID link
- **THEN** that ID is reported as skipped and the book does not change

#### Scenario: Open Library down during the audit
- **WHEN** Open Library times out for one candidate
- **THEN** that candidate is reported with the error, and the other candidates are still checked
