# API request body limit and HTTP server timeouts

Status: implemented

Issue: #623 (backlog proposal). Out of scope: rate limiting of registration (#624), Caddy timeouts, worker (no HTTP server).

## Goal

Stop the api (`cmd/api`) from accepting unbounded request bodies and holding slow connections forever. Today only `POST /api/v1/admin/books/import` has a body limit (Echo `BodyLimit("2M")` on the route) and `e.Start` runs a default `http.Server` (no `ReadHeaderTimeout` / `ReadTimeout` / `WriteTimeout` / `IdleTimeout`), so public JSON endpoints (`/auth/register`, `/books/:id/reviews`, ...) take any body size and slowloris-style connections.

## User Stories

- As the operator, I want every API request body capped by default so a single request cannot make the api buffer or decode megabytes of JSON.
- As the operator, I want slow or idle connections dropped so they cannot exhaust the api.
- As an admin, I want file import, paste and bulk/sync admin operations to keep working exactly as today.

## Acceptance Criteria

- [ ] Every route has a default request body limit of **1 MiB** (1 048 576 bytes).
- [ ] `POST /api/v1/admin/books/import` keeps its **2 MiB** limit (larger than the default, not capped by it). Its behavior in `openspec/specs/catalog-import/book-file-import` (413 before parsing, 500-row 422) is unchanged.
- [ ] A body over the limit is rejected with **413** and the API's JSON error shape `{"error": "request body too large"}`, before the handler runs. The same applies when the body has no declared length (chunked): it must be a 413, not the handler's 400 `invalid request`.
- [ ] A body exactly at the limit is accepted, and the handler reads the whole body unchanged.
- [ ] The 413 response carries the normal CORS headers (so browsers can read the status) and is logged by the request logger.
- [ ] The api's `http.Server` has `ReadHeaderTimeout` 10s, `ReadTimeout` 60s, `WriteTimeout` 5m and `IdleTimeout` 120s by default. Each can be overridden by an env var (Go duration).
- [ ] Legitimate long requests still complete: synchronous Wikidata admin operations (up to 50 books/authors), the 500-row file import, Swagger UI, cover files.
- [ ] Graceful shutdown keeps working (SIGTERM, `e.Shutdown` with the existing 10s budget).
- [ ] Web and admin need no changes (see Frontend).

## Business Rules

- Default limit: 1 MiB. Rationale for the biggest legitimate JSON bodies:
  - curated paste preview `POST /admin/collections/:id/books/paste/preview`: at most 100 000 characters (`MaxPasteChars`). Worst case: Cyrillic text that the client JSON-escapes as `\uXXXX`, 6 bytes per character, about 600 KB. Plain UTF-8 is about 200 KB. Both fit under 1 MiB, so no override is needed. A test pins this (see Tests).
  - bulk add (500 UUIDs), bulk status, merge-batch, AI apply/dismiss (50 items), AI description generate (100 ids), sync/enrich (50 ids), reviews (10 000 runes), shelf descriptions, book descriptions: all are well below 100 KB.
- Override table (a route larger than the default): only `POST /api/v1/admin/books/import` gets 2 MiB. It is the only multipart upload in the API (`c.FormFile` is used only there). Any future upload route must add its own entry to the table. A route-level `BodyLimit` on top of the global one cannot raise the global limit, so the table is the only way to raise it.
- The body limit is a code constant, not an env var. Nothing needs tuning per deployment, and a constant keeps the override table honest.
- The limit applies to every method and every path, including unknown routes (404) and requests that should not have a body. The default applies there.

## API

No new endpoints. A new error response on every route:

| Status | Body | When |
|---|---|---|
| 413 | `{"error": "request body too large"}` | Declared `Content-Length` above the route's limit, or a body without a declared length that exceeds it |

- For the import route this replaces Echo's default body `{"message":"Request Entity Too Large"}` with the `{"error": ...}` shape. The status is still 413. The admin import panel branches on `err.status === 413` only, so it is unaffected.
- Swagger: add `@Failure 413 {object} map[string]string` only where it already exists (import). Do not annotate every handler with 413: it is a global transport rule, documented in DEPLOYMENT.md. Regenerate swagger only if annotations change.

## Backend

### 1. Body limit middleware: `backend/internal/transport/http/middleware/body_limit.go`

