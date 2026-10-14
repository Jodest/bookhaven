# Proposal

Epic #413, feature 4 of 5: bring curated collections into the rest of the site.

## Why
Curated collections exist only behind `/top`. A reader looking at a book does not learn that it is «№3 в „Топ-100 книг всех времён“», and the home page does not mention collections. Both are cheap entry points into `/top`.

## What Changes
- New public endpoint `GET /api/v1/books/{id}/collections`. It returns the book's public places in published collections. Each place is computed with the same ranking as `/top/{slug}`, so reserve and hidden books never get a badge.
- The public book page shows badges «№{place} в „{title}“», each linking to the page of `/top/{slug}` that shows the place. The best places come first. Three are visible, and «Ещё {n}» reveals the rest.
- `GET /api/v1/collections` gets an optional `limit` query parameter. `total` still counts every published collection. This is not breaking: without `limit` the response is unchanged.
- The home page gets a «Подборки» block: up to 3 non-empty published collections as `/top` index cards, plus «Все подборки» → `/top`. The block is hidden when there is nothing to show, and it is independent of the `/home` payload.
- No database change, no new admin UI, no change to ranking or editing.

## Capabilities

### New Capabilities
- (none)

### Modified Capabilities
- `collections/curated-collections`: index `limit`; the book's collection places API; collection badges on the book page.
- `home/home-page`: the layout gets the «Подборки» block, which also renders when the home payload fails.
- `home/catalog-rows`: the new «Подборки» row.

## Impact
- backend: `curated` module (repository query, service, handler, route `GET /books/:id/collections`), index `limit`, swagger.
- frontend: `packages/api-client` (`collections.forBook`, `list({limit})`); `apps/web` entity `curated-collection` (badges, place link, card heading level), `views/book-detail`, new widget for the home row, `views/home`.
- devops: none.
- docs: `docs/ROADMAP.md` (epic #413 feature 4).
