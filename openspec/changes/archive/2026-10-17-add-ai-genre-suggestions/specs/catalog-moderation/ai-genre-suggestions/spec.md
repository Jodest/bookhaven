## Purpose

Gives the admin AI suggestions for cleaning up the genre vocabulary: groups of duplicate genres to merge, narrow genres to merge into a general one, and junk genres to reject. Suggestions are computed in the background and only proposed; every merge or rejection is an admin's action through the existing genre merge and bulk status.

## ADDED Requirements

### Requirement: Suggestion content and storage
The system SHALL store AI genre suggestions separately from genre data. Each suggestion holds:
- `kind`: `duplicate` (spelling, language or synonym variants of one genre), `narrow` (overly narrow genres that belong in a more general one) or `junk` (a name that is not a genre);
- its genres: for `duplicate` and `narrow`, one target and 1 to 19 sources (at most 20 genres in all); for `junk`, exactly one genre and no target;
- `reason_codes`: for `duplicate` one or more of `spelling`, `language`, `synonym`; for `narrow` exactly `too_narrow`; for `junk` one or more of `not_a_genre`, `date`, `marketing`, `format`;
- `reason`: a short Russian explanation, at most 300 characters;
- `confidence`: a number from 0 to 1;
- the model id, prompt version and analysis run that produced it, and the time it was created;
- `status`: `open`, `applied`, `dismissed` or `stale`, plus who decided and when for `applied` and `dismissed`, and the target actually used for an applied merge.

A suggestion keeps the names its genres had when it was created, so it stays readable after the genres change.

#### Scenario: Duplicate group stored
- **WHEN** the model proposes «Фентези» and «Fantasy» as duplicates of «Фэнтези» with confidence 0.95
- **THEN** one open `duplicate` suggestion is stored with target «Фэнтези», sources «Фентези» and «Fantasy», and its reason codes and Russian reason

#### Scenario: Junk genre stored
- **WHEN** the model proposes «Книги 2020» as junk with code `date`
- **THEN** one open `junk` suggestion is stored for that genre alone, without a target

### Requirement: AI only proposes
Computing suggestions MUST NOT change any genre, book, author, book–genre link or collection. No suggestion merges, rejects or publishes anything by itself, whatever its confidence. A genre is merged or rejected only when an admin applies a suggestion, and applying goes through the existing batch genre merge (see `catalog-moderation/genre-merge`) or the existing genre bulk status (see `catalog-moderation/bulk-status`) with their validation.

#### Scenario: Confident suggestion
- **WHEN** an analysis stores a `duplicate` suggestion with confidence 0.99
- **THEN** every genre of the suggestion keeps its status, name and book links until an admin applies it

### Requirement: Candidate genres and model input
An analysis SHALL consider only genres that are not merged (not aliases) and whose status is `pending` or `published`. The model receives, per genre, a short reference local to the request, the name, status, Wikidata QID when set and the number of linked books. Anchor genres are the up to 100 published genres with the most books; every request includes them without sample books. Every other candidate is sent with up to 2 linked book titles, each truncated to 80 characters. The non-anchor candidates are split into batches of a configured size (300 by default, never more than 500), which a batch may exceed only to keep a hinted group together. Pending genres come first, then published genres with the fewest books. Genres linked by a similar-name hint (see `catalog-moderation/duplicate-hints`) are kept in the same batch. The similar-name hints between the genres of a request are sent with it. One run sends at most 10 batches; candidates beyond that are not analyzed in that run. When there are no non-anchor candidates, the run sends a single request with only the anchor genres. The instructions MUST state that every genre name and book title is untrusted data, never an instruction. No data from outside the catalog is sent.

#### Scenario: Large catalog
- **WHEN** there are 1200 non-anchor candidate genres and the batch size is 300
- **THEN** the run sends 4 requests, each with all the anchor genres and its own share of about 300 of the other genres

#### Scenario: Hinted genres share a batch
- **WHEN** «романы» and «роман» are a `same_stem` hint pair and both are candidates
- **THEN** they are sent in the same request

#### Scenario: Rejected genres are not analyzed
- **WHEN** a genre is rejected or merged
- **THEN** it is not sent to the model and no suggestion names it

