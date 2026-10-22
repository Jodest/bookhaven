# Proposal

GitHub issue: #489 (reused as the epic), branch `feat/self-hosted-covers`.

## Why

Cover images hotlinked from Open Library take 2.3–2.9 s per cold request, because of two redirects to archive.org and no caching. The Next image cache is per width, lasts 60 s and is lost on deploy. Keeping our own copies of covers and serving them ourselves takes Open Library and archive.org out of page loads: a cold cover drops from about 2.5 s to tens of milliseconds.

## What Changes

- `books.cover_url` stays the **source** link. New columns on `books` record the copy: the source it was made from, the stored file, the state (`pending` / `stored` / `failed` / `absent`), attempts, the error, the next attempt time and the stored/checked times. Migration `000037` (forward-only).
- Every place the API shows a cover now returns our own `/covers/{shard}/{hash}.{ext}` while a copy of the current link is stored, `null` when the source says there is no cover, and the source link otherwise (fallback, so nothing breaks). The **admin book record** keeps returning the source link and gets a new `cover_copy` object.
- A new worker queue `covers` (concurrency 1, at least 1 s between downloads):
  - `covers:mirror` is queued after any save that writes a cover link: admin create/update, CSV/JSON import and Wikidata harvest with Open Library enrichment;
  - a periodic `covers:sweep` does backfill, backoff retries, recovery after a lost volume and cleanup of orphan files.
- Safe download:
  - only allowlisted hosts, http/https, ports 80/443;
  - private, loopback and link-local addresses are refused at dial time, also after redirects; at most 5 redirects;
  - 30 s timeout and a 5 MB limit;
  - JPEG/PNG/WebP checked by the bytes;
  - Open Library is asked with `default=false`, so its empty placeholder (404) counts as no cover.
- Storage behind an interface, so it can move to S3 later. Files are named by content hash and served by the backend at `GET /covers/*` as public files with `Cache-Control: public, max-age=31536000, immutable`. When a cover changes, the old file is garbage-collected.
- `apps/web` and `apps/admin` proxy `/covers/*` to the backend (like `/api`). Web raises `images.minimumCacheTTL`. Book page metadata makes a self-hosted cover absolute.
- The admin edit page shows a short cover copy status under «URL обложки».
- devops: named volume `covers-data`, which the worker writes and the backend reads (read-only), in both compose files. `COVERS_DIR` defaults to `./storage/covers` for native runs. The production image pre-creates `/data/covers` owned by nonroot.

## Capabilities

### New Capabilities

- `catalog/book-covers`: cover copies, the displayed cover link, queuing, download rules, retries, the sweep, storage cleanup and the public `/covers/*` endpoint.

### Modified Capabilities

- `catalog-admin/book-editing`: added requirements for the source link plus `cover_copy` on the admin book record, and the cover copy hint on the edit page.
- `catalog/book-page`: «Search and preview metadata». The Open Graph image and the structured-data image are absolute URLs, also for a self-hosted cover.

## Impact

- backend: new `covers` module (domain, repository, service with mirror/sweep/GC, HTTP file handler); `pkg/coverstore` (Store interface, filesystem and in-memory implementations); `pkg/coverfetch` (safe downloader); display-URL resolution in every handler that maps `cover_url` (catalog, shelves, home, curated, reviews if any); admin service queues copies; config `COVERS_*`; worker queue, cron and wiring; migration `000037`; swagger.
- frontend: `apps/web` next.config (rewrite, `minimumCacheTTL`), book SEO absolute URL (`SITE_URL`); `apps/admin` next.config rewrite, edit-page hint; `packages/api-client` admin `cover_copy` type.
- devops: `covers-data` volume and `COVERS_DIR` in `docker-compose.yml` and `docker-compose.prod.yml`; `SITE_URL` for web in prod; backend Dockerfile pre-creates `/data/covers`; `.gitignore` `backend/storage/`.
- docs: `docs/DEPLOYMENT.md`, `docs/admin-guide/adding-content.md`, `docs/ROADMAP.md`.
