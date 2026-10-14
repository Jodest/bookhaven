# Proposal

Epic #395 «Автоматическое наполнение каталога», feature 3 of 4 (AI curator of import lists).

## Why

Admins know what the catalog lacks («русская классика XIX века», «лауреаты премии Хьюго») but have to find every author by hand on the Wikidata import page. An LLM can propose the author names. Each name is then checked against Wikidata, and the admin takes the ones they want through the existing import and harvest flows. The AI only proposes names; every fact still comes from Wikidata.

## What Changes

- New admin sub-page Импорт → Авторы → «Подбор ИИ» (`/import/authors/ai`). It is the third author sub-tab, after «Wikidata».
- New admin-only endpoints under `/api/v1/admin/ai-curator/`:
  - status;
  - create a curation request;
  - list recent requests;
  - get a request with its candidates.
- A curation request is processed asynchronously in the worker:
  - one structured-output model call returns at most N author names (with an optional short note);
  - each name is resolved through the rate-limited Wikidata client, keeping only people that have a Russian label;
  - the page polls the request until it finishes.
- Requests and candidates are stored, so admins can come back to them. They are kept for 30 days.
- Limits:
  - prompt of at most 500 characters;
  - 1–30 names (20 by default);
  - one active request per admin;
  - 20 requests per admin per rolling 24 h.
- Taking candidates reuses the existing flows only:
  - author import from Wikidata (lands `pending`);
  - book harvest (one job per bulk action);
  - the existing author `auto_harvest` flag.

  Nothing is auto-picked or auto-published.
- New config `AI_CURATOR_ENABLED` and `AI_CURATOR_MODEL` (default `claude-haiku-5-5`), sharing `ANTHROPIC_API_KEY`. The feature is off by default.
- New forward-only migration `000023_ai_curation`.

## Capabilities

### New Capabilities

- `catalog-import/ai-author-curation`: AI-proposed author lists for a free-text request, resolved against Wikidata, with review and taking through the existing import flows.

### Modified Capabilities

- `catalog-import/admin-import-section`: the Авторы tab gains the «Подбор ИИ» sub-page.

## Impact

- backend:
  - new module `aicurator` (prompt, validation, resolution, asynq task, HTTP handlers, repository);
  - config;
  - worker wiring (own `ai_curator` queue, Concurrency 1);
  - api routes;
  - migration 000023;
  - exported author-eligibility predicate in the admin service.
- frontend (apps/admin, packages/api-client):
  - new route and sub-tab;
  - curation widget and features;
  - `authorToInput` moved to `entities/author`.
- devops: compose passes `AI_CURATOR_ENABLED` / `AI_CURATOR_MODEL` to `api` and `worker`.
- docs: DEPLOYMENT (env, cost), ROADMAP, API docs (swagger).
