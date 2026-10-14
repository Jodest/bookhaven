# collections/curated-collection-admin Specification

## Purpose
Lets an admin create, edit, fill and order BookHaven's manual curated collections, see which books are currently hidden from readers, and know when the reserve has run out.

## Requirements

### Requirement: Admin collection endpoints
Every endpoint under `/api/v1/admin/collections` SHALL require the `admin` role (a moderator or reader MUST get 403); state-changing requests MUST also carry a bearer header (see `auth/authorization`). `GET /admin/collections` SHALL return `{items, total}` with every collection, draft or published, newest first by creation, each with `id`, `slug`, `title`, `kind`, `size`, `is_published`, `books_count`, `shown_count` and `updated_at`.

#### Scenario: Moderator lists collections
- **WHEN** a moderator calls `GET /api/v1/admin/collections`
- **THEN** the response is 403

#### Scenario: Cookie-only collection write
- **WHEN** an admin's browser sends `POST /api/v1/admin/collections` with only the cookie
- **THEN** the response is 401 and nothing is created

### Requirement: Create, edit and delete a collection
`POST /admin/collections` SHALL create a manual collection from `title`, `size` and optional `slug`, `description`, `source`, `source_url`, `genre_id`, `is_published` (201 with the collection). An empty slug MUST be derived from the title by transliteration. `PATCH /admin/collections/{id}` SHALL change any of those fields (200). An invalid field MUST return 400 with the field named; a slug already used by another collection MUST return 409 «slug already exists»; a genre that does not exist, is rejected or was merged MUST return 400 «invalid genre_id». Publishing a collection MUST record when it was first published; later unpublish/republish MUST keep that time. `DELETE /admin/collections/{id}` SHALL delete the collection and its book list (200), never the books. An unknown id MUST return 404.

#### Scenario: Slug taken
- **WHEN** an admin creates a collection with the slug of an existing one
- **THEN** the response is 409 and nothing is created

#### Scenario: Slug from title
- **WHEN** an admin creates «Топ 100 фантастики» without a slug
- **THEN** the slug is `top-100-fantastiki`

#### Scenario: Shrinking the size
- **WHEN** an admin changes the size of a collection from 100 to 50
- **THEN** readers see the first 50 published books and the rest become reserve, with the stored order unchanged

### Requirement: Collection detail for admins
`GET /admin/collections/{id}` SHALL return the collection with all its fields and every book in the admin's order as `{position, place, state, book}`, where `position` is 1..k over all books, `book` carries `id`, `title`, `slug`, `status`, `cover_url` and `authors`, `state` is `ranked` (shown publicly, with `place` 1..N), `reserve` (published but beyond N, `place` null) or `hidden` (not published, `place` null), plus `shown_count` and `books_count`. Drafts are returned the same way, as if they were published.

#### Scenario: Hidden book in the middle
- **WHEN** an admin opens a collection of size 2 holding published A, pending B, published C
- **THEN** A is `ranked` place 1, B is `hidden`, C is `ranked` place 2

### Requirement: Adding and removing books
`POST /admin/collections/{id}/books` `{book_id, position?}` SHALL add a catalog book whose status is `pending` or `published`, at the end of the order or, when `position` is given, at that position (clamped to 1..k+1) shifting the following books down (201). A pending book is accepted so it can wait in the order until it is published. A rejected book MUST return 422 «book is rejected»; an unknown book 404 «book not found»; a book already in the collection 409 «book already in collection»; a collection already holding 1000 books 422 «collection is full». `DELETE /admin/collections/{id}/books/{bookId}` SHALL remove the book and close the gap (200); a book not in the collection MUST return 404. Books MUST NOT be created from a collection: a book missing from the catalog has to be added to the catalog first.

#### Scenario: Pending book queued
- **WHEN** an admin adds a pending book at the end of a full collection
- **THEN** it is stored as `hidden` and readers do not see it

#### Scenario: Rejected book
- **WHEN** an admin adds a rejected book
- **THEN** the response is 422 and the collection is unchanged

### Requirement: Reordering books
`PUT /admin/collections/{id}/books/{bookId}/position` `{position}` SHALL move the book to that 1-based position in the admin order (clamped to 1..k), shifting the books in between, in one transaction (200). A book not in the collection MUST return 404; a position below 1 or not an integer MUST return 400. Positions SHALL always stay 1..k without gaps or duplicates, also under concurrent edits of the same collection.

#### Scenario: Move to the top
- **WHEN** an admin moves the book at position 5 of A, B, C, D, E to position 1
- **THEN** the order is E, A, B, C, D

### Requirement: Admin collections section
The admin app SHALL have an admin-only sidebar section «Подборки» (`/collections`) with a table of collections (title, «Опубликована»/«Черновик», «{shown_count} из {size}», books count, «Редактировать») and «Новая подборка» (`/collections/new`). The edit page (`/collections/{id}`) SHALL offer the fields «Название», «Адрес (slug)», «Описание», «Количество мест», «Источник», «Ссылка на источник», «Жанр» (published genres) and «Опубликована», and «Удалить подборку» with a confirmation. Its book panel SHALL list every book in the admin's order with its place or the marker «Резерв» or «Скрыта: не опубликована», a divider before the reserve, «Выше»/«Ниже» buttons, a position field «Переместить на» and «Убрать»; and a catalog search «Найти книгу в каталоге» that shows each result's status and offers «Добавить», disabled with «Уже в подборке» or for rejected books. When `shown_count` is below `size` the page and the table row SHALL warn «В подборке видно {shown_count} из {size} книг — пополните резерв».

#### Scenario: Reserve warning
- **WHEN** an admin opens a collection of size 100 that shows 98 places
- **THEN** the page warns «В подборке видно 98 из 100 книг — пополните резерв»

#### Scenario: Moderator does not see the section
- **WHEN** a moderator uses the admin app
- **THEN** the sidebar has no «Подборки» section

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
