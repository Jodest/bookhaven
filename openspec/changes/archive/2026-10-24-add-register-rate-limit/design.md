## Context

`POST /api/v1/auth/register` (auth service `Register`) validates name/password, looks up the email (409 `user already exists` if taken), creates the account and enqueues a verification email; it has no throttling. Login (epic #565) already has the exact shape we need: per-IP and per-address `ratelimit.RedisLimiter`s with fixed windows, slots reserved before the account lookup, `Release` on internal errors, fail open with a warn log. Resend-verification and forgot-password use `checkEmailRequestLimits` (fail closed, no release) and are not the model here.

## Decisions

- **Limits.** `rl:register-ip`: `WindowLimit 10`, `Window 1h`, `MinInterval 0`, key = `c.RealIP()`. `rl:register-email`: `WindowLimit 5`, `Window 1h`, `MinInterval 0`, key = `emailRateLimitKey(email)` (sha256 of trimmed lowercased address; no plain addresses in Redis). A real user registers once; 10/h per IP leaves room for a shared NAT (family, office) and a couple of typos/409s, while capping one IP at 240 accounts (= 240 verification emails to strangers) per day instead of unlimited.
- **Every validated outcome counts** (201 and 409). This is what keeps the 429 independent of account existence and also throttles 409-probing of the address list. The 409 itself already reveals existence (spec `Unique email`, unchanged); this change does not try to remove that, only to rate-limit it.
- **Order** (mirrors `Login`): handler bind/validate (400) -> service name/password checks (400) -> IP `Allow` (deny -> return, address quota untouched) -> address `Allow` (deny -> return, IP slot stays spent) -> `GetByEmail` -> create -> email. 400s happen before any limiter so they never consume quota.
- **Internal errors release both reserved slots** (`Release`), exactly like login's `settleInternal`: lookup error, bcrypt error, `users.Create` error. A verification-email failure is not an error (registration still answers 201), so the slots stay spent.
- **Fail open** on limiter errors with `logger.Warn("register: ip|email rate limiter unavailable, failing open")`, same as login. Rationale: the mail queue (asynq) uses the same Redis, so while Redis is down no verification email can be enqueued anyway; failing closed would only block sign-ups without stopping any spam.
- **No shared register+resend email counter.** Register sends at most one email per address for the lifetime of the account (a repeat register is 409 without an email); further emails to that address go through resend, which keeps its own 1/min + 5/day per-address and 20/h per-IP limits. Registration also must not consume the resend quota: otherwise an attacker could pre-exhaust it and stop a victim from getting a new link. Max per address per day therefore stays 1 + 5.
- **Limiter type.** Register needs `Allow`/`Release`, i.e. the existing `LoginLimiter` interface. Add `type RegisterLimiters struct { IP, Email LoginLimiter }` (nil = no limit, so existing tests passing a zero value keep working) and a `registerLimiters RegisterLimiters` parameter on `NewService` after `loginLimiters`. Do not reuse `ResendLimiters` (no `Release`, fail-closed semantics).
- **Client IP.** No change: `e.IPExtractor = echo.ExtractIPFromXFFHeader()` already trusts only private/loopback hops (Caddy over the Docker network). Extend the comment in `cmd/api/main.go` that lists the per-IP limited endpoints.
- **Admin app** has no registration (only `/login`), nothing to change there.

## API

`POST /api/v1/auth/register` — public, no bearer, unchanged request/201/400/409 bodies. New:

| Status | Headers | Body |
|---|---|---|
| 429 | `Retry-After: <s>` | `{"error":"too many requests","code":"rate_limited","retry_after_seconds":<s>}` (via existing `writeRateLimited`) |

Service signature becomes `Register(ctx, email, password, name, ip string)`; the handler passes `c.RealIP()` and maps `*service.RateLimitError` with `errors.As` before the other error checks (as in `Login`). Swagger: add `@Failure 429 {object} rateLimitedResponse "code: rate_limited"`, `@Header 429 {integer} Retry-After "Секунд до следующей попытки"`, and a sentence on the limits in `@Description` (10 запросов в час с одного IP, 5 в час для одного адреса, одинаково для новых и существующих адресов), then regenerate `backend/docs`.

## Frontend

`apps/web` only. In `entities/session/model/auth-store.ts`, `register` maps a 429 / `rate_limited` `ApiError` to «Слишком много попыток регистрации. Попробуйте снова через ${formatRetryAfter(seconds)}.» with fallback 3600 s («1 ч») when `retryAfterSeconds` is missing or not a positive finite number — same structure as `loginRateLimitMessage` (a small shared helper parameterised by prefix and fallback is fine inside the same file). Other errors keep the current behaviour. `features/auth/ui/register-form.tsx` already renders `error` from the store and stays on the form on failure; no UI change expected beyond the message. No FSD boundary changes.

## Existing Patterns

- Backend: `Service.Login` (reserve -> settle) and its tests `service/login_throttle_test.go`, `transport/http/login_throttle_test.go`; limiter wiring in `cmd/api/main.go` (`loginIPLimiter`/`loginEmailLimiter`); `writeRateLimited` in the auth handler; `ratelimit` package (`Allow`/`Release`).
- Frontend: `loginRateLimitMessage` in `apps/web/src/entities/session/model/auth-store.ts` and its tests in `auth-store.test.ts`; `formatRetryAfter` in `apps/web/src/shared/lib`.

## Risks

- **Shared IPs** (campus/office NAT, mobile CGNAT): >10 sign-ups per hour from one IP get 429. Acceptable for a small catalog; numbers are constants in `cmd/api/main.go` and easy to raise.
- **Distributed flooding** (many IPs, many addresses) is not stopped by per-IP/per-address limits; a global cap or CAPTCHA is out of scope.
- **Proxy change**: if another proxy is put in front of Caddy, `RealIP` trust must be revisited (already documented in `docs/DEPLOYMENT.md`).
- **NewService signature change** touches every call site, including tests outside the auth module; grep the whole backend for `authservice.NewService(` / `service.NewService(` in auth packages.