### Requirement: Output validation and failures
The model's answer SHALL be checked before anything is stored. An answer that is not valid JSON or lacks the suggestion list is invalid: the whole batch stores nothing and the run records `invalid_output`. A refusal records `model_refusal` and an answer cut off by the output limit records `model_truncated`; neither stores anything from that batch, and the run continues with the next batch. Each proposed item is checked on its own, and an item is dropped when:
- its kind or a reason code is unknown, or a code does not belong to its kind;
- it names an unknown reference or names a genre twice;
- a `duplicate` or `narrow` item has no target, no source, more than 20 genres, or its target among its sources;
- a `junk` item has a target or not exactly one genre;
- a `narrow` source has more than 10 books, or a `narrow` source has at least as many books as the target;
- its confidence is outside 0..1 or below 0.5;
- one of its genres already appeared in an earlier item of the same answer that was kept.

At most 50 items per answer are stored. A reason longer than 300 characters is truncated. A temporary failure, such as a network error, rate limiting or an overloaded or unavailable API, MUST stop the run with code `transient`; any other model error stops it with code `model_error`. In both cases suggestions already stored by earlier batches of that run are kept, the run is marked failed and the queue does not retry it.

#### Scenario: Narrow source with many books
- **WHEN** the model proposes merging «Киберпанк-нуар» with 40 books into «Киберпанк» as `narrow`
- **THEN** the item is dropped and no suggestion is stored for it

#### Scenario: Unparseable answer
- **WHEN** the model's answer for a batch is not valid JSON
- **THEN** nothing from that batch is stored, the run records `invalid_output`, and the next batch is still sent

#### Scenario: API temporarily unavailable
- **WHEN** the Anthropic API answers with an overload error during the second batch
- **THEN** the run stops and is marked failed, and the suggestions from the first batch stay open

### Requirement: Repeats, overlaps and dismissal memory
The set of a suggestion is its action (merge for `duplicate` and `narrow`, reject for `junk`) together with all its genres, target included. A new suggestion MUST NOT be stored when an open or dismissed suggestion has the same set, so a suggestion the admin dismissed is never proposed again for the same set of genres. A new suggestion MUST NOT be stored either when any of its genres belongs to another open suggestion. At the start of every run, open suggestions that have become stale are marked `stale`.

#### Scenario: Dismissed group comes back
- **WHEN** an admin dismissed the merge of «Фентези» into «Фэнтези» and a later run proposes the same two genres with «Фентези» as the target
- **THEN** no suggestion is stored for it

#### Scenario: Different set after a dismissal
- **WHEN** an admin dismissed the merge of «Фентези» into «Фэнтези» and a later run proposes «Фентези», «Fantasy» and «Фэнтези»
- **THEN** the new three-genre suggestion is stored

### Requirement: Staleness
An open suggestion SHALL be stale when any of its genres, the target included, no longer exists, has been merged into another genre or has been rejected. The suggestion list MUST show such suggestions as stale while they are still open. A stale suggestion MUST NOT be applied: an attempt to apply it changes no genre, marks it `stale` and is refused.

#### Scenario: Genre merged by hand
- **WHEN** an open suggestion names «Фентези» and an admin merges «Фентези» from the genre queue
- **THEN** the suggestion is listed as stale and applying it changes nothing and marks it `stale`

#### Scenario: Junk genre already rejected
- **WHEN** an open `junk` suggestion names «Бестселлер» and an admin rejects «Бестселлер» from the genre queue
- **THEN** the suggestion is stale

### Requirement: Analysis runs
Suggestions SHALL be computed only by the background worker, never while an HTTP request is being served. The model calls of a run are made one batch at a time with a minimum pause between calls, and only one run is queued or running at a time. Each run records its trigger (`manual` or `scheduled`), status (`queued`, `running`, `done` or `failed`), the number of batches planned and done, the number of suggestions stored, the last failure code, token counts and its start and finish times. When AI genre analysis is on, the worker MUST start a scheduled run on a configurable schedule (weekly by default). A scheduled run MUST do nothing when no candidate genre was added, renamed or changed status or QID since the last `done` run that recorded no failure code and the prompt version is unchanged; a `done` run with a failed batch does not count. A run that stays `queued` or `running` for more than 2 hours is treated as failed with code `interrupted`.

#### Scenario: Nothing changed since last week
- **WHEN** the weekly schedule fires and the candidate genres are the same as in the last finished run
- **THEN** no model call is made

#### Scenario: New genres harvested
- **WHEN** a harvest created 15 pending genres since the last finished run
- **THEN** the next scheduled run analyzes the catalog and may store suggestions for them

