# screen-works/book-adaptations Specification

## Purpose
Shows readers the screen adaptations of a book on its public page and lets admins hide a wrong adaptation link without deleting it.

## Requirements

### Requirement: Public adaptations endpoint
`GET /api/v1/books/{id}/adaptations` SHALL be open to everyone without a token. It returns 200 with `{items, total}`, the book's visible adaptations without pagination. Each item is `{id, title, original_title, kind, year, director, imdb_url, kinopoisk_url, poster}`:
- `kind` is one of `film`, `tv_film`, `series`, `animated_film`, `animated_series` and `teleplay`.
- Fields without a value are `null`.
- `imdb_url` is `https://www.imdb.com/title/<imdb id>/` and `kinopoisk_url` is `https://www.kinopoisk.ru/film/<kinopoisk id>/`.
- `poster` is `null` for a work without a poster, else `{url, source, author, license, license_url, source_url}`:
  - `source` is `manual` for a poster uploaded by an admin, `commons` for a Wikimedia Commons image and `tmdb` for a TMDB poster.
  - `url` is the site path `/posters/<shard>/<file>` for `manual`. For `commons` and `tmdb` it is the site path `/covers/<shard>/<file>` once a copy of the current poster link is stored (see `screen-works/poster-copies`). Before that it is the source link: an `https://upload.wikimedia.org/...` or `https://thumb.wikimedia.org/...` thumbnail for `commons`, and `https://image.tmdb.org/t/p/w500/<file>` for `tmdb`.
  - `author`, `license`, `license_url` and `source_url` (the Commons file page) are set only for `commons`, each `null` when unknown, and are `null` for the other sources.

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

#### Scenario: Poster of an adaptation
- **WHEN** a published book has one adaptation with an uploaded poster and one without any poster
- **THEN** the first item has `poster.source: "manual"` and a `poster.url` starting with `/posters/`, and the second item has `poster: null`

#### Scenario: Commons poster attribution
- **WHEN** an adaptation's poster comes from a CC BY-SA file on Wikimedia Commons
- **THEN** its `poster` has `source: "commons"`, the file's author and licence, the licence URL and the Commons file page in `source_url`

#### Scenario: Copied TMDB poster
- **WHEN** an adaptation's TMDB poster has a stored copy
- **THEN** its `poster` has `source: "tmdb"` and a `url` starting with `/covers/`

### Requirement: Adaptations block on the book page
The public book page SHALL show the book's visible adaptations in the section «Экранизации». It is the first section after the genres (see `catalog/book-page`), has a visible heading «Экранизации» and can be reached at the in-page anchor `#adaptations`. The adaptations are compact horizontal cards, in one column on narrow screens and in two columns on wide screens. Each card shows:
- on the left, a small poster thumbnail in a 2:3 frame, about 72 px wide and 108 px high, with the adaptation's title as its alternative text;
- on the right, from top to bottom:
  - the title;
  - one wrapping line with the year, the type, «Режиссёр: <names>» when directors are known, and the links «Кинопоиск» and «IMDb» for the ids it has, opening in a new tab. The type is shown as «Фильм», «Телефильм», «Сериал», «Мультфильм», «Мультсериал» or «Телеспектакль»;
  - the verdict line and the voting control (see «Verdict on the book page» and «Voting on the book page»);
  - the compact Commons caption, when the poster has one (see «Poster attribution»).

A card without a poster, or whose poster image fails to load, shows a placeholder of the same thumbnail size instead. The placeholder has the type's icon and the year, or only the icon when the year is unknown.

When the book has more than four visible adaptations, only the first four, in the endpoint's order, are shown at first. They are followed by the button «Показать все (<N>)», where N is the number of visible adaptations. Activating it reveals the remaining cards without reloading the page, moves the keyboard focus to the first revealed card, and removes the button. With four adaptations or fewer there is no button.

Every card MUST be part of the server-rendered page, including the cards not yet revealed. When there is no visible adaptation, or the adaptations cannot be loaded, the «Экранизации» section MUST NOT be rendered and the rest of the page is unaffected.

#### Scenario: Book with adaptations
- **WHEN** a reader opens a published book with two visible adaptations, one with a Kinopoisk id, an IMDb id and a poster, and one from 1975 with neither id and no poster
- **THEN** the section «Экранизации» shows both compact cards. The first has its poster thumbnail and the links «Кинопоиск» and «IMDb». The second has the thumbnail-sized placeholder with its type icon and «1975» and has no links. There is no «Показать все» button.

#### Scenario: Nothing to show
- **WHEN** a reader opens a book whose only adaptation is hidden
- **THEN** the page has no «Экранизации» section, and «Похожие книги» (when there are similar books) and «Рецензии» follow the genres

#### Scenario: Broken poster image
- **WHEN** the poster image of an adaptation fails to load in the reader's browser
- **THEN** the card shows the thumbnail-sized placeholder with the type icon and the year instead

