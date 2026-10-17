# Proposal

## Why

The genre vocabulary grows with every harvest and import: spelling and language variants of one genre («Фентези», «Фэнтези», «Fantasy»), overly narrow genres with a handful of books, and "genres" that are not genres («Книги 2020», «Бестселлер»). Similar-name hints only catch spelling neighbours; an AI analysis can propose merges and rejections across the whole catalog, while every change stays an admin's decision.

## What Changes

- A background AI analysis of the genre catalog (manual trigger plus a weekly schedule) that stores **suggestions** of three kinds: `duplicate` (merge a group into a target), `narrow` (merge narrow genres into a general one) and `junk` (reject a genre that is not a genre). The AI never merges or rejects anything itself.
- Applying a suggestion goes only through the existing batch genre merge (one transaction per applied merge, collections follow the merge) or the existing genre bulk status (reject). The admin may change the target of a merge suggestion before applying.
- A suggestion whose genres changed since the analysis (merged, rejected, deleted) is stale: it is never applied and is marked stale. A dismissed suggestion is never proposed again for the same set of genres.
- New admin endpoints under `/api/v1/admin/genres/ai-suggestions` (list, status, analyze, apply, bulk apply, dismiss, bulk dismiss), admin role only.
- New admin page `/moderation/genres/ai-suggestions` («Предложения ИИ по жанрам») with per-item and bulk «Применить» / «Отклонить», reached from the genre moderation queue.
- New config switch `AI_GENRES_ENABLED` (+ model, cron, batch size); off without it or without an Anthropic API key, like AI moderation.

## Capabilities

### New Capabilities

- `catalog-moderation/ai-genre-suggestions`: AI genre analysis runs, suggestion storage and validation, staleness and dismissal memory, the admin endpoints, and the suggestions page.

### Modified Capabilities

None. Applying reuses `catalog-moderation/genre-merge` and `catalog-moderation/bulk-status` unchanged.

## Impact

- backend: new module (domain/repository/service/transport) with its own asynq queue; read helpers on the admin service for candidate genres; migration `000032`; config; router; worker wiring; swagger (additive).
- frontend: `apps/admin` — new route, entity, feature and view; the merge preview summary moves from the merge-genre feature to the genre entity so both features can use it; a link on the genre queue.
- devops: new env vars in `.env.example`, `.env.production.example`, both compose files.
- docs: `docs/admin-guide/duplicates.md` new section, `docs/DEPLOYMENT.md` env vars.
