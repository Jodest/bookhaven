# Health readiness check (PostgreSQL + Redis)

Status: done

Issue: #248

## Goal

`GET /health` returns a static `{"status":"ok"}`, so the post-deploy check and external uptime monitoring stay green while PostgreSQL or Redis is unreachable. Add a readiness endpoint that actually checks both dependencies, and point deploy verification and uptime monitoring at it — without turning a short DB/Redis outage into a full API outage at the proxy.

## User Stories

- As the maintainer, I want uptime monitoring to alert me when the API cannot reach PostgreSQL or Redis, not only when the process is dead.
- As the maintainer, I want a deploy to fail its post-deploy check when the new backend starts but cannot reach its dependencies.
- As the maintainer, I do not want a short Redis blip to make Caddy drop the only backend upstream and return 502/503 for every API request, including ones that do not need Redis.

## Acceptance Criteria

- [x] `GET /health` (liveness) is unchanged: always `200 {"status":"ok"}` while the process serves HTTP, no dependency checks. Backward compatible for Caddy `health_uri`, README, docs.
- [x] New `GET /health/ready` (readiness) checks PostgreSQL and Redis.
  - Both reachable → `200 {"status":"ok","checks":{"database":"ok","redis":"ok"}}`.
  - Any unreachable → `503 {"status":"unavailable","checks":{"database":"ok|unavailable","redis":"ok|unavailable"}}`.
- [x] Each check has its own timeout (2 s); checks run concurrently, so the endpoint answers in about 2 s at worst even if both dependencies hang (a dead TCP peer must not hang the request).
- [x] The response body never contains error messages, hostnames, DSNs, driver errors or versions — only the fixed words `ok` / `unavailable`. The failure reason is logged server-side (zap, Warn, with the check name and error).
- [x] Both endpoints are public (no auth), GET only, outside `/api/v1`, and respond with `Cache-Control: no-store`.
- [x] Caddy keeps `health_uri /health` (liveness) — not switched to `/health/ready`.
- [x] `deploy.yml` post-deploy check hits `https://${API_DOMAIN}/health/ready` with retries (the backend may need a few seconds after `up -d`), and still dumps backend/caddy logs and fails on error.
- [x] `docs/DEPLOYMENT.md` points uptime monitoring and the manual verify step at `/health/ready` and explains liveness vs readiness.

## Business Rules

- Liveness (`/health`) = "the process is up and serving HTTP". Used by Caddy's active health check. It must not depend on PostgreSQL/Redis: Caddy has a single backend upstream, so marking it down on a dependency blip would replace endpoint-specific errors with a blanket 502/503 and also hide the readiness answer from the uptime checker.
- Readiness (`/health/ready`) = "the API can reach PostgreSQL and Redis". Used by the deploy check and external uptime monitoring (alerting). Nothing restarts containers based on it.
- Checks: PostgreSQL = ping over the existing GORM pool (`sql.DB.PingContext`); Redis = `PING` over the existing go-redis client. The asynq client/inspector use the same Redis address, so one Redis check covers them. No `SELECT` on app tables, no writes.
- `status` is `ok` only if every check is `ok`.

## API

| Method | Path | Auth | Success | Failure |
|---|---|---|---|---|
| GET | `/health` | none | 200 `{"status":"ok"}` (unchanged) | — |
| GET | `/health/ready` | none | 200 `{"status":"ok","checks":{"database":"ok","redis":"ok"}}` | 503 `{"status":"unavailable","checks":{...}}` |

Check keys are fixed: `database`, `redis`. Values: `ok` | `unavailable`. Not documented in swagger: its `@BasePath` is `/api/v1` and these routes are served at the root (the old `/health` was undocumented too).

## Database

None.

## Backend

- New file `backend/internal/transport/http/health.go`:
  - small interface `HealthChecker { Name() string; Check(ctx) error }` (or `map[string]func(ctx) error`) — keep it minimal;
  - adapters: GORM (`db.DB()` → `PingContext`) and Redis (`client.Ping(ctx).Err()`);
  - `RegisterHealthRoutes(e *echo.Echo, logger *zap.Logger, timeout time.Duration, checks ...)` registering `/health` and `/health/ready`; checks run in goroutines, each under `context.WithTimeout(c.Request().Context(), timeout)`.
- `backend/cmd/api/main.go`: replace the inline `e.GET("/health", ...)` with `RegisterHealthRoutes(...)` passing `db`, `redisClient`, `logger`, 2 s timeout. No other wiring changes.
- Out of scope: the worker (no HTTP server), the similarity CLI, migrate — none expose health endpoints.

## Frontend

None. No app reads `/health` (verified: web/admin, packages, run-bookhaven skill do not).

## DevOps

- `Caddyfile`: no change (stays on `/health`); add a one-line comment explaining why it is liveness, not readiness.
- `.github/workflows/deploy.yml`: replace `curl -fsS "https://${API_DOMAIN}/health"` with `curl -fsS --retry 6 --retry-delay 5 --retry-all-errors "https://${API_DOMAIN}/health/ready"` (keep the `|| (logs ... && exit 1)` fallback).
- No `healthcheck:` added to the backend container in `docker-compose*.yml`: the prod image is distroless (no shell/curl; would need a separate static probe binary per `.claude/agents/devops.md`), and Docker does not restart on `unhealthy` anyway. Caddy already probes liveness. Leave as a possible follow-up.
- `docker-compose.yml` (dev): no change.

## Existing Patterns

- Route registration in `backend/internal/transport/http/router.go`; handler tests use `httptest` + `echo.New()` as in `router_screenworks_test.go`.
- Shared `redisClient` and `db` already built in `cmd/api/main.go`.

## Tests

`backend/internal/transport/http/health_test.go` with fake checkers:
- `/health` → 200 `{"status":"ok"}` even when checkers fail.
- `/health/ready` all ok → 200, both `ok`.
- DB failing → 503, `database: unavailable`, `redis: ok`; Redis failing → mirror.
- Body of a failing response does not contain the checker's error text.
- Hanging checker (blocks until ctx done) with a short injected timeout (e.g. 50 ms) → 503 returned within a small bound; both hanging still finish within ~one timeout (concurrency).
- `Cache-Control: no-store` set.
Verification: `go build ./... && go vet ./... && go test ./internal/transport/http/...` from `backend/`.

## Documentation

- `docs/DEPLOYMENT.md`: first-deploy verify step → `/health/ready` returns `{"status":"ok",...}`; Monitoring → uptime checker on `https://<API_DOMAIN>/health/ready`; short liveness vs readiness paragraph (Caddy uses `/health`).
- `README.md` services table: mention `/health/ready`.
- `docs/ROADMAP.md`: mark the #248 item done.

## Risks

- Public unauthenticated endpoint that touches PostgreSQL/Redis: the ping is cheap and uses the existing pool, but it is a small amplification vector. Acceptable at current scale; if abused, rate-limit at Caddy later.
- Request log noise: Caddy hits `/health` every 30 s (already the case); uptime checker adds `/health/ready` hits. Acceptable.
- `gorm.Open` still fatal-exits if PostgreSQL is down at startup (container restarts via `restart: unless-stopped`); readiness only covers outages after startup. Unchanged, out of scope.
- If someone later switches Caddy `health_uri` to `/health/ready`, a dependency blip becomes a full API outage — the Caddyfile comment guards against this.
