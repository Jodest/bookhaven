# Proposal

## Why

After a reader marks a book «Прочитано» on the book page and navigates back to `/` without a reload, the home card «Читательский вызов» (and the personal blocks) keep showing the old figures for up to a minute, because shelf changes on the web app bypass the client cache. The backend count is live and correct; only the web client is stale.

## What Changes

- Query-key factories for shelves, the reading challenge and the personal home payload live in the corresponding entities; every cached query and every invalidation uses them (no literal keys).
- All reader-side shelf work in `apps/web` (book-page / card shelving, «Мои полки», own shelf page) goes through the shared query cache with queries and mutations instead of ad-hoc local state.
- Any change of which books are on the reader's shelves (status pick, custom-shelf toggle, «Убрать с полки», shelf deletion) marks the reader's shelves, the reading challenge and the personal home payload as stale, so the next view of `/` or of a shelf shows current data. Other shelf edits (rename, description, privacy) refresh shelf data only.
- Optimistic toggles and rollback on failure behave exactly as today.
- No user-visible change other than the fix; no API change. Not **BREAKING**.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `reading-challenges/progress`: the home challenge card reflects shelf changes made earlier in the same visit.
- `home/personal-blocks`: the personal blocks reflect shelf changes made earlier in the same visit.

## Impact

- backend: none.
- frontend: `apps/web` only — entities `shelf`, `home`, `reading-challenge` (key factories), feature `shelve-book`, feature `manage-reading-challenge`, widget `personal-home`, views `shelves` and `shelf-detail`, plus their tests.
- devops: none.
- docs: none beyond this change (specs updated on archive).
