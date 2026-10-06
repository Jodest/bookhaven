# Harvest Job Report

Status: implemented

This is the next increment on top of `docs/features/admin-harvest-trigger.md` (`POST /api/v1/admin/harvest`, `GET /api/v1/admin/harvest/jobs`/`/jobs/:id`, the admin `/harvest` page and its jobs table) and `docs/features/harvest-progress.md` (live progress bar for `active` jobs, `HarvestResult.Total`). Today the jobs table only shows an aggregate one-line summary per job ("создано X, пропущено Y, ошибок Z") — the per-record detail that already exists in `HarvestResult.Errors` (title + message per failed record) is fetched by the frontend but never rendered, and there is no per-record detail at all for skips (only a count). This contract makes the full report — every error with its reason, every skip with its title — viewable from the admin jobs table, without changing any harvest business logic.

## Scope boundaries (read first)

**Everything from the two prior contracts stays exactly as decided — not reopened here:**
- `asynq`'s Redis-backed task/queue state remains the sole source of truth; no new Postgres table.
- 5-second fixed-interval polling of `GET /jobs` stays; no WebSockets/SSE.
- The progress bar for `active` jobs (`harvest-progress.md`) is unchanged — this contract only extends the "Результат" cell with an additional expand/collapse affordance, and only for states where there is per-record detail to show.
- No change to *why* records get skipped or error out. Today a skip has exactly one possible cause: the record's Wikidata QID already exists in the catalog (`FindBookByExternalID` hit in `processRecord`, `backend/internal/modules/harvest/service/handler.go`). This contract surfaces *which* records that happened to, not a new reason.
- No CSV/export, no pagination/virtualization of the error/skip list (bounded by the existing `-limit` clamp of 5000 records per job — see Risks), no change to polling cadence.

**Explicitly out of scope (do not build):**
- Any change to `processRecord`, `runHarvest`'s control flow, or what counts as a skip/error.
- A dedicated job-detail page/route. This stays inline in the existing jobs table (expand/collapse per row), not a new `/harvest/jobs/:id` page.
- Wiring `GET /admin/harvest/jobs/:id` into the frontend for this feature — see the Frontend decision below for why, and Risks for when to revisit.

## Research notes (verified against current code, not assumed)

