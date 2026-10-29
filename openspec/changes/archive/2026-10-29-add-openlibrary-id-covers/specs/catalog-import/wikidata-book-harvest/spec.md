## MODIFIED Requirements

### Requirement: Open Library enrichment fills gaps only
When a work lacks a cover, description, page count or ISBN, the job SHALL try Open Library and fill only the fields Wikidata left empty. It tries, in this order: by ISBN, then by the work's Open Library IDs (cover only), then by title and first author. Wikidata values MUST NOT be overwritten. A non-Russian Open Library description MUST be dropped silently instead of failing the book.

**Cover by Open Library ID.** When the work still has no cover after the ISBN step, the job SHALL use the Open Library IDs that Wikidata lists for the work (property P648):
- Only work IDs (`OL<digits>W`) and edition IDs (`OL<digits>M`) are used. Other values are ignored.
- Duplicates are removed. Work IDs are tried before edition IDs, each kind in ascending numeric order. At most 3 IDs are tried per work.
- For each ID the job reads the Open Library work or edition record and takes the first cover ID in its cover list that is greater than zero. Zero and negative cover IDs are skipped.
- The first ID that gives a cover ID wins, and no further IDs are requested. The cover link is `https://covers.openlibrary.org/b/id/<cover ID>-L.jpg`.
- An ID whose record is missing, has no usable cover ID, or whose request fails moves on to the next ID. Such failures MUST NOT fail the book.
- This step MUST fill only the cover. Description, ISBN, page count and title are never taken from these records.
- A cover found this way is used even when it belongs to an edition in another language. The book shows no marker for it.

The title search still runs as before when the ISBN step found nothing. A cover from the title search is used only when the work still has no cover.

