## MODIFIED Requirements

### Requirement: Create, edit and delete a collection
`POST /admin/collections` SHALL create a collection (201 with the collection).
- **Fields.** The request has `title` and `size`, plus optional `slug`, `description`, `source`, `source_url`, `genre_id`, `is_published`, `kind` (`manual` by default, or `auto`) and `rule`.
- **Rule.** `rule` is `{metric, period_days?, min_ratings?, confidence?}`.
  - It is required for kind `auto`, and MUST be rejected for kind `manual` with 400 «rule: only for automatic collections».
  - A missing rule for `auto` MUST return 400 «rule: required for automatic collections».
  - `metric` MUST be `weighted_rating`, `most_read`, `most_wanted`, `trending`, `book_beats_adaptation` or `adaptation_beats_book`.
  - `period_days` is null or 1-3650. It defaults to 30 for `trending`, and to null (all time) otherwise.
  - `min_ratings` and `confidence` apply to `weighted_rating` and to the verdict metrics `book_beats_adaptation` and `adaptation_beats_book`. They are stored as null for the other metrics.
    - For `weighted_rating`, `min_ratings` is the minimum rating count: 1-10000, default 3.
    - For the verdict metrics, `min_ratings` is the minimum vote count of the source adaptation: 5-10000, default 5. A value below 5 MUST return 400 «rule.min_ratings: must be between 5 and 10000».
    - `confidence` is 1-10000, default 10.
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

The `admin` role and bearer rules of «Admin collection endpoints» apply unchanged.

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

#### Scenario: Verdict collection with defaults
- **WHEN** an admin creates an `auto` collection with `rule: {metric: "book_beats_adaptation"}`
- **THEN** the response is 201 with `min_ratings` 5, `confidence` 10, `period_days` null, and a computed snapshot

#### Scenario: Verdict minimum below 5
- **WHEN** an admin creates an `auto` collection with `rule: {metric: "adaptation_beats_book", min_ratings: 3}`
- **THEN** the response is 400 «rule.min_ratings: must be between 5 and 10000» and nothing is created

#### Scenario: Moderator creates a verdict collection
- **WHEN** a moderator sends `POST /api/v1/admin/collections` with `rule: {metric: "book_beats_adaptation"}`
- **THEN** the response is 403 and nothing is created

### Requirement: Collection detail for admins
`GET /admin/collections/{id}` SHALL return the collection with all its fields, `shown_count` and `books_count`, and every book as `{position, place, state, book}`.
- **Order.** For a manual collection the books follow the admin's order. For an automatic collection they follow the order of the current snapshot.
- **Item fields.**
  - `position` is 1..k over all books.
  - `book` carries `id`, `title`, `slug`, `status`, `cover_url` and `authors`.
  - `state` is `ranked` (shown publicly, with `place` 1..N), `reserve` (published but beyond N, `place` null) or `hidden` (not published, `place` null).
- **Automatic collections.** The response also carries `rule`, `computed_at` and `baseline_at` (the time of the movement baseline, or null). Each item also carries `score` and `movement` as in the public API, and `adaptation` with the same shape and rules as in the public API, null for metrics other than the verdict metrics.
- **Drafts.** Drafts are returned the same way, as if they were published.

#### Scenario: Hidden book in the middle
- **WHEN** an admin opens a collection of size 2 holding published A, pending B, published C
- **THEN** A is `ranked` place 1, B is `hidden`, C is `ranked` place 2

#### Scenario: Automatic collection detail
- **WHEN** an admin opens an automatic collection
- **THEN** the books come in snapshot order with their `score`, and the response has `rule` and `computed_at`

#### Scenario: Verdict collection detail
- **WHEN** an admin opens a `book_beats_adaptation` collection
- **THEN** each item has its `score` (0-1) and its `adaptation` with `votes_count` and `share`

### Requirement: Automatic collection admin page
- **Kind choice.** On «Новая подборка» the admin SHALL choose «Тип»: «Ручная — книги и порядок задаёт администратор» (default) or «Автоматическая — по правилу из данных сайта». The kind is shown read-only on the edit page.
- **Rule fields.** For an automatic collection the form SHALL show:
  - «Показатель», with the options «Взвешенный рейтинг (оценки читателей)», «Чаще всего прочитаны», «Чаще всего хотят прочитать», «В тренде (активность за период)», «Книга лучше экранизации (голоса читателей)» and «Экранизация лучше книги (голоса читателей)»;
  - «Период, дней», empty meaning «за всё время», prefilled with 30 for «В тренде»;
  - for the weighted rating only, «Минимум оценок» (default 3) and «Порог доверия (m)» (default 10), with the hint «Чем больше m, тем сильнее рейтинг книг с малым числом оценок тянется к среднему по сайту»;
  - for the two verdict metrics only, «Минимум голосов» (default 5, at least 5) and «Порог доверия (m)» (default 10), with the hint «Книга оценивается по экранизации с наибольшим числом голосов. Чем больше m, тем сильнее доля голосов у книг с малым числом голосов тянется к средней по сайту». Switching between the weighted rating and a verdict metric resets the minimum to that metric's default.

  «Жанр» acts as the rule's genre filter. «Количество мест» is the size.
- **Snapshot panel.** Instead of the book panel, the edit page SHALL show:
  - «Пересчитано: {date and time}», or «Ещё не пересчитана»;
  - when there is a baseline, «Движение — относительно пересчёта {date and time}»;
  - the button «Пересчитать сейчас»;
  - a table of the snapshot in order, with the place or «Резерв» / «Скрыта: не опубликована», the movement as on the public page, the title with authors, and the score. The score shows 2 decimals for the weighted rating, a whole percent for the verdict metrics (a score of 0.884 shows «88%»), and a whole number otherwise.
  - For the verdict metrics, each row with an `adaptation` also shows «Экранизация: «{title}» ({year}) · {N} {голос|голоса|голосов}» under the authors, with « ({year})» left out when the year is unknown.
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

#### Scenario: Rule form for a verdict metric
- **WHEN** an admin creates an automatic collection and picks «Книга лучше экранизации (голоса читателей)»
- **THEN** the form shows «Минимум голосов» 5 and «Порог доверия (m)» 10, and no «Минимум оценок»

#### Scenario: Verdict snapshot row
- **WHEN** an admin opens a `book_beats_adaptation` collection whose first book has the score 0.884 and the adaptation «Война и мир» (1967) with 40 votes
- **THEN** its row shows the score «88%» and «Экранизация: «Война и мир» (1967) · 40 голосов»
