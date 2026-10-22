# HTTP Security Headers

Status: done (browser smoke, Swagger "Try it out" and live HSTS over TLS still to be checked manually — see unchecked criteria)

Source: backlog proposal #197.

## Goal

Every response from web, admin and the API carries baseline hardening headers (CSP, frame protection, nosniff, Referrer-Policy, Permissions-Policy, COOP; HSTS over TLS). Today the Caddyfile, both `next.config.js` files and the Echo stack (Recover, CORS, request logger) set none, apart from `X-Content-Type-Options` on `/covers/*`.

No OpenSpec domain's observable behavior changes. Cookies, CORS, redirects, rewrites and response bodies stay exactly as they are.

## Acceptance Criteria

- [x] `curl -I http://localhost:3000/` (web, local compose) returns `Content-Security-Policy`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy`, `Cross-Origin-Opener-Policy: same-origin`, and **no** `Strict-Transport-Security`.
- [x] `curl -I http://localhost:3001/login` (admin; use the actual admin port from `docker-compose.yml`) returns the same set plus `X-Robots-Tag: noindex, nofollow`. A redirect response from the admin middleware (unauthenticated `/`) also carries them.
- [x] `curl -I http://localhost:8080/api/v1/<any public GET>` and a 404/401 response return `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and no HSTS. CORS preflight (`OPTIONS` with `Origin: http://localhost:3000`) still returns the same `Access-Control-*` headers as before.
- [ ] `/swagger/index.html` gets the Swagger CSP (below) and still renders and runs "Try it out" in a browser.
- [x] `/covers/...` still returns exactly one `X-Content-Type-Options: nosniff` (not two).
- [x] No header is emitted twice on web/admin pages: Next.js does not apply its header rule to the proxied `/api/*` and `/covers/*` paths, where the backend's own headers pass through.
- [ ] Manual browser smoke on local compose (devtools console has **no** CSP violations): web home, book page (cover, JSON-LD), author page with a Wikimedia photo, a profile with an external avatar URL, login/logout/refresh, a review like. Admin: login, dashboard, a book edit with a cover, Import → Wikidata search, a catalog list. Hot reload still works in both dev servers.
- [ ] Caddyfile: each of the three sites sets `Strict-Transport-Security: max-age=31536000` (no `includeSubDomains`, no `preload`). `caddy validate` / `caddy adapt` on the Caddyfile succeed.

## Business Rules

- Each app owns its headers (Next.js `headers()` for web/admin, an Echo middleware for the API). This way the headers are the same in local dev, which has no Caddy, and in prod. Caddy adds **only** HSTS, because only Caddy knows the connection is TLS. Caddy must not set any header that the apps set (no duplicates or conflicts).
- HSTS is never sent over plain HTTP: not by the apps and not in local dev. The Echo middleware must not set HSTS, so do not use Echo's `Secure` middleware with `HSTSMaxAge`. No `preload` and no `includeSubDomains` until prod has run on TLS for a while. Adding `includeSubDomains` would also cover admin and api, which is fine, but it is still deferred.
- CSP is **enforced, not Report-Only**. Report-Only without a report endpoint protects nothing, and we have no collector. The policy is deliberately pragmatic so it cannot break the apps (see Risks).
- `X-XSS-Protection` is not sent (it is deprecated and harmful in old browsers).
- Admin is at least as strict as web, plus `X-Robots-Tag: noindex, nofollow`.

## Header values

Common to web and admin (Next.js):

| Header | Value |
|---|---|
| Content-Security-Policy | see below |
| X-Frame-Options | `DENY` |
| X-Content-Type-Options | `nosniff` |
| Referrer-Policy | `strict-origin-when-cross-origin` |
| Permissions-Policy | `camera=(), microphone=(), geolocation=(), payment=(), usb=(), browsing-topics=()` |
| Cross-Origin-Opener-Policy | `same-origin` |

Admin also gets `X-Robots-Tag: noindex, nofollow`.

Web/admin CSP (the directives are joined with `; `):

```
default-src 'self'
script-src 'self' 'unsafe-inline'            (+ 'unsafe-eval' when NODE_ENV=development)
style-src 'self' 'unsafe-inline'
img-src 'self' data: blob: https:
font-src 'self' data:
connect-src 'self' <API origin>              (+ ws: wss: when NODE_ENV=development, for HMR)
frame-src 'none'
object-src 'none'
base-uri 'self'
form-action 'self'
frame-ancestors 'none'
```

- `<API origin>` = `new URL(process.env.NEXT_PUBLIC_API_URL).origin` when it is set. The browser calls the API cross-origin (`packages/api-client`: the client uses `NEXT_PUBLIC_API_URL`; dev uses `http://localhost:8080`, prod uses `https://${API_DOMAIN}`). If the variable is empty, the API origin is left out and only `'self'` applies, because the `/api` rewrite is same-origin. `headers()` is evaluated at build time. Prod already passes `NEXT_PUBLIC_API_URL` at build, so check that it is a build arg in `docker-compose.prod.yml` / `Dockerfile.node`.
- `img-src https:` is required: Wikimedia author photos are hotlinked (`unoptimized`), avatars are arbitrary user-entered https URLs, and admin renders external cover/photo URLs. Do not narrow it to a host list.
- `'unsafe-inline'` for scripts is required because App Router emits inline bootstrap/RSC scripts and the web book page has an inline JSON-LD `<script>`. A nonce-based CSP would force dynamic rendering of every page via middleware (losing static/ISR caching) and would need nonces threaded through the layouts. That is out of scope; note it as a follow-up.
- `next/font/local` is self-hosted, so `font-src 'self'` covers it. No analytics or third-party scripts exist.

API (Echo middleware):

