# Design

## Context

`backend/internal/modules/auth/transport/http/handler.go` has two package-level helpers: `setAuthCookies` (called by Login and Refresh) and `clearAuthCookies` (called by Refresh on `email_not_verified`, Logout and ResetPassword). Both hardcode `Secure: false`; `clearAuthCookies` also omits `SameSite`. These two helpers are the only places in the repo that set or clear the auth cookies: no OAuth flow exists, and `apps/web` / `apps/admin` only *read* `access_token` / `has_session` in their Edge `middleware.ts` (the admin keeps its bearer token under its own localStorage key). The Next `/api` rewrite proxies `Set-Cookie` unchanged, so no frontend change is needed.

`APP_ENV` is already `production` in `docker-compose.prod.yml` (api and worker) and defaults to `development` in `docker-compose.yml` and `config.Load`.

## Decisions

- **Env `AUTH_COOKIE_SECURE`**, parsed with `strconv.ParseBool`. Unset or unparsable means the environment default: `true` when `APP_ENV == "production"`, otherwise `false`. An explicit valid value always wins (e.g. `true` to test behind a local HTTPS proxy; `false` for an HTTP-only staging box). Do not reject `false` in production — just log a warning at API startup (`cfg.Env == "production" && !cfg.AuthCookieSecure`), since there is no real prod yet and a hard fail would block HTTP-only experiments.
- New field `Config.AuthCookieSecure bool` in `backend/internal/config/config.go`, with a small helper (e.g. `parseAuthCookieSecure(env string) bool` using `os.LookupEnv`) so it is unit-testable like the other parsers.
- **Handler option, not a global**: change `NewHandler(service *service.Service)` to accept the setting. Preferred: functional option `NewHandler(svc, WithSecureCookies(cfg.AuthCookieSecure))` (keeps existing test call sites compiling), storing `secureCookies bool` on `Handler`. Turn `setAuthCookies` / `clearAuthCookies` into methods (`h.setAuthCookies(c, tokens)`, `h.clearAuthCookies(c)`) and update the five call sites.
- **One attribute source**: build cookies through one internal constructor (e.g. `h.authCookie(name, value, path string, maxAge int) *http.Cookie`) that always sets `HttpOnly: true`, `SameSite: http.SameSiteStrictMode`, `Secure: h.secureCookies`. Clearing uses the same constructor with `Value: ""`, `MaxAge: -1`. This guarantees set/clear attribute parity and fixes the missing `SameSite` on clear.
- Wire in `backend/cmd/api/main.go` line ~287: `authhttp.NewHandler(authService, authhttp.WithSecureCookies(cfg.AuthCookieSecure))`. The worker does not set cookies; it needs nothing.

## API

No endpoint, body or status change. Only the `Set-Cookie` attributes on `POST /api/v1/auth/login`, `/refresh`, `/logout`, `/reset-password` change (Secure added when enabled; SameSite=Strict now also on the clearing cookies). Authorization rules are unchanged (login/refresh/reset-password public, logout requires authentication).

## Frontend

None. Note for reviewers: Chrome/Firefox treat `http://localhost` as a secure context and would accept `Secure` cookies there, but other local hostnames (LAN IP, `*.local`, Docker host names) would not — that is why development defaults to off.

## Existing Patterns

- Env parsing: `parseBoolEnv` / `parseBoolEnvDefaultTrue` and the `os.LookupEnv` branch for `AI_GENRES_CRON` in `config.go`; production guards at the end of `Load()`.
- Handler options: other modules' `NewHandler(svc, opts...)` (e.g. `cataloghttp.NewHandler(catalogService, catalogOpts...)` in `internal/transport/http/router.go`).
- Cookie tests: `findSetCookie` in `auth/transport/http/handler_test.go`, also used by `login_gate_test.go` and `password_reset_test.go`.

## Risks

- Forgetting a call site leaves a non-Secure clear: mitigated by making both helpers methods on `Handler` (the free functions disappear, so the compiler catches every call site).
- Running the prod compose without TLS in front would make browsers drop the cookies (login looks successful but the session is lost). Documented in `docs/DEPLOYMENT.md` with the override.
- Existing tests calling `setAuthCookies(c, ...)` / `clearAuthCookies(c)` directly must be updated to go through a handler.
