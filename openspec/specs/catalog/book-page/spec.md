# catalog/book-page Specification

## Purpose
Defines how the public book page `/books/{slug}` presents a published book. This covers the layout, the rating summary, the description, the book details, the genre links, the sign-in links and the metadata that search engines and link previews read.

## Requirements

### Requirement: Page layout
The public book page SHALL arrange a published book in two parts: a cover-and-actions part and a content part.
- **Cover and actions.** On wide screens (768 px and up) the left column holds the cover. Under the cover, in this order, it holds the reading status control, «На полку» and the reader's own rating. There is no lists control. This column stays in view while the whole page scrolls, and scrolls inside itself when it is taller than the screen.
- **Content.** The right column holds, in this order, the title (the page's only level-1 heading), the authors, the rating summary, the collection badges, the «Описание» block, the book details and the genres. The two columns run to the end of the page.
- **Sections.** After the genres, in the right column, come «Экранизации», then «Похожие книги», then «Рецензии». They are stacked one under another and all visible at once, never as tabs, and each has a visible heading. Each section is shown only under its own rules (see `screen-works/book-adaptations` and `reviews/book-reviews`). All section headings on the page share one heading style.
- **Similar books.** «Похожие книги» is a single row of small book covers, each with the book's title under it and linking to that book's page. The row is about one cover high and scrolls horizontally when it does not fit. The row MUST be exposed as a list and MUST be reachable with the Tab key, and it scrolls with the arrow keys once focused. When there are no similar books, or they cannot be loaded, the section is not shown and no empty-state text appears.
- **Anchors.** The address fragment `#adaptations` leads to «Экранизации» and `#reviews` leads to «Рецензии». A section reached through an anchor is scrolled below the site header, not under it. The address parameter `tab` has no effect on the page.
- **Mobile.** On narrow screens the cover takes about 40% of the width and sits beside the title, the authors and the rating summary. The actions follow at full width, then the rest of the content and the sections in the same order. Nothing is pinned on narrow screens. The cover MUST NOT take the full screen width.
- **No cover badge.** The cover MUST NOT carry a rating badge; the rating is shown only in the rating summary.
- **No cover.** A book without a cover shows «Нет обложки» in the cover place.
- **Independent loading.** The adaptations and the collection places are loaded independently of each other. A failure of either MUST NOT prevent the page from rendering.

#### Scenario: Section order
- **WHEN** a visitor opens a published book with adaptations, similar books and reviews
- **THEN** after the book's content, in the right column, the sections «Экранизации», «Похожие книги» and «Рецензии» appear in this order, all visible at once without any tabs

#### Scenario: No rating badge on the cover
- **WHEN** a visitor opens a book rated 4.5
- **THEN** the cover shows no rating, and 4.5 appears only in the rating summary

#### Scenario: Both side requests fail
- **WHEN** both the adaptations request and the collection places request fail
- **THEN** the page renders the book with no «Экранизации» block, no badges and no error message

#### Scenario: No lists control
- **WHEN** a signed-in reader opens a book page
- **THEN** the actions show the status control, «На полку» (when they have custom shelves) and the rating, and neither «В список» nor «Добавить в список»

#### Scenario: Similar books row
- **WHEN** a visitor opens a book with eight similar books on a narrow screen
- **THEN** «Похожие книги» shows one row of small covers with titles that scrolls horizontally, and pressing Tab focuses the row so the arrow keys scroll it

#### Scenario: No similar books
- **WHEN** a visitor opens a book without similar books
- **THEN** the page has no «Похожие книги» section and «Похожих книг пока нет.» appears nowhere

#### Scenario: Reviews anchor
- **WHEN** a visitor opens the book address with `#reviews`, or clicks «3 рецензии» in the rating summary
- **THEN** the page scrolls to the «Рецензии» section

#### Scenario: Old tab address
- **WHEN** a visitor opens a book address with `?tab=similar`
- **THEN** the page is the same as without the parameter, with all sections stacked in the usual order

### Requirement: Rating summary
Under the authors, the page SHALL show a one-line rating summary.
- **Rating.** When the book has ratings, the summary shows a star, the average with one decimal and the count of ratings, e.g. «★ 4.3 · 12 оценок». A book without ratings shows «Пока нет оценок» instead.
- **Reviews.** The summary also shows the count of visible reviews, e.g. «5 рецензий», as a link that moves to the «Рецензии» section of the same page. A book without reviews shows «Написать рецензию» as the link instead.
- **Plural forms.** Both counts MUST use the Russian plural forms: «1 оценка», «2 оценки», «5 оценок», «11 оценок», «21 оценка», «22 оценки», and «1 рецензия», «3 рецензии», «12 рецензий» likewise.

#### Scenario: Rated and reviewed book
- **WHEN** a visitor opens a book with average 4.25 from 21 ratings and 3 visible reviews
- **THEN** the summary reads «4.3 · 21 оценка · 3 рецензии», and «3 рецензии» leads to the «Рецензии» section

#### Scenario: Book without ratings or reviews
- **WHEN** a visitor opens a book with no ratings and no reviews
- **THEN** the summary shows «Пока нет оценок» and the link «Написать рецензию» to the «Рецензии» section

### Requirement: Description block
The «Описание» block SHALL show the book's own description, or the approved AI description with its notice (see `catalog-admin/ai-descriptions`).
- **Paragraphs.** Line breaks in the text MUST be kept as paragraph and line breaks.
- **Long text.** A long description (more than 400 characters or more than four line breaks) is first shown collapsed to about six lines, with the button «Ещё». «Ещё» expands the full text and becomes «Свернуть», which collapses it again. A short description shows no button.
- **Complete page source.** The full text is part of the page in both states.

#### Scenario: Long description
- **WHEN** a visitor opens a book whose description has 900 characters
- **THEN** the description is collapsed with «Ещё», and pressing «Ещё» shows the whole text and the button «Свернуть»

#### Scenario: Paragraphs kept
- **WHEN** a description contains two paragraphs separated by a line break
- **THEN** the page shows them as two separate paragraphs

#### Scenario: AI description stays labelled
- **WHEN** a book without its own description has a long approved AI description
- **THEN** the collapsed text is followed by «Описание создано ИИ и может содержать неточности.»

### Requirement: Book details
The page SHALL list the book's details as term–value pairs.
- **Terms.** In this order: «Год издания», «Страниц», «Язык», «ISBN».
- **Missing values.** A term without a value is omitted.
- **Language.** The language is shown by its Russian name with a capital letter, e.g. «Русский» or «Английский». An unknown code is shown as is.

#### Scenario: Language shown
- **WHEN** a visitor opens a book published in 1862 with language `ru` and no page count or ISBN
- **THEN** the details show «Год издания» 1862 and «Язык» «Русский», and no «Страниц» or «ISBN»

### Requirement: Genre links
Each genre of the book SHALL be shown as a chip that links to the catalog filtered by that genre, `/books?genre_id={genre id}`.

#### Scenario: Clicking a genre
- **WHEN** a visitor clicks the genre «Роман» on a book page
- **THEN** the catalog opens filtered by «Роман»

### Requirement: Sign-in links return to the book
Every sign-in link on the book page SHALL lead to `/login` with the URL-encoded book path as the `redirect` parameter, e.g. `/login?redirect=%2Fbooks%2Fotcy-i-deti`. After signing in, the reader returns to the book page. This covers the status control, «Войдите, чтобы оценить» and «Войдите, чтобы написать рецензию».

#### Scenario: Guest rates
- **WHEN** a visitor who is not signed in clicks «Войдите, чтобы оценить» on `/books/otcy-i-deti`
- **THEN** they go to `/login?redirect=%2Fbooks%2Fotcy-i-deti`

### Requirement: Search and preview metadata
The book page SHALL provide its own document metadata.
- **Title.** The page title is «{title} — {authors, comma-separated} — BookHaven». When the book has no authors it is «{title} — BookHaven».
- **Description.** The meta description is the book's own description cut to about 160 characters at a word boundary, with «…» added when cut. Without an own description it is «{title}: оценки, рецензии и экранизации на BookHaven.».
- **Open Graph.** The Open Graph data repeats the title and the description. The book cover is the Open Graph image when the book has one. The image MUST be an absolute URL: a self-hosted cover (`/covers/…`) is prefixed with the public site's origin.
- **Structured data.** The page embeds schema.org `Book` structured data with:
  - the name and the authors (as `Person`);
  - the ISBN, the page count, the publication year and the language, when known;
  - the cover image, when the book has one, as the same absolute URL as the Open Graph image;
  - an `aggregateRating` (`ratingValue`, `ratingCount`, best 5, worst 1) only when the book has at least one rating.
- **No AI text.** An AI-generated description MUST NOT appear in the page metadata or the structured data.
- **Unknown book.** An unknown or unpublished book keeps the not-found page.

#### Scenario: Metadata of a rated book
- **WHEN** a crawler fetches a published book with a cover, two authors and 12 ratings
- **THEN** the title names the book and both authors, the cover is the Open Graph image, and the structured data has `aggregateRating.ratingCount` 12

#### Scenario: No ratings, no aggregate
- **WHEN** a crawler fetches a published book without ratings
- **THEN** the structured data has no `aggregateRating`

#### Scenario: Only an AI description
- **WHEN** a published book has only an approved AI description
- **THEN** the meta description is «{title}: оценки, рецензии и экранизации на BookHaven.» and the structured data has no description

#### Scenario: Self-hosted cover in metadata
- **WHEN** a crawler fetches a published book whose cover is served from `/covers/ab/abc….jpg` on the site `https://bookhaven.example`
- **THEN** the Open Graph image and the structured-data image are both `https://bookhaven.example/covers/ab/abc….jpg`
