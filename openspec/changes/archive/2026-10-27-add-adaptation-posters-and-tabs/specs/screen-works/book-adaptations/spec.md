## MODIFIED Requirements

### Requirement: Public adaptations endpoint
`GET /api/v1/books/{id}/adaptations` SHALL be open to everyone without a token. It returns 200 with `{items, total}`, the book's visible adaptations without pagination. Each item is `{id, title, original_title, kind, year, director, imdb_url, kinopoisk_url, poster}`:
- `kind` is one of `film`, `tv_film`, `series`, `animated_film`, `animated_series` and `teleplay`.
- Fields without a value are `null`.
- `imdb_url` is `https://www.imdb.com/title/<imdb id>/` and `kinopoisk_url` is `https://www.kinopoisk.ru/film/<kinopoisk id>/`.
- `poster` is `null` for a work without a poster, else `{url, source, author, license, license_url, source_url}`:
  - `source` is `manual` for a poster uploaded by an admin, `commons` for a Wikimedia Commons image and `tmdb` for a TMDB poster.
  - `url` is the site path `/posters/<shard>/<file>` for `manual`, an `https://upload.wikimedia.org/...` or `https://thumb.wikimedia.org/...` thumbnail for `commons`, and `https://image.tmdb.org/t/p/w342/<file>` for `tmdb`.
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

### Requirement: Adaptations block on the book page
The public book page SHALL show the book's visible adaptations in the «Экранизации» tab (see «Book page tabs») as a grid of cards. Each card shows, from top to bottom:
- a poster in a 2:3 frame, with the adaptation's title as its alternative text;
- the title;
- one wrapping line with the year, the type, «Режиссёр: <names>» when directors are known, and the links «Кинопоиск» and «IMDb» for the ids it has, opening in a new tab. The type is shown as «Фильм», «Телефильм», «Сериал», «Мультфильм», «Мультсериал» or «Телеспектакль».

A card without a poster, or whose poster image fails to load, shows a placeholder of the same 2:3 size instead. The placeholder has the type's icon and the year, or only the icon when the year is unknown. The adaptations MUST be part of the server-rendered page. When there is no visible adaptation, or the adaptations cannot be loaded, the «Экранизации» tab and its content MUST NOT be rendered and the rest of the page is unaffected.

#### Scenario: Book with adaptations
- **WHEN** a reader opens a published book with two visible adaptations, one with a Kinopoisk id, an IMDb id and a poster, and one from 1975 with neither id and no poster
- **THEN** the tab «Экранизации (2)» shows both cards, the first one with its poster and the links «Кинопоиск» and «IMDb», and the second one with the placeholder showing its type icon and «1975» and without links

#### Scenario: Nothing to show
- **WHEN** a reader opens a book whose only adaptation is hidden
- **THEN** the page has no «Экранизации» tab and «Рецензии» is selected

#### Scenario: Broken poster image
- **WHEN** the poster image of an adaptation fails to load in the reader's browser
- **THEN** the card shows the placeholder with the type icon and the year instead

#### Scenario: Server-rendered adaptations
- **WHEN** a crawler requests a book page with visible adaptations without running scripts
- **THEN** the returned HTML contains the titles of the adaptations

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

### Requirement: Verdict on the book page
Each adaptation card in the «Экранизации» tab of the public book page SHALL show the community verdict under its metadata line, with no divider line between them:
- From `min_votes` votes, «<P>% читателей: <leader> · <N> <голос|голоса|голосов>». The leader is the value with the most votes: «книга лучше», «экранизация лучше» or «одинаково». P is its share of all the votes, rounded to a whole number. When two values share the highest count, the line reads «Мнения разделились · <N> <голос|голоса|голосов>».
- Below `min_votes`, including no votes at all, no verdict line is shown. No placeholder text is shown instead.

When the verdicts cannot be loaded, no verdict or voting control is shown, and the rest of the tab is unaffected. The public author page («Фильмы об авторе») MUST NOT show verdicts or voting controls, because films about an author are not adaptations of a book.

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
- **THEN** the «Экранизации» tab lists the adaptations without verdict lines or voting controls

