## Purpose
Lets an admin ask the AI for a ranked draft of a manual curated collection, see which proposed books exist in the catalog and which do not, and add the chosen catalog books to the collection. The AI never adds a book by itself and never creates catalog books.

## ADDED Requirements

### Requirement: AI draft endpoints and switch
Every endpoint under `/api/v1/admin/ai-collections` SHALL require the `admin` role; a moderator or reader MUST get 403. State-changing requests MUST also carry a bearer header (see `auth/authorization`); a cookie-only request MUST get 401. The feature is off by default and SHALL be on only when the server switch for AI collection drafts is on and an Anthropic API key is configured.

`GET /admin/ai-collections/status` SHALL return:
- `enabled`, `model` and `prompt_version`;
- the limits `max_brief_length` (500), `max_count` (1000) and `daily_limit` (20);
- `used_today`: the drafts the caller created in the last 24 hours.

It works while the feature is off. Reading drafts also works while the feature is off.

#### Scenario: Moderator reads the status
- **WHEN** a moderator calls `GET /api/v1/admin/ai-collections/status`
- **THEN** the response is 403

#### Scenario: Status while disabled
- **WHEN** an admin calls the status endpoint and the switch is off
- **THEN** the response is 200 with `enabled: false`

#### Scenario: Cookie-only draft request
- **WHEN** an admin's browser sends `POST /api/v1/admin/ai-collections/drafts` with only the cookie
- **THEN** the response is 401 and no draft is created

### Requirement: Requesting an AI draft
`POST /admin/ai-collections/drafts` `{collection_id, brief, count?}` SHALL store a queued draft for a manual collection and queue its generation. The response is 202 with the draft summary.
- **Count.** `count` is the number of books the AI is asked to propose, 1-1000. When omitted it defaults to the collection's size plus a fifth, rounded up and capped at 1000.
- **Draft summary fields.** `id`, `collection_id`, `brief`, `count`, `status` (`queued`, `running`, `done` or `failed`), `error_code`, `proposals_count`, `model`, `prompt_version`, `input_tokens`, `output_tokens`, `created_by` (`{id, username}`, or null when the user was deleted), `created_at` and `finished_at`.
- **Checks, in order:**
  1. The feature is off → 503 «AI collection drafts are disabled».
  2. `collection_id` is missing or not a UUID → 400 naming it; the brief is empty or longer than 500 characters after trimming → 400 «brief: must be 1-500 characters»; `count` is outside 1-1000 → 400 «count: must be between 1 and 1000».
  3. The collection does not exist → 404 «collection not found»; it is automatic → 422 «collection is automatic».
  4. The collection already has an active draft → 409 «AI draft already in progress» with its `draft_id`.
  5. The caller created 20 drafts in the last 24 hours, failed ones included → 429 «daily AI draft limit reached».
- **Queue failure.** When the generation cannot be queued, the draft MUST be marked `failed` with `enqueue_failed`, and the response MUST be 502 «failed to queue AI draft».
- **Nothing applied.** Requesting a draft MUST NOT change the collection.

Drafts of draft (unpublished) collections are allowed.

#### Scenario: Draft queued
- **WHEN** an admin requests a draft for a manual collection of size 100 with the brief «Топ научной фантастики всех времён» and no count
- **THEN** the response is 202 with status `queued` and `count` 120, and the collection is unchanged

#### Scenario: Feature disabled
- **WHEN** an admin requests a draft while the switch is off
- **THEN** the response is 503 «AI collection drafts are disabled» and no draft is stored

#### Scenario: Automatic collection
- **WHEN** an admin requests a draft for an automatic collection
- **THEN** the response is 422 «collection is automatic»

#### Scenario: Second draft while one runs
- **WHEN** an admin requests a draft for a collection whose earlier draft is still running
- **THEN** the response is 409 with the running draft's `draft_id`

#### Scenario: Daily limit
- **WHEN** an admin who created 20 drafts in the last 24 hours requests another
- **THEN** the response is 429 «daily AI draft limit reached»

