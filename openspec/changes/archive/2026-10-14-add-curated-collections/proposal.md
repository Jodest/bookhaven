# Proposal

Epic #413, feature 1 of 5.

## Why

BookHaven wants its own ranked book collections («Топ-100 книг всех времён», «Топ-200 научной фантастики») that readers can browse. Readers' lists cannot carry this: they belong to a user, have no ranked places and no source attribution, so curated collections need their own entity, public pages and admin tooling.

## What Changes

- New entity «подборка» (curated collection), owned by BookHaven: title, slug, description, size N (ranked places), optional source attribution (text + URL), optional catalog genre, draft/published visibility, and a `kind` that is always `manual` for now (automatic collections arrive in feature 3 without a breaking change).
- A manual collection holds one admin-ordered list of catalog books that may be longer than N (the reserve). Readers see the first N **published** books in that order, numbered 1..N; places are computed at read time, so unpublishing a book shifts the rest up and republishing restores it, with nothing rewritten.
- Public API `GET /api/v1/collections` and `GET /api/v1/collections/{slug}` (no auth); web pages `/top` (all published collections) and `/top/{slug}` (ranked places), plus a «Подборки» header link for everyone.
- Admin API under `/api/v1/admin/collections` (admin role, bearer on writes): list, create, read, edit, delete a collection; add a catalog book (pending or published, never rejected), remove a book, move a book to a position.
- Admin app section «Подборки» (`/collections`): collection list, create/edit form, catalog search to add books, reordering by up/down and «move to position», hidden-book markers and the «пополните резерв» warning.
- Out of scope: bulk paste with matching (feature 2), automatic collections (feature 3), book page badge and home block (feature 4), AI drafting (feature 5).

## Capabilities

### New Capabilities

- `collections/curated-collections`: the collection model, the public ranking rule, the public API and the web pages `/top` and `/top/{slug}`.
- `collections/curated-collection-admin`: the admin API and the admin app section for managing manual collections.

### Modified Capabilities

None.

## Impact

- backend: new module `curated` (domain/repository/service/transport), migration `000025_curated_collections`, public and admin routes, shared slug helper extracted from the admin service.
- frontend: `packages/api-client` types and clients; `apps/web` pages `/top`, `/top/{slug}` and a header link; `apps/admin` new nav section, list and edit pages.
- devops: none (migration runs with the normal `migrate up`).
- docs: `docs/ROADMAP.md` epic #413 entry, API doc for the new routes.
