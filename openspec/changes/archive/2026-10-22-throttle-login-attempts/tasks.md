# Tasks

## 1. Backend

- [x] 1.1 `backend/internal/pkg/ratelimit`: add `Release(ctx, key)` (decrement the window counter only if it exists and is > 0; never create a TTL-less key) and `Reset(ctx, key)` (delete the min and window keys); miniredis tests for decrement, no-op on a missing key, reset. Verify: `go test ./internal/pkg/ratelimit/...`.
- [x] 1.2 `authservice`: add `LoginLimiter` / `LoginLimiters`, the new `NewService` parameter, and `Login(ctx, email, password, ip)` with reserve-then-settle (IP then email, before the user lookup; keep the IP slot + reset email on correct password incl. `ErrEmailNotVerified`; release both on internal errors; nil limiters = no limits; limiter errors logged and ignored = fail-open). Update the 3 test call sites of `NewService`.
- [x] 1.3 `ResetPassword`: after the transaction commits, `Reset` the login email counter for the user's address; log and ignore errors.
- [x] 1.4 Service tests (fake limiters): 10 failures then 11th → `*RateLimitError` with the retry; limited before lookup (unknown and existing address behave the same, password not checked); success after 9 failures resets the email counter and keeps the IP slot; alternating victim guesses with own-account correct logins still fills the IP quota; 403 unverified also resets; IP-limited request does not touch the email limiter; limiter error → login still succeeds; reset-password calls `Reset` with the sha256 key.
- [x] 1.5 Handler: pass `c.RealIP()`, map `*RateLimitError` to `writeRateLimited`; godoc `@Failure 429` + `@Header 429 Retry-After`; tests for 429 body/`Retry-After` and `UsesRealIP` (pattern: `password_reset_test.go`).
- [x] 1.6 `cmd/api/main.go`: wire `rl:login-ip` (60 / 15 min, all attempts) and `rl:login-email` (10 / 15 min), update the IPExtractor comment. Regenerate Swagger with the Makefile `swag init` target.
- [x] 1.7 Verify from `backend/`: `go build ./...`, `go vet ./...`, `go test ./...`. Format only changed files with gofmt (never the whole tree).

## 2. Frontend

- [x] 2.1 web `entities/session/model/auth-store.ts`: 429 / `rate_limited` on login → «Слишком много попыток входа. Попробуйте снова через N.» via `formatRetryAfter` (fallback 900 s); test in the store or `features/auth/ui/login-form.test.tsx` for 600 s → «10 мин» and the fallback «15 мин».
- [x] 2.2 admin: copy `format-retry-after.ts` and its test into `apps/admin/src/shared/lib/`, export it; same 429 branch in `entities/session/model/auth-store.ts`; test in `features/auth/ui/login-form.test.tsx`.
- [x] 2.3 Verify per app: `tsc --noEmit`, `next lint`, unit tests (vitest) for web and admin. No native `next build`.

## 3. Docs

- [x] 3.1 `docs/DEPLOYMENT.md`: add `rl:login-ip:*` (60 attempts of any outcome / 15 min per IP) and `rl:login-email:*` (10 failed / 15 min per sha256 address, cleared on a correct password or a password reset; window key is named `:day:` by the package), note login fails open on Redis errors while the email endpoints fail closed; mention login in the "Client IP behind Caddy" paragraph; update the launch-checklist item about credential stuffing on `/api/v1/auth/login` (app-layer limit now in place, Caddy limit optional).
