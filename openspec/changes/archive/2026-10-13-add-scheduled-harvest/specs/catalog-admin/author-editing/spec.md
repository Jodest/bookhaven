## MODIFIED Requirements

### Requirement: Create and update endpoints
`POST /api/v1/admin/authors` and `PATCH /api/v1/admin/authors/{id}` SHALL require the `admin` role and a bearer header. The body is `{name, slug, biography, photo_url, birth_date, death_date, status}`; `PATCH` also accepts an optional boolean `auto_harvest`. A created author is always published (`status` is ignored on create) and never flagged for automatic harvesting (`auto_harvest` is ignored on create). On update an omitted `status` or `auto_harvest` keeps the current value. Setting `auto_harvest: true` on an author without a Wikidata QID MUST return 422 and change nothing. Clearing the flag is always allowed. Apart from `status` and `auto_harvest`, an update replaces every field, and an omitted or empty date clears it. The author's Wikidata QID is never changed here. Admin author payloads include `auto_harvest` (boolean) and `last_auto_harvested_at` (timestamp or null, read-only). `GET /api/v1/admin/authors/{id}` returns an author of any status; an unknown author returns 404.

#### Scenario: Clearing a date
- **WHEN** an admin saves an author without `death_date`
- **THEN** the author has no death date

#### Scenario: Flagging an author with a QID
- **WHEN** an admin saves an author that has Wikidata QID `Q7200` with `auto_harvest: true`
- **THEN** the response carries `auto_harvest: true` and the author becomes eligible for scheduled harvesting once published

#### Scenario: Flagging an author without a QID
- **WHEN** an admin saves an author without a Wikidata QID with `auto_harvest: true`
- **THEN** the response is 422 and nothing changes

#### Scenario: Omitted flag
- **WHEN** an admin saves a flagged author's biography without sending `auto_harvest`
- **THEN** the author stays flagged

### Requirement: Author edit page
The admin app SHALL provide `/catalog/authors/{id}/edit` «Редактирование автора» with «Имя *», «Slug» («Автоматически из имени»), «Биография», «URL фото» (with the warning «При смене фото атрибуция будет удалена»), «Дата рождения», «Дата смерти», «Статус» and a checkbox «Автоматически пополнять книги из Wikidata». Under the checkbox the page shows «Последний автоматический харвест: <date>», or «Ещё не выполнялся». For an author without a Wikidata QID the checkbox is disabled and shows «Нужен Wikidata QID». After saving, the admin returns to the list they came from. An unknown author shows «Автор не найден.».

#### Scenario: Saving an author
- **WHEN** an admin corrects an author's biography and saves
- **THEN** they are back on the author list they came from

#### Scenario: Author without a QID
- **WHEN** an admin opens the edit page of an author without a Wikidata QID
- **THEN** the checkbox «Автоматически пополнять книги из Wikidata» is disabled with the hint «Нужен Wikidata QID»

#### Scenario: Enabling automatic harvesting
- **WHEN** an admin ticks «Автоматически пополнять книги из Wikidata» on an author with a QID and saves
- **THEN** the author is saved with `auto_harvest: true`