- `harvest/transport/http/handler.go`'s `toHarvestJobResponse` surfaces `harvestJobResponse.Result` unconditionally whenever `len(info.Result) > 0` and it decodes cleanly into `harvestservice.HarvestResult` — **no gate on `State`**, confirmed unchanged since `harvest-progress.md` (the `russian-only-content.md` increment touched SPARQL query builders and `validateBookInput`, not this handler or this mapping). This means a new field added to `HarvestResult` needs **no transport-layer change** to flow through both `GET /jobs` and `GET /jobs/:id`.
- `HarvestResult` today: `Processed`, `Total`, `Created`, `Skipped int` (`json:"skipped_existing"`, a count only — no per-skip detail), `Errors []HarvestRecordError` (`Index`, `Title`, `Message` — full per-record detail, populated in `runHarvest`'s existing loop). Confirmed there is no `HarvestSkippedRecord`-shaped type anywhere in the codebase today.
- `apps/admin/src/views/harvest/ui/harvest-view.tsx`'s current "Результат" cell: `JobResultCell` renders a progress bar for `active` jobs, and `jobResultSummary(job)` (a one-line string, "создано X, пропущено Y, ошибок Z") for everything else. **`job.result.errors` (already present in every `GET /jobs` poll response today) is never rendered per-record** — only its `.length` is used for the count. There is no expand/collapse, accordion, or "show more" pattern anywhere in `apps/admin/src` or `packages/ui/src/components/ui/*` today (`button`/`input`/`textarea`/`label`/`select`/`card`/`table`/`badge`/`progress` only — no Radix `Collapsible`/`Accordion` dependency). This contract introduces the pattern with plain React state + existing components, not a new package dependency.
- `apps/admin/src/entities/harvest/api/index.ts` already re-exports `getJob: api.admin.harvest.jobs.get` (calling `GET /jobs/:id`), and `harvest-view.test.tsx` already mocks it — but `harvest-view.tsx` itself never calls it. **This is a pre-existing, not new, condition.** Decision below keeps it that way for this contract (see Frontend).

## User Stories

- As a content admin reviewing a completed (or still-running) harvest job, I can expand its row in the jobs table and see every error with its record title and the exact reason, and every skipped record's title, without leaving the admin panel or inspecting raw JSON.
- As a content admin, if a job has no errors and no skips, I don't see a distracting "show details" control for it — the report affordance only appears when there's something to show.

## Acceptance Criteria

**Backend**
- [x] `backend/internal/modules/harvest/service/handler.go`: new exported type
  ```go
  // HarvestSkippedRecord describes a single record the harvest run skipped
  // because a book with the same (external_source, external_id) already
  // exists in the catalog - today the only possible skip cause (see
  // processRecord's idempotency check). No Message field: unlike
  // HarvestRecordError, there is exactly one reason, so the frontend renders
  // a fixed label rather than this type carrying a per-record message.
  type HarvestSkippedRecord struct {
      Index int    `json:"index"`
      Title string `json:"title"`
  }
  ```
- [x] `HarvestResult` gains `SkippedRecords []HarvestSkippedRecord \`json:"skipped_records"\`` alongside the existing `Skipped int \`json:"skipped_existing"\`` (the count field is **unchanged**, stays authoritative for the existing summary text — `len(SkippedRecords) == Skipped` must hold at all times, enforce via test).
- [x] `runHarvest`: initialize `result.SkippedRecords = []HarvestSkippedRecord{}` alongside the existing `Errors: []HarvestRecordError{}` initialization (empty-slice, not nil, for consistent `[]` JSON when empty — matches existing `Errors` convention). In the existing per-record loop, when `outcome == outcomeSkipped`, append `HarvestSkippedRecord{Index: i, Title: rec.Title}` in the same branch that currently does `result.Skipped++` — no new loop, no new mechanism, mirrors exactly how `Errors` is populated in the adjacent branch.
- [x] `reportProgress`'s existing per-record calls (from `harvest-progress.md`) automatically carry the growing `SkippedRecords` list mid-job, same as `Errors` already does today — no code change needed beyond the struct/loop change above (confirm with a test, don't just assume).
- [x] No changes to `harvest/transport/http/handler.go` — `toHarvestJobResponse`'s existing unconditional `Result` mapping already carries the new field through both `GET /jobs` and `GET /jobs/:id`, for any state where a `Result` is present (including `active`, per `harvest-progress.md`). Confirm with a test rather than assuming (see Tests).
- [x] No route, payload, or authorization changes anywhere in this contract — same `RoleAdmin` + existing read-auth on `GET /jobs`/`GET /jobs/:id`, unchanged.

