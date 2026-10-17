# Proposal

## Why

Cleaning up duplicate genres one by one is slow: an admin often finds several spellings of one genre on the same queue page. A genre merge also leaves curated collections pointing at the merged genre, so an automatic collection filtered by it ranks nothing on its next recompute.

## What Changes

- New `POST /api/v1/admin/genres/merge-batch`: merge many source genres into one target in a single all-or-nothing transaction, with the same per-source behavior as the existing single merge.
- New read-only `POST /api/v1/admin/genres/merge-preview`: for a set of sources and a target, returns the number of books that would move and the curated collections that would be switched.
- Both the single and the batch merge re-point curated collections linked to a source genre to the target (no deletion, merge never refused because of collections). Automatic collections that were switched are recomputed right after the merge, as when their genre is edited.
- The single-merge response gains `collections_switched` (additive).
- One-off data fix: collections still linked to a genre that was merged earlier are switched to its target.
- Admin: «Слить выбранные» in the genre moderation queue opens a batch merge dialog (target by radio among the selection or by search); both merge dialogs show a preview summary with the books count and «Подборка «…» будет переключена на жанр «…».» lines before confirmation.

## Capabilities

### New Capabilities

### Modified Capabilities

- `catalog-moderation/genre-merge`: batch merge endpoint and dialog, merge preview endpoint, curated collections follow a merge, `collections_switched` in the single-merge response, preview summary in the single merge dialog.

## Impact

- backend: admin module (repository, service, handler, routes), a small public method on the curated service for recomputing switched automatic collections, wiring in `cmd/api`, one forward-only data migration, swagger regeneration (additive).
- frontend: `packages/api-client` (new types and calls), `apps/admin` merge-genre feature (preview, batch dialog) and genre moderation view.
- docs: `docs/admin-guide/duplicates.md` (batch merge, collections switching).
- devops: none.
