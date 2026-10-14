## ADDED Requirements

### Requirement: Bulk paste preview
`POST /api/v1/admin/collections/{id}/books/paste/preview` `{text}` SHALL parse the pasted text, match every line against the catalog and return a per-line report without changing anything. The endpoint MUST require the `admin` role (a moderator or reader gets 403) and a bearer header (a cookie-only request gets 401), like every other collection write route. Blank lines SHALL be ignored. Every other line produces one report entry `{line, text, title, author, status, book, candidates, duplicate_of}`, where `line` is the 1-based number of the line in the pasted text (blank lines counted). Entries SHALL be in pasted order. The response SHALL also carry `books_count`, `free_slots` (1000 minus `books_count`) and the count of entries per status. Text with more than 500 non-blank lines MUST return 422 «too many lines (max 500)». Text over 100000 characters, or with no non-blank line, MUST return 400 naming `text`. An unknown collection MUST return 404.

#### Scenario: Preview writes nothing
- **WHEN** an admin previews a paste of three lines that all match catalog books
- **THEN** the response is 200 with three «found» entries, and the collection is unchanged

#### Scenario: Too many lines
- **WHEN** an admin previews a paste with 501 non-blank lines
- **THEN** the response is 422 «too many lines (max 500)»

#### Scenario: Moderator preview
- **WHEN** a moderator calls the preview endpoint
- **THEN** the response is 403

#### Scenario: Blank lines keep numbering
- **WHEN** a paste has a book on line 1, a blank line 2 and a book on line 3
- **THEN** the report has two entries, with `line` 1 and 3

### Requirement: Pasted line parsing
Each non-blank line SHALL be read as one book, title first. Leading numbering MUST be dropped: a number followed by «.» or «)», optionally prefixed by «№», as in «1. », «12) », «№ 3. ». A number not followed by «.» or «)», as in «1984», is part of the title. The rest SHALL be split into title and author at the first tab or, when there is no tab, at the last spaced dash («—», «–» or «-» with spaces on both sides) or spaced «/». Without a separator the whole line is the title and the author is empty. With three or more tab-separated cells whose first cell is a number, that cell is the numbering, the second is the title and the third is the author. Surrounding quotes («», "", „“, '') and trailing punctuation MUST be removed from the title and the author. A line whose title is empty after this MUST be reported as `invalid` («Не распознано»).

#### Scenario: Numbered line with an author
- **WHEN** a line is «12) «Мастер и Маргарита» — Михаил Булгаков»
- **THEN** the title is «Мастер и Маргарита» and the author is «Михаил Булгаков»

#### Scenario: Number that is a title
- **WHEN** a line is «1984 — Джордж Оруэлл»
- **THEN** the title is «1984»

#### Scenario: Tab-separated row
- **WHEN** a line is «3», tab, «Пикник на обочине», tab, «Стругацкие»
- **THEN** the title is «Пикник на обочине» and the author is «Стругацкие»

#### Scenario: Numbering only
- **WHEN** a line is «7.»
- **THEN** the entry is `invalid`

### Requirement: Catalog matching of pasted lines
A line SHALL match catalog books whose title equals the pasted title after normalization. Normalization lowercases, folds «ё» to «е», turns every character that is not a letter or digit into a space, and collapses spaces. There is no fuzzy or partial title matching. When the line has an author, the candidates SHALL be narrowed to books with at least one author it matches. The author part may name several authors separated by «,», «;», «&» or « и ». An author matches when every word of one pasted name equals a word of the author's normalized name, or is a single letter equal to the first letter of such a word, so «Булгаков» and «М. Булгаков» both match «Михаил Булгаков». When the split into title and author yields no title match, the whole line SHALL be tried as a title-only line. Rejected books never count as a match, but they are listed as candidates. The line status SHALL be:
- `found` («Найдено») when exactly one non-rejected book remains, returned in `book`;
- `ambiguous` («Неоднозначно») when several non-rejected books remain, or when the title matched non-rejected books but none of them has a matching author; up to 10 candidates are returned, published first, for the admin to choose from;
- `rejected` («Отклонена — нельзя добавить») when only rejected books match;
- `not_found` («Не найдено — сначала добавьте книгу в каталог») when no book matches;
- `in_collection` («Уже в подборке») when every non-rejected match is already in the collection (`book` is set when there is one);
- `duplicate` («Повтор строки N») when an earlier entry resolved to the same book or has the same normalized text, with `duplicate_of` set to that earlier line.

Every returned book and candidate SHALL carry `id`, `title`, `slug`, `status`, `cover_url`, `published_year`, `authors` (`id`, `name`) and `in_collection`. No line SHALL ever create a book, an author or a placeholder.

#### Scenario: Case, «ё» and punctuation do not matter
- **WHEN** the catalog has «Ёлка: повесть» and a line is «ЕЛКА ПОВЕСТЬ»
- **THEN** the entry is `found` with that book

#### Scenario: Same title, different authors
- **WHEN** the catalog has two published books «Мать», by Горький and by Чапек, and a line is «Мать»
- **THEN** the entry is `ambiguous` with both books as candidates

#### Scenario: Author narrows the match
- **WHEN** the catalog has two published books «Мать», by Горький and by Чапек, and a line is «Мать — М. Горький»
- **THEN** the entry is `found` with Горький's book

#### Scenario: Author does not match
- **WHEN** the catalog has one published book «Мать», by Горький, and a line is «Мать — Толстой»
- **THEN** the entry is `ambiguous` with Горький's book as the only candidate