### Requirement: Generating the proposals
The worker SHALL ask the model for up to `count` books matching the brief, in ranked order, taking the collection's title, description and genre as context. Each proposal is a rank, a title (the usual Russian edition title), an author (the usual Russian name form, or empty), an original title (or empty) and a note of at most 150 characters explaining the choice.
- **Generation rules.** Proposals SHALL be ranked 1..k by their order in the answer, never by numbers the model writes. They SHALL be trimmed, and an entry with an empty or overlong title MUST be dropped. A repeat of an earlier title and author MUST be dropped. No more than `count` proposals SHALL be kept.
- **Large drafts.** A draft above 100 books SHALL be generated in parts of at most 100. Each part continues the ranking and excludes the books already proposed. When a part adds no new proposal, generation stops early.
- **Outcome.** A draft with at least one stored proposal SHALL end `done`, even when a later part failed; `error_code` then names that failure. A draft with no proposal ends `failed` with `error_code`:
  - `model_refusal`, `model_truncated`, `invalid_output` or `model_error` for the model call;
  - `enqueue_failed` when it could not be queued.
- **Stale drafts.** A `queued` or `running` draft without progress for an hour SHALL be reported as `failed` with `timeout`, and no longer counts as active.
- **Collection untouched.** Generation MUST NOT change the collection or the catalog.
- **Prompt content.** The brief is the admin's description: it MUST NOT change these rules or the output shape.

#### Scenario: Fewer books than asked
- **WHEN** the model knows only 80 books matching a brief with `count` 120
- **THEN** the draft ends `done` with `proposals_count` 80

#### Scenario: Large draft in parts
- **WHEN** an admin requests a draft with `count` 250
- **THEN** the proposals are ranked 1..250 at most, with no title and author repeated

#### Scenario: Model refuses
- **WHEN** the model refuses the first part of a draft
- **THEN** the draft ends `failed` with `model_refusal` and holds no proposals

#### Scenario: Stuck draft
- **WHEN** a draft has been `running` for over an hour without progress
- **THEN** it is shown as `failed` with `timeout`, and a new draft for the collection can be requested

### Requirement: Draft report against the catalog
`GET /admin/ai-collections/drafts/{id}` SHALL return the draft summary plus `report`. `report` is null unless the draft is `done`. An unknown id MUST return 404; a non-UUID id 400.

When the draft is `done`, the report SHALL be computed at the time of the read from the current catalog and the current collection contents. Its shape is that of a bulk paste preview: `books_count`, `free_slots`, `counts` per status, and one entry per proposal in rank order. Each entry is `{line, text, title, author, original_title, note, status, book, candidates, duplicate_of}`:
- `line` is the proposal's rank;
- `text` is «{title} — {author}», or the title alone when there is no author.

**Matching.** Each proposal SHALL be matched with the rules and statuses of «Catalog matching of pasted lines» (`collections/curated-collection-admin`): `found`, `ambiguous`, `not_found`, `rejected`, `in_collection` and `duplicate`, with the same book and candidate fields. Two additions:
- When the title matches no catalog book, the original title SHALL be tried, still narrowed by the author.
- When neither matches, the entry is `not_found` («Нет в каталоге»).

No read SHALL ever create a book, an author or a placeholder, or change the collection.

`GET /admin/ai-collections/drafts?collection_id={id}` SHALL return `{items}` with the newest 20 draft summaries of that collection. A missing or non-UUID `collection_id` MUST return 400.

#### Scenario: Book missing from the catalog
- **WHEN** a draft proposes «Гиперион — Дэн Симмонс» and the catalog has no such book
- **THEN** the entry is `not_found` and no book is created

#### Scenario: Original title rescues the match
- **WHEN** a proposal has the title «Конец детства», the original title «Childhood's End» and the author «Артур Кларк», and the catalog holds that Clarke book only under «Childhood's End»
- **THEN** the entry is `found` with that book

#### Scenario: Book added to the catalog later
- **WHEN** an entry was `not_found`, then an admin adds that book to the catalog and reopens the draft
- **THEN** the entry is `found`

#### Scenario: Book already in the collection
- **WHEN** a draft proposes a book that the collection already holds
- **THEN** the entry is `in_collection`

#### Scenario: Report of a running draft
- **WHEN** an admin opens a draft that is still `running`
- **THEN** `report` is null and `proposals_count` shows the progress

