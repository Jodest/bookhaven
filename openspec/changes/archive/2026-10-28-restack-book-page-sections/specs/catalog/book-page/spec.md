## MODIFIED Requirements

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
