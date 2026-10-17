# Proposal

## Why

The public book page today is one long column of same-looking blocks, so the description sits far down and reviews and adaptations are squeezed into a narrow column. It also has real bugs: a hydration mismatch in the star widget, «N оценок» with no plural forms, an invalid `dt`/`dd` markup, login links that are not URL-encoded, and no page metadata at all. This change redesigns the page into a cover-and-actions column plus a content column, and fixes those defects. It is frontend only (apps/web) and covers the book-page part of roadmap item #199 (SEO metadata).

## What Changes

- New layout. On desktop a sticky cover sits over the reader actions. The content column holds the title, authors, a rating summary, collection badges, the description, the book details and the genres. Below them, full width: «Экранизации», then «Похожие книги», then «Рецензии». On mobile a compact cover sits beside the title. The rating badge on the cover is removed.
- A one-line rating summary: «★ 4.3 · 12 оценок · 5 рецензий». The counts use correct Russian plural forms, and the reviews part jumps to the «Рецензии» section.
- The description keeps its paragraphs. A long one is collapsed to about six lines with «Ещё» / «Свернуть». The AI label stays.
- The book details become a proper definition list and now include «Язык». Each genre links to the catalog filtered by it.
- **BREAKING (UI)**: the three status buttons on the book page become one status control. Custom shelves move behind «На полку». Placeholders hold the controls' place while the reader's shelves load. Book cards (the «Книга дня» card) keep their buttons.
- Lists («Добавить в список») are unchanged; they only move into the actions column.
- Login links on the page carry an encoded return path to the book.
- New page metadata: the title, the description and the og:image (the cover). The page also gets schema.org `Book` JSON-LD.
- The star widget's hydration mismatch is fixed. The adaptations and collection-places requests run in parallel.

## Capabilities

### New Capabilities

- `catalog/book-page`: layout, rating summary, description, book details, genre links, sign-in links and search metadata of the public book page.

### Modified Capabilities

- `collections/shelf-books`: «Shelving from the book page» now has a single status control, a «На полку» popover and loading placeholders on the book page.
- `reviews/book-ratings`: «Rating widget» moves into the actions column and updates the page's rating summary after a change.

## Impact

- frontend: apps/web (the book-detail view, the `/books/[slug]` route, features shelve-book / rate-book, widget reviews-section, a one-line login-href change in add-to-list, shared lib). packages/ui gets a Popover primitive, adding the dependency `@radix-ui/react-popover`.
- backend: none. No API or DB change.
- devops: none (only the pnpm lockfile changes).
- docs: openspec config domain list.