`func BodyLimit(defaultLimit int64, overrides map[string]int64) echo.MiddlewareFunc`

- Key = `c.Request().Method + " " + c.Path()`. `c.Path()` is the matched route pattern, e.g. `POST /api/v1/admin/books/import`. Middleware registered with `e.Use` (not `e.Pre`) runs after routing in Echo v4, so `c.Path()` is set. Verify this with a test.
- `limit = overrides[key]`, otherwise `defaultLimit`.
- `ContentLength > limit`: return the 413 JSON at once, without reading the body.
- `ContentLength == -1` (unknown/chunked): read at most `limit+1` bytes (`io.LimitReader`) into memory. Above `limit`: 413. Otherwise replace `Request().Body` with `io.NopCloser(bytes.NewReader(buf))` and set `ContentLength = len(buf)`. This buffering is bounded by `limit` (at most 2 MiB). It gives a deterministic 413 instead of the handler mapping the read error from `c.Bind` to 400 `invalid request`, which is what Echo's own `BodyLimit` does for chunked bodies.
- `0 <= ContentLength <= limit`: pass through. `net/http` never reads past the declared length.
- 413 response: `c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"})`. Use the same literal as the convention in module handlers.

### 2. Limits and override table: `backend/internal/transport/http`

- `const DefaultBodyLimit int64 = 1 << 20`, plus `func BodyLimitOverrides() map[string]int64`, which returns `{"POST /api/v1/admin/books/import": adminhttp.ImportMaxRequestBodyBytes}`.
- `backend/internal/modules/admin/transport/http/import_parse.go`: replace `ImportMaxRequestBody = "2M"` with `ImportMaxRequestBodyBytes int64 = 2 << 20`, and keep the doc comment (the 413 is now applied by the global middleware through the override table).
- `router.go`: remove `echomiddleware.BodyLimit(...)` from the import route. Drop the `echomiddleware` import if it becomes unused.

### 3. Wiring in `backend/cmd/api/main.go`

Middleware order: `Recover`, `ErrorReporting` (if enabled), `SecurityHeaders`, `CORS`, `RequestLogger`, then **`middleware.BodyLimit(httptransport.DefaultBodyLimit, httptransport.BodyLimitOverrides())`**. Placing it after CORS puts CORS headers on the 413. Placing it after the logger logs the 413.

### 4. Server timeouts: `internal/config` plus `cmd/api`

- `config.Config` gets `HTTPReadHeaderTimeout`, `HTTPReadTimeout`, `HTTPWriteTimeout`, `HTTPIdleTimeout` (`time.Duration`). They are parsed with the existing `parseNonNegDuration`, so empty, invalid or negative values fall back to the default and `0` means no limit, as with the `DB_CONN_*` vars. Exported defaults:

  | Env var | Default | Why |
  |---|---|---|
  | `HTTP_READ_HEADER_TIMEOUT` | `10s` | slowloris guard; headers are tiny |
  | `HTTP_READ_TIMEOUT` | `60s` | whole request, including the body: a 2 MiB import at about 40 KB/s still fits |
  | `HTTP_WRITE_TIMEOUT` | `5m` | backstop only: synchronous admin Wikidata calls (sync-adaptations, sync-screen-works, enrich-wikidata for up to 50 ids; 15s HTTP timeout per Wikidata call plus a shared rate limiter) and the 500-row import can legitimately run for minutes |
  | `HTTP_IDLE_TIMEOUT` | `120s` | keep-alive between requests from Caddy |

- `main.go`: before `e.Start`, set `e.Server.ReadHeaderTimeout/ReadTimeout/WriteTimeout/IdleTimeout` from cfg. `e.Start` uses `e.Server` and only fills `Addr`. Equivalently, use `e.StartServer(&http.Server{Addr: ":" + cfg.Port, ...})`. Keep the `http.ErrServerClosed` check. Log the four values once at startup, as is done for the DB pool.
- To make this testable, put the assignment in a small helper, e.g. `httptransport.ApplyServerTimeouts(s *http.Server, t ServerTimeouts)` with `config` filling a `ServerTimeouts`. A unit test asserts the fields. Do not add a new abstraction beyond that.
- Graceful shutdown is unchanged: `e.Shutdown(ctx)` with 10s. Note that `WriteTimeout` (5m) is longer than the shutdown budget, so a long admin sync that is in flight at SIGTERM is cut after 10s. This is the same as today, so accept it and document it. Do not raise the shutdown budget in this change: Docker's default stop grace period is 10s too.
- No SSE or streaming exists (no `text/event-stream` or `Flush` in the backend), so `WriteTimeout` does not affect a streaming route.

