# catalog-admin/book-editing Specification

## Purpose

Defines how an admin creates and edits a book and which data rules every created book must meet.

## Requirements

### Requirement: Create and update endpoints
`POST /api/v1/admin/books` and `PATCH /api/v1/admin/books/{id}` SHALL require the `admin` role and a bearer header. The body carries the full book: `title`, `slug`, `description`, `cover_url`, `isbn`, `published_year`, `page_count`, `language`, `author_ids`, `genre_ids`, `status` and `allow_non_russian_title`. An update replaces every field and the author and genre links. `GET /api/v1/admin/books/{id}` returns a book of any status; an unknown book returns 404. A created book is published unless `status` says otherwise; an omitted `status` on update keeps the status.

#### Scenario: Replacing the authors
- **WHEN** an admin saves a book with one author id where it had two authors
- **THEN** the book has only that author

### Requirement: Book data rules
A book SHALL have a non-empty title written mainly in Cyrillic (at least half of its letters), unless `allow_non_russian_title` is set or, on update, the title is unchanged. A description, when given, MUST also be mainly Cyrillic. The language MUST be one of the supported codes (`ru`, `en`, `uk`, `be`, `kk`, `uz`, `hy`, `ka`, `az`, `lt`, `lv`, `et`, `mo`, `tg`, `tk`, `ky`, `fr`, `de`, `es`, `pl`, `it`, `pt`); the publication year from 1000 to 2100; the page count positive; a non-blank ISBN MUST have a valid checksum; every author and genre id MUST exist. The slug is taken from the request or transliterated from the title, MUST contain only letters, digits and hyphens and MUST be unique. A cover link that does not start with `http://` or `https://` is dropped without an error. A rule violation returns 422, a taken slug 409 and an invalid status 400. The same rules apply to every way of creating a book (see `catalog-import/book-file-import`).

#### Scenario: Latin title
- **WHEN** an admin creates «Empire V» without `allow_non_russian_title`
- **THEN** the response is 422 «title must be in Russian»

#### Scenario: Editing a grandfathered title
- **WHEN** an admin changes only the description of an existing book titled «Empire V»
- **THEN** the save succeeds without `allow_non_russian_title`

#### Scenario: Slug taken
- **WHEN** an admin saves a book with the slug of another book
- **THEN** the response is 409 and nothing changes

### Requirement: External source on update
An update SHALL write the book's external source and id from the request. The admin form does not send them, so saving a harvested book in the admin clears its external source and id: the book loses the «Импортировано» mark and is no longer recognised as already harvested.

#### Scenario: Editing a harvested book
- **WHEN** an admin fixes the description of a book that came from a Wikidata harvest and saves
- **THEN** the book no longer has an external source

### Requirement: Book edit page
The admin app SHALL provide `/catalog/books/{id}/edit` «Редактирование книги» with the book form: «Название *», «Название на латинице допустимо», «Slug» («Автоматически из названия»), «ISBN», «Описание», «URL обложки», «Год издания», «Количество страниц», «Язык *», «Авторы», «Жанры» and «Статус». Selected authors and genres that are not published are listed with their status under the pickers. Changing the status asks «Вы уверены, что хотите сменить статус на «<status>» у этой книги?». After saving, the admin returns to the list they came from. Errors show «Книга с таким slug уже существует.», the hint to tick «Название на латинице допустимо» for a non-Russian title, the server's own message for other rule violations, «Ошибка сервера. Попробуйте позже.» or «Не удалось сохранить книгу. Проверьте введенные данные.». An unknown book shows «Книга не найдена.».

#### Scenario: Non-Russian title hint
- **WHEN** an admin saves a Latin title without ticking «Название на латинице допустимо»
- **THEN** the form explains that the title must be in Russian and suggests ticking it
