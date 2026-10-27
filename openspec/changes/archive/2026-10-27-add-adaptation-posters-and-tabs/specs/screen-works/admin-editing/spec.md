## MODIFIED Requirements

### Requirement: Work detail and update endpoints
`GET /api/v1/admin/screen-works/{id}` SHALL require the `admin` role. It returns the list item fields plus `books`, `authors`, `poster` and `poster_removed`:
- each book is `{id, title, status, hidden, source}`;
- each author is `{id, name, status, hidden, source, relation}`;
- `poster` has the shape of the public adaptation item's `poster` (see `screen-works/book-adaptations`), or `null`;
- `poster_removed` is `true` when an admin removed an automatic poster, so the sync does not fill one again, and `false` otherwise.

`PATCH /api/v1/admin/screen-works/{id}` SHALL require the `admin` role and a bearer header. The body carries the full data (`title`, `original_title`, `kind`, `year`, `director`, `imdb_id`, `kinopoisk_id`), and an update replaces every field. `qid` is optional:
- an omitted `qid`, or one equal to the stored QID, keeps it;
- a work without a QID may get one. If another work has that QID, the response is 409 «screen work with this qid already exists»;
- a work with a QID MUST NOT change or lose it: the response is 422 «qid cannot be changed».

Every successful update marks the work as edited by hand (`manually_edited_at`) and returns 200 with the detail. The update never changes the poster. The visibility and source of the work's links do not change. An unknown work returns 404 «screen work not found». A malformed id returns 400. A moderator gets 403 on both endpoints.

#### Scenario: Fix a title
- **WHEN** an admin changes the title of a work that is linked to a published book
- **THEN** the response is 200 with `manually_edited_at` set, and the public book page shows the new title

#### Scenario: Add a QID
- **WHEN** an admin sets the QID of a manual work that has none and no other work has it
- **THEN** the work keeps its id and links, and a later sync of a book that Wikidata lists for that QID links this work instead of creating another

#### Scenario: Change a QID
- **WHEN** an admin sends a different QID for a work that already has one
- **THEN** the response is 422 «qid cannot be changed» and nothing changes

#### Scenario: Moderator
- **WHEN** a moderator requests a work's detail
- **THEN** the response is 403

#### Scenario: Update keeps the poster
- **WHEN** an admin saves new data for a work with a TMDB poster
- **THEN** the detail still has that poster

## ADDED Requirements

### Requirement: Poster upload and removal endpoints
`PUT /api/v1/admin/screen-works/{id}/poster` SHALL require the `admin` role and a bearer header. It takes `multipart/form-data` with one image in the field `file`. The image is at most 5 MiB and is a JPEG, PNG or WebP image, judged by its content, not its name. The upload replaces any current poster, automatic or uploaded, and clears `poster_removed`. The work then has a poster with source `manual`, served from the site under `/posters/`. The response is 200 with the work detail.

`DELETE /api/v1/admin/screen-works/{id}/poster` SHALL require the `admin` role and a bearer header and returns 200 with the work detail:
- An uploaded poster is removed. The work has no poster, and a later sync MAY fill an automatic one.
- An automatic poster (Commons or TMDB) is removed and `poster_removed` becomes `true`. The sync MUST NOT fill a poster for that work again until an admin uploads one.
- A work without a poster is left unchanged.

Uploading or removing a poster MUST NOT mark the work as edited by hand and MUST NOT change its data or links.

Errors:
- no `file` returns 400 «file is required»;
- a file over 5 MiB returns 413 «poster is too large»;
- any other content returns 422 «poster must be a JPEG, PNG or WebP image»;
- a malformed id returns 400;
- an unknown work returns 404 «screen work not found»;
- no token returns 401, and a moderator gets 403.

A failed request leaves the current poster unchanged.

#### Scenario: Upload a poster
- **WHEN** an admin uploads a 300 KB JPEG for a linked work that had a Commons poster
- **THEN** the response is 200 with `poster.source: "manual"`, `manually_edited_at` is unchanged, and the public book page shows the uploaded poster

#### Scenario: Not an image
- **WHEN** an admin uploads a PDF file renamed to `poster.jpg`
- **THEN** the response is 422 «poster must be a JPEG, PNG or WebP image» and the poster is unchanged

#### Scenario: Too large
- **WHEN** an admin uploads a 6 MiB PNG
- **THEN** the response is 413 and the poster is unchanged

#### Scenario: Remove an uploaded poster
- **WHEN** an admin removes the uploaded poster of a work that was never edited by hand and whose Wikidata item has a TMDB id, with the TMDB key configured
- **THEN** the response has `poster: null` and `poster_removed: false`, and the next sync of a linked book fills the TMDB poster

#### Scenario: Remove a wrong automatic poster
- **WHEN** an admin removes the Commons poster of a work and the book is synced again
- **THEN** the work has no poster, `poster_removed` is `true`, and the public card shows the placeholder

#### Scenario: Moderator uploads
- **WHEN** a moderator uploads a poster
- **THEN** the response is 403 and the poster is unchanged

### Requirement: Poster section on the work edit page
The admin work edit page «Редактирование фильма» SHALL have a section «Постер», separate from the data form, whose actions apply at once:
- It shows the current poster in a 2:3 frame with «Источник: Загружен вручную», «Источник: Wikimedia Commons» or «Источник: TMDB». A Commons poster also shows its attribution.
- Without a poster it shows «Постера нет.». When `poster_removed` is true it adds «Автоматический постер удалён — синхронизация не подставит его снова.»
- «Загрузить постер», or «Заменить постер» when there is one, opens a file chooser for JPEG, PNG and WebP. The file is checked before sending, with «Нужен файл JPEG, PNG или WebP.» and «Файл больше 5 МБ.». On success the section refreshes and shows «Постер загружен.». A failure shows «Не удалось загрузить постер.».
- «Удалить постер» is offered when there is a poster. It asks «Удалить постер фильма «<title>»?» and then removes it, showing «Постер удалён.», or «Не удалось удалить постер.» on failure.

The poster buttons are disabled while a poster request runs. Poster actions do not change the form's fields or the mark «Изменён вручную».

#### Scenario: Upload from the edit page
- **WHEN** an admin opens a work without a poster, clicks «Загрузить постер» and picks a PNG of 1 MB
- **THEN** the section shows the poster with «Источник: Загружен вручную» and «Постер загружен.»

#### Scenario: Wrong file type
- **WHEN** an admin picks a GIF file
- **THEN** «Нужен файл JPEG, PNG или WebP.» is shown and nothing is sent

#### Scenario: Removed automatic poster
- **WHEN** an admin removes a TMDB poster and confirms
- **THEN** the section shows «Постер удалён.», «Постера нет.» and «Автоматический постер удалён — синхронизация не подставит его снова.»
