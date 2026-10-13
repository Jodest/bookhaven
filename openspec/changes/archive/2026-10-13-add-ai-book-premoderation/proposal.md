# Proposal

## Why

Epic «Автоматическое наполнение каталога» (#395), feature 2 of 4. Scheduled harvest makes `/moderation/books` fill up faster than one admin can read every row. An LLM (Claude via the Anthropic API) should pre-sort pending books into «опубликовать / отклонить / нужен человек», with reason codes and a short reason, so the admin can publish obvious books in bulk and spend time on the hard ones. A human still clicks every publish.

## What Changes

- New AI verdict per book: `publish` / `reject` / `needs_human`, machine reason codes, a short Russian reason, a confidence, the model id, a prompt version and a verdict time. Verdicts are stored separately from the book. The AI never writes title, authors, ISBN, year, description, genres or status.
- Verdicts are computed only in the background worker, never inside an HTTP request: a periodic sweep over pending books that have no verdict or an outdated one (new harvests, imports and edits are picked up within one sweep interval), plus an admin trigger «Проверить ИИ» for selected books or for all unreviewed pending books.
- Verdicts become stale when the book's data or the prompt version changes. Stale verdicts are flagged in the queue and recomputed.
- `GET /admin/books/moderation`: each item gains `ai_verdict` (or `null`), and a new filter `ai=publish|reject|needs_human|none`.
- New admin endpoints `POST /admin/books/ai-review` (queue a check) and `GET /admin/books/ai-review/status` (enabled flag, model, number of books waiting for a check). Both are admin only.
- `/moderation/books`: a column «ИИ», the «ИИ» filter, «Проверить ИИ (N)» for the selected rows, «Проверить ИИ все непроверенные», and «Выбрать уверенные «Опубликовать» (N)», which only selects rows. The admin still publishes through the existing bulk action and its confirmation. Nothing is auto-published.
- The feature is fully off unless `AI_MODERATION_ENABLED=true` and `ANTHROPIC_API_KEY` are set. The local stack and CI run without a key. The default model is `claude-haiku-5-5`, configurable.
- Follow-up from feature 1: a flagged author with a malformed QID no longer fails every scheduled run. That author is skipped, logged and stamped.
- Follow-up from feature 1: saving an author that is already flagged no longer returns 422 because the author lacks a QID. The flag is validated only when it is switched on, and the admin form sends it only when it changed. The checkbox stays clickable to clear the flag.

## Capabilities

### New Capabilities

- `catalog-moderation/ai-verdicts`: AI pre-moderation verdicts. Covers storage, what the model may see and do, the sweep and the manual trigger, staleness, failure handling, the trigger/status endpoints, and the queue UI («ИИ» column, the check buttons, confident-publish selection).

### Modified Capabilities

- `catalog-moderation/moderation-queues`: the book queue gains the `ai` filter (Queue filters) and the `ai_verdict` field (Book context).
- `catalog-import/wikidata-book-harvest`: a scheduled run skips and stamps flagged authors with a malformed QID instead of failing.
- `catalog-admin/author-editing`: the 422 for `auto_harvest: true` applies only when the flag is being switched on. The edit page lets the admin clear a flag on an author without a QID.

## Impact

- backend: migration `000022` (verdict table), new `internal/pkg/llm` (Anthropic client behind an interface), new `modules/aimoderation` (prompt, validation, worker handlers, HTTP trigger/status), admin service/repository (verdict storage, queue join and filter, review inputs), worker wiring (new `ai_moderation` server and schedule), config, harvest schedule tick fix, admin author update fix. New Go dependency `github.com/anthropics/anthropic-sdk-go`.
- frontend: apps/admin `widgets/moderation-queue` (toolbar slot and programmatic selection), `entities/book` (verdict cell), new `features/ai-review-books`, `views/book-moderation`, the `manage-author` form fix, packages/api-client and packages/types.
- devops: `AI_MODERATION_ENABLED`, `ANTHROPIC_API_KEY`, `AI_MODERATION_MODEL`, `AI_MODERATION_CRON`, `AI_MODERATION_BATCH` in the docker-compose files (default off).
- docs: docs/DEPLOYMENT.md (env, cost), docs/ROADMAP.md (epic feature 2), worker package doc comment, swagger.