## Database

None.

## Frontend

No changes. Legitimate UIs cannot reach 1 MiB: form fields have server max lengths, and the paste textarea's 100 000-character cap stays under the limit. The admin file import already maps 413 to its «файл слишком большой» message by status. `packages/api-client` reads `body.error`, so any other 413 surfaces as «request body too large» through the generic error path. That is acceptable for a case that only a non-UI client can trigger.

## Existing Patterns

- Error shape `map[string]string{"error": ...}`, as in the admin and screenworks handlers.
- Middleware with tests: `backend/internal/transport/http/middleware/security_headers.go` and `security_headers_test.go`.
- Duration env vars: `DB_CONN_MAX_LIFETIME` / `DB_CONN_MAX_IDLE_TIME` in `internal/config/config.go` (`parseNonNegDuration`) and their table test in `config_test.go`.
- Route-level tests through the real router: `backend/internal/transport/http/router_*_test.go`.

## Tests

- `middleware/body_limit_test.go`:
  - Declared length above the limit: 413 with `{"error":"request body too large"}`, handler not called.
  - At the limit: handler called and reads all bytes.
  - Chunked (`ContentLength = -1`) above the limit: 413, handler not called.
  - Chunked under the limit: handler gets the exact bytes.
  - Override key: a 1.5 MiB body passes on the override route and gets 413 on another route. The override route rejects 2 MiB + 1.
  - Unknown route: the default limit applies.
- Router-level test with the middleware in the `cmd/api` order:
  - `POST /api/v1/auth/register` with a 1 MiB + 1 body: 413 JSON with CORS headers for an allowed origin.
  - `POST /api/v1/admin/books/import` (admin bearer) with a valid multipart file of about 1.5 MiB: not 413 (expect 200 or 422 depending on the rows).
  - The same route above 2 MiB: 413.
  - Paste preview with 100 000 Cyrillic characters, ASCII-escaped as `\uXXXX` in the JSON: not 413.
- `config_test.go`: defaults, overrides (`HTTP_READ_TIMEOUT=30s` and so on), and invalid/negative values falling back to the default. Same table shape as the DB pool test.
- The timeouts helper test asserts that the four `http.Server` fields are set.
- Verify with `go build ./...`, `go vet ./...`, `go test ./...` from `backend/`. Run gofmt only on changed files.

## Documentation

- `docs/DEPLOYMENT.md`:
  - Add the four `HTTP_*` rows to "Environment variables (backend / api)".
  - Add a short paragraph on the body limit: the 1 MiB default, the 2 MiB import, the 413 shape, and that new upload routes need an override entry.
  - Add a note that a WriteTimeout-length admin request in flight at deploy is cut by the 10s shutdown.
- `.env.example` and `.env.production.example`: add the commented `# HTTP_READ_HEADER_TIMEOUT=10s` and the other three, next to the `DB_CONN_*` block.
- `docs/ROADMAP.md`: tick the #623 line (line ~155) and point it to this contract.
- Swagger: no regeneration unless annotations change.

## Risks

- **Echo middleware ordering and `c.Path()`.** If `c.Path()` is empty when the middleware runs, the override silently falls back to 1 MiB, and imports between 1 and 2 MiB break. The router-level test for a 1.5 MiB import guards this.
- **Hidden large bodies.** An endpoint that is not in the inventory above could need more than 1 MiB. The inventory covered every POST/PUT/PATCH in `router.go` and `cmd/api/main.go`, and the only multipart route is the import. The 413 is clear and easy to diagnose, and the fix is one override entry.
- **WriteTimeout is a hard deadline.** It does not cancel the handler context: the handler keeps running and only the response is lost. 5m is generous for the current synchronous admin operations. If the 50-id Wikidata syncs grow, move them to the worker instead of raising the timeout.
- **Buffering of chunked bodies** costs up to the route's limit in memory per in-flight request. This is bounded (1 MiB, or 2 MiB for import), and browsers always send a `Content-Length` for fetch bodies.
- Caddy has its own (default, unlimited) proxy timeouts and no `request_body` limit. This is left as is: the api is the enforcement point.
