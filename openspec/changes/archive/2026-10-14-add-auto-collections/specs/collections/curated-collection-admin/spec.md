## MODIFIED Requirements

### Requirement: Admin collection endpoints
Every endpoint under `/api/v1/admin/collections` SHALL require the `admin` role (a moderator or reader MUST get 403); state-changing requests MUST also carry a bearer header (see `auth/authorization`). `GET /admin/collections` SHALL return `{items, total}` with every collection, draft or published, manual or automatic, newest first by creation. Each item has `id`, `slug`, `title`, `kind`, `size`, `is_published`, `books_count`, `shown_count`, `computed_at` (null for manual collections and before the first computation) and `updated_at`.

#### Scenario: Moderator lists collections
- **WHEN** a moderator calls `GET /api/v1/admin/collections`
- **THEN** the response is 403

#### Scenario: Cookie-only collection write
- **WHEN** an admin's browser sends `POST /api/v1/admin/collections` with only the cookie
- **THEN** the response is 401 and nothing is created

### Requirement: Create, edit and delete a collection
`POST /admin/collections` SHALL create a collection (201 with the collection).
- **Fields.** The request has `title` and `size`, plus optional `slug`, `description`, `source`, `source_url`, `genre_id`, `is_published`, `kind` (`manual` by default, or `auto`) and `rule`.
- **Rule.** `rule` is `{metric, period_days?, min_ratings?, confidence?}`.
  - It is required for kind `auto`, and MUST be rejected for kind `manual` with 400 «rule: only for automatic collections».
  - A missing rule for `auto` MUST return 400 «rule: required for automatic collections».
  - `metric` MUST be `weighted_rating`, `most_read`, `most_wanted` or `trending`.
  - `period_days` is null or 1-3650. It defaults to 30 for `trending`, and to null (all time) otherwise.
  - `min_ratings` and `confidence` are 1-10000 and default to 3 and 10. They apply to `weighted_rating` only and are stored as null for the other metrics.
  - An invalid rule field MUST return 400 naming it, e.g. «rule.metric: …».
- **Immediate computation.** Creating an automatic collection SHALL compute its first snapshot at once, without a movement baseline. If that computation fails, the collection is still created, with no snapshot.
- **Slug.** An empty slug MUST be derived from the title by transliteration.
- **Editing.** `PATCH /admin/collections/{id}` SHALL change any of those fields except `kind` (200). `rule` is replaced as a whole. Sending `kind` MUST return 400 «kind: cannot be changed». Changing the rule, the linked genre or the size of an automatic collection SHALL recompute it at once without a movement baseline.
- **Errors.**
  - An invalid field MUST return 400 with the field named.
  - A slug already used by another collection MUST return 409 «slug already exists».
  - A genre that does not exist, is rejected or was merged MUST return 400 «invalid genre_id».
  - An unknown id MUST return 404.
- **Publish time.** Publishing a collection MUST record when it was first published. Later unpublish/republish MUST keep that time.
- **Deleting.** `DELETE /admin/collections/{id}` SHALL delete the collection and its book list or snapshot (200), never the books.

#### Scenario: Slug taken
- **WHEN** an admin creates a collection with the slug of an existing one
- **THEN** the response is 409 and nothing is created

#### Scenario: Slug from title
- **WHEN** an admin creates «Топ 100 фантастики» without a slug
- **THEN** the slug is `top-100-fantastiki`

#### Scenario: Shrinking the size
- **WHEN** an admin changes the size of a manual collection from 100 to 50
- **THEN** readers see the first 50 published books and the rest become reserve, with the stored order unchanged

#### Scenario: Automatic collection with defaults
- **WHEN** an admin creates an `auto` collection with `rule: {metric: "weighted_rating"}`
- **THEN** the response is 201 with `min_ratings` 3, `confidence` 10, `period_days` null, and a computed snapshot

#### Scenario: Automatic collection without a rule
- **WHEN** an admin creates an `auto` collection without `rule`
- **THEN** the response is 400 «rule: required for automatic collections» and nothing is created

#### Scenario: Changing the kind
- **WHEN** an admin sends `PATCH` with `kind: "auto"` for a manual collection
- **THEN** the response is 400 «kind: cannot be changed» and the collection is unchanged

### Requirement: Collection detail for admins
`GET /admin/collections/{id}` SHALL return the collection with all its fields, `shown_count` and `books_count`, and every book as `{position, place, state, book}`.
- **Order.** For a manual collection the books follow the admin's order. For an automatic collection they follow the order of the current snapshot.
- **Item fields.**
  - `position` is 1..k over all books.
  - `book` carries `id`, `title`, `slug`, `status`, `cover_url` and `authors`.
  - `state` is `ranked` (shown publicly, with `place` 1..N), `reserve` (published but beyond N, `place` null) or `hidden` (not published, `place` null).
- **Automatic collections.** The response also carries `rule`, `computed_at` and `baseline_at` (the time of the movement baseline, or null). Each item also carries `score` and `movement` as in the public API.
- **Drafts.** Drafts are returned the same way, as if they were published.

#### Scenario: Hidden book in the middle
- **WHEN** an admin opens a collection of size 2 holding published A, pending B, published C
- **THEN** A is `ranked` place 1, B is `hidden`, C is `ranked` place 2

#### Scenario: Automatic collection detail
- **WHEN** an admin opens an automatic collection
- **THEN** the books come in snapshot order with their `score`, and the response has `rule` and `computed_at`

