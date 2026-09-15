# Middleware Session Persistence

Status: done

## Goal

Stop `apps/web/src/middleware.ts` and `apps/admin/src/middleware.ts` from hard-redirecting to `/login` purely because the 15-minute `access_token` cookie has expired (been deleted by the browser), when a still-valid 30-day refresh token exists in the same browser. Let the already-shipped client-side reactive/proactive refresh (`silent-token-refresh.md`, done) get a chance to run by letting the page render instead of short-circuiting at the Edge, before `Providers`/`SessionInitializer`/React ever mount.

## User Stories

- As a user who closes the tab/app and returns hours or days later (well within the 30-day refresh-token window), when I load a protected page (`/profile`, `/shelves`), I want to land on the page — not get redirected to `/login` — and have my session silently restored, matching how "modern sites" behave.
- As an admin whose `access_token` cookie has expired between visits, I want the same behavior for admin's protected routes.
- As a user whose refresh token is truly expired/revoked/absent (or who explicitly logged out), I still want to be redirected to `/login` for protected routes — this fix must not create an unauthenticated-access hole or weaken real logout.

## Acceptance Criteria

- [ ] Loading/reloading a protected route (`/profile`, `/shelves` in `apps/web`; any non-`/login` route in `apps/admin`) after the 15-min `access_token` cookie has expired, but while the 30-day refresh token is still valid, no longer hard-redirects at the middleware/Edge layer — the page renders and the existing client-side reactive-refresh path silently restores the session (no visible re-login, no manual action).
- [ ] Loading a protected route with no session at all (never logged in, explicitly logged out, or a refresh token that has itself naturally expired in the browser) still redirects to `/login` at the middleware layer, unchanged from today.
- [ ] Explicit logout clears every session-presence signal so protected routes correctly hard-gate immediately after logout (no regression).
- [ ] No change to backend token TTLs, rotation, the existing `refresh_token` cookie's `Path`, or the shipped client-side reactive/proactive refresh logic in `packages/api-client` or either app's `auth-store.ts`/`session-initializer.tsx`.
- [ ] `apps/admin`'s middleware receives the equivalent fix for its protected routes (same bug shape confirmed present there).
- [ ] `apps/web`'s `authRoutes` redirect-away-from-`/login`-when-authenticated branch is explicitly left unchanged/out of scope for this fix (documented, not silently forgotten — see Risks).

## Business Rules

- A new, non-sensitive marker cookie `has_session` is the sole new signal middleware uses to decide "maybe still logged in, let the client resolve it" vs. "definitely logged out, hard-redirect now."
- `has_session` carries no authorization value by itself. No request is ever authorized based on it — the backend still authorizes every API call via the real access token (cookie or bearer) exactly as today. It exists purely to prevent a false-negative hard-redirect at the Edge before the client gets a chance to run the real (already-shipped) refresh flow.
- `has_session` is always set together with `refresh_token` (same request, same `MaxAge` = 30 days) and always cleared together with it, so the two can never diverge in the browser — since both are set at the same instant with the same lifetime, once `has_session` naturally expires from the browser, the refresh token cookie has necessarily expired in that same browser too.
- Protected-route middleware redirects to `/login` if and only if both `access_token` and `has_session` are absent.

## UX

