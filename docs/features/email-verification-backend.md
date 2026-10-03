# Email Verification Backend

Status: implemented

> **Changed by #160** ([email-verification-login-gate.md](email-verification-login-gate.md)): `POST /auth/register` no longer logs the user in (201 `{email, verification_required}`, no cookies), login/refresh/auth middleware reject unverified users (`email_not_verified`), and `POST /auth/resend-verification` became public (body `{email}`, always 202, per-IP + per-email limits; no 409). The sections below describe the #158 state.

GitHub issue: #158 (epic #162). Depends on #157 ([mailer-infrastructure.md](mailer-infrastructure.md), merged). Must be merged to `main` together with #159 (frontend `/verify-email` page), otherwise the link in the email leads nowhere. This is the first consumer of `internal/pkg/mailer`: it does the `cmd/api` wiring and adds the first template.

## Goal

On registration the user receives (asynchronously) a Russian email with a one-time link `<WEB_BASE_URL>/verify-email?token=...`. The web page (#159) posts the token to the API, which marks the email as verified. A logged-in user can request a new link (rate-limited). `email_verified` is exposed to clients.

## Scope boundaries

**In scope:** migration + backfill, verification token storage, Register hook, `POST /auth/verify-email`, `POST /auth/resend-verification`, Redis rate limiter, `email_verified` in user responses, mail template rendering, `WEB_BASE_URL` config, `cmd/api` wiring, seed accounts verified, API client types/methods/`ApiError.code`, docs. Optional cheap mailer fix (see Implementation Notes).

**Out of scope:** any UI (#159), restricting unverified users (#160), password reset (#161), periodic cleanup of old token rows, email change flow, OAuth-specific behaviour (OAuth users are not created by any current code path).

## User Stories

- As a new user, after registering I get an email in Russian with a link that confirms my address; I am logged in immediately and registration never fails just because email is down.
- As a user, I can open the link in any browser (even logged out) and my email gets confirmed.
- As a logged-in unverified user, I can request a new link; old links stop working; I cannot spam the button.
- As the frontend (#159), I get distinct machine-readable error codes (invalid / expired / used / already verified / rate limited) and `email_verified` on the current user.
- As an existing user / seeded test account, I am already verified after the migration / seed.

## Acceptance Criteria

- [x] Migration `000011_email_verification_tokens` (up/down) creates the table (see Database) and backfills `UPDATE users SET email_verified_at = now() WHERE email_verified_at IS NULL`. Down drops the table only (backfill is not reverted - documented in the down file comment).
- [x] `cmd/seed`: admin and test accounts are verified - set `EmailVerifiedAt` on create and run an idempotent `UPDATE users SET email_verified_at = now() WHERE email IN (admin, test) AND email_verified_at IS NULL` after `FirstOrCreate` (covers rows that existed before).
- [x] `config.Config.WebBaseURL` from `WEB_BASE_URL`, default `http://localhost:3000`; trailing `/` trimmed; `config.Load` errors if it is not an absolute `http`/`https` URL with a host, or if `APP_ENV=production` and `WEB_BASE_URL` is not explicitly set (same pattern as `MAIL_FROM`).
- [x] `Register` (service): after the user row is created, issues a verification token and enqueues the `verify_email` email. Any failure in token creation, rendering or enqueue is logged at `Error` (user id, never the token/URL) and **does not** fail registration; the HTTP response is unchanged except `user.email_verified: false`.
- [x] `POST /api/v1/auth/verify-email` (no auth) behaves exactly per the API table; a token can verify at most once (conditional update, race-safe); TTL 24h.
- [x] `POST /api/v1/auth/resend-verification` (authMiddleware) behaves per the API table: already verified -> 409 `already_verified` (checked before the rate limiter, does not consume quota); otherwise rate limit 1/min and 5/24h per user -> 429 `rate_limited` with `Retry-After` header and `retry_after_seconds` in body; otherwise all unused tokens of the user are deleted, a new one is created and the email enqueued -> 202. Enqueue failure here **is** an error (500 `email_send_failed`).
- [x] `email_verified: bool` (= `EmailVerifiedAt != nil`) in auth `userResponse` (`/auth/me`, `/auth/login`, `/auth/register`) **and** in users `ownerUserResponse` (`PATCH /users/me`, which the client also feeds through `normalizeUser`).
- [x] Email copy is Russian; template rendered in the api at enqueue time via `mailer.Render`; HTML part escapes the user's name.
- [x] `cmd/api` wires: `taskClient` (rename of `harvestClient`) -> `mailer.NewQueue(taskClient)`; a go-redis client from `cfg.RedisURL` (closed on shutdown) -> rate limiter; both + logger injected into auth service; new routes registered.
- [x] API client: `User.emailVerified: boolean`, `RawUser.email_verified`, `normalizeUser` maps it; `api.auth.verifyEmail(token)` and `api.auth.resendVerification()`; `ApiError` gains optional `code` and `retryAfterSeconds` parsed from the error body. Typecheck of web/admin passes.
- [x] Manual check in Mailpit: register -> email with Cyrillic subject, working link to `http://localhost:3000/verify-email?token=...`; POST of that token verifies; second POST -> `already_verified`; resend twice quickly -> 429.
- [x] `go build ./...`, `go vet ./...`, `go test ./...` (Docker) and `pnpm` typecheck/lint/test clean. Docs updated.

## Business Rules

- Token: 32 random bytes (`crypto/rand`), hex-encoded (64 chars) in the link; DB stores only `sha256` hex (reuse `hashToken` from `auth/service`). Plain token is never logged or persisted outside the email payload.
- TTL 24h from creation. One-time: `used_at` set on success.
- Token is bound to the email it was sent to (`email` column); if `users.email` differs at verify time -> `invalid_token` (future-proofing for email change; costs one column).
- verify-email needs no auth: the 256-bit token alone is the credential, so the link works in any browser/device. It does not log the user in and does not touch cookies.
- Resend invalidates old links by **deleting** the user's unused tokens (old link -> `invalid_token`). Used rows are kept so a repeated click on the already-used link answers `already_verified`.
- Rate limit (resend only): per user id, fixed windows in Redis - `rl:verify-resend:min:<uid>` (`SET NX EX 60`) and `rl:verify-resend:day:<uid>` (`INCR`, `EXPIRE 86400` when the counter becomes 1). Over limit -> 429 with remaining TTL as retry-after. Registration's own email does not count against the quota. Redis error -> 500 (enqueue would fail anyway; fail closed).
- Register does not wrap user + token in one transaction: users go through `users.Repository` and the email must not be able to fail registration anyway; a user without a token simply uses resend.
- Already-verified users: verify with any known token (even used/expired) -> 200 `already_verified` (token marked used if it was not); resend -> 409 `already_verified`.
- At-least-once delivery (from #157): duplicate emails carry the same valid link - harmless.
- Seed/test accounts and all pre-existing users are verified.

## UX

Backend only. Email (`verify_email`), Russian, e.g.:
- Subject: `Подтвердите адрес электронной почты — BookHaven`
- Body: greeting with the user's name (fallback `Здравствуйте!` when name empty), one sentence explaining why, a prominent button/link `Подтвердить email`, the raw URL as fallback text, "Ссылка действительна 24 часа.", "Если вы не регистрировались в BookHaven, просто проигнорируйте это письмо."

## API

All errors keep the existing `{"error": "<message>"}` shape and add `"code"`.

### `POST /api/v1/auth/verify-email` (public)

Body `{"token": "<64 hex>"}`.

| Case | Status | Body |
|---|---|---|
| Valid, unused, unexpired; user unverified | 200 | `{"status":"verified"}` |
| Known token (any used/expiry state), user already verified | 200 | `{"status":"already_verified"}` |
| Missing/malformed token (not 64 hex), unknown hash, email mismatch | 400 | `{"error":"invalid token","code":"invalid_token"}` |
| Unused but `expires_at <= now` | 400 | `{"error":"token expired","code":"token_expired"}` |
| `used_at` set and user not verified (defensive, normally unreachable) | 400 | `{"error":"token already used","code":"token_used"}` |

Check order: format -> lookup by hash -> email match -> user already verified (mark token used if unused, return `already_verified`) -> used (`token_used`) -> expired (`token_expired`) -> verify. Verification runs in one DB transaction: `UPDATE email_verification_tokens SET used_at=now() WHERE id=? AND used_at IS NULL` (0 rows -> re-evaluate as used) and `UPDATE users SET email_verified_at=now() WHERE id=? AND email_verified_at IS NULL`.

### `POST /api/v1/auth/resend-verification` (authMiddleware)

No body. Authorization: only acts on the authenticated user's own id from context (no user id in request).

| Case | Status | Body |
|---|---|---|
| Sent | 202 | `{"status":"sent"}` |
| Already verified | 409 | `{"error":"email already verified","code":"already_verified"}` |
| Rate limited | 429 + `Retry-After: <s>` | `{"error":"too many requests","code":"rate_limited","retry_after_seconds":<s>}` |
| Enqueue/token failure | 500 | `{"error":"failed to send verification email","code":"email_send_failed"}` |

`retry_after_seconds` is duplicated in the body because `Retry-After` is not a CORS-safelisted response header.

### User responses

`email_verified: boolean` added to `userResponse` (auth) and `ownerUserResponse` (users). Not added to public profile responses.

Swaggo annotations (Russian summaries, like existing ones) for both new endpoints; regenerate `backend/docs` the way previous features did.

## Database

`backend/migrations/000011_email_verification_tokens.up.sql`:

```sql
CREATE TABLE email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);
UPDATE users SET email_verified_at = now() WHERE email_verified_at IS NULL;
```

Down: `DROP TABLE IF EXISTS email_verification_tokens;` (backfill intentionally not reverted). Timestamps are `TIMESTAMPTZ` like `users` (not BIGINT like `refresh_tokens`). GORM model `auth/domain.EmailVerificationToken` with `TableName()`, mirroring `RefreshToken`.

## Backend

- `internal/config`: `WebBaseURL` (see AC).
- `internal/pkg/mailer`:
  - `templates/` with `//go:embed`: `layout.html.tmpl`, `verify_email.html.tmpl`, `verify_email.txt.tmpl` (`{{define "subject"}}` in the txt file).
  - `Render(name string, data any) (Message, error)` - returns Subject/TextBody/HTMLBody (caller sets `To`); `html/template` for HTML, `text/template` for text; templates parsed once (package-level, panic-free: parse errors surface in tests). Template data struct for this email: `Name string`, `VerifyURL string`, `TTLHours int`.
- `internal/pkg/ratelimit` (new, small, reused by #161): `RedisLimiter` over `*redis.Client` (go-redis v9 - already an indirect dependency via asynq; promote to direct, no new module) implementing the two fixed windows above; returns `(allowed bool, retryAfter time.Duration, err error)`. Keep key prefix/limits as params so #161 can reuse it.
- `modules/auth`:
  - `domain/tokens.go`: `EmailVerificationToken`.
  - `service`: new errors `ErrVerificationTokenInvalid`, `ErrVerificationTokenExpired`, `ErrVerificationTokenUsed`, `ErrAlreadyVerified`, `ErrRateLimited` (carrying retry-after, e.g. typed `*RateLimitError`); methods `VerifyEmail(ctx, token) (alreadyVerified bool, err error)`, `ResendVerification(ctx, userID) error`, private `issueVerification(ctx, user) error`. Small local interfaces: `Mailer` (= `mailer.Enqueuer` shape) and `ResendLimiter`. `NewService` gains `mailer`, `limiter`, `*zap.Logger`, and `Config` gains `WebBaseURL`, `VerificationTokenTTL` (24h). Use `s.db` directly for the token table, as for refresh tokens; marking the user verified is a raw `UPDATE` inside the same `s.db` transaction (no change to `users.Repository` interface).
  - `transport/http`: `VerifyEmail`, `ResendVerification` handlers, `email_verified` in `userResponse`; map service errors to the API table.
- `modules/users/transport/http`: `email_verified` in `ownerUserResponse`.
- `cmd/api/main.go`: rename `harvestClient` -> `taskClient` (still passed to admin routes), `mailer.NewQueue(taskClient)`, `redis.NewClient(&redis.Options{Addr: cfg.RedisURL})` + `defer Close()`, routes `api.POST("/auth/verify-email", ...)` and `api.POST("/auth/resend-verification", ..., authMiddleware)`.
- `cmd/seed`: verified accounts (see AC).

## Frontend

Only `packages/api-client/src/index.ts` (UI is #159):
- `User.emailVerified: boolean`, `RawUser.email_verified: boolean`, mapped in `normalizeUser`.
- `ApiError`: optional `code?: string` and `retryAfterSeconds?: number` (append as optional trailing constructor params so existing `new ApiError(...)` calls compile); `fetcher` fills them from `body.code` / `body.retry_after_seconds`.
- `api.auth.verifyEmail(token: string): Promise<{ status: 'verified' | 'already_verified' }>` and `api.auth.resendVerification(): Promise<{ status: 'sent' }>`. Add `/api/v1/auth/verify-email` (public) to `AUTH_REFRESH_EXCLUDED_PATHS`; `resend-verification` stays out of it (a 401 there should trigger the normal refresh). Export a `VerificationErrorCode` string-union type.

## Existing Patterns

- Token generation/hashing and GORM table access via `s.db`: `backend/internal/modules/auth/service/service.go` (`createTokenPair`, `hashToken`), model in `auth/domain/tokens.go`; see `docs/features/refresh-token-hash-fix.md`.
- Mailer consumer contract and template convention: `docs/features/mailer-infrastructure.md` ("How later features use this"); fake enqueuer pattern: `harvest/transport/http/handler_test.go` (`fakeEnqueuer`).
- Config validation with production guard: `internal/config/config.go` (`MAIL_FROM`, `JWT_SECRET`).
- snake_case raw type + `normalize*` camelCase mapping in api-client: `RawUser`/`normalizeUser` (lesson of #129/#131: every backend field must be mapped explicitly).
- Seed idempotency: `cmd/seed/main.go` `FirstOrCreate`.

## Tests

Backend (no real Redis/SMTP; Postgres-specific SQL avoided in service code so SQLite tests work):
- Service (in-memory SQLite + DDL for `email_verification_tokens` and a minimal `users` table, or fake users repo + raw SQLite users table - pick what lets the verify transaction's `UPDATE users` run; fake mailer recording messages; fake limiter):
  - Register: token row created with hash != plain, `expires_at` ~ +24h, one message to the user's email whose text contains `/verify-email?token=<plain>` and hash(plain) == stored hash; enqueue error -> Register still returns user, no error; token-insert error -> same.
  - VerifyEmail: success sets `used_at` and `email_verified_at`; second call -> already_verified; expired -> `ErrVerificationTokenExpired`; unknown/malformed -> invalid; email mismatch -> invalid; verified user + fresh token -> already_verified and token marked used; verified user + expired token -> already_verified.
  - ResendVerification: verified -> `ErrAlreadyVerified`, limiter not called; limiter denies -> `ErrRateLimited` with retry-after, nothing enqueued; success deletes old unused tokens (old token -> invalid) and enqueues one message; enqueue error -> error returned.
- Handlers: status codes + `code` for each row of both API tables; 429 sets `Retry-After` header and body field; `/auth/me` includes `email_verified`.
- `ratelimit.RedisLimiter`: use `github.com/alicebob/miniredis/v2` as a **test-only** dependency if acceptable; otherwise test against an interface fake and cover the Redis implementation by the manual check. Cases: first call allowed; second within a minute denied with retry-after <= 60s; 6th call within a day denied (advance miniredis time).
- `mailer.Render`: `verify_email` renders subject (Cyrillic), text contains URL, HTML escapes `<script>` in name, unknown template -> error.
- `config.Load`: default `WEB_BASE_URL`; invalid URL -> error; production without explicit `WEB_BASE_URL` -> error; trailing slash trimmed.
- Seed: extend `cmd/seed/main_test.go` only if it already tests `seedAccounts` against a DB; otherwise covered by manual check.
- Frontend: api-client typecheck; if the package has tests, `normalizeUser` maps `email_verified` and `fetcher` populates `ApiError.code`.

## Documentation

- `docs/ROADMAP.md`: move #157 to "Completed" (link `mailer-infrastructure.md`); mark #158 in progress with a link to this contract; note #158 merges to `main` together with #159.
- `docs/DEPLOYMENT.md`: `backend` (api) env `WEB_BASE_URL` (set in `docker-compose.prod.yml` as `https://${WEB_DOMAIN}`, no new `.env` key); explicit warning that real SMTP + SPF/DKIM/DMARC must be configured on the worker **before** #158/#159 go live, otherwise new users never receive the link (emails only logged); note that api already uses Redis (now also for rate limiting).
- `docker-compose.prod.yml`: `backend.environment.WEB_BASE_URL: https://${WEB_DOMAIN}`. `docker-compose.yml`: optional `WEB_BASE_URL: ${WEB_BASE_URL:-http://localhost:3000}` on `backend`. `.env.example`/`.env.production.example`: document `WEB_BASE_URL` (dev override only).
- `docs/features/mailer-infrastructure.md`: one line that the template convention is now implemented (`mailer.Render`).
- Swagger regenerated for the two endpoints and new response field.
- Set this contract's Status to `implemented` when done.

## Implementation Notes

- Optional cheap fix from #157 review: in `SMTPSender.Send`, when `User != ""`, port != 465, STARTTLS not advertised, and host is not localhost, return an error wrapping `asynq.SkipRetry` (or a sentinel the handler maps to SkipRetry) - it is a misconfiguration that retries cannot fix. Add one `smtp_test.go` case with the existing fake server (no STARTTLS, non-localhost cannot be simulated easily -> test the decision via a small pure helper). Skip if it grows beyond ~20 lines.
- Build the link with `url.Values{"token": {plain}}` on `WebBaseURL + "/verify-email"`.
- Do not log request bodies for `/auth/verify-email` (current request logger logs only method/URI/status - fine; token is in the body, not the URI).

## Risks

- **Emails silently not delivered in prod** if `SMTP_HOST` is unset on the worker (LogSender). With #160 this would lock users out; mitigated by the DEPLOYMENT warning. Making `SMTP_HOST` mandatory is not possible from the api (it does not see SMTP vars) - left to the worker's startup `Warn`.
- **Link before #159 exists**: merging to `main` alone sends links to a 404 page - merge together with #159 (stated in the issue).
- **Token in Redis payload / Mailpit / dev logs**: archived failed tasks keep the token until trimmed; tokens expire in 24h. Acceptable (see #157).
- **Rate limit fixed windows** allow small bursts at window edges; acceptable for this purpose.
- **Token table growth**: used/expired rows are never purged. Low volume; add a cleanup later (tech-debt note).
- **Constructor change** of `authservice.NewService` touches `cmd/api` and the auth service test helper only.
- **miniredis** would be a new test-only dependency - implementer should flag it rather than add silently if repo policy forbids.
- **Registration enumeration / abuse**: Register already reveals existing emails (409); anyone can trigger one email per registration to an arbitrary address - same as any signup flow; no extra limit on Register in this scope.

## Implementation Notes (as built)

- Verification token model: `auth/domain.EmailVerificationToken`; service logic in `auth/service/service.go` (`VerifyEmail`, `ResendVerification`, `issueVerification`); handlers in `auth/transport/http/handler.go`. The user is read through `users.Repository.GetByID`; the token row and the conditional `UPDATE users` run on `s.db` (one transaction for verify). Timestamps are bound from Go (`time.Now()`), not `now()`, so the service SQL also runs on SQLite in tests.
- `internal/pkg/ratelimit.RedisLimiter` (`Options{Prefix, MinInterval, WindowLimit, Window}`): before the minute `SET NX` it reads the window counter, so a user over the daily quota always gets the long (daily) retry-after and a minute-window denial never consumes daily quota. A key that lost its TTL is repaired on read.
- `mailer.Render` (`render.go`) + `templates/{layout.html.tmpl, verify_email.html.tmpl, verify_email.txt.tmpl}`; template data type `mailer.VerifyEmailData`. Templates are parsed once (`sync.Once`); parse errors are returned from `Render` (and asserted in tests), never panic.
- `config.Load` is shared by api, worker, harvest, seed and mailtest, so the production guard on `WEB_BASE_URL` also applies to the worker: `docker-compose.prod.yml` sets `WEB_BASE_URL: https://${WEB_DOMAIN}` on both `backend` and `worker`.
- Optional SMTP fix included: `SMTPSender.Send` returns `mailer.ErrInsecureAuth` when credentials are set, the port is not 465, STARTTLS was not negotiated and the host is not loopback; `mailer.Handler` maps it to `asynq.SkipRetry`. Decision covered by `isInsecureAuth` unit test.
- Test-only dependency: `github.com/alicebob/miniredis/v2` (ratelimit tests). `github.com/redis/go-redis/v9` promoted from indirect to direct (same version).
- Swagger regenerated with `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`; it had been stale, so the regeneration also picks up endpoints from earlier features (admin harvest/import/merge, notifications, public shelves).