### Requirement: Switched off without configuration
AI genre analysis SHALL be off unless the deployment switches it on explicitly and provides an Anthropic API key. When off, the worker MUST NOT schedule or run an analysis and no model call is made. The analyze endpoint MUST then return 503 `{"error": "AI genre analysis is disabled"}`. Existing suggestions stay visible, and applying and dismissing them keep working. The model id is configurable, and the default is `claude-sonnet-5-5`.

#### Scenario: Local stack without a key
- **WHEN** the stack runs without an Anthropic API key
- **THEN** the status endpoint reports `enabled: false` and `POST /api/v1/admin/genres/ai-suggestions/analyze` returns 503

### Requirement: Analysis trigger endpoint
The system SHALL accept `POST /api/v1/admin/genres/ai-suggestions/analyze` with an empty body or `{}`. It queues one manual run over the candidate genres, whether or not they changed, and answers 202 `{run_id}`. When a run is already queued or running, it MUST return 409 `{"error": "AI genre analysis already running"}` and queue nothing. A failure to queue MUST return 502 and mark the run failed. The endpoint MUST require the `admin` role and a bearer token, and a moderator MUST get 403.

#### Scenario: Manual analysis
- **WHEN** an admin calls the analyze endpoint while no run is active
- **THEN** the response is 202 with the run id and the run is computed in the background

#### Scenario: Second analysis while one runs
- **WHEN** an admin calls the analyze endpoint while a run is still running
- **THEN** the response is 409 and nothing is queued

#### Scenario: Moderator is refused
- **WHEN** a moderator calls `POST /api/v1/admin/genres/ai-suggestions/analyze`
- **THEN** the response is 403 and nothing is queued

### Requirement: Status endpoint
The system SHALL expose `GET /api/v1/admin/genres/ai-suggestions/status`, returning `{enabled, model, prompt_version, open_count, last_run}`. `open_count` is the number of open suggestions, stale ones included. `last_run` is `null` when no run exists, otherwise the latest run as `{id, trigger, status, batches_total, batches_done, suggestions_created, error_code, created_at, finished_at}`. The endpoint MUST require the `admin` role, and a moderator MUST get 403.

#### Scenario: Never analyzed
- **WHEN** no analysis has ever run
- **THEN** the status returns `last_run: null` and `open_count: 0`

### Requirement: Suggestion list endpoint
The system SHALL expose `GET /api/v1/admin/genres/ai-suggestions` with the query parameters `status` (`open` by default, or `applied`, `dismissed`, `stale`), `kind` (`duplicate`, `narrow` or `junk`, optional), `page` and `limit` (20 by default, at most 50). An unknown `status` or `kind` MUST return 400. The response is `{items, total}`. Open suggestions are ordered by confidence, highest first, then newest first, and the others newest decision first. Each item carries `{id, kind, status, reason_codes, reason, confidence, model, prompt_version, created_at, decided_at, stale, target, genres, applied_target}`:
- `target` is the suggested target, or `null` for `junk`;
- `genres` lists every genre of the suggestion, target first, as `{id, name, status, merged, exists, wikidata_qid, books_total, sample_books}` with the current values. A genre that no longer exists shows its stored name with `exists: false`;
- `stale` is computed at read time for open items;
- `applied_target` is the target used for an applied merge, or `null`.

The endpoint MUST require the `admin` role, and a moderator MUST get 403.

#### Scenario: Kind filter
- **WHEN** an admin lists open suggestions with `kind=junk`
- **THEN** only open `junk` suggestions are returned

#### Scenario: Unknown kind
- **WHEN** an admin lists suggestions with `kind=other`
- **THEN** the response is 400

### Requirement: Apply endpoint
The system SHALL accept `POST /api/v1/admin/genres/ai-suggestions/{id}/apply` with an optional body `{"target_id"}`. The rules are:
- An unknown id MUST return 404 `{"error": "suggestion not found"}`.
- A suggestion that is not open MUST return 409 `{"error": "suggestion is not open"}`.
- A stale suggestion MUST be marked `stale`, change no genre and return 409 `{"error": "suggestion is stale"}`.
- For a `duplicate` or `narrow` suggestion, the chosen target is `target_id` when given, otherwise the suggested target. When the chosen target is one of the suggestion's genres, the sources are all other genres of the suggestion. When it is another genre, every genre of the suggestion is a source. The sources and the chosen target are merged through the batch genre merge in one transaction, with its 400 and 404 rules, so a missing or merged chosen target returns 404 and nothing changes.
- For a `junk` suggestion, `target_id` MUST return 400, and the genre is set to `rejected` through the genre bulk status.
- A non-UUID id or `target_id` MUST return 400.
- On success the suggestion becomes `applied` with the deciding admin, the time and the used target, and the response is 200 `{suggestion, merge, status_report}`: `merge` is the batch merge result `{genre, sources_merged, books_reassigned, collections_switched}` for a merge and `null` otherwise, and `status_report` is the bulk status report for a `junk` suggestion and `null` otherwise.

