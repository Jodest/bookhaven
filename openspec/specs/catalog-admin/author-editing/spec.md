# catalog-admin/author-editing Specification

## Purpose

Defines how an admin creates and edits an author, including dates and a photo with its attribution.

## Requirements

### Requirement: Create and update endpoints
`POST /api/v1/admin/authors` and `PATCH /api/v1/admin/authors/{id}` SHALL require the `admin` role and a bearer header, with `{name, slug, biography, photo_url, birth_date, death_date, status}`. A created author is always published (`status` is ignored on create); on update an omitted `status` keeps it. An update replaces every field; an omitted or empty date clears it. The author's Wikidata QID is never changed here. `GET /api/v1/admin/authors/{id}` returns an author of any status; an unknown author returns 404.

#### Scenario: Clearing a date
- **WHEN** an admin saves an author without `death_date`
- **THEN** the author has no death date

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
The admin app SHALL provide `/catalog/authors/{id}/edit` «Редактирование автора» with «Имя *», «Slug» («Автоматически из имени»), «Биография», «URL фото» (with the warning «При смене фото атрибуция будет удалена»), «Дата рождения», «Дата смерти» and «Статус». After saving, the admin returns to the list they came from. An unknown author shows «Автор не найден.».

#### Scenario: Saving an author
- **WHEN** an admin corrects an author's biography and saves
- **THEN** they are back on the author list they came from
