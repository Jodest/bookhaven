# Design

## Context

Login (`authservice.Service.Login`, handler `Handler.Login` in `backend/internal/modules/auth/transport/http/handler.go`) has no throttling. Resend-verification and forgot-password already use `backend/internal/pkg/ratelimit.RedisLimiter` (wired in `backend/cmd/api/main.go`, keys documented in `docs/DEPLOYMENT.md`), the `*RateLimitError` / `ErrRateLimited` error, the `writeRateLimited` 429 writer and `emailRateLimitKey` (sha256 of the trimmed, lowercased address). The client IP is `c.RealIP()` via `e.IPExtractor = echo.ExtractIPFromXFFHeader()` behind Caddy.

## Decisions

1. **Throttle failed attempts, never hard-lock.** Two fixed 15-minute windows: per client IP 60 attempts of any outcome, per address 10 failures. A lockout keyed on email alone would let anyone lock any user out indefinitely; a self-expiring 15-minute throttle bounds that DoS, and Decision 5 gives the owner a way out.
2. **Reserve-then-settle, race-free.** Counting only failures with a "check, then record after bcrypt" split lets a burst of parallel requests all pass the check (bcrypt takes about 50-100 ms each). Instead each attempt *reserves* a slot up front with the existing `Allow` (GET >= limit → blocked without INCR; else INCR, and INCR result > limit → blocked — already atomic against bursts), then settles:
   - wrong password / unknown email / no password → keep both reservations (a failure);
   - correct password (200 or 403 `email_not_verified`) → **reset** the email counter (DEL) and keep the IP slot: the IP counter counts every attempt. Review finding: releasing the IP slot on success let an attacker with a valid account of their own alternate (victim + wrong password) with (own account + correct password) and keep the IP counter flat, defeating the per-IP limit against credential stuffing. The limit is 60 (not 30) to leave headroom for shared NAT/office/university networks;
   - internal error (DB, token creation) → release both slots (not a guess; `Release` is still used for this);
   - email limited after the IP slot was reserved → keep the IP slot (an attacker hammering a blocked address still spends IP quota).
   Order: IP first, then email (as `checkEmailRequestLimits`), both before `users.GetByEmail`.
3. **Fail-open on limiter errors, logged.** The email-sending endpoints fail closed (500) because blocking them costs nothing; failing closed on login would sign everyone out of the product during a Redis outage. Any `Allow` error → `logger.Warn` and continue unlimited; `Release`/`Reset` errors → `logger.Warn`, ignored. bcrypt remains the per-guess cost floor. Nil limiters (tests, misconfiguration) mean "no limits", not an error.
4. **Same 429 contract** as the other limited endpoints: `writeRateLimited` → `Retry-After` header + `{error: "too many requests", code: "rate_limited", retry_after_seconds}`. `RetryAfter` comes from the window key TTL (`RedisLimiter.ttl` already repairs a TTL-less key).
5. **Reset password clears the address counter** (`Reset(emailRateLimitKey(user.Email))` after the reset transaction commits; error logged, not returned). The reset link proves mailbox ownership, so it is a safe unblock path.
6. **Key prefixes** `rl:login-ip` and `rl:login-email`. The package names the window key `<prefix>:day:<key>` regardless of window length; accept that (documented in DEPLOYMENT.md) rather than change key names for the existing limiters. `MinInterval: 0` for both.

## API

`POST /api/v1/auth/login` — unchanged request/200/400/401/403; new **429** `rate_limited` (`retry_after_seconds`, `Retry-After`). Public, no bearer. Add `@Failure 429 {object} rateLimitedResponse` and `@Header 429 {integer} Retry-After "Секунд до следующей попытки"` to the godoc, extend `@Description` with one sentence about the limits, regenerate `backend/docs` with the Makefile's `swag init` target.

## Backend shape

- `ratelimit.RedisLimiter`: add
  - `Release(ctx, key) error` — decrement `<prefix>:day:<key>` only if it exists and is > 0 (small Lua script, or GET/DECR guarded so a missing key is never created without TTL);
  - `Reset(ctx, key) error` — DEL `<prefix>:min:<key>` and `<prefix>:day:<key>`.
  Tests with miniredis (already used in `ratelimit_test.go`): release decrements and never creates a key; reset deletes; release on a missing key is a no-op.
- `authservice`: new `LoginLimiter` interface (`Allow`, `Release`, `Reset`) and `LoginLimiters{IP, Email LoginLimiter}`; new `NewService` parameter after `resetLimiters` (4 call sites: `cmd/api/main.go` + 3 test files; tests may pass `LoginLimiters{}` = no limits). `Login(ctx, email, password, ip string)`; returns `*RateLimitError` when limited. `ResetPassword` calls `Email.Reset` after commit.
- Handler: pass `c.RealIP()`; map `ErrRateLimited` via `errors.As(*RateLimitError)` → `writeRateLimited` (same as ForgotPassword).
- `cmd/api/main.go`: two `NewRedisLimiter` with `Window: 15 * time.Minute`, `WindowLimit: 60` (ip) / `10` (email); update the IPExtractor comment to mention login.

## Frontend

- **web** `apps/web/src/entities/session/model/auth-store.ts` `login` catch: for `isApiError(error) && (error.status === 429 || error.code === 'rate_limited')` set `error` to `Слишком много попыток входа. Попробуйте снова через ${formatRetryAfter(secs)}.` (`secs` = `retryAfterSeconds` if finite and > 0, else 900). `formatRetryAfter` is in web `shared/lib` (entities → shared is allowed). Other errors unchanged.
- **admin** `apps/admin/src/entities/session/model/auth-store.ts` `login` catch: same branch before the existing `email_not_verified` mapping. Copy `format-retry-after.ts` (+ its test) into `apps/admin/src/shared/lib/` and export it from the shared lib index (apps cannot import each other).
- Login form components do not change; they already render the store `error`.

## Existing Patterns

- Limiter wiring and error type: forgot-password (`ForgotPassword`, `checkEmailRequestLimits`, `rl:pwreset-*`).
- 429 handler tests: `transport/http/password_reset_test.go` (Retry-After, `UsesRealIP` with a stub `IPExtractor`).
- 429 UI mapping: `apps/web/src/features/forgot-password/model/forgot-state.ts`.

## Risks

- **Targeted throttle of a victim** (10 wrong passwords every 15 minutes keeps an address blocked). Bounded by the window; owner can reset the password to clear it. Accepted versus unlimited guessing.
- **Fail-open** lets guessing run unthrottled while Redis is down; logged at warn so it is visible. Redis down also breaks the worker queues, so it will not go unnoticed.
- **Shared NAT** (office, university): 60 attempts per 15 min per IP; successful logins now count too, so the limit was doubled for headroom.
- **IP trust**: per-IP limit depends on the Caddy/`IPExtractor` setup in DEPLOYMENT.md; an extra proxy in front would collapse all clients to one IP.
- Web login still shows the backend's English «invalid credentials» for 401 — pre-existing, out of scope.
