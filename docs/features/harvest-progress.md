# Harvest Progress

Status: implemented

This is the next increment on top of `docs/features/admin-harvest-trigger.md` (implemented: `POST /api/v1/admin/harvest`, `GET /api/v1/admin/harvest/jobs`/`/jobs/:id` backed by `asynq.Inspector`, and the admin `/harvest` page polling every 5s). Today the admin UI can only show which of asynq's five coarse task states a job is in (`pending`/`active`/`retry`/`completed`/`archived`) plus, once `completed`, a final `HarvestResult` summary. This contract adds a real "N of M processed" progress bar for jobs that are currently `active`, by having the harvest handler report incremental progress while it runs — not just a result at the end.

## Scope boundaries (read first)

**Everything from `async-harvest-queue.md` and `admin-harvest-trigger.md` stays exactly as decided — not reopened here:**
- Curated author-QID-only input, no raw SPARQL in the admin UI.
- `asynq`'s own Redis-backed task/queue state remains the sole source of truth — no new Postgres table, no new persistence layer beyond what's described below.
- The coarse `state` badge (pending/active/retry/completed/archived) stays and is not replaced — this contract *augments* the `active` row with a progress bar, it does not redesign the state model.
- 5-second fixed-interval polling stays — no WebSockets/SSE. Explicitly re-confirmed out of scope by the request; not reopened.
- The worker's queue concurrency (1), rate limiting/throttling between Open Library calls, enrichment merge policy ("Wikidata wins"), and per-record error handling are all unchanged. This contract only adds a counter/reporting hook around the existing loop — no change to *what* gets processed or in what order.
- Cancelling/retrying jobs from the admin UI, bulk triggering, a persistent job-history table — all still out of scope, unchanged from the prior contract.

**Explicitly out of scope (do not build):**
- Any change to `cmd/harvest`'s (the CLI's) own console output.
- Progress reporting for any other async task type — there is only one today (`harvest:wikidata_query`); this is not a general-purpose "task progress" framework.
- A dedicated Redis client dependency in the HTTP handler layer *if* the `asynq.ResultWriter`-based design (Plan A, below) holds up — see the research note before Acceptance Criteria for when this would change.

## Research note — asynq's live-progress capability (read before implementing)

The request asked whether `hibiken/asynq` (pinned `v0.25.1` in `backend/go.mod`) has a built-in mechanism for a handler to report progress *while a task is still `active`*, as opposed to only writing a final result after the handler returns.

**What I could confirm by reading this repo's code:** `harvest/transport/http/handler.go`'s `toHarvestJobResponse` already surfaces `TaskInfo.Result` unconditionally whenever `len(info.Result) > 0` and it unmarshals cleanly into `harvestservice.HarvestResult` — it does **not** gate this on `State == completed` (only the doc comment on the `harvestJobResponse.Result` field claims that; the code itself has no such check). That is a strong signal the endpoint was already, incidentally, written in a way compatible with a result being present on a non-terminal task.

