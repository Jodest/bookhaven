# Proposal

Epic #395 «Автоматическое наполнение каталога», feature 4 of 4 (last).

## Why

Many harvested books have no description: Wikidata has none, and Open Library has none or only an English one, which is dropped. A short AI-written annotation can fill the gap. But it is free text that can hallucinate, so it must never pass as a sourced description, and it never goes public without an admin's approval.

## What Changes

- New: an admin can queue AI generation of a short Russian annotation (2–4 sentences) for one book, for a selection, or for up to 200 published books with no description. Generation runs only in the worker. It is off unless `AI_DESCRIPTIONS_ENABLED` is set and `ANTHROPIC_API_KEY` is present.
- The model gets only catalog metadata (title, authors, year, ISBN, language, genres). If it does not know the work well enough, it answers "not enough knowledge", and nothing is invented.
- Generated text is stored apart from the book's own description, as a **draft**, with its model, prompt version and input hash. An admin approves it (as is or edited) or discards it: on the book edit page, or in the new tab «Описания ИИ» of the moderation section. There is no bulk approval, and no AI text is ever copied into the book's description.
- The public book page shows an approved AI description only when the book has no description of its own, under the label «Описание создано ИИ и может содержать неточности.». A sourced or admin-written description always takes over. AI text appears nowhere else: not in the home excerpt, search or lists.
- AI pre-moderation (feature 2) never receives AI-generated descriptions as input.
- New table `book_ai_descriptions` (forward-only migration 000024). New env vars `AI_DESCRIPTIONS_ENABLED` and `AI_DESCRIPTIONS_MODEL` (default `claude-sonnet-5-5`); the `api` and `worker` services both need them.

## Capabilities

### New Capabilities
- `catalog-admin/ai-descriptions`: AI description drafts: generation, the "not enough knowledge" outcome, failures, admin review (approve, edit, discard), the review tab, public display with the label, and the configuration switch.

### Modified Capabilities
- `catalog-moderation/ai-verdicts`: the model input explicitly excludes AI-generated descriptions.

## Impact

- backend: new module `aidescriptions` (domain, repository, service, transport); a migration; config; the worker's queue `ai_descriptions`; admin routes; the public book detail response gains `ai_description`.
- frontend: admin: the panel on the book edit page, the moderation tab `/moderation/descriptions` and api-client types. Web: the labelled fallback on the book page and the shared type.
- devops: the env vars in both compose files and `.env.example`, for `api` and `worker`.
- docs: `docs/DEPLOYMENT.md` (vars, cost), `docs/ROADMAP.md` (epic item done), `docs/API.md` if present.
