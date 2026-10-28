## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: Book page tabs
**Reason**: Readers do not open tabs, so reviews and similar books hidden behind them went unseen. The book page goes back to stacked sections that are all visible at once (see `catalog/book-page` «Page layout»).
**Migration**: The `tab` address parameter is no longer handled and is ignored. Only local links ever used it, and no production links exist. `#reviews` and the new `#adaptations` are plain in-page anchors. The review links of the rating summary and the header adaptations link use these anchors. The canonical address stays the book address.
