# Proposal

## Why
Screen adaptations are a BookHaven feature competitors lack, but on the book page they are a plain text list at the bottom. This change makes them a main content type: a default tab with poster cards, a vote from the first reader and a lighter voting control. It reverses the F1 decision "no posters" on the owner's explicit request (the project is non-commercial).

## What Changes
- **BREAKING (contract value)**: the verdicts endpoint returns `min_votes: 1`, so counts are revealed from the first vote. The threshold stays one constant, so 5 can be restored with one edit. With 0 votes the book page shows no verdict line; «Пока мало голосов» is gone.
- The voting control becomes a compact dropdown «Что лучше: [Не выбрано ▾]» with «Книга», «Экранизация», «Одинаково», plus «Убрать голос» when a vote exists. The divider line above the verdict is removed.
- The bottom of the book page becomes accessible tabs «Экранизации (N)» · «Рецензии» · «Похожие книги»:
  - «Экранизации» is first and selected by default when the book has visible adaptations, otherwise the tab is absent and «Рецензии» is the default;
  - the selected tab is in `?tab=`, and the adaptations stay server-rendered;
  - an accent link «N экранизаций» in the book header selects the tab and scrolls to it.
- Every adaptation and author film can carry a poster, shown as a 2:3 card. Sources in priority order:
  - an admin upload, stored on our server;
  - a free Wikimedia Commons image from Wikidata (P3383, then P18), hotlinked with attribution;
  - a TMDB poster resolved from the Wikidata TMDB ids (P4947/P4983), hotlinked. This step only runs when `TMDB_API_KEY` is set.
  - A styled placeholder with the type icon and year is shown when there is no poster.
- The adaptation sync fills posters (empty only, never over an admin upload or an admin removal), so re-running it backfills existing works. Sync reports gain a `posters` counter.
- New admin endpoints and a «Постер» section on the work edit page, to upload and remove a poster.
- Site footer shows the TMDB attribution notice.

## Capabilities

### New Capabilities
- none

### Modified Capabilities
- `screen-works/book-adaptations`: changes the endpoint item (`poster`), the poster card grid, book-page tabs, the header link, `min_votes` 1, the verdict line without a placeholder, the dropdown vote control and the poster attribution and TMDB notice.
- `screen-works/author-films`: changes the endpoint item (`poster`) and makes the author page use poster cards.
- `screen-works/admin-editing`: adds `poster` and `poster_removed` to the work detail, poster upload and removal endpoints, and a «Постер» section on the work edit page.
- `catalog-import/wikidata-adaptation-sync`: adds poster sources and fill rules, `posters` counters in the endpoint, the catalog job result and the admin banners.

## Impact
- backend: migration `000043_screen_work_posters` (forward-only); screenworks domain, repository, service, transport; `bookdata` SPARQL and Commons filename parsing; new `tmdb` client; reuse of `coverstore` FS for a separate posters store; public `/posters/...` file route; config `TMDB_API_KEY`, `POSTERS_DIR`; body-limit override for the upload route.
- frontend (apps/web): book page tabs and header link, poster cards, dropdown vote control, verdict line, author page cards, footer notice, `/posters` rewrite, canonical URL.
- frontend (apps/admin): «Постер» section on the work edit page, sync banners with the poster count, `/posters` rewrite.
- packages: api-client types and admin poster methods.
- devops: `TMDB_API_KEY` in `.env.example` (empty) and passed through compose to api and worker. New `posters-data` volume mounted read-write in the api, in both compose files. No prod secret is set.
- docs: `docs/DEPLOYMENT.md` (posters volume, TMDB key, backup note), swagger regeneration.
