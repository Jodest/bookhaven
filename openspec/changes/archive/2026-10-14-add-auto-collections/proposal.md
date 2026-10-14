# Proposal

Epic #413, feature 3 of 5.

## Why

Manual collections need an admin to pick and order every book. BookHaven already has the data for rankings such as «Топ-100 BookHaven» (reader ratings, «Прочитано» and «Хочу прочитать» shelves), so collections should be able to build themselves from a rule and stay current without manual work.

## What Changes

- New collection kind `auto`, chosen at creation and fixed afterwards. An automatic collection has a **rule** instead of a hand-made book list. The rule has a metric, an optional genre filter (the collection's linked genre), an optional period, the size N and, for the rating metric, a minimum rating count and a confidence threshold.
- v1 metrics:
  - `weighted_rating`: Bayesian (IMDb-style) weighted rating `(v·R + m·C)/(v + m)`, default m = 10 and min ratings = 3.
  - `most_read`: how many readers have the book on «Прочитано».
  - `most_wanted`: how many readers have it on «Хочу прочитать».
  - `trending`: ratings and reading-status shelvings in the last N days, 30 by default.
  - «Most reviewed» is deferred because every review already carries a rating, so it adds little over the rating metric.
- Rankings are stored as a **snapshot** in the existing collection items. The worker recomputes the snapshot on a schedule: daily by default, controlled by `AUTO_COLLECTIONS_SCHEDULE_ENABLED` / `AUTO_COLLECTIONS_SCHEDULE_CRON`. Places therefore stay stable during the day. The snapshot keeps a small reserve. Between recomputes an unpublished book leaves its place and the rest shift up, the same as in manual collections.
- Each snapshot remembers every book's position in the previous snapshot. The public page shows movement: «↑3», «↓2», «Новинка». Changing the rule, genre or size resets the baseline, so no movement is shown until the next recompute.
- New admin action `POST /admin/collections/{id}/recompute` («Пересчитать сейчас»). Creating an automatic collection, or changing its rule, genre or size, recomputes it immediately.
- Admin «Подборки»:
  - The kind is chosen on create.
  - Automatic collections get a rule form, the last computed time and a preview of the current snapshot.
  - Book add/remove/reorder, bulk paste and bulk add return 422 «collection is automatic» for automatic collections.
- Public API and pages:
  - Expose `rule` and `computed_at` on collections and `movement` on places.
  - `/top/{slug}` shows the movement and the line «Составлена автоматически по … Обновлено …».
  - `/top` lists automatic collections like any other.
- Out of scope: the «№N в …» book badge and the home block (feature 4), and AI drafting (feature 5).

## Capabilities

### New Capabilities

### Modified Capabilities
- `collections/curated-collections`: adds the `auto` kind, the ranking metrics with the weighted rating, the stored snapshot with scheduled recompute, position movement, and `rule` / `computed_at` / `movement` in the public API and pages.
- `collections/curated-collection-admin`: kind and rule on create/edit, the recompute action, the auto fields in the admin detail and list, manual-only book editing, and the automatic collection admin page.

## Impact

- **backend**:
  - Migration `000027_auto_collections`, which widens the `kind` CHECK and adds the rule, snapshot-time and previous-position columns.
  - The `curated` module (domain ranking, repository aggregates, snapshot write, service, handlers).
  - A new worker queue/server with a scheduled task, and config.
- **frontend**:
  - `packages/api-client` types and calls.
  - `apps/web` movement badge and auto caption.
  - `apps/admin` kind/rule form, snapshot widget and recompute button.
- **devops**: new worker env vars in `docker-compose.yml` / `docker-compose.prod.yml`.
- **docs**: `docs/DEPLOYMENT.md` env vars, `docs/ROADMAP.md` epic #413 progress, regenerated swagger.
