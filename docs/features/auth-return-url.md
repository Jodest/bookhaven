# Auth Return URL

Status: done

## Goal

After an unauthenticated user is redirected to login (either automatically by route protection, or by clicking a "sign in to do X" link), a successful login (or registration) should return them to the page they were originally on, instead of unconditionally sending them to `/profile`.

## User Stories

- As a visitor browsing a book page, when I click "Log in to add a rating/review/shelve/list", after logging in I want to land back on that same book page, not on my profile.
- As a visitor who tries to open a protected page (`/profile`, `/shelves`) directly while logged out, after logging in I want to land on the page I originally requested.
- As a visitor who clicks the generic "Войти" (Log in) link in the header while browsing some page, after logging in I want to return to that page.
- As a new user who registers via `/register`, I want the same return-to behavior as login.

## Acceptance Criteria

- [ ] Middleware-driven redirects to `/login` (protected routes `/profile`, `/shelves`) already set `?redirect=<path>`; this continues to work and is now honored on completion.
- [ ] Middleware captures the full original path including query string (`pathname + search`), not just `pathname`, so filtered/paginated views are preserved.
- [ ] All existing feature-level "log in to do X" links (`shelve-book-buttons`, `book-rating`, `add-to-list-button`, `reviews-section`) continue to pass `?redirect=` — no change needed there, but verify format stays consistent (`redirect` param, relative path).
- [ ] The header's generic "Войти" link also passes `?redirect=<currentPath>` (using `usePathname` + query string) so login from any page returns the user to that page.
- [ ] `LoginForm` reads the `redirect` query param on submit and, on successful login, navigates to it instead of hardcoded `/profile`.
- [ ] `RegisterForm` reads the `redirect` query param the same way and applies the same behavior on successful registration.
- [ ] If `redirect` is absent, invalid, or unsafe, fall back to `/profile` (current behavior) for both login and register.
- [ ] Only same-origin, relative paths are accepted as redirect targets (see Business Rules — open redirect protection). External URLs, protocol-relative URLs (`//evil.com`), and non-`/`-prefixed values are rejected and fall back to `/profile`.
- [ ] Logout flow is unaffected (still redirects to `/`).
- [ ] Already-authenticated users hitting `/login` or `/register` are still bounced to `/profile` by middleware (unchanged, out of scope for this feature).

## Business Rules

- The redirect target must be validated before use to prevent open-redirect vulnerabilities:
  - Must start with a single `/` (relative path).
  - Must not start with `//` or `/\` (protocol-relative / browser-normalized absolute URLs).
  - Must not contain a scheme (`http://`, `https://`, `javascript:`, etc.) — enforced implicitly by requiring a single leading `/`, but validate explicitly to be safe.
  - Recommended implementation: a small shared helper, e.g. `isSafeRedirectPath(value: string | null): value is string`, reused by `LoginForm`, `RegisterForm`, and (if desired) middleware.
- Query param name stays `redirect` (already used across the codebase — do not introduce a second competing name like `returnTo`).
- No redirect-loop guard is required for v1: gated-action links and middleware only ever set `redirect` to real content pages, never to `/login` or `/register` itself. If this changes later, add a check rejecting `redirect` values that point to `/login` or `/register`.

## UX

- No new UI. Behavior-only change: the destination after login/register differs based on where the user came from.
- No loading/error states change; existing error handling in the forms is untouched.

## API

None. This is a frontend-only, client-side navigation change. No backend/API changes required.

## Database

None.

## Frontend

- `apps/web/src/middleware.ts`
  - Change `loginUrl.searchParams.set('redirect', pathname)` to include the search string: `pathname + request.nextUrl.search` (guard against double `?`).
- `apps/web/src/features/auth/ui/login-form.tsx`
  - Add `useSearchParams()` to read `redirect`.
  - Validate with the shared safe-redirect helper; on success, `router.push(safeRedirect ?? '/profile')` instead of hardcoded `/profile`.
- `apps/web/src/features/auth/ui/register-form.tsx`
  - Same change as `login-form.tsx`.
- `apps/web/src/app/login/page.tsx` and `apps/web/src/app/register/page.tsx`
  - Wrap the view in `<Suspense>` (same pattern already used in `apps/web/src/app/books/page.tsx` for `useSearchParams()` consumers), since `LoginForm`/`RegisterForm` will now call `useSearchParams()`.
