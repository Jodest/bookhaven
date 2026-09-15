# Refresh Token Hash Lookup Fix

Status: done

## Goal

`POST /api/v1/auth/refresh` has never worked, for any user, since the endpoint was introduced — it always returns `401 {"error":"invalid or expired refresh token"}`, even for a token issued seconds earlier and still valid. Root cause: `hashToken()` in `backend/internal/modules/auth/service/service.go` uses `bcrypt.GenerateFromPassword`, which is non-deterministic (random salt per call). It is used both to compute the hash stored at issuance (`createTokenPair`) and to compute the hash used in an exact-match SQL lookup at validation time (`RefreshTokens`). The two hashes of the same plaintext token never match, so the lookup always misses. Fix: replace `hashToken()` with a deterministic hash (SHA-256) suitable for exact-match lookup of an already-high-entropy random token. Backend-only fix; no frontend changes.

Verified against current code on `fix/refresh-token-hash-lookup` (`backend/internal/modules/auth/service/service.go` lines 141, 198, 228-231; `backend/internal/modules/auth/domain/tokens.go`; `backend/internal/modules/auth/transport/http/handler.go`; `backend/migrations/000002_refresh_tokens.up.sql`) — analysis in the task description matches the code exactly.

## User Stories

- As a logged-in user with an active session, when my access token expires and the client silently calls `/api/v1/auth/refresh` with my still-valid refresh token, I want it to succeed and issue me a new token pair, so my session doesn't die every ~15 minutes.
- As a returning user, I want to log in once and stay logged in for up to 30 days (the refresh token TTL) without being forced to re-authenticate, as long as I keep using the app.

## Acceptance Criteria

- [ ] `hashToken()` produces a deterministic output: calling it twice with the same plaintext input yields the same string.
- [ ] A refresh token issued via `Login`, `Register`, or `RefreshTokens` (i.e. via `createTokenPair`) can be immediately validated by a subsequent call to `RefreshTokens` with that same plaintext token — this must be covered by an automated test that fails against the current (buggy) code and passes after the fix.
- [ ] Refresh token rotation still works correctly: after a successful `RefreshTokens` call, the old refresh token is revoked (`revoked_at` set) and a second attempt to use the same old plaintext token fails with `ErrTokenInvalid`; the newly-issued refresh token from the response works for a subsequent refresh.
- [ ] Existing password hashing (`bcrypt.GenerateFromPassword`/`CompareHashAndPassword` in `Register`/`Login`) is untouched — this fix is scoped to `hashToken()` only.
- [ ] No change to the `POST /api/v1/auth/refresh` request/response contract, cookies, or TTLs.
- [ ] `unique_active_token_per_user` unique constraint on `(user_id, token_hash)` continues to hold (deterministic hash of a 256-bit random value has negligible collision probability; no schema change needed since `token_hash` is `VARCHAR(255)` and a hex-encoded SHA-256 digest is 64 chars).

## Business Rules

- Refresh tokens are stored only as a hash (never plaintext) in `refresh_tokens.token_hash`, so that a leak of the `refresh_tokens` table alone does not let an attacker replay a valid refresh token.
- Because the refresh token itself is a 32-byte (`crypto/rand`) value — i.e. already maximally high-entropy — the hash does not need per-value salting or an adaptive/slow algorithm the way a low-entropy user password does. It needs to be deterministic (so the same plaintext always maps to the same lookup key) and one-way (irreversible given only the stored hash).
- **No data migration or backfill for existing `refresh_tokens` rows.** Because the bug means refresh has never succeeded for any token, every existing stored `token_hash` value is already permanently unmatchable (dead) — switching the hash algorithm does not newly invalidate any session that was ever capable of refreshing, because none were. Existing sessions need one more fresh login (as they always effectively did, given the bug); from that point on they run entirely on the fixed code path. Dead rows are left to expire naturally via their existing `expires_at` and are never read again — not worth a cleanup migration for a one-time, zero-risk, self-resolving condition. (An opportunistic `DELETE FROM refresh_tokens WHERE expires_at < now` housekeeping job is a reasonable independent idea but is out of scope here and not required by this fix.)

## API

None. `POST /api/v1/auth/refresh` (and `/login`, `/register`, `/logout`) keep their existing request/response shape, status codes, and cookie behavior (`backend/internal/modules/auth/transport/http/handler.go`). The only observable change is that `/auth/refresh` now actually succeeds (200) for a valid, unexpired, unrevoked refresh token instead of always returning 401.

## Database

None (no migration). `refresh_tokens.token_hash` is already `VARCHAR(255)` (`backend/migrations/000002_refresh_tokens.up.sql`), which comfortably fits a hex-encoded SHA-256 digest (64 chars) alongside the previous bcrypt format (60 chars) — no column resize needed, and old bcrypt-hash rows simply age out via `expires_at` without ever being read via the new format.

## Backend Implementation

Single-function fix in `backend/internal/modules/auth/service/service.go`. No call-site changes: both `createTokenPair()` (line ~141) and `RefreshTokens()` (line ~198) already call `hashToken(x)` identically and use the result as an opaque string for storage/lookup — confirmed no other code reads or writes `RefreshToken.TokenHash` in a bcrypt-format-dependent way (checked `domain/tokens.go`, `Logout()` — which only touches `revoked_at`, not `token_hash` — and the migrations; no admin tooling reads this field).

Replace:

```go
func hashToken(token string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	return string(hash)
}
```

with a deterministic SHA-256 hash, hex-encoded (the package already imports `encoding/hex` for the token generation itself):

```go
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
```

Add `"crypto/sha256"` to imports. If `bcrypt` becomes unused elsewhere in the file after this change, verify it's still needed — it is, for `Register`/`Login` password hashing, so the import stays.