**What I could not confirm by direct execution:** this planning environment has no shell/Docker access (per this repo's own dev-environment note, the Go backend is Docker-only) and no local Go module cache, so I could not run `go doc github.com/hibiken/asynq ResultWriter.Write` or read the vendored `v0.25.1` source directly. Based on documented `asynq` behavior (not this-repo evidence): `(*asynq.Task).ResultWriter().Write(data)` performs a Redis write against the task's own canonical hash key (`asynq:{queue}:t:{id}`) — the same key `Inspector.GetTaskInfo`/`List*Tasks` read `Result` from regardless of the task's current state (pending/active/retry/completed/archived all reference the same hash entry; only membership in per-state ZSETs differs). If that holds for `v0.25.1`, `Write` can safely be called multiple times during a single task's execution, and each call's bytes become visible via `GetTaskInfo`/`List*Tasks` immediately — including while `State == active`.

**Required first step for the developer, before building the rest of this contract:** confirm this in the actual pinned `v0.25.1` source (`docker compose run --rm backend go doc github.com/hibiken/asynq ResultWriter.Write`, and/or read `rdb.go`'s `WriteResult`/`GetTaskInfo` in the module cache inside the container). Two outcomes:
- **Confirmed (expected, "Plan A" below):** proceed with the `ResultWriter`-based design — no new Redis client dependency, reuses the exact same `Result` field/decoding path the prior contract already built.
- **Falsified** (e.g. `Write` errors or is documented as callable only once, or `GetTaskInfo` only surfaces `Result` for terminal states): fall back to **"Plan B"**, sketched in Risks — a dedicated `harvest:progress:<task_id>` Redis hash key (`processed`, `total` fields, short TTL e.g. 1h) written directly by the handler via a small Redis client interface (not through `asynq`), read by a new code path in the HTTP handler that only kicks in for `active` jobs. Plan B is a larger diff (new Redis dependency in the transport layer, a second read path alongside `TaskInfo`) — do not build it preemptively; only switch to it if Plan A is actually falsified, and if so, come back and update this contract's Acceptance Criteria/API sections accordingly before implementing.

The rest of this contract is written assuming Plan A (the expected outcome).

## User Stories

- As a content admin watching an active harvest job, I can see roughly how far through it is (e.g. "37 / 200 (18%)") instead of just an "active" badge, without leaving the admin panel or refreshing manually.
- As a content admin, once a job finishes I still see the same final summary (created/skipped/errors) as before — this contract doesn't change what's shown for completed/archived jobs, only what's shown while a job is active.

## Acceptance Criteria

**Backend — progress reporting in the handler**
- [x] Verify the Plan A premise (see Research note) in the pinned `asynq v0.25.1` before writing the rest of this code; record the verification (e.g. a one-line comment near the new progress-writing code citing what was checked) since this contract's design depends on it.
- [x] `backend/internal/modules/harvest/service/handler.go`: `HarvestResult` gains a `Total int `json:"total"`` field (alongside existing `Processed`, `Created`, `Skipped`, `Errors`). `Total` = `len(records)` — known immediately after `h.wikidata.Query` returns, before the per-record loop starts. `Processed` keeps its existing meaning (count of records the loop has handled so far, whether created/skipped/errored) but is now also observable *while the loop is running*, not just in the final written result.
- [x] New unexported interface `progressWriter` (satisfied by `*asynq.ResultWriter`, i.e. `t.ResultWriter()`, in production):
  ```go
  type progressWriter interface {
      Write(data []byte) (int, error)
  }
  ```
- [x] `runHarvest` gains a `progress progressWriter` parameter: `func (h *Handler) runHarvest(ctx context.Context, payload HarvestPayload, progress progressWriter) (HarvestResult, error)`. `HandleTask` calls it as `h.runHarvest(ctx, payload, t.ResultWriter())`. Existing/new unit tests (which construct tasks via `asynq.NewTask` and have no live `ResultWriter`) pass `nil` or a hand-rolled fake; `runHarvest`/its helper must treat `nil` as "no-op, don't write" (tests that don't care about progress reporting are unaffected).
- [x] A small helper (e.g. `reportProgress(progress progressWriter, result HarvestResult)`) marshals `result` to JSON and calls `progress.Write(data)`, swallowing/logging (not returning) any error — a progress-write failure must never fail the harvest job itself.
- [x] `runHarvest` calls `reportProgress` once immediately after the query returns (`Total` set, `Processed: 0`), and again after each record is processed in the existing loop (after `result.Processed`/`Created`/`Skipped`/`Errors` are updated for that iteration) — i.e. progress is visible after every record, not batched/throttled. This is intentionally simple: at ~1 req/sec per record (existing Open Library enrichment throttle), one Redis write per record is negligible overhead and needs no batching.
- [x] `HandleTask`'s existing final `t.ResultWriter().Write(resultBytes)` after `runHarvest` returns is unchanged (still there, still redundant-but-harmless with the last in-loop write — keeps the "final result" contract from `async-harvest-queue.md` intact without relying on the loop's last iteration having actually executed, e.g. for a zero-record result).
- [x] No change to `HandleTask`'s error paths (`asynq.SkipRetry` for bad payload, retry-eligible errors for query failures) — progress reporting only happens inside the existing success path (`runHarvest`'s loop), consistent with "no change to the worker's processing logic beyond the counter/reporting hook."

**Backend — job status endpoints**
- [x] No route or handler-shape change required in `harvest/transport/http/handler.go` under Plan A: `toHarvestJobResponse` already surfaces `Result` (now including `Total`) unconditionally whenever present and decodable, regardless of `State`. Confirm this explicitly with a test (see Tests) rather than assuming — this is the one place where "it already works" must be proven, not just asserted.
- [x] `packages/api-client` and Go swaggo docs both need the new `Total`/`total` field documented (see Frontend + existing `@Success` annotations on `ListHarvestJobs`/`GetHarvestJob` — no shape change, just the new field flowing through the existing `harvestservice.HarvestResult` reference).
- [x] Edge case: a `pending`/`retry` job has no `Result` yet (no live `ResultWriter` exists until the task is dequeued and actively processing) — `Result` stays `nil`, exactly as today. Document this as expected, not a bug, and cover it with a test.
- [x] Edge case: `Total == 0` (query returned zero records) is a valid, representable state (`Processed: 0, Total: 0`) — the frontend must not divide by zero when computing a percentage (see Frontend).

**Frontend (`apps/admin`)**
- [x] `packages/api-client/src/index.ts`: `AdminHarvestJob.result` gains `total: number` alongside the existing `processed`/`created`/`skipped_existing`/`errors` fields.
- [x] New minimal `Progress` component in `packages/ui` (checked first: no existing progress/meter-shaped component in `packages/ui/src/components/ui/*` — `button`/`input`/`textarea`/`label`/`select`/`card`/`table`/`badge` only, and no `@radix-ui/react-progress` dependency in `packages/ui/package.json`). Build a plain div-based bar, matching `select.tsx`'s "plain HTML element + `cn`" style (no new Radix dependency needed — the existing `@radix-ui/react-slot` dependency is unrelated, used only by `Button`'s `asChild`):
  ```tsx
  // packages/ui/src/components/ui/progress.tsx
  export interface ProgressProps extends React.HTMLAttributes<HTMLDivElement> {
    value: number; // 0-100, caller clamps before passing if needed
  }
  function Progress({ value, className, ...props }: ProgressProps) {
    const clamped = Math.min(100, Math.max(0, value));
    return (
      <div
        role="progressbar"
        aria-valuenow={clamped}
        aria-valuemin={0}
        aria-valuemax={100}
        className={cn('h-2 w-full overflow-hidden rounded-full bg-secondary', className)}
        {...props}
      >
        <div className="h-full bg-primary transition-all" style={{ width: `${clamped}%` }} />
      </div>
    );
  }
  export { Progress };
  ```
  Export from `packages/ui/src/index.ts` alongside the other `components/ui/*` exports.