- `apps/web/src/widgets/header/ui/header.tsx`
  - Use `usePathname()` (and current search params, if trivial) to build `/login?redirect=<currentPath>` for the "Войти" link instead of a bare `/login`.
  - Implementation note: since `Header` is rendered from the root layout on every route, inlining `useSearchParams()` directly in `Header` would require wrapping the entire `Header` (or `layout.tsx`) in `<Suspense>`, de-opting every page's static shell. Instead, the pathname+search logic was extracted into a small `apps/web/src/widgets/header/ui/header-login-link.tsx` client component, which is wrapped in a local `<Suspense>` (fallback: a plain `/login` link) inside `Header`. This scopes the dynamic dependency to just that link.
- New shared helper: `apps/web/src/shared/lib/safe-redirect.ts`
  - `isSafeRedirectPath(value: string | null | undefined): value is string`
  - Used by `login-form.tsx`, `register-form.tsx`. Not used by `middleware.ts` since the middleware only ever sets `redirect` from its own trusted `pathname + search`, never reads/forwards an externally supplied value.

## Existing Patterns

- `apps/web/src/middleware.ts` already establishes the `?redirect=<path>` convention for protected-route bounces.
- `apps/web/src/features/shelve-book/ui/shelve-book-buttons.tsx`, `apps/web/src/features/rate-book/ui/book-rating.tsx`, `apps/web/src/features/add-to-list/ui/add-to-list-button.tsx`, and `apps/web/src/widgets/reviews-section/ui/reviews-section.tsx` already link to `/login?redirect=/books/${slug}` for gated actions — this feature just makes the login/register forms actually honor that param, closing the loop.
- `apps/web/src/app/books/page.tsx` shows the established `<Suspense>` wrapper pattern for pages whose view uses `useSearchParams()`.
- `apps/web/src/views/catalog/ui/catalog-view.tsx` shows the existing pattern for reading/writing query params via `useSearchParams()`/`useRouter()` in this codebase.

## Tests

- Unit test for `isSafeRedirectPath`: accepts `/books/some-slug`, `/profile`, `/shelves?tab=custom`; rejects `//evil.com`, `https://evil.com`, `javascript:alert(1)`, `` (empty), `null`/`undefined`.
- `LoginForm`: on submit with `?redirect=/books/abc` in the URL, after successful `login()`, asserts `router.push('/books/abc')` was called (mock `useSearchParams`/`useRouter`).
- `LoginForm`: on submit with no `redirect` param, asserts fallback to `/profile`.
- `LoginForm`: on submit with an unsafe `redirect` value (e.g. `//evil.com`), asserts fallback to `/profile`.
- Same three cases for `RegisterForm`.
- Middleware test: skipped — no middleware test precedent existed in this repo, and the contract permits skipping in that case. Verified manually via code review of the one-line change (`pathname` → `pathname + search`) and via `next build` succeeding.
- No frontend unit test runner (Vitest/Jest) previously existed in this repo. Added `vitest` + `@testing-library/react` + `@testing-library/jest-dom` + `jsdom` + `@vitejs/plugin-react` as devDependencies to `apps/web` (with `vitest.config.ts` / `vitest.setup.ts` and a `test` script), scoped to `apps/web`, to satisfy this feature's explicit test requirements. No changes to `apps/admin` or other packages.

## Documentation

- No ROADMAP entry needed (this is a small fix within the already-completed "Frontend authentication" item), unless the team wants a changelog note. Update `docs/features/auth-return-url.md` status to `done` after implementation, per existing convention seen in other `docs/features/*.md` files.

## Risks

- `useSearchParams()` requires a `<Suspense>` boundary around the login/register pages; forgetting this will cause a Next.js build/runtime error or de-opt the whole route to client-only rendering. Mitigation: follow the `books/page.tsx` precedent exactly.
- Open redirect if validation is skipped or implemented loosely (e.g., only checking `startsWith('/')` without excluding `//`). Mitigation: dedicated helper + unit tests covering the bypass cases above.
- Middleware change to include `request.nextUrl.search` must correctly handle the case where `search` is empty (avoid producing `path?` with a trailing empty query).
- Low risk of regressing the "already authenticated visits /login" bounce-to-profile behavior if the safe-redirect logic is accidentally applied there too — this feature intentionally leaves that middleware branch unchanged.
