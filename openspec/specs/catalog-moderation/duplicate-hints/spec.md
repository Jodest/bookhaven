# catalog-moderation/duplicate-hints Specification

## Purpose

Points a moderator at entities that are probably the same book, person or genre as a queue item, so duplicates can be merged or rejected instead of published. Hints are display-only: they are recomputed on every queue request and never stored or acted on automatically.

## Requirements

### Requirement: Hints are display-only
Each queue item SHALL carry `similar`, a list of at most 5 hints `{<entity>, match}`, never null. Hints MUST be computed on every queue request, MUST NOT be stored and MUST NOT change any entity. An item never hints at itself. Candidates include entities outside the current page and the other items on the page, so two duplicates on one page hint at each other.

#### Scenario: Two duplicates on one page
- **WHEN** two pending authors «Лев Толстой» and «Толстой, Лев» are on the same queue page
- **THEN** each has the other as a `surname_initial` hint

### Requirement: Name normalization
Names and titles SHALL be compared after normalization: lowercase, «ё» folded to «е», every character that is not a letter or digit turned into a space, and spaces collapsed.

#### Scenario: Punctuation and ё
- **WHEN** one genre is «Фэнтези.» and another «фэнтези»
- **THEN** they match as `exact_name`

### Requirement: Author hints
An author's surname SHALL be the part before a comma («Толстой, Лев») or else the last word; surnames shorter than 3 letters produce no hints. A candidate MUST share the surname, and the match is `exact_name` (same normalized name), `surname_initial` (same first letter of the first given name) or `surname`. Rejected authors MUST NOT be hints. Hints are ordered by match strength (`exact_name`, `surname_initial`, `surname`), then published before pending, then name. The admin labels them «то же имя», «фамилия и инициал» and «фамилия», shows the hint's status and Wikidata link, warns «разные QID — вероятно, тёзки» when both have different QIDs, and offers «Это он — слить» (see `catalog-moderation/author-merge`).

#### Scenario: Surname and initial
- **WHEN** a pending author is «Л. Толстой» and a published author is «Лев Толстой»
- **THEN** the published author is a `surname_initial` hint

#### Scenario: Namesakes with different QIDs
- **WHEN** an author and its hint have different Wikidata QIDs
- **THEN** the hint shows «разные QID — вероятно, тёзки»

### Requirement: Genre hints
Genre names SHALL be compared word by word after removing a common Russian ending from each word, from a fixed list, only when at least 4 letters remain. The match is `exact_name` (same normalized name), `same_stem` (all word stems equal) or `shared_word` (a common stem of at least 5 letters). Rejected and merged genres MUST NOT be hints. Hints are ordered by match strength, then published first, then name. The admin labels them «то же название», «та же основа» and «общее слово», warns «разные QID — возможно, разные понятия» when the QIDs differ, and offers «Это он — слить» (see `catalog-moderation/genre-merge`).

#### Scenario: Singular and plural
- **WHEN** a pending genre is «романы» and a published genre is «роман»
- **THEN** «роман» is a `same_stem` hint

### Requirement: Book hints
A candidate book SHALL match as `same_isbn` when the normalized ISBNs are equal, where an ISBN-10 also matches its 978-prefixed ISBN-13; otherwise as `same_title_author` when the normalized titles are equal and the books share an author. Books of every status MUST be candidates, including rejected ones. Each candidate is listed once with its strongest match, ordered by match strength, then published, pending, rejected, then title. The admin labels them «тот же ISBN» and «то же название и автор», links each to its edit page, and shows its status, ISBN and year.

#### Scenario: ISBN-10 against ISBN-13
- **WHEN** a pending book has an ISBN-10 and a published book has the same ISBN in its 978-prefixed ISBN-13 form
- **THEN** the published book is a `same_isbn` hint

#### Scenario: Rejected duplicate
- **WHEN** a rejected book has the same title and author as a pending book
- **THEN** the rejected book is a `same_title_author` hint