**Frontend (`apps/admin`)**
- [x] `packages/api-client/src/index.ts`: `AdminHarvestJob.result` gains `skipped_records: { index: number; title: string }[]` alongside the existing `errors` field.
- [x] `apps/admin/src/views/harvest/ui/harvest-view.tsx`: add local expand/collapse state, e.g. `const [expandedJobIds, setExpandedJobIds] = useState<Set<string>>(new Set())`, toggled per job `id`. No new `packages/ui` component — plain `useState` + the existing `Button`/`Table` primitives (mirrors this repo's existing "plain element" style, see Research notes).
- [x] In the jobs table's last (currently empty) `TableHead`/`TableCell` column (already used for the "К очереди на проверку" per-row link), add a `Button variant="outline" size="sm"` reading "Подробнее" / "Скрыть" (toggles on click), **rendered only when** `job.result` is present and `(job.result.errors.length > 0 || job.result.skipped_records.length > 0)` — a job with nothing to report gets no button, per User Stories. This works identically for any state (`active`/`completed`/`retry`/etc.) that happens to carry a non-empty `errors`/`skipped_records` list — not gated to `completed` only, since `harvest-progress.md` already established that `active`/partial results can carry real per-record data mid-run.
- [x] When a job's id is in `expandedJobIds`, render one additional `TableRow` immediately below that job's row, `<TableCell colSpan={4}>` containing:
  - If `job.result.errors.length > 0`: a small heading ("Ошибки") + a nested `Table` with columns `№` (1-based, `index + 1`), `Название` (`title || '—'`), `Причина` (`message`) — one row per `job.result.errors` entry, mirroring `book-import-view.tsx`'s existing error-table pattern exactly (same component usage, same `key={`${index}-${message}`}`-style key).
  - If `job.result.skipped_records.length > 0`: a small heading ("Пропущено") + a nested `Table` with columns `№` (1-based), `Название` (`title || '—'`), `Причина` — **every row's `Причина` cell is the fixed string "уже есть в каталоге (совпадение по внешнему источнику)"**, not a per-record message (there is exactly one skip cause today — do not invent per-record messages, see Backend `HarvestSkippedRecord`).
  - If both are empty (shouldn't happen given the button's visibility gate, but defensively), render nothing extra.
- [x] The existing `jobResultSummary`/`JobResultCell` text/progress-bar rendering is unchanged — this contract only adds the expand/collapse row below, not a replacement of the summary cell.
- [x] **Decision: reuse the already-polled `GET /jobs` list response's `result.errors`/`result.skipped_records` for the expanded view — do not add a new `GET /jobs/:id` call.** `GET /jobs` already returns full per-record `errors` detail for every job on every 5-second poll today (established, unchanged, by `harvest-progress.md`); adding `skipped_records` alongside it is the same shape of data, not a new class of problem. Wiring `adminHarvestApi.getJob`/`GET /jobs/:id` into the UI (already present but unused, per Research notes) is **not done by this contract** — flagged in Risks as a pre-existing follow-up opportunity, not required here, since the list response already contains everything the expand/collapse view needs.

## Business Rules

- A skip has exactly one cause today (idempotency: `external_source`/`external_id` already exists) — `HarvestSkippedRecord` intentionally has no `Message` field, and the frontend renders one fixed, hardcoded Russian label for every skipped-record row rather than echoing a backend-supplied string. If a second skip cause is ever introduced, this is the first place that would need a real per-record message (out of scope now).
- `len(HarvestResult.SkippedRecords) == HarvestResult.Skipped` always (both incremented in the same loop iteration, same branch) — a test-enforced invariant, not just a convention.
- No new authorization surface: this contract adds no new endpoint, field-only changes to existing responses already gated by `RoleAdmin` + existing read-auth from `admin-harvest-trigger.md`. Not re-specified here.
- This is a visibility-only change: nothing about which records get created, skipped, or errored is altered.

## UX

- Jobs table gains a "Подробнее"/"Скрыть" toggle button per row (in the existing trailing action column, alongside the "К очереди на проверку" link), shown only when there is at least one error or skip to report.
- Later ([author-moderation-queue.md](author-moderation-queue.md)): a "К авторам на модерации" link to `/authors/moderation` sits next to both "К очереди на проверку" links (header and per completed job). It opens the plain queue (newest first); per-job author filtering is deliberately not built, since jobs record no created author IDs.
- Expanding a row reveals a full breakdown directly beneath it: an "Ошибки" table (№/Название/Причина) if any errors, a "Пропущено" table (№/Название/Причина) if any skips — both scoped to that one job, collapsible independently per row.
- No new page, no new nav entry, no change to the 5-second polling cadence or the progress-bar behavior from `harvest-progress.md`.

## API

No new endpoint, no route change. `GET /api/v1/admin/harvest/jobs` and `GET /api/v1/admin/harvest/jobs/:id` response shape is unchanged except `result`'s underlying `HarvestResult` gains `skipped_records`:
```json
{
  "id": "...", "queue": "harvest", "state": "completed", "type": "harvest:wikidata_query",
  "max_retry": 3, "retried": 0,
  "result": {
    "processed": 40, "total": 40, "created": 35, "skipped_existing": 3,
    "errors": [{ "index": 12, "title": "...", "message": "..." }],
    "skipped_records": [{ "index": 5, "title": "..." }, { "index": 9, "title": "..." }, { "index": 20, "title": "..." }]
  }
}
```

## Database

None.

## Frontend

- `packages/api-client/src/index.ts` — `AdminHarvestJob.result.skipped_records: { index: number; title: string }[]` (new field).
- `apps/admin/src/views/harvest/ui/harvest-view.tsx` — expand/collapse state + per-row detail render, extending the jobs `Table`; existing form/progress-bar/summary rendering unchanged.

## Existing Patterns

- `backend/internal/modules/harvest/service/handler.go` — `runHarvest`'s per-record loop, `HarvestRecordError`, the `Errors: []HarvestRecordError{}` empty-slice-init convention — mirror exactly for `HarvestSkippedRecord`/`SkippedRecords`.
- `backend/internal/modules/harvest/transport/http/handler.go` — `toHarvestJobResponse`'s existing unconditional `Result` mapping — the mechanism this contract relies on for zero transport changes; confirm with a test rather than assuming (per `harvest-progress.md`'s own precedent of proving this rather than asserting it).
- `apps/admin/src/views/book-import/ui/book-import-view.tsx` — its `report.errors` → nested `Table` (Строка/Название/Ошибка) rendering is the closest existing analog for the new "Ошибки"/"Пропущено" nested tables; reuse the same `Table`/`TableHeader`/`TableRow`/`TableCell` composition and key style.
- `apps/admin/src/views/harvest/ui/harvest-view.tsx` — `JobResultCell`, `jobResultSummary`, `stateBadgeVariant`, the jobs `Table` — extend the trailing action column and add a new detail row; don't touch the Result-cell or State-badge logic.

