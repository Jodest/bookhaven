# Design

## Context

- `refresh_tokens` (000002): `expires_at`/`revoked_at` are BIGINT unix seconds; indexes only on `user_id` and `token_hash`. Rows are revoked (never deleted) on rotation, logout, unconfirmed-email refresh and password reset.
- `email_verification_tokens` (000011) / `password_reset_tokens` (000012): `expires_at`/`used_at` TIMESTAMPTZ; index only on `user_id`. Unused rows are already deleted on resend / new reset request / successful reset; used and expired rows stay forever.
- The auth service (`backend/internal/modules/auth/service/service.go`) works on `*gorm.DB` directly (no repository package). Behaviour that depends on stored rows:
  - `RefreshTokens` filters `revoked_at IS NULL AND expires_at > now`; a missing row and a revoked/expired row both give `ErrTokenInvalid` (401). **There is no reuse/theft detection** — revoked rows are never read again. Deleting them is unobservable.
  - `VerifyEmail`: lookup by hash; missing row -> `invalid_token`; used/expired row -> `already_verified` (account confirmed) or `token_used` / `token_expired`. Deleting a row turns those into `invalid_token` — hence the 30-day retention and the spec change.
  - `ResetPassword`: same pattern (`token_used` / `token_expired` vs `invalid_token`).
- A used token always has `used_at <= expires_at` (it cannot be used after expiry), so filtering on `expires_at` alone also gives every used row at least the full retention after expiry.

## Decisions

1. **Retention constants** (in `auth/domain/tokens.go`, exported, documented):
   - `RefreshTokenRetention = 7 * 24h` — rows with `revoked_at < now-7d` or `expires_at < now-7d`. 7 days is a short forensic trail for support/incident questions; not needed by any code path.
   - `OneTimeTokenRetention = 30 * 24h` — verification and reset rows with `expires_at < now-30d`, used or not. Reset links live 1h, verification 24h, so a link keeps its specific answer for ~a month, which covers any realistic "clicked an old email" case.
   Not configurable (no env vars) — proportionate; change the constants if ever needed.
2. **One daily job, one task**: `auth:purge_tokens` (empty payload) purges all three tables in one run. Each table is independent: a failure on one is logged and the others still run; the handler returns an aggregated error at the end so asynq retries (the purge is idempotent).
3. **Bounded batches**: delete by primary key in batches of 1000 —
   `DELETE FROM <t> WHERE id IN (SELECT id FROM <t> WHERE <predicate> LIMIT ?)` — looping until a batch deletes fewer than the batch size, checking `ctx.Err()` between batches. Each statement is its own short implicit transaction (no wrapping transaction). Refresh tokens run two loops (expired predicate, then revoked predicate) so each uses its own index instead of an `OR`. Works on both Postgres and SQLite (tests). Batch size is a field on the job (default 1000) so tests can use 2 to exercise the loop.
4. **Clock**: the job takes `now func() time.Time` (default `time.Now`), always used as UTC. Refresh cutoffs are `now.Add(-retention).Unix()`; the others pass a `time.Time`.
5. **Worker wiring**: its own small asynq server (queue `auth_maintenance`, Concurrency 1), **always on** like `similarity` (no flag — purging is not optional). Scheduled daily at `45 3 * * *` UTC (`authTokenPurgeCronSpec` constant in `cmd/worker/main.go`, between the 03:30 similarity recompute and the 04:00 harvest tick). Task options: `Queue("auth_maintenance")`, `MaxRetry(2)`, `Timeout(10m)`, `Unique(6h)` — the Unique is defense in depth only; the single-worker-replica rule in DEPLOYMENT.md still applies.
6. **Logging**: one info line per run, `auth tokens purged`, with `refresh_deleted`, `verification_deleted`, `reset_deleted` counts (log even when zero — it is the daily health signal); a warn line per failing table. Never log token hashes, emails or user ids.
7. **Indexes**: add them (migration 000038). Without them every batch is a sequential scan; with partial/plain btree indexes each batch is an index range scan and the purge stays cheap as tables grow.

## API

No endpoint changes. Observable change only for links older than expiry + 30 days: `POST /api/v1/auth/verify-email` and `POST /api/v1/auth/reset-password` answer 400 `invalid_token` instead of `already_verified` / `token_used` / `token_expired`. `POST /api/v1/auth/refresh` is unchanged (401 either way).

## Database

New forward-only migration `backend/migrations/000038_auth_token_purge_indexes.{up,down}.sql`:

```sql
-- up
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_revoked_at ON refresh_tokens(revoked_at) WHERE revoked_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_expires_at ON email_verification_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at ON password_reset_tokens(expires_at);
-- down: DROP INDEX IF EXISTS for the four (indexes only, no data loss)
```

Plain `CREATE INDEX` (not CONCURRENTLY), same as 000013 — the tables are small and the lock is momentary. No column or data changes; the purge itself is application code, not a migration.

## Frontend

None. A purged link already maps to the existing `invalid_token` state on `/verify-email` and `/reset-password`.

## Existing Patterns

- Scheduled task + dedicated small server: `catalog/service/similarity_task.go` (task type, queue const, `RecomputeTaskOptions` with `Unique`) and its wiring in `cmd/worker/main.go` (`similaritySrv`, `registerSchedules`, startup log after `scheduler.Start()`).
- Purge-with-logged-counts style: `aicurator/service/tasks.go` `purge` (info log only when something was deleted there; here log every run).
- Tests: SQLite DDL helpers in `auth/service/service_test.go` (`refreshTokensDDL`, `setupTestDB`) and `verification_test.go` (`emailVerificationTokensDDL`, `passwordResetTokensDDL`, `setupVerificationEnv`); worker schedule tests with `fakeRegistrar` in `cmd/worker/main_test.go`.

## Risks

- **Worker schedule tests**: several tests in `cmd/worker/main_test.go` assert `len(f.regs) == 1` ("only the similarity schedule"). The always-on purge makes the baseline 2; update those assertions (e.g. a helper that ignores the purge registration, or expect 2) — do not make the purge conditional to dodge them.
- **SQLite time comparison**: TIMESTAMPTZ columns are text in SQLite; seed and compare in UTC only, or comparisons are wrong in tests.
- **Spec change**: very old verification/reset links now answer `invalid_token`. Acceptable UX (the page already offers a new link), called out as BREAKING (minor) in the proposal.
- **Single scheduler**: an extra schedule does not change the one-replica rule; `Unique(6h)` is only a safety net.
- First run on a large backlog: batches keep each lock short; the 10-minute timeout plus retries resume naturally because the purge is idempotent.
