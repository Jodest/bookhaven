# catalog-import/admin-import-section Specification

## Purpose

Organizes the admin «Импорт» section by entity and import method, so every way of adding a book, author or genre lives in exactly one place with a bookmarkable URL.

## Requirements

### Requirement: Entity tabs
The «Импорт» section SHALL have the tabs Книги, Авторы, Жанры and Задачи (`/import/jobs`), in that order. All of them MUST be visible to admins only. The frontend guard is UX only; backend role checks stay authoritative.

#### Scenario: Moderator
- **WHEN** a moderator opens an import page
- **THEN** access is denied

### Requirement: One sub-page per import method
Each entity tab SHALL have one page per available method, in the fixed order Вручную → Wikidata → Из файла: `/import/books/{manual,wikidata,file}`, `/import/authors/{manual,wikidata}` and `/import/genres/{manual,wikidata}`. A method an entity does not support MUST have no tab, route or placeholder. Each sub-page renders only its own method with its own heading.

#### Scenario: Authors have no file import
- **WHEN** an admin opens the Авторы tab
- **THEN** only the «Вручную» and «Wikidata» sub-tabs are shown

### Requirement: Default sub-pages and redirects
The tab defaults SHALL be `/import/books/wikidata`, `/import/authors/manual` and `/import/genres/manual`. The bare paths `/import/books`, `/import/authors` and `/import/genres` MUST answer 307 to their default, and `/import` MUST lead to the first visible tab's default. Legacy URLs MUST redirect, non-permanently, to their exact sub-page: `/import/wikidata` → `/import/books/wikidata`, `/import/file` → `/import/books/file`, `/import/author` → `/import/authors/wikidata`, `/catalog/books/new` → `/import/books/manual`.

#### Scenario: Old bookmark
- **WHEN** an admin opens `/import/file`
- **THEN** the browser lands on `/import/books/file`

#### Scenario: Bare entity path
- **WHEN** an admin requests `/import/books`
- **THEN** the response is 307 to `/import/books/wikidata`

### Requirement: Sub-tab navigation
A secondary sub-tab row SHALL be shown under the entity tabs only when the current tab has at least two visible sub-tabs. The current sub-tab MUST have `aria-current="page"`, the entity tab stays active on every one of its sub-pages, and the sidebar highlights «Импорт».

#### Scenario: Jobs tab
- **WHEN** an admin opens `/import/jobs`
- **THEN** no sub-tab row is shown

### Requirement: Creation lives only under Import
Manual creation of books, authors and genres SHALL be available only on the «Вручную» import sub-pages. The catalog MUST NOT offer an «add» action; it keeps listing, search, filters, editing and merging. Manually created authors and genres are created `published`.

#### Scenario: Catalog books list
- **WHEN** an admin opens Каталог → Книги
- **THEN** there is no «Добавить книгу» button

### Requirement: Stay on the page after manual creation
After a successful manual creation, the admin SHALL stay on the import page: the form resets and a success banner links to the created record's edit page (`/catalog/books/{id}/edit`, `/catalog/authors/{id}/edit`, `/catalog/genres/{id}/edit`). Errors MUST be shown inline.

#### Scenario: Book created
- **WHEN** an admin saves a new book on `/import/books/manual`
- **THEN** the form resets and a banner «Книга «…» создана» links to the book's edit page