A result of the title-and-author search MUST be used only when it really is the same work. The job checks the first 5 results in Open Library's order and uses the first one that passes both rules below. Titles and names are compared after normalization: lower case, «ё» read as «е», every character that is not a letter, digit or space removed, runs of spaces collapsed.
- **Title.** The result's title equals the work's Russian label or its English Wikidata label. A result title followed by a subtitle also matches: the part before the first «:», «(», « — » or « - » is compared instead. An empty normalized title never matches.
- **Author.** The last word of one of the result's author names equals the last word of the Russian or English Wikidata label of one of the work's authors. A result without author names never matches.

When no result passes, or when the search answer reports more than 10 000 matching books (Open Library ignored the query), the job MUST take nothing from the search: no cover, description, ISBN or page count. The work is still created from its Wikidata data. The English labels and the Open Library IDs are used only for these lookups and are not stored.

#### Scenario: English excerpt
- **WHEN** Open Library returns an English description for a work
- **THEN** the book is created without a description

#### Scenario: Unrelated search result
- **WHEN** the title search for «История Петра I» by «Александр Сергеевич Пушкин» returns "I, Robot" by Isaac Asimov first, and the work has no Open Library ID in Wikidata
- **THEN** the book is created without a cover, description or ISBN from Open Library

#### Scenario: Same author, other work
- **WHEN** the title search for «Капитанская дочка» returns only a different Pushkin work, and the work has no Open Library ID in Wikidata
- **THEN** that result is not used and the book gets no cover from Open Library

#### Scenario: Match by English label
- **WHEN** a work labelled «Белый Клык» in Russian and "White Fang" in English, by «Джек Лондон» / "Jack London", has no Open Library ID in Wikidata and gets a second search result titled "White Fang" by "Jack London" and an unrelated first result
- **THEN** the cover of the second result is used

#### Scenario: Title with a subtitle
- **WHEN** the search result is titled «Зов предков: повесть» by «Джек Лондон» for the work «Зов предков»
- **THEN** the result is used

#### Scenario: Query ignored by Open Library
- **WHEN** the search answer reports 907 849 matching books
- **THEN** no result of that answer is used, even one whose title and author match

#### Scenario: Cover from the Open Library work ID
- **WHEN** a work without an ISBN has the Open Library ID `OL27479W` in Wikidata, and that work's cover list is `[-1, 8406786]`
- **THEN** the book is created with the cover link `https://covers.openlibrary.org/b/id/8406786-L.jpg`, and a cover from the title search is not used

#### Scenario: Several Open Library IDs
- **WHEN** a work has the Open Library IDs `OL27516W`, `OL27479W` and `OL27455W`, the work `OL27455W` has no covers, and `OL27479W` has covers
- **THEN** `OL27455W` is read first, then `OL27479W`, its first positive cover ID is used, and `OL27516W` is not requested

#### Scenario: Edition ID only
- **WHEN** a work's only Open Library ID is the edition ID `OL7353617M` and that edition has a cover
- **THEN** the book gets the cover of that edition

#### Scenario: English edition cover
- **WHEN** the only cover found by a work's Open Library ID is from an English edition
- **THEN** the book is created with that cover and no marker

#### Scenario: Open Library ID without a cover
- **WHEN** a work's only Open Library ID has an empty cover list and the title search finds a matching result with a cover
- **THEN** the cover of the matching title-search result is used

#### Scenario: Description not taken from the ID record
- **WHEN** a work gets its cover through its Open Library work ID and that work record has a Russian description
- **THEN** the description is not taken from that record, and the title search may still fill it

#### Scenario: Open Library down for the ID lookup
- **WHEN** reading the record behind a work's Open Library ID times out
- **THEN** the book is still created, and the title search is still tried for its cover

## ADDED Requirements

### Requirement: Cover backfill by Open Library ID
The system SHALL provide a maintenance command that gives existing harvested books a cover through their Open Library IDs. An operator with database access runs it from the command line. It has no HTTP endpoint and no admin UI.

The candidates are books that:
- have Wikidata as their source;
- have a stored Wikidata QID matching `^Q[0-9]+$`;
- have no cover link;
- are not `rejected`.

The command reads the candidates' Open Library IDs (P648) from Wikidata in batches of at most 50 QIDs. To each candidate with IDs it applies the rule of "Cover by Open Library ID" in the Open Library enrichment requirement.

By default the command MUST change nothing. It prints:
- each candidate that got a cover: book ID, status, title, QID, the Open Library ID used and the cover link;
- each candidate that has IDs but got no cover: book ID, title, QID and its IDs;
- the totals of candidates, candidates with IDs, covers found and errors.

Only with an explicit apply option does it write. It then sets the cover link of every candidate that got a cover. When an explicit list of book IDs is also given, it writes only those books. A listed ID that is unknown or is not a candidate MUST be reported and skipped.

Writing MUST go through the normal book save, so a cover copy is queued as `catalog/book-covers` requires. Each write MUST re-read the book first and skip it when it has a cover link by then. Only the cover link changes; the ISBN, description, status and every other field stay as they are. Each write reports `updated`, `skipped: …` or `error: …`.

A Wikidata failure for a batch MUST be reported for the books of that batch. An Open Library failure for one book MUST be reported for that book. Neither stops the run. The command never clears or deletes anything. A second run therefore finds only the books that are still without a cover.

#### Scenario: Backfill report only
- **WHEN** an operator runs the backfill without the apply option, and a pending book from Wikidata without a cover has the Open Library ID `OL27479W` whose work has covers
- **THEN** that book is listed with `OL27479W` and its cover link, and no book changes

#### Scenario: Backfill applied
- **WHEN** an operator runs the backfill with the apply option
- **THEN** every listed book gets its cover link through the normal save and a cover copy is queued for it, and no other field of those books changes

#### Scenario: Cover set in the meantime
- **WHEN** an admin set a cover link on a candidate after the report was built and before the write
- **THEN** that book is reported as skipped and keeps the admin's cover

#### Scenario: Rejected book
- **WHEN** a rejected book from Wikidata has no cover link but has an Open Library ID with a cover
- **THEN** it is not a candidate and does not change

#### Scenario: Wikidata down for one batch
- **WHEN** Wikidata fails for one batch of 50 QIDs
- **THEN** those 50 books are reported with the error, and the other batches are still processed
