# home/home-page Specification

## Purpose

Assembles the public home page from independent blocks, keeps it fast with a short-lived cache, and never lets one broken block take the page down.

## Requirements

### Requirement: Public home payload
`GET /api/v1/home` SHALL be available to anyone and return `{date, book_of_the_day, calendar, new_books, top_rated, recent_reviews}`. `date` is today's date in Moscow time (UTC+3), which defines "today" for every daily rule of the home page. `book_of_the_day` and `calendar` are `null` when there is nothing to show; the three lists are empty arrays.

#### Scenario: Day boundary
- **WHEN** the home page is requested at 22:30 UTC on 9 March
- **THEN** `date` is 10 March

### Requirement: Independent blocks
Each public block (book of the day, calendar, new books, top rated, recent reviews) SHALL be computed independently. A block that fails MUST come back empty while the others are returned; only when every block fails does the request return 500.

#### Scenario: One block fails
- **WHEN** the calendar query fails and the other blocks succeed
- **THEN** the response is 200 with `calendar: null` and the other blocks filled

### Requirement: Selection cache
The choice of which books and reviews each block shows SHALL be cached for 10 minutes per Moscow date, and only when no block failed. Book cards and reviews MUST still be read live on every request, so a book or review hidden after the choice was cached disappears at once, and an author or anniversary whose books are all hidden is dropped. Newly published books and new reviews MAY take up to 10 minutes to appear. The cache is optional: without it every request computes the blocks.

#### Scenario: Book hidden after caching
- **WHEN** a moderator unpublishes the book of the day five minutes after the payload was cached
- **THEN** the next home page response has `book_of_the_day: null` until the cache expires

### Requirement: Home page layout
The web app SHALL render `/` on the server without a hero: there is no page heading, tagline, call-to-action buttons or genre links, since the header and the catalog cover them. On wide screens the page has two columns. The main column holds, in order: «Продолжить чтение» for signed-in users (see `home/personal-blocks`), «Книга дня», «Из вашего «Хочу прочитать»» for signed-in users, «Вам может понравиться», «Новое в каталоге», «Высоко оценённые», «Подборки» (see `home/catalog-rows`) and «Свежие рецензии». A narrow sidebar next to it holds, for signed-in users, the reading-challenge card, then «Что почитать?» and the literary calendar. On narrow screens the page is one column: «Продолжить чтение», «Книга дня» and «Из вашего «Хочу прочитать»», then the sidebar blocks, then the remaining rows. Book rows show one row of books that fits the column, without horizontal scrolling. An empty block renders nothing. «Подборки» does not come from the home payload. It SHALL be loaded on its own, like the genre list of «Что почитать?». If the home payload cannot be loaded, the page still shows the personal blocks, «Что почитать?» and «Подборки».

#### Scenario: Guest on an empty catalog
- **WHEN** a guest opens `/` while no book is published
- **THEN** the page shows «Что почитать?» and no book block, heading, buttons or genre links

#### Scenario: Home payload fails
- **WHEN** a guest opens `/` while the home payload request fails and one collection with places is published
- **THEN** the page shows «Что почитать?» and the «Подборки» block, and no other book block
