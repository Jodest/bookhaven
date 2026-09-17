# Async Harvest Queue

Status: implemented

This is the async-infrastructure increment for catalog harvesting. It builds the Redis-backed job queue, worker process, and harvest job handler described in `docs/ai/features/catalog-data-import/business-analysis.md` (Decision 2 option C, previously deferred — now explicitly commissioned). It supersedes the earlier plan of a purely synchronous, blocking `cmd/harvest` CLI (business analysis Decision 2 option A) with a queueable design from the start, per explicit product decision. Treat the business analysis's source selection, licensing, and author/genre-matching decisions as settled — this contract only covers the async plumbing.

## Goal

Make catalog harvesting from Wikidata/Open Library queueable and asynchronously trackable: a caller enqueues a harvest job (a SPARQL query) without blocking on an HTTP request or needing direct server/DB access at execution time, and can later check what happened via asynq's own task/result inspection.

## Scope boundaries (read first)

**In scope:**
1. Add `github.com/hibiken/asynq` as a dependency, wired to the existing `RedisURL` config.
2. A harvest job handler that calls `bookdata.WikidataClient.Query`, optionally enriches via `bookdata.OpenLibraryClient`, resolves/creates authors and genres by name, and creates books via `admin.Service.CreateBook` — always with `IsHidden = true` (OQ-002 default: crowdsourced source, human review before publish; not caller-configurable in this increment).
3. `backend/cmd/worker` — new binary running the asynq server, registering the harvest handler.
4. `backend/cmd/harvest` becomes a thin CLI that **enqueues** a job (reads a SPARQL query from a flag/file, connects to Redis, enqueues, prints the task ID). It no longer runs the harvest inline.
5. Job status/result visibility via asynq's built-in `Inspector` + per-task `ResultWriter` — no new DB table.
6. Migration: nullable `books.external_source` / `books.external_id` + partial unique index, for idempotent re-harvest.
7. `docker-compose.yml` / `docker-compose.prod.yml`: new `worker` service (same backend image/build, different command/entrypoint).

**Explicitly out of scope (do not build, do not let scope creep in):**
- Any admin HTTP endpoint or admin UI to trigger a harvest job or view its progress. Reachable in this increment only via the `cmd/harvest` CLI (ops/server access) and asynq's own inspection tooling (`asynq.Inspector`, or the community `asynqmon` CLI/web UI run ad hoc against the same Redis — not something this contract wires into the app).
- `GET /api/v1/admin/books/lookup` single-lookup endpoint (separate, previously-scoped increment in the business analysis — not this one).
- Author/genre fuzzy matching, CSV/file-based bulk import, Google Books/LitRes/Z39.50.
- Any change to public catalog API or existing admin book CRUD HTTP endpoints.
- `external_source`/`external_id` on `authors` (business analysis flagged as "if needed" — not needed for this increment; author dedup continues to rely on the existing `slugify`-derived unique `authors.slug`, see Business Rules).
- CI workflow changes: `ci.yml` already runs `go build ./...`, `go vet ./...`, `go test ./...` across the whole backend module, which covers the new packages/binaries automatically. `deploy.yml` builds a single backend Docker image already used for both `api` and `migrate`; the same image will now also contain `worker` and `harvest` — no new build-and-push job needed, only a `Dockerfile` change (see Frontend/Deployment section below). If this assumption turns out wrong once the developer is in the code, flag it back rather than silently expanding into `devops` territory.

## User Stories

- As an ops/developer operator, I can enqueue a Wikidata harvest job via a CLI command and immediately get back a task ID, without waiting for the harvest to finish.
- As an ops/developer operator, I can check a previously enqueued job's state (pending/active/completed/failed) and, once completed, its result summary (created/skipped/errors) via `asynq.Inspector` (or an ad hoc `asynqmon` instance against the same Redis) — without SSHing into a running process or tailing logs.
- As the system, a harvest job failure (e.g. Wikidata 5xx) is retried automatically with backoff instead of silently losing the job.
- As the system, re-running the same harvest job (retry, or manual re-enqueue of the same query) does not create duplicate books.

## Acceptance Criteria

