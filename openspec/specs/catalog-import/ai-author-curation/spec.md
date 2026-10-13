# catalog-import/ai-author-curation Specification

## Purpose
Lets an admin describe in free text which authors the catalog is missing. An AI model proposes author names, the system resolves each name against Wikidata, and the admin picks candidates and takes them through the existing author import and book harvest flows. The AI never supplies catalog data, and nothing is published automatically.

## Requirements

### Requirement: Feature switch and status
AI curation SHALL be on only when deployment configuration enables it and an Anthropic API key is set. It is off by default, and its switch is separate from AI pre-moderation. The system SHALL expose `GET /api/v1/admin/ai-curator/status`, which returns 200 with:
- `enabled`, `model` and `prompt_version`;
- the limits `max_prompt_length` (500), `max_names` (30), `default_names` (20) and `daily_limit` (20);
- for the calling admin, `used_today` (requests created in the last 24 hours) and `active_request_id` (the admin's active request, or `null`).

The endpoint MUST require the `admin` role. When the feature is off, reading existing requests MUST keep working.

#### Scenario: Flag without key
- **WHEN** the curation flag is on but no API key is configured
- **THEN** the status reports `enabled: false` and creating a request answers 503

#### Scenario: Moderator
- **WHEN** a moderator calls any `/api/v1/admin/ai-curator/` endpoint
- **THEN** the response is 403

### Requirement: Create a curation request
The system SHALL accept `POST /api/v1/admin/ai-curator/requests` with `{"prompt": string, "max_names"?: number}`. It MUST require the `admin` role and a bearer token. The checks run in this order:
1. Feature off → 503.
2. Prompt empty after trimming, or longer than 500 characters → 400.
3. `max_names` outside 1..30 → 400. A missing `max_names` means 20.
4. The admin already has an active (queued or running) request → 409 with that request's id.
5. The admin created 20 requests in the last 24 hours, including failed ones → 429.

On success the request is stored as `queued` with its creator, and one background task is queued. The response is 202 with the request. If queuing fails, the request is stored as `failed` with error code `enqueue_failed` and the response is 502. The model MUST NOT be called during the HTTP request.

#### Scenario: Valid request
- **WHEN** an admin submits `{"prompt": "русская классика XIX века"}`
- **THEN** the response is 202 with a `queued` request for 20 names, created by that admin

#### Scenario: Second request while one is active
- **WHEN** an admin submits a request while their previous request is still running
- **THEN** the response is 409 and no new request is stored

#### Scenario: Daily limit
- **WHEN** an admin who created 20 requests in the last 24 hours submits another
- **THEN** the response is 429

#### Scenario: Prompt too long
- **WHEN** an admin submits a prompt of 501 characters
- **THEN** the response is 400 and nothing is stored or queued

### Requirement: Asynchronous processing
A curation request SHALL be processed in the background worker, one request at a time. The status goes from `queued` to `running` to `done` or `failed`. The worker MUST call the model at most once per request: a retried task that already has stored candidates resumes the Wikidata resolution and makes no second model call. A request that reached `done` or `failed` MUST NOT be processed again. While the request runs, its candidates and progress MUST become readable as they are stored. A request that has stayed `queued` or `running` without progress for more than one hour MUST be reported as `failed` with error code `timeout`, and it no longer counts as the admin's active request.

#### Scenario: Retry after a Wikidata outage
- **WHEN** the worker stored 30 names, resolved 12, and then Wikidata became unavailable, so the task is retried
- **THEN** the retry resolves the remaining 18 names and does not call the model again

#### Scenario: Stuck request
- **WHEN** a request has been `running` without progress for 2 hours
- **THEN** it is shown as `failed` with `timeout` and the admin can create a new request

### Requirement: Model output is names only
The model SHALL get the admin's prompt as data inside a fixed instruction, and it SHALL answer in a fixed structured format: a list of objects, each with an author name and a short note. Any other shape MUST be rejected.

The system MUST clean the list before storing it:
- trim names and collapse whitespace;
- drop empty names and names longer than 150 characters;
- drop case-insensitive duplicates;
- keep at most `max_names` names, in the model's order;
- cut each note to 200 characters.

The model's answer MUST NOT be written into any catalog field. Names and notes are display-only and MUST be shown as plain text. Failures end the request as `failed` with no candidates:
- a refusal → `model_refusal`;
- a truncated answer → `model_truncated`;
- an answer that breaks the format → `invalid_output`;
- a model call that fails after retries → `model_error`.

An empty list ends the request as `done` with no candidates.

#### Scenario: Oversized answer
- **WHEN** the request asks for 10 names and the model returns 14 valid names
- **THEN** only the first 10 are stored as candidates

#### Scenario: Markup in a note
- **WHEN** a note contains `<b>Хьюго</b>`
- **THEN** the page shows the literal text `<b>Хьюго</b>` and renders no markup

#### Scenario: Refusal
- **WHEN** the model refuses to answer
- **THEN** the request is `failed` with error code `model_refusal` and no candidates

### Requirement: Wikidata resolution of proposed names
Each stored name SHALL be looked up with the same Wikidata author search the import pages use, through the same rate-limited Wikidata access as harvests. Only results that pass both checks MUST be kept as options:
- the entity is a human, pseudonym or collective pseudonym (instance of `Q5`, `Q61002` or `Q16017119`);
- the entity has a Russian label that passes the direct author import's Russian-label check.

Each option carries `{qid, label, description}`, where the label is the Russian label, and options are deduplicated by canonical QID. The candidate's match status MUST be one of:
- `matched`: exactly one option;
- `ambiguous`: two or more options;
- `not_found`: no options;
- `error`: Wikidata stayed unavailable after the task's retries;
- `pending`: not yet resolved.

The system MUST NOT pick an option automatically, even for `matched`.

#### Scenario: Book with the author's name
- **WHEN** the search for a proposed name returns one writer and one novel titled after them
- **THEN** only the writer is kept and the candidate is `matched`

#### Scenario: Invented name
- **WHEN** the model proposes a name that Wikidata does not know
- **THEN** the candidate is `not_found` with no options and nothing is imported

#### Scenario: Two namesakes
- **WHEN** a proposed name matches two people with Russian labels
- **THEN** the candidate is `ambiguous` with both options and neither is selected

### Requirement: Reading requests
The system SHALL expose two endpoints. Both MUST require the `admin` role.
- `GET /api/v1/admin/ai-curator/requests` returns the 20 most recent requests of all admins, newest first. Each item carries `{id, prompt, max_names, status, error_code, model, prompt_version, created_by: {id, username} | null, created_at, finished_at, candidates_total, candidates_resolved}`.
- `GET /api/v1/admin/ai-curator/requests/{id}` returns the same fields plus the token counts and `candidates`, in the model's order. Each candidate is `{id, position, name, note, match_status, options}`. Each option is `{qid, label, description, existing}`. A malformed id returns 400 and an unknown id returns 404.

`existing` MUST be computed when the request is read. It is the catalog author that now carries that QID (`{id, name, slug, status}`), or `null`, so markers reflect imports made after the request finished.

#### Scenario: Author imported after the request
- **WHEN** an admin imports an option's author and reloads the request
- **THEN** that option's `existing` carries the new pending author

### Requirement: Retention
Curation requests and their candidates SHALL be deleted once they are older than 30 days. Catalog data created from them MUST NOT be affected.

#### Scenario: Old request
- **WHEN** a request was created 31 days ago
- **THEN** it no longer appears in the list and its detail returns 404

### Requirement: Curation page
The page `/import/authors/ai` («Подбор ИИ») SHALL contain:
- a prompt field with a character counter (limit 500);
- a «Сколько авторов» number field (1–30, default 20);
- a «Подобрать авторов» button;
- the list «Недавние подборки».

When the feature is off, the page MUST say «Подбор ИИ выключен» and offer no submit action. Submit is disabled while the admin has an active request. 409, 429, 503 and 502 MUST each produce a readable Russian message.

A selected request MUST be bookmarkable (`?request=<id>`). While the request is queued or running, the page polls it every 3 seconds and shows the progress «Проверено в Wikidata: X из Y». Each candidate shows:
- the proposed name and note as plain text, with the note marked as the AI's opinion;
- a match badge: «Найден», «Неоднозначно», «Не найден», «Ошибка Wikidata» or «Проверяется…»;
- its options, each in the same form as the Wikidata author import (label, description or «без описания», a Wikidata link in a new tab, and the catalog status with a link when the author is already in BookHaven).

A failed request shows a readable message for its error code.

#### Scenario: Feature off
- **WHEN** an admin opens «Подбор ИИ» while AI curation is switched off
- **THEN** the page says «Подбор ИИ выключен», the submit button is absent, and recent requests are still listed

#### Scenario: Request in progress
- **WHEN** an admin submits a prompt
- **THEN** the page shows the new request and its progress, and candidates appear as they are resolved

### Requirement: Taking candidates through existing flows
For each option, the page SHALL offer the same actions as the Wikidata author import:
- «Импортировать автора» or «Импортировать с книгами» for an author not in the catalog;
- «Импортировать книги» for a pending or published author;
- no action and «Отклонён ранее» for a rejected author.

Options not rejected MAY be ticked. Nothing is ticked by default. The «Выбрать все найденные» button ticks the single option of every `matched` candidate that is not yet in the catalog.

The bulk actions «Импортировать выбранных» and «Импортировать выбранных с книгами» take an optional «Включить автопополнение книг» checkbox. They MUST ask for confirmation, then import each ticked option through the existing author import, one at a time, showing progress.
- «с книгами» then queues one harvest job for the QIDs of every import whose outcome was `created` or `existing`.
- With «Включить автопополнение книг», each such author not yet flagged is flagged for automatic harvesting through the existing author update.

A `rejected` outcome or an import error MUST NOT start a harvest or set the flag, and is reported per author. Imported authors and harvested books MUST stay `pending`. Nothing in this flow publishes anything.

#### Scenario: Bulk import with books and auto-harvest
- **WHEN** an admin ticks three new authors, enables «Включить автопополнение книг» and confirms «Импортировать выбранных с книгами»
- **THEN** three pending authors are created, one harvest job for their three QIDs is queued, and each author is flagged for automatic harvesting

#### Scenario: Rejected outcome in a batch
- **WHEN** one of the ticked authors matches a rejected catalog author
- **THEN** that author is reported as rejected, is not harvested or flagged, and the other authors are processed

#### Scenario: Nothing preselected
- **WHEN** a request finishes with 15 `matched` candidates
- **THEN** no option is ticked until the admin ticks options or presses «Выбрать все найденные»
