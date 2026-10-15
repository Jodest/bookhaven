# Proposal

## Why
A reader on an author page cannot see the films made about the author, although Wikidata lists them through "main subject" (P921): for example «Толкин» (2019) for Tolkien and «Magic Beyond Words» (2011) for Rowling. This change is F2 of the screen works series. F1 (book adaptations, `add-book-screen-adaptations`) is done. F3, manual editing of works in the admin, comes later and is out of scope.

## What Changes
- A new link table between authors and the existing screen works. A link records its relation: `biopic` (a fiction film about the author) or `documentary`. It can be hidden by an admin and records its source (`wikidata` now, `manual` in F3). One work can be both a book adaptation and a film about an author. It is stored once per Wikidata QID.
- New admin endpoint `POST /api/v1/admin/authors/sync-screen-works` `{ids}` (1 to 50 authors). It finds video works whose main subject (P921) is the author's QID. It uses the F1 class whitelist and exclusions, plus documentary and biographical film classes. A work counts as documentary when its class or its genre (P136) is documentary. Links are published at once, without moderation. The sync is idempotent by the work's QID, only adds, fills only empty fields of works nobody edited by hand, and never brings back a link an admin hid.
- Admin actions «Найти фильмы об авторе в Wikidata»: on author list rows, in the author moderation queue (one row and the selected rows with a QID), and on the author edit page. Each shows a result banner.
- Auto sync, best-effort: after a direct Wikidata author import creates an author, and after a harvest job for the authors it created. A failure is only logged or counted.
- New public endpoint `GET /api/v1/authors/{id}/screen-works` and a «Фильмы об авторе» block on the public author page. It shows title, year, type, the mark «Документальный», director and links to Кинопоиск and IMDb. The block is hidden when there is nothing to show.
- Link moderation: `GET /api/v1/admin/authors/{id}/screen-works`, `PATCH /api/v1/admin/authors/{id}/screen-works/{screen_work_id}` `{hidden}`, and a «Фильмы об авторе» section on the admin author edit page with «Скрыть» and «Показать».
- An author merge moves the duplicate's film links to the canonical author and keeps every hide.
- Harvest job results gain `author_films_added` and `author_film_sync_errors`. This is additive.

## Capabilities

### New Capabilities
- `screen-works/author-films`: the public films-about-author endpoint and the author page block, the admin link list, hide and show, and visibility rules.
- `catalog-import/wikidata-author-film-sync`: the sync endpoint, which Wikidata items are films about an author, biopic or documentary, the data, idempotency and hidden-link rules, and the admin sync actions with their banners.

### Modified Capabilities
- `catalog-import/wikidata-author-import`: adds a requirement that a created author's films are synced after the import.
- `catalog-import/wikidata-book-harvest`: adds a requirement that a harvest job syncs the films about the authors it created.
- `catalog-moderation/author-merge`: adds a requirement that film links move to the canonical author and hides are kept.

## Impact
- backend: migration `000030_author_screen_works`; `screenworks` module gains the author links (repository, service, transport); new Wikidata author-film query in `bookdata`; the author import, the harvest handler and the author merge call or touch it; api and worker wiring; swagger.
- frontend: `packages/api-client`. In `apps/web`: an `AuthorFilms` component in `entities/screen-work` and the block in `views/author-detail`. In `apps/admin`: a sync feature, a row action in the author list, row and bulk actions in the author moderation queue, and the «Фильмы об авторе» section on the author edit page.
- devops: none (reuses the Wikidata client configuration).
- docs: `docs/ROADMAP.md`.
