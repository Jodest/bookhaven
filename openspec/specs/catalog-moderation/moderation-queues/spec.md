# catalog-moderation/moderation-queues Specification

## Purpose

Gives admins one review queue per entity type (books, authors, genres) where entities waiting for moderation are listed with enough context to publish, reject or merge them.

## Requirements

### Requirement: Queue endpoints
The system SHALL expose `GET /api/v1/admin/books/moderation`, `GET /api/v1/admin/authors/moderation` and `GET /api/v1/admin/genres/moderation`, each requiring the `admin` role. Each MUST return `{items, total, page, limit}` for one page of entities with the requested `status` (default `pending`), newest first. `limit` defaults to 20 and is capped at 50. An unknown `status` or filter value MUST return 400.

#### Scenario: Default queue
- **WHEN** an admin requests `GET /api/v1/admin/authors/moderation` without parameters
- **THEN** the response lists pending authors, newest first, 20 per page

#### Scenario: Invalid filter
- **WHEN** an admin requests `?qid=maybe`
- **THEN** the response is 400

### Requirement: Queue filters
Each queue SHALL accept `search` and these filters, which combine with the status and with each other:
- authors: `qid=with|without` (has a Wikidata QID or not) and `books=none` (authors with no linked books); `search` is a full-text search on name and biography;
- genres: `qid=with|without`; `search` matches the name as a substring;
- books: `source=manual|harvested` (no external source, or created by the harvest) and `ai=publish|reject|needs_human|none` (the AI verdict, stale or not, or `none` for books without any verdict, see `catalog-moderation/ai-verdicts`); `search` is a full-text search on title, ISBN and description.

#### Scenario: Authors without books
- **WHEN** an admin requests `?books=none`
- **THEN** only pending authors with no linked books are listed

#### Scenario: Harvested pending books
- **WHEN** an admin requests `?status=pending&source=harvested`
- **THEN** only pending books created by the harvest are listed

#### Scenario: Books the AI would publish
- **WHEN** an admin requests `?status=pending&ai=publish`
- **THEN** only pending books whose current AI verdict is `publish` are listed

#### Scenario: Books never checked by the AI
- **WHEN** an admin requests `?ai=none`
- **THEN** only books without any AI verdict are listed

#### Scenario: Unknown AI filter
- **WHEN** an admin requests `GET /api/v1/admin/books/moderation?ai=maybe`
- **THEN** the response is 400

### Requirement: Merged genres never appear
The genre queue SHALL NOT list merged genres (aliases), whatever the status filter.

#### Scenario: Rejected tab
- **WHEN** an admin opens the rejected genre queue after merging «романы» into «роман»
- **THEN** «романы» is not listed

### Requirement: Author and genre context
Each author or genre queue item SHALL carry the entity, its `created_at`, `books_total` (all linked books), `books_visible` (linked published books), `sample_books` (up to 3 linked books ordered by title, each `{id, title, status}`) and `similar` hints (see `catalog-moderation/duplicate-hints`).

#### Scenario: Author with hidden books
- **WHEN** a pending author is linked to two pending books and one published book
- **THEN** the item has `books_total: 3` and `books_visible: 1`

### Requirement: Book context
Each book queue item SHALL carry the book with its authors and genres (each with its own status), its `created_at`, its external source and `external_id`, `similar` hints (see `catalog-moderation/duplicate-hints`) and `ai_verdict`: `null` when the book has no verdict, otherwise `{verdict, reason_codes, reason, confidence, model, prompt_version, created_at, stale}` (see `catalog-moderation/ai-verdicts`).

#### Scenario: Harvested book
- **WHEN** a harvested pending book is listed
- **THEN** the item shows its source and external ID

#### Scenario: Book without a verdict
- **WHEN** a pending book that was never checked by the AI is listed
- **THEN** the item has `ai_verdict: null`

#### Scenario: Book with a stale verdict
- **WHEN** a pending book was edited after its AI verdict was computed
- **THEN** the item's `ai_verdict` has `stale: true`

### Requirement: Moderation section
The admin «Модерация» section SHALL have the tabs «Книги» (`/moderation/books`), «Авторы» (`/moderation/authors`) and «Жанры» (`/moderation/genres`) for admins only, plus «Отзывы» for admins and moderators. The tabs MUST show the pending counts of books, authors and genres, and the section MUST show their sum.

#### Scenario: Pending badges
- **WHEN** there are 3 pending books, 2 pending authors and 1 pending genre
- **THEN** the tabs show 3, 2 and 1 and the section shows 6

### Requirement: Queue page
Each queue page SHALL have status tabs «На модерации», «Опубликованные» and «Отклонённые», a search field, the entity's filters, a table with checkboxes and pagination; the status, search, filters and page MUST be kept in the URL. Row actions depend on the tab:
- «На модерации»: «Опубликовать» and «Отклонить»;
- «Опубликованные»: «Отклонить»;
- «Отклонённые»: «Вернуть на модерацию» and «Опубликовать».

The same actions MUST be offered for the selected rows («Опубликовать выбранные (N)» and so on) through the bulk status endpoint. Every action MUST ask for confirmation first. Rejecting an author or genre that has published books MUST warn that the links stay: the author's name stays on those book pages without a link, and the genre disappears from them. An empty pending queue MUST say so («Нет авторов на модерации» and similar); an empty filtered result shows «… не найдены».

#### Scenario: Reject with visible books
- **WHEN** an admin rejects an author linked to 4 published books
- **THEN** the confirmation says the links stay and the name stays without a link on the pages of the published books (4)

#### Scenario: Restoring a rejected book
- **WHEN** an admin confirms «Вернуть на модерацию» on the rejected tab
- **THEN** the book becomes pending and leaves the rejected list
