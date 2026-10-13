# Tasks

Rules for every group:
- Never run destructive DB commands (`migrate down`, `DROP`, resetting or wiping the DB). Apply the new migration only with `migrate up`.
- Format only the Go files you changed. Never `gofmt -w` the whole tree.
- Never run a native `next build`; CI is the typedRoutes gate.
- Tests must never call the real Anthropic API. Use fakes or `httptest` only.
- Never commit an API key.

## 1. Backend: follow-ups from feature 1

- [x] 1.1 `harvest/service/schedule.go`: split candidates into valid and malformed QIDs (export `bookdata.IsWikidataQID` if needed). Log the malformed ones, enqueue only the valid ones, and stamp all picked ids. With no valid QIDs, stamp and return nil without enqueueing (design.md Decision 12). Verify with fake-based tests: mixed batch (job has 2 QIDs, 3 ids stamped), all malformed (no enqueue, stamped, nil error), enqueue failure still stamps nothing.
- [x] 1.2 `adminservice.UpdateAuthor`: return the QID 422 only when switching the flag on (Decision 13). Verify with new cases in `auto_harvest_test.go`: already-flagged author without a QID sends `true` → saved and flag kept; unflagged author without a QID sends `true` → `ErrAutoHarvestRequiresQID`. Existing cases stay green.

## 2. Backend: LLM client package

- [x] 2.1 `go get github.com/anthropics/anthropic-sdk-go`, then `go mod tidy` in `backend/`. Add `internal/pkg/llm` with the `Client` interface, the request/response types, `ErrTransient` and the Anthropic implementation (design.md Decision 3: exact model id, `MaxTokens` 1024, system block with cache_control, a single user message, no prefill, `output_config` effort `low` + `json_schema` format, max retries 2).
- [x] 2.2 Adapter tests using `httptest.Server` + `option.WithBaseURL`:
  - the request JSON carries `model`, `output_config.effort == "low"`, `output_config.format.type == "json_schema"`, system `cache_control`, and no assistant message;
  - `end_turn`, `max_tokens` and `refusal` (with stop_details) responses map correctly;
  - 429/529/503 map to `ErrTransient`.

## 3. Backend: verdict storage and queue

- [x] 3.1 Migration `000022_book_ai_verdicts` (up/down) as in design.md → Database. Apply with `migrate up` only. (Applied to a running stack with `make migrate` — up only; not applied during implementation because the local stack was down.)
- [x] 3.2 Admin repository: the `BookAIVerdict` row, `UpsertBookAIVerdict`, `GetBookAIVerdicts`, `ListAIReviewCandidateIDs`, `CountAIReviewCandidates`, `LoadBooksForReview`, and the `AI` filter on `ListBooksForModeration`. Update every `repository.Repository` fake. Verify with SQLite repository tests for the upsert (replace + created_at refresh), the candidate order and prefilter (no verdict first, old prompt version, updated after verdict, non-pending excluded), and the `ai` filter values including `none`.
- [x] 3.3 Admin service:
  - `BookReviewInput` + `InputHash` (Decision 5), with deterministic order and the description cut to 1500 runes;
  - extract the hint computation into a shared helper;
  - `BookReviewInputs`, `SaveBookAIVerdict`, `ListAIReviewCandidates`, `CountAIReviewCandidates`;
  - `ListBookModerationQueue` validates `ai` and attaches `AIVerdict` with `Stale`.

  Verify with service tests: the hash is stable under author/genre order and changes on a title or author-name edit; stale on a prompt version or hash mismatch; `ai=bogus` → `ErrInvalidModerationFilter`; the existing queue tests stay green.
- [x] 3.4 Admin transport: `ai` query param, the `ai_verdict` response object (null when absent, `reason_codes` never null), swagger annotations. Verify by extending `handler_book_moderation_test.go` (shape with and without a verdict, 400 on a bad `ai`).

## 4. Backend: aimoderation module and worker

- [x] 4.1 Config:
  - `AI_MODERATION_ENABLED` (default false), `ANTHROPIC_API_KEY`, `AI_MODERATION_MODEL` (default `claude-haiku-5-5`), `AI_MODERATION_CRON` (default `*/30 * * * *`), `AI_MODERATION_BATCH` (default 50, clamped 1..200);
  - `AIModerationActive()`.

  Verify with config tests for the defaults, clamping, and active only when the flag and key are both set.
