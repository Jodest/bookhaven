# Tasks

## 1. Backend

- [x] 1.1 Refactor the curated paste core: move the `PreviewPaste` matching loop into `matchItems` with per-item retries (design Decision 6), and keep `PreviewPaste` behavior identical. Verify: the existing `curated` domain, service and handler paste tests pass unchanged (`go test ./internal/modules/curated/...`).
- [x] 1.2 Add `curatedservice.(*Service).MatchProposals` (manual-only check first; original-title retry that keeps the author; text «title — author»). Verify with service tests on a fake repository:
  - found;
  - not_found;
  - an original-title rescue;
  - an author mismatch → ambiguous;
  - in_collection;
  - a duplicate across ranks;
  - automatic → `ErrCollectionAutomatic`;
  - unknown → not found.
- [x] 1.3 Add a collection lookup for create (manual check plus size) on the curated service, and export the paste-preview response mapper from `curated/transport/http` for reuse. Verify: `go build ./...` and the existing handler tests.
- [x] 1.4 Add the forward-only migration `000028_ai_collection_drafts.{up,down}.sql` (design Database). Never run the down migration. Verify: apply it with the usual `migrate up` against the local DB only.
- [x] 1.5 Config: `AI_COLLECTIONS_ENABLED`, `AI_COLLECTIONS_MODEL` (default `claude-sonnet-5-5`) and `AICollectionsActive()`. Verify: `config_test.go` cases for the defaults, the flag without a key, and both set.
- [x] 1.6 Module `aicollections/domain`: the limits, statuses, error codes and `PromptVersion`. Add `repository` (GORM) with SQLite tests: create/get, list by collection with creator, active and stale predicate, count since, append proposals with the rank offset, token sums and heartbeat, mark running/done/failed, purge, and the cascade from the collection.
- [x] 1.7 `service`: create checks in the spec order, status, list, detail with the timeout projection, and a report only for `done` via `ProposalMatcher`. Unit tests cover every create error, the default count (size 100 → 120, size 1000 → 1000), and the report being null while running.
- [x] 1.8 `service/prompt.go` and `output.go`: system prompt, user message with the rank range and the exclusion list, schema, and validation order. Tests: refusal, truncation, unknown field, cleaning (quotes, caps, notes cut, in-chunk dedupe), and ranks by position.
- [x] 1.9 `service/tasks.go`: the `ai_collections:draft` handler with chunking, cross-chunk dedupe, early stop, resume after a transient error, the final-attempt rule (`done` with `error_code` when anything is stored, else `failed`), purge, a missing or deleted draft returning nil, and pacing with an injected clock. Tests use a fake `llm.Client`: 250 → three chunks; a transient error then resume with no repeated model call for stored chunks; a refusal on the first chunk → `failed`; a refusal on the second chunk → `done` with `model_refusal`; an empty chunk stops early.
- [x] 1.10 `transport/http` handler plus the `AICollectionsDeps` router group (`/admin/ai-collections`, `adminRole`, `requireBearer` on POST, `/status` before `/drafts/:id`), wired in `cmd/api/main.go`. Swagger annotations are in Russian. Tests:
  - handler status codes 503/400/404/422/409/429/502/202, and 400/404 on detail;
  - a router test with moderator → 403 and cookie-only POST → 401.
- [x] 1.11 Worker: the `ai_collections` queue with its own `asynq.Server` (`Concurrency: 1`), registered only when `AICollectionsActive()`, plus the disabled and missing-key warnings. Verify: a `main_test.go` server config test.
- [x] 1.12 Regenerate the swagger docs the usual way. Run `gofmt` only on changed files, never on the whole tree. Verify: `go build ./...`, `go vet ./...` and `go test ./...` from `backend/`.

## 2. Frontend (admin)

- [x] 2.1 `packages/api-client`: AI collection draft types and `adminAiCollectionsApi.{status, create, list, get}`. Verify: `tsc --noEmit` for the package and the admin app.
- [x] 2.2 Move the selection reducer, status labels, match-report table and bulk-add hook from `features/paste-collection-books` into `entities/curated-collection`, with no behavior change. The table gets an optional per-row extra slot and a hide-not-found option. Verify: the existing paste panel and selection tests pass without edits apart from import paths.
- [x] 2.3 `features/ai-collection-draft`:
  - hooks: status, create, list, and detail with polling only while queued or running;
  - libs: error texts, error_code labels, default count and not-found text;
  - `AiDraftPanel`: disabled state, form with counter and count, recent drafts, progress, report table, «Нет в каталоге (n)» with copy and «Добавить в каталог», «В конец»/«На позицию», «Добавить выбранные (N)» with the capacity rule, and the result report.
  - Verify with Vitest/RTL tests per the design Frontend list.
- [x] 2.4 `widgets/collection-books`: the «Черновик ИИ» toggle next to «Вставить список», open by default for an empty collection. Verify: the widget test for the toggle and the default open state; the automatic collection page (snapshot widget) has no control, covered by the existing `collection-edit-view` test plus one assertion.
- [x] 2.5 Verify: `tsc --noEmit` and `next lint` for `apps/admin` and `packages/api-client`, and the admin Vitest suite. No native `next build`: the CI frontend job is the typedRoutes gate.

## 3. DevOps and docs

- [x] 3.1 Add `AI_COLLECTIONS_ENABLED=false` and `AI_COLLECTIONS_MODEL=claude-sonnet-5-5` to `.env.example` and `.env.production.example`, and pass both to the `api` and `worker` services in `docker-compose.yml` and `docker-compose.prod.yml`. Verify: `docker compose config` renders both variables for both services.
- [x] 3.2 `docs/DEPLOYMENT.md`: the AI collection drafts section, covering the switch, the model choice, the shared key, that both services need the variables, the limits, the cost table and the token SQL. Update `docs/ROADMAP.md` (epic #413, feature 5).
- [x] 3.3 Verify the change folder: `npx -y @fission-ai/openspec@1.13.2 validate add-ai-collection-drafts --strict` (with `OPENSPEC_TELEMETRY=0`).
