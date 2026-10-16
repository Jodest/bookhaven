# catalog-admin/admin-help Specification

## Purpose
Gives admins and moderators a built-in, task-oriented guide to the admin app, rendered from the same markdown chapters that live in the repository.

## Requirements

### Requirement: Help chapters
The admin app SHALL provide a help section «Справка» in the sidebar (after «Пользователи»). `/help` MUST list the guide chapters available to the user, in this order and with these titles: «Первые шаги» (`/help/getting-started`), «Как добавить книгу, автора или жанр» (`/help/adding-content`), «Задачи импорта» (`/help/import-jobs`), «Подборки и ИИ-кураторы» (`/help/collections`), «Проверка каталога» (`/help/catalog-review`), «Модерация отзывов» (`/help/moderation`), «Слияние дублей» (`/help/duplicates`), «Экранизации» (`/help/screen-works`), «Пользователи и роли» (`/help/users-roles`). A chapter page MUST show the chapter rendered from the guide's markdown file of the same name in the repository, together with the list of available chapters for navigation, the current one highlighted. The chapter content is fixed when the admin app is built: an edited chapter appears after the next build. An unknown chapter path MUST show the not-found page. The help content is not confidential; role scoping is a navigation aid, not access control.

#### Scenario: Admin opens the help
- **WHEN** an admin clicks «Справка» in the sidebar
- **THEN** `/help` lists all nine chapters in the order above

#### Scenario: Admin opens a chapter
- **WHEN** an admin clicks «Задачи импорта» on `/help`
- **THEN** `/help/import-jobs` shows that chapter's content with «Задачи импорта» highlighted in the chapter list

#### Scenario: Unknown chapter
- **WHEN** an admin opens `/help/no-such-chapter`
- **THEN** the not-found page is shown

### Requirement: Role-scoped help
The chapter «Модерация отзывов» SHALL be available to `admin` and `moderator`; every other chapter MUST be available to `admin` only. A moderator MUST see «Справка» in the sidebar, and `/help` MUST list only «Модерация отзывов» for them. A moderator opening any other chapter MUST see «Этот раздел доступен только администраторам.» with the button «К отзывам». Adding the help section MUST NOT change any user's start page.

#### Scenario: Moderator opens the help
- **WHEN** a moderator opens `/help`
- **THEN** the only chapter listed is «Модерация отзывов»

#### Scenario: Moderator opens an admin-only chapter
- **WHEN** a moderator opens `/help/adding-content`
- **THEN** the page shows «Этот раздел доступен только администраторам.» and the button «К отзывам»

#### Scenario: Moderator start page unchanged
- **WHEN** a moderator signs in to the admin app
- **THEN** they land on the review moderation page, as before

### Requirement: Section help link
Every admin page outside `/help` and `/login` SHALL show a «?» link (accessible name «Справка по разделу») above the page content that opens the help chapter for the current section: «Обзор» → «Первые шаги»; «Модерация» → «Книги», «Авторы», «Жанры», «Описания ИИ» → «Проверка каталога»; «Модерация» → «Отзывы» → «Модерация отзывов»; «Каталог» → «Книги», «Авторы», «Жанры» (lists and edit pages) → «Как добавить книгу, автора или жанр»; «Каталог» → «Фильмы и сериалы» (list and edit pages) → «Экранизации»; «Подборки» (list, new and edit pages) → «Подборки и ИИ-кураторы»; «Импорт» → «Книги», «Авторы», «Жанры» (every method sub-page) → «Как добавить книгу, автора или жанр»; «Импорт» → «Задачи» → «Задачи импорта»; «Пользователи» → «Пользователи и роли». The link MUST only point to a chapter available to the user's role.

#### Scenario: Help from import jobs
- **WHEN** an admin on «Импорт» → «Задачи» clicks «?»
- **THEN** `/help/import-jobs` opens

#### Scenario: Help from an import method
- **WHEN** an admin on «Импорт» → «Книги» → «Из файла» clicks «?»
- **THEN** `/help/adding-content` opens

#### Scenario: Help from review moderation
- **WHEN** a moderator on «Модерация» → «Отзывы» clicks «?»
- **THEN** `/help/moderation` opens

### Requirement: Safe guide rendering
The help SHALL render the chapters' markdown headings, paragraphs, emphasis, ordered and unordered lists, tables, block quotes, inline code, code blocks and links. Raw HTML in a chapter MUST NOT be rendered as HTML (it is dropped or shown as text). A link to another chapter file of the guide MUST open that chapter under `/help` (keeping a `#fragment`); a link to an external site MUST open in a new tab.

#### Scenario: Link to another chapter
- **WHEN** a chapter links to the guide file of «Слияние дублей» and the admin clicks the link in the help
- **THEN** `/help/duplicates` opens

#### Scenario: Raw HTML in a chapter
- **WHEN** a chapter contains a `<script>` tag
- **THEN** the help page does not execute or render it as an element

### Requirement: Guide content
The guide SHALL be written in Russian and organized by tasks, not by pages. Each chapter MUST describe the current admin app: UI labels quoted exactly as rendered, menu paths written as «Раздел → Вкладка → Подвкладка». «Первые шаги» MUST contain a map of all sidebar sections with what each is for; «Как добавить книгу, автора или жанр» MUST cover all four ways (Wikidata, ИИ, файл, вручную) with guidance on when to use which; «Модерация отзывов» MUST be self-contained for a moderator (no required reading of admin-only chapters).

#### Scenario: Moderator reads their chapter
- **WHEN** a moderator reads «Модерация отзывов»
- **THEN** it explains how to find, hide and restore reviews without referring them to chapters they cannot open