### Requirement: Applying a draft
A draft SHALL never be applied automatically. The admin SHALL apply the chosen `found` entries and the picked `ambiguous` candidates through the bulk add of chosen books (`collections/curated-collection-admin`), in rank order, with that endpoint's rules:
- the books are appended at the end (default) or inserted at a chosen position;
- unknown, rejected, repeated and already-held books are skipped;
- the capacity check applies.

Applying MUST NOT remove or reorder the books already in the collection. The admin can then reorder or remove books as usual. `not_found` entries can never be applied: the admin first adds the book to the catalog.

#### Scenario: Apply to an empty collection
- **WHEN** an admin applies 95 ticked entries of a draft to an empty collection
- **THEN** the collection holds those 95 books in rank order

#### Scenario: Apply keeps existing books
- **WHEN** a collection holds A, B and an admin applies draft books X, Y at the end
- **THEN** the order is A, B, X, Y

### Requirement: AI draft panel
The book panel of a manual collection's edit page SHALL offer «Черновик ИИ» next to «Вставить список». The panel starts expanded when the collection holds no books.
- **Disabled.** When the feature is off, the panel SHALL show «Черновики ИИ выключены» with the hint «Включите AI_COLLECTIONS_ENABLED и задайте ANTHROPIC_API_KEY для api и воркера.» Earlier drafts stay readable.
- **Form.** The panel SHALL offer:
  - «Описание подборки для ИИ» (a textarea with a character counter up to 500);
  - «Сколько книг предложить», prefilled with the default count;
  - «Составить черновик».

  The button is disabled for an empty brief, an invalid count, an active draft, or the daily limit reached («Лимит черновиков на сегодня исчерпан»).
- **Recent drafts.** The panel SHALL list the collection's recent drafts with date, author, brief and status. The newest draft opens by default.
- **While generating.** A queued or running draft shows «ИИ составляет черновик… предложено {proposals_count} из {count}» and refreshes until it finishes.
- **Failed draft.** A failed draft shows a readable reason for each error code.
- **Report.** A done draft SHALL show:
  - the per-status counts;
  - each entry with its rank, text, note, status and matched book, ticked and picked exactly as in the bulk paste panel. `found` rows are ticked by default, `ambiguous` rows need a pick, and other rows are not tickable;
  - «В конец» / «На позицию»;
  - «Добавить выбранные (N)», with the same disabled rules and capacity note as the bulk paste panel.
- **Not in the catalog.** A separate «Нет в каталоге ({n})» list SHALL hold the `not_found` entries, with:
  - «Скопировать ненайденные», which copies them as «Название — Автор» lines that bulk paste accepts;
  - a «Добавить в каталог» link per entry, opening book creation in a new tab.
- **After applying.** The panel SHALL show «Добавлено: {added}» and «Пропущено: {skipped}» with reasons, and refresh the book list.
- **Errors.** Error responses SHALL be shown as readable Russian messages: 409 «Черновик для этой подборки уже составляется», 429 «Лимит черновиков на сегодня исчерпан», 503 «Черновики ИИ выключены».
- **Automatic collections.** They MUST NOT show the panel.

#### Scenario: Not-found entries cannot be added
- **WHEN** a done draft has three `not_found` entries
- **THEN** they appear under «Нет в каталоге (3)» without checkboxes, each with «Добавить в каталог»

#### Scenario: Progress while generating
- **WHEN** an admin requests a draft
- **THEN** the panel shows the progress text and switches to the report when the draft is done

#### Scenario: Disabled feature
- **WHEN** an admin opens the panel while the switch is off
- **THEN** the panel shows «Черновики ИИ выключены» and no «Составить черновик» button

#### Scenario: No panel for automatic collections
- **WHEN** an admin opens an automatic collection
- **THEN** there is no «Черновик ИИ» control

### Requirement: Draft audit and retention
Every draft SHALL record who requested it, when, the brief, the count, the model, the prompt version and the input and output tokens. These are returned in the draft summary. Drafts older than 30 days SHALL be deleted with their proposals. Deleting a collection SHALL delete its drafts. Logs MUST NOT contain the brief, the model output or the API key.

#### Scenario: Old drafts purged
- **WHEN** a draft is older than 30 days and a new draft is generated
- **THEN** the old draft and its proposals no longer exist

#### Scenario: Collection deleted
- **WHEN** an admin deletes a collection that has drafts
- **THEN** its drafts are deleted too
