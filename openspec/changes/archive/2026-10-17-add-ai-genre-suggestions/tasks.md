# Tasks

## 1. Backend: schema, config, admin read helpers

- [x] 1.1 Add forward-only migration `backend/migrations/000032_genre_ai_suggestions.{up,down}.sql` per design (three tables, partial unique index on `set_key`); never edit earlier migrations; do not run `migrate down`.
- [x] 1.2 Config: `AI_GENRES_ENABLED`, `AI_GENRES_MODEL` (default `claude-sonnet-5-5`), `AI_GENRES_CRON` (default `0 3 * * 1`, empty = no schedule), `AI_GENRES_BATCH` (default 300, clamp to 500), `AIGenresActive()`; extend `config_test.go` (defaults, active only with flag + key, clamping, other AI switches unaffected).
- [x] 1.3 Admin repository/service read methods: `ListGenreAnalysisCandidates` (non-alias, pending|published, `books_total`, QID, created_at), `GenresByIDs` (includes aliases and reports missing ids), service wrappers for `ListGenreSampleBooks` and `GenreHintPairs` (reusing the existing hint code, no behavior change to the queue). sqlite tests in `admin/repository` (aliases and rejected excluded from candidates; `GenresByIDs` sees aliases); `go test ./internal/modules/admin/...`.

## 2. Backend: aigenres module

- [x] 2.1 `aigenres/domain`: kinds, statuses, reason codes per kind, `PromptVersion = 1`, set-key helper; unit tests for the set key (order-independent, action-specific).
- [x] 2.2 `aigenres/repository` (GORM): runs (create, mark running/done/failed, interrupt >2h, latest, last done fingerprint, purge >180 days), suggestions (insert batch skipping same-set open/dismissed and open-genre overlap, list with filters/order/pagination, get, mark stale/applied/dismissed with `WHERE status='open'`, count open). sqlite tests incl. dismissal memory, overlap skip and the concurrent-mark guard.
- [x] 2.3 `aigenres/service/prompt.go` + `output.go`: system prompt (Russian rules, kinds, codes, untrusted-data rule), user message builder (refs, anchors, samples ≤2 × 80 chars, hints), JSON schema, validation per spec (every drop rule, ≥0.5, ≤50 items, reason truncation, refusal/truncation/invalid mapping). Table tests for each drop rule.
- [x] 2.4 `aigenres/service` analyzer: candidate loading, anchors, ordering, hint-cluster batching, ≤10 batches, pacing, staleness sweep at run start, per-batch insert, run bookkeeping, transient stop. Tests with a fake `llm.Client` and fake catalog: batching sizes and cluster cohesion, refs mapped back, transient stop keeps earlier batches, refusal continues, no genre/book write ever (fake applier never called).
- [x] 2.5 Scheduled vs manual tasks (`ai_genres:scheduled`, `ai_genres:analyze`, queue `ai_genres`, options per design): fingerprint skip for scheduled runs; tests.
- [x] 2.6 Service apply/dismiss: single and bulk per spec (source/target computation incl. target inside/outside the group, 400/404/409 mapping, stale marking, junk via `BulkSetGenreStatus`, `decided_by`). Tests with fake applier: merge called once per applied item with the expected source order, never for stale items; bulk overlap → `applied:1, stale:1`.
- [x] 2.7 `aigenres/transport/http` handler + swagger annotations (Russian): list, status, analyze (503/409/502), apply, bulk apply, dismiss, bulk dismiss. Handler tests incl. 400 validation cases, 404/409 bodies, and moderator 403 through the real admin-role middleware for every route.
- [x] 2.8 Wire routes in `backend/internal/transport/http/router.go` (admin group, bearer on POSTs), API composition in `cmd/api`, worker server (Concurrency 1, queue `ai_genres`) and cron only when `AIGenresActive()` in `cmd/worker/main.go`; extend `cmd/worker/main_test.go` (server config, cron spec/task type).
- [x] 2.9 Regenerate swagger (additive only; diff shows new paths/definitions, no removals). Verify: `go build ./... && go vet ./... && go test ./...` from `backend/`; gofmt only the changed files.

## 3. Frontend: apps/admin

- [x] 3.1 `packages/api-client` types for suggestions, runs, status and request/response bodies.
- [x] 3.2 Move `MergePreviewSummary` (and its preview hook, and the genre name search input if it lives in `features/merge-genre`) to `entities/genre`; update `features/merge-genre` imports; existing merge dialog tests pass unchanged.
- [x] 3.3 `entities/genre-ai-suggestion`: api functions + `lib/labels.ts` (kind, reason codes, run status) with vitest.
- [x] 3.4 `features/genre-ai-suggestions`: apply dialog (radio + «Другой жанр…», rejected-target warning, preview gating), junk confirm, bulk apply dialog (per-item previews, enabled when all settled), dismiss confirms, analyze button; vitest for preview gating, target change sending `target_id`, error texts (409 stale, 404, other), success messages incl. switched-collections suffix.
- [x] 3.5 `views/genre-ai-suggestions` + `app/moderation/genres/ai-suggestions/page.tsx`: tabs, kind filter, URL state, table, stale rows, selection, report messages, disabled-state text; admin-only routing and «Жанры» tab highlighting on the sub-route; vitest for the view (stale row disables «Применить», switched-off hides «Запустить анализ», report message).
- [x] 3.6 Genre queue link «Предложения ИИ (N)» in `views/genre-moderation`; extend its test.
- [x] 3.7 Verify: `tsc --noEmit` and `next lint` for `apps/admin` (and `packages/api-client` type check); vitest for touched slices. No native `next build` — CI is the typedRoutes gate.

## 4. DevOps and docs

- [x] 4.1 Add `AI_GENRES_*` to `.env.example`, `.env.production.example`, `docker-compose.yml`, `docker-compose.prod.yml` (api and worker), off by default.
- [x] 4.2 `docs/DEPLOYMENT.md`: env vars, cost note (≤10 batches per run, weekly default).
- [x] 4.3 `docs/admin-guide/duplicates.md`: new section «Предложения ИИ по жанрам» (Russian: kinds, how to run, apply with preview, change the target, dismiss is remembered, stale suggestions, nothing is applied automatically); link it from `docs/admin-guide/README.md`. Update `docs/ROADMAP.md` entry if one exists.
