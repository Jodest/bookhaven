# catalog-admin/author-editing Specification

## Purpose

Defines how an admin creates and edits an author, including dates and a photo with its attribution.

## Requirements

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

### Requirement: Author data rules
The name SHALL be non-empty. Dates MUST be `YYYY-MM-DD`. The slug is taken from the request or transliterated from the name, MUST contain only letters, digits and hyphens and MUST be unique. A photo link that does not start with `http://` or `https://` is dropped without an error. A rule violation returns 422, a taken slug 409 and an invalid status 400.

#### Scenario: Bad date
- **WHEN** an admin saves `birth_date: "12.05.1828"`
- **THEN** the response is 422 and nothing changes

### Requirement: Photo attribution
An author photo filled from Wikimedia Commons SHALL carry its attribution (author, license, license link, source page), returned as `photo_attribution`. When an update changes the photo link, the attribution MUST be removed; saving the same link keeps it.

#### Scenario: Replacing an enriched photo
- **WHEN** an admin replaces a Commons photo with another link
- **THEN** the author has the new photo and no attribution

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
