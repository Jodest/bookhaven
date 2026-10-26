# screen-works/book-adaptations Specification

## Purpose
Shows readers the screen adaptations of a book on its public page and lets admins hide a wrong adaptation link without deleting it.

## Requirements

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
The admin book edit page SHALL show a section «Экранизации» listing every linked adaptation with its title, year, type and directors. Hidden ones carry the mark «Скрыта», and manual links carry the mark «Вручную». Each row has:
- «Скрыть», or «Показать» for a hidden one, which applies at once and refreshes the list;
- «Изменить», which opens the work's edit page and returns to the book afterwards;
- for a manual link only, «Удалить», which asks «Удалить связь с «<title>»? Сам фильм останется в каталоге.» and then deletes the link and refreshes the list.

The section always offers «Добавить экранизацию» (see `screen-works/admin-editing`), and offers «Найти экранизации в Wikidata» when the book has a Wikidata QID. It shows:
- «Экранизаций пока нет.» with no links;
- «Не удалось загрузить экранизации.» when the list fails to load;
- «Не удалось изменить видимость экранизации.» when hiding or showing fails;
- «Не удалось удалить связь.» when deleting fails.

#### Scenario: Hiding from the edit page
- **WHEN** an admin clicks «Скрыть» on an adaptation in the section
- **THEN** the row shows «Скрыта» and the button becomes «Показать»

#### Scenario: Wikidata row has no delete
- **WHEN** the section lists one adaptation from the sync and one added by hand
- **THEN** only the manual one has «Вручную» and «Удалить», and both have «Скрыть» and «Изменить»

#### Scenario: Book without a QID
- **WHEN** an admin opens the edit page of a book without a Wikidata QID
- **THEN** the section has «Добавить экранизацию» and no «Найти экранизации в Wikidata»

### Requirement: Adaptation filter in the catalog API
`GET /api/v1/books` SHALL accept the optional query parameter `has_adaptation`. With `has_adaptation=true` the list, its `total` and its pages MUST contain only books that have at least one visible adaptation link, that is a link an admin did not hide. The filter combines with every other catalog filter, the search and the sort. Any other value of the parameter, or its absence, MUST leave the list unfiltered. The endpoint stays open to everyone without a token, and still lists only published books.

#### Scenario: Only books with a visible adaptation
- **WHEN** anyone requests `/api/v1/books?has_adaptation=true` and the catalog has a published book with one visible adaptation, a published book whose only adaptation is hidden and a published book without adaptations
- **THEN** only the first book is returned and `total` is 1

#### Scenario: Combined with a genre filter
- **WHEN** anyone requests the books of one genre with `has_adaptation=true`
- **THEN** only books of that genre with a visible adaptation are returned

#### Scenario: Unknown value
- **WHEN** anyone requests `/api/v1/books?has_adaptation=yes`
- **THEN** the list is the same as without the parameter

### Requirement: «Есть экранизация» in the catalog filters
The public catalog filter panel SHALL offer the checkbox «Есть экранизация», on desktop and in the mobile filters. Checking it adds `has_adaptation=true` to the page URL, resets to the first page and shows only books with a visible adaptation. While it is on, the active filters show the chip «Есть экранизация», and removing the chip or unchecking the box removes the parameter. Opening a catalog URL with `has_adaptation=true` shows the checkbox checked and the chip.

#### Scenario: Reader turns the filter on
- **WHEN** a reader on page 3 of the catalog checks «Есть экранизация»
- **THEN** the URL has `has_adaptation=true` and no page beyond the first, the list shows only books with a visible adaptation, and the chip «Есть экранизация» is shown

#### Scenario: Shared link
- **WHEN** a reader opens a catalog link with `has_adaptation=true`
- **THEN** the checkbox «Есть экранизация» is checked and the chip is shown

#### Scenario: Removing the chip
- **WHEN** a reader removes the chip «Есть экранизация»
- **THEN** the parameter leaves the URL and the list is no longer filtered by adaptations

### Requirement: Adaptation vote endpoints
`PUT /api/v1/books/{id}/adaptations/{screen_work_id}/vote` with `{"value": "book" | "adaptation" | "same"}` SHALL require a signed-in user of any role. `book` means «Книга лучше», `adaptation` means «Экранизация лучше» and `same` means «Одинаково». A user has at most one vote per book and adaptation. The request creates the caller's vote, or replaces its value, and returns 200 with the caller's verdict item for that adaptation (see «Adaptation verdicts endpoint»).

To create or change a vote, the caller MUST have finished reading the book at least once (see `collections/reading-journal`), whatever the book's current status. A reader re-reading the book on «Читаю» may vote. Otherwise the response is 403 «only readers who have read the book can vote» and nothing changes.