**Design decision — plain SHA-256, no HMAC pepper:** considered HMAC-SHA256 keyed with `cfg.JWTSecret` (or a new pepper config value) for defense-in-depth against a raw DB leak. Rejected as unwarranted complexity here: the value being hashed is already a uniformly random 256-bit secret (`crypto/rand`), not a low-entropy value vulnerable to precomputation/rainbow-table attacks — the entire reason peppers/salts help password hashing (defending against precomputation over a small/guessable input space) does not apply when the input space is 2^256. Plain SHA-256 is one-way and collision-resistant, which is all that's needed to keep the plaintext token unrecoverable from a leaked hash. Introducing a pepper would add a new config value, a key-rotation concern, and no measurable security benefit for this specific input — over-engineering for this case. Plain SHA-256 is also exactly the standard pattern for hashing high-entropy opaque tokens (e.g. OAuth/session refresh tokens).

**Design decision — `ConstantTimeCompare()` stays unused (or is removed), not wired in:** confirmed it is currently dead code (defined, never called). The new lookup is `db.Where("token_hash = ? AND ...", hash).First(...)` — an exact-match SQL/index lookup, not an application-level byte comparison of two secret values. The timing side-channel that constant-time comparison defends against (leaking information about a secret via how long a byte-by-byte comparison takes) doesn't meaningfully apply to a database index lookup, whose timing is dominated by index traversal/I/O, not per-byte comparison of the hash, and where the "secret" being compared against is itself a public-in-effect hash of a value the attacker doesn't have. This is the standard, ordinary pattern for this kind of token-hash lookup (unlike comparing a login password hash where `CompareHashAndPassword` already handles that internally). Recommendation: leave `ConstantTimeCompare` unused as-is (do not force a call site that doesn't need it) — or delete it as unrelated dead-code cleanup if the developer prefers a tidier diff; either is acceptable, note the decision made in the PR description.

## Existing Patterns

- `backend/internal/modules/auth/service/service.go` — refresh token generation (`crypto/rand`, `hex.EncodeToString`) is already correct and unchanged; only the hashing step is wrong.
- `backend/internal/modules/auth/domain/tokens.go` — `RefreshToken.TokenHash` is treated as an opaque string everywhere; no format assumptions elsewhere in the codebase.

## Tests

Add `backend/internal/modules/auth/service/service_test.go` (no test file currently exists in this package — this gap is exactly what let the bug ship undetected). Needs a test DB/sqlite or the project's existing repository test-setup pattern (check `backend/internal/modules/users/repository` or similar for the project's preferred lightweight DB-in-test approach before choosing one; use whatever the codebase already uses for GORM-backed service tests, don't introduce a new pattern). Cover:

1. **`hashToken` determinism** (pure unit test, no DB): same input twice yields identical output.
2. **Core regression test** — the one that would have caught this bug: create a user, call `createTokenPair` (or `Login`), then immediately call `RefreshTokens` with the returned plaintext refresh token, assert success (no error, new `TokenPair` returned).
3. **Rotation correctness**: after a successful `RefreshTokens` call, calling `RefreshTokens` again with the *original* (now-revoked) plaintext token fails with `ErrTokenInvalid`; calling `RefreshTokens` with the *newly issued* refresh token from the first call's response succeeds.
4. **Expired/revoked/garbage token still rejected**: `RefreshTokens` with a random unknown plaintext string, an expired token, or a revoked token all return `ErrTokenInvalid` (guards against overcorrecting into "always succeeds").

Do not modify `backend/internal/modules/auth/transport/http/handler_test.go` unless it currently asserts on the (buggy) 401 behavior of `/auth/refresh` — check it as part of implementation; if it has a happy-path refresh test that was previously skipped/failing/absent because of this bug, add or un-skip it there too, but the primary coverage belongs in the service-layer test above since this is a service-layer bug.

## Documentation

- Add a line under `docs/ROADMAP.md`'s "Completed" section noting the refresh-token hash fix, linking to this contract, once shipped (mirrors how `silent-token-refresh.md` was linked).
- `docs/features/silent-token-refresh.md`'s Goal section currently states "The backend refresh flow (`POST /api/v1/auth/refresh`) is complete and correct" — this was incorrect (masked by the fact that both prior fixes were still independently necessary and correct for their own scope). Add a short note/correction there pointing to this contract, so the history isn't misleading to future readers. Not blocking, but recommended for documentation accuracy.
- Update this file's `Status:` to `done` after implementation and verification (matches convention in other feature docs).

## Risks

- **Low implementation risk**: this is a single small function body change with no call-site or schema changes, and the existing behavior it replaces has never worked, so there is no "working" behavior to regress.
- **Verification burden is on testing, not code complexity**: the actual risk historically was the *absence* of a test that exercises issuance-then-immediate-validation end-to-end — that's why the regression test above is called out as the primary deliverable alongside the one-line fix, not an afterthought.
- **Stale rows**: dead `refresh_tokens` rows from before the fix remain in the table until their `expires_at` passes (up to 30 days). They are inert (unmatchable, harmless) and require no action, but worth a one-line mention in the PR description so it isn't mistaken for a leak or leftover bug if noticed in the DB.
- **Manual empirical verification recommended before merge**: repeat the login → immediate `/api/v1/auth/refresh` call (as described in the bug report) against the fixed code and confirm 200, in addition to the automated tests, since this bug previously escaped automated coverage entirely.

## Sub-issue scope note

This is a narrow, single-file, backend-only bugfix with a well-understood root cause and no architectural ambiguity. Recommend filing as a single `[backend][fix]` issue rather than an epic with sub-issues — splitting "fix the function," "add the test," and "update docs" into separate issues would add process overhead disproportionate to the change size. One issue, one PR, covering the code fix + regression test + doc note.
