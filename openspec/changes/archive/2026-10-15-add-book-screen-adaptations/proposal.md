# Proposal

## Why
A reader on a book page cannot see that the book was filmed, although Wikidata knows it for many catalog books: 17 of 142 local books have 30 adaptations. This change is F1 of a three-part series: F1 is book adaptations, F2 is films about an author (biopics and documentaries), F3 is manual editing of adaptations in the admin. The data model is built so that F2 and F3 only add to it.

## What Changes
- New tables for screen works (films, series, cartoons, teleplays) and their links to books. A link can be hidden by an admin and records its source (`wikidata` now, `manual` in F3). A work records whether it was edited by hand.
- New admin endpoint `POST /api/v1/admin/books/sync-adaptations` `{ids}` (1 to 50 books). It finds adaptations in Wikidata through "based on" (P144). Only allowed work classes are taken: film, TV film, series, animated film or series, teleplay. Film series, games, list articles and episodes are excluded. The sync takes the earliest year, the title in the order Russian label, then English, then original title, the IMDb and Kinopoisk ids and the directors. Links are public at once, without moderation. The sync is idempotent by the work's QID, only adds and never changes the title or type. It fills only empty fields of works nobody edited by hand, and never brings back a link an admin hid.
- Admin actions «Найти экранизации в Wikidata»: one book in the book list, one or several books in the book moderation queue, and one book on the book edit page. Each shows a result banner, like author enrichment.
- A harvest job syncs adaptations of the books it created after the author enrichment step, on a best-effort basis. This also covers «Импортировать с книгами» and scheduled harvests.
- New public endpoint `GET /api/v1/books/{id}/adaptations` and a «Экранизации» block on the public book page with title, year, type, director and links to Кинопоиск and IMDb. The block is hidden when there is nothing to show.
- Minimal link moderation, so that wrong data published at once can be removed: `GET /api/v1/admin/books/{id}/adaptations` and `PATCH /api/v1/admin/books/{id}/adaptations/{screen_work_id}` `{hidden}`, plus a «Экранизации» section on the admin book edit page with «Скрыть» and «Показать». Creating and editing works by hand is F3.
- Admin book responses gain `external_id`. This is additive.
- No poster images are stored or shown, because their licenses are not free.

## Capabilities

### New Capabilities
- `catalog-import/wikidata-adaptation-sync`: the sync endpoint, the selection and data rules, idempotency, fill-empty-only and the hidden-link rule, and the admin sync actions with their banners.
- `screen-works/book-adaptations`: the public adaptations endpoint and the book page block, the admin link list and hide/show, and visibility rules. This starts a new `screen-works` domain that F2 (`screen-works/author-films`) and F3 will extend.

### Modified Capabilities
- `catalog-import/wikidata-book-harvest`: adds a requirement that a harvest job syncs adaptations of the books it created after the job.

## Impact
- backend: migration `000029_screen_works`; new module `screenworks` (domain, repository, service, transport); new Wikidata adaptation query in `bookdata`; the harvest handler calls the sync after the job; worker and api wiring; `external_id` on admin book responses; swagger.
- frontend: `packages/api-client`. In `apps/web`: a new `screen-work` entity and the block in `views/book-detail`. In `apps/admin`: a sync feature, a row action in the book list, row and bulk actions in the book moderation queue, and the «Экранизации» section on the book edit page.
- devops: none (reuses the Wikidata client configuration).
- docs: `docs/ROADMAP.md`. The new `screen-works` domain is added to the domain list in `openspec/config.yaml` and `docs/PIPELINE.md`.
