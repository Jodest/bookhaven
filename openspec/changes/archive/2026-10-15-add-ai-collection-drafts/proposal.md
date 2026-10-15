# Proposal

## Why
Filling a manual curated collection («Топ-100 научной фантастики всех времён») by hand or by pasting a list found elsewhere is slow. Epic #413, feature 5: an admin describes the collection, the AI proposes a ranked list, and the system matches it against the catalog. Catalog books only: a book missing from the catalog is never added, only reported, and nothing is applied without the admin.

## What Changes
- New admin-only endpoints `/api/v1/admin/ai-collections` (`status`, create draft, list a collection's drafts, draft detail). They follow the other AI modules: off by default behind `AI_COLLECTIONS_ENABLED`, with the shared `ANTHROPIC_API_KEY`, model `AI_COLLECTIONS_MODEL` (default `claude-sonnet-5-5`), and 503 when disabled.
- A draft is generated asynchronously in the worker (own queue, one at a time). The model proposes up to `count` (1-1000) ranked books as title, author, original title and a short note. Large drafts are generated in chunks of 100. Limits: one active draft per collection, 20 drafts per admin per rolling 24 h, and drafts are kept for 30 days.
- The draft detail matches every proposal against the catalog at read time, with the bulk-paste matching rules and statuses (`found`, `ambiguous`, `not_found`, …), plus a retry by the original title. A book that is not in the catalog is listed as «Нет в каталоге» and never created or added.
- Applying a draft adds the chosen matched books through the existing bulk-add endpoint: appended at the end (default) or inserted at a position, never replacing the current books. The admin then reorders as usual.
- Admin UI: a «Черновик ИИ» panel on the edit page of a manual collection. It has a brief form, the collection's recent drafts, a match report with tick/pick (shared with «Вставить список»), apply, and a «Нет в каталоге» list with copy and «Добавить в каталог».
- Forward-only migration `000028`: tables `ai_collection_drafts` and `ai_collection_proposals`.

## Capabilities

### New Capabilities
- `collections/curated-collection-ai-drafts`: AI drafts of a manual curated collection: switch and limits, asynchronous generation, catalog matching of the proposals, applying them, the admin panel and retention.

### Modified Capabilities
(none: bulk paste preview, bulk add and the book panel keep their contract; the panel only gains a sibling control, specified in the new capability)

## Impact
- backend: new module `internal/modules/aicollections`; `curated` service gains a proposal-matching entry point that shares the paste matching core; config, router, `cmd/api` and `cmd/worker` wiring; migration 000028; swagger.
- frontend: `packages/api-client` types and API; admin `entities/curated-collection` takes over the shared match-report UI, selection and bulk-add hook from `features/paste-collection-books`; new `features/ai-collection-draft`; `widgets/collection-books` gets the toggle.
- devops: `AI_COLLECTIONS_ENABLED` and `AI_COLLECTIONS_MODEL` in `.env.example`, `.env.production.example` and both compose files (api and worker).
- docs: `docs/DEPLOYMENT.md` (switch, cost, both services), `docs/ROADMAP.md` (epic #413 feature 5).
