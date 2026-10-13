# Proposal

## Why

Epic «Автоматическое наполнение каталога», feature 1 of 4: the catalog only grows when an admin triggers a harvest by hand, so new books of authors already in the catalog are never picked up. A periodic worker task should re-harvest chosen authors in small, throttled batches, with every result landing as `pending` in the existing moderation queue. Prerequisite: fix #191, where the harvest worker runs several jobs at once despite the documented concurrency 1.

## What Changes

- Fix #191: the harvest worker processes at most one harvest job at a time, regardless of CPU count.
- New per-author flag «автоматически пополнять из Wikidata», which an admin sets on the author edit page. It can be set only on an author that has a Wikidata QID. Existing authors start with it off.
- New scheduled harvest, off by default and switched on by env. On each run (daily by default) the worker picks a bounded batch of flagged, published authors with a QID. Authors never auto-harvested come first, then the least recently harvested ones. An author is never re-picked within 30 days. For the batch, the worker enqueues one ordinary harvest job through the existing pipeline. A run is skipped if a scheduled job is still pending, active or retrying.
- Harvest jobs record how they were started (`manual` / `scheduled`) and which author QIDs they cover. The jobs page shows an «Авто» / «Вручную» badge.
- Automation never publishes anything. Harvested books stay `pending`, and the create-only matching rules (QID first, rejected tombstones kept) are unchanged.
- Admin author payloads gain `auto_harvest` and `last_auto_harvested_at`. `PATCH /admin/authors/{id}` accepts `auto_harvest`, and leaving it out keeps the current value.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `catalog-import/wikidata-book-harvest`: the serialized processing guarantee is now actually enforced (#191); a new scheduled-harvest requirement; jobs carry a trigger and author QIDs; the jobs page shows the trigger.
- `catalog-admin/author-editing`: the auto-harvest flag on the update endpoint and on the edit page.

## Impact

- backend: migration (two author columns), admin author service/repository/transport, harvest service (schedule tick handler, payload metadata), harvest transport (job trigger fields), worker wiring (concurrency fix, new schedule server, asynq client/inspector), config env.
- frontend: apps/admin author edit form (toggle), harvest jobs view (trigger badge), packages/api-client and packages/types.
- devops: `HARVEST_SCHEDULE_ENABLED`, `HARVEST_SCHEDULE_CRON`, `HARVEST_SCHEDULE_BATCH` in docker-compose files (default off).
- docs: docs/DEPLOYMENT.md env vars, docs/ROADMAP.md (#191 done, epic entry), worker and harvest package doc comments.
