# Proposal

## Why

New admins and moderators have no written guide to the admin app: how to add books, authors and genres, what to do when an import job fails, how moderation works. One task-oriented guide, kept in the repository and shown inside the admin app, makes onboarding independent of the maintainer.

## What Changes

- A task-oriented user guide for the admin app in Russian, as markdown chapters in `docs/admin-guide/`: first steps and section map; adding a book/author/genre (Wikidata, AI, file, manual, and when to use which); import jobs and failures; collections and AI curators; catalog review queues; review moderation (short chapter for the `moderator` role); merging duplicates; screen adaptations; users and roles.
- Built-in help in the admin app: a «Справка» sidebar section, `/help` (chapter list) and `/help/<chapter>` rendering the same markdown files, built into the app at build time. A moderator sees only the «Модерация отзывов» chapter.
- A «?» link above every admin section page that opens the help chapter for that section.
- A rule in `docs/PIPELINE.md`: a feature that changes admin UX updates the matching guide chapter in the same PR.
- Out of scope (separate tasks): Playwright auto-screenshots, a driver.js onboarding tour for moderators.

## Capabilities

### New Capabilities

- `catalog-admin/admin-help`: built-in admin help (chapters, role scoping, per-section help link, safe markdown rendering).

### Modified Capabilities

- `auth/authorization`: «Admin app access» — a moderator also sees the help chapter for review moderation.

## Impact

- backend: none.
- frontend: `apps/admin` — new `/help` routes, navigation config (help section, chapter mapping per tab), shell help link, markdown renderer dependency.
- devops: the admin build reads `docs/admin-guide/` (already in the Docker build context); turbo cache input for the admin build.
- docs: new `docs/admin-guide/`, rule in `docs/PIPELINE.md`.
