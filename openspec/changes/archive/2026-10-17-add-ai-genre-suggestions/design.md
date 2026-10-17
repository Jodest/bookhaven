# Design

## Context

AI pre-moderation of books (`backend/internal/modules/aimoderation`, spec `catalog-moderation/ai-verdicts`) is the template: own asynq queue with Concurrency 1, `PromptVersion`, reason codes, `llm.Client.Structured`, validated output, a switch that needs a flag and `ANTHROPIC_API_KEY`, and "the human decides". AI collection drafts (`modules/aicollections`) are the template for a module with its **own repository/tables**, a run/draft status lifecycle and an admin who applies results through an existing endpoint.

Applying must reuse, unchanged:
- `adminservice.Service.MergeGenresBatch(ctx, sourceIDs, targetID)` (`admin/service/merge_genre_batch.go`): one transaction, aliases, QID carry-over, collections switched and recomputed.
- `adminservice.Service.PreviewGenreMerge` (via the existing `POST /admin/genres/merge-preview` from the UI).
- `adminservice.Service.BulkSetGenreStatus(ctx, []uuid.UUID{id}, "rejected")` (`admin/service/genre_moderation.go`).

## Decisions

1. **New module `backend/internal/modules/aigenres`** (`domain`, `repository`, `service`, `transport/http`), like `aicollections`. It owns the tables `genre_ai_runs`, `genre_ai_suggestions` and `genre_ai_suggestion_genres`. It reaches genres only through small interfaces satisfied by `*adminservice.Service` (no direct writes to `genres`/`book_genres`):
   ```go
   type GenreCatalog interface {
       ListGenreAnalysisCandidates(ctx) ([]adminservice.GenreAnalysisCandidate, error) // non-alias, pending|published, books_total, qid
       ListGenreSampleBooks(ctx, ids []uuid.UUID) (map[uuid.UUID][]repository.SampleBook, error) // existing repo method, expose via service
       GenreHintPairs(ctx, genres []catalogDomain.Genre) []adminservice.GenreHintPair // wraps existing buildGenreHints/genreMatchKind
       GenresByIDs(ctx, ids []uuid.UUID) (map[uuid.UUID]adminservice.GenreState, error) // includes aliases: id, name, status, merged_into_id, qid, books_total
   }
   type GenreApplier interface {
       MergeGenresBatch(ctx, sourceIDs []uuid.UUID, targetID uuid.UUID) (*adminservice.GenreBatchMergeOutcome, error)
       BulkSetGenreStatus(ctx, ids []uuid.UUID, status string) (adminservice.GenreBulkStatusReport, error)
   }
   ```
   New admin repo/service read methods are the only admin-module changes; `ListGenreSampleBooks` already returns up to 3 per genre — the analyzer keeps 2.