### Requirement: Voting on the book page
The «Экранизации» tab SHALL offer voting according to the visitor:
- A guest sees the link «Войдите, чтобы сравнить книгу и экранизацию» once, at the top of the tab, and not in each adaptation. The link leads to the login page and back to the book.
- A signed-in user who may vote sees, in each adaptation card under the verdict line (or under the metadata line when there is no verdict line), «Что лучше:» on the same line as a compact dropdown. No explanatory note is shown.
  - Without a vote the dropdown shows «Не выбрано» and offers «Книга», «Экранизация» and «Одинаково».
  - With a vote it shows the voted value and offers «Книга», «Экранизация», «Одинаково» and «Убрать голос».
  - Choosing another value changes the vote, and choosing «Убрать голос» removes it, after which the dropdown shows «Не выбрано».
  - The dropdown is disabled while a request runs. A failure shows «Не удалось сохранить голос.» and the dropdown keeps the previous value.
- A signed-in user who may not vote sees «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.» once, at the top of the tab. When they have a vote on an adaptation, that adaptation shows «Ваш голос: <Книга лучше|Экранизация лучше|Одинаково>» and the button «Убрать голос».

The tab-level prompts are not shown while the verdicts load or when they cannot be loaded. A user may vote once they have finished the book at least once, whatever its current status. After a vote changes, the verdict line updates without reloading the page. When the reader changes the book's reading status on the same page, the voting controls and prompts update without a reload.

#### Scenario: Reader votes
- **WHEN** a reader with the book on «Прочитано» chooses «Экранизация» in the dropdown of an adaptation that had no votes
- **THEN** the dropdown shows «Экранизация» and the line «100% читателей: экранизация лучше · 1 голос» appears without a reload

#### Scenario: Reader removes the vote
- **WHEN** the reader then chooses «Убрать голос» in the same dropdown
- **THEN** the vote is removed, the dropdown shows «Не выбрано» and no longer offers «Убрать голос»

#### Scenario: Guest prompt
- **WHEN** a guest opens a book with two adaptations that both have votes
- **THEN** each adaptation shows its verdict line and no dropdown, and «Войдите, чтобы сравнить книгу и экранизацию» appears once at the top of the «Экранизации» tab

#### Scenario: Not read yet
- **WHEN** a signed-in user who never finished the book opens a book with two adaptations
- **THEN** «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.» appears once at the top of the «Экранизации» tab, and no adaptation shows the dropdown

#### Scenario: Marking the book as read on the page
- **WHEN** that user marks the book «Прочитано» with the shelf buttons of the same page
- **THEN** the tab-level prompt disappears and the dropdown appears in each adaptation without a reload

#### Scenario: Vote kept after leaving «Прочитано»
- **WHEN** a reader who voted «Книга лучше» and finished the book only once removes it from «Прочитано» and looks at the adaptation
- **THEN** it shows «Ваш голос: Книга лучше» and «Убрать голос», and no dropdown

#### Scenario: Re-reading keeps the voting buttons
- **WHEN** a reader who voted «Книга лучше» presses «Читаю снова» on the same page
- **THEN** the dropdown stays and shows «Книга»

#### Scenario: Failed vote
- **WHEN** a reader with the vote «Книга» chooses «Одинаково» and the request fails
- **THEN** «Не удалось сохранить голос.» is shown and the dropdown shows «Книга» again

## ADDED Requirements

### Requirement: Book page tabs
The public book page SHALL show its lower sections as tabs, in this order: «Экранизации (<N>)», where N is the number of visible adaptations, then «Рецензии», then «Похожие книги».
- The «Экранизации» tab exists only when the book has at least one visible adaptation and the adaptations loaded. It is then selected by default. Otherwise «Рецензии» is selected by default.
- The address parameter `tab` with `adaptations`, `reviews` or `similar` selects that tab when the page opens, already in the server-rendered page. An unknown value, or `adaptations` for a book without that tab, selects the default tab.
- Selecting a tab shows its panel and hides the others. It puts its value into `tab` of the page address without reloading the page, without scrolling and without adding a history entry.
- Opening the page with the address fragment `#reviews` selects «Рецензии».
- The tabs MUST be accessible:
  - they are exposed as a tab list of tabs with a selected state, and each panel is labelled by its tab;
  - only the selected tab is in the Tab key order;
  - the Left and Right arrow keys move to and select the previous and next tab, wrapping around, and Home and End move to the first and last tab.
