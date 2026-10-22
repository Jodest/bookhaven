# Tasks

## 1. Backend

- [x] 1.1 Add `AuthCookieSecure bool` to `Config` and parse `AUTH_COOKIE_SECURE` in `config.Load` (unset/unparsable: `true` iff `APP_ENV=production`; explicit valid value wins). Add table tests in `config_test.go`: dev unset -> false, production unset -> true, production `false` -> false, dev `true` -> true, garbage in production -> true.
- [x] 1.2 In `auth/transport/http/handler.go`, add `secureCookies` to `Handler` and an option `WithSecureCookies(bool)` on `NewHandler`; convert `setAuthCookies` / `clearAuthCookies` into methods built on one cookie constructor (HttpOnly, SameSite=Strict, Secure from the handler, same paths/MaxAge as today; clear = empty value, MaxAge -1). Update the call sites in Login, Refresh (success and `email_not_verified`), Logout and ResetPassword.
- [x] 1.3 Wire `authhttp.WithSecureCookies(cfg.AuthCookieSecure)` in `backend/cmd/api/main.go`; log a warning at startup when `APP_ENV=production` and secure cookies are off.
- [x] 1.4 Tests in `handler_test.go` (and adjust `login_gate_test.go`, `password_reset_test.go` if they call the helpers): with secure on, all three cookies on set and on clear have `Secure`, `HttpOnly`, `SameSite=Strict` and the expected paths; with secure off, none has `Secure`; clear cookies have `MaxAge < 0`. Existing scope/MaxAge assertions keep passing.
- [x] 1.5 Verify from `backend/`: `go build ./...`, `go vet ./...`, `go test ./internal/config/... ./internal/modules/auth/...`. Format only the changed files with gofmt.

## 2. DevOps

- [x] 2.1 `docker-compose.prod.yml`, api service env: add `AUTH_COOKIE_SECURE: ${AUTH_COOKIE_SECURE:-true}`. Leave `docker-compose.yml` without it (development default stays off) — confirm the local stack still logs in over `http://localhost:3000`.
- [x] 2.2 `.env.example`: commented `# AUTH_COOKIE_SECURE=false` with a one-line explanation; `.env.production.example`: commented note that prod compose defaults it to `true` and it must only be turned off for an HTTP-only test box.

## 3. Docs

- [x] 3.1 `docs/DEPLOYMENT.md` env table: add an `AUTH_COOKIE_SECURE` row (default on in production, off in development, override semantics, TLS must terminate in front of the api/web or browsers drop the cookies). Remove the item from any "parked until prod" list if it is mentioned there.
