# Design

## Context

`Service.RefreshTokens` (`backend/internal/modules/auth/service/service.go`) does `SELECT ... WHERE token_hash=? AND revoked_at IS NULL AND expires_at > now`, then `Model(&token).Update("revoked_at", now)` (unconditional, rows affected unchecked), then `createTokenPair` outside any transaction. Two concurrent refreshes both pass the SELECT and both mint a pair. A rotated token is filtered out by the SELECT and answered `ErrTokenInvalid`, so a replay is invisible.

`refresh_tokens` (migration 000002): `id, user_id, token_hash, expires_at, created_at, revoked_at` (BIGINT unix seconds), indexes on user_id, token_hash, plus purge indexes from 000038. Revocation reasons are not distinguishable today. Cookies are host-only (no `Domain`); each app proxies `/api` through its own origin, so all tabs of one app share one cookie jar (locally web and admin even share it, because cookies ignore the port). The api-client dedups refreshes only inside one JS context (one tab).

## Decisions

1. **Atomic rotation.** In one `db.Transaction`: `UPDATE refresh_tokens SET revoked_at=?, replaced_by=? WHERE id=? AND revoked_at IS NULL` (new id pre-generated with `uuid.New()`), check `RowsAffected == 1`, then `INSERT` the successor with the same `family_id`. `RowsAffected == 0` means a concurrent refresh won: roll back and return the new `ErrRefreshSuperseded` (the winner committed microseconds ago, so this is by definition within the grace window). Under PostgreSQL READ COMMITTED the loser's UPDATE blocks on the winner's row lock and re-evaluates `revoked_at IS NULL` after commit, so exactly one wins. The UPDATE goes first so the row lock is taken before any insert.
2. **Session family = one login.** New column `family_id`; a login creates a token with `family_id = id`; rotation copies the parent's `family_id`. Reuse revokes `WHERE family_id=? AND revoked_at IS NULL` only. Rationale: the evidence of compromise is one session's cookie; killing the user's other devices punishes unrelated sessions without adding protection (the attacker only ever held this family). "Log out everywhere" remains available via logout and password reset, which already revoke every token of the user.
3. **Rotation vs other revocations.** New column `replaced_by` (UUID, nullable, no FK): set only by rotation. Detection fires only for `revoked_at IS NOT NULL AND replaced_by IS NOT NULL`. Logout, password reset, the unconfirmed-email branch and family revocation set only `revoked_at`, so they never trigger detection. No separate reason enum needed. No FK on `replaced_by` because the purge deletes rows in independent batches and a successor can be purged before/after its parent.
4. **Grace window of 30 seconds (recommended over strict detection).** Strict detection turns two common benign events into a forced logout of the whole session: two tabs of one app firing refresh together (all timers/401 retries fire at once after the laptop wakes up), and a lost refresh response followed by a retry. The grace window only suppresses the family revocation; it never issues tokens for a rotated token (only hashes are stored, so the successor cannot be returned anyway), so an attacker gains no access within it. 30 s covers slow wake-up networks; detection still fires for any replay after that. Constant `RefreshReuseGrace = 30 * time.Second` in `auth/domain/tokens.go`, next to `RefreshTokenRetention`.
5. **Response codes.** Within grace: 401 `{error, code: "refresh_token_superseded"}` so the client can retry once (the sibling tab's Set-Cookie has put the successor in the shared jar). Reuse after grace: plain 401 `{"error":"invalid or expired refresh token"}`, byte-identical to an unknown token, so a thief cannot tell that detection fired or probe which tokens were rotated; the signal goes to the server log instead. The cookies are not cleared on reuse (same as the existing invalid-token path). The superseded code leaks only "this token was rotated in the last 30 s" to its holder — negligible.
6. **Lookup order** in `RefreshTokens`: find by `token_hash` without state filters → not found: `ErrTokenInvalid`; revoked with `replaced_by` set: grace → `ErrRefreshSuperseded`, else revoke family + `logger.Warn("refresh token reuse detected", user_id, family_id, revoked_count, seconds_since_rotation)` + `ErrTokenInvalid`; revoked otherwise or expired: `ErrTokenInvalid`; user missing: `ErrTokenInvalid`; unverified: existing `Logout` + `ErrEmailNotVerified`; else atomic rotation. Revoked is checked before expiry so a replay is detected even if the old token has meanwhile expired. The family revocation runs even if it affects 0 rows (family already dead); the warning is still logged.
7. **No absolute family lifetime** (sliding 30 days stays as today) — out of scope.
8. **Purge** unchanged; once a rotated row is purged (>= 7 days after rotation) its replay is an unknown token: plain 401, no detection. Accepted and spelled out in the spec.
9. **Frontend retry.** `packages/api-client` `performRefresh`: if `rawRefresh` rejects with an `ApiError` of status 401 and code `refresh_token_superseded`, wait ~1000 ms and call `rawRefresh` once more inside the same `refreshPromise` (so other callers in the tab keep sharing it). Any other failure, or a second failure, rejects as today. Without this, atomicity would turn today's (accidentally working) simultaneous multi-tab refresh into a spurious sign-out of the losing tab.

## API

`POST /api/v1/auth/refresh` (public, no bearer, no role):
- 200 unchanged.
- 401 `{"error":"refresh token superseded","code":"refresh_token_superseded"}` — token rotated within the last 30 s (includes the loser of a concurrent refresh). Cookies untouched.
- 401 `{"error":"invalid or expired refresh token"}` — unknown/expired/revoked token and reuse after the grace window (unchanged body).
- 401 `email_not_verified` unchanged.
Handler: add `codeRefreshTokenSuperseded = "refresh_token_superseded"` next to `codeEmailNotVerified`, map `service.ErrRefreshSuperseded`; update the swagger `@Failure 401` comment.

## Database

Forward migration `backend/migrations/000040_refresh_token_families.up.sql` (verify 000040 is still the next free number when implementing):

```sql
ALTER TABLE refresh_tokens ADD COLUMN family_id UUID;
UPDATE refresh_tokens SET family_id = id WHERE family_id IS NULL;
ALTER TABLE refresh_tokens ALTER COLUMN family_id SET DEFAULT gen_random_uuid();
ALTER TABLE refresh_tokens ALTER COLUMN family_id SET NOT NULL;
ALTER TABLE refresh_tokens ADD COLUMN replaced_by UUID;
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON refresh_tokens(family_id) WHERE revoked_at IS NULL;
```

- The `DEFAULT gen_random_uuid()` keeps the old binary working between `migrate up` and `up -d` (deploy order in DEPLOYMENT.md): its inserts get their own family. New code always sets `family_id` explicitly.
- The table is small (purged to 7 days of revoked rows), so the backfill UPDATE and `SET NOT NULL` scan are cheap.
- `.down.sql` drops the index and the two columns (repo convention; never run by agents).
- Domain struct: `FamilyID uuid.UUID gorm:"type:uuid;not null;index"`, `ReplacedBy *uuid.UUID gorm:"type:uuid"`.
- SQLite test DDLs must gain both columns: `refreshTokensDDL` in `auth/service/service_test.go` and `testRefreshTokensDDL` in `auth/transport/http/verification_test.go` (grep for other copies of `CREATE TABLE refresh_tokens` in tests, e.g. users/purge tests).

## Frontend

Only `packages/api-client/src/index.ts` (`performRefresh`, plus an exported `REFRESH_TOKEN_SUPERSEDED_CODE` constant and an updated comment). No app code or UI text changes; both apps get the behavior through the shared client. The api-client package has no test runner, so the test lives in `apps/web` (vitest, mock `fetch`) and calls the exported refresh entry point (`authApi.refresh` around line 2516).

## Existing Patterns

- Transactions with a helper taking `*gorm.DB`: `revokeRefreshTokens(db, userID, now)` and the password-reset transaction (`ResetPassword`, `s.db.WithContext(ctx).Transaction`).
- Error-code responses: `errorResponse{Error, Code}` and `codeEmailNotVerified` in the auth handler.
- Logging without secrets: `TokenPurgeJob` log lines ("No hashes, emails or ids" there; here user_id and family_id are fine, never the token or its hash).
- Clock: tests set `created_at`/`revoked_at` directly in rows to simulate "rotated 5 s / 60 s ago" rather than injecting a clock into the service.

## Risks

- **Lost response after the grace window**: a client whose refresh response was lost and which retries much later kills its own family and must log in again. Same outcome as strict detection; acceptable.
- **Shared cookie jar locally** (web on :3000 and admin on :3001 share host-only cookies on localhost): cross-app races locally are covered by the same grace + retry; in production the apps are separate hosts.
- **Migration number collision** with another in-flight branch — check before committing.
- **SQLite vs PostgreSQL**: the true concurrent race cannot be reproduced on in-memory SQLite; test the conditional update by rotating from a stale snapshot (second call must report "not rotated" and leave exactly one successor). Real locking semantics rely on PostgreSQL READ COMMITTED.
- `createTokenPair` must be split so the insert can run on the transaction handle (`tx`) with a given id and family; login keeps calling it with a fresh family.
