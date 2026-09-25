# Admin Harvest Trigger

Status: implemented

This is the next increment on top of `docs/features/async-harvest-queue.md` (implemented: Redis-backed asynq queue, `worker` process, `harvest:wikidata_query` task) and `docs/features/harvest-review-queue.md` (implemented: `bookdata.BuildAuthorListQuery` curated-author-QID SPARQL builder, `pending_review` admin book-list filter + publish action). It gives content admins a way to *trigger* and *observe* the existing harvest pipeline from the admin panel, instead of only via the `cmd/harvest` ops CLI.

## Scope boundaries (read first)

**Deliberate supersession of a prior scoping decision.** `docs/ai/features/catalog-data-import/business-analysis.md` (Решение 2/4, FR-010) explicitly scoped harvesting as an offline ops/developer CLI tool, *not* exposed via admin HTTP API, and accepted R-004 ("способ A bypasses admin-API-level audit") as a low-priority residual risk specifically *because* the CLI was assumed rare/ops-only. The product owner has now explicitly asked for a content-admin-facing trigger. This contract treats that as a deliberate, informed change to *that one decision* — not a contradiction to flag as blocking — and implements it. `docs/features/async-harvest-queue.md`'s Scope-boundaries line ("no admin HTTP endpoint... in this increment") is superseded the same way, for the same reason.

**Why this doesn't reopen the synchronous-HTTP-timeout concern (FR-010's other stated rationale):** the actual harvest work (Wikidata SPARQL query + per-record Open Library enrichment at ~1 req/sec) runs entirely inside `worker`, asynchronously. Enqueueing — building the SPARQL string and calling `asynq.Client.Enqueue` — is fast and does not touch any external network API (exactly what `cmd/harvest` already does today). An HTTP endpoint that only enqueues is not exposed to that timeout risk; nothing in this contract makes the worker's processing synchronous.

**Everything else from the two prior contracts stays exactly as decided — not reopened by this request:**
- Curated author-QID-only input (Approach A) — no raw/arbitrary SPARQL in the admin UI. `-query`/`-query-file` stay CLI-only, exactly as `harvest-review-queue.md` already split "convenience mode" (author QIDs) from "raw mode" (SPARQL) for the CLI itself. This contract only exposes the convenience-mode input shape (author QIDs + limit) to the admin UI.
- `bookdata.BuildAuthorListQuery`'s bounded `LIMIT` (default 500, clamp to 5000) — unchanged, reused as-is.
- Harvested books are always `is_hidden = true` with `external_source`/`external_id` set — unchanged, worker-side rule, not caller-configurable.
- The existing `pending_review` queue/publish flow (`visibility=pending_review`, "Импортировано" badge, "Опубликовать" action, dashboard tile) is how an admin acts on harvest results — unchanged, this contract just links to it.
- The `harvest:wikidata_query` task type, `HarvestPayload{SPARQLQuery string}`, `harvestservice.NewHarvestTask` (queue `"harvest"`, `MaxRetry(3)`, `Timeout(30m)`, `Retention(24h)`), the worker's per-record processing logic, and `bookdata` clients are all unchanged — this feature is purely "give admins a way to enqueue + observe the existing pipeline."

**Explicitly out of scope (do not build):**
- Raw/arbitrary SPARQL input anywhere in the admin UI (CLI-only, per above).
- Cancelling or retrying a job from the admin UI — not requested; asynq supports it, but don't wire it in this increment.
- A persistent harvest-job-history table in Postgres. `asynq`'s own task/queue state (via `asynq.Inspector`) is the source of truth, matching `async-harvest-queue.md`'s original decision not to duplicate it in a DB table. Consequence: job visibility is bounded by asynq's own retention (`Retention(24h)` on completed/archived tasks; pending/active/retry tasks are visible until processed) — there is no "harvest trigger history" beyond what Redis currently holds. Document this limitation in the UI/README, don't work around it with new persistence.
- Any change to the worker's processing logic, `bookdata` clients, or the task/payload shape.
- WebSockets/SSE for live job status — simple interval polling from the admin frontend is sufficient for this MVP tool.
- Bulk/batch triggering (multiple separate jobs from one form submission) — one form submission = one enqueued task, matching `cmd/harvest`'s own one-invocation-one-task model.

## User Stories