Errors:
- no token returns 401;
- a malformed id returns 400 «invalid book id» or «invalid screen work id»;
- a missing or other `value` returns 400 «invalid vote value»;
- an unknown or unpublished book returns 404 «book not found»;
- a work that is not linked to the book, or whose link an admin hid, returns 404 «adaptation not found».

`DELETE /api/v1/books/{id}/adaptations/{screen_work_id}/vote` SHALL require a signed-in user of any role. It removes the caller's vote for that book and adaptation and returns 204, also when there was no vote. It does not check the reading journal, the shelf, the link or the book status. A malformed id returns 400. A user can never change or remove another user's vote.

#### Scenario: First vote
- **WHEN** a reader with the book on «Прочитано» sends `{"value": "book"}` for a visible adaptation of the published book
- **THEN** the response is 200 with `my_vote: "book"`, and `votes_count` has grown by one

#### Scenario: Changing a vote
- **WHEN** that reader then sends `{"value": "same"}` for the same adaptation
- **THEN** the response is 200 with `my_vote: "same"`, and `votes_count` is unchanged

#### Scenario: Book not read
- **WHEN** a signed-in user who has the book on «Хочу прочитать» and never finished it votes for one of its adaptations
- **THEN** the response is 403 «only readers who have read the book can vote» and no vote is stored

#### Scenario: Hidden link
- **WHEN** a reader votes for an adaptation whose link an admin hid
- **THEN** the response is 404 «adaptation not found»

#### Scenario: Invalid value
- **WHEN** a reader sends `{"value": "film"}`
- **THEN** the response is 400 «invalid vote value»

#### Scenario: Guest
- **WHEN** a request without a token sets a vote
- **THEN** the response is 401

#### Scenario: Removing a vote after leaving «Прочитано»
- **WHEN** a reader who voted has since moved the book to «Читаю» and deletes the vote
- **THEN** the response is 204 and the vote is gone

#### Scenario: Re-reader votes
- **WHEN** a reader who finished the book once and moved it back to «Читаю» to re-read it sends `{"value": "adaptation"}`
- **THEN** the response is 200 with `my_vote: "adaptation"`

#### Scenario: Read undone
- **WHEN** a reader who finished the book only once removes it from «Прочитано» and then votes
- **THEN** the response is 403 «only readers who have read the book can vote»

### Requirement: Adaptation verdicts endpoint
`GET /api/v1/books/{id}/adaptations/verdicts` SHALL be open to everyone, with an optional token. It returns 200 with `{can_vote, min_votes, items}`:
- `can_vote` is `true` only for a signed-in caller who has finished reading the book at least once (see `collections/reading-journal`), whatever its current status;
- `min_votes` is 5;
- `items` has one item per visible adaptation of the book, with `{screen_work_id, votes_count, book_better, adaptation_better, same, my_vote}`.

`votes_count` is the number of votes for that book and adaptation. `book_better`, `adaptation_better` and `same` are the vote counts per value when `votes_count` is at least `min_votes`, and MUST be `null` below it. `my_vote` is the caller's value, or `null` for a guest or a caller without a vote.

Hidden links are not returned. A malformed id returns 400 «invalid book id». An unknown or unpublished book returns 404 «book not found». A missing or invalid token is treated as a guest.

#### Scenario: Verdict revealed
- **WHEN** an adaptation of a published book has 18 votes «book», 3 votes «adaptation» and 2 votes «same»
- **THEN** its item has `votes_count: 23`, `book_better: 18`, `adaptation_better: 3` and `same: 2`

#### Scenario: Below the minimum
- **WHEN** an adaptation has 4 votes
- **THEN** its item has `votes_count: 4` and `null` for the three counts

#### Scenario: Guest
- **WHEN** a guest requests the verdicts
- **THEN** `can_vote` is `false` and every `my_vote` is `null`

#### Scenario: Reader of the book
- **WHEN** a signed-in user with the book on «Прочитано» who voted «adaptation» for one adaptation requests the verdicts
- **THEN** `can_vote` is `true` and that item has `my_vote: "adaptation"`

#### Scenario: Re-reader
- **WHEN** a signed-in user who finished the book once and is re-reading it on «Читаю» requests the verdicts
- **THEN** `can_vote` is `true`

### Requirement: Adaptation vote lifecycle
Votes SHALL belong to the link between a book and a work:
- A vote MUST be kept when the book later leaves the voter's «Прочитано» shelf, and it keeps counting.
- A vote for a link that an admin hid MUST be kept but is neither counted nor shown while the link is hidden. When the link is shown again, the votes count again.
- Deleting the link MUST remove its votes. Deleting the book or the user also removes the votes. A work can only be deleted without links (see `screen-works/admin-editing`), so it never has votes left.
- Editing a work's data and the Wikidata sync leave votes unchanged.
- A book that is no longer published keeps its votes, but its verdicts are not served.

