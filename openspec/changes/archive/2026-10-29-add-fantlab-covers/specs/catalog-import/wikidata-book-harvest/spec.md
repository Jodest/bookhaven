## MODIFIED Requirements

### Requirement: Open Library enrichment fills gaps only
When a work lacks a cover, description, page count or ISBN, the job SHALL try Open Library and fill only the fields Wikidata left empty. It tries, in this order: by ISBN, then by the work's Open Library IDs (cover only), then by the work's FantLab work IDs (cover only, see "Cover from a FantLab edition"), then by title and first author. Wikidata values MUST NOT be overwritten. A non-Russian Open Library description MUST be dropped silently instead of failing the book.

**Cover by Open Library ID.** When the work still has no cover after the ISBN step, the job SHALL use the Open Library IDs that Wikidata lists for the work (property P648):
- Only work IDs (`OL<digits>W`) and edition IDs (`OL<digits>M`) are used. Other values are ignored.
- Duplicates are removed. Work IDs are tried before edition IDs, each kind in ascending numeric order. At most 3 IDs are tried per work.
- For each ID the job reads the Open Library work or edition record and takes the first cover ID in its cover list that is greater than zero. Zero and negative cover IDs are skipped.
- The first ID that gives a cover ID wins, and no further IDs are requested. The cover link is `https://covers.openlibrary.org/b/id/<cover ID>-L.jpg`.
- An ID whose record is missing, has no usable cover ID, or whose request fails moves on to the next ID. Such failures MUST NOT fail the book.
- This step MUST fill only the cover. Description, ISBN, page count and title are never taken from these records.
- A cover found this way is used even when it belongs to an edition in another language. The book shows no marker for it.

The title search still runs as before when the ISBN step found nothing. A cover from the title search is used only when the work still has no cover after the Open Library ID and FantLab steps.

