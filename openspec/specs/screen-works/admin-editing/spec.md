# screen-works/admin-editing Specification

## Purpose
Lets admins maintain screen works by hand. They can link a film to a book or an author, create and fix works, change how a film relates to an author and remove manual links or unused works. Data edited by hand is never overwritten by the Wikidata sync.

## Requirements

### Requirement: Screen work data rules
Every work created or edited by an admin SHALL satisfy these rules. Values are trimmed, and an empty optional value is stored as empty (`null`).
- `title` is required, at most 500 characters.
- `original_title` is optional, at most 500 characters. It is stored empty when equal to the title.
- `kind` is required, one of `film`, `tv_film`, `series`, `animated_film`, `animated_series` and `teleplay`.
- `year` is optional, from 1880 to 2100.
- `director` is optional, at most 500 characters.
- `imdb_id` is optional: `tt` followed by digits, at most 20 characters.
- `kinopoisk_id` is optional: digits only, at most 20 characters.
- `qid` is optional: `Q` followed by digits.

A violation MUST return 422 with `{"error": <message>}` and change nothing. The messages are `title is required`, `title is too long`, `original title is too long`, `invalid kind`, `year must be between 1880 and 2100`, `director is too long`, `invalid imdb id`, `invalid kinopoisk id` and `invalid qid`.

#### Scenario: Year out of range
- **WHEN** an admin creates a work with `year: 1850`
- **THEN** the response is 422 «year must be between 1880 and 2100» and no work or link is created

#### Scenario: IMDb id as a link
- **WHEN** an admin saves a work with `imdb_id: "https://www.imdb.com/title/tt0120737/"`
- **THEN** the response is 422 «invalid imdb id»

#### Scenario: Blank optional fields
- **WHEN** an admin saves a work with `director: "  "` and `original_title` equal to the title
- **THEN** both are stored empty and returned as `null`

### Requirement: Manual book adaptation link
`POST /api/v1/admin/books/{id}/adaptations` SHALL require the `admin` role and a bearer header. The body has exactly one of:
- `{"screen_work_id": "<uuid>"}`, which links an existing work;
- `{"work": {title, original_title, kind, year, director, imdb_id, kinopoisk_id, qid}}`, which creates a work and links it.

It works for a book of any status and creates a link with source `manual` that is not hidden. It returns 201 with the admin adaptation item (see `screen-works/book-adaptations`) plus `work_created`.

When `work.qid` matches a work already stored, that work is linked unchanged, the other submitted fields are ignored, and `work_created` is `false`. Without a QID match, a new work whose `imdb_id` or `kinopoisk_id` equals that of a stored work returns 409 `{"error": "screen work with this imdb id already exists", "screen_work_id": "<id>"}` (or `… kinopoisk id …`).

Errors:
- neither or both of `screen_work_id` and `work` return 400 «either screen_work_id or work is required»;
- a malformed id returns 400;
- an unknown book returns 404 «book not found»;
- an unknown `screen_work_id` returns 404 «screen work not found»;
- invalid work data returns 422 (see «Screen work data rules»);
- a work already linked to the book, hidden or not, returns 409 «screen work is already linked to this book».

A moderator gets 403.

#### Scenario: New work for a book without a QID
- **WHEN** an admin adds a new work «Собачье сердце» (1988, `tv_film`) to a published book that was added by hand and has no QID
- **THEN** the response is 201 with `source: "manual"`, `hidden: false` and `work_created: true`, and the public book page lists the adaptation at once

#### Scenario: QID already stored
- **WHEN** an admin creates a work with the QID of a work that the sync already stored, for another book
- **THEN** the response is 201 with `work_created: false`, no second work exists, and the stored work's data is unchanged

#### Scenario: Existing work
- **WHEN** an admin links an existing work by `screen_work_id` to a second book
- **THEN** one work is linked to both books

#### Scenario: Already linked
- **WHEN** an admin links a work that the sync already linked to the book and that an admin hid
- **THEN** the response is 409 and the link stays hidden with source `wikidata`

#### Scenario: Duplicate IMDb id
- **WHEN** an admin creates a work without a QID whose IMDb id is that of a stored work
- **THEN** the response is 409 with that work's id in `screen_work_id`, and nothing is created

#### Scenario: Moderator
- **WHEN** a moderator posts a manual adaptation link
- **THEN** the response is 403