- No new UI/pages. For the "expired access token, valid refresh token" case, the user now briefly sees the page's existing loading/unauthenticated-fallback state (`profile-view.tsx`'s "Загрузка профиля...", `shelves-view.tsx`'s loading state, admin `Shell`'s "Загрузка...") which then resolves to the authenticated view within one `fetchUser()` round trip (the reactive-refresh-on-401 path already built), instead of a hard bounce to `/login`.
- If the refresh genuinely fails (see Risks — revoked/expired token whose cookies haven't yet naturally expired), the user ends up seeing the same "not authenticated" fallback UI that already exists (`profile-view.tsx`'s "Необходимо войти в аккаунт.", admin `Shell`'s redirect-to-login effect) — no new failure UI needed.

## API

No REST contract changes to `POST /api/v1/auth/login`, `/register`, `/refresh`, `/logout` request/response bodies. Only their `Set-Cookie` behavior changes (one additional cookie, set/cleared alongside the existing two).

## Database

None.

## Backend

`backend/internal/modules/auth/transport/http/handler.go`:

- `setAuthCookies`: add a third cookie alongside `access_token`/`refresh_token`:
  ```go
  c.SetCookie(&http.Cookie{
      Name:     "has_session",
      Value:    "1",
      HttpOnly: true, // no legitimate reason for client JS to read/write it
      Secure:   false, // matches existing cookies; set true in production with HTTPS
      SameSite: http.SameSiteStrictMode,
      Path:     "/",
      MaxAge:   60 * 60 * 24 * 30, // 30 days — must always match refresh_token's MaxAge
  })
  ```
- `clearAuthCookies`: clear `has_session` the same way `access_token`/`refresh_token` are cleared (`Value: ""`, `MaxAge: -1`, `Path: "/"`).
- No other backend changes. `RefreshTokens`/rotation logic in `backend/internal/modules/auth/service/service.go` is untouched — `has_session` is a dumb presence flag, never validated or rotated server-side.

## Frontend

`apps/web/src/middleware.ts`:

- Read `has_session` in addition to `access_token`.
- Change the protected-route gate from `!isAuthenticated` to `!isAuthenticated && !hasSession` (where `hasSession = Boolean(request.cookies.get('has_session')?.value)`).
- Leave the `authRoutes` branch (redirect away from `/login`/`/register` when `isAuthenticated`) based on `access_token` only, unchanged — out of scope, see Risks.

`apps/admin/src/middleware.ts`:

- Same treatment: read `has_session`, change `if (!accessToken)` to `if (!accessToken && !hasSession)`.
- No `authRoutes`-equivalent branch exists here (admin's `config.matcher` already excludes `/login` entirely), so no additional decision needed for that case.
- No change needed to `apps/admin/src/widgets/admin-shell/ui/shell.tsx` — it already renders a "Загрузка..." state while `isLoading` and redirects client-side via `router.push` only after `fetchUser()` resolves with no user, so it already tolerates the page mounting before auth is known (same role as web's `profile-view.tsx`).

No changes to `packages/api-client`, either app's `auth-store.ts`, or `session-initializer.tsx` — the existing reactive/proactive refresh machinery is reused as-is; it only needs the chance to run, which not hard-redirecting at the Edge provides.

## Existing Patterns

- `apps/web/src/views/profile/ui/profile-view.tsx` / `apps/web/src/views/shelves/ui/shelves-view.tsx` — already-existing client-side loading/unauthenticated fallback states; reused as the acceptable UX surface for the brief resolution window, no new loading UI needed.
- `apps/admin/src/widgets/admin-shell/ui/shell.tsx` — already-existing `isLoading` guard + client-side redirect effect, same role for admin.
- `backend/internal/modules/auth/transport/http/handler.go`'s `setAuthCookies`/`clearAuthCookies` — the natural, already-established place to add/remove a cookie alongside the existing two.
- `packages/api-client/src/index.ts`'s `fetcher()` 401-retry + `apps/*/entities/session/model/auth-store.ts`'s proactive timer — the already-shipped machinery this fix is designed to let run, untouched here.

## Tests

- Backend: add a unit test on `setAuthCookies`/`clearAuthCookies` (constructing an `echo.Context` and inspecting the `Set-Cookie` response headers) asserting `has_session` is set on `Register`/`Login`/`Refresh` with the same `MaxAge` as `refresh_token`, and cleared on `Logout`. Follow whatever test pattern (or its absence) already exists in `backend/internal/modules/auth/transport/http/` before deciding whether to add a new test file.
- `apps/web`/`apps/admin` middleware: unit test the exported `middleware` function against a minimal mock/`NextRequest` (needs only `.cookies.get()` and `.nextUrl`), covering: (a) protected route + no cookies → redirect; (b) protected route + `access_token` only → no redirect; (c) protected route + `has_session` only (the new case this fix adds) → no redirect; (d) protected route + neither → redirect. `apps/web` already has `vitest` configured (per the prior feature's implementation note); add these there. `apps/admin` has no test harness yet — apply the same judgment call the prior feature applied (don't scaffold a harness from scratch just for this), and note the gap if skipped.
- Manual verification (both apps): log in, delete the `access_token` cookie via devtools (simulating its 15-min expiry) while leaving `refresh_token`/`has_session` intact, then hard-reload a protected route — page should render and silently restore the session (watch the network tab for a `POST /api/v1/auth/refresh` firing on load, no redirect to `/login`). Then delete all cookies and reload the same route — should redirect to `/login` as before.

## Documentation

- Add a note/link in `docs/ROADMAP.md`'s "Completed" section next to the existing silent-token-refresh line, pointing at this contract, once implemented — same convention as the prior fix.
- Update this file's `Status:` to `done` after implementation.

## Risks

- **`authRoutes` branch left as-is in `apps/web`**: the redirect-away-from-`/login`-when-authenticated check still keys off `access_token` alone. A user with an expired access token but a valid `has_session`/refresh token who navigates directly to `/login` will briefly see the login page render instead of being bounced to `/profile`. Pre-existing gap, not introduced by this fix, and out of scope per the reported bug (which is about protected routes wrongly redirecting *away*, not `/login` wrongly rendering for an already-valid session). Low impact: `SessionInitializer` still runs on `/login` and restores the session client-side. Flag as a possible small follow-up, not blocking this contract.
- **Optimistic pass-through on a revoked-but-not-yet-expired refresh token**: if a refresh token is ever revoked server-side before its cookie naturally expires in the browser, `has_session` will still be present, so middleware lets the request through optimistically. The client-side `fetchUser()` → reactive-refresh-on-401 will then correctly fail, and the auth-store falls back to the existing "not authenticated" UI. Not a security hole — no protected data renders without a real, separately-authorized API call succeeding — just one extra failed round trip versus a hard Edge redirect. Accepted trade-off per the brief.
- **Cookie drift risk**: `has_session` is a parallel, manually-kept-in-sync cookie (not derived from `refresh_token` itself). Any future edit to `refresh_token`'s `MaxAge` without updating `has_session`'s identically would break the "always expire together" invariant this fix relies on. Mitigate by keeping both `SetCookie` calls adjacent in `setAuthCookies`/`clearAuthCookies` and adding a code comment cross-referencing them.
- **`Secure: false` on new cookie**: intentionally mirrors the existing `access_token`/`refresh_token` flags (already has a `// Set to true in production with HTTPS` TODO) for consistency — inherited pre-existing issue, not newly introduced or addressed here.
- **Considered and rejected — widen `refresh_token`'s own `Path` to `/`**: would cause the actual sensitive, `HttpOnly` refresh token to be transmitted on every same-origin request (all page navigations, RSC fetches, etc. — confirmed both `apps/web` and `apps/admin` proxy `/api/*` to the backend via Next.js `rewrites()` in their respective `next.config.js`, so these cookies are already first-party to each app's own origin), not just the one endpoint that needs it, for no functional benefit over a boolean presence flag. `has_session` achieves the same Edge-visibility goal with strictly less exposure of the sensitive token, since it carries no authorization value of its own.
- **Considered and rejected — refresh directly inside `middleware.ts`**: rejected primarily due to the concurrent-refresh race. The backend rotates/revokes the refresh token on every use (`RefreshTokens` in `backend/internal/modules/auth/service/service.go`), and Edge middleware invocations are stateless per-request with no in-process mutex across concurrent invocations (unlike the client-side fix's single in-memory promise dedup) — parallel requests (multiple tabs, or a navigation racing a prefetch) could each attempt a refresh, with all but one spuriously failing (`ErrTokenInvalid`) and risking a wrongful logout of a valid session. Reusing the already-shipped, already-reviewed client-side reactive-refresh path avoids introducing that new race entirely, at the cost of one extra client round trip that direction wouldn't have needed.

## Sub-issue scope note

Two sub-issues are appropriate for GitHub filing: one `[backend]` (add/clear `has_session` cookie in `handler.go`, plus its test) and one `[frontend]` (both `middleware.ts` files' condition change, plus their tests) — small enough that a single combined issue would also be reasonable if the maintainer prefers not to split it.
