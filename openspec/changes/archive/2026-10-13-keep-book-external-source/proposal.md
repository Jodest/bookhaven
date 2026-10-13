# Proposal

## Why

Saving a book in the admin clears its external source and id, because the update writes them from the request and the admin form never sends them. A harvested book that an admin edits (a common step before publishing it from the queue) loses the «Импортировано» mark and its harvest idempotency key, so a later harvest of the same author no longer recognises it, contrary to `catalog-import/wikidata-book-harvest` → "Idempotent, create-only harvesting". GitHub issue: [#378](https://github.com/Jodest/bookhaven/issues/378).

## What Changes

- A book update keeps the stored external source and id; nothing in an update can change them. They are set only when a book is created (by the harvest).
- No API shape change: the update request never had these fields.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `catalog-admin/book-editing`: "External source on update" now keeps the source and id.

## Impact

- backend: `admin/service.UpdateBook` and its tests.
- frontend, devops, docs: none.