- [x] `apps/admin/src/views/harvest/ui/harvest-view.tsx`: for a job with `state === 'active'` and a present `result` with `result.total > 0`, render the `Progress` bar plus a text label (`"{processed} / {total} ({percent}%)"`, percent computed client-side as `Math.round((processed / total) * 100)`, `processed` clamped to `total`) in place of the existing plain-text `jobResultSummary` output, in the same "Результат" table cell — no new column. For `state === 'active'` with `result` absent (job dequeued but the first `reportProgress` write hasn't landed yet — a brief window right after the query starts) or `result.total === 0` (zero-record query), show a short "Выполняется…" text instead of a bar (avoid a div-by-zero / a misleading 0%-forever bar). All other states (`pending`/`retry`/`completed`/`archived`) keep their existing `jobResultSummary`/badge treatment exactly as implemented — **do not** surface a stale progress bar for `retry` (a retried job's last-written `Result` reflects the *previous, failed* attempt, not the upcoming one — see Risks); treat `retry` the same as today (last_error or "—").
- [x] The `Badge`/state-variant mapping (`stateBadgeVariant`) is unchanged — this contract augments the Result cell only, not the State cell.

## Business Rules

- `Total` is fixed once known (right after the SPARQL query executes) and never recomputed mid-job — matches the existing pipeline shape (one query, then a fixed-size per-record loop; no dynamic addition of records mid-job).
- Progress is best-effort and non-authoritative: a progress-write failure (e.g. a transient Redis hiccup) must never fail or retry the harvest job — only the final explicit `ResultWriter().Write` at the end of `HandleTask` is treated as required (its existing error path, returning an error from `HandleTask`, is unchanged).
- A `retry`-state job's `Result` (if any) reflects its most recent *failed* attempt's partial progress, not a preview of the next attempt — the frontend intentionally does not surface it, to avoid showing stale/confusing numbers (see Frontend + Risks).
- No new authorization surface: this contract adds no new endpoint or route, only new fields on the existing `GET /admin/harvest/jobs`/`/jobs/:id` responses, already gated by the existing `RoleAdmin` + read-auth requirements from `admin-harvest-trigger.md`. Not re-specified here.

