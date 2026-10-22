# Database Connection Pool Limits

Status: done

Issue: #249 (ROADMAP line "Configure database connection pool limits")

## Goal

Bound the number of PostgreSQL connections each long-running backend process can open. Today `cmd/api`, `cmd/worker` and `cmd/similarity` call `gorm.Open` and never configure the underlying `*sql.DB`. `database/sql` defaults to unlimited open connections, 2 idle connections and no lifetime, so a load spike in one process can use up PostgreSQL's `max_connections` (default 100) for every process that shares it.

## Acceptance Criteria

- [x] `cmd/api`, `cmd/worker` and `cmd/similarity` open the database through one shared helper that applies pool limits to the underlying `*sql.DB`: `SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime` and `SetConnMaxIdleTime`.
- [x] Each limit can be set through an env var. Without one, a safe default applies (see Business Rules).
- [x] An invalid value (not a number, unparsable duration, negative) falls back to the default instead of failing startup. The idle limit is capped at the open limit.
- [x] If the helper cannot get `*sql.DB` from GORM, the binary fails at startup with the same fatal/exit path as a `gorm.Open` error.
- [x] `cmd/seed` and `cmd/migrate` are unchanged. They are one-off tools with sequential access.
- [x] Unit tests cover config parsing and the helper. `go build ./... && go vet ./... && go test ./...` pass from `backend/`.
- [x] The new env vars are documented in `.env.example`, `.env.production.example` (commented, optional) and the `docs/DEPLOYMENT.md` env table.

## Business Rules

1. Env vars and their defaults:
   | Variable | Default | Meaning |
   |---|---|---|
   | `DB_MAX_OPEN_CONNS` | `20` | Max open connections per process (int >= 1) |
   | `DB_MAX_IDLE_CONNS` | `5` | Max idle connections kept (int >= 0, capped at the open limit) |
   | `DB_CONN_MAX_LIFETIME` | `30m` | Recycle a connection after this age (Go duration, `0` = no limit) |
   | `DB_CONN_MAX_IDLE_TIME` | `5m` | Close a connection idle this long (Go duration, `0` = no limit) |
2. Budget check: api (20) + worker (20) + one similarity run (20) + migrate/seed stays well under PostgreSQL's default 100. The worker's asynq servers run at most about 13 tasks at once (email `Concurrency: 4`, every other server 1), so 20 does not throttle it. Nothing in compose changes `max_connections`.
3. The values are per process. The api and worker services each have their own `environment:` block in compose, so operators can tune them separately. This plan does not add those keys to compose files; defaults are enough.
4. One setting per value: no per-binary defaults in code. The similarity CLI gets the same defaults (its job runs sequentially, so it uses few connections).

## API

None. `/health/ready` (PR #552) pings the same pooled `*sql.DB`; its behavior does not change.

## Database

No schema changes or migrations.

## Backend

- `backend/internal/config/config.go`: add `DBMaxOpenConns int`, `DBMaxIdleConns int`, `DBConnMaxLifetime time.Duration` and `DBConnMaxIdleTime time.Duration` to `Config`, with exported `Default…` constants next to the other defaults. Parse them in `Load()` with small helpers next to `parseClampedInt`: an int parser with a minimum and a duration parser using `time.ParseDuration`, falling back to the default when empty, invalid or negative. Apply `idle = min(idle, open)` after parsing.
- New package `backend/internal/pkg/database` (matching `internal/pkg/*`) with:
  - `type PoolConfig struct { MaxOpenConns, MaxIdleConns int; ConnMaxLifetime, ConnMaxIdleTime time.Duration }` and `func PoolConfigFrom(cfg *config.Config) PoolConfig` (or build the struct inline in each main; either is fine, keep it small);
  - `func ApplyPool(db *gorm.DB, p PoolConfig) error`: `db.DB()`, then the four setters;
  - `func Open(dsn string, p PoolConfig) (*gorm.DB, error)`: `gorm.Open(postgres.Open(dsn), &gorm.Config{})` followed by `ApplyPool`.
- `cmd/api/main.go`, `cmd/worker/main.go`, `cmd/similarity/main.go`: replace the direct `gorm.Open(postgres.Open(cfg.DatabaseURL), …)` with `database.Open(cfg.DatabaseURL, …)` and keep the existing error handling. Drop the now-unused `postgres`/`gorm` imports where they become unused.
- Optional: one info log line at startup in api/worker with the applied limits. No secrets.

## Existing Patterns

- Env parsing with defaults and clamping: `parseClampedInt`, `parseBoolEnvDefaultTrue`, `parseAuthCookieSecure` in `config.go`, with tests in `config_test.go` (set env via `t.Setenv`).
- Tests already call `sqlDB.SetMaxOpenConns(1)` on an in-memory SQLite GORM DB, e.g. `backend/internal/modules/admin/repository/within_transaction_sqlite_test.go`. Use the same setup to test `ApplyPool`.

## Tests

- `config_test.go`: defaults when unset; valid overrides; invalid int or duration falls back to the default; negative values fall back; idle > open is capped to open; `0` duration is accepted (no limit).
- `internal/pkg/database`: `ApplyPool` on an in-memory SQLite `*gorm.DB` sets `sqlDB.Stats().MaxOpenConnections` to the configured value. Idle count and lifetimes are not observable through `Stats()`, so asserting the open limit and a nil error is enough. Do not test `Open` against Postgres (there is none in unit tests).
- Run only the backend checks: `go build ./... && go vet ./... && go test ./...` from `backend/`. Format only the changed files (never `gofmt -w` the whole tree).

## Documentation

- `docs/DEPLOYMENT.md`: four rows in the env var table, plus one sentence on the connection budget (processes × `DB_MAX_OPEN_CONNS` must stay below PostgreSQL `max_connections`).
- `.env.example`, `.env.production.example`: the four keys, commented out, with their defaults.
- `docs/ROADMAP.md`: mark the #249 line done with a short "done:" note.

## Risks

- Setting `DB_MAX_OPEN_CONNS` too low makes requests wait for a connection under load instead of failing. That is the intended trade-off, but a long similarity recompute or harvest transaction now competes for a bounded pool within its own process only, never across processes. The defaults leave plenty of headroom.
- `ConnMaxLifetime` recycles connections every 30 minutes, which is harmless for PostgreSQL behind no pooler. If PgBouncer is added later, revisit these values.
- Operators running additional api replicas must multiply the budget accordingly. The DEPLOYMENT.md note covers this.
