# catalog-admin/genre-editing Specification

## Purpose

Defines how an admin creates and edits a genre and its Wikidata QID.

## Requirements

### Requirement: Create and update endpoints
`POST /api/v1/admin/genres` and `PATCH /api/v1/admin/genres/{id}` SHALL require the `admin` role and a bearer header, with `{name, slug, status, wikidata_qid}`. A created genre is always published; on update an omitted `status` keeps it. On update an omitted `wikidata_qid` keeps the QID and an empty one removes it. `GET /api/v1/admin/genres/{id}` returns a genre of any status; an unknown or merged genre returns 404 on update.

#### Scenario: Removing the QID
- **WHEN** an admin saves a genre with `wikidata_qid: ""`
- **THEN** the genre has no QID

### Requirement: Genre data rules
The name SHALL be non-empty. On create the first letter of the name is lowercased («Фэнтези» becomes «фэнтези»); an update stores the name as given. The slug is taken from the request or transliterated from the name, MUST contain only letters, digits and hyphens and MUST be unique (422 for an unsafe slug, 409 when taken). The QID is uppercased and MUST look like `Q123` (400 otherwise) and MUST NOT belong to another genre (409).

#### Scenario: QID of another genre
- **WHEN** an admin sets a QID that another genre already has
- **THEN** the response is 409 and nothing changes

#### Scenario: Name case on create
- **WHEN** an admin creates the genre «Фэнтези»
- **THEN** it is stored as «фэнтези»

### Requirement: Genre edit page
The admin app SHALL provide `/catalog/genres/{id}/edit` «Редактирование жанра» with «Название *», «Slug» («Автоматически из названия»), «Статус» and a QID field («Например, Q8261», «QID должен иметь вид Q123»). After saving, the admin returns to the list they came from. An unknown genre shows «Жанр не найден.».

#### Scenario: Malformed QID
- **WHEN** an admin enters «8261» as the QID
- **THEN** the form shows «QID должен иметь вид Q123»
