# Error Tracking (Sentry)

Status: implemented

Source: backlog proposal #253, ROADMAP "Error tracking (e.g. Sentry) for API, worker and web apps".

No OpenSpec domain's observable behavior changes: no endpoint, status code, response body, header, CSP or UI changes. This is an ops integration that is **off unless a DSN is configured**.

## Goal

Unhandled production errors (API 5xx and panics, worker tasks that finally fail, Next.js server-side render/route errors) reach a Sentry project instead of being visible only in container logs. With no DSN configured, every app behaves exactly as today.

## Scope

In scope (this contract):

- **A. Backend** — `cmd/api` and `cmd/worker`, with `github.com/getsentry/sentry-go` (core SDK only, no framework integration packages).
- **B. Next.js server side** — `apps/web` and `apps/admin`, with `@sentry/nextjs`, server (Node.js runtime) only.

Out of scope (follow-up, do not implement):

- **Browser-side reporting** (client SDK in the bundle). Reasons: there is no prod traffic yet; the browser SDK adds roughly 25-30 kB gzip to every first load; it needs either a CSP `connect-src` for the ingest host or a tunnel route through the app. When it is picked up later, prefer the SDK's `tunnelRoute` (same-origin POST, `connect-src 'self'` keeps working, no CSP change) over adding the ingest host to CSP. Record this decision in DEPLOYMENT.md.
- Performance tracing, profiling, session replay, source map upload, user feedback.
- Edge runtime (the `middleware.ts` of both apps).
- Other CLIs (`cmd/harvest`, `cmd/seed`, `cmd/similarity`, `cmd/migrate`, `cmd/mailtest`).
- Rewriting the ~100 handler sites that answer `c.JSON(500, ...)` to return the underlying error.

## User Stories

- As the operator, when the API answers 5xx or panics, I get one Sentry issue per route that tells me the method, route template, status and release, so I notice it without reading logs.
- As the operator, when a background task fails for good (after asynq retries), I get a Sentry issue with the task type and queue.
- As the operator, when a Next.js server render or route handler throws, I get a Sentry issue tagged with the app.
- As a developer or CI, I never need a Sentry account: without a DSN nothing is sent and nothing fails.

## Acceptance Criteria

- [ ] With `SENTRY_DSN` unset or empty in every process: no Sentry client is created, no network calls are made, startup logs contain one info line `error tracking disabled` (backend), CI (`go test`, frontend lint/type-check/test/build) is green without any Sentry secret.
- [ ] Backend api: a request ending with status >= 500 (handler wrote a 5xx JSON, returned a 5xx `*echo.HTTPError`, or returned a plain error that becomes 500) produces exactly one event; a panic in a handler produces one event and the client still gets the existing `Recover()` 500.
- [ ] Backend api: 4xx responses (400/401/403/404/409/422/429…) never produce events. `/health` and `/health/ready` (including readiness 503) never produce events.
- [ ] Worker: a task error is reported only when it is the **final** failure (retry count reached max retry, or the error wraps `asynq.SkipRetry`); intermediate failures that asynq will retry are not reported. Panics recovered by asynq follow the same rule. All asynq servers in `cmd/worker` have the error handler.
- [ ] Next.js web/admin: with `SENTRY_DSN` set at runtime, an exception thrown in a server component / route handler / server-side data fetch is reported from the Node.js runtime. `notFound()` / `redirect()` are not reported.
- [ ] No client-side Sentry SDK code is shipped: no `sentry.client.config.*` / `instrumentation-client.*`; the CI build's First Load JS sizes of web and admin grow by no more than ~2 kB per route compared with the previous `dev` build.
- [ ] CSP of web, admin and API is byte-for-byte unchanged (existing `security-headers` tests stay green without edits).
- [ ] No PII leaves the process (see Business Rules 5).
- [ ] Every event carries `environment`, `release` (when known) and a `component` tag (`api` / `worker` / `web` / `admin`).
- [ ] SDK versions are pinned exactly (Go: `go.mod`; JS: exact version, no `^`/`~`).
- [ ] An invalid DSN logs a warning and the process continues without reporting (never crashes, never blocks startup).
- [ ] DEPLOYMENT.md, `.env.example`, `docker-compose.prod.yml` document and pass the new variables.

## Business Rules