## UX

- The jobs table's "Результат" column, for an `active` job, becomes a progress bar (`N / M (P%)`) instead of "—". For `completed` jobs it is unchanged (created/skipped/errors summary). For `pending`/`retry`/`archived` it is unchanged ("—" / `last_error`).
- No new page, no new nav entry, no change to the 5-second polling cadence — the existing `useQuery({ refetchInterval: 5000 })` already re-fetches `GET /jobs`, which now happens to carry richer `result` data for active jobs; the bar simply animates forward on each poll tick.

## API

`GET /api/v1/admin/harvest/jobs` and `GET /api/v1/admin/harvest/jobs/:id` — response shape unchanged (`harvestJobResponse` with an optional `result`), except `result`'s underlying `harvestservice.HarvestResult` gains `total`:
```json
{
  "id": "...", "queue": "harvest", "state": "active", "type": "harvest:wikidata_query",
  "max_retry": 3, "retried": 0,
  "result": { "processed": 37, "total": 200, "created": 0, "skipped_existing": 0, "errors": [] }
}
```
`result` may now be present (with a partial `processed`/`total`) for `state: "active"` jobs, not only `"completed"` — this is the behavior change; no new endpoint, no new status code, no change to `POST /admin/harvest`.

## Database

None.

## Frontend

- `packages/api-client/src/index.ts` — `AdminHarvestJob.result.total: number` (new field).
- `packages/ui/src/components/ui/progress.tsx` — new, plain div-based `Progress` component; exported from `packages/ui/src/index.ts`.
- `apps/admin/src/views/harvest/ui/harvest-view.tsx` — Result-cell rendering gains an active-job progress-bar branch; existing form/table/other-state rendering unchanged.

## Existing Patterns

- `backend/internal/modules/harvest/service/handler.go` — `HandleTask`/`runHarvest`/`processRecord`, `HarvestResult`/`HarvestRecordError`, `NewHarvestTask` — extend in place, do not restructure the module (no new `harvest` submodule needed for this).
- `backend/internal/modules/harvest/transport/http/handler.go` — `toHarvestJobResponse`'s existing unconditional `Result` mapping is the mechanism this contract relies on; read it before touching anything here, since (per the Research note) the expectation is **zero changes needed** in this file.
- `packages/ui/src/components/ui/badge.tsx` / `select.tsx` — "plain element + `cn` + typed props" style to mirror for the new `Progress` component; no Radix/new dependency precedent to follow here (Badge/Select/Input/Textarea are all plain elements).
- `apps/admin/src/views/harvest/ui/harvest-view.tsx` — `stateBadgeVariant`, `jobResultSummary`, the jobs `Table` rendering — extend `jobResultSummary`'s call site (not its badge logic) for the active-progress branch.

## Tests

