# Proposal

GitHub issue: #508 (part 1 of 3 of «Книга или экранизация?»; #509 and #510 follow and are out of scope).

## Why

Adaptations are synced from Wikidata only for up to 50 selected books or for books a harvest/import creates, so books already in the catalog are never synced: 36 of 683 published books (5%) have an adaptation link. Without coverage the upcoming verdict feature (#509) is invisible, and readers cannot find books that have a film.

## What Changes

- New admin action «Найти экранизации для всего каталога» on «Импорт» → «Задачи»: queues one background job that syncs every published book with a Wikidata QID in chunks of 50, with the existing rules (add-only, hidden links stay hidden, hand-edited works untouched).
- New endpoint `POST /api/v1/admin/books/sync-adaptations/catalog` (admin + bearer): 201 with the task, 409 while a catalog sync is already queued or running.
- The job runs in the same single worker slot as harvest jobs and appears in `GET /api/v1/admin/harvest/jobs` and on `/import/jobs` with its progress and totals. Job items gain `kind` (`harvest` | `adaptation_sync`) and `adaptation_sync` (the sync report, `null` for harvests).
- Optional periodic catalog re-sync, off by default (`ADAPTATION_SYNC_SCHEDULE_ENABLED=false`), modelled on the scheduled harvest.
- Public catalog filter: `GET /api/v1/books?has_adaptation=true` returns only published books with at least one visible (not hidden) adaptation; the web catalog filter panel gets the checkbox «Есть экранизация» with a removable chip.
- No database migration.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `catalog-import/wikidata-adaptation-sync`: catalog-wide sync job, its report and progress, the scheduled re-sync and the admin action.
- `catalog-import/wikidata-book-harvest`: the job list and the jobs page also show catalog adaptation sync jobs.
- `screen-works/book-adaptations`: the «Есть экранизация» catalog filter (API and web).

## Impact

- backend: screenworks module (catalog sync job, tick, repository query), harvest transport (trigger endpoint, job list mapping), catalog list filter, worker wiring, config, swagger.
- frontend: admin (`/import/jobs` action and job rows, api-client/types); web (catalog filter panel, chip, URL param).
- devops: `ADAPTATION_SYNC_SCHEDULE_ENABLED`, `ADAPTATION_SYNC_SCHEDULE_CRON` in both docker-compose files (off by default).
- docs: `docs/DEPLOYMENT.md`, `docs/admin-guide/import-jobs.md`, `docs/admin-guide/screen-works.md`.
