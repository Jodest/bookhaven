# Password Reset by Email

Status: implemented

GitHub issue: #161 "Сброс пароля по email" (epic #181). Depends on #157 ([mailer-infrastructure.md](mailer-infrastructure.md)), reuses the one-time token approach of #158 ([email-verification-backend.md](email-verification-backend.md)) and the public enumeration-safe endpoint pattern of #160 ([email-verification-login-gate.md](email-verification-login-gate.md)). All three are merged to `main`. Branch `feat/password-reset` from `main`.

## Goal

A user who forgot the password enters the email on `/forgot-password` and receives (asynchronously) a Russian email with a one-time link `<WEB_BASE_URL>/reset-password?token=...`, valid for 1 hour. On that page the user sets a new password; all existing sessions are revoked and the user logs in with the new password. The public endpoint never reveals whether an email is registered. Completing a reset also verifies the email (the link proves mailbox ownership), which unblocks unverified users who lost the verification email.

## Scope boundaries

**In scope:** `password_reset_tokens` table, `POST /auth/forgot-password`, `POST /auth/reset-password`, per-IP + per-email Redis limits, `reset_password` email template, api-client methods, web `/forgot-password` and `/reset-password` pages, "Забыли пароль?" link on `/login`, tests, Swagger, docs.

**Out of scope:** admin app (no link or pages; admins use the web flow - same accounts, same `/auth/login`), change-password for logged-in users, auto-login after reset, setting a password for accounts without one (OAuth - no such accounts are created today), purging old token rows, revoking already-issued access JWTs (not revocable; they die within 15 min - same as `Logout`), password strength rules beyond the existing register rule.

## User Stories

- As a user who forgot my password, I click "Забыли пароль?" on the login page, enter my email and get a link that lets me set a new password.
- As that user, after setting a new password I see "Пароль изменён" and log in with it; any session someone else had on my account stops working.
- As a user whose link expired / was already used / was superseded by a newer one, I see a clear message and can request a new link.
- As an unverified user who lost the verification email, resetting my password through the emailed link also confirms my email, so I can log in.
- As the platform, the public endpoint does not reveal who is registered and cannot be used to spam a mailbox or scan addresses.

## Acceptance Criteria

- [x] Migration `000012_password_reset_tokens` (up/down) creates the table (see Database). Down drops it.
- [x] `POST /api/v1/auth/forgot-password` (public) behaves per the API table: always `202 {"status":"sent"}` for a well-formed, non-limited request regardless of whether the email exists; per-IP then per-email limits checked **before** the user lookup.
- [x] For an existing account with a password (verified **or** unverified) the user's unused reset tokens are deleted, a new token is created (TTL 1h) and the `reset_password` email is enqueued to `user.Email`. Unknown email / no password -> nothing sent. Errors after the lookup are logged (user id only) and still 202.
- [x] `POST /api/v1/auth/reset-password` (public) behaves per the API table; codes `invalid_token` / `token_expired` / `token_used` like verify-email, plus `invalid_password`.
- [x] On success, in **one transaction**: token marked used (conditional `UPDATE ... WHERE used_at IS NULL`, 0 rows -> `token_used`), `users.password_hash` updated (bcrypt, `DefaultCost`), `email_verified_at` set if NULL, all the user's other unused reset tokens deleted, all the user's refresh tokens revoked. Response clears auth cookies. No tokens issued.
- [x] Old password stops working, new password logs in; old refresh tokens -> 401 on `/auth/refresh`.
- [x] Reset link email is Russian, mentions 1-hour validity and "если вы не запрашивали - проигнорируйте"; HTML escapes the name.
- [x] Web: "Забыли пароль?" link on `/login`; `/forgot-password` with neutral confirmation and 429 handling; `/reset-password?token=...` with states form / success / invalid / expired / used / missing token. Both pages `noindex` + `referrer: no-referrer`, both public in `middleware.ts`.
- [x] api-client: `forgotPassword(email)`, `resetPassword(token, password)`; both paths in `AUTH_REFRESH_EXCLUDED_PATHS`.
- [x] Docker `go build/vet/test ./...`; web + api-client typecheck/lint (incl. FSD boundaries)/test clean; admin still compiles. Swagger regenerated.

