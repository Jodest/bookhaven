## Purpose

Lets an admin create, edit, fill and order BookHaven's manual curated collections, see which books are currently hidden from readers, and know when the reserve has run out.

## ADDED Requirements

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