- A hidden panel keeps its content in the page, so the adaptations stay in the server-rendered HTML whatever tab is selected.
- The «Похожие книги» panel shows «Похожих книг пока нет.» when there are no similar books or they cannot be loaded.
- The review links of the rating summary, the review count and «Написать рецензию», select «Рецензии» and scroll it into view.
- The page's canonical address is the book address without `tab`.

#### Scenario: Book with adaptations opens on them
- **WHEN** a reader opens a book with three visible adaptations
- **THEN** the tabs are «Экранизации (3)», «Рецензии» and «Похожие книги» in this order, and «Экранизации» is selected

#### Scenario: Book without adaptations
- **WHEN** a reader opens a book without visible adaptations
- **THEN** the tabs are «Рецензии» and «Похожие книги», and «Рецензии» is selected

#### Scenario: Linked tab
- **WHEN** a reader opens a book with adaptations at an address with `?tab=reviews`
- **THEN** the server-rendered page has «Рецензии» selected and its panel shown, and the adaptations are in the page but hidden

#### Scenario: Switching tabs
- **WHEN** a reader clicks «Похожие книги»
- **THEN** its panel is shown, the address has `tab=similar`, and the page is neither reloaded nor scrolled

#### Scenario: Keyboard
- **WHEN** the focus is on the selected tab «Экранизации» and the reader presses the Right arrow key
- **THEN** «Рецензии» gets the focus and is selected

#### Scenario: Review count link
- **WHEN** a reader on the «Экранизации» tab clicks the review count «3 рецензии» in the rating summary
- **THEN** «Рецензии» is selected and scrolled into view

#### Scenario: Tab that does not exist
- **WHEN** a reader opens a book without adaptations at an address with `?tab=adaptations`
- **THEN** «Рецензии» is selected

#### Scenario: No similar books
- **WHEN** a reader selects «Похожие книги» for a book without similar books
- **THEN** the panel shows «Похожих книг пока нет.»

### Requirement: Adaptations link in the book header
When the book has at least one visible adaptation, the book header SHALL show, next to the rating summary, an accent link with a film icon and «<N> <экранизация|экранизации|экранизаций>». Activating it selects «Экранизации», puts `tab=adaptations` into the address and scrolls the tabs into view. Its target is the book page with `?tab=adaptations`, so it also works when opened in a new browser tab. Without visible adaptations the link MUST NOT be shown.

#### Scenario: Header link
- **WHEN** a reader on the «Рецензии» tab of a book with two visible adaptations clicks «2 экранизации» in the header
- **THEN** «Экранизации» is selected, the address has `tab=adaptations` and the tabs are scrolled into view

#### Scenario: No header link
- **WHEN** a reader opens a book without visible adaptations
- **THEN** the header has no adaptations link

### Requirement: Poster attribution
Wherever a poster is shown on the public site, a Wikimedia Commons poster SHALL be credited under its image as «Постер: <author>, <license>, Wikimedia Commons». The author part is left out when unknown, and the licence links to its address when known. «Wikimedia Commons» links to the file page. Both links open in a new tab. Uploaded and TMDB posters have no caption. The public site footer SHALL always show «Постеры фильмов — Wikimedia Commons и TMDB. This product uses the TMDB API but is not endorsed or certified by TMDB.», where «TMDB» links to `https://www.themoviedb.org`.

#### Scenario: Commons poster caption
- **WHEN** an adaptation's poster is a Commons file by «Jane Doe» under «CC BY-SA 4.0»
- **THEN** the card shows «Постер: Jane Doe, CC BY-SA 4.0, Wikimedia Commons» with links to the licence and the file page

#### Scenario: TMDB notice
- **WHEN** a reader opens any page of the public site
- **THEN** the footer contains «This product uses the TMDB API but is not endorsed or certified by TMDB.»
