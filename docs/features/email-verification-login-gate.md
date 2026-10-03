# Email Verification Login Gate

Status: implemented

GitHub issue: #160 "Вход только после подтверждения email" (epic #162). Depends on #158 ([email-verification-backend.md](email-verification-backend.md)) and #159 ([email-verification-web.md](email-verification-web.md)), both merged to `main`. Branch `feat/unverified-user-restrictions` from `main`. Supersedes the earlier selective-restrictions plan (rejected by the owner).

## Goal

A user whose email is not verified cannot authenticate at all. Registration creates the account and sends the verification email but does not log the user in; login, token refresh and every authenticated request reject unverified users with the machine-readable code `email_not_verified`. Until verified the person uses the site as a guest. A verification email can be re-requested without a session by entering the email address.

All pre-existing users and seed accounts were backfilled as verified by #158, so this only affects users registered after #158 went live.

## Scope boundaries

**In scope:** register without session, login/refresh/auth-middleware gate, public enumeration-safe resend with per-email + per-IP rate limits, real-client-IP extraction, api-client changes, web register/login/verify flows, removal of the unverified banner, admin app compile + graceful message, tests, Swagger, docs.

**Out of scope:** auto-login after verification, email prefill on `/login` after verification, password reset (#161, see note), changing the registration 409 (it already reveals existing emails), cleanup of unverified accounts, any change to `/auth/verify-email` semantics.

## User Stories

- As a new user, after registering I see "Проверьте почту" and am not logged in; after clicking the link I log in normally.
- As an unverified user who tries to log in with the correct password, I am told my email is not confirmed and can resend the link right there.
- As someone who lost the email (or whose link expired), I can request a new one by entering my email - from the login screen or the verify-email error page - without being logged in.
- As the platform, nobody without a confirmed address holds a session, and the public resend endpoint does not reveal which emails are registered.
- As a user who already had a session issued between the #158 deploy and this one while unverified, my session stops working and I land in the guest state (no redirect loop).

## Acceptance Criteria

- [x] `POST /auth/register` creates the user and issues the verification email exactly as today, sets **no** cookies and returns `201 {"email":"<email>","verification_required":true}` (no tokens, no user object). 400/409/500 unchanged.
- [x] `POST /auth/login`: unknown email / wrong password / no password -> unchanged `401 invalid credentials`. Correct password + `email_verified_at IS NULL` -> `403 {"error":"email not verified","code":"email_not_verified"}`, no tokens created, no cookies set. Password is always checked first (no enumeration).
- [x] `POST /auth/refresh`: if the token's user is unverified -> revoke **all** of the user's refresh tokens, clear auth cookies, `401 {"error":"email not verified","code":"email_not_verified"}`. Otherwise unchanged.
- [x] `middleware.Auth`: loaded user with `EmailVerifiedAt == nil` -> `401 {"error":"email not verified","code":"email_not_verified"}`. `middleware.OptionalAuth`: unverified user is treated as anonymous (no `user`/`user_id` set). No extra queries (user is already loaded).
- [x] `POST /auth/resend-verification` is **public**, body `{"email": "..."}`; behaves per the API table; the authenticated variant is removed (same path, no `authMiddleware`).
- [x] `cmd/api` sets `e.IPExtractor = echo.ExtractIPFromXFFHeader()` so `c.RealIP()` is the client IP behind Caddy and not spoofable when called directly.
- [x] `POST /auth/verify-email` unchanged.
- [x] Web: register shows the check-email panel and leaves the session store logged out; login with 403 `email_not_verified` shows the not-verified panel with a working resend for the typed email; `/verify-email` has no session-dependent branches; the site-wide banner widget is deleted.
- [x] Admin app compiles; login with an unverified account shows an error without crashing (generic text acceptable).
- [x] `go build/vet/test ./...` (Docker) and `pnpm` typecheck/lint (incl. FSD boundaries)/test clean for web, admin and api-client. Swagger regenerated.

## Business Rules

- "Authenticated" now means "valid token **and** verified email". There is no state where an unverified user holds a usable session.
- Login order: lookup -> password check -> verification check -> token creation. Only someone who knows the password learns that the account is unverified.
- Refresh rejection revokes all refresh tokens of the unverified user (same as `Logout`) - closes sessions issued between the #158 deploy and this one. Access tokens die on the next request via the middleware (they are not revocable, but every authenticated request loads the user).
- Status code for stale sessions is **401** (not 403) on both the middleware and refresh: semantically "not authenticated", existing callers already treat 401 as logged out, and the api-client's reactive refresh retries at most once (`isRetry`), so the sequence is `request 401 -> refresh 401 -> surface 401` - no loop. Refresh also clears cookies (incl. `has_session`), so the Next middleware stops treating the browser as "maybe logged in" and later page loads do not repeat the refresh attempt.
- Resend (public):
  - Response is `202 {"status":"sent"}` whenever the request is well-formed and not rate-limited - for an unverified user (email sent), an unknown email, an already verified user, or an account without password (nothing sent). No 409 anymore.
  - Rate limits are applied **before** the user lookup and keyed only on request data, so a 429 never depends on whether the account exists:
    - per IP: `rl:verify-resend-ip`, `MinInterval: 0`, `WindowLimit: 20`, `Window: 1h` (blunts scanning many addresses from one IP);
    - per email: `rl:verify-resend-email`, `MinInterval: 1m`, `WindowLimit: 5`, `Window: 24h` (same limits as today, now per address). Key = `sha256(lowercase(trim(email)))` hex - no plain addresses in Redis.
    - Order: per-IP, then per-email (an IP-limited request does not consume the email quota). Registration's own email still does not count.
  - Limiter (Redis) error -> 500 `email_send_failed` before the lookup (existence-independent). Token/enqueue failure **after** the lookup -> logged at `Error` (user id only) and still `202`, so a 500 cannot single out existing unverified accounts. Accepted trade-off: rare silent failure; the user can retry after the cool-down.
  - Lookup uses the email exactly as given (same as `Login`/`Register`, which do not normalise case); only the rate-limit key is normalised.
  - Enumeration safety is best-effort consistency, not a hard guarantee: `POST /auth/register` already returns 409 for existing emails, and response timing differs slightly (DB writes + enqueue only for real unverified users). Accepted.
- `verify-email`: unchanged; it does not log the user in. **No auto-login** after verification: the link may be opened on another device or by someone else, and auto-login would turn the emailed token into a login credential (bigger blast radius if the email leaks). The success screen offers "Войти".
- Note for #161 (password reset, not implemented here): completing a reset through an emailed link proves mailbox ownership, so the reset flow may also set `email_verified_at` if NULL - record this in the #161 contract.
- Existing #158 behaviour that resend deletes the user's unused tokens (old links -> `invalid_token`) stays.

## UX

All copy Russian; reuse existing card layouts and `@bookhaven/ui` `Button`.

### Register (`/register`)

Unchanged form. On success the in-place "Проверьте почту" panel (existing `check-email-panel.tsx`) shows: "Мы отправили письмо со ссылкой для подтверждения на **{email}**. Ссылка действительна 24 часа. Войти можно после подтверждения." + resend button (for that email) + primary `Button asChild` -> `Link` "Войти" to `/login` (preserve a safe `redirect` param: `/login?redirect=<safe>`). Remove the "Продолжить" link and the "Подтвердить email можно позже…" note. The session store stays logged out.

### Login (`/login`)

On 403 `email_not_verified` show, above the form (the form stays usable so the user can log in again after verifying), a neutral panel (`rounded-md border bg-secondary p-3 text-sm`, `role="status"`): "Email **{email}** не подтверждён. Перейдите по ссылке из письма, которое мы отправили при регистрации, и войдите снова." + resend button for that email. The generic red error is not shown in this case. Other errors unchanged.

### `/verify-email`

States and copy as in #159, minus all session branches:
- `verified` / `already_verified`: "Войти" (`/login`) + "К каталогу" (`/books`).
- `token_expired` / `invalid_token` / `token_used` / `missing_token`: resend **form** - email input (`type="email"`, required) + "Отправить письмо ещё раз".
- `error`: "Попробовать снова" (unchanged).

### Resend feedback (shared)

- sending: "Отправляем…", disabled.
- sent: "Если этот адрес зарегистрирован и не подтверждён, мы отправили на него новое письмо." Button disabled 60 s.
- rate_limited: unchanged ("Слишком много запросов. Попробуйте снова через N сек/мин/ч.").
- error: unchanged. (`already_verified` state removed.)

### Banner

Removed (no logged-in unverified users can exist).

## API

All errors keep `{"error","code"?}`.

### `POST /api/v1/auth/register` (public)

| Case | Status | Body |
|---|---|---|
| Created (email enqueue failure is still logged and ignored, as in #158) | 201 | `{"email":"a@b.c","verification_required":true}` |
| Validation | 400 | unchanged |
| Exists | 409 | unchanged |

No `Set-Cookie`. Handler no longer calls `service.Login`.

### `POST /api/v1/auth/login` (public)

| Case | Status | Body |
|---|---|---|
| Unknown email / wrong password / no password hash | 401 | `{"error":"invalid credentials"}` (unchanged) |
| Correct password, unverified | 403 | `{"error":"email not verified","code":"email_not_verified"}` |
| OK | 200 | unchanged `authResponse` + cookies |

### `POST /api/v1/auth/refresh` (public)

| Case | Status | Body |
|---|---|---|
| Unverified user | 401 + cookies cleared | `{"error":"email not verified","code":"email_not_verified"}` |
| Other cases | unchanged | unchanged |

### Authenticated endpoints (via `middleware.Auth`)

Unverified user -> 401 `{"error":"email not verified","code":"email_not_verified"}`.

### `POST /api/v1/auth/resend-verification` (public, was authenticated)

Body `{"email": "<email>"}` (validated `required,email`).

| Case | Status | Body |
|---|---|---|
| Malformed body / invalid email | 400 | `{"error":"invalid email","code":"invalid_email"}` |
| Per-IP or per-email limit hit | 429 + `Retry-After` | `{"error":"too many requests","code":"rate_limited","retry_after_seconds":N}` (unchanged shape) |
| Limiter error | 500 | `{"error":"failed to send verification email","code":"email_send_failed"}` |
| Anything else (sent, unknown, verified, no password, send failure after lookup) | 202 | `{"status":"sent"}` |

`409 already_verified` no longer exists. Swagger: update annotations of register, login, refresh, resend (Russian summaries), regenerate as in #158 (`swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`).

## Database

No changes.

## Backend

- `modules/auth/service/service.go`:
  - New `ErrEmailNotVerified`.
  - `Login`: after `bcrypt` succeeds, `if user.EmailVerifiedAt == nil { return user, nil, ErrEmailNotVerified }` (no `createTokenPair`).
  - `RefreshTokens`: after the token row is found, load `s.users.GetByID(token.UserID)`; if unverified -> `s.Logout(ctx, token.UserID)` (revoke all) and return `ErrEmailNotVerified`; otherwise existing revoke + rotate.
  - `ResendVerification(ctx, email, ip string) error` replaces `ResendVerification(ctx, userID)`: IP limiter -> email limiter (hashed normalised key) -> `GetByEmail` (not found -> nil) -> verified or `PasswordHash == nil` -> nil -> delete unused tokens + `issueVerification`; errors after the lookup are logged and swallowed (return nil). Return `*RateLimitError` for either limiter (larger retry-after if both would deny is unnecessary - first denial wins). Limiter errors returned as-is (handler -> 500).
  - Constructor: replace the single `limiter ResendLimiter` with two (`ipLimiter`, `emailLimiter`) - e.g. a small `ResendLimiters{IP, Email ResendLimiter}` struct param. Touches `cmd/api` and the service test helper only. Remove the now-unused `ErrAlreadyVerified` resend path (keep the var only if something else uses it).
- `modules/auth/transport/http/handler.go`:
  - `Register`: drop auto-login and `setAuthCookies`; respond with `registerResponse{Email, VerificationRequired: true}`.
  - `Login`: map `ErrEmailNotVerified` -> 403 with code; never set cookies on error.
  - `Refresh`: map `ErrEmailNotVerified` -> `clearAuthCookies(c)` + 401 with code; other errors unchanged.
  - `ResendVerification`: bind `resendRequest{Email string validate:"required,email"}`, call service with `c.RealIP()`; map per the API table. No `user_id` from context.
  - New const `codeEmailNotVerified = "email_not_verified"`, `codeInvalidEmail = "invalid_email"`.
- `internal/transport/http/middleware/auth.go`: `Auth` returns the 401 with code when `user.EmailVerifiedAt == nil`; `OptionalAuth` skips setting the user. (Middleware may define its own copy of the code string to avoid importing the auth transport.)
- `cmd/api/main.go`: `e.IPExtractor = echo.ExtractIPFromXFFHeader()` (default trust: loopback, link-local, private nets - Caddy reaches the api over the private Docker network and, as Caddy v2 does not trust client-supplied `X-Forwarded-For` by default, the header holds the real client IP; direct callers cannot spoof it because only trusted-proxy hops are honoured); two `ratelimit.NewRedisLimiter` instances with the options above; route `api.POST("/auth/resend-verification", authHandler.ResendVerification)` **without** `authMiddleware`.
- `cmd/seed`: no change (accounts already verified).

## Frontend

### `packages/api-client/src/index.ts`

- `RegisterResponse { email: string; verificationRequired: true }` (raw `{email, verification_required}`); `api.auth.register` returns it (no `normalizeAuthResponse`). `AuthResponse` stays for login.
- `api.auth.resendVerification(email: string)` -> POST with `{ email }`; update the doc comment (codes: `invalid_email`, `rate_limited` + `retryAfterSeconds`, `email_send_failed`).
- Add `/api/v1/auth/resend-verification` to `AUTH_REFRESH_EXCLUDED_PATHS` (public now) and update that comment.
- Export `EMAIL_NOT_VERIFIED_CODE = 'email_not_verified'` and `isEmailNotVerifiedError(err: unknown): boolean` (`isApiError(err) && err.code === EMAIL_NOT_VERIFIED_CODE`) so web and admin share it.
- Update `VerificationErrorCode` union if it lists `already_verified` for resend.

### `apps/web/src`

FSD: `features/auth` cannot import `features/resend-verification`; composition happens in `views` (same pattern #159 used for the check-email panel).

- `entities/session/model/auth-store.ts`: `register` calls the API and returns `{ email }`; it does **not** set `user`/`isAuthenticated` or schedule a refresh. `login`: when `isEmailNotVerifiedError(error)`, do not set `error` (rethrow only) - the view shows its own panel.
- `features/auth/ui/login-form.tsx`: new optional prop `onEmailNotVerified?: (email: string) => void`; in `catch`, if `isEmailNotVerifiedError(err)` call it with the typed email (if the prop is absent, show inline text "Email не подтверждён. Проверьте почту." so the form is never silent).
- `features/auth/ui/register-form.tsx`: without `onRegistered` it must no longer push to `/profile` (no session) - push to `/login` instead (safe `redirect` preserved). `onRegistered` path unchanged.
- `views/login/ui/login-view.tsx`: holds `unverifiedEmail` state; renders the not-verified panel (new `views/login/ui/email-not-verified-panel.tsx` using `ResendVerificationButton email={unverifiedEmail}`) above `LoginForm onEmailNotVerified={setUnverifiedEmail}`; reset on next submit via the callback being called again / cleared when login succeeds (navigation).
- `views/register/ui/check-email-panel.tsx`: copy per UX; `ResendVerificationButton email={email}`; "Войти" link to `/login` (+ safe redirect).
- `features/resend-verification/`:
  - `model/resend-state.ts`: remove `already_verified` kind and 409 mapping; new `sent` message (UX).
  - `model/use-resend-verification.ts`: `resend(email: string)`; no `fetchUser`, no `useAuthStore` import.
  - `ui/resend-verification-button.tsx`: required `email: string` prop.
  - `ui/resend-verification-form.tsx` (new): email input + button using the same hook; client-side check "non-empty, contains @" only (backend validates).
  - `index.ts`: export the form.
- `features/verify-email/model/use-verify-email.ts`: drop the `fetchUser()` call (no session to refresh); StrictMode ref guard unchanged.
- `views/verify-email/ui/verify-email-view.tsx`: remove `useAuthStore` usage and all logged-in/logged-out branches; success -> "Войти" + "К каталогу"; failure -> `ResendVerificationForm`.
- Delete `widgets/email-verification-banner/` (incl. test) and its import/render in `app/layout.tsx`.
- `middleware.ts`: no change (`/verify-email` stays public; `/login`, `/register` stay auth routes).

### `apps/admin`

Only uses `api.auth.login`. Its store sets `error` from the message; a 403 shows "email not verified" - acceptable. Optional one-liner: map `isEmailNotVerifiedError` to "Email не подтверждён. Подтвердите его по ссылке из письма." Must compile with the api-client changes.

## Existing Patterns

- Error body with `code`, 429 mapping with `Retry-After`: `auth/transport/http/handler.go` (#158).
- `ratelimit.NewRedisLimiter(client, Options{...})` wiring: `cmd/api/main.go` (`verifyResendLimiter`); tests with miniredis: `internal/pkg/ratelimit`.
- Middleware tests: `internal/transport/http/middleware/auth_test.go`.
- Auth handler/service tests incl. verification: `auth/transport/http/verification_test.go`, auth service tests (SQLite + fakes).
- View composes `features/auth` with another feature via a callback prop: `views/register` + `RegisterForm onRegistered` (#159).
- Web test mocking of `ApiError`: `views/verify-email/ui/verify-email-view.test.tsx`.

## Tests

Backend (Docker `go test ./...`):
- Service `Login`: unverified + correct password -> `ErrEmailNotVerified` and no refresh-token row created; unverified + wrong password -> `ErrInvalidCredentials`; verified -> tokens.
- Service `RefreshTokens`: unverified user's valid token -> `ErrEmailNotVerified` and **all** the user's refresh tokens revoked; verified -> rotation unchanged.
- Service `ResendVerification` (fake limiters + fake mailer): unknown email -> nil, nothing sent, both limiters consulted; verified -> nil, nothing sent; unverified -> old unused tokens deleted, one email; IP limiter denies -> `RateLimitError`, email limiter not called, no lookup; email limiter denies -> `RateLimitError`, no lookup; email key is the hash of the lowercased/trimmed email (two casings share a key); limiter error -> error; enqueue error -> nil (logged).
- Service `Register`: unchanged tests still pass.
- Handlers: register -> 201 body `{email, verification_required:true}` and **no** `Set-Cookie`; login unverified -> 403 code and no `Set-Cookie`; login wrong password for unverified -> 401 without code; refresh unverified -> 401 code + cookies cleared (`MaxAge -1`); resend: 400 `invalid_email`, 202 for unknown/verified/unverified, 429 with header and body, 500 on limiter error; resend works without auth.
- Middleware: `Auth` with unverified user -> 401 `email_not_verified`, next not called; verified -> next; `OptionalAuth` with unverified -> next called with no `user` in context.
- Existing tests that register-then-use-tokens (if any) updated to verify the user first.

Frontend (Vitest):
- api-client (if it has tests): `register` maps `verification_required`; `resendVerification(email)` sends the body; `isEmailNotVerifiedError`.
- `entities/session/model/auth-store.test.ts`: `register` leaves `isAuthenticated: false`, `user: null`, no refresh scheduled; `login` with 403 code rethrows and leaves `error` null.
- `features/auth/ui/login-form.test.tsx` (new or extended): 403 code -> `onEmailNotVerified` called with the typed email, no generic error; without the prop -> inline text.
- `views/login/ui/login-view.test.tsx` (new): 403 -> panel with email and resend button; resend -> `api.auth.resendVerification` called with that email; success message.
- `features/resend-verification/model/resend-state.test.ts`: remove 409 cases; 202 -> new sent text.
- `views/register/ui/register-view.test.tsx` + `register-form.test.tsx`: panel shows "Войти" link (with safe redirect); resend uses the registered email; store not authenticated; form without `onRegistered` pushes `/login`.
- `views/verify-email/ui/verify-email-view.test.tsx`: rewrite - no session mocks; success -> "Войти"; failure -> email form submits `resendVerification(email)`.
- `features/verify-email/model/use-verify-email.test.tsx`: drop `fetchUser` expectations.
- Delete `widgets/email-verification-banner/ui/email-verification-banner.test.tsx`.
- Manual (Mailpit): register -> panel, not logged in (header shows guest); login before verifying -> panel, resend works, quick second resend -> 429; resend from `/verify-email` with an unknown email -> same "Если этот адрес…" message, no email in Mailpit; click link -> "Email подтверждён" -> "Войти" -> login works; a pre-existing stale unverified session (simulate: log in a verified user, `UPDATE users SET email_verified_at = NULL`) -> next request falls back to guest without loops; seed admin logs into admin app.

## Documentation

- `docs/ROADMAP.md`: #160 entry renamed to "Вход только после подтверждения email" (English line consistent with the file), link to this contract, "in progress". #158/#159 are already under Completed.
- `docs/DEPLOYMENT.md`: in the email/SMTP section add that unverified users **cannot log in at all** since #160, so real SMTP with SPF/DKIM/DMARC must be working before go-live; note that the api relies on `X-Forwarded-For` from Caddy for per-IP rate limiting (do not put another proxy in front without updating `IPExtractor` trust).
- `docs/features/email-verification-web.md`: one line under the header that the banner and logged-in branches were removed by #160 (link this contract). `email-verification-backend.md`: one line that resend became public and register no longer logs in (link).
- Swagger regenerated.
- This contract: `Status: implemented` + "Implementation Notes (as built)" when done.

## Risks

- **Lockout if email delivery is broken** in prod: new users can never log in. Mitigation: DEPLOYMENT warning; worker logs; manual Mailpit/real-SMTP check before go-live.
- **Behaviour change for API consumers** of `register` (no tokens) and `resend-verification` (public, body required): only web/admin use them; admin does not call either.
- **IP extraction misconfiguration**: if the trust settings are wrong, all requests may share Caddy's IP (IP limit hits everyone after 20/h) or the header becomes spoofable. Covered by the explicit `IPExtractor` choice and DEPLOYMENT note; verify in the manual check by reading the logged IP if convenient.
- **Shared NAT / office IPs** could hit the per-IP resend limit (20/h). Acceptable; tune later.
- **Silent send failures** after lookup return 202 (enumeration safety). Logged at `Error`; user can retry after the cool-down.
- **Partial enumeration** via register 409 and timing remains (pre-existing, documented).
- **Unverified users who registered after #158 and kept a session** are logged out on deploy; they log in again after verifying. Intended.
- **Constructor change** of `authservice.NewService` (two limiters) touches `cmd/api` and the service test helper.

## Implementation Notes (as built)

Sub-issues: #177 (backend), #178 (frontend), epic #176.

Backend:
- `auth/service`: `ErrEmailNotVerified`; `ErrAlreadyVerified` removed (no other users). `Login` returns `(user, nil, ErrEmailNotVerified)` after a successful bcrypt check. `RefreshTokens` loads the token's user (missing user -> `ErrTokenInvalid`); unverified -> `Logout` (revoke all) + `ErrEmailNotVerified`. `NewService(..., limiters ResendLimiters, logger)` with `ResendLimiters{IP, Email}`. `ResendVerification(ctx, email, ip)`: IP limiter -> email limiter (key `resendEmailKey` = `hashToken(lower(trim(email)))`) -> `GetByEmail`; lookup errors other than not-found, token deletion and issue failures are logged at `Error` (user id only) and return nil.
- `auth/transport/http`: `registerResponse{email, verification_required}`; `resendVerificationRequest{email}` (bind/validate errors -> 400 `invalid_email`, before any limiter); login 403 / refresh 401 + `clearAuthCookies` with `email_not_verified`; `codeAlreadyVerified` removed.
- `middleware.Auth` -> 401 `{"error":"email not verified","code":"email_not_verified"}` (own const); `OptionalAuth` sets nothing for unverified users.
- `cmd/api`: `e.IPExtractor = echo.ExtractIPFromXFFHeader()`; limiters `rl:verify-resend-ip` (0 / 20 per 1h) and `rl:verify-resend-email` (1m / 5 per 24h); resend route without `authMiddleware`. The old `rl:verify-resend:*` keys simply expire.
- Swagger regenerated (`swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`).
- Tests: `auth/service/login_gate_test.go` (login/refresh gate, public resend incl. limiter order, key normalisation, swallowed enqueue error), `auth/transport/http/login_gate_test.go` (register/login/refresh/resend handlers, `RealIP` used as the IP key), `middleware/auth_gate_test.go`. Existing refresh tests now use a verified fixture user; the old user-id resend tests were replaced.

Frontend:
- `packages/api-client`: `RegisterResponse`, `register` returns it; `resendVerification(email)`; resend path in `AUTH_REFRESH_EXCLUDED_PATHS`; `EMAIL_NOT_VERIFIED_CODE`, `isEmailNotVerifiedError`; `VerificationErrorCode` drops `already_verified`, adds `invalid_email`.
- `apps/web`: new `loginHrefWithRedirect` in `shared/lib/safe-redirect.ts` (used by `RegisterForm` without `onRegistered` and by the check-email panel). `views/login` is now a client component holding `unverifiedEmail`; the panel (`email-not-verified-panel.tsx`) is keyed by email so the resend cool-down resets per address. `ResendVerificationForm` lives in `features/resend-verification/ui/`. `widgets/email-verification-banner/` deleted.
- `apps/admin`: login maps `isEmailNotVerifiedError` to "Email не подтверждён. Подтвердите его по ссылке из письма." (+ store tests).

Verification: Docker `go build/vet/test ./...`; web `tsc --noEmit`, `next lint`, `vitest run`; admin `tsc`/`next lint`/`vitest` inside the running `bookhaven-admin` container (the host `apps/admin/node_modules` symlinks are broken locally); api-client `tsc --noEmit`. Manual API-level check against the local stack + Mailpit: register 201 without `Set-Cookie`; login unverified 403 / wrong password 401; resend unknown email 202 with no mail; second resend (different casing) 429 `Retry-After: 60`; the old link after a resend -> `invalid_token`, new link -> `verified`; login 200; after `UPDATE users SET email_verified_at = NULL`: `/auth/me` 401 `email_not_verified`, public `GET /books` with the token 200 (guest), refresh 401 with all three cookies cleared and 0 active refresh tokens.
