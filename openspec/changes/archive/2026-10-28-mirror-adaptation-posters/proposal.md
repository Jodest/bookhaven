# Proposal

## Why
Readers' browsers still load every Commons and TMDB poster from `upload.wikimedia.org` and `image.tmdb.org`. That makes poster loading slow and exposes readers' IP addresses to third parties. The book-cover mirror already solves the same problem for covers, so this change reuses it for posters. It also brings TMDB use in line with the TMDB API terms: no cached content older than 6 months, a purge when the licence ends, and the TMDB logo with the official notice.

## What Changes
- The cover mirror (worker queue `covers`, store `covers-data`, sweep, GC and volume marker) gains a second owner: screen work posters with source `commons` or `tmdb`. Copies are stored in the existing cover store and served from `/covers/...`. There is no new volume. The design leaves room for author photos as a third owner (proposal #676, out of scope).
- New mirror state columns on `screen_works`, modelled on `books.cover_*`. `poster_url` stays the source link. The API returns `/covers/...` once a copy is stored and the source link before that. Attribution fields are unchanged.
- The adaptation sync (api admin sync, worker catalog job, sync after a harvest) queues a poster copy whenever it sets a Commons or TMDB poster. The sweep backfills existing posters. When an admin removes an automatic poster or uploads one, the copy reference is dropped and GC deletes the file.
- `image.tmdb.org` joins the default `COVERS_ALLOWED_HOSTS`. New TMDB lookups use `w500` instead of `w342`, and the migration upgrades the existing `w342` links.
- TMDB compliance:
  - Every TMDB poster is re-checked through the TMDB API about 5 months after it was last confirmed, with the checks spread over time. A changed poster is re-downloaded. A missing poster, a missing API key or a missing TMDB id drops the poster. Nothing TMDB-sourced is kept beyond 175 days without a successful check.
  - A new maintenance command `tmdbpurge` removes every TMDB poster and copy, and supports `-dry-run`.
  - The footer shows the official TMDB logo and the notice from the terms. **BREAKING** (copy only): the footer notice text changes.
- `docs/DEPLOYMENT.md`: «Self-hosted covers» also covers posters; «Screen work posters» no longer says hotlinked; backup notes; the purge procedure.

## Capabilities

### New Capabilities
- `screen-works/poster-copies`: self-hosted copies of Commons/TMDB posters, the displayed poster link, queuing, TMDB re-check and the TMDB purge.

### Modified Capabilities
- `catalog/book-covers`: the shared download allowlist gains `image.tmdb.org`; retries, the sweep and storage cleanup cover poster copies too.
- `screen-works/book-adaptations`: the public `poster.url` can be a `/covers/...` path, and TMDB links are `w500`; the footer gets the TMDB logo and the reworded notice.
- `catalog-import/wikidata-adaptation-sync`: posters are copied rather than linked, TMDB posters use width 500, and the TMDB id of a used TMDB poster is kept for the re-check.

## Impact
- backend: migration `000044`, generalised `covers` module (owners), screenworks repository/service (copy columns, enqueue after sync, displayed URL), `pkg/tmdb` (w500), coverfetch default hosts, new `cmd/tmdbpurge`, api/worker wiring, Dockerfile.
- frontend: `apps/web` footer (TMDB logo SVG asset and notice). Poster rendering needs no change: `/covers/` paths already go through `next/image`, and admin previews them through its `/covers` rewrite.
- devops: no new volume. `tmdbpurge` binary in the backend image, run in the `worker` container.
- docs: `docs/DEPLOYMENT.md`.
