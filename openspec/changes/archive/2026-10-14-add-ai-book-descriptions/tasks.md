# Tasks

Ground rules for every group:
- Never run destructive DB commands: no `migrate down`, no `DROP`, no DB reset or volume wipe. `migrate up` on the running local stack is allowed.
- Never run `gofmt -w` on the whole tree: format only the files you changed.
- Never run a native `next build`; CI is the typedRoutes gate.
- Tests never call the real Anthropic API: use a fake `llm.Client`, and `httptest` only for adapter-level tests.
- Verify with `go build ./... && go vet ./... && go test ./...` from `backend/`, and with `tsc --noEmit` and `next lint` in each touched app.

## 1. Backend: storage and config

- [x] 1.1 Add the forward-only migration `backend/migrations/000024_book_ai_descriptions.{up,down}.sql`, as in design.md → Database. Never edit an earlier migration. Verify with `migrate up` on the local stack.
- [x] 1.2 Add `AIDescriptionsEnabled`, `AIDescriptionsModel` (default `claude-sonnet-5-5`) and `AIDescriptionsActive()` to `internal/config`. Config tests cover the default, the flag without a key, and the flag with a key.
- [x] 1.3 Create `modules/aidescriptions/domain` (statuses, error codes, `PromptVersion = 1`, `MaxBulk = 200`, `MaxIDs = 100`, text limits) and its `repository`, with every method in design.md. Add SQLite repository tests: upsert resets the review columns; missing candidates exclude books with any record, with an own description or not published; conditional approve and discard; `superseded`; the approved reader.

## 2. Backend: generation

- [x] 2.1 Add the thin `adminservice.LoadBooksWithRelations`, which wraps `LoadBooksForReview`. Export the Cyrillic-share helper if needed. No change to `admin/repository.Repository`.
- [x] 2.2 Add `service/prompt.go` (system prompt, schema, input builder plus `InputHash`) and `service/output.go` (validation, Decision 8). Unit tests: refusal, truncated, invalid JSON, an unknown field, `known=false` with and without text, too short, too long, English, a valid draft, and a stable hash under author or genre reordering.
- [x] 2.3 Add `service/tasks.go` with the `generate` and `sweep` handlers (Decision 10) and pacing through an injected clock. Fake-client tests:
  - ineligible books are skipped (own description, rejected, deleted);
  - the `queued_at` skip on retry;
  - a transient error retries `generate` and stops `sweep`;
  - `SkipRetry` on the final attempt;
  - the pause between calls.
- [x] 2.4 Worker: an `ai_descriptions` server (`Concurrency: 1`) and handler registration only when `AIDescriptionsActive()`, with the disabled log and the warning log. Add a server-config helper test in `cmd/worker/main_test.go`.

## 3. Backend: API

- [x] 3.1 Add `aidescriptions/transport/http` with the status, generate, list, get, approve and discard handlers (design.md → API), plus Russian Swagger annotations. Handler tests cover:
  - 503 first;
  - every 400 case, 409 on a duplicate, 502;
  - approve: 404, 409 for the wrong status, 409 when the book has its own description, 422 for bad text, `edited`;
  - the list's `stale` and `superseded`.
- [x] 3.2 Add the `AIDescriptionsDeps` route group (`adminRole`, `requireBearer` on POSTs, static routes before `/:book_id`) and wire it in `cmd/api/main.go`. Router tests: a moderator gets 403 on generate, approve and discard; a missing bearer is rejected.
- [x] 3.3 Public: add the catalog `AIDescriptionReader` and `ai_description` to the book detail response. Handler tests: approved with a blank own description → text; an own description → null; a draft → null; a reader error → null, and the page still returns 200.
- [x] 3.4 Add a feature-2 regression test: the AI verdict input and its hash for a book with an approved AI description equal the input with an empty description.

## 4. Frontend: admin

- [x] 4.1 Add the api-client types and `adminAiDescriptionsApi`, and `entities/ai-description` (keys, labels, the text component). Unit test the labels.
- [x] 4.2 Add `features/generate-ai-description` and `features/review-ai-description`, with hook tests for the 409, 503 and 422 mappings. FSD: no cross-feature imports.
- [x] 4.3 Add `widgets/ai-description-panel` and render it in `views/book-edit`. Tests:
  - shown or hidden by the own-description and record rules;
  - the regenerate confirmation;
  - the own-description notice;
  - the polling stop.
- [x] 4.4 Add `views/ai-description-moderation`, `app/moderation/descriptions/page.tsx` and the `moderation.descriptions` subtab. Update `navigation.test.ts` and `section-tabs.test.tsx`. View tests: the URL filter, row approve and discard, and the toolbar hidden when `enabled: false`.

## 5. Frontend: web

- [x] 5.1 Map `ai_description` → `aiDescription` in the public book type. In `views/book-detail`, render the fallback with «Описание создано ИИ и может содержать неточности.». Tests: the label shown, the own description wins with no label, and null shows no block.

## 6. DevOps and docs

- [x] 6.1 Add `AI_DESCRIPTIONS_ENABLED` (default false) and `AI_DESCRIPTIONS_MODEL` (default `claude-sonnet-5-5`) to both `api` and `worker` in `docker-compose.yml` and `docker-compose.prod.yml`, and to `.env.example` with a comment. Quote values that contain spaces.
- [x] 6.2 Update `docs/DEPLOYMENT.md`: the vars, that both services need them, the cost table, the token SQL, and the copyright and review notes. Mark the epic item in `docs/ROADMAP.md` line 48 done. Update `docs/API.md` (or the existing API doc) for the new admin routes and the public `ai_description` field.
- [x] 6.3 Run `npx -y @fission-ai/openspec@1.13.2 validate add-ai-book-descriptions --strict` (`OPENSPEC_TELEMETRY=0`) and fix any errors.
