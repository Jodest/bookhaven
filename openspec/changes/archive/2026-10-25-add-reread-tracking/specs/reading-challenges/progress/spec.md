## MODIFIED Requirements

### Requirement: Counting read books
The books read in a year SHALL be the distinct books with at least one finished reading by the caller (see `collections/reading-journal`) whose finish time falls within the calendar year (UTC). The rules are:
- A book finished twice in one year counts once for that year.
- A book finished again in a later year counts in each year it was finished.
- The count is computed live on every request, whatever the book's current shelf. Moving a book from «Прочитано» to another status keeps its finished readings, so it keeps counting.
- Removing a book from «Прочитано» undoes only its latest finished reading. The year of that reading stops counting the book, unless another finished reading of the book falls in the same year.
- Books are counted whatever their publication status.

#### Scenario: Book moved back to «Читаю»
- **WHEN** a reader moves a book finished this year from «Прочитано» back to «Читаю» to re-read it
- **THEN** the year's count is unchanged

#### Scenario: Shelved last year
- **WHEN** a book was finished, by adding it to «Прочитано», on 31 December of last year (UTC)
- **THEN** it counts for last year, not this year

#### Scenario: Re-read in a new year
- **WHEN** a reader finished a book in 2025 and finishes a re-read of it in 2026
- **THEN** the book counts once for 2025 and once for 2026

#### Scenario: Read twice in one year
- **WHEN** a reader finishes a book twice in 2026
- **THEN** it counts once for 2026

#### Scenario: Undoing «Прочитано»
- **WHEN** a reader who finished a book only once, this year, removes it from «Прочитано»
- **THEN** the year's count goes down by one

#### Scenario: Undoing a re-read from an earlier year
- **WHEN** a reader who finished a book in 2025 and again in 2026 removes it from «Прочитано» in 2026
- **THEN** the 2026 count goes down by one and the 2025 count is unchanged