#### Scenario: Book leaves «Прочитано»
- **WHEN** a reader who voted «book» removes the book from «Прочитано»
- **THEN** the adaptation's `votes_count` and `book_better` are unchanged

#### Scenario: Hide and show again
- **WHEN** an admin hides a link with 6 votes and later shows it again
- **THEN** while hidden the adaptation is absent from the verdicts, and afterwards its item has `votes_count: 6` again

#### Scenario: Manual link deleted
- **WHEN** an admin deletes a manual adaptation link with votes and later links the same work to the book again
- **THEN** the new link has `votes_count: 0`

### Requirement: Verdict on the book page
Each adaptation in the «Экранизации» block of the public book page SHALL show the community verdict under its links:
- From `min_votes` votes, «<P>% читателей: <leader> · <N> <голос|голоса|голосов>». The leader is the value with the most votes: «книга лучше», «экранизация лучше» or «одинаково». P is its share of all the votes, rounded to a whole number. When two values share the highest count, the line reads «Мнения разделились · <N> <голос|голоса|голосов>».
- Below `min_votes`, including no votes at all, «Пока мало голосов».

When the verdicts cannot be loaded, no verdict or voting control is shown, and the rest of the block is unaffected. The public author page («Фильмы об авторе») MUST NOT show verdicts or voting controls, because films about an author are not adaptations of a book.

#### Scenario: Verdict shown
- **WHEN** a reader opens a book whose adaptation has 18 votes «book», 3 votes «adaptation» and 2 votes «same»
- **THEN** the adaptation shows «78% читателей: книга лучше · 23 голоса»

#### Scenario: Few votes
- **WHEN** an adaptation has 3 votes
- **THEN** it shows «Пока мало голосов»

#### Scenario: Tie
- **WHEN** an adaptation has 3 votes «book», 3 votes «adaptation» and 0 votes «same»
- **THEN** it shows «Мнения разделились · 6 голосов»

#### Scenario: Verdicts unavailable
- **WHEN** the verdicts request fails
- **THEN** the «Экранизации» block lists the adaptations without verdict lines or voting controls

### Requirement: Voting on the book page
The «Экранизации» block SHALL offer voting according to the visitor:
- A guest sees the link «Войдите, чтобы сравнить книгу и экранизацию» once, under the block heading, and not in each adaptation. The link leads to the login page and back to the book.
- A signed-in user who may vote sees, under the verdict of each adaptation, «Что лучше:» on the same line as a compact segmented control with the buttons «Книга», «Экранизация» and «Одинаково». No explanatory note is shown. The button of the reader's current vote is shown as selected. Clicking another button changes the vote, and clicking the selected one removes it. The buttons are disabled while a request runs. A failure shows «Не удалось сохранить голос.» and keeps the previous state.
- A signed-in user who may not vote sees «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.» once, under the block heading. When they have a vote on an adaptation, that adaptation shows «Ваш голос: <Книга лучше|Экранизация лучше|Одинаково>» and the button «Убрать голос».

The block-level prompts are not shown while the verdicts load or when they cannot be loaded. A user may vote once they have finished the book at least once, whatever its current status. After a vote changes, the verdict line updates without reloading the page. When the reader changes the book's reading status on the same page, the voting controls and prompts update without a reload.

#### Scenario: Reader votes
- **WHEN** a reader with the book on «Прочитано» clicks «Экранизация» under an adaptation
- **THEN** that button is shown as selected and the verdict line reflects the new vote

#### Scenario: Reader removes the vote
- **WHEN** the reader clicks the selected «Экранизация» again
- **THEN** no button is selected and the vote is removed

#### Scenario: Guest prompt
- **WHEN** a guest opens a book with two adaptations
- **THEN** each adaptation shows its verdict line and no voting buttons, and «Войдите, чтобы сравнить книгу и экранизацию» appears once under the block heading

#### Scenario: Not read yet
- **WHEN** a signed-in user who never finished the book opens a book with two adaptations
- **THEN** «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.» appears once under the block heading, and no adaptation shows voting buttons

#### Scenario: Marking the book as read on the page
- **WHEN** that user marks the book «Прочитано» with the shelf buttons of the same page
- **THEN** the block-level prompt disappears and the voting buttons appear under each adaptation without a reload

#### Scenario: Vote kept after leaving «Прочитано»
- **WHEN** a reader who voted «Книга лучше» and finished the book only once removes it from «Прочитано» and looks at the adaptation
- **THEN** it shows «Ваш голос: Книга лучше» and «Убрать голос», and no voting buttons

#### Scenario: Re-reading keeps the voting buttons
- **WHEN** a reader who voted «Книга лучше» presses «Читаю снова» on the same page
- **THEN** the voting buttons stay, with «Книга» selected