- As a content admin, I can open a page in the admin panel, enter a list of Wikidata author QIDs and an optional limit, and trigger a harvest job without using a terminal or having server access.
- As a content admin, after triggering a job I can see its current state (queued / running / retrying / done / failed) without leaving the admin panel, and without needing to know what asynq or Redis is.
- As a content admin, once a job completes I can see a summary (processed/created/skipped/errors) and follow a link straight to the pending-review queue to check what it produced.
- As a content admin, I cannot trigger a harvest with hand-written SPARQL or an unbounded result size — the form only accepts the same safe, curated input the CLI's `-authors`/`-limit` flags already accept.

## Acceptance Criteria

**Backend — trigger endpoint**
- [x] New package `backend/internal/modules/harvest/transport/http` (new `transport/http` subpackage of the existing `harvest` module, mirroring `admin/transport/http`'s `Handler`/interface-dependency shape). It does **not** live inside the `admin` module — the admin module stays free of an `asynq` dependency, matching how `harvest/service` today only depends on `adminservice`, never the other way around.
- [x] `Handler` depends on two small interfaces (not the concrete `*asynq.Client`/`*asynq.Inspector` directly), satisfied by the real asynq types in production and by hand-rolled fakes in tests (repo's existing no-mocking-library convention):
  ```go
  type TaskEnqueuer interface {
      Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
  }
  type TaskInspector interface {
      GetTaskInfo(qname, id string) (*asynq.TaskInfo, error)
      ListPendingTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
      ListActiveTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
      ListRetryTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
      ListArchivedTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
      ListCompletedTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
  }
  func NewHandler(enqueuer TaskEnqueuer, inspector TaskInspector) *Handler
  ```
- [x] `POST /api/v1/admin/harvest`: body `{"author_qids": string[], "limit"?: number}`. Handler trims/passes `author_qids` straight into `bookdata.BuildAuthorListQuery(authorQIDs, limit)` (the same function `cmd/harvest -authors` already uses) — no separate comma/newline-splitting logic on the backend (see API section for why the array shape was chosen). A `bookdata.ErrInvalidQuery`-wrapped error (empty list, malformed QID) → `400` with the underlying message. On success: builds the task via `harvestservice.NewHarvestTask(query)` (unchanged, options untouched) and calls `enqueuer.Enqueue(task)`; a Redis/enqueue error → `502` with a generic message (mirrors how other handlers map downstream-dependency failures, e.g. `admin` handler's `500` pattern, but distinguished as `502` since this is explicitly an upstream-dependency failure, not a server bug — developer's call to keep `500` instead if that reads as inconsistent with the rest of the codebase's status-code conventions; document whichever is chosen). **Decision: `502`** — implemented as chosen; see the comment above `TriggerHarvest`'s enqueue-error branch in `backend/internal/modules/harvest/transport/http/handler.go`, which mirrors `admin` handler's `LookupBook` use of `502` for `bookdata.ErrRateLimited`/`ErrSourceUnavailable` (downstream-dependency failure, not a server bug). Success response `201`: `{"task_id": "...", "queue": "harvest", "state": "pending"}`.
- [x] Route registered `admin.Group("/harvest", adminRole)` → `harvest.POST("", h.TriggerHarvest, requireBearer)` — same `adminRole` (`RequireRole(usersdomain.RoleAdmin)`) + `requireBearer` (`middleware.RequireBearerAuth()`) combination already used for every other state-changing admin route (`books.POST`, `books.PATCH`, etc. in `backend/internal/transport/http/router.go`). Moderators get `403`, same as today for books/authors/genres.
- [x] Swaggo doc comments on the new handler methods, matching the existing convention (`// @Summary`, `@Tags admin`, `@Security BearerAuth`, etc., see `CreateBook` in `backend/internal/modules/admin/transport/http/handler.go` for the exact shape to mirror).

**Backend — job status endpoints**
- Superseded in part by `docs/features/harvest-progress.md`: `harvestservice.HarvestResult` gained a `total` field, and `result` can now be present (with a partial `processed`/`total`) for `state: "active"` jobs, not only `"completed"` — see that contract for the progress-reporting mechanism.
- [x] `GET /api/v1/admin/harvest/jobs` (no `requireBearer` — read-only, same convention as `books.GET`/`books.GET("/:id")`, just `adminRole`): calls `ListPendingTasks`, `ListActiveTasks`, `ListRetryTasks`, `ListCompletedTasks`, `ListArchivedTasks` on `harvestservice.QueueName` ("harvest"), each bounded to a page size of 50 (`asynq.PageSize(50)` — document this number: "recent" = up to 50 most-recent tasks per state, which given the accepted low-frequency usage pattern for this tool comfortably covers realistic admin usage; revisit only if it proves too small in practice). Concatenates results (grouped by state, in the fixed order active → retry → pending → completed → archived — **not** a single global chronological sort, since `asynq.TaskInfo` doesn't expose a single comparable timestamp valid across all states; document this ordering caveat) into one JSON array.
- [x] `GET /api/v1/admin/harvest/jobs/:id`: calls `inspector.GetTaskInfo(harvestservice.QueueName, id)` (queue is hardcoded — there is currently only one harvest queue). `asynq.ErrTaskNotFound` → `404`. 
- [x] Both endpoints map each `*asynq.TaskInfo` to:
  ```go
  type harvestJobResponse struct {
      ID        string                        `json:"id"`
      Queue     string                        `json:"queue"`
      State     string                        `json:"state"` // "pending"|"active"|"retry"|"archived"|"completed", via TaskInfo.State.String()
      Type      string                        `json:"type"`
      MaxRetry  int                           `json:"max_retry"`
      Retried   int                           `json:"retried"`
      LastError string                        `json:"last_error,omitempty"`
      Result    *harvestservice.HarvestResult `json:"result,omitempty"` // only populated when State == completed and TaskInfo.Result decodes cleanly
  }
  ```
  A `TaskInfo.Result` that fails to `json.Unmarshal` into `harvestservice.HarvestResult` (should not happen in practice, but the field is untyped `[]byte` from asynq's perspective) is treated as absent (`Result` stays `nil`, response is otherwise unaffected) — not a `500`; this is defensive, not expected to trigger.
- [x] `backend/internal/transport/http/router.go`'s `RegisterAdminRoutes` gains two new parameters (`harvestEnqueuer harvesthttp.TaskEnqueuer, harvestInspector harvesthttp.TaskInspector`, or a small options struct — developer's call on exact signature shape, minimize diff) and registers the new `harvestHandler` + routes alongside the existing `books`/`authors`/`genres`/`reviews` groups.
- [x] `backend/cmd/api/main.go` constructs a real `*asynq.Client` (`asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisURL})`, same construction `cmd/harvest` already uses) and a real `*asynq.Inspector` (`asynq.NewInspector(asynq.RedisClientOpt{Addr: cfg.RedisURL})`), passes both into `RegisterAdminRoutes`, and `defer client.Close()` / `defer inspector.Close()` alongside the existing shutdown sequence (mirrors `cmd/harvest`'s `defer client.Close()`).

**Frontend (`apps/admin`)**
- [x] `packages/api-client/src/index.ts` gains:
  - `AdminHarvestTriggerInput { author_qids: string[]; limit?: number }`
  - `AdminHarvestTriggerResponse { task_id: string; queue: string; state: string }`
  - `AdminHarvestJob { id: string; queue: string; state: string; type: string; max_retry: number; retried: number; last_error?: string; result?: { processed: number; created: number; skipped_existing: number; errors: { index: number; title: string; message: string }[] } }`
  - `api.admin.harvest = { trigger: (data: AdminHarvestTriggerInput) => Promise<AdminHarvestTriggerResponse>, jobs: { list: () => Promise<{ items: AdminHarvestJob[] }>, get: (id: string) => Promise<AdminHarvestJob> } }`, following the existing `api.admin.books`/`api.admin.reviews` structure/typed-fetcher style exactly.
- [x] New entity module `apps/admin/src/entities/harvest/api/index.ts` exporting `adminHarvestApi = { trigger: api.admin.harvest.trigger, listJobs: api.admin.harvest.jobs.list, getJob: api.admin.harvest.jobs.get }` (thin re-export, mirrors `apps/admin/src/entities/book/api/index.ts`).
- [x] New view `apps/admin/src/views/harvest/ui/harvest-view.tsx`:
  - A form: a `Textarea`-or-multiline `Input` for author QIDs (one per line **or** comma-separated — accept both: split client-side on `/[,\n]/`, trim, drop blanks, dedupe before building the `author_qids` array sent to the API; document this in a short helper-text line under the field, e.g. "Один QID на строку или через запятую, например Q7200"), a numeric `Input` for limit (placeholder `500`, no hard client-side max beyond a sane `type="number"` — the real clamp is server-side via `BuildAuthorListQuery`), and a submit `Button`.
  - Client-side validation: at least one QID required after parsing (mirrors `book-import-view.tsx`'s pattern of a local `error` state + inline message, not a full form library) before calling the mutation; malformed-QID errors surface via the `400` response message from `bookdata.ErrInvalidQuery` (same "show `err.message` from `ApiError`" pattern already used in `book-import-view.tsx`'s `onError`).
  - `useMutation` calling `adminHarvestApi.trigger`; on success, invalidate/refetch the jobs list query (`queryClient.invalidateQueries({ queryKey: ['admin', 'harvest', 'jobs'] })`) so the newly triggered job appears without waiting for the next poll tick.
  - A jobs section: `useQuery({ queryKey: ['admin', 'harvest', 'jobs'], queryFn: adminHarvestApi.listJobs, refetchInterval: 5000 })` (simple fixed 5s interval poll — document as an accepted MVP simplification, no backoff/visibility-based pausing required). Rendered as a `Table` (reuse `@bookhaven/ui` `Table`/`TableHeader`/`TableRow`/`TableCell` exactly as `book-import-view.tsx` already does for its error rows) with columns: ID (truncated/monospace), State (`Badge`, variant mapped per state — e.g. `active`→`default`, `pending`/`retry`→`secondary`, `archived`→`destructive`, `completed`→`outline`; exact mapping is the developer's call, just be consistent), Result summary (when `completed`: "создано X, пропущено Y, ошибок Z"; when `archived`/has `last_error`: show `last_error`; otherwise "—"), and a link "К очереди на проверку" → `/books?visibility=pending_review` shown when a `completed` job's `result.created > 0`.
  - No new routes/pages beyond this one view — this is a net-new page, not an extension of an existing view (unlike the prior two harvest contracts, which were additive to existing books-list/dashboard).
- [x] `apps/admin/src/app/harvest/page.tsx`: `export default function Page() { return <HarvestView />; }`, mirrors `apps/admin/src/app/books/import/page.tsx`.
- [x] `apps/admin/src/widgets/admin-shell/ui/shell.tsx`: add `{ href: '/harvest', label: 'Харвестер', icon: <some lucide icon not already used, e.g. Sparkles or DownloadCloud> }` to `adminNav` (admin-only, same list as `/books`/`/authors`/`/genres`), **and** add `/harvest` to the existing `adminOnlyPaths` array (line ~99) so a moderator hitting `/harvest` gets the existing "AccessDenied" treatment instead of a broken/empty page — this is an easy-to-miss second place that needs the new route, don't forget it.

## Business Rules

- Input shape decision: the admin form sends `author_qids` as a **JSON string array**, not a raw comma/newline-separated string, to the backend. Splitting/trimming/deduping of the admin's free-text entry happens client-side (in `harvest-view.tsx`) before the API call; the backend does no string-splitting of its own and passes the array straight to `bookdata.BuildAuthorListQuery`, which already validates format (`^Q[0-9]+$`) and dedupes. This avoids adding a second, backend-side parsing function that would have to stay in sync with the CLI's `splitAuthorQIDs`/`parseAuthorQIDsFile` (`backend/cmd/harvest/main.go`) — those stay CLI-only and untouched.
- `limit` behavior is unchanged from the CLI: `<= 0` (or omitted) defaults to 500; a requested value above 5000 is silently clamped to 5000, not rejected — see `bookdata.BuildAuthorListQuery`'s existing doc comment. The admin form does not need its own clamping logic.
- A harvest job triggered via this endpoint is indistinguishable, once enqueued, from one triggered via `cmd/harvest -authors` — same task type, same queue, same worker, same idempotency/`is_hidden`/review-queue behavior. This endpoint is purely an alternate, curated-only front door onto the same pipeline.
- "Recent jobs" shown by `GET /jobs` is bounded by asynq's own retained state (see Scope boundaries) — a job that has aged out of Redis (past `Retention(24h)` for completed/archived tasks) simply stops appearing; there is no separate app-level history.
- Authorization: identical to every other state-changing admin action — `RoleAdmin` required (moderators get `403`); `POST /harvest` additionally requires a bearer token (not cookie-only), matching every other state-changing admin route. `GET` endpoints require `RoleAdmin` + existing cookie-or-bearer read auth, matching `books.GET`.

## UX

- New standalone admin page at `/harvest` (not an extension of an existing view), linked from the sidebar nav (admin-only).
- Single-column form (QIDs textarea + limit input + submit) above a jobs table, matching the overall layout rhythm of `book-import-view.tsx` (form card, then results below).
- Status is shown via `Badge` + a plain-text result/error summary, refreshed by a 5-second poll — no toast/websocket/real-time push.
- A "К очереди на проверку" link appears next to any completed job that actually created books, pointing at the existing `pending_review` filter (`docs/features/harvest-review-queue.md`) — this is the hand-off point from "I triggered a harvest" to "I'm reviewing what it made," and this contract does not duplicate or re-implement that review UI.

## API

`POST /api/v1/admin/harvest`
- Request: `{"author_qids": ["Q7200", "Q1183296"], "limit": 500}` (`limit` optional).
- `201`: `{"task_id": "...", "queue": "harvest", "state": "pending"}`.
- `400`: malformed/empty `author_qids` (message from `bookdata.ErrInvalidQuery`).
- `401`/`403`: auth/role failures, standard shape.
- `502`: enqueue/Redis failure (decision made, see Acceptance Criteria).

`GET /api/v1/admin/harvest/jobs`
- `200`: `{"items": [harvestJobResponse, ...]}`, grouped by state (active, retry, pending, completed, archived), up to 50 per state.

`GET /api/v1/admin/harvest/jobs/:id`
- `200`: `harvestJobResponse`.
- `404`: task not found (aged out of Redis, wrong ID, or never existed).

No changes to any existing endpoint (`/admin/books`, `/admin/dashboard`, etc.).

## Database

None. No new migration, no new table — `asynq`'s Redis-backed task/queue state is the source of truth for job status, per `async-harvest-queue.md`'s original decision (not duplicated in Postgres here).

## Frontend

- `packages/api-client/src/index.ts` — new types + `api.admin.harvest` (see Acceptance Criteria).
- `apps/admin/src/entities/harvest/api/index.ts` — new, thin re-export.
- `apps/admin/src/views/harvest/ui/harvest-view.tsx` — new.
- `apps/admin/src/app/harvest/page.tsx` — new.
- `apps/admin/src/widgets/admin-shell/ui/shell.tsx` — nav item + `adminOnlyPaths` entry (existing file, two small edits).

## Existing Patterns

- `backend/cmd/harvest/main.go` — `buildQuery`, `bookdata.BuildAuthorListQuery` usage, `harvestservice.NewHarvestTask`, `asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisURL})` construction — mirror exactly for the enqueue path; do not invent a second way to build the task or a second client-construction pattern.
- `backend/internal/modules/harvest/service/handler.go` — `TaskTypeHarvestWikidataQuery`, `QueueName`, `HarvestResult`/`HarvestRecordError` — reuse directly for decoding `TaskInfo.Result` in the new `GET /jobs` endpoints; do not redefine these shapes.
- `backend/internal/modules/admin/transport/http/handler.go` — `Service` interface + `Handler{service Service}` + swaggo doc-comment style — mirror the "handler depends on a small interface, not a concrete struct" shape for the new `harvest/transport/http` package's `TaskEnqueuer`/`TaskInspector` interfaces.
- `backend/internal/transport/http/router.go` — `RegisterAdminRoutes`'s existing `admin.Group(...)` / `adminRole` / `requireBearer` wiring for `books`/`authors`/`genres`/`reviews` — add a `harvest` group the same way.
- `apps/admin/src/views/book-import/ui/book-import-view.tsx` — form + `useMutation` + local `error`/report state + `Table` for a report list — closest existing analog for the new trigger form + jobs table.
- `apps/admin/src/views/dashboard/ui/dashboard-view.tsx` — `useQuery` + `@tanstack/react-query` conventions already in this codebase (also relevant: `refetchInterval` is not used anywhere yet in this repo — this is the first polling consumer; keep the option minimal, no need for a shared polling hook/abstraction for just one usage).
- `apps/admin/src/entities/book/api/index.ts` — thin `adminBooksApi` re-export pattern — mirror for `adminHarvestApi`.
- `apps/admin/src/widgets/admin-shell/ui/shell.tsx` — `adminNav` array + `adminOnlyPaths` array — both need a new entry.

## Tests

- Backend, `backend/internal/modules/harvest/transport/http/handler_test.go` (new): table-driven, hand-rolled fake `TaskEnqueuer`/`TaskInspector` (no mocking library, per repo convention). Cover: valid `author_qids` → `201` with `task_id`/`queue`/`state`, fake enqueuer receives a task built from the expected query; empty/malformed QID → `400` with the `bookdata.ErrInvalidQuery` message surfaced; enqueue error from the fake → the chosen error status (`502`/`500`); `GET /jobs` merges fake per-state list results in the documented order, capped at the documented page size; `GET /jobs/:id` found → `200` with mapped fields, `completed` state with a valid JSON `Result` → `result` populated, `completed` state with malformed `Result` bytes → `result` omitted, not a `500`; not-found (`asynq.ErrTaskNotFound` from the fake) → `404`.
- Backend: extend `backend/internal/transport/http/router.go`'s tests (if any exist at that level) or add a minimal smoke test that `RegisterAdminRoutes` wires the new group without panicking, given the router-level tests convention in this repo (check `router_test.go` if present before adding — reuse existing harness rather than inventing one).
- `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`.
- Frontend, `apps/admin/src/views/harvest/ui/harvest-view.test.tsx` (new, mirrors `book-import-view.test.tsx`'s conventions if that file exists — check first): form renders and requires at least one QID before submitting; comma/newline parsing produces the expected `author_qids` array passed to the mutation; successful trigger refetches/shows the jobs list; a `400` `ApiError` surfaces its message; jobs table renders state badges and, for a completed job with `created > 0`, renders the pending-review link.
- `apps/admin/src/widgets/admin-shell/ui/shell.tsx` — if `shell.test.tsx`-equivalent coverage exists for nav/`adminOnlyPaths`, extend it for `/harvest`; otherwise note as manual verification (consistent with `harvest-review-queue.md`'s own stated testing bar for thin frontend coverage areas).
- Admin app type-check/lint clean; no requirement to run the full monorepo test suite (per `docs/features/admin.md`'s existing stated bar).

## Documentation

- Mark this file `Status: implemented` when done.
- `docs/ai/features/catalog-data-import/business-analysis.md`: add a short status note near Решение 4 / R-004 / FR-010 stating that the "CLI-only, no admin HTTP trigger" decision has been superseded by this contract for the *triggering* concern specifically (curated author-QID input only, via the existing async queue — not a reopening of "avoid synchronous external-API calls in an HTTP request", which this contract does not violate). One or two lines, not a rewrite, matching how `harvest-review-queue.md` updated OQ-005.
- `cmd/harvest -h` / file header comment: no change needed — the CLI is untouched.
- No `docs/ROADMAP.md` change required (no existing "harvest" placeholder found there; consistent with the prior two increments' own documented decision not to add one).

## Risks

- **R-004 residual risk is now smaller but not zero for the new path.** The admin HTTP endpoint *does* have RoleAdmin + bearer auth (unlike the CLI, which relies on server/ops access as its implicit authorization), so triggering via this endpoint is actually *better* audited than the CLI was — but nothing in this contract adds an audit log entry (who triggered which job, when) beyond what's implicit in asynq's own task metadata (no "triggered by user X" field on `HarvestPayload`, since that would touch the payload shape this contract explicitly leaves unchanged). If per-trigger attribution is wanted later, it's a small follow-up (e.g. logging triggering-admin-ID at the HTTP layer, not threading it into the task itself) — flagged, not built here.
- **`GET /jobs`' per-state, capped, non-chronologically-merged listing is a real UX limitation**, not just an implementation detail: an admin can't easily tell "what's the single most recent thing that happened" across states without scanning multiple groups. Acceptable for this MVP tool's expected usage (occasional, low-volume triggering by a small number of admins) — revisit with a proper aggregation (e.g. sort by whatever timestamp each state does expose, accepting the type is state-dependent) if usage grows.
- **Two authorization surfaces for the same underlying action now exist** (`cmd/harvest` CLI with ops/server-access-as-authorization, and this admin-role-gated HTTP endpoint) — not a defect, but worth noting: revoking a compromised admin's HTTP access does not revoke server/ops access to the CLI, and vice versa. Pre-existing condition this contract doesn't change, just makes more visible by adding the second surface.
- **5-second fixed polling** with no pause when the browser tab is backgrounded/inactive is a minor, accepted inefficiency for this low-traffic internal tool — not worth a visibility-API-based pause/resume mechanism for this increment.
- **No hard ceiling on concurrent triggers from the admin UI**: nothing stops two admins from independently triggering overlapping author-QID harvests around the same time. The worker's existing queue concurrency=1 (see `async-harvest-queue.md`) means they'd simply run sequentially, not concurrently corrupt anything — and the existing `(external_source, external_id)` idempotency check means a genuinely duplicate QID harvested twice mostly just re-skips already-created books. Not mitigated further here (e.g. no "is there already a pending/active job?" pre-check before allowing a new trigger) — flag as a follow-up only if it proves a real operational annoyance.
