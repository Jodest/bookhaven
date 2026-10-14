# Tasks

Ground rules for every group:
- **No destructive DB commands.** Never run `migrate down`, `DROP`, `TRUNCATE`, a DB reset or a volume wipe. Apply the new migration only with `migrate up`.
- **No whole-tree `gofmt -w`.** Format only the Go files you changed.
- **No native `next build`.** The CI frontend job is the typedRoutes gate.
- **No real API calls in tests.** Use no `ANTHROPIC_API_KEY` and make no Wikidata network calls: use the fake `llm.Client`, fake searchers and fetchers, and `httptest`.
- **Verification commands:**
  - backend, from `backend/`: `go build ./...`, `go vet ./...` and `go test ./...`;
  - frontend, per touched app or package: `pnpm --filter <pkg> exec tsc --noEmit`, `next lint` and the vitest tests.

## 1. Backend: config and migration

- [x] 1.1 Add `AICuratorEnabled` (`AI_CURATOR_ENABLED`, `parseBoolEnv`, default false), `AICuratorModel` (`AI_CURATOR_MODEL`, default `claude-haiku-5-5`, blank → default) and `cfg.AICuratorActive()`, which reuses `AnthropicAPIKey`. Add config tests for the defaults, flag without key, and flag with key.
- [x] 1.2 Add the forward-only migration `backend/migrations/000023_ai_curation.{up,down}.sql` per design (Database). Verify with `migrate up` on the local DB only. Never run the down migration.

## 2. Backend: admin service seam

- [x] 2.1 Extract the exported `adminservice.IsImportableAuthorRecord(*bookdata.ExternalAuthorRecord) bool` (person P31 allowlist and the existing Russian-label check) and use it inside `ImportWikidataAuthor` with no behavior change. Existing author-import tests stay green, and new table tests cover Q5, Q16017119, a novel, an empty ru label and a Latin-only label.

## 3. Backend: aicurator module

- [x] 3.1 `domain`: statuses, match statuses, error codes, limit constants and `PromptVersion = 1`.
- [x] 3.2 `repository`: the GORM implementation of the methods listed in design (Database). Repository tests (SQLite, as in other modules) cover:
  - the active predicate, including staleness;
  - `CountRequestsSince`;
  - `SaveModelResult` atomicity;
  - `SaveCandidateResolution`, which bumps `updated_at`;
  - the purge cascade;
  - list counts.
- [x] 3.3 `service/prompt.go` + `output.go`: the system prompt, schema, user message and validation (design Decision 6). Unit tests cover refusal, max_tokens, an unknown field, a missing field, trimming and whitespace collapse, a name over 150 runes, case-insensitive dedupe, truncation to `max_names`, note cut to 200 runes, an empty list, and markup kept as literal text.
- [x] 3.4 `service/resolve.go` (Decision 7), with fakes for `SearchAuthors` and `FetchAuthors`. Tests cover:
  - matched;
  - ambiguous;
  - not_found (no hits, or only non-persons);
  - a redirect deduped to the canonical QID;
  - no `FetchAuthors` call when there are no hits;
  - `ErrInvalidQuery` → not_found;
  - an unavailable source propagated.
- [x] 3.5 `service` create/read: `CreateRequest` in the order 503 → 400 → 409 → 429 (Decision 9), `GetRequest` with live `existing` (one finder call; a finder error gives all null), `ListRecent`, and the timeout projection. Tests use a fake clock.
- [x] 3.6 `service/tasks.go`: the `ai_curator:curate` task, its options (`Queue("ai_curator")`, `MaxRetry(2)`, `Timeout(15m)`) and the handler (Decision 5). Tests with a fake `llm.Client`, which counts calls, and fake Wikidata cover:
  - the happy path;
  - a retry after stored candidates making no second model call;
  - a transient model error on a non-final attempt → error returned, and on the final attempt → `failed`/`model_error`;
  - a non-transient error → SkipRetry and `failed`;
  - Wikidata unavailable mid-run → resumes at that candidate, and on the final attempt the remaining candidates become `error` and the request is `done`;
  - a terminal request → no-op;
  - the purge called.