#### Scenario: Book missing from the catalog
- **WHEN** a line names a title that no catalog book has
- **THEN** the entry is `not_found`, and no book is created

#### Scenario: Only a rejected book
- **WHEN** a line's only title match is a rejected book
- **THEN** the entry is `rejected`, and the book is listed as a candidate with status `rejected`

#### Scenario: Pending book matches
- **WHEN** a line's only title match is a pending book
- **THEN** the entry is `found` with that book and its status `pending`

#### Scenario: Already in the collection
- **WHEN** a line matches exactly one book that the collection already holds
- **THEN** the entry is `in_collection`

#### Scenario: Same book pasted twice
- **WHEN** lines 2 and 9 both match the same book
- **THEN** line 2 is `found` and line 9 is `duplicate` with `duplicate_of: 2`

#### Scenario: Dash inside the title
- **WHEN** the catalog has a book titled «Москва - Петушки» and a line is «Москва - Петушки»
- **THEN** the split title «Москва» does not match, the whole line is tried as a title, and the entry is `found`

### Requirement: Bulk add of chosen books
`POST /api/v1/admin/collections/{id}/books/bulk` `{book_ids, position?}` SHALL add the given catalog books in the given order as one contiguous block. Without `position` the block goes at the end; with `position` it goes at that position (clamped to 1..k+1) and shifts the following books down. Everything SHALL happen in one transaction that serializes with other edits of the same collection, and positions SHALL stay 1..k without gaps or duplicates. The endpoint MUST require the `admin` role and a bearer header. These ids MUST be skipped, without failing the others: an id that is not a catalog book (`not_found`), a rejected book (`rejected`), a book already in the collection, including one added concurrently (`already_in_collection`), and a repeat of an earlier id in the same request (`duplicate`). Pending and published books SHALL be added, the same as with single add. When the books left to add exceed the free places (1000 minus the books already held), the response MUST be 422 «collection is full» and nothing is added. The success response SHALL be 200 `{added: [{book_id, position}], skipped: [{book_id, reason}], books_count}`. An empty `book_ids`, more than 500 ids, an id that is not a UUID, or a position below 1 MUST return 400; an unknown collection 404. Books MUST NOT be created by this endpoint.

#### Scenario: Append in pasted order
- **WHEN** an admin bulk-adds books X, Y, Z to a collection holding A, B
- **THEN** the order is A, B, X, Y, Z

#### Scenario: Insert at a position
- **WHEN** an admin bulk-adds X, Y at position 2 to a collection holding A, B, C
- **THEN** the order is A, X, Y, B, C

#### Scenario: Mixed request
- **WHEN** an admin bulk-adds a published book, a rejected book and a book already in the collection
- **THEN** the response is 200 with one added book, and skipped entries with `rejected` and `already_in_collection`

#### Scenario: Over capacity
- **WHEN** a collection holds 990 books and an admin bulk-adds 11 new books
- **THEN** the response is 422 «collection is full» and the collection still holds 990 books

#### Scenario: Cookie-only bulk add
- **WHEN** an admin's browser sends the bulk-add request with only the cookie
- **THEN** the response is 401 and nothing is added

### Requirement: Bulk paste panel
The admin collection edit page SHALL offer «Вставить список» in the book panel. It opens a textarea «Список книг» with the hint «Одна книга в строке: Название — Автор. Нумерация и кавычки допускаются. До 500 строк.» and the button «Найти в каталоге». The preview SHALL show the per-status counts, then one row per entry with its line number, the pasted text, the status label, and the matched book (title, authors, year, «Опубликована» or «На модерации»). `found` rows SHALL be ticked by default. An `ambiguous` row SHALL list its candidates to pick from, with none chosen; picking one ticks the row. Rejected candidates and candidates already in the collection MUST NOT be pickable. `not_found`, `rejected`, `in_collection`, `duplicate` and `invalid` rows MUST NOT be tickable. A `not_found` row SHALL offer «Добавить в каталог», opening book creation in a new tab. When two ticked rows point at the same book, only the first SHALL stay ticked. The admin SHALL choose «В конец» (default) or «На позицию» with a number, and then «Добавить выбранные (N)». The button MUST be disabled when nothing is ticked, and also when N exceeds the free places, with the note «Свободно мест: {free} — снимите лишние отметки». Changing the text MUST discard the preview. After adding, the panel SHALL show «Добавлено: {added}» and «Пропущено: {skipped}» with each skipped line's reason, and refresh the book list. «Скопировать ненайденные» SHALL copy the `not_found` lines, one per line, so that the admin can re-paste them after adding the books to the catalog. An error response MUST be shown as a readable message, and 422 «collection is full» MUST be shown as «Не хватает мест в подборке (максимум 1000 книг)».

#### Scenario: Resolve an ambiguous line
- **WHEN** an admin previews «Мать», sees two candidates and picks Горький's book
- **THEN** the row becomes ticked and «Добавить выбранные» counts it

#### Scenario: Not found cannot be added
- **WHEN** the preview has a `not_found` row
- **THEN** the row has no checkbox, shows «Не найдено — сначала добавьте книгу в каталог» and the «Добавить в каталог» link

#### Scenario: Too many ticked rows
- **WHEN** the collection has 5 free places and 7 rows are ticked
- **THEN** «Добавить выбранные (7)» is disabled and the page shows «Свободно мест: 5 — снимите лишние отметки»

#### Scenario: Edited text discards the preview
- **WHEN** an admin changes the textarea after a preview
- **THEN** the preview table is hidden until «Найти в каталоге» is pressed again
