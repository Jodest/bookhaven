# Silent Token Refresh

Status: done

## Goal

Stop logging users out after ~15 minutes of access-token life when they still hold a valid 30-day refresh token. The backend refresh flow (`POST /api/v1/auth/refresh`) is complete and correct; nothing on the frontend ever calls it. Fix is frontend-only: retry transparently on a 401 (reactive), and refresh proactively before expiry via a timer, for both `apps/web` and `apps/admin`, which share `packages/api-client`.

## User Stories

- As a logged-in user who has been idle (or just has an app tab open) for more than 15 minutes, when I take my next action, I want it to succeed transparently instead of bouncing me to `/login`, as long as my refresh token (30 days) is still valid.
- As an admin user performing a mutation (create/update book, author, genre, moderate review) with an expired 15-minute bearer token, I want the mutation to succeed transparently via a silent refresh instead of failing with 401.
- As a user whose refresh token is itself expired or revoked (e.g. truly stale session, or logged out elsewhere), I still want to be correctly logged out / redirected to `/login` — this feature must not weaken that case.

## Acceptance Criteria

- [ ] A 401 from any API call (except the auth endpoints listed below) triggers exactly one call to `POST /api/v1/auth/refresh`; on success, the original request is retried once with fresh credentials and its result is returned to the caller transparently (caller code, e.g. `auth-store.fetchUser`, sees no error).
- [ ] `/api/v1/auth/login`, `/api/v1/auth/register`, `/api/v1/auth/refresh`, and `/api/v1/auth/logout` are excluded from the 401-retry path (a 401 from these means "bad credentials" / "refresh itself is dead" / "already logging out", not "access token expired" — retrying would be wasteful or recursive).
- [ ] If refresh fails (refresh token expired/revoked/absent), the original 401 propagates to the caller unchanged, and existing logout/redirect behavior (auth-store setting `user: null, isAuthenticated: false`, `middleware.ts` redirecting to `/login`) still applies. No behavior change for genuinely dead sessions.
- [ ] Concurrent requests that 401 around the same time (e.g. a page firing several parallel fetches) trigger only one `/auth/refresh` call; all of them await the same in-flight promise and are retried once that resolves. No second parallel refresh call is ever fired while one is in flight (avoids the rotation race — the backend revokes the old refresh token on every refresh, so a second parallel call would legitimately fail).
- [ ] A proactive refresh fires automatically before the access token expires for an active session, so most users never observe a 401 in normal use. Implemented as a timer scheduled at ~80% of the access-token TTL.
- [ ] The proactive timer is (re)scheduled on: login success, register success, successful refresh (reactive or proactive), and app bootstrap (`fetchUser` succeeding with an existing valid session). It is cleared on logout (including admin's `signOut`/`signOutAndRedirect`).
- [ ] `apps/admin`'s in-memory bearer token (`setAccessToken`) and its `localStorage` persistence (`admin_access_token`) are updated after *any* successful refresh, including one triggered reactively deep inside `fetcher()` — not just ones the admin auth-store initiated itself.
- [ ] `apps/web` and `apps/admin` behave consistently: same dedup/retry semantics from the shared `fetcher()`, same proactive-timer trigger points in their respective auth-stores.
- [ ] No change to backend token TTLs, rotation, or cookie flags.
- [ ] No change to `POST /api/v1/auth/refresh` request/response contract.

## Business Rules

- Only one refresh may be in flight at a time, shared across (a) any number of concurrent 401s and (b) the proactive timer — reactive and proactive refreshes must dedupe against each other too, since the backend rotates the refresh token on every call.
- A request is retried after refresh at most once. If the retried request itself 401s again, that failure is surfaced as-is (no second refresh attempt) — this should only happen if the brand-new access token is somehow already invalid, which indicates a real problem, not a transient race.
- Refresh must never be attempted for the excluded auth endpoints (see Acceptance Criteria) to avoid recursion and nonsensical retries.
- The proactive timer's schedule is best-effort, not authoritative: if it fires too early, late, or not at all (e.g. browser throttles `setTimeout` in a backgrounded tab), the reactive 401-retry path is the correctness backstop. The proactive path exists purely to reduce how often users hit a 401 in the first place.

## UX

- No new UI/screens. The user-visible change is the *absence* of unexpected logouts during a valid 30-day session.
- No loading-state changes needed for the retried request — the retry+original request round trip is invisible to callers (they just get their normal promise, slightly later).
- Optional (recommended, not required for the fix): re-validate the session (call `fetchUser()`) on `visibilitychange` → visible, to recover promptly if a proactive timer was throttled while the tab was hidden/backgrounded (e.g. laptop sleep) — the very next click would trigger reactive refresh anyway, so this only smooths the "resume from sleep" case.

## API

None — no backend/API changes. `POST /api/v1/auth/refresh` already exists, is already correct, and its contract (`access_token`, `refresh_token`, `expires_in`, cookie `Set-Cookie` for both `access_token` and `refresh_token`) is unchanged. Verified against `backend/internal/modules/auth/transport/http/handler.go` (`Refresh`, `setAuthCookies`) and `backend/internal/modules/auth/domain/tokens.go` (`TokenPair`).

## Database

None.

## Frontend

### `packages/api-client/src/index.ts` (shared by `apps/web` and `apps/admin`)

- Add a module-level in-flight guard, e.g. `let refreshPromise: Promise<{ accessToken: string; refreshToken: string; expiresIn: number }> | null = null;`, and a `performRefresh()` function that:
  - Returns `refreshPromise` if one is already pending (dedup).
  - Otherwise starts one raw `POST /api/v1/auth/refresh` call (bypassing the 401-retry logic below, to avoid recursion), stores it in `refreshPromise`, and clears `refreshPromise` in a `finally`.
  - On success: calls `setAccessToken(newAccessToken)` internally (this alone keeps admin's bearer header fresh for any *subsequent* request even before app code reacts) and notifies subscribers (see `onTokenRefresh` below).
  - Have `api.auth.refresh()` call `performRefresh()` instead of hitting the endpoint directly, so proactive-timer-triggered refreshes and reactive 401-triggered refreshes always share the same dedup guard.
- Add a tiny pub/sub so app-level auth-stores can react to a refresh that happened *inside* `fetcher()` (invisible to them otherwise): `export function onTokenRefresh(listener: (tokens: { accessToken: string; refreshToken: string; expiresIn: number }) => void): () => void` (returns an unsubscribe function). Called once per successful `performRefresh()`.
- Refactor `fetcher()`:
  - Extract header-building (the existing bearer-attach-for-admin-mutations logic) into a small local function so it can be re-run on retry with a possibly-new `accessToken`.
  - Define an exclusion list of paths that must never trigger auto-refresh: `/api/v1/auth/login`, `/api/v1/auth/register`, `/api/v1/auth/refresh`, `/api/v1/auth/logout`.
  - On a 401 response for a non-excluded path, and only if this is the first attempt for this call (guard against double-retry): call `performRefresh()`. If it resolves, rebuild headers and re-issue the original `fetch` exactly once, returning that result (or throwing if it errors). If `performRefresh()` rejects, throw the *original* `ApiError` (not the refresh failure) so existing "treat 401 as logged out" logic in callers is unaffected.
- Export a `scheduleTokenRefresh(expiresInSeconds: number, onDue: () => void): () => void` helper (plain `setTimeout` at `expiresInSeconds * 0.8`, clamped to a small minimum e.g. 5s; returns a cancel function) so both apps schedule proactive refreshes identically instead of duplicating the math.
- Export a constant, e.g. `export const FALLBACK_ACCESS_TOKEN_TTL_SECONDS = 15 * 60;` with a comment pointing at `backend/cmd/api/main.go`'s `AccessTokenTTL`, for use when scheduling a proactive refresh from a context that doesn't have a real `expires_in` (see `fetchUser` below). This is the one place a manual sync with the backend constant is required — see Risks.

### `apps/web/src/entities/session/model/auth-store.ts` and `apps/admin/src/entities/session/model/auth-store.ts`

- Both stores: import `onTokenRefresh`, `scheduleTokenRefresh`, `FALLBACK_ACCESS_TOKEN_TTL_SECONDS` from `@bookhaven/api-client`.
- Module-level `let cancelRefreshTimer: (() => void) | null = null;` plus a local `scheduleRefresh(expiresIn: number)` helper that cancels any existing timer and schedules a new one whose `onDue` calls `api.auth.refresh()` (errors ignored — a failed proactive refresh just falls through to the reactive path on the next real request).
- Register `onTokenRefresh((tokens) => scheduleRefresh(tokens.expiresIn))` once at module load (same place admin already restores `initialToken` from `localStorage`), so *any* successful refresh — proactive or reactive, from inside `fetcher()` or from this store's own timer — reschedules the next one with an accurate `expires_in`. In `apps/admin`, this same module-load subscription also calls `setStoredToken(tokens.accessToken)` to persist the rotated bearer token to `localStorage` (this is the piece that would otherwise be missed for reactive refreshes triggered deep inside `fetcher()`).
- `login`/`register` (web) and `login` (admin): after setting `user`/`isAuthenticated`, call `scheduleRefresh(response.expiresIn)` using the real value from the login/register response.
- `fetchUser`: on success (`user` non-null, i.e. an existing session is still valid), call `scheduleRefresh(FALLBACK_ACCESS_TOKEN_TTL_SECONDS)` — approximate, since `GET /api/v1/auth/me` doesn't return `expires_in`; acceptable because the reactive path backstops any inaccuracy (see Risks).
- `logout` (web/admin), `signOut`, `signOutAndRedirect` (admin): call `cancelRefreshTimer?.()` and clear the reference, in addition to existing cleanup.

### `apps/web/src/entities/session/ui/session-initializer.tsx` and `apps/admin/src/entities/session/ui/session-initializer.tsx`

- Optional but recommended: add a `visibilitychange` listener that calls `fetchUser()` when `document.visibilityState === 'visible'`, to promptly recover a session after the tab was hidden/backgrounded long enough for the browser to throttle the proactive timer past expiry. Clean up the listener on unmount.

## Existing Patterns

- `packages/api-client/src/index.ts` — `fetcher()`, `setAccessToken`/`getAccessToken`, `api.auth.refresh()` (already defines the exact request/response shape to reuse; just needs to be invoked from somewhere).
- `apps/admin/src/entities/session/model/auth-store.ts` — the existing module-load pattern that restores `admin_access_token` from `localStorage` and calls `setAccessToken` is the right place to add the `onTokenRefresh` subscription (same lifecycle: runs once when the module is first imported).
- `apps/web/src/entities/session/ui/session-initializer.tsx` / `apps/admin/.../session-initializer.tsx` — identical `useEffect(() => { fetchUser(); }, [fetchUser])` pattern already establishes where app-bootstrap session logic lives; the `visibilitychange` addition follows the same shape.

## Tests

- `packages/api-client`: unit tests for `fetcher()`'s 401-retry behavior (mock `fetch`):
  - A single 401 followed by a successful refresh results in exactly one call to `/auth/refresh` and one retried call to the original path, resolving successfully.
  - Refresh failure (refresh endpoint itself returns non-2xx) causes the *original* `ApiError` (matching the original path/status) to propagate, not a refresh-specific error.
  - N concurrent 401s (e.g. 3 parallel `fetcher()` calls) result in exactly one `/auth/refresh` call total.
  - 401 from `/api/v1/auth/login`, `/api/v1/auth/register`, `/api/v1/auth/refresh`, `/api/v1/auth/logout` never triggers a refresh call.
  - A retried request that itself 401s again does not trigger a second refresh (no infinite loop).
- `packages/api-client`: unit test for `scheduleTokenRefresh` — schedules at ~80% of the given TTL (fake timers), and the returned cancel function prevents `onDue` from firing.
- `apps/web` / `apps/admin` auth-store: unit tests that `login`/`fetchUser` schedule a refresh timer, `logout` cancels it, and that `onTokenRefresh` callback reschedules using the new `expires_in`. For `apps/admin` specifically, assert `localStorage`'s `admin_access_token` is updated after a simulated `onTokenRefresh` event, not just after an explicit `login()`.

**Implementation note:** `packages/api-client` and `apps/admin` have no test harness configured (no `vitest`/`jest` config or `devDependency`, no `test` script) as of this implementation, so the `fetcher()`/`scheduleTokenRefresh()` unit tests and the `apps/admin` auth-store test were not added, per the constraint to not scaffold a new test harness from scratch for this fix. `apps/web` already has `vitest` configured; the `apps/web` auth-store tests listed above were added there (`apps/web/src/entities/session/model/auth-store.test.ts`), covering the login/register/fetchUser scheduling, logout cancellation, and `onTokenRefresh` rescheduling cases. A follow-up could add a `vitest` setup to `packages/api-client` and `apps/admin` to cover the remaining cases.

## Documentation

- Update `docs/ROADMAP.md`: no new top-level entry needed (this is a bugfix within the already-completed "Frontend authentication" item) — add a short note/link to this contract under "Completed" next to that line, consistent with how `auth-return-url.md` was referenced for the redirect fix (that one wasn't linked from ROADMAP either, per current content — follow whatever convention the maintainer prefers; not blocking).
- Update this file's `Status:` to `done` after implementation, per the convention seen in `docs/features/auth-return-url.md`.

## Risks

- **TTL constant drift**: `FALLBACK_ACCESS_TOKEN_TTL_SECONDS` in `packages/api-client` duplicates the backend's `AccessTokenTTL` (`backend/cmd/api/main.go`, currently 15 minutes) as a hardcoded value, because `GET /api/v1/auth/me` doesn't return `expires_in` and the access token itself is in an HttpOnly cookie the frontend can't read. If the backend TTL ever changes, this constant must be updated manually or the proactive timer for the "existing session on page load" case becomes inaccurate. Mitigation: the reactive 401-retry path is a full correctness backstop regardless of this constant being stale — worst case is more 401-then-retry round trips, not broken auth. A future improvement (out of scope here per the stated constraint of not touching the backend unless strictly required) would be to have `/auth/me` also return `expires_in` (or `expires_at`), removing the need for this constant entirely.
- **Backgrounded-tab timer throttling**: browsers throttle/delay `setTimeout` in hidden tabs, so the proactive timer may fire late or effectively not at all for long-backgrounded tabs. Mitigated by the reactive path (always correct) and the optional `visibilitychange` recheck.
- **Retry correctness for non-idempotent requests**: retrying the exact same `RequestInit` (including body) after a 401 is safe here because a 401 means the request was rejected by auth middleware *before* any handler logic ran (no partial side effects) — must confirm this holds for all routes (i.e. auth middleware runs before any mutation logic) when implementing, but this matches standard Echo middleware ordering and is not expected to be an issue.
- **Header rebuild on retry**: if the retry path forgets to rebuild the `Authorization` header with the new token (reusing the stale pre-refresh `Headers` object instead), admin mutations would retry with the *old* expired token and fail again — this is the most likely implementation slip, called out explicitly in the Frontend section.
- **Recursion guard**: `performRefresh()`'s own request must not itself go through the 401-retry path (it's already excluded by path), otherwise a dead refresh token could cause infinite recursion.

## Sub-issue scope note

Backend is **not** touched by this feature — the entire fix lives in `packages/api-client`, `apps/web`, and `apps/admin`. A single `[frontend]` sub-issue covering all three is appropriate; no `[backend]` sub-issue is needed.