A result of the title-and-author search MUST be used only when it really is the same work. The job checks the first 5 results in Open Library's order and uses the first one that passes both rules below. Titles and names are compared after normalization: lower case, «ё» read as «е», every character that is not a letter, digit or space removed, runs of spaces collapsed.
- **Title.** The result's title equals the work's Russian label or its English Wikidata label. A result title followed by a subtitle also matches: the part before the first «:», «(», « — » or « - » is compared instead. An empty normalized title never matches.
- **Author.** The last word of one of the result's author names equals the last word of the Russian or English Wikidata label of one of the work's authors. A result without author names never matches.

When no result passes, or when the search answer reports more than 10 000 matching books (Open Library ignored the query), the job MUST take nothing from the search: no cover, description, ISBN or page count. The work is still created from its Wikidata data. The English labels, the Open Library IDs and the FantLab work IDs are used only for these lookups and are not stored.

#### Scenario: English excerpt
- **WHEN** Open Library returns an English description for a work
- **THEN** the book is created without a description

#### Scenario: Unrelated search result
- **WHEN** the title search for «История Петра I» by «Александр Сергеевич Пушкин» returns "I, Robot" by Isaac Asimov first, and the work has neither an Open Library ID nor a FantLab work ID in Wikidata
- **THEN** the book is created without a cover, description or ISBN from Open Library

#### Scenario: Same author, other work
- **WHEN** the title search for «Капитанская дочка» returns only a different Pushkin work, and the work has neither an Open Library ID nor a FantLab work ID in Wikidata
- **THEN** that result is not used and the book gets no cover from Open Library

#### Scenario: Match by English label
- **WHEN** a work labelled «Белый Клык» in Russian and "White Fang" in English, by «Джек Лондон» / "Jack London", has neither an Open Library ID nor a FantLab work ID in Wikidata, and its second search result is titled "White Fang" by "Jack London" while the first result is unrelated
- **THEN** the cover of the second result is used

#### Scenario: Title with a subtitle
- **WHEN** the search result is titled «Зов предков: повесть» by «Джек Лондон» for the work «Зов предков»
- **THEN** the result is used

#### Scenario: Query ignored by Open Library
- **WHEN** the search answer reports 907 849 matching books
- **THEN** no result of that answer is used, even one whose title and author match

#### Scenario: Cover from the Open Library work ID
- **WHEN** a work without an ISBN has the Open Library ID `OL27479W` in Wikidata, and that work's cover list is `[-1, 8406786]`
- **THEN** the book is created with the cover link `https://covers.openlibrary.org/b/id/8406786-L.jpg`, and neither a FantLab cover nor a cover from the title search is used

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
- **WHEN** a work's only Open Library ID has an empty cover list, the work has no FantLab work ID, and the title search finds a matching result with a cover
- **THEN** the cover of the matching title-search result is used

#### Scenario: Description not taken from the ID record
- **WHEN** a work gets its cover through its Open Library work ID and that work record has a Russian description
- **THEN** the description is not taken from that record, and the title search may still fill it

#### Scenario: Open Library down for the ID lookup
- **WHEN** reading the record behind a work's Open Library ID times out, and the work has no FantLab work ID
- **THEN** the book is still created, and the title search is still tried for its cover

#### Scenario: FantLab cover before the title search
- **WHEN** a work has no Open Library cover by ID, gets a FantLab cover, and the title search also finds a matching result with a cover and a Russian description
- **THEN** the book gets the FantLab cover link and the description from the title search, and the title-search cover is not used

### Requirement: Cover backfill by Open Library ID
The system SHALL provide a maintenance command that gives existing harvested books a cover through their Open Library IDs and, when those give none, through their FantLab work IDs. An operator with database access runs it from the command line. It has no HTTP endpoint and no admin UI.

The candidates are books that:
- have Wikidata as their source;
- have a stored Wikidata QID matching `^Q[0-9]+$`;
- have no cover link;
- are not `rejected`.

The command reads the candidates' Open Library IDs (P648) and FantLab work IDs (P7439) from Wikidata in batches of at most 50 QIDs. To each candidate with Open Library IDs it applies the rule of "Cover by Open Library ID" in the Open Library enrichment requirement. To each candidate that got no cover that way (no Open Library IDs, no usable cover, or an Open Library error) and that has FantLab work IDs, it applies the rule of "Cover from a FantLab edition", with the book's stored title as its Russian title. FantLab requests follow the pacing of that requirement. The per-job request cap does not apply, because the command is not a harvest job.

By default the command MUST change nothing. It prints:
- each candidate that got a cover: book ID, status, title, QID, the source ID used (the Open Library ID, or `fantlab:<work ID>/<edition ID>`) and the cover link;
- each candidate that has IDs but got no cover: book ID, title, QID and its IDs;
- the totals of candidates, candidates with Open Library IDs, candidates with FantLab work IDs, covers found from Open Library, covers found from FantLab, and errors.

Only with an explicit apply option does it write. It then sets the cover link of every candidate that got a cover. When an explicit list of book IDs is also given, it writes only those books. A listed ID that is unknown or is not a candidate MUST be reported and skipped.

Writing MUST go through the normal book save, so the cover copy follows `catalog/book-covers` (a FantLab link becomes `linked` and is never downloaded). Each write MUST re-read the book first and skip it when it has a cover link by then. Only the cover link changes; the ISBN, description, status and every other field stay as they are. Each write reports `updated`, `skipped: …` or `error: …`.

A Wikidata failure for a batch MUST be reported for the books of that batch. An Open Library or FantLab failure for one book MUST be reported for that book. Neither stops the run. The command never clears or deletes anything. A second run therefore finds only the books that are still without a cover.

#### Scenario: Backfill report only
- **WHEN** an operator runs the backfill without the apply option, and a pending book from Wikidata without a cover has the Open Library ID `OL27479W` whose work has covers
- **THEN** that book is listed with `OL27479W` and its cover link, and no book changes

#### Scenario: Backfill applied
- **WHEN** an operator runs the backfill with the apply option
- **THEN** every listed book gets its cover link through the normal save, the cover copy follows `catalog/book-covers` for it, and no other field of those books changes

#### Scenario: Cover set in the meantime
- **WHEN** an admin set a cover link on a candidate after the report was built and before the write
- **THEN** that book is reported as skipped and keeps the admin's cover

#### Scenario: Rejected book
- **WHEN** a rejected book from Wikidata has no cover link but has an Open Library ID with a cover
- **THEN** it is not a candidate and does not change

#### Scenario: Wikidata down for one batch
- **WHEN** Wikidata fails for one batch of 50 QIDs
- **THEN** those 50 books are reported with the error, and the other batches are still processed

#### Scenario: Backfill from FantLab
- **WHEN** an operator runs the backfill without the apply option, and a book «Пикник на обочине» without a cover has no Open Library ID but has the FantLab work ID `7193`, whose Russian paper edition `16469` is named «Пикник на обочине» and has an image
- **THEN** the book is listed with `fantlab:7193/16469` and the cover link `https://fantlab.ru/images/editions/big/16469`, and no book changes

#### Scenario: Open Library cover wins in the backfill
- **WHEN** a candidate has both an Open Library ID with a cover and a FantLab work ID
- **THEN** the Open Library cover is listed and FantLab is not requested for that book

#### Scenario: FantLab down for one book
- **WHEN** FantLab answers 503 for one candidate's work
- **THEN** that candidate is reported with the error, and the other candidates are still checked

## ADDED Requirements

### Requirement: Cover from a FantLab edition
When a work still has no cover after the Open Library ID step, the job SHALL try the FantLab work IDs that Wikidata lists for the work (property P7439). The rule is strict: no cover is better than a wrong one.
- **IDs.** The numeric FantLab work ID is taken from each value, which can be written as `work<digits>` or `<digits>`. Other values are ignored. Duplicates are removed, and the IDs are tried in ascending numeric order. At most 2 IDs are tried per work.
- **Work.** For each ID the job reads the FantLab work with its list of editions. A work whose normalized FantLab name is not equal to the normalized Russian title of the book is not used.
- **Editions.** Only paper editions with the language `ru` are considered. Audio editions, magazine publications and planned editions are ignored. An edition qualifies only when its normalized name equals the normalized Russian title of the book. Normalization is the same as for the title search, but subtitles are never cut, so a collection under another name never qualifies.
- **Choice.** Qualifying editions are tried newest year first. Editions without a year come last, and ties go to the lower edition ID. At most 3 editions are checked per work. The first edition that FantLab reports as having an image wins. Its cover link is `https://fantlab.ru/images/editions/big/<edition ID>`. The image itself is never requested by the harvest.
- **Failures.** A missing work, a work without a qualifying edition with an image, or a failed request moves on to the next ID. None of these fails the book.
- **Scope.** This step MUST fill only the cover. Description, ISBN, page count, year and title are never taken from FantLab.
- **Pacing.** Requests to FantLab MUST be at least 1.2 seconds apart. One work makes at most 4 FantLab requests. One harvest job makes at most 250 FantLab requests. After that, the remaining works of that job skip this step, and the backfill can cover them later.

#### Scenario: Self-titled Russian edition
- **WHEN** a work «Пикник на обочине» without an Open Library cover has the FantLab work ID `7193`, and FantLab lists the Russian paper edition `16469` named «Пикник на обочине» with an image
- **THEN** the book is created with the cover link `https://fantlab.ru/images/editions/big/16469`

#### Scenario: Only a collection edition
- **WHEN** the only Russian paper editions of a work «Улитка на склоне» are named «Улитка на склоне. Повести» and «Избранное»
- **THEN** the book gets no cover from FantLab

#### Scenario: Wrong FantLab work
- **WHEN** a work «Трудно быть богом» has a FantLab work ID whose FantLab work is named «Понедельник начинается в субботу»
- **THEN** none of that work's editions is used

#### Scenario: Newest edition with an image
- **WHEN** three Russian paper editions named like the work exist, from 2015 without an image, from 2008 with an image, and from 1990 with an image
- **THEN** the 2015 edition is checked first, and the cover of the 2008 edition is used

#### Scenario: Audio and foreign editions ignored
- **WHEN** the only editions named like the work are an audiobook and an English paper edition
- **THEN** the book gets no cover from FantLab

#### Scenario: FantLab unavailable
- **WHEN** FantLab times out for a work's FantLab ID
- **THEN** the book is still created, and the title search is still tried for its cover

#### Scenario: Request cap reached
- **WHEN** a harvest job has already made 250 FantLab requests
- **THEN** the job's remaining works are created without a FantLab lookup

#### Scenario: Open Library cover by ID wins
- **WHEN** a work gets a cover through its Open Library ID and also has a FantLab work ID
- **THEN** no FantLab request is made for that work
