# Proposal

## Why

A book file import silently creates every author (as published) and genre (as pending) it cannot match by name. The report lists only per-row book outcomes, so near-duplicate authors and new pending genres go unnoticed until someone finds them in the catalog. This was an open risk in `docs/features/catalog-manual-import.md`.

## What Changes

- The `POST /api/v1/admin/books/import` report gains two always-present arrays, `created_authors` and `created_genres`. Each entry is `{id, name, row}`: the entity this import created and the 1-based data row that created it. The change is additive: existing fields are unchanged and nothing is **BREAKING**.
- Entities are listed even when their row later failed validation or was skipped as an existing book, because they stay in the catalog.
- The admin page `/import/books/file` shows «Созданные авторы» and «Созданные жанры» lists in the result view. Each entry links to its edit page, and the genres list notes that new genres are pending.
- No change to matching or autocreate rules, and no database change.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `catalog-import/book-file-import`: the report shape gains `created_authors`/`created_genres` with their semantics, and the file import page shows them.

## Impact

- backend: admin service (file import report and author/genre name resolution), regenerated Swagger.
- frontend: `packages/api-client` types, the admin book file import widget.
- devops: none.
- docs: Swagger only. The frozen `docs/features/*` contracts are not edited.