#### Scenario: Server-rendered adaptations
- **WHEN** a crawler requests a book page with six visible adaptations without running scripts
- **THEN** the returned HTML contains the titles of all six adaptations, including the two not yet revealed

#### Scenario: More than four adaptations
- **WHEN** a reader opens a book with six visible adaptations
- **THEN** the first four cards are visible, followed by «Показать все (6)», and activating it shows all six cards, moves the focus to the fifth card and removes the button

#### Scenario: Exactly four adaptations
- **WHEN** a reader opens a book with four visible adaptations
- **THEN** all four cards are visible and there is no «Показать все» button

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
- `min_votes` is 1, so a verdict is revealed from the first vote;
- `items` has one item per visible adaptation of the book, with `{screen_work_id, votes_count, book_better, adaptation_better, same, my_vote}`.

`votes_count` is the number of votes for that book and adaptation. `book_better`, `adaptation_better` and `same` are the vote counts per value when `votes_count` is at least `min_votes`, and MUST be `null` below it. With `min_votes` 1 they are `null` only for an adaptation without votes. `my_vote` is the caller's value, or `null` for a guest or a caller without a vote.

Hidden links are not returned. A malformed id returns 400 «invalid book id». An unknown or unpublished book returns 404 «book not found». A missing or invalid token is treated as a guest.

#### Scenario: Verdict revealed
- **WHEN** an adaptation of a published book has 18 votes «book», 3 votes «adaptation» and 2 votes «same»
- **THEN** its item has `votes_count: 23`, `book_better: 18`, `adaptation_better: 3` and `same: 2`

#### Scenario: Below the minimum
- **WHEN** an adaptation has no votes
- **THEN** its item has `votes_count: 0` and `null` for the three counts, and the response has `min_votes: 1`

#### Scenario: First vote revealed
- **WHEN** an adaptation has a single vote «book»
- **THEN** its item has `votes_count: 1`, `book_better: 1`, `adaptation_better: 0` and `same: 0`

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
Each adaptation card in the «Экранизации» section of the public book page SHALL show the community verdict under its metadata line, with no divider line between them:
- From `min_votes` votes, «<P>% читателей: <leader> · <N> <голос|голоса|голосов>». The leader is the value with the most votes: «книга лучше», «экранизация лучше» or «одинаково». P is its share of all the votes, rounded to a whole number. When two values share the highest count, the line reads «Мнения разделились · <N> <голос|голоса|голосов>».
- Below `min_votes`, including no votes at all, no verdict line is shown. No placeholder text is shown instead.

When the verdicts cannot be loaded, no verdict or voting control is shown, and the rest of the section is unaffected. The public author page («Фильмы об авторе») MUST NOT show verdicts or voting controls, because films about an author are not adaptations of a book.

#### Scenario: Verdict shown
- **WHEN** a reader opens a book whose adaptation has 18 votes «book», 3 votes «adaptation» and 2 votes «same»
- **THEN** the adaptation shows «78% читателей: книга лучше · 23 голоса»

#### Scenario: Few votes
- **WHEN** an adaptation has no votes
- **THEN** its card shows no verdict line and the text «Пока мало голосов» appears nowhere

#### Scenario: Single vote
- **WHEN** an adaptation has one vote «book»
- **THEN** it shows «100% читателей: книга лучше · 1 голос»

#### Scenario: Tie
- **WHEN** an adaptation has 3 votes «book», 3 votes «adaptation» and 0 votes «same»
- **THEN** it shows «Мнения разделились · 6 голосов»

#### Scenario: Verdicts unavailable
- **WHEN** the verdicts request fails
- **THEN** the «Экранизации» section lists the adaptations without verdict lines or voting controls

### Requirement: Voting on the book page
The «Экранизации» section SHALL offer voting according to the visitor:
- A guest sees the link «Войдите, чтобы сравнить книгу и экранизацию» once, under the section heading and above the cards, and not in each adaptation. The link leads to the login page and back to the book.
- A signed-in user who may vote sees, in each adaptation card under the verdict line (or under the metadata line when there is no verdict line), «Что лучше:» on the same line as a compact dropdown. No explanatory note is shown.
  - Without a vote the dropdown shows «Не выбрано» and offers «Книга», «Экранизация» and «Одинаково».
  - With a vote it shows the voted value and offers «Книга», «Экранизация», «Одинаково» and «Убрать голос».
  - Choosing another value changes the vote, and choosing «Убрать голос» removes it, after which the dropdown shows «Не выбрано».
  - The dropdown is disabled while a request runs. A failure shows «Не удалось сохранить голос.» and the dropdown keeps the previous value.
- A signed-in user who may not vote sees «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.» once, under the section heading and above the cards. When they have a vote on an adaptation, that adaptation shows «Ваш голос: <Книга лучше|Экранизация лучше|Одинаково>» and the button «Убрать голос».

The section-level prompts are not shown while the verdicts load or when they cannot be loaded. A user may vote once they have finished the book at least once, whatever its current status. After a vote changes, the verdict line updates without reloading the page. When the reader changes the book's reading status on the same page, the voting controls and prompts update without a reload. Cards revealed by «Показать все» offer the same controls as the first four.