- [x] 4.2 `modules/aimoderation/service`:
  - the prompt (`PromptVersion = 1`, untrusted-data rule, the code definitions) and the schema;
  - validation (Decision 6);
  - the `Reviewer`, which builds the input, calls `llm.Client`, validates and saves; it skips non-pending books and paces calls through an injected clock;
  - task constructors and options for `ai_moderation:sweep` and `ai_moderation:review_books` (Decision 7).

  Verify with fake-client tests:
  - valid publish / reject / needs_human;
  - invalid JSON, unknown code, publish with codes, needs_human without codes and confidence 1.5 → `invalid_output`;
  - refusal → `model_refusal`; max_tokens → `model_truncated`;
  - a transient error stores nothing and stops the sweep;
  - a book no longer pending is skipped;
  - `review_books` forces a recompute of a fresh verdict;
  - the reason is truncated to 300 runes;
  - the stored model, prompt version and hash are set;
  - the system prompt contains the untrusted-data rule and the book JSON sits only in the user message.
- [x] 4.3 `modules/aimoderation/transport/http`: `POST /admin/books/ai-review` and `GET /admin/books/ai-review/status` (design.md → API). Wire them in `router.go` (admin role, bearer on POST, static paths before `/:id`) and in `cmd/api/main.go`. Verify with handler tests:
  - 503 when inactive;
  - 400 for `{}`, both forms, a bad UUID and >100 ids;
  - 202 with dedup;
  - 409 on `ErrDuplicateTask`;
  - 502 on another enqueue error;
  - status shape;
  - a route auth test where a moderator gets 403.
- [x] 4.4 Worker wiring:
  - when `AIModerationActive()`, build the Anthropic client, the `ai_moderation` server (Concurrency 1, own config helper) and the handlers, and register the sweep schedule in `registerSchedules`;
  - otherwise log `AI moderation disabled` (and the flag-without-key warning);
  - shut down in reverse order;
  - update the package doc comment.

  Verify by extending `main_test.go` (registered only when active, the server config, an invalid cron fails), then run `go build ./... && go vet ./... && go test ./...` from `backend/`.

## 5. Frontend: admin

- [x] 5.1 `packages/types` / `packages/api-client`: `AdminBookAIVerdict`, `ai_verdict` on the queue item, the `ai` param, `aiReview` and `aiReviewStatus`. Verify with `tsc --noEmit` on the touched packages.
- [x] 5.2 `widgets/moderation-queue`: `controls.select(ids)` and the `renderToolbar` slot. Verify with new cases in `moderation-queue.test.tsx` (the toolbar renders with page items; `select` checks exactly those rows).
- [x] 5.3 `entities/book`: `AiVerdictCell` + `AI_REASON_LABELS` (spec labels, «устарел», «—», plain-text reason). Verify with a component test.
- [x] 5.4 `features/ai-review-books`:
  - the status and mutation hooks (409 and 503 messages);
  - `confidentPublishIds` (threshold 0.9, not stale, no hints, publish only) with a unit test;
  - the toolbar and the selected-rows button with confirm texts from the spec.
- [x] 5.5 `views/book-moderation`:
  - the «ИИ» column, the `ai` filter and its param;
  - the toolbar only on «На модерации»;
  - the check buttons only when enabled;
  - the confident selection, which only selects.

  Extend `book-moderation-view.test.tsx`:
  - the filter goes to the URL and params;
  - the selection picks the right rows and calls no mutation;
  - clicking «Проверить ИИ (N)» posts the ids;
  - with enabled false the buttons are hidden.
- [x] 5.6 `features/manage-author` (follow-up 2): `authorToInput` without `auto_harvest`; the form sends it only when changed; the checkbox is disabled only for unflagged authors without a QID. Update `author-form.test.tsx`. Verify with `tsc --noEmit`, `next lint` and vitest in `apps/admin`.

## 6. DevOps and docs

- [x] 6.1 `docker-compose.yml` and `docker-compose.prod.yml`: add `AI_MODERATION_ENABLED` (default false), `ANTHROPIC_API_KEY` (default empty), `AI_MODERATION_MODEL`, `AI_MODERATION_CRON` and `AI_MODERATION_BATCH` to both `api` and `worker`. Add them to `.env.example` if present.
- [x] 6.2 `docs/DEPLOYMENT.md`: the env vars, the note that api and worker both need the flag and key, the single-replica note, and the cost table from design.md → Cost. `docs/ROADMAP.md`: the epic #395 entry gets feature 2 done (link this change). Run the swagger regeneration used by the repo for the touched handlers.
- [x] 6.3 Run `npx -y @fission-ai/openspec@1.13.2 validate add-ai-book-premoderation --strict` (with `OPENSPEC_TELEMETRY=0`) and confirm it passes.