### Requirement: Admin collections section
The admin app SHALL have an admin-only sidebar section «Подборки» (`/collections`).
- **List.** The section has a table of collections and «Новая подборка» (`/collections/new`). The table shows the title, «Ручная»/«Авто», «Опубликована»/«Черновик», «{shown_count} из {size}», the books count and «Редактировать».
- **Edit page fields.** The edit page (`/collections/{id}`) SHALL offer:
  - «Название», «Адрес (slug)», «Описание», «Количество мест», «Источник», «Ссылка на источник», «Жанр» (published genres) and «Опубликована»;
  - «Удалить подборку» with a confirmation.
- **Book panel (manual collections).** It SHALL list every book in the admin's order:
  - each book shows its place or the marker «Резерв» or «Скрыта: не опубликована»;
  - a divider separates the reserve;
  - each book has «Выше»/«Ниже» buttons, a position field «Переместить на» and «Убрать».
- **Catalog search (manual collections).** «Найти книгу в каталоге» shows each result's status and offers «Добавить». «Добавить» is disabled with «Уже в подборке», and for rejected books.
- **Reserve warning.** When the `shown_count` of a manual collection is below `size`, the page and the table row SHALL warn «В подборке видно {shown_count} из {size} книг — пополните резерв».
- **Automatic collections** get the page described in «Automatic collection admin page» instead of the book panel.

#### Scenario: Reserve warning
- **WHEN** an admin opens a manual collection of size 100 that shows 98 places
- **THEN** the page warns «В подборке видно 98 из 100 книг — пополните резерв»

#### Scenario: Moderator does not see the section
- **WHEN** a moderator uses the admin app
- **THEN** the sidebar has no «Подборки» section

## ADDED Requirements

### Requirement: Book editing only in manual collections
These requests MUST return 422 «collection is automatic» and change nothing when the collection is automatic:
- adding a book (`POST /admin/collections/{id}/books`);
- removing a book;
- moving a book (`PUT …/books/{bookId}/position`);
- bulk paste preview (`POST …/books/paste/preview`);
- bulk add (`POST …/books/bulk`).

The role and bearer rules of these endpoints are unchanged: a moderator gets 403 and a cookie-only request 401, before the kind is checked.

#### Scenario: Adding a book to an automatic collection
- **WHEN** an admin adds a published book to an automatic collection
- **THEN** the response is 422 «collection is automatic» and the snapshot is unchanged

#### Scenario: Bulk paste into an automatic collection
- **WHEN** an admin previews a paste for an automatic collection
- **THEN** the response is 422 «collection is automatic»

### Requirement: Recompute now
`POST /api/v1/admin/collections/{id}/recompute` SHALL recompute the snapshot of an automatic collection at once, using the current snapshot as the movement baseline, and return 200 with the collection detail. It MUST require the `admin` role (a moderator or reader gets 403) and a bearer header (a cookie-only request gets 401). A manual collection MUST return 422 «collection is not automatic»; an unknown id 404. It works for drafts as well as published collections.

#### Scenario: Recompute an automatic collection
- **WHEN** an admin recomputes an automatic collection after new ratings were given
- **THEN** the response is 200 with the new snapshot, a new `computed_at`, and `baseline_at` equal to the previous `computed_at`

#### Scenario: Recompute a manual collection
- **WHEN** an admin calls recompute for a manual collection
- **THEN** the response is 422 «collection is not automatic»

#### Scenario: Moderator recompute
- **WHEN** a moderator calls recompute
- **THEN** the response is 403

### Requirement: Automatic collection admin page
- **Kind choice.** On «Новая подборка» the admin SHALL choose «Тип»: «Ручная — книги и порядок задаёт администратор» (default) or «Автоматическая — по правилу из данных сайта». The kind is shown read-only on the edit page.
- **Rule fields.** For an automatic collection the form SHALL show:
  - «Показатель», with the options «Взвешенный рейтинг (оценки читателей)», «Чаще всего прочитаны», «Чаще всего хотят прочитать» and «В тренде (активность за период)»;
  - «Период, дней», empty meaning «за всё время», prefilled with 30 for «В тренде»;
  - for the weighted rating only, «Минимум оценок» (default 3) and «Порог доверия (m)» (default 10), with the hint «Чем больше m, тем сильнее рейтинг книг с малым числом оценок тянется к среднему по сайту».

  «Жанр» acts as the rule's genre filter. «Количество мест» is the size.
- **Snapshot panel.** Instead of the book panel, the edit page SHALL show:
  - «Пересчитано: {date and time}», or «Ещё не пересчитана»;
  - when there is a baseline, «Движение — относительно пересчёта {date and time}»;
  - the button «Пересчитать сейчас»;
  - a table of the snapshot in order, with the place or «Резерв» / «Скрыта: не опубликована», the movement as on the public page, the title with authors, and the score. The score shows 2 decimals for the weighted rating and a whole number otherwise.
- **After changes.** Saving a changed rule, genre or size, or pressing «Пересчитать сейчас», SHALL refresh the snapshot table.
- **Short snapshot warning.** When `shown_count` is below `size`, the page and the table row SHALL warn «Правилу соответствуют только {shown_count} из {size} книг».
- **Hidden controls.** No add, remove, reorder or «Вставить список» control is shown for an automatic collection.

#### Scenario: Rule form for the weighted rating
- **WHEN** an admin creates an automatic collection and picks «Взвешенный рейтинг (оценки читателей)»
- **THEN** the form shows «Минимум оценок» 3 and «Порог доверия (m)» 10

#### Scenario: Recompute from the page
- **WHEN** an admin presses «Пересчитать сейчас» on an automatic collection
- **THEN** the snapshot table and «Пересчитано» are refreshed

#### Scenario: Short snapshot warning
- **WHEN** an automatic collection of size 100 shows 40 places
- **THEN** the page warns «Правилу соответствуют только 40 из 100 книг»

#### Scenario: No book editing controls
- **WHEN** an admin opens an automatic collection
- **THEN** there is no catalog search, no «Вставить список» and no reorder control
