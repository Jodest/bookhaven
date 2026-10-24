# Tasks

## 1. Backend

- [x] 1.1 Auth service: add `RegisterLimiters{IP, Email LoginLimiter}` and a `registerLimiters` parameter to `NewService` (after `loginLimiters`); change `Register` to take `ip string` and, after the name/password checks and before `GetByEmail`, reserve an IP slot then an address slot (`emailRateLimitKey`) exactly like `Login` (IP deny returns before the address check; limiter errors fail open with a warn log; nil limiter = no limit). Release both reserved slots on internal errors (lookup, bcrypt, create); keep them on 201, 409 and on a verification-email failure. Update the doc comment. Verify: `go build ./...` from `backend/`.
- [x] 1.2 Update every `NewService(` and `Register(` call site (grep the whole `backend/`, including tests outside the auth module) to the new signatures; existing tests pass a zero `RegisterLimiters{}`.
- [x] 1.3 Handler: pass `c.RealIP()` to `Register`; map `*service.RateLimitError` via `errors.As` to `writeRateLimited` before the other error checks; extend the Register godoc with the 429 failure, `Retry-After` header and a sentence on the limits.
- [x] 1.4 `cmd/api/main.go`: add `registerIPLimiter` (`rl:register-ip`, 10 per 1h) and `registerEmailLimiter` (`rl:register-email`, 5 per 1h), pass them to `NewService`; add registration to the `IPExtractor` comment listing per-IP limited endpoints.
- [x] 1.5 Service tests (new `service/register_throttle_test.go`, modelled on `login_throttle_test.go` with fake limiters): 11th request from one IP is `*RateLimitError` and creates no user and enqueues no mail; IP deny does not touch the address limiter; address deny keeps the IP slot; 409 consumes both slots like 201; invalid name/password (400 errors) consume nothing; an internal error (failing users repo create) releases both; limiter error -> registration succeeds (fail open); address key is the same for `" User@X.com "` and `user@x.com`.
- [x] 1.6 Handler tests (new `transport/http/register_throttle_test.go`, modelled on `login_throttle_test.go`, miniredis-backed real `ratelimit.RedisLimiter` if that is what the login test does): 429 body has `code: rate_limited` and `retry_after_seconds` >= 1, `Retry-After` header matches; the limited response is byte-identical for an existing and an unknown address; 400 requests do not count.
- [x] 1.7 Regenerate Swagger: `docker compose exec backend swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`; check `/auth/register` lists 429 in `backend/docs/swagger.yaml`.
- [x] 1.8 Verify: from `backend/`, `go build ./...`, `go vet ./...`, `go test ./internal/modules/auth/... ./internal/pkg/ratelimit/...` (and any other package whose call sites changed). gofmt only the changed files. No destructive DB commands; no migrations needed.

## 2. Frontend (apps/web)

- [x] 2.1 `apps/web/src/entities/session/model/auth-store.ts`: in `register`, map a 429 / `rate_limited` `ApiError` to «Слишком много попыток регистрации. Попробуйте снова через N.» using `formatRetryAfter`, fallback 3600 s («1 ч») when `retryAfterSeconds` is missing/invalid; other errors unchanged. Reuse the login helper structure (a shared in-file helper with prefix + fallback is fine).
- [x] 2.2 Tests in `auth-store.test.ts`: 429 with `retry_after_seconds` 600 -> «Слишком много попыток регистрации. Попробуйте снова через 10 мин.»; 429 without it -> «… через 1 ч.»; existing login rate-limit tests still pass. If `features/auth/ui/register-form.test.tsx` mocks the store, add one case that the form shows the store error and does not call `onRegistered` on 429.
- [x] 2.3 Verify: in `apps/web`, `pnpm exec tsc --noEmit`, `pnpm lint` (next lint) and the vitest suite for the touched files. No native `next build` (CI is the typedRoutes gate). `apps/admin` is not touched (no registration there).

## 3. Docs

- [x] 3.1 `docs/DEPLOYMENT.md`: in the paragraph about per-IP limits relying on Caddy's `X-Forwarded-For`, add registration to the list of per-IP limited endpoints (10/h per IP, 5/h per address).
- [x] 3.2 `docs/ROADMAP.md`: mark the #624 entry done with a pointer to this change. Do not edit `openspec/specs/` (updated on archive).