## Tests

- Backend, `backend/internal/modules/harvest/service/handler_test.go` (extend existing): a run with a mix of created/skipped/error records asserts `result.SkippedRecords` has one entry per skipped record with the correct `Index`/`Title`, in encounter order; asserts `len(result.SkippedRecords) == result.Skipped` for that run; asserts a `reportProgress` capture (fake `progressWriter`, existing harness from `harvest-progress.md`) shows `SkippedRecords` growing incrementally alongside `Errors`, not just present in the final result.
- Backend, `backend/internal/modules/harvest/transport/http/handler_test.go` (extend existing): a fake `TaskInspector` returning a `TaskInfo` whose `Result` JSON includes a non-empty `skipped_records` array → `GET /jobs`/`GET /jobs/:id` responses include it unchanged/undropped, for both an `active`-state and a `completed`-state fake task (proves the "no transport change needed" claim for both states, not just one).
- `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`.
- Frontend, `apps/admin/src/views/harvest/ui/harvest-view.test.tsx` (extend existing, per Research notes — file already exists and already mocks `getJob`): a completed job with non-empty `errors` and `skipped_records` renders a "Подробнее" button; clicking it reveals both nested tables with the expected per-record rows (error rows show the real `message`; skipped rows show the fixed "уже есть в каталоге..." label regardless of input); clicking "Скрыть" collapses them again; a job with empty `errors` and empty `skipped_records` renders no "Подробнее" button; an `active` job with a partial result that already has some errors/skips still renders and works the button (not gated to `completed`); existing progress-bar/summary rendering for other states is unchanged (regression check).
- Admin app type-check/lint clean; `packages/ui`/`packages/api-client` type-check clean (no `packages/ui` changes expected, verify no new dependency was introduced).

## Documentation

- Mark this file `Status: implemented` when done.
- `docs/features/harvest-progress.md`: no change needed (its own contract already documents `Result` being available mid-job; this contract only adds a sibling field, not a new mechanism) — skip unless review finds an explicit inaccuracy introduced.
- `docs/features/admin-harvest-trigger.md`: optionally add a one-line pointer near its `AdminHarvestJob` type listing noting `skipped_records` was added by this contract — one line, matching the existing cross-contract pointer convention (e.g. how it already notes `harvest-progress.md` added `total`). Not required if it becomes noisy; developer's call.

## Risks

- **List-response payload size**: `GET /jobs` already returns full `errors` detail for up to 50 jobs per state (250 jobs total) on every 5-second poll, unchanged since `harvest-progress.md`; this contract adds a parallel `skipped_records` list of the same order of magnitude. Worst case (a large `-limit` harvest where most records are either errors or skips) roughly doubles that pre-existing payload characteristic — not a new problem, but worth monitoring. If this proves too heavy in practice, the documented follow-up is to move per-record detail (`errors`+`skipped_records`) out of `GET /jobs` and behind an on-demand `GET /jobs/:id` fetch triggered by expanding a row (the endpoint and the frontend `adminHarvestApi.getJob` call already exist for this, just unused) — deliberately not built now, since the existing `errors`-in-list precedent means this isn't a regression introduced by this contract, and the realistic usage pattern (occasional, low-volume admin triggering, per `admin-harvest-trigger.md`'s own accepted usage assumption) doesn't obviously warrant it yet.
- **`GET /jobs/:id` stays wired-but-unused after this contract** — a pre-existing condition (confirmed in Research notes, not introduced here) that this contract deliberately does not resolve, per the payload-size risk above. Flagged so a future contract doesn't have to re-derive this.
- **No message/reason field on skipped records is a deliberate simplification that could look "incomplete" to an admin** expecting a proper per-record explanation — mitigated by the fixed label being accurate today (there really is only one cause), and by this contract's Business Rules calling out exactly where a real per-record message would need to be added if a second skip cause is ever introduced.