| Path | Content-Security-Policy |
|---|---|
| everything except `/swagger/*` | `default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'` |
| `/swagger/*` | `default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'` |

Plus, on all API paths: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`. Do **not** add `Cross-Origin-Resource-Policy` or COOP/COEP to the API: web/admin load API resources cross-origin, and CORP would risk breaking cover/image loads.

## API

No endpoint, contract or swagger changes. A new global Echo middleware is added.

- `backend/internal/transport/http/middleware/security_headers.go`: `SecurityHeaders() echo.MiddlewareFunc`. Before calling `next`, it does `Header().Set(...)` for the fixed headers and picks the CSP by `strings.HasPrefix(c.Request().URL.Path, "/swagger/")`. A hand-written middleware is preferred over `echomiddleware.Secure` because Secure cannot emit Permissions-Policy/Referrer per path cleanly, defaults `X-XSS-Protection: 1; mode=block`, and has HSTS logic we don't want.
- Register it in `backend/cmd/api/main.go` right after `Recover()` and before CORS: `e.Use(middleware.SecurityHeaders())`. Headers are set before the handler runs, so they also land on errors, 404s, auth failures, rate-limit 429s and CORS preflights. The covers handler's own `Set("X-Content-Type-Options", "nosniff")` just overwrites the same value, so there are no duplicates.
- Do not touch the CORS config.

## Frontend

- `apps/web/next.config.js` and `apps/admin/next.config.js` get an `async headers()` that returns one rule, `{ source: '/((?!api/|covers/).*)', headers: [...] }`. The source must exclude the proxied `/api/*` and `/covers/*` so backend headers are not duplicated. Keep the existing `images`, `redirects`, `rewrites` and `typedRoutes` untouched.
- Build the header list in a small CommonJS helper per app, e.g. `securityHeaders({ isDev, apiUrl, noindex })` in `apps/<app>/security-headers.js` (or `src/shared/config/` if vitest's `include` only covers `src/`; check `vitest.config`). `next.config.js` `require`s it, and a colocated unit test asserts the exact values. The two apps may each have their own copy; a shared package is not warranted for ~30 lines.
- No changes to `middleware.ts`. Next.js applies `headers()` to middleware redirects too; verify this with curl on admin `/`.

## DevOps (devops agent)

- `Caddyfile`: add a snippet
  ```
  (hsts) {
      header Strict-Transport-Security "max-age=31536000"
  }
  ```
  and `import hsts` in each of the three site blocks. Nothing else goes in it: no CSP and no other headers, because the apps own those. Optionally add `-Server` in the same `header` line to hide the Caddy server banner; this is harmless.
- Verify with `docker run --rm -v ${PWD}/Caddyfile:/etc/caddy/Caddyfile:ro -e WEB_DOMAIN=example.test -e ADMIN_DOMAIN=admin.example.test -e API_DOMAIN=api.example.test -e ACME_EMAIL=a@example.test caddy:2-alpine caddy validate --config /etc/caddy/Caddyfile` (and `caddy adapt`). No real prod deploy exists; the post-deploy check is `curl -I https://<domain>` once prod is live.

## Existing Patterns

- `backend/internal/transport/http/middleware/` (auth.go): middleware location and test style. Test with `httptest` + `echo.New()`.
- Covers handler (`backend/internal/modules/covers/transport/http/handler.go`): it already sets nosniff explicitly.

## Tests

- Go `security_headers_test.go`:
  - A normal JSON route, a 404 (unknown path) and an `/swagger/index.html` path each get the expected CSP and fixed headers.
  - No `Strict-Transport-Security` on any path.
  - Headers are present on an error returned by the handler.
  - A CORS preflight through `e.Use(SecurityHeaders()); e.Use(CORS...)` still has `Access-Control-Allow-Origin`.
- Vitest per app for the header helper:
  - Prod values match the table.
  - dev adds `'unsafe-eval'` and `ws:`.
  - The API origin is derived from `NEXT_PUBLIC_API_URL` and left out when it is empty.
  - Admin includes `X-Robots-Tag` and web does not.
- No native `next build`; CI's frontend job is the build gate (typedRoutes).
- The manual browser smoke and curl checks from the Acceptance Criteria are run on local compose.

## Documentation

- `docs/DEPLOYMENT.md`:
  - Add a short "Security headers" note: who sets what (apps vs Caddy HSTS), the CSP's `'unsafe-inline'` trade-off, and that HSTS has no preload.
  - Add a security checklist item: `[x] security headers on web/admin/api; [ ] after first TLS deploy, curl -I each domain and confirm HSTS + CSP; consider includeSubDomains/preload later`.
- Mark this contract `Status: done` when the work lands, and close #197.

## Risks

- **CSP breaking pages.** The main risks are an unanticipated external host (e.g. a cover/photo on `http:` rather than `https:`, which would be blocked by `img-src https:`), dev HMR, and Swagger. Mitigations are the browser smoke on both apps with devtools open and the dev-only relaxations. Mixed-content `http:` images would already be blocked on a TLS prod, so this changes nothing there.
- **Weak script protection.** `'unsafe-inline'` scripts make the CSP mostly a framing/object/base-uri/form-action/third-party-script guard, not an XSS stopper. Follow-up: nonce-based CSP via middleware if the XSS surface grows. Record it as a backlog note, not in this change.
- **Build-time API origin.** `connect-src` uses the API origin as it was at build time. A prod image built with the wrong `NEXT_PUBLIC_API_URL` would block every API call. This is already true of the client itself, so it is not a new failure mode.
- **Duplicate headers.** Next.js rewrites pass backend headers through, so the `headers()` source must exclude `/api/` and `/covers/`. The acceptance check covers this.
- **HSTS is sticky for a year.** Without `includeSubDomains`/`preload` it is easy to back out by lowering `max-age`.