1. **Opt-in.** Reporting is enabled per process iff `SENTRY_DSN` is non-empty after trimming. `config.Load` never fails because of Sentry variables.
2. **What is an error (api).** Status >= 500 of a request, plus panics. The status is taken from the returned error when the handler returns one (`*echo.HTTPError` code, otherwise 500), else from `c.Response().Status`. Routes under `/health` are excluded. Event message: `HTTP <status> <METHOD> <route template>` (e.g. `HTTP 500 GET /api/v1/shelves/:id`) using `c.Path()`, never the raw URI (raw URIs contain IDs and query strings). Fingerprint `{method, route template, status}` so each failing route groups into one issue. When the handler returned a Go error, attach it as the exception; when it only wrote a 5xx JSON (the common case — the underlying error is lost), send the message event; this is accepted, the logs still hold the request line.
3. **What is an error (worker): final failure only.** Justification: harvest/Wikidata/Open Library/SMTP/model calls fail transiently and asynq retries them with backoff (mail: up to 8 retries); reporting each attempt would flood the free tier and train the operator to ignore the feed. Final = `retried >= maxRetry` (from `asynq.GetRetryCount` / `asynq.GetMaxRetry` on the handler context) or `errors.Is(err, asynq.SkipRetry)`. Tags: `task_type`, `queue`, `retried`, `max_retry`. **Never** attach the task payload (contains email addresses and bodies for `email:send`, user content for AI tasks). Fingerprint `{task_type, error-type}`; message `task <type> failed: <scrubbed error string>`.
4. **What is an error (Next.js).** Whatever `@sentry/nextjs` captures on the server for App Router render errors, route handlers and server-side unhandled rejections, minus the SDK's built-in ignore of `notFound`/`redirect`. `beforeSend` drops events whose original exception has a numeric `status`/`statusCode` in 400..499 (expected API 4xx surfaced during SSR).
5. **Privacy (all four processes).**
   - `send_default_pii` / `sendDefaultPii` = false (Go: `SendDefaultPII: false`; `@sentry/nextjs` 11 has no such option - PII collection is off by default there - so the Next side relies on `beforeSend`).
   - No breadcrumbs: backend uses no integrations that add them; Next sets `maxBreadcrumbs: 0` (fetch breadcrumbs carry URLs with query strings).
   - Backend never attaches the HTTP request to events (no headers, cookies, body, query, client IP) and never sets the user.
   - `beforeSend` (Go and JS) additionally: removes `request.cookies`, `request.headers`, `request.data`, `request.query_string`, `user`; redacts in the message and every exception value: email addresses → `[email]`, `Bearer <x>` → `Bearer [redacted]`, JWT-shaped strings (`eyJ…\.…\.…`) → `[jwt]`, `token=`/`password=`/`secret=` query/key-value values → `[redacted]`. Returning the event with redactions applied, never dropping it for that reason.
   - Server name: the SDK default (container hostname) is fine; no env dump, no `extra` with config.
6. **Sampling.** Errors `SampleRate` 1.0; tracing off (no `TracesSampleRate` / `tracesSampleRate` set, or 0). No profiling, no replay.
7. **Environment / release.** `environment` = `SENTRY_ENVIRONMENT`, else backend `APP_ENV`, Next `NODE_ENV`. `release` = `SENTRY_RELEASE` if set, else unset. In prod compose, `SENTRY_RELEASE: ${IMAGE_TAG:-}` — the 12-char git SHA the deploy workflow already writes to `.env`, so no build arg or image change is needed and the release matches the deployed image.
8. **Shutdown.** api and worker `Flush(2s)` on graceful shutdown (after `e.Shutdown` / `shutdownServers`). `logger.Fatal` paths are not flushed (accepted).

## Configuration

| Variable | Processes | Meaning |
|---|---|---|
| `SENTRY_DSN` | api, worker, web, admin (runtime) | Empty = disabled (default everywhere). |
| `SENTRY_ENVIRONMENT` | all (optional) | Overrides the environment tag. |
| `SENTRY_RELEASE` | all (optional) | Release tag; prod compose fills it from `IMAGE_TAG`. |

`docker-compose.prod.yml`: backend and worker get `SENTRY_DSN: ${SENTRY_DSN_BACKEND:-}`, web and admin get `SENTRY_DSN: ${SENTRY_DSN_FRONTEND:-}` (two Sentry projects: Go and Next.js; web and admin are told apart by the `component` tag); all four get `SENTRY_RELEASE: ${IMAGE_TAG:-}`. Not a `NEXT_PUBLIC_*` variable: the DSN is read at runtime on the server and never inlined into client bundles. The local `docker-compose.yml` is not changed (no DSN in dev).

## API

No endpoint changes. New Echo middleware only.

## Database

None.

## Backend design

