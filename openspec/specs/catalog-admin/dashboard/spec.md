# catalog-admin/dashboard Specification

## Purpose

Gives an admin a one-screen overview of the catalog and of what is waiting for moderation or import.

## Requirements

### Requirement: Dashboard counters
`GET /api/v1/admin/dashboard` SHALL require the `admin` role and return the counts of all books, pending books, rejected books, authors, genres (merged genres excluded), pending authors, pending genres and hidden reviews.

#### Scenario: Pending and rejected books
- **WHEN** the catalog has four books: two pending, one rejected and one published
- **THEN** the dashboard reports 4 books, 2 pending and 1 rejected

### Requirement: Overview page
The admin start page «Обзор» SHALL show the counters as tiles in the groups «Модерация» («Книги на модерации», «Авторы на модерации», «Жанры на модерации», «Скрытых отзывов») and «Каталог» («Всего книг», «Отклонённых книг», «Авторов», «Жанров»). Each tile links to the matching list: the moderation queues, `/moderation/reviews?visibility=hidden`, `/catalog/books`, `/catalog/books?status=rejected`, `/catalog/authors` and `/catalog/genres`. The group «Импорт» shows the latest harvest jobs («Задачи харвеста») with their state, or «Нет задач», with links to all jobs and to the Wikidata book harvest.

#### Scenario: Rejected books tile
- **WHEN** an admin clicks «Отклонённых книг»
- **THEN** the book list opens filtered to rejected books