## Business Rules

- **Token:** identical to verification tokens - 32 bytes `crypto/rand`, hex (64 chars) in the link, only `sha256` hex stored (`hashToken`), bound to the email it was sent to (`email` column; mismatch with `users.email` at reset time -> `invalid_token`). Plain token is never logged or persisted outside the email payload.
- **TTL:** 1 hour from creation (service constant `passwordResetTokenTTL = time.Hour`; not configurable - the email text says "1 час").
- **Single use:** `used_at` set inside the reset transaction via conditional update; a concurrent second request gets `token_used`.
- **Only the newest link works:** issuing a new token deletes the user's unused reset tokens (old link -> `invalid_token`); a successful reset deletes any remaining unused ones. Used rows are kept so a repeated click on a used link answers `token_used`.
- **Who gets an email:** any account with `password_hash IS NOT NULL`, including **unverified** accounts. Rationale: resetting through the emailed link proves mailbox ownership exactly like verify-email does, so it is the natural recovery path for an unverified user who also lost the password. Accounts without a password: nothing sent (same as resend-verification).
- **Reset verifies the email:** `UPDATE users SET password_hash=?, email_verified_at=COALESCE(email_verified_at, ?), updated_at=? WHERE id=?`. Owner decision context: #160 blocks unverified logins, and the #160 contract explicitly anticipated this. The user's unused verification tokens may stay (verify-email answers `already_verified` for them).
- **Password rule:** identical to register - `validate:"required,min=8"` on the request (backend) and `z.string().min(8, 'Пароль должен быть не короче 8 символов')` (web). Password is validated in the handler **before** the token is consumed, so a rejected password never burns the link.
- **Session revocation:** all refresh tokens of the user are revoked in the same transaction (same `UPDATE refresh_tokens SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL` as `Logout`; extract a tx-capable helper rather than duplicating). Access JWTs already issued remain valid until expiry (<= 15 min) - pre-existing limitation, accepted.
- **No auto-login:** the emailed token must not become a login credential (same reasoning as verify-email in #160). The success screen offers "Войти".
- **Clear cookies on success:** the reset response calls `clearAuthCookies`. Not strictly required (the server-side sessions are already revoked and the link is often opened in another browser), but it is free and keeps the current browser consistent: without it, a browser that was logged in would keep `has_session`/`access_token` and look logged in for up to 15 min after "Пароль изменён", and `/login` would bounce to `/profile` via the Next middleware. The web success handler additionally drops the in-memory session (see Frontend).
- **Forgot-password rate limits** (before lookup, keyed only on request data, so a 429 never depends on account existence), separate Redis prefixes from resend-verification so the two flows do not share quota:
  - per IP: `rl:pwreset-ip`, `MinInterval: 0`, `WindowLimit: 20`, `Window: 1h`;
  - per email: `rl:pwreset-email`, `MinInterval: 1m`, `WindowLimit: 5`, `Window: 24h`; key = `sha256(lowercase(trim(email)))` hex (reuse `resendEmailKey`, renamed e.g. `emailRateLimitKey`).
  - Order: IP, then email (an IP-limited request does not consume the email quota). Limiter (Redis) error -> 500 `email_send_failed` (before lookup, existence-independent).
- **Reset-password has no rate limit:** the credential is a 256-bit random token; brute force is infeasible and each attempt is one indexed lookup.
- **Lookup** uses the email exactly as given (same as Login/Register/resend); only the rate-limit key is normalised.
- **Enumeration:** best-effort as in #160 - identical status/body for all outcomes; timing differs (DB writes + enqueue only for real accounts, a few ms) and `POST /auth/register` already reveals existing emails via 409. Accepted; not worth async issuance.
- At-least-once mail delivery (from #157): a duplicate email carries the same valid link - harmless.

## UX

All copy Russian; reuse the centred card layout of `/login` / `/verify-email` and `@bookhaven/ui` `Button`. Password inputs are plain `type="password"` like the register form (it has no show/hide toggle - none added here).

### Email (`reset_password`)

- Subject: `Сброс пароля — BookHaven`
- Body: greeting (`Здравствуйте, {Name}!` / `Здравствуйте!`), "Мы получили запрос на сброс пароля для вашего аккаунта BookHaven. Чтобы задать новый пароль, перейдите по ссылке:", button/link `Задать новый пароль`, raw URL as fallback, "Ссылка действительна 1 час и может быть использована один раз.", "Если вы не запрашивали сброс пароля, просто проигнорируйте это письмо - ваш пароль останется прежним."

### `/login`

Under the password field (right-aligned, small): link "Забыли пароль?" -> `/forgot-password` (no email in the URL).

### `/forgot-password`

- Title "Восстановление пароля", hint "Введите email, указанный при регистрации. Мы отправим ссылку для сброса пароля."
- Form: email (`type="email"`, `autoComplete="email"`, zod `email('Введите корректный email')`), button "Отправить ссылку" / "Отправляем…" (disabled).
- `sent`: form replaced by a neutral panel (`role="status"`): "Если аккаунт с таким email существует, мы отправили на него письмо со ссылкой для сброса пароля. Ссылка действительна 1 час." + secondary button "Отправить ещё раз" (disabled for 60 s, returns to the form) + link "Вернуться ко входу" -> `/login`.
- `rate_limited` (429): "Слишком много запросов. Попробуйте снова через N сек/мин/ч." (`formatRetryAfter`), form stays.
- `error` (500/network): "Не удалось отправить письмо. Попробуйте позже."
- 400 `invalid_email`: field error "Введите корректный email".
- Footer link "Вернуться ко входу".

### `/reset-password?token=...`

No request on page load (unlike verify-email); the token is checked when the form is submitted.

- `missing_token` (no/empty `token`): "Ссылка неполная" / "В ссылке нет кода сброса. Откройте ссылку из письма целиком или запросите новую." + `Button` "Запросить новую ссылку" -> `/forgot-password`.
- `form`: title "Новый пароль"; fields "Новый пароль" and "Повторите пароль" (`autoComplete="new-password"`); errors "Пароль должен быть не короче 8 символов", "Пароли не совпадают"; button "Сохранить пароль" / "Сохраняем…".
- `success`: "Пароль изменён" / "Теперь войдите с новым паролем. Все прежние сеансы завершены." + `Button` "Войти" -> `/login`.
- `token_expired`: "Срок действия ссылки истёк" / "Ссылка действительна 1 час. Запросите новую." + "Запросить новую ссылку".
- `invalid_token`: "Ссылка недействительна" / "Ссылка повреждена или устарела. Если вы запрашивали сброс повторно, используйте ссылку из последнего письма." + "Запросить новую ссылку".
- `token_used`: "Ссылка уже использована" / "По этой ссылке пароль уже был изменён. Если нужно, запросите новую." + "Запросить новую ссылку" and secondary "Войти".
- `invalid_password` (400, normally prevented client-side): field error under "Новый пароль", form stays.
- `error` (500/network): inline "Не удалось сменить пароль. Попробуйте снова." - form stays (token not consumed on server errors before commit).

## API

All errors keep `{"error","code"?}`.

### `POST /api/v1/auth/forgot-password` (public)

Body `{"email": "<email>"}` (`required,email`).

| Case | Status | Body |
|---|---|---|
| Malformed body / invalid email | 400 | `{"error":"invalid email","code":"invalid_email"}` |
| Per-IP or per-email limit hit | 429 + `Retry-After` | `{"error":"too many requests","code":"rate_limited","retry_after_seconds":N}` |
| Limiter error | 500 | `{"error":"failed to send password reset email","code":"email_send_failed"}` |
| Anything else (sent, unknown email, no password, failure after lookup) | 202 | `{"status":"sent"}` |

### `POST /api/v1/auth/reset-password` (public)

Body `{"token": "<64 hex>", "password": "<new>"}`.

| Case | Status | Body |
|---|---|---|
| Malformed body | 400 | `{"error":"invalid token","code":"invalid_token"}` |
| Password fails `required,min=8` (checked before the token) | 400 | `{"error":"<validator message>","code":"invalid_password"}` |
| Missing/malformed token, unknown hash, user gone, email mismatch | 400 | `{"error":"invalid token","code":"invalid_token"}` |
| `used_at` set (incl. lost race) | 400 | `{"error":"token already used","code":"token_used"}` |
| Unused, `expires_at <= now` | 400 | `{"error":"token expired","code":"token_expired"}` |
| Success | 200 + auth cookies cleared | `{"status":"password_reset"}` |
| Other error | 500 | `{"error":"failed to reset password"}` |

Service check order: format -> lookup by hash -> user -> email match -> used -> expired -> bcrypt hash (outside the tx) -> transaction (mark used conditionally; 0 rows -> `ErrResetTokenUsed`; update user; delete other unused reset tokens; revoke refresh tokens).

Swagger: Russian summaries/descriptions (mention enumeration safety and limits on forgot-password; codes on both), `@Router /auth/forgot-password [post]`, `/auth/reset-password [post]`; regenerate with `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`.

## Database

`backend/migrations/000012_password_reset_tokens.up.sql`:

```sql
-- One-time password reset tokens (#161). Only the sha256 hex of the token is
-- stored; `email` binds the token to the address it was sent to.
CREATE TABLE password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
```

Down: `DROP TABLE IF EXISTS password_reset_tokens;`. GORM model `auth/domain.PasswordResetToken` with `TableName()`, mirroring `EmailVerificationToken`. Timestamps bound from Go (`time.Now()`), not `now()`, so service SQL runs on SQLite in tests.

## Backend

- `internal/pkg/mailer`: `templates/reset_password.txt.tmpl` (`subject` + `text`) and `reset_password.html.tmpl` (`content`, wrapped by the layout); `ResetPasswordData{Name, ResetURL string}` in `render.go`.
- `modules/auth/domain/tokens.go`: `PasswordResetToken`.
- `modules/auth/service`:
  - Errors `ErrResetTokenInvalid`, `ErrResetTokenExpired`, `ErrResetTokenUsed`.
  - Constructor: add a `resetLimiters ResendLimiters` parameter after `limiters` (same `{IP, Email}` shape; name the type generically if desired, e.g. keep `ResendLimiters`). Touches `cmd/api` and three test helpers (`service_test.go`, `service/verification_test.go`, `transport/http/verification_test.go`).
  - `ForgotPassword(ctx, email, ip string) error`: structurally a copy of `ResendVerification` (nil-limiter guard, IP -> email limiter, lookup, `PasswordHash == nil` -> nil; **no** verified check), then delete unused reset tokens + `issuePasswordReset`; errors after lookup logged at `Error` with `user_id` only, return nil.
  - `issuePasswordReset(ctx, user)`: like `issueVerification` (token row, `mailer.Render("reset_password", ...)`, `msg.To = user.Email`, enqueue). URL: `WebBaseURL + "/reset-password?" + url.Values{"token": {plain}}.Encode()`.
  - `ResetPassword(ctx, plain, newPassword string) error` per the order above. Reuse `isVerificationTokenFormat` (rename to `isTokenFormat`). Lost race -> `ErrResetTokenUsed`. Refactor `Logout`'s update into `revokeRefreshTokens(db *gorm.DB, userID, now)` used by both.
- `modules/auth/transport/http/handler.go`: `forgotPasswordRequest{Email required,email}`, `resetPasswordRequest{Token string; Password required,min=8}`; `ForgotPassword` (same mapping as `ResendVerification`, incl. `c.RealIP()`; extract the 429 writer into a small helper shared by both), `ResetPassword` (bind error -> `invalid_token`; validate error -> `invalid_password`; map service errors; success -> `clearAuthCookies(c)` + 200). New const `codeInvalidPassword = "invalid_password"`.
- `cmd/api/main.go`: two `ratelimit.NewRedisLimiter` instances (`rl:pwreset-ip`, `rl:pwreset-email`, options above) passed to `NewService`; routes `api.POST("/auth/forgot-password", ...)`, `api.POST("/auth/reset-password", ...)` without `authMiddleware`.
- Request logger logs method/URI/status only - the token is in the body, fine.

## Frontend

### `packages/api-client/src/index.ts`

- `api.auth.forgotPassword(email: string): Promise<{ status: 'sent' }>` and `api.auth.resetPassword(token: string, password: string): Promise<{ status: 'password_reset' }>` (doc comments list codes).
- Add `/api/v1/auth/forgot-password` and `/api/v1/auth/reset-password` to `AUTH_REFRESH_EXCLUDED_PATHS` (public; a 400 must not trigger refresh).
- Export `PasswordResetErrorCode = 'invalid_token' | 'token_expired' | 'token_used' | 'invalid_password' | 'invalid_email' | 'rate_limited' | 'email_send_failed'` (or extend `VerificationErrorCode` with `invalid_password` - implementer's choice, keep one source).

### `apps/web/src` (FSD)

- `shared/lib/format-retry-after.ts` (+ test): move `formatRetryAfter` out of `features/resend-verification/model/resend-state.ts` (features cannot import other features); `resend-state.ts` imports it from shared; move its test cases.
- `features/forgot-password/`: `model/forgot-state.ts` (`idle | sending | sent | rate_limited{retryAfterSeconds} | invalid_email | error`, `forgotErrorToState(err)` mirroring `resendErrorToState`, messages), `model/use-forgot-password.ts`, `ui/forgot-password-form.tsx` (form + sent panel + 60 s cool-down), `index.ts`.
- `features/reset-password/`: `model/reset-state.ts` (`resetErrorToState(err)` -> `invalid_token | token_expired | token_used | invalid_password | error`), `model/use-reset-password.ts`, `ui/reset-password-form.tsx` (two fields, zod `min(8)` + `refine` match). On success, if `useAuthStore.getState().isAuthenticated`, call `logout()` and swallow its error (server sessions are revoked and cookies cleared; this drops the in-memory user and the refresh timer).
- `views/forgot-password/`, `views/reset-password/` (reads `token` via `useSearchParams`, trims; renders state copy per UX; links to `/forgot-password`, `/login`).
- `app/forgot-password/page.tsx`, `app/reset-password/page.tsx`: `metadata` with `title` (`Восстановление пароля — BookHaven` / `Новый пароль — BookHaven`), `robots: { index: false, follow: false }`, `referrer: 'no-referrer'`; `<Suspense>` around the view (like `/verify-email`).
- `features/auth/ui/login-form.tsx`: "Забыли пароль?" link (`next/link`).
- `middleware.ts`: no list change - both paths stay public (in neither list); extend the `/verify-email` comment to cover them. **Recommendation: do not redirect logged-in users** away from either page. `/reset-password` is an emailed link that must work in any browser state (as `/verify-email`); `/forgot-password` is harmless for a logged-in user and redirecting it would add a special case for no benefit.

### `apps/admin`

No change; must still compile.

## Existing Patterns

- Token model/hash/single-use transaction: `auth/service/service.go` (`issueVerification`, `VerifyEmail`, `markVerificationTokenUsed`, `hashToken`), `auth/domain/tokens.go`, migration `000011_email_verification_tokens`.
- Public enumeration-safe limited endpoint + 429 mapping: `ResendVerification` in service and handler; limiter wiring in `cmd/api/main.go`.
- Templates: `internal/pkg/mailer/templates/verify_email.*.tmpl`, `render.go`, `render_test.go`.
- Tests: `auth/service/login_gate_test.go`, `auth/service/verification_test.go` (SQLite DDL + fakes), `auth/transport/http/login_gate_test.go`.
- Web: `features/resend-verification` (state mapping, cool-down, form), `views/verify-email` (state copy table, `Suspense` page, metadata), `views/login` + `features/auth/ui/login-form.tsx`.

## Tests

Backend (Docker `go test ./...`; add `password_reset_tokens` DDL to the SQLite test setup):
- Service `ForgotPassword`: unknown email -> nil, nothing sent, both limiters consulted; verified user -> one email to `user.Email`, text contains `/reset-password?token=<plain>`, stored hash = `hashToken(plain)`, `expires_at` ~ +1h; unverified user -> email sent too; no password -> nothing; second request deletes the first unused token (old -> `ErrResetTokenInvalid`); IP limiter denies -> `RateLimitError`, email limiter not called, no lookup; email limiter denies -> `RateLimitError`, no lookup; email key normalised (two casings share a key); limiter error -> error; enqueue error -> nil.
- Service `ResetPassword`: success -> new password passes bcrypt, old fails, `used_at` set, all refresh tokens revoked, other unused reset tokens deleted; unverified user -> `email_verified_at` set; verified user -> `email_verified_at` unchanged; second call -> `ErrResetTokenUsed`; expired -> `ErrResetTokenExpired` (password unchanged); malformed/unknown/email mismatch -> `ErrResetTokenInvalid`; after reset `RefreshTokens(old)` -> `ErrTokenInvalid` and `Login(new)` succeeds.
- Handlers: forgot-password 400 `invalid_email`, 202 for unknown/known, 429 header + body, 500 on limiter error, `RealIP` used as IP key, no auth required; reset-password 400 for each code, `invalid_password` returned **and token still unused**, 200 body + three cookies cleared (`MaxAge -1`).
- `mailer.Render("reset_password", ...)`: Cyrillic subject, URL in text, HTML escapes `<script>` in name.

Frontend (Vitest):
- `shared/lib/format-retry-after.test.ts` (moved cases); `resend-state.test.ts` still passes.
- `forgot-state.test.ts`, `reset-state.test.ts`: error -> state mapping (429 with/without `retryAfterSeconds`, codes, non-ApiError).
- `views/forgot-password` test: submit -> `api.auth.forgotPassword(email)`; 202 -> neutral message; 429 -> retry text.
- `views/reset-password` test: missing token -> missing state + link to `/forgot-password`; mismatch / short password -> no API call; success -> "Пароль изменён" + "Войти"; each error code -> its copy and link; logged-in store -> `logout` called on success.
- `login-form.test.tsx`: "Забыли пароль?" link to `/forgot-password`.
- api-client: typecheck (tests if the package has them).
- Manual (Mailpit): forgot for unknown email -> 202, no mail; known -> mail with working link, Cyrillic subject; second request quickly -> 429; older link -> invalid; reset -> success, old password 401, new works; reuse link -> used; expired (`UPDATE password_reset_tokens SET expires_at = now() - interval '1 minute'`) -> expired; unverified registered user resets -> can log in; logged-in browser + reset -> session gone.

## Documentation

- `docs/ROADMAP.md`: move #160 to "Completed" (merged to `main`, link its contract); Up Next item 2 -> "Password reset by email - in progress (see [password-reset.md](features/password-reset.md), #161)". Technical Debt: add purge of used/expired `password_reset_tokens` next to the verification-tokens item.
- `docs/DEPLOYMENT.md`: list new Redis key prefixes `rl:pwreset-ip:*`, `rl:pwreset-email:*` alongside the verify-resend ones; note that password reset also depends on working SMTP and on the `X-Forwarded-For` / `IPExtractor` setup for the per-IP limit.
- `docs/features/email-verification-login-gate.md`: one line under the header that #161 implemented the "reset verifies the email" note (link this contract).
- Swagger regenerated.
- This contract: `Status: implemented` + "Implementation Notes (as built)" when done.

## Risks

- **Constructor change** of `authservice.NewService` (extra limiter pair) touches `cmd/api` and three test helpers.
- **bcrypt 72-byte limit:** `golang.org/x/crypto/bcrypt` rejects passwords > 72 bytes; with the register rule (no `max`) such a password yields 500 on both register and reset. Kept identical on purpose; follow-up candidate: add `max=72` to both. **Resolved** by OpenSpec change `cap-password-length-at-72-bytes`: the auth service rejects passwords over 72 bytes (UTF-8) first, handlers answer 400 (`invalid_password` on reset) and the web forms check the same byte limit.
- **Reset-verifies-email** is a deliberate policy change: whoever controls the mailbox gets a verified account. Same trust model as verify-email, so no new exposure.
- **Access tokens survive up to 15 min** after reset (JWTs not revocable) - pre-existing, same as Logout.
- **Timing side channel / register 409** still allow partial enumeration (documented, accepted as in #160).
- **Shared NAT** may hit 20/h per IP; tune later.
- **Silent send failures** after lookup return 202; logged at `Error`, user can retry after the cool-down.
- **Token in URL** of `/reset-password`: mitigated by `no-referrer`, `noindex`, 1h TTL, single use; may remain in browser history until used/expired.
- **Lockout if SMTP is broken** in prod: reset is unusable (no worse than registration under #160); DEPLOYMENT warning already exists.

## Implementation Notes (as built)

- **Backend:** migration `000012_password_reset_tokens` as specified; `auth/domain.PasswordResetToken`; templates `reset_password.{txt,html}.tmpl` + `mailer.ResetPasswordData`. In the service, the per-IP -> per-email limiter sequence of `ResendVerification` was extracted into `checkEmailRequestLimits(ctx, limiters, email, ip)` and is shared by `ResendVerification` and `ForgotPassword` (behaviour unchanged; the nil-limiter error text is now "rate limiters not configured"). `resendEmailKey` -> `emailRateLimitKey`, `isVerificationTokenFormat` -> `isTokenFormat`. `Logout` now delegates to `revokeRefreshTokens(db, userID, now)`, which the reset transaction calls with `tx`. `ResendLimiters` kept its name and is reused for the forgot-password pair (`NewService(cfg, db, users, mailer, limiters, resetLimiters, logger)`); a fourth caller, `internal/transport/http/middleware/auth_gate_test.go`, also needed the extra argument.
- **ResetPassword** returns `ErrResetTokenUsed` directly from inside the transaction when the conditional `used_at` update affects 0 rows (rolls back; no re-read needed, unlike `VerifyEmail`). The `invalid_password` error body carries the raw validator message (as Register does).
- **Handler:** `writeRateLimited(c, *RateLimitError)` is the shared 429 writer for resend-verification and forgot-password. Swagger regenerated (only the two new paths and request types were added).
- **api-client:** separate `PasswordResetErrorCode` union (the verification union is unchanged).
- **Web:** `formatRetryAfter` moved to `shared/lib/format-retry-after.ts` and is no longer re-exported from `features/resend-verification`. `features/forgot-password` owns the whole card body below the title (hint, form, sent panel, "Вернуться ко входу"); the 60 s cool-down (and the `retry_after_seconds` wait after a 429) disables both "Отправить ещё раз" and the form submit, like the resend form. `features/reset-password` exposes `useResetPassword(token)` + `ResetPasswordForm`; `views/reset-password` renders the link-failure / success screens and keys the content by token. On success the hook calls the store `logout()` fire-and-forget (error swallowed) only when `isAuthenticated`. The "Забыли пароль?" link sits under the password field of `login-form.tsx`.
- **Manual check (local stack + Mailpit):** unknown email -> 202, no mail; known -> one mail, Cyrillic subject, name HTML-escaped; second request -> 429 with `Retry-After`; older link -> `invalid_token`; short password -> `invalid_password` with the token still unused; valid reset -> 200 + three cleared cookies, old password 401, new password 200, old refresh token 401; reuse -> `token_used`; expired -> `token_expired`; unverified user -> reset -> login 200. `/forgot-password` and `/reset-password` serve `noindex, nofollow` + `no-referrer` and are not redirected for a logged-in cookie; `/login` shows the link.
