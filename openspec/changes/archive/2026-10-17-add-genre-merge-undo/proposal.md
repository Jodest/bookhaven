# Proposal

## Why

A genre merge (single or batch) is currently irreversible and leaves no record of what it changed, so a wrong merge can only be repaired by hand. Admins need a journal of merges and a safe one-click undo.

## What Changes

- Every single and batch merge writes a merge journal entry in its own transaction: who, when, target, and per source the previous status/QID, QID carry-over, moved books, books whose source link was dropped as a duplicate, re-pointed aliases and switched collections. A batch merge is one entry.
- `POST /api/v1/admin/genres/{id}/merge` and `POST /api/v1/admin/genres/merge-batch` responses gain `merge_id` (additive).
- New admin-only endpoints: `GET /api/v1/admin/genres/merges` (history), `GET /api/v1/admin/genres/merges/{id}/undo-preview`, `POST /api/v1/admin/genres/merges/{id}/undo`.
- Undo restores the snapshot in one transaction, skipping whatever no longer matches it (deleted books, removed links, reconfigured collections); a batch is undone as a whole. Undo is refused with 409 and a `code` when it cannot be done safely (already undone, target gone or merged away, source gone or no longer an alias of the target, QID taken by another genre).
- Merges made before this change have no journal entry and cannot be undone.
- Admin app: new page «История слияний жанров» (`/moderation/genres/merges`) linked from the genre queue, with an «Отменить слияние» dialog and preview; merge success messages offer «Отменить». The merge dialogs no longer say the merge is irreversible.
- Migration 000033 (additive, three new tables).

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `catalog-moderation/genre-merge`: merge responses return `merge_id`; merges are journaled; merge history, undo preview and undo endpoints; history page and undo dialog; dialog texts no longer call the merge irreversible.

## Impact

- backend: admin module repository (journal write inside the merge transaction, undo, preview, history), service, handlers, router, swagger; migration 000033.
- frontend: `packages/api-client` (types + methods), `apps/admin` (entities/genre api, new feature `undo-genre-merge`, new view `genre-merge-history`, new route, small edits in `views/genre-moderation`, `features/merge-genre`, `widgets/moderation-queue`).
- devops: none (migration runs with the usual `migrate up`); see the migration-number note in design.md.
- docs: `docs/admin-guide/duplicates.md`.