### Requirement: Manual author film link
`POST /api/v1/admin/authors/{id}/screen-works` SHALL require the `admin` role and a bearer header. It behaves as the manual book adaptation link, for an author of any status. The body also requires `relation`, either `biopic` or `documentary`. It returns 201 with the admin author film item (see `screen-works/author-films`) plus `work_created`.

Errors:
- a missing or other `relation` returns 400 «invalid relation»;
- an unknown author returns 404 «author not found»;
- a work already linked to the author returns 409 «screen work is already linked to this author».

A moderator gets 403.

#### Scenario: Documentary for an author without a QID
- **WHEN** an admin adds a new documentary work to a published author without a QID with `relation: "documentary"`
- **THEN** the response is 201, and the public author page lists the film with the mark «Документальный»

#### Scenario: Missing relation
- **WHEN** an admin links an existing work to an author without `relation`
- **THEN** the response is 400 «invalid relation» and no link is created

### Requirement: Deleting links
`DELETE /api/v1/admin/books/{id}/adaptations/{screen_work_id}` and `DELETE /api/v1/admin/authors/{id}/screen-works/{screen_work_id}` SHALL require the `admin` role and a bearer header. They delete a link with source `manual` and return 204. The work stays in the catalog.

A link with source `wikidata` MUST NOT be deleted: the response is 409 «only manual links can be deleted; hide the link instead». A link that does not exist returns 404 («adaptation link not found» or «author film link not found»). A malformed id returns 400. A moderator gets 403.

A deleted manual link to a work with a QID MAY be created again by a later sync, as a `wikidata` link, when Wikidata lists that work for the book or author. Hiding is the way to suppress a work permanently.

#### Scenario: Delete a manual link
- **WHEN** an admin deletes a manual adaptation link of a published book
- **THEN** the response is 204, the public book page no longer lists the work, and the work is still in the work list

#### Scenario: Wikidata link
- **WHEN** an admin deletes a link created by the sync
- **THEN** the response is 409 and the link is unchanged

### Requirement: Work list endpoint
`GET /api/v1/admin/screen-works` SHALL require the `admin` role. It returns `{items, total, page, limit}`, with `page` defaulting to 1 and `limit` to 20, at most 100. Works are ordered by title, then year, then id.

`search` matches:
- a case-insensitive substring of the title or the original title;
- the exact QID, IMDb id or Kinopoisk id when the query has that form.

`unlinked=true` keeps only works without any book or author link.

Each item is `{id, title, original_title, kind, year, director, imdb_id, kinopoisk_id, qid, manually_edited_at, book_count, author_count}`. The counts include hidden links. A moderator gets 403.

#### Scenario: Search by title
- **WHEN** an admin searches «мастер» and the work «Мастер и Маргарита» exists
- **THEN** it is listed with its book and author counts

#### Scenario: Search by IMDb id
- **WHEN** an admin searches `tt0120737`
- **THEN** only the work with that IMDb id is listed

#### Scenario: Unlinked works
- **WHEN** an admin lists works with `unlinked=true`
- **THEN** only works without any book or author link are listed

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

### Requirement: Manual data wins over the sync
A work created or edited by an admin SHALL carry the mark «edited by hand». The book adaptation sync and the author film sync MUST NOT change any field of such a work, even an empty one. They still link it to further books or authors when its QID is listed. A sync that finds a work already linked by hand MUST keep the link's source `manual`, its visibility and its relation.

#### Scenario: Re-sync after an edit
- **WHEN** an admin clears the year of a Wikidata work and fixes its title, and then the book is synced again
- **THEN** the work keeps the admin's title and has no year

#### Scenario: Manual link then sync
- **WHEN** an admin links a work with a QID to a book by hand and later syncs that book, and Wikidata lists that work
- **THEN** the book has one link to the work, with source `manual`, and the outcome counts it as not added

### Requirement: Deleting a work
`DELETE /api/v1/admin/screen-works/{id}` SHALL require the `admin` role and a bearer header. It deletes a work that has no book link and no author link and returns 204. A work with any link, hidden ones included, MUST NOT be deleted: the response is 409 «screen work has links». The check and the delete are atomic. An unknown work returns 404. A moderator gets 403.

#### Scenario: Unlinked work
- **WHEN** an admin deletes a work after removing its only manual link
- **THEN** the response is 204 and the work is gone from the list

#### Scenario: Work with a hidden Wikidata link
- **WHEN** an admin deletes a work whose only link is a hidden Wikidata link
- **THEN** the response is 409, and the work and the hidden link are unchanged