The endpoint MUST require the `admin` role and a bearer token, and a moderator MUST get 403.

#### Scenario: Apply a duplicate suggestion
- **WHEN** an admin applies an open suggestion to merge «Фентези» and «Fantasy» into «Фэнтези»
- **THEN** both genres become aliases of «Фэнтези» in one transaction, their curated collections switch to it, and the suggestion is `applied`

#### Scenario: Target changed by the admin
- **WHEN** an admin applies the suggestion «Фентези», «Fantasy» → «Фэнтези» with `target_id` of «Fantasy»
- **THEN** «Фентези» and «Фэнтези» are merged into «Fantasy» and the suggestion records «Fantasy» as the used target

#### Scenario: Apply a junk suggestion
- **WHEN** an admin applies an open `junk` suggestion for «Бестселлер»
- **THEN** «Бестселлер» becomes `rejected`, its book links stay, and the suggestion is `applied`

#### Scenario: Stale on apply
- **WHEN** an admin applies a suggestion whose source was rejected after the analysis
- **THEN** the response is 409, no genre changes and the suggestion becomes `stale`

#### Scenario: Missing chosen target
- **WHEN** an admin applies a merge suggestion with a `target_id` that does not exist
- **THEN** the response is 404 and nothing changes

#### Scenario: Moderator is refused
- **WHEN** a moderator calls `POST /api/v1/admin/genres/ai-suggestions/{id}/apply`
- **THEN** the response is 403 and nothing changes

### Requirement: Bulk apply endpoint
The system SHALL accept `POST /api/v1/admin/genres/ai-suggestions/apply` with `{"items": [{"id", "target_id"?}]}`. The request MUST return 400, changing nothing, when the body is malformed, `items` is empty, there are more than 50 items, an id is listed twice or any id is not a UUID. Each item MUST be applied exactly as the apply endpoint would, one after another in request order. Each merge has its own transaction, so a failed item does not undo earlier ones, and a later item made stale by an earlier one is marked `stale`. The response is 200 `{processed, applied, stale, errors}`, where each error is `{suggestion_id, message}` for an item that was not found, not open or failed. The endpoint MUST require the `admin` role and a bearer token, and a moderator MUST get 403.

#### Scenario: Overlapping items
- **WHEN** an admin bulk-applies two suggestions and the first one merges a genre that the second one also names
- **THEN** the first is applied, the second becomes `stale`, and the report shows `applied: 1, stale: 1`

### Requirement: Dismiss endpoints
The system SHALL accept `POST /api/v1/admin/genres/ai-suggestions/{id}/dismiss` and `POST /api/v1/admin/genres/ai-suggestions/dismiss` with `{"ids": string[]}` (1 to 50 UUIDs, deduplicated, otherwise 400). Dismissing changes no genre. It sets an open suggestion, stale or not, to `dismissed` with the deciding admin and time. The single endpoint returns 200 with the suggestion, 404 for an unknown id and 409 `{"error": "suggestion is not open"}` for a suggestion that is not open. The bulk endpoint returns 200 `{processed, dismissed, errors}` with the same per-id errors. Both MUST require the `admin` role and a bearer token, and a moderator MUST get 403.

#### Scenario: Dismiss a suggestion
- **WHEN** an admin dismisses an open suggestion
- **THEN** it becomes `dismissed`, no genre changes, and later runs never propose the same set of genres again

### Requirement: Suggestions page
The admin SHALL have the page `/moderation/genres/ai-suggestions` titled «Предложения ИИ по жанрам», for admins only, inside the «Жанры» tab of «Модерация». The page shows:
- the last run: «Последний анализ: {date}», with «идёт…» while a run is queued or running and «завершился с ошибкой» for a failed run;
- the status tabs «Открытые», «Применённые», «Отклонённые» and «Устаревшие»;
- the kind filter «Все типы», «Дубликаты», «Узкий → общий» and «Не жанр»;
- a table with checkboxes and pagination. Tab, filter and page are kept in the URL.

