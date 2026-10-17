## MODIFIED Requirements

### Requirement: Page layout
The public book page SHALL arrange a published book in two parts: a cover-and-actions part and a content part.
- **Cover and actions.** On wide screens (768 px and up) the left column holds the cover. Under the cover, in this order, it holds the reading status control, «На полку» and the reader's own rating. There is no lists control. This column stays in view while the whole page scrolls, and scrolls inside itself when it is taller than the screen.
- **Content.** The right column holds, in this order, the title (the page's only level-1 heading), the authors, the rating summary, the collection badges, the «Описание» block, the book details and the genres. The two columns run to the end of the page.
- **Sections.** After the genres, in the right column, come «Экранизации», then «Похожие книги», then «Рецензии». Each section is shown only under its own rules (see `screen-works/book-adaptations` and `reviews/book-reviews`). All section headings on the page share one heading style.
- **Mobile.** On narrow screens the cover takes about 40% of the width and sits beside the title, the authors and the rating summary. The actions follow at full width, then the rest of the content and the sections in the same order. Nothing is pinned on narrow screens. The cover MUST NOT take the full screen width.
- **No cover badge.** The cover MUST NOT carry a rating badge; the rating is shown only in the rating summary.
- **No cover.** A book without a cover shows «Нет обложки» in the cover place.
- **Independent loading.** The adaptations and the collection places are loaded independently of each other. A failure of either MUST NOT prevent the page from rendering.

#### Scenario: Section order
- **WHEN** a visitor opens a published book with adaptations, similar books and reviews
- **THEN** after the book's content, in the right column, the sections «Экранизации», «Похожие книги» and «Рецензии» appear in this order

#### Scenario: No rating badge on the cover
- **WHEN** a visitor opens a book rated 4.5
- **THEN** the cover shows no rating, and 4.5 appears only in the rating summary

#### Scenario: Both side requests fail
- **WHEN** both the adaptations request and the collection places request fail
- **THEN** the page renders the book with no «Экранизации» block, no badges and no error message

#### Scenario: No lists control
- **WHEN** a signed-in reader opens a book page
- **THEN** the actions show the status control, «На полку» (when they have custom shelves) and the rating, and neither «В список» nor «Добавить в список»

### Requirement: Sign-in links return to the book
Every sign-in link on the book page SHALL lead to `/login` with the URL-encoded book path as the `redirect` parameter, e.g. `/login?redirect=%2Fbooks%2Fotcy-i-deti`. After signing in, the reader returns to the book page. This covers the status control, «Войдите, чтобы оценить» and «Войдите, чтобы написать рецензию».

#### Scenario: Guest rates
- **WHEN** a visitor who is not signed in clicks «Войдите, чтобы оценить» on `/books/otcy-i-deti`
- **THEN** they go to `/login?redirect=%2Fbooks%2Fotcy-i-deti`