- [x] 3.7 `transport/http`: the status, create, list and get handlers, with Russian swagger annotations. Handler tests cover every status code in design (API), including 400 on a bad UUID, 502 marking `enqueue_failed`, and the user id taken from the context.

## 4. Backend: wiring

- [x] 4.1 `RegisterAdminRoutes`: an `AICuratorDeps` struct and the group `/admin/ai-curator` with `adminRole`, plus `requireBearer` on POST. Wire it in `backend/cmd/api/main.go` with the existing asynq client and admin service. Router tests check that a moderator gets 403 and that POST without a bearer is rejected.
- [x] 4.2 Worker: add `aiCuratorServerConfig` (`Queues {"ai_curator":1}`, `Concurrency 1`). Start the server and register the handler only when `cfg.AICuratorActive()`. The handler gets the **same** `wikidataClient` instance as harvest and `llm.NewAnthropic(key, cfg.AICuratorModel)`. Log the disabled state and the warning for a flag without a key. Extend `main_test.go` for the server config.
- [x] 4.3 `docker-compose*.yml` / `.env.example`: pass `AI_CURATOR_ENABLED` and `AI_CURATOR_MODEL` to both `api` and `worker`, with the default off. Then run `go build ./... && go vet ./... && go test ./...` from `backend/`, and run gofmt only on the changed files.

## 5. Frontend

- [x] 5.1 `packages/api-client`: the curation types and `adminAiCuratorApi` (design, Frontend). Verify with `tsc --noEmit` for the package.
- [x] 5.2 Move `authorToInput` to `entities/author/lib` and update `features/manage-author` to import it. The existing `author-form` tests stay green.
- [x] 5.3 `entities/ai-curation`: query keys plus the `MATCH_STATUS_LABELS` and `CURATION_ERROR_MESSAGES` labels, with a unit test for the label maps, including an unknown code shown raw.
- [x] 5.4 `features/request-ai-curation`: the status, create (error mapping) and request (3 s polling only while active) hooks and `CurationPromptForm`. Tests cover the counter and the 500 limit, submit disabled while a request is active, the off state, and the error messages for 409, 429, 503 and 502.
- [x] 5.5 `features/take-curated-authors`: `useTakeCuratedAuthors` (sequential imports, one harvest with `limit = min(5000, 300·n)` for created/existing only, then GET+PATCH `auto_harvest: true` for authors not yet flagged; per-author results; invalidations) and `selectAllMatched`. Tests mock the API and cover a rejected outcome (no harvest, no flag), an import error continuing the batch, and the harvest skipped when no QIDs are left.
- [x] 5.6 `widgets/ai-author-curation` + `views/import-authors` method `'ai'` + `app/import/authors/ai/page.tsx` (Suspense for `useSearchParams`). Widget tests cover:
  - the off state;
  - progress text;
  - match badges;
  - plain-text notes (`<b>` rendered literally);
  - nothing preselected;
  - «Выбрать все найденные»;
  - rejected options not selectable;
  - the per-option actions reusing the import hook.
- [x] 5.7 `shared/config/navigation.ts`: the `import.authors.ai` subtab after Wikidata. Update `navigation.test.ts` and `section-tabs.test.tsx`, where the Авторы tab shows three sub-tabs in order. Run `tsc --noEmit`, `next lint` and vitest for `apps/admin`, with no native `next build`.

## 6. Docs

- [x] 6.1 `docs/DEPLOYMENT.md`: `AI_CURATOR_ENABLED` and `AI_CURATOR_MODEL` (both services), the shared `ANTHROPIC_API_KEY`, the cost table and the token-sum SQL, and the 30-day retention.
- [x] 6.2 `docs/ROADMAP.md`: update the epic #395 line (features 1–3 done, OpenSpec change `add-ai-author-curation`). Regenerate swagger the way the repo already does, if it is generated. Run `npx -y @fission-ai/openspec@1.13.2 validate add-ai-author-curation --strict` with `OPENSPEC_TELEMETRY=0`.