- **`internal/config`**: add `SentryDSN`, `SentryEnvironment` (defaults to `Env`), `SentryRelease` (all trimmed strings). No validation.
- **`internal/pkg/errtrack`** (new, the only package importing `sentry-go`):
  - `Init(opts Options{DSN, Environment, Release, Component string}, logger) (Reporter, func(time.Duration))` — empty DSN → no-op `Reporter` and no-op flush, logs `error tracking disabled`; `sentry.Init` error → warning (without the DSN value), no-op reporter. Otherwise a `Reporter` backed by its own `*sentry.Client`/`*sentry.Hub` (not the global hub), `AttachStacktrace: true`, `SendDefaultPII: false`, `BeforeSend: scrub`, component tag on the scope; logs `error tracking enabled` with environment/release/component (never the DSN).
  - `Reporter` interface: `CaptureHTTP(ctx, HTTPFailure{Method, Route string; Status int; Err error})`, `CapturePanic(ctx, recovered any, method, route string)`, `CaptureTaskFailure(ctx, TaskFailure{Type, Queue string; Retried, MaxRetry int; Err error})`. Each capture clones the hub scope, sets tags + fingerprint, captures. The interface is the test seam for the middleware and the worker handler.
  - `scrub(*sentry.Event, *sentry.EventHint) *sentry.Event` and the redaction regexes, exported for tests only via an `_test` in the same package.
- **`internal/transport/http/middleware/error_reporting.go`**: `ErrorReporting(r errtrack.Reporter) echo.MiddlewareFunc` per Business Rule 2; on panic: `CapturePanic` then re-panic. Registered in `cmd/api/main.go` **directly after** `echomiddleware.Recover()` (Echo: first `Use` is outermost, so Recover still catches the re-panic and answers 500 as today). When the reporter is the no-op one, the middleware is a pass-through (or simply not registered — pick one, test it).
- **`cmd/worker/main.go`**: one `taskErrorHandler(r errtrack.Reporter) asynq.ErrorHandler` per Business Rule 3 (queue name from `asynq.GetQueueName(ctx)`). Set it on **every** `asynq.Config` — the `*ServerConfig` helpers get it via a small `withErrorHandler(cfg asynq.Config, h asynq.ErrorHandler) asynq.Config` wrapper applied at every `asynq.NewServer` call, including the inline email and similarity servers. Do not change queues, concurrency or retry options.
- **`cmd/api` / `cmd/worker`**: call `errtrack.Init` right after `config.Load`, `defer`/explicit flush on shutdown, component `api` / `worker`.
- Dependency: `github.com/getsentry/sentry-go` latest stable release at implementation time, exact version in `go.mod`, `go mod tidy`. No `sentry-go/echo` / `otel` subpackages.

## Frontend design (web and admin, identical)