2. **Batch-local refs.** The model sees `g1…gN`, never UUIDs (fewer tokens, no hallucinated ids); the analyzer maps refs back. Unknown refs drop the item.
3. **Batching** (bounded cost): anchors = top 100 published by `books_total` (ties by name), sent in every batch without samples. Non-anchors ordered pending first (oldest `created_at` first), then published by `books_total` asc; union-find over hint pairs (`exact_name`/`same_stem`/`shared_word`, computed by the existing hint code over the candidate set) keeps a hinted cluster together — a cluster is moved whole into the batch where its first member lands (a batch may exceed the size by one cluster, never by more than 50). `AI_GENRES_BATCH` (default 300, max 500) non-anchors per batch; `maxBatchesPerRun = 10`. Worst case ≈ 10 × (100×12 + 500×45) ≈ 240k input tokens/run; the system prompt is cached (as in aimoderation). Output `MaxTokens` 8192; ≤ 50 items stored per answer.
   User message per batch: `<genres>` JSON lines `{"ref","name","status","qid","books","samples":[...],"anchor":true}` and `<hints>` `[["g3","g17","same_stem"],…]`. Genre names and titles are untrusted data (same wording as aimoderation's `<book>` rule).
4. **Output schema** (`llm.StructuredRequest.Schema`): `{"suggestions":[{"kind","target":"gN"|null,"genres":["gN",…],"reason_codes":[…],"reason","confidence"}]}` — `genres` are the sources for merge kinds and the single genre for junk. Validation (`service/output.go`) per spec; item-level drops are logged as counts, never names.
5. **`PromptVersion = 1`** in `aigenres/domain`; bump on any change to the system prompt, schema or input shape.
6. **Set key & overlap.** `set_key = sha256(action + ":" + sorted genre UUIDs joined by ",")`, action `merge` | `reject`. Partial unique index on `set_key WHERE status IN ('open','dismissed')` backs the "never again" rule; the analyzer also checks it and the open-genre overlap in one query before insert (`INSERT … ON CONFLICT DO NOTHING` semantics via the index as a last guard). Insert happens per batch in one transaction.
7. **Staleness** is computed from `GenresByIDs`: any member (target included) missing, `merged_into_id` set, or `status = rejected`. Read endpoints compute `stale` without writing; the run start and the apply path write `status = stale`.
8. **Apply flow** (`service.Apply`): load suggestion (404) → status open? (409 not open) → stale check (mark stale, 409) → compute sources/target per spec → `MergeGenresBatch` (map `ErrGenreMergeNoSources`/`ErrGenreMergeDuplicateSource`/`ErrGenreMergeTooLarge`/`ErrCannotMergeGenreIntoSelf` → 400; `ErrGenreNotFound` → re-run the stale check: stale → mark stale + 409, else 404) or `BulkSetGenreStatus` (row error → mark stale + 409) → mark applied (`decided_by` from the JWT user, `decided_at`, `applied_target_id`). Marking applied is a separate write after the merge transaction; if it fails the merge stands and the suggestion is then shown stale (its sources are aliases) — log at error level. The update uses `WHERE status = 'open'` so two concurrent applies cannot both mark it; the second merge would fail with 404 → stale anyway.
9. **Bulk apply** loops `Apply` in order (≤ 50), collecting `{applied, stale, errors}`; each merge its own transaction (as the user required).
10. **Runs & concurrency.** `analyze` inserts a `genre_ai_runs` row (`queued`) and enqueues `ai_genres:analyze {run_id}` with `asynq.Queue("ai_genres")`, `MaxRetry(0)`, `Timeout(2h)`, `Unique(2h)`. 409 when a `queued`/`running` run younger than 2h exists (older ones are first updated to `failed/interrupted`). Scheduled: cron enqueues `ai_genres:scheduled` (Unique 1h); its handler computes the fingerprint `sha256` over sorted `(id,name,status,qid)` of the candidates + prompt version, compares with the last `done` run's `fingerprint`, and only then creates a run row (`trigger=scheduled`). Handlers: transient model error → run `failed/transient`, return nil (no asynq retry; next tick or a manual run continues). Pace `minCallInterval = 1s` between batch calls.
11. **Config**: `AI_GENRES_ENABLED` (bool), `AI_GENRES_MODEL` (default `claude-sonnet-5-5`), `AI_GENRES_CRON` (default `0 3 * * 1`, UTC Monday 03:00; empty → no schedule), `AI_GENRES_BATCH` (default 300, clamp 1..500). `cfg.AIGenresActive() = AIGenresEnabled && AnthropicAPIKey != ""`. API needs `active` + model for 503 and status; worker registers the server/handlers/cron only when active.
12. **Retention**: suggestions are kept (dismissed ones are the memory; volume is small). Runs older than 180 days are purged by the analyze handler at the start of each run (cheap `DELETE`); suggestions' `run_id` is `ON DELETE SET NULL`.

## API

All under the existing `admin.Group("/genres", adminRole)` in `backend/internal/transport/http/router.go`; register the static `ai-suggestions` routes **before** `/:id` routes for readability (Echo prefers static anyway). Moderator → 403 via `adminRole`.

| Method | Path | Auth | Success | Errors |
|---|---|---|---|---|
| GET | `/api/v1/admin/genres/ai-suggestions` | admin | 200 `{items,total}` | 400 bad status/kind |
| GET | `/api/v1/admin/genres/ai-suggestions/status` | admin | 200 `{enabled,model,prompt_version,open_count,last_run}` | 500 |
| POST | `/api/v1/admin/genres/ai-suggestions/analyze` | admin + bearer | 202 `{run_id}` | 409, 502, 503 |
| POST | `/api/v1/admin/genres/ai-suggestions/apply` | admin + bearer | 200 `{processed,applied,stale,errors:[{suggestion_id,message}]}` | 400 |
| POST | `/api/v1/admin/genres/ai-suggestions/dismiss` | admin + bearer | 200 `{processed,dismissed,errors}` | 400 |
| POST | `/api/v1/admin/genres/ai-suggestions/:id/apply` | admin + bearer | 200 `{suggestion,merge,status_report}` | 400, 404, 409 |
| POST | `/api/v1/admin/genres/ai-suggestions/:id/dismiss` | admin + bearer | 200 suggestion | 400, 404, 409 |

Error bodies: `{"error": "..."}` with the exact strings from the spec (`suggestion not found`, `suggestion is not open`, `suggestion is stale`, `AI genre analysis is disabled`, `AI genre analysis already running`). `merge` reuses the JSON shape of the existing merge-batch response (reuse its response struct/mapper from the admin handler, or a shared DTO). Swagger annotations in Russian like the other admin handlers; regenerate additively.

Item JSON:
```json
{"id":"…","kind":"duplicate","status":"open","reason_codes":["spelling","language"],"reason":"…","confidence":0.93,
 "model":"claude-sonnet-5-5","prompt_version":1,"created_at":"…","decided_at":null,"stale":false,
 "target":{"id":"…","name":"Фэнтези"},
 "genres":[{"id":"…","name":"Фэнтези","status":"published","merged":false,"exists":true,"wikidata_qid":"Q132311","books_total":120,"sample_books":[{"id":"…","title":"…","status":"published"}]}],
 "applied_target":null}
```

## Database

`backend/migrations/000032_genre_ai_suggestions.up.sql` / `.down.sql` (down drops only these three tables; never run it in tasks).

```sql
CREATE TABLE genre_ai_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  trigger VARCHAR(20) NOT NULL CHECK (trigger IN ('manual','scheduled')),
  status VARCHAR(20) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','done','failed')),
  error_code VARCHAR(40),
  model VARCHAR(100) NOT NULL DEFAULT '',
  prompt_version INTEGER NOT NULL,
  fingerprint CHAR(64) NOT NULL DEFAULT '',
  batches_total INTEGER NOT NULL DEFAULT 0,
  batches_done INTEGER NOT NULL DEFAULT 0,
  suggestions_created INTEGER NOT NULL DEFAULT 0,
  input_tokens INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ
);
CREATE INDEX idx_genre_ai_runs_created_at ON genre_ai_runs (created_at DESC);

CREATE TABLE genre_ai_suggestions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id UUID REFERENCES genre_ai_runs(id) ON DELETE SET NULL,
  kind VARCHAR(20) NOT NULL CHECK (kind IN ('duplicate','narrow','junk')),
  status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open','applied','dismissed','stale')),
  set_key CHAR(64) NOT NULL,
  reason_codes TEXT NOT NULL DEFAULT '[]',
  reason TEXT NOT NULL DEFAULT '',
  confidence NUMERIC(3,2) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model VARCHAR(100) NOT NULL,
  prompt_version INTEGER NOT NULL,
  applied_target_id UUID,           -- no FK: history survives genre changes
  decided_by UUID REFERENCES users(id) ON DELETE SET NULL,
  decided_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_genre_ai_suggestions_set_key_live ON genre_ai_suggestions (set_key) WHERE status IN ('open','dismissed');
CREATE INDEX idx_genre_ai_suggestions_status ON genre_ai_suggestions (status, kind, confidence DESC, created_at DESC);

CREATE TABLE genre_ai_suggestion_genres (
  suggestion_id UUID NOT NULL REFERENCES genre_ai_suggestions(id) ON DELETE CASCADE,
  genre_id UUID NOT NULL,           -- no FK: a deleted genre makes the suggestion stale, not vanish
  role VARCHAR(10) NOT NULL CHECK (role IN ('target','source')),
  position INTEGER NOT NULL,
  name_snapshot VARCHAR(255) NOT NULL,
  PRIMARY KEY (suggestion_id, genre_id)
);
CREATE INDEX idx_genre_ai_suggestion_genres_genre ON genre_ai_suggestion_genres (genre_id);
```
Junk members use `role = 'source'`. Check `genres.name` length in the existing schema and match `name_snapshot` to it. sqlite tests: the partial unique index must exist in the sqlite test schema too (sqlite supports partial indexes).

## Frontend (apps/admin)

- `packages/api-client`: types `AdminGenreAISuggestion`, `AdminGenreAISuggestionKind`, `…Status`, `AdminGenreAIRun`, `AdminGenreAIStatus`, request/response types (next to `AdminGenre*`).
- `entities/genre`: **move** `merge-preview-summary.tsx` and its preview query hook from `features/merge-genre` to `entities/genre/ui` (export from `entities/genre/index.ts`); `features/merge-genre` imports it from there. Needed because `features/genre-ai-suggestions` may not import another feature (FSD rule in `openspec/config.yaml`). Existing merge dialog tests must stay green unchanged in behavior.
- `entities/genre-ai-suggestion`: `api/` (list, status, analyze, apply, bulkApply, dismiss, bulkDismiss — follow `entities/ai-description/api`), `lib/labels.ts` (kind, reason-code, run-status labels) + `labels.test.ts`.
- `features/genre-ai-suggestions`: `ApplySuggestionDialog` (radio list of genres + «Другой жанр…» search reusing the genre search used by `BatchMergeGenresDialog` — move that search input to `entities/genre` too if it lives in the feature), `ApplyJunkConfirm`, `BulkApplyDialog`, `AnalyzeButton`, mutations with react-query invalidation of the suggestions list, status and genre moderation queue keys.
- `views/genre-ai-suggestions`: page composing status line, tabs, kind filter (URL state like `widgets/moderation-queue/model/use-queue-url-state.ts`), table, selection, messages.
- `app/moderation/genres/ai-suggestions/page.tsx`; admin-only route; make sure the section navigation config highlights «Модерация» → «Жанры» on this sub-route and that the role guard treats it as admin-only (check the navigation/locator config used by `widgets/section-tabs` and `widgets/admin-shell`).
- `views/genre-moderation`: header link «Предложения ИИ ({open_count})» from the status endpoint, shown when `enabled || open_count > 0`.

## Existing Patterns

- Worker/queue/cron wiring: `backend/cmd/worker/main.go` (aimoderation server config, scheduler registration) and `main_test.go` (Concurrency 1, cron spec assertions).
- Model call + pacing + failure mapping: `aimoderation/service/reviewer.go`, `tasks.go` (`ErrModelCall`, `llm.ErrTransient`, refusal/truncation via `resp.StopReason`).
- Own tables + run lifecycle + sqlite repo tests + fake LLM: `aicollections` (`repository_sqlite_test.go`, `service/fakes_test.go`).
- Handler with enqueuer interface and 503/409/502 mapping: `aimoderation/transport/http/handler.go`.
- UI: `features/ai-review-books` (queue message texts, enabled gating), `features/merge-genre/ui/batch-merge-genres-dialog.tsx` (radio target + other-genre search + preview gating).

## Risks

- **Hallucinated or harmful merges**: mitigated by refs, validation, confidence ≥ 0.5, narrow book limit, mandatory preview in every apply dialog, and no auto-apply. Merges are irreversible — the dialog says so.
- **Coverage on very large catalogs**: 10 batches × 500 cap ≈ 5000 non-anchor genres per run; beyond that the low-priority tail (published with many books) waits. Cross-batch synonyms between two non-anchor genres can be missed unless hinted — acceptable for v1; anchors cover the common targets.
- **Prompt injection via genre names / book titles**: untrusted-data rule; the output can only reference batch refs and fixed codes, and nothing applies without an admin.
- **Mark-applied after the merge is not in the merge transaction**: worst case the suggestion shows stale instead of applied; logged.
- **Bulk dialog with many previews**: up to 50 parallel `merge-preview` calls; acceptable for an admin tool; cap enforced client-side as well.
- **FSD move of `MergePreviewSummary`** touches the existing merge dialogs; keep their tests passing.
