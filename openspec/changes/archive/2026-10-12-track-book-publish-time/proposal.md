# Proposal

## Why

Books published from the harvest review queue keep their harvest-time creation date, so the home page row «Новое в каталоге» (ordered by creation date) can miss books that just became visible on the site. GitHub issue: [#245](https://github.com/Jodest/bookhaven/issues/245).

## What Changes

- Each book gets a first-publication time. It is set once, the first time the book becomes `published`, by any path: manual creation, file import, single status change, bulk status change, and the seed.
- Unpublishing (to `pending` or `rejected`) keeps the time. Republishing keeps the original time, so toggling a book's status cannot push it back to the top of «Новое в каталоге».
- Existing published books are backfilled with their creation time, so the home row looks the same right after deploy.
- «Новое в каталоге» on the home page orders by first-publication time, newest first, instead of by creation time.
- Admin book payloads gain a read-only, nullable `published_at`. Public payloads do not change.
- A bulk status change now also writes the first-publication time when it publishes a book for the first time. The requirement "Only the status changes" is modified to say so; its scenario keeps its name.
- Unchanged on purpose: the public book list sorts (default by rating, `sort=new` by publication year), the admin list ordering, and authors and genres (no publish time for them).

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `catalog-moderation/entity-status`: new requirements for the book's first-publication time and for the «Новое в каталоге» ordering.
- `catalog-moderation/bulk-status`: "Only the status changes" also allows the first-publication time to be written.

## Impact

- backend: new migration `000020` (column, backfill, partial index); catalog book domain; admin service and repository (create, update, bulk status); the admin book response; the home repository query; the seed; swagger regeneration.
- frontend: `AdminBook` in `packages/api-client` gains `published_at: string | null`. No UI change.
- devops: none. The migration runs through the usual `migrate up`.
- docs: `docs/features/home-page-content.md` ("Новое в каталоге" section and Risks) and `docs/ROADMAP.md` (#245 entry).