- [ ] `github.com/hibiken/asynq` is a direct dependency in `backend/go.mod`/`go.sum` (`go get` + `go mod tidy`).
- [ ] `backend/cmd/worker/main.go` connects to Postgres (gorm, same pattern as `cmd/api`) and Redis (`asynq.RedisClientOpt{Addr: cfg.RedisURL}`), builds the harvest handler with real `WikidataClient`/`OpenLibraryClient`/admin repository+service, registers it on an `asynq.ServeMux` under a stable task type constant, and runs `asynq.Server` with graceful shutdown on SIGINT/SIGTERM (mirrors `cmd/api`'s shutdown pattern).
- [ ] `backend/cmd/harvest/main.go` is rewritten as a thin enqueue-only CLI: accepts a SPARQL query (`-query` flag or `-query-file` flag reading a `.rq`/`.txt` file), builds an `asynq.Client` from `cfg.RedisURL`, enqueues one `harvest:wikidata_query` task, prints the resulting task ID and queue name, exits 0. It performs no DB or bookdata-client calls itself.
- [ ] Harvest job payload is a small JSON struct (see API section) decoded inside the handler; malformed payload is a non-retryable failure (`asynq.SkipRetry`).
- [ ] Handler behavior per record returned by `WikidataClient.Query`: skip (idempotent, not an error) if a book with the same `(external_source, external_id)` already exists; otherwise optionally enrich via `OpenLibraryClient.FindByISBN`/`FindByTitleAuthor` for missing `CoverURL`/`Description`/`PageCount`; resolve-or-create authors/genres by name; call `admin.Service.CreateBook` with `IsHidden: true` and the resolved IDs plus `ExternalSource`/`ExternalID`.
- [ ] A single record's failure (validation error, DB error) does not abort the rest of the job — per-record errors are collected into the result summary (partial success, matching the business analysis's FR-004 spirit even though this is the queue increment, not the CSV-import increment).
- [ ] The handler writes a JSON result summary via `task.ResultWriter().Write(...)` before returning (see API section for shape), retrievable later via `Inspector.GetTaskInfo`.
- [ ] Migration `000009_books_external_source` adds nullable `books.external_source VARCHAR(50)`, `books.external_id VARCHAR(255)`, and a partial unique index `(external_source, external_id) WHERE external_source IS NOT NULL`; `.down.sql` reverses it.
- [ ] `catalogDomain.Book` gains `ExternalSource *string` / `ExternalID *string` fields (gorm-mapped); `admin/service.BookInput` gains the same two fields, threaded through `CreateBook` (and, for symmetry/no special-casing, `UpdateBook`, left `nil`/unused by existing callers).
- [ ] A new admin-service lookup, `Service.FindBookByExternalID(ctx, source, id string) (*catalogDomain.Book, error)`, returns `ErrBookNotFound` (existing sentinel) when absent — used by the handler for the idempotency skip-check ahead of `CreateBook`.
- [ ] A new admin-service resolve-or-create helper (exact placement — new file in `admin/service`, see Existing Patterns) resolves author/genre names to IDs, creating missing ones via the existing `CreateAuthor`/`CreateGenre` + `slugify`, reusing `GetAuthorBySlug`/`GetGenreBySlug` for the "already exists" check (normalized-name matching = slug matching, per business analysis Decision 3.a).
- [ ] `docker-compose.yml` gets a `worker` service: same backend build context/target as `backend`, `command` overridden to run `cmd/worker` (e.g. `["go", "run", "./cmd/worker"]`), same `DATABASE_URL`/`REDIS_URL` env, `depends_on` postgres+redis healthy.
- [ ] `docker-compose.prod.yml` gets a `worker` service: same `${BACKEND_IMAGE}:${IMAGE_TAG}` image, `entrypoint: ["/app/worker"]` override, same env block as `backend` (minus HTTP-only vars like CORS), `depends_on` postgres+redis healthy, `networks: [internal]` (no exposed ports — it's not an HTTP service).
- [ ] `backend/Dockerfile` builder stage additionally builds `/app/bin/worker` (from `./cmd/worker`) and `/app/bin/harvest` (from `./cmd/harvest`); production stage additionally `COPY`s both into `/app/`.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`.
- [ ] Handler's core orchestration logic (record → book-creation decision) is unit-tested without a real Redis connection and without real network calls (fakes for the Wikidata/OpenLibrary query interfaces and the admin repository, per existing repo test conventions).

## Business Rules

- Books created by the harvest handler are **always** `is_hidden = true` (draft), regardless of payload — this is a hardcoded business rule for this increment (OQ-002), not a per-job option. Publishing is a manual admin action via the existing book edit UI, out of scope here.
- Idempotency key is `(books.external_source, books.external_id)`, source-scoped: `external_source = "wikidata"`, `external_id = ` the Wikidata QID (`SourceID` on `ExternalBookRecord`, e.g. `"Q170583"`). A record already present is skipped, not updated — harvesting never edits existing books (matches business analysis "Out of scope: updating existing books via re-harvest").
- Author/genre matching reuses the existing slug uniqueness mechanism: `slugify(name)` is the normalized-name key. If `GetAuthorBySlug`/`GetGenreBySlug` finds a match, reuse its ID; otherwise create via the existing `CreateAuthor`/`CreateGenre` service methods (which themselves call `slugify`). This is Decision 3.a from the business analysis (exact normalized-name match, autocreate on miss, no fuzzy matching) implemented via existing infrastructure rather than a new comparison method.
- Enrichment via Open Library only fills fields Wikidata left empty (`CoverURL`, `Description`, `PageCount`, and `ISBN` if absent) — Wikidata's own values are never overwritten by Open Library's. This is the "who wins" merge policy the `bookdata` package explicitly deferred to this increment.
- ISBN checksum validation stays owned by `admin/service.isValidISBN`, invoked inside `CreateBook` exactly as today — the handler does not pre-validate or strip invalid ISBNs itself; a record with a structurally-normalized-but-checksum-invalid ISBN fails that record's `CreateBook` call and is recorded as a per-record error (not an entire-job failure).
- Concurrency: the harvest task type is processed on a dedicated asynq queue (e.g. `"harvest"`) configured with **concurrency 1** on the worker server for that queue. Reason: `bookdata.WikidataClient`/`OpenLibraryClient` throttle *per client instance*; if the worker ran multiple harvest tasks concurrently (each constructing its own client instances), outbound request rate to the external services could multiply beyond the ~1 req/sec/client design intent. Serializing harvest-queue processing keeps the existing throttling guarantee true without redesigning `bookdata`. Other asynq queues (if any exist later) are unaffected.
- Task retry/timeout: enqueue with `asynq.MaxRetry(3)`, `asynq.Timeout(30 * time.Minute)` (a bulk SPARQL query plus per-record enrichment at ~1 req/sec can legitimately take minutes for a few dozen records), `asynq.Retention(24 * time.Hour)` (so the result stays inspectable for a day after completion — asynq drops results at the default TTL otherwise).

## UX

None — no end-user- or admin-UI-facing surface in this increment. The only "interface" is the `cmd/harvest` CLI (ops/developer, terminal) and asynq's own inspection tooling (`asynq.Inspector` used from Go, or `asynqmon` run ad hoc — not wired into this repo).

## API

No HTTP API changes. Internal contracts:

**Task type**: `harvest:wikidata_query` (define as an exported `const TaskTypeHarvestWikidataQuery = "harvest:wikidata_query"` alongside the handler, imported by both `cmd/harvest` and `cmd/worker` so they can't drift).

**Payload** (JSON, keep intentionally minimal per the request's own guidance — do not add a multi-source job-type system):
```go
type HarvestPayload struct {
    SPARQLQuery string `json:"sparql_query"`
}
```

**Result** (JSON, written via `task.ResultWriter().Write(...)` before the handler returns):
```go
type HarvestResult struct {
    Processed int                 `json:"processed"`         // records returned by Query
    Created   int                 `json:"created"`
    Skipped   int                 `json:"skipped_existing"`  // already imported, idempotent skip
    Errors    []HarvestRecordError `json:"errors"`
}

type HarvestRecordError struct {
    Index   int    `json:"index"`   // position in the Query result slice
    Title   string `json:"title"`   // best-effort, for readability
    Message string `json:"message"`
}
```

A caller retrieves this via `asynq.Inspector.GetTaskInfo(queue, taskID)` → `TaskInfo.Result` (only populated once `TaskInfo.State == asynq.TaskStateCompleted`); `TaskInfo.State` itself answers "what happened" for pending/active/retrying/archived jobs without needing the result payload.

## Database

New migration `backend/migrations/000009_books_external_source.up.sql`:
```sql
ALTER TABLE books ADD COLUMN external_source VARCHAR(50);
ALTER TABLE books ADD COLUMN external_id VARCHAR(255);
CREATE UNIQUE INDEX idx_books_external_source_id ON books(external_source, external_id) WHERE external_source IS NOT NULL;
```
`.down.sql`:
```sql
DROP INDEX IF EXISTS idx_books_external_source_id;
ALTER TABLE books DROP COLUMN IF EXISTS external_id;
ALTER TABLE books DROP COLUMN IF EXISTS external_source;
```
Confirmed via `backend/migrations/` listing: no prior migration added these columns (`bookdata-clients` increment explicitly deferred it); next free number is `000009` (last existing is `000008_reading_challenges`).

`catalogDomain.Book` (in `backend/internal/modules/catalog/domain`) needs the two new fields added with gorm tags matching existing nullable-string field conventions in that struct (mirror how `Description`/`CoverURL`/`ISBN` are declared — read the struct before editing, don't guess the tag style).

No changes to `authors`/`genres` schema in this increment (see Scope boundaries).

## Frontend

None.

## Backend / Deployment wiring

- `backend/Dockerfile`: builder stage add two `go build` lines (`./cmd/worker` → `/app/bin/worker`, `./cmd/harvest` → `/app/bin/harvest`); production stage add two `COPY --from=builder` lines. `ENTRYPOINT` stays `/app/api` (unchanged) — `worker`/`harvest` are invoked via explicit `entrypoint:`/`command:` overrides in compose, the same pattern already used for `/app/migrate` in `.github/workflows/deploy.yml`.
- `docker-compose.yml` (dev): new `worker` service, same `build:` block as `backend` (context `./backend`, dockerfile `Dockerfile`, target `dev`), `command: ["go", "run", "./cmd/worker"]`, same volumes as `backend` (`./backend:/app`, `go-cache:/go/pkg/mod`) so it picks up local edits like `backend` does, same `DATABASE_URL`/`REDIS_URL` env, `depends_on` postgres+redis `service_healthy`. No `ports:` (not an HTTP service).
- `docker-compose.prod.yml`: new `worker` service, `image: ${BACKEND_IMAGE}:${IMAGE_TAG:-latest}` (reuse the already-built backend image — no new build-and-push job in `deploy.yml`), `entrypoint: ["/app/worker"]`, env subset of `backend`'s block (`DATABASE_URL`, `REDIS_URL`, `LOG_LEVEL`; omit HTTP-only vars: `APP_PORT`, `CORS_ALLOWED_ORIGINS`, OAuth client vars, `JWT_*` — the worker doesn't serve HTTP or authenticate users), `depends_on` postgres+redis `service_healthy`, `networks: [internal]`, no `ports:`.
- No `.github/workflows/*.yml` changes expected (see Scope boundaries) — flag back if the developer finds this assumption wrong (e.g. if `deploy.yml`'s health-check-and-rollback step needs to also verify the worker container started cleanly, that's a small addition worth a `devops`-labeled follow-up, not a blocker for this contract).
- Redis has no auth/password in either compose file today (pre-existing, not introduced by this feature) and is not port-exposed in `docker-compose.prod.yml` (`internal` network only, no `ports:` — unlike the dev compose which does expose `REDIS_PORT`). Acceptable as-is; note it as a pre-existing condition this feature now actually depends on (previously Redis ran unused).

## Existing Patterns

- `backend/internal/pkg/bookdata/*.go` — `WikidataClient.Query`, `OpenLibraryClient.FindByISBN`/`FindByTitleAuthor`, `ExternalBookRecord`, sentinel errors (`ErrNotFound`, `ErrRateLimited`, `ErrSourceUnavailable`, `ErrInvalidQuery`). Do not modify this package; consume it as-is. Do not add throttling logic in the handler — the clients already throttle themselves per-instance (see Business Rules re: queue concurrency = 1).
- `backend/internal/modules/admin/service/service.go` — `BookInput`, `CreateBook`, `slugify`, `isValidISBN`, `GetAuthorBySlug`/`GetGenreBySlug`/`CreateAuthor`/`CreateGenre`. Reuse directly; extend `BookInput` with `ExternalSource`/`ExternalID` fields (additive, does not break existing callers who leave them `nil`).
- `backend/internal/modules/admin/repository/repository.go` — `Repository` interface + `GormRepository`. Add `GetBookByExternalID(ctx, source, id string) (*catalogDomain.Book, error)` (mirrors `GetAuthorBySlug`'s shape: `First(...).Error` passthrough, including `gorm.ErrRecordNotFound`).
- New resolve-or-create helper: put it in `backend/internal/modules/admin/service/` (new file, e.g. `harvest_resolve.go`) as exported `Service` methods (e.g. `ResolveOrCreateAuthorIDsByNames`, `ResolveOrCreateGenreIDsByNames`), **not** in a new `harvest`-owned package — it only needs repository methods the admin module already owns (`GetAuthorBySlug`, `CreateAuthor`, etc.), and keeping it in `admin/service` avoids a second place that knows how author/genre creation validation works. The harvest handler package imports `adminservice`, calls these plus `CreateBook`/`FindBookByExternalID` — it does not talk to the admin repository directly.
- `backend/cmd/seed/main.go` — `FirstOrCreate`-by-natural-key idempotency pattern (conceptual precedent for "safe to re-run"), direct `covers.openlibrary.org` hotlink pattern (already what `bookdata.CoverURL` produces).
- `backend/cmd/api/main.go` — service/repo wiring order, `zap` logger setup, SIGINT/SIGTERM graceful shutdown pattern to mirror in `cmd/worker`.
- New handler code location: `backend/internal/modules/harvest/service/handler.go` (new module, service-only — no `domain`/`repository`/`transport` subpackages needed since it introduces no new persistent entity and no HTTP route; it orchestrates existing modules). Exposes the asynq `HandlerFunc`-compatible entry point plus a small unexported/testable core function (e.g. `processRecord(ctx, rec bookdata.ExternalBookRecord, wikidata WikidataQuerier, enrich bookdata.Client, admin *adminservice.Service) error`) so tests don't need a real `asynq.Task`/Redis.
- `backend/internal/modules/admin/service/service_test.go` and `backend/internal/pkg/bookdata/*_test.go` — repo-wide test convention (stdlib `testing`, table-driven, hand-rolled fakes, no mocking library). Follow it for the new handler tests.

## Tests

- **Handler core logic** (`backend/internal/modules/harvest/service/handler_test.go`): table-driven, using a fake `WikidataQuerier` (returns a canned `[]ExternalBookRecord`), a fake enrichment `bookdata.Client` (canned `FindByISBN`/`FindByTitleAuthor` responses/errors), and a fake admin repository (same hand-rolled-fake convention as `admin/service/service_test.go`) wired into a real `adminservice.Service`. Cover: record with no existing `external_id` → created hidden; record whose `external_id` already exists → skipped, not re-created; enrichment only fills empty fields, never overwrites Wikidata's; author/genre resolve-or-create hits `GetAuthorBySlug` first and only creates on miss; one bad record (e.g. invalid ISBN) doesn't stop processing of the rest, and shows up in `HarvestResult.Errors`; malformed JSON payload → handled as a non-retryable failure.
- **Payload/result (de)serialization**: a small test constructing a real `asynq.NewTask(TaskTypeHarvestWikidataQuery, payloadBytes)` and asserting the handler entry point decodes it correctly — this exercises the asynq `Task` type itself (no Redis needed to construct one) without needing a live queue.
- **`cmd/harvest` CLI**: unit-testable to the extent its argument parsing/payload construction is factored out of `main()` into a small testable function; do not attempt to test the actual `asynq.Client.Enqueue` call against a real Redis in this increment (see below).
- **Integration test against real/miniredis**: **not required for this increment.** `miniredis` does not reliably support all Redis commands/behaviors `asynq` depends on (blocking list ops, scripting) and asynq itself doesn't officially document miniredis compatibility — attempting it risks a flaky, high-maintenance test for low marginal confidence. Instead: document a manual smoke-test procedure (enqueue via `cmd/harvest` against the `docker-compose.yml` Redis, confirm `cmd/worker` picks it up and a book appears with `is_hidden = true`) as part of this feature's PR description / `Documentation` below, not as an automated test. Revisit automated integration coverage if a real Redis test instance becomes part of CI later (would need a `redis:7-alpine` service container added to `ci.yml`'s backend job — flag that as a `devops`-adjacent follow-up if it's ever wanted, not part of this contract).
- `go build ./...`, `go vet ./...`, `go test ./...` clean, matching existing CI (`ci.yml` already runs these against the whole `backend/` module — no workflow edit needed for these binaries/packages to be covered).

## Documentation

- `docs/features/async-harvest-queue.md` (this file) — mark `Status: implemented` when done, per repo convention (see `docs/features/bookdata-clients.md`).
- Package-level doc comment on the new `harvest/service` package (mirror `bookdata.go`'s style): summarize the task type, payload/result shape, the concurrency=1 rationale, and the hardcoded `is_hidden=true` rule.
- `cmd/harvest -h` output (flag usage) should be self-explanatory enough to serve as the operator-facing "documentation" for this CLI — no separate ops runbook required for this increment, but add a short comment block at the top of `cmd/harvest/main.go` with a one-line manual smoke-test recipe (enqueue → check worker logs → check via Inspector) for the next person touching this code.
- No `docs/ROADMAP.md` change strictly required (this isn't itself a numbered roadmap item — it fulfills a prerequisite named in the `catalog-data-import` business analysis); add a one-line pointer there only if the developer finds an obvious existing "harvest infra" placeholder to update — don't invent a new roadmap section for it.

## Manual Smoke Test

Automated integration coverage against a real queue is explicitly out of scope for this increment (see Tests/Risks). To verify the end-to-end path manually:

1. `docker compose up postgres redis backend worker` (the `worker` service builds from the same `backend` dev image/target and runs `go run ./cmd/worker`).
2. Run the migration if not already applied: `docker compose exec backend go run ./cmd/migrate up` (or however migrations are normally applied in your dev setup).
3. From a shell with the backend module available, enqueue a small query, e.g.:
   `cd backend && REDIS_URL=localhost:6379 go run ./cmd/harvest -query 'SELECT ?item ?itemLabel WHERE { ?item wdt:P31 wd:Q7725634 . ?item rdfs:label ?itemLabel . FILTER(LANG(?itemLabel)="ru") } LIMIT 3'`
4. Note the printed `task id=... queue=harvest`.
5. Watch `docker compose logs -f worker` for processing.
6. Confirm results via `asynq.Inspector` (small Go snippet or the `asynqmon` CLI/web UI run ad hoc against the same Redis) using the printed queue name + task ID, and/or check the admin book list — a completed job should have created book(s) with `is_hidden = true`.

## Implementation Notes

- `asynq` was pinned to `v0.25.1` rather than the latest `v0.26.0`: the latest release bumps its own `go.mod` `go` directive to `1.24.0`, which would have forced this module's `go` directive up from `1.23` (CI's Go toolchain) for no functional benefit. `v0.25.1` requires only `go 1.22`, so `backend/go.mod`'s `go 1.23` directive is unaffected. Revisit this pin if a future asynq feature/fix is needed that only ships in a newer release.
- `backend/cmd/harvest` did not exist yet on this branch (it was named as a "becomes a thin CLI" rewrite target in this contract's framing, but the earlier synchronous-CLI increment it refers to was never actually implemented) — it was created fresh as the enqueue-only CLI described here, not rewritten from a prior version.

## Risks

- **Duplicate-key ambiguity on retry**: `admin/service.isDuplicateKeyError` currently maps *any* Postgres `23505` to the generic `ErrDuplicateSlug`, regardless of which unique constraint fired (`books.slug` vs. the new `(external_source, external_id)` partial index). The handler mitigates this by proactively checking `FindBookByExternalID` *before* calling `CreateBook` (so the common idempotent-retry path is a clean "skip", not an error-path guess), but a genuine slug collision from an unrelated source and a same-request-retried external-id collision would still surface as the same error type if the pre-check ever races. Low impact (both cases are reasonably treated as "don't create a duplicate"); flagged for awareness, not blocking.
- **No automated integration test against a real queue** (see Tests) — behavior is verified at the unit level (handler logic) and the asynq `Task` type level, but the actual "enqueue → worker picks it up → processes → result inspectable" round trip is only manually smoke-tested per this contract. Acceptable for this increment's risk profile (internal ops tooling, not user-facing), but should be revisited if harvest jobs become a routine/critical operational path.
- **Concurrency=1 on the harvest queue** caps overall throughput to one job at a time; for the currently-anticipated usage pattern (occasional ops-triggered bulk harvests, not a high-frequency job type) this is an acceptable trade-off for correctness (not exceeding external API etiquette) over speed. If harvest volume grows, revisit by making `bookdata` client instances shared/injected singletons with their own internal rate limiter (already the design) rather than raising queue concurrency, since raising concurrency without a shared limiter would reintroduce the throttling-multiplication problem.
- **`external_source`/`external_id` not added to `authors`**: two harvested books whose author strings don't slugify identically (e.g. transliteration variance) will still create duplicate author rows, same known risk (R-002) the business analysis already accepted for MVP; this increment doesn't change that trade-off, only builds the queue around it.
- **`asynq` is a new, previously-unused dependency** — first real consumer of the already-provisioned-but-idle Redis service; verify `go mod tidy` doesn't pull in an unexpectedly large transitive dependency tree (asynq depends on `redis/go-redis/v9`) and that it's compatible with `go 1.23` (current `go.mod` version) before finalizing `go.sum`.
- **Wikidata SPARQL query correctness is still only unit-tested against fixtures** (inherited caveat from `bookdata-clients.md`) — actual harvest quality/volume against the live endpoint is unverified until a real ops-triggered smoke test happens; not a defect of this increment specifically.
