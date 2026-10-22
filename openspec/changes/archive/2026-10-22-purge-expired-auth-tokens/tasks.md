# Tasks

## 1. Backend: migration

- [x] 1.1 Add `backend/migrations/000038_auth_token_purge_indexes.up.sql` / `.down.sql` exactly as in design.md (four `CREATE INDEX IF NOT EXISTS`, down = `DROP INDEX IF EXISTS`). Do not edit existing migrations. Apply forward only (`migrate up`); never run `migrate down`, drop or any DB reset to verify — confirm with `\d refresh_tokens` etc. if a local DB is up.

## 2. Backend: purge job in the auth module

- [x] 2.1 Add `RefreshTokenRetention` (7d) and `OneTimeTokenRetention` (30d) to `backend/internal/modules/auth/domain/tokens.go` with doc comments pointing at the auth specs.
- [x] 2.2 Add `backend/internal/modules/auth/service/purge.go`: `TaskTypePurgeTokens = "auth:purge_tokens"`, `PurgeQueue = "auth_maintenance"`, `PurgeTaskOptions()` (Queue, MaxRetry(2), Timeout(10m), Unique(6h)), `NewPurgeTokensTask()`, and a `TokenPurgeJob` (db, logger, clock, batch size default 1000; test options for clock and batch size) with `Run(ctx) (PurgeStats, error)` and `HandleTask`. Batched `DELETE ... WHERE id IN (SELECT id ... LIMIT ?)` per design.md; refresh tokens in two loops (expired, revoked); per-table failure logged, others continue, aggregated error returned; one `auth tokens purged` info log with the three counts; no hashes/emails/user ids in logs.
- [x] 2.3 Tests in `backend/internal/modules/auth/service/purge_test.go` (SQLite, reuse `refreshTokensDDL` / `emailVerificationTokensDDL` / `passwordResetTokensDDL`, fixed UTC clock, batch size 2): seed rows on both sides of every cutoff (active; revoked 6d vs 8d ago; expired 6d vs 8d ago; verification/reset used and unused with expiry 29d vs 31d ago; not-yet-expired) and assert exactly the old ones are deleted and counts match; more rows than the batch size are all deleted; a second run deletes 0; a cancelled context stops early with an error; `PurgeTaskOptions` contains the queue, `Unique(6h)` and the task type is `auth:purge_tokens`.
- [x] 2.4 Behaviour tests (same package): after purging a used verification token of a confirmed user, `VerifyEmail` returns `ErrVerificationTokenInvalid`; before the cutoff it still returns already-verified; after purging a used reset token `ResetPassword` returns `ErrResetTokenInvalid` and the password is unchanged; an active refresh token still refreshes after a purge run.
- [x] 2.5 Verify: `go build ./...`, `go vet ./...`, `go test ./internal/modules/auth/...` from `backend/`. gofmt only the changed files.

## 3. Backend: worker wiring

- [x] 3.1 In `backend/cmd/worker/main.go`: build `TokenPurgeJob` with the shared `db` and logger; start an always-on `auth_maintenance` server (Concurrency 1, own config func like the others) and include it in shutdown; add `authTokenPurgeCronSpec = "45 3 * * *"` and register it unconditionally in `registerSchedules`; log `auth token purge scheduled` (task, cron, UTC) after `scheduler.Start()`; extend the package doc comment.
- [x] 3.2 Update `backend/cmd/worker/main_test.go`: a test that `registerSchedules` (with nil cfg and with a default cfg) registers `auth:purge_tokens` on `authTokenPurgeCronSpec` on the `auth_maintenance` queue; adapt the existing "only the similarity schedule" assertions to the new always-on baseline without weakening what they check about the optional schedules.
- [x] 3.3 Verify: `go build ./...`, `go vet ./...`, `go test ./cmd/worker/...` from `backend/`.

## 4. Docs

- [x] 4.1 `docs/DEPLOYMENT.md` "Background jobs (worker)": list the `auth_maintenance` server in the first paragraph and add a short "Auth token purge" subsection (task, queue, 03:45 UTC, retention 7d refresh / 30d one-time links, batches of 1000, always on, log line `auth tokens purged` to check, single-replica rule applies).
- [x] 4.2 `docs/ROADMAP.md`: mark the #244 item done/in progress per the roadmap's conventions.
- [x] 4.3 Leave `openspec/specs/` untouched (updated on archive); run `npx -y @fission-ai/openspec@1.13.2 validate purge-expired-auth-tokens --strict` and fix any finding in the change folder.