#### Scenario: Reader votes
- **WHEN** a reader with the book on «Прочитано» chooses «Экранизация» in the dropdown of an adaptation that had no votes
- **THEN** the dropdown shows «Экранизация» and the line «100% читателей: экранизация лучше · 1 голос» appears without a reload

#### Scenario: Reader removes the vote
- **WHEN** the reader then chooses «Убрать голос» in the same dropdown
- **THEN** the vote is removed, the dropdown shows «Не выбрано» and no longer offers «Убрать голос»

#### Scenario: Guest prompt
- **WHEN** a guest opens a book with two adaptations that both have votes
- **THEN** each adaptation shows its verdict line and no dropdown, and «Войдите, чтобы сравнить книгу и экранизацию» appears once, under the «Экранизации» heading

#### Scenario: Not read yet
- **WHEN** a signed-in user who never finished the book opens a book with two adaptations
- **THEN** «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.» appears once, under the «Экранизации» heading, and no adaptation shows the dropdown

#### Scenario: Marking the book as read on the page
- **WHEN** that user marks the book «Прочитано» with the shelf buttons of the same page
- **THEN** the section-level prompt disappears and the dropdown appears in each adaptation without a reload

#### Scenario: Vote kept after leaving «Прочитано»
- **WHEN** a reader who voted «Книга лучше» and finished the book only once removes it from «Прочитано» and looks at the adaptation
- **THEN** it shows «Ваш голос: Книга лучше» and «Убрать голос», and no dropdown

#### Scenario: Re-reading keeps the voting buttons
- **WHEN** a reader who voted «Книга лучше» presses «Читаю снова» on the same page
- **THEN** the dropdown stays and shows «Книга»

#### Scenario: Failed vote
- **WHEN** a reader with the vote «Книга» chooses «Одинаково» and the request fails
- **THEN** «Не удалось сохранить голос.» is shown and the dropdown shows «Книга» again

#### Scenario: Voting on a revealed card
- **WHEN** a reader with the book on «Прочитано» opens a book with six adaptations and activates «Показать все (6)»
- **THEN** the fifth and sixth cards show «Что лучше:» with the dropdown

### Requirement: Adaptations link in the book header
When the book has at least one visible adaptation, the book header SHALL show, next to the rating summary, an accent link with a film icon and «<N> <экранизация|экранизации|экранизаций>». It is an in-page link to the anchor `#adaptations`. Activating it scrolls the page to the «Экранизации» section without reloading, and it also works without scripts. The section's heading stays visible below the site header after the scroll. The link MUST NOT add a `tab` parameter to the address. Without visible adaptations the link MUST NOT be shown.

#### Scenario: Header link
- **WHEN** a reader who has scrolled down to the reviews of a book with two visible adaptations clicks «2 экранизации» in the header
- **THEN** the page scrolls to the «Экранизации» section without a reload, and the address ends with `#adaptations` and has no `tab` parameter

#### Scenario: No header link
- **WHEN** a reader opens a book without visible adaptations
- **THEN** the header has no adaptations link

### Requirement: Poster attribution
Wherever a poster is shown on the public site, a Wikimedia Commons poster SHALL be credited on its card in a compact caption «Постер: <author>, <license>, Wikimedia Commons». On the compact adaptation and film cards, the caption is the last line of the card's text, next to the thumbnail, in a smaller type than the metadata line. The author part is left out when unknown, and the licence links to its address when known. «Wikimedia Commons» links to the file page. Both links open in a new tab. Uploaded and TMDB posters have no caption. Copied posters are credited exactly like posters shown from their source.

The public site footer SHALL always show, in this order:
- «Постеры фильмов — Wikimedia Commons и TMDB.»;
- the official TMDB logo. It links to `https://www.themoviedb.org`, opens in a new tab and has the accessible name «TMDB». It MUST be shown smaller and less prominent than the BookHaven logo of the site header;
- the notice «BookHaven uses TMDB and the TMDB APIs but is not endorsed, certified, or otherwise approved by TMDB.»

#### Scenario: Commons poster caption
- **WHEN** an adaptation's poster is a Commons file by «Jane Doe» under «CC BY-SA 4.0»
- **THEN** the card shows «Постер: Jane Doe, CC BY-SA 4.0, Wikimedia Commons» with links to the licence and the file page

#### Scenario: TMDB notice
- **WHEN** a reader opens any page of the public site
- **THEN** the footer contains «BookHaven uses TMDB and the TMDB APIs but is not endorsed, certified, or otherwise approved by TMDB.» and no longer contains «This product uses the TMDB API but is not endorsed or certified by TMDB.»

#### Scenario: TMDB logo
- **WHEN** a reader looks at the footer of any page
- **THEN** it shows the TMDB logo as a link named «TMDB» to `https://www.themoviedb.org`, smaller than the BookHaven logo in the header