- Backend, `backend/internal/modules/harvest/service/handler_test.go` (extend existing): a fake `progressWriter` capturing every `Write` call's bytes — assert `reportProgress` is called once with `Processed: 0, Total: N` immediately after the query, and once more per record with `Processed` monotonically increasing to `N`; assert a `nil` progress writer doesn't panic (covers the existing tests that don't pass one); assert a fake writer returning an error from `Write` does not fail `runHarvest`/`HandleTask` (job still completes/returns its final result normally).
- Backend, `backend/internal/modules/harvest/transport/http/handler_test.go` (extend existing): a fake `TaskInspector` returning a `*asynq.TaskInfo` with `State == asynq.TaskStateActive` and a valid partial `HarvestResult` (`Processed < Total`) in `Result` → `GET /jobs`/`GET /jobs/:id` responses include that partial `result` (proves the "no handler-shape change needed" claim in Acceptance Criteria, rather than just asserting it); a `TaskStateActive` task with no `Result` yet → `result` is `nil`/omitted, no error.
- `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`.
- Frontend, `packages/ui` — a small render test for the new `Progress` component (renders with correct `aria-valuenow`/width style at a given `value`, clamps out-of-range input) if `packages/ui` has any existing component test precedent to follow; otherwise note as manual/type-check-only verification, consistent with this package's existing testing bar (check for a `*.test.tsx` file anywhere under `packages/ui/src` before deciding which).
- Frontend, `apps/admin/src/views/harvest/ui/harvest-view.test.tsx` (extend if it exists, per `admin-harvest-trigger.md`'s own note to check first): an `active` job with `result.total > 0` renders a progress bar with the expected `N / M (P%)` text; an `active` job with no `result` or `result.total === 0` renders "Выполняется…", not a bar; a `retry` job with a stale `result` still renders "—"/`last_error` as before, not a bar (guards the intentional no-stale-progress-for-retry decision); `completed`/`archived` rendering is unchanged (regression check).
- Admin app type-check/lint clean; `packages/ui` type-check/lint clean.

## Documentation

- Mark this file `Status: implemented` when done.
- `docs/features/admin-harvest-trigger.md`: add a one-line pointer near its `GET /jobs`/`jobs/:id` Acceptance Criteria noting that `harvest-progress.md` extended `HarvestResult` with `total` and that `active`-state jobs can now carry a partial `result` — one or two lines, not a rewrite, matching the existing cross-contract pointer convention already used in this repo (e.g. how `admin-harvest-trigger.md` itself references superseding `async-harvest-queue.md`).
- If Plan A is falsified and Plan B (Redis progress key) is built instead, this file's Research note / API / Acceptance Criteria sections need a follow-up edit before merge to reflect what was actually built — don't let the merged code and this contract drift.

## Risks

- **This contract's core design (Plan A) rests on an asynq behavior I could not directly verify in this planning session** (no shell/Docker access here; the claim is based on documented `asynq` Redis-backed result-storage behavior, not a read of this repo's vendored `v0.25.1` source). This is the single biggest risk to this contract and is called out explicitly as the first Acceptance Criterion the developer must confirm before writing the rest of the code. If falsified, budget extra time for Plan B (new Redis client dependency + a second read path in the HTTP handler, not just a field addition) — noticeably larger than the diff described above.
- **Retry-state stale progress**: a job's `Result` after a failed attempt (now in `retry` state, awaiting backoff) reflects that failed attempt's partial progress, not the next one. This contract's mitigation is UI-only (don't render a bar for `retry`) — the underlying stale data is still technically retrievable via `GET /jobs/:id` by anyone inspecting the raw response; low-impact (internal admin tool, and the state badge already flags "retry" distinctly), not mitigated further.
- **Per-record Redis writes add outbound calls the worker didn't make before** (one extra `HSET`-equivalent per record, on top of the existing per-record Postgres writes and the ~1 req/sec Open Library call) — negligible relative to the existing per-record latency budget (dominated by the Open Library rate limit), but worth noting as a small new dependency-coupling point: if Redis is briefly unavailable mid-job, progress-reporting failures are swallowed (per Business Rules) but a developer debugging "why isn't progress showing" should know to check Redis health, not just worker logs.
- **A job whose very first record takes a while (e.g., a slow enrichment lookup) shows "Выполняется…" with no bar for that window** — acceptable, matches the existing 5-second-poll-driven UX (the bar appears on the next poll after the first `reportProgress` call lands), not worth special-casing further for this MVP tool.
