## Purpose

Shows readers the screen adaptations of a book on its public page and lets admins hide a wrong adaptation link without deleting it.

## ADDED Requirements

### Requirement: Public adaptations endpoint
`GET /api/v1/books/{id}/adaptations` SHALL be open to everyone without a token. It returns 200 with `{items, total}`, the book's visible adaptations without pagination. Each item is `{id, title, original_title, kind, year, director, imdb_url, kinopoisk_url}`:
- `kind` is one of `film`, `tv_film`, `series`, `animated_film`, `animated_series` and `teleplay`.
- Fields without a value are `null`.
- `imdb_url` is `https://www.imdb.com/title/<imdb id>/` and `kinopoisk_url` is `https://www.kinopoisk.ru/film/<kinopoisk id>/`.

Items are ordered by year, oldest first, with works without a year last, then by title. A link hidden by an admin MUST NOT be returned. A malformed id returns 400 «invalid book id». An unknown book, or a book that is not published, returns 404 «book not found».

#### Scenario: Visible adaptations
- **WHEN** a published book has a film from 1994 linked, a series from 1972 linked and a hidden film linked
- **THEN** the response has `total: 2`, with the 1972 series first, and the hidden film is absent

#### Scenario: Pending book
- **WHEN** anyone requests the adaptations of a pending book
- **THEN** the response is 404

#### Scenario: Book without adaptations
- **WHEN** a published book has no visible adaptation
- **THEN** the response is 200 with `items: []` and `total: 0`

### Requirement: Adaptations block on the book page
The public book page SHALL show a block «Экранизации» when the book has at least one visible adaptation. Each adaptation shows its title, its year, its type and «Режиссёр: <names>» when directors are known. The type is shown as «Фильм», «Телефильм», «Сериал», «Мультфильм», «Мультсериал» or «Телеспектакль». The adaptation also shows the links «Кинопоиск» and «IMDb» for the ids it has, opening in a new tab. No poster is shown. When there is no visible adaptation, or the adaptations cannot be loaded, the block MUST NOT be rendered and the rest of the page is unaffected.

#### Scenario: Book with adaptations
- **WHEN** a reader opens a published book with two visible adaptations, one with a Kinopoisk id and an IMDb id and one with neither
- **THEN** the block «Экранизации» lists both, the first one with the links «Кинопоиск» and «IMDb» and the second one without links

#### Scenario: Nothing to show
- **WHEN** a reader opens a book whose only adaptation is hidden
- **THEN** the page has no «Экранизации» block

### Requirement: Admin adaptation links
`GET /api/v1/admin/books/{id}/adaptations` SHALL require the `admin` role and return, for a book of any status, every linked adaptation including hidden ones. Each item has the public fields plus `hidden`, `source` (`wikidata` or `manual`), `qid` (the work's Wikidata QID or `null`), `imdb_id` and `kinopoisk_id`. `PATCH /api/v1/admin/books/{id}/adaptations/{screen_work_id}` with `{"hidden": true|false}` SHALL require the `admin` role and a bearer header. It sets the link's visibility and returns 200 with the updated item. A missing `hidden` or a malformed id returns 400. An unknown book or a link that does not exist returns 404. Hiding never deletes the link or the work, so a later sync cannot bring it back (see `catalog-import/wikidata-adaptation-sync`). A moderator gets 403 on both endpoints.

#### Scenario: Hide a link
- **WHEN** an admin sends `{"hidden": true}` for a linked adaptation of a published book
- **THEN** the response is 200 with `hidden: true` and the public endpoint no longer returns that adaptation

#### Scenario: Show again
- **WHEN** an admin sends `{"hidden": false}` for a hidden link
- **THEN** the adaptation is visible on the public book page again

#### Scenario: Unknown link
- **WHEN** an admin patches an adaptation that is not linked to the book
- **THEN** the response is 404

### Requirement: Adaptations section on the book edit page
The admin book edit page SHALL show a section «Экранизации» listing every linked adaptation with its title, year, type and directors. Hidden ones carry the mark «Скрыта». Each row has «Скрыть», or «Показать» for a hidden one, which applies at once and refreshes the list. The section offers «Найти экранизации в Wikidata» when the book has a Wikidata QID. With no links it shows «Экранизаций пока нет.». It shows «Не удалось загрузить экранизации.» when the list fails to load and «Не удалось изменить видимость экранизации.» when hiding or showing fails.

#### Scenario: Hiding from the edit page
- **WHEN** an admin clicks «Скрыть» on an adaptation in the section
- **THEN** the row shows «Скрыта» and the button becomes «Показать»