Each row shows a kind badge («Дубликат», «Узкий → общий» or «Не жанр»), and every genre with its status, «книг: {books_total}» and its sample books. The action is «Слить в «{target}»» for a merge or «Отклонить жанр» for junk. The row also shows the reason with the Russian labels of its codes («написание», «другой язык», «синоним», «слишком узкий», «не жанр», «дата», «маркетинг», «формат издания») and the confidence in percent. An open stale row shows «устарело» and its «Применить» is disabled. Open rows offer «Применить» and «Отклонить». When AI genre analysis is on, the page offers «Запустить анализ», which asks «Запустить ИИ-анализ жанров? Это займёт несколько минут.». After queueing, the page shows «Анализ поставлен в очередь. Предложения появятся через несколько минут.», and a 409 shows «Анализ уже идёт.». When it is off, the button is hidden and the page shows «ИИ-анализ жанров выключен.», while existing suggestions and their actions remain. An empty open tab shows «Открытых предложений нет».

#### Scenario: Stale row
- **WHEN** an open suggestion is stale
- **THEN** its row shows «устарело» and «Применить» is disabled while «Отклонить» stays available

#### Scenario: Switched off
- **WHEN** the status endpoint reports `enabled: false`
- **THEN** «Запустить анализ» is not shown and open suggestions can still be applied

### Requirement: Apply and dismiss dialogs
«Применить» on a merge suggestion SHALL open the dialog «Применить предложение: слить жанры ({N})». It lists the suggestion's genres with a radio button each, the suggested target preselected, plus «Другой жанр…» with the name search «Поиск жанра по названию». The dialog explains that the books move to the target and the sources disappear from the lists while their names and QIDs keep pointing to the target, and that the action is irreversible. It shows the rejected-target warning and the merge preview summary exactly as the batch merge dialog does (see `catalog-moderation/genre-merge`). «Слить {sources} в «{target}»» is enabled only once the preview has loaded. «Применить» on a junk suggestion asks «Отклонить жанр «{name}»? Связи с книгами ({books_total}) сохранятся, жанр исчезнет со страниц книг.» before applying. The success messages are:
- after a merge, «Жанры слиты в «{target}». Перенесено книг: {books_reassigned}.», followed by « Переключено подборок: {collections_switched}.» when that is above zero;
- after a junk apply, «Жанр «{name}» отклонён.»

A 409 for a stale suggestion shows «Предложение устарело: жанры изменились после анализа.», and a 404 for the chosen target says that the genre was not found and may already be merged. Any other error shows «Не удалось применить предложение. Попробуйте позже.». «Отклонить» asks «Отклонить предложение? ИИ больше не предложит его для тех же жанров.» and then shows «Предложение отклонено.».

#### Scenario: Preview before applying
- **WHEN** an admin opens «Применить» on a merge suggestion whose source is linked to the collection «Лучшее фэнтези»
- **THEN** the dialog shows «Подборка «Лучшее фэнтези» будет переключена на жанр «{target}».» before the merge button can be clicked

#### Scenario: Admin picks another target
- **WHEN** an admin chooses «Другой жанр…» and picks «Фантастика»
- **THEN** the preview is recomputed with every genre of the suggestion as a source of «Фантастика», and applying sends that `target_id`

### Requirement: Bulk actions on the page
For the selected open rows the page SHALL offer «Применить выбранные (N)» and «Отклонить выбранные (N)». «Применить выбранные (N)» opens a confirmation that lists every selected suggestion with its action. Each merge suggestion shows its merge preview summary for its suggested target. The confirm button «Применить (N)» is enabled once every preview has loaded or failed, and a failed preview is shown on its item. The selected suggestions are sent to the bulk apply endpoint with the suggested targets. Afterwards the selection is cleared and the page shows «Применено: {applied}. Устарело: {stale}. Ошибок: {errors}.». «Отклонить выбранные (N)» asks «Отклонить выбранные предложения (N)? ИИ больше не предложит их для тех же жанров.» and shows «Отклонено предложений: {dismissed}.».

#### Scenario: Apply a selection
- **WHEN** an admin selects three open suggestions and confirms «Применить (3)»
- **THEN** the three ids are sent to the bulk apply endpoint and the report message is shown

### Requirement: Entry from the genre queue
The genre moderation queue `/moderation/genres` SHALL show the link «Предложения ИИ ({open_count})» to `/moderation/genres/ai-suggestions` when AI genre analysis is on or there are open suggestions.

#### Scenario: Open suggestions exist
- **WHEN** there are 4 open suggestions
- **THEN** the genre queue shows «Предложения ИИ (4)» linking to the suggestions page