- Dependency: `@sentry/nextjs` pinned **exactly** to the newest release that officially supports `next@14.2.x` and React 18 (check its peerDependencies; do not upgrade Next). Add to each app's `package.json`, update `pnpm-lock.yaml` with pnpm.
- `next.config.js`:
  - ~~`experimental.instrumentationHook: true`~~ — removed on the Next 15 upgrade (#594); `src/instrumentation.ts` is stable and needs no flag.
  - Export `withSentryConfig(nextConfig, { silent: true, telemetry: false, sourcemaps: { disable: true }, autoInstrumentMiddleware: false, disableLogger: true, widenClientFileUpload: false })` — no `org`, `project`, `authToken`, **no `tunnelRoute`**. The wrapper is applied always (prod images are built in CI without a DSN, and the DSN is read at runtime); it is what auto-instruments App Router server components / route handlers on Next 14.
  - `headers()` and `rewrites()` unchanged; `security-headers.js` unchanged.
- `src/instrumentation.ts`: `register()` imports `./sentry.server.config` only when `process.env.NEXT_RUNTIME === 'nodejs'` and `shouldInitSentry(process.env)` is true. No edge config file.
- `src/sentry.server.config.ts`: `Sentry.init({ dsn, environment, release, sendDefaultPii: false, maxBreadcrumbs: 0, beforeSend: scrubEvent, initialScope: { tags: { component: 'web' | 'admin' } } })`; no `tracesSampleRate`.
- `src/shared/lib/sentry-scrub.ts` (FSD `shared` layer; identical copy per app, like `security-headers.js`): pure `shouldInitSentry(env)`, `scrubEvent(event, hint)` (Business Rules 4-5). No `@sentry/*` runtime import in this file except types, so it is unit-testable.
- Must **not** add `sentry.client.config.*`, `instrumentation-client.*`, `global-error.tsx` Sentry calls, or anything under `NEXT_PUBLIC_SENTRY_*`.
- `Dockerfile.node`: no change expected (standalone output traces the dependency). If the standalone image lacks the package at runtime, fix in the Dockerfile and note it.

## Existing Patterns

- Opt-in feature flags with a logged "disabled" line: AI modules in `cmd/worker/main.go`, `cfg.AIModerationActive()`.
- Middleware location and style: `backend/internal/transport/http/middleware` (`SecurityHeaders()`).
- Per-app duplicated config module: `apps/web/security-headers.js` / `apps/admin/security-headers.js` (`docs/features/security-headers.md`).
- Worker server config helpers + table tests: `harvestServerConfig`, `authPurgeServerConfig` in `cmd/worker/main.go`.

## Tests

Backend (`go test ./...`):
- `internal/config`: `SENTRY_DSN`/`SENTRY_ENVIRONMENT`/`SENTRY_RELEASE` parsing, environment defaults to `APP_ENV`, whitespace-only DSN = empty.
- `internal/pkg/errtrack`: empty DSN → no-op reporter, captures do not panic, flush returns; malformed DSN → no-op + warning; with a fake DSN (`https://key@o0.ingest.invalid/0`) and a recording `sentry.Transport`: `CaptureHTTP` event has message, tags (method, route, status, component), fingerprint, no `Request`, no `User`; `CaptureTaskFailure` has task tags and no payload; `scrub` redacts email, `Bearer`, JWT, `token=`/`password=` in message and exception values and strips request/user. No real network.
- `internal/transport/http/middleware`: with a fake `Reporter` and `httptest`: handler writes 500 JSON → 1 capture with route template `/x/:id`; returns `echo.NewHTTPError(503)` → 1; returns `errors.New` → 1 (status 500); returns 400/401/403/404/409/429 (both written and returned) → 0; 200 → 0; panic with `Recover()` outermost → 1 capture and response 500; `/health/ready` 503 → 0.
- `cmd/worker`: `taskErrorHandler` with a fake reporter: retried < max → 0; retried == max → 1 with type/queue; `SkipRetry`-wrapped at retry 0 → 1; every server config produced in `main` has a non-nil `ErrorHandler` (test the config helpers via `withErrorHandler`).

Frontend (`vitest`, both apps):
- `sentry-scrub.test.ts`: `shouldInitSentry` false for undefined/empty/whitespace DSN, true otherwise; `scrubEvent` strips request cookies/headers/data/query_string and user, redacts email/Bearer/JWT/token in message and exception values, drops events whose original exception has status 404/401, keeps status 500 and plain errors.
- Existing `security-headers` tests must pass unchanged (proves CSP is untouched).
- No native `next build` (Windows wipes node_modules; typedRoutes is only checked by the CI build). The CI frontend build is the gate for the `withSentryConfig` wrapper; compare First Load JS sizes in the CI log against the last `dev` run and report them in the PR.

Manual (optional, only if a throwaway Sentry project is at hand; not required for merge): set `SENTRY_DSN` locally, hit a route forced to 500, confirm one event with no PII.

## Documentation

- `docs/DEPLOYMENT.md` → Monitoring: replace the "Errors (optional…)" bullet with an "Error tracking (Sentry)" subsection: opt-in, the variable table above, what is/isn't reported (5xx/panics, final task failures, Next server errors; no 4xx, no health, no retries), privacy rules, release = `IMAGE_TAG`, two projects (Go, Next.js), free-tier quota note, the browser-SDK follow-up decision (tunnel route preferred, CSP unchanged today). Add `SENTRY_DSN_BACKEND`/`SENTRY_DSN_FRONTEND` to the server `.env` instructions and a checklist line ("DSNs only in server `.env`").
- `.env.example` (root): commented `SENTRY_DSN_BACKEND=`, `SENTRY_DSN_FRONTEND=`, `# SENTRY_ENVIRONMENT=` with a pointer to DEPLOYMENT.md; state that dev leaves them empty.
- `docker-compose.prod.yml`: the env passes from "Configuration".
- `cmd/worker/main.go` package doc: one paragraph on final-failure reporting.
- `docs/ROADMAP.md`: tick the item (#253); add a new unchecked follow-up "Browser-side error reporting (Sentry tunnel route)".
- This file: `Status: implemented` when done.

## Risks

- **`withSentryConfig` on Next 14 + `typedRoutes` + `transpilePackages`.** It changes the webpack build; failures surface only in the CI frontend build. Mitigation: land group A (backend) first in its own commit; if the wrapper cannot be made green or pulls a client SDK into the bundle (size check above), stop and report — ship the backend part and move Next.js to a follow-up rather than hacking around the build.
- **Lost root causes for written-500s.** Most handlers write `c.JSON(500, …)` and drop the error, so API events show route + status but no stack/error text. Accepted for now; the per-route issue is the alert, the logs hold detail. A later pass can return errors from the noisiest handlers.
- **Free-tier quota during an outage.** A DB outage makes every route 5xx. Grouping by route keeps issues few, but event volume can hit the monthly quota; configure a spike-protection / rate limit in the Sentry project (documented), no client-side rate limiting.
- **PII through error strings.** Error messages can embed emails (mail task failures) or tokens. Mitigated by redaction regexes + no payload/request/user; regexes are best-effort — keep the "no payload" rule absolute.
- **Worker handlers that log and return nil** are never reported. Accepted; reporting covers what asynq sees as failed.
- **Edge middleware and browser errors** are not covered until the follow-up.