### Requirement: Work list page
The admin app SHALL provide «Фильмы и сериалы» (`/catalog/screen-works`), a tab of «Каталог» visible to admins only. It shows 20 rows per page, with:
- the search box «Поиск по названию, QID, IMDb или Кинопоиску»;
- the checkbox «Только без связей».

Each row shows the title, the year, the type, the directors, the QID, «Книг: <n>», «Авторов: <m>» and «Изменить». Search, filter and page are kept in the address. The page shows «Фильмы не найдены.» for an empty result and «Не удалось загрузить фильмы.» when loading fails. The page has no create action: works are created from a book or an author.

#### Scenario: Open a work from the list
- **WHEN** an admin searches «солярис» and clicks «Изменить» on a row
- **THEN** the work's edit page opens, and «← Назад» returns to the list with the same search

### Requirement: Work edit page
The admin app SHALL provide `/catalog/screen-works/{id}/edit` «Редактирование фильма». The form has «Название *», «Оригинальное название», «Тип *» («Фильм», «Телефильм», «Сериал», «Мультфильм», «Мультсериал», «Телеспектакль»), «Год», «Режиссёр», «IMDb id», «Кинопоиск id» and «QID Wikidata». The QID field is read-only when the work has a QID. A pasted IMDb or Kinopoisk film link is reduced to its id before saving. The page shows «Изменён вручную <date>» for a work edited by hand. It lists the linked books and authors as links to their edit pages, with «Скрыта»/«Скрыт», «Вручную» and «Документальный» marks.

«Удалить фильм» is offered only when the work has no links, after the confirmation «Удалить фильм «<title>» из каталога?». A linked work shows «Фильм можно удалить, только когда у него нет связей с книгами и авторами.». After saving or deleting, the admin returns to where they came from: the work list, a book edit page or an author edit page.

Errors:
- field rules are checked before sending, with «Укажите название.», «Год должен быть от 1880 до 2100.», «IMDb id должен иметь вид tt и цифры.», «Кинопоиск id должен состоять из цифр.» and «QID должен иметь вид Q и цифры.»;
- «Фильм с таким QID уже есть в каталоге.» for a taken QID;
- «Не удалось сохранить фильм.» or «Не удалось удалить фильм.» for other failures;
- «Фильм не найден.» for an unknown work.

#### Scenario: Linked work
- **WHEN** an admin opens a work linked to one book
- **THEN** the page lists that book and has no «Удалить фильм» button

#### Scenario: Pasted Kinopoisk link
- **WHEN** an admin pastes `https://www.kinopoisk.ru/film/43911/` into «Кинопоиск id» and saves
- **THEN** the work is saved with the Kinopoisk id `43911`

### Requirement: Add film dialog
The «Добавить экранизацию» action of a book and the «Добавить фильм об авторе» action of an author SHALL open a dialog with two modes:
- «Выбрать из каталога» searches the work list. Each result shows the title, year, type and QID. A work already linked to this book or author is marked «Уже привязан» and cannot be chosen.
- «Создать новый» shows the work fields of the edit page, with the same checks.

For an author, the dialog also asks «Тип связи» with «Биографический» (default) and «Документальный». On success the dialog closes, the section refreshes and shows:
- «Экранизация «<title>» добавлена.» for a book;
- «Фильм «<title>» добавлен.» for an author;
- «Фильм с этим QID уже был в каталоге — привязан он.» added when `work_created` is `false`.

Errors:
- «Этот фильм уже привязан.» for 409 already linked;
- «Фильм с таким IMDb id уже есть в каталоге — выберите его в поиске.» or the same text for the Kinopoisk id, for a duplicate external id;
- «Не удалось добавить экранизацию.» or «Не удалось добавить фильм.» otherwise.

The submit button is disabled while the request runs.

#### Scenario: Pick an existing work
- **WHEN** an admin opens «Добавить экранизацию», searches «гамлет» and chooses a result
- **THEN** the work is linked to the book with the mark «Вручную» and the banner «Экранизация «Гамлет» добавлена.» is shown

#### Scenario: Duplicate IMDb id in the dialog
- **WHEN** an admin creates a new work in the dialog with the IMDb id of a stored work
- **THEN** the dialog stays open with «Фильм с таким IMDb id уже есть в каталоге — выберите его в поиске.»

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
