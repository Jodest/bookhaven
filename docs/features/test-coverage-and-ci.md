# Test Coverage and CI (Catalog Backend Gap + Admin Frontend Test Setup)

Status: implemented

## Goal

Close the two remaining concrete, code-level gaps identified in the "Tests, CI/CD and Production" roadmap item:

1. The `catalog` backend module has no `service`/`transport` test coverage while every other module does.
2. `apps/admin` has no frontend test tooling at all, and the CI `frontend` job never runs any test step even though `apps/web` already has runnable tests.

This is a test-infrastructure/coverage pass only. Real server/VPS/domain provisioning ("Production") is explicitly out of scope and must not be touched — see `docs/PIPELINE.md` and `docs/DEPLOYMENT.md`.

## Non-Goals

- Provisioning or configuring the production VPS/domain (`docker-compose.prod.yml`, `Caddyfile`, `.github/workflows/deploy.yml`) — out of scope, manual human action.
- Introducing a new test framework, mocking library, or assertions library anywhere (backend keeps stdlib `testing` + hand-rolled fakes; frontend keeps vitest + Testing Library, mirrored 1:1 from `apps/web`).
- Full coverage of `catalog` or `apps/admin` — only bring `catalog` up to the same level as sibling modules (happy-path + key error-path table-driven tests), and only add 2-4 representative `apps/admin` tests.
- Coverage reporting, coverage badges, or other new CI features — trivially cheap ones are not being added either, to keep this PR narrowly scoped. Call out explicitly: rejected as out of scope.
- Any change to `docker-compose.prod.yml`, `Caddyfile`, `.github/workflows/deploy.yml`.

## Acceptance Criteria

- [ ] `backend/internal/modules/catalog/service/service_test.go` exists with table-driven tests covering all `Service` methods (`ListBooks`, `GetBookBySlug`, `GetAuthorBySlug`, `ListGenres`, `ListAuthors`, `ListLanguages`, `ParseUUID`), including the not-found error-translation paths (`ErrBookNotFound`, `ErrAuthorNotFound`).
- [ ] `backend/internal/modules/catalog/transport/http/handler_test.go` exists with tests covering all six `Handler` methods (`ListBooks`, `GetBook`, `ListAuthors`, `GetAuthor`, `ListGenres`, `ListLanguages`), including query-param parsing edge cases (pagination defaults/clamping, invalid IDs ignored) and 404 responses.
- [ ] `go test ./...` in `backend/` passes and includes the new catalog tests (verify via `go test ./internal/modules/catalog/...`).
- [ ] `apps/admin/package.json` has vitest devDependencies and a `"test": "vitest run"` script mirroring `apps/web/package.json` exactly (same versions).
- [ ] `apps/admin/vitest.config.ts` and `apps/admin/vitest.setup.ts` exist, mirroring `apps/web`'s files (jsdom environment, `@` path alias to `./src`, `@testing-library/jest-dom/vitest` setup).
- [ ] At least 3 new test files exist under `apps/admin/src/**` covering: a pure utility (`shared/lib/redirect.ts`), a Zustand store/hook (`entities/session/model/auth-store.ts`), and a form component with validation (`features/auth/ui/login-form.tsx`).
- [ ] `pnpm --filter @bookhaven/admin test` (and root `pnpm test` via turbo) runs and passes the new admin tests without touching `apps/web`'s existing tests.
- [ ] `.github/workflows/ci.yml`'s `frontend` job runs a test step (after lint/type-check, before or after build) that executes both `apps/web`'s and `apps/admin`'s tests, so a failing test fails the PR check.
- [ ] `docs/ROADMAP.md`'s "Tests, CI/CD and Production" line is left as-is (still references server provisioning as the remaining piece) — this contract only closes the two sub-gaps; do not mark the roadmap item done.

## Business Rules

- No behavior change to catalog's public API — this is test-only work; if a test reveals an actual bug, flag it as a risk rather than silently "fixing" behavior beyond what's needed to make an accurately-specified test pass.
- Catalog's public endpoints (`GET /books`, `/books/:slug`, `/authors`, `/authors/:slug`, `/genres`, `/languages`) require no authentication (confirmed in `backend/internal/transport/http/router.go` lines 41-46 — no `authMiddleware`/`optionalAuth` on any of them). Handler tests should not assert any auth requirement for catalog; this differs from `ratings`/`lists`/`reviews` which do require auth — do not copy their auth-check test cases into catalog by rote.

## Existing Patterns (reuse, do not reinvent)

Backend:
- Fake repository style: hand-rolled struct implementing the module's `Repository` interface backed by in-memory maps, no mocking framework. See `backend/internal/modules/ratings/service/service_test.go` (`fakeRepository`) and `backend/internal/modules/lists/transport/http/handler_test.go` (separate `fakeRepository` defined again inside the `http` test package — each package's test file defines its own minimal fake; they are not shared/exported across packages).
- Assertions: plain `if ... { t.Fatalf(...) }` / `errors.Is` — no `testify` anywhere in this codebase. Keep that convention in catalog's new tests.
- Handler tests: construct `echo.New()`, `httptest.NewRequest`/`NewRecorder`, `e.NewContext(...)`, then `c.SetPath`/`c.SetParamNames`/`c.SetParamValues` for path params and `c.Set("user_id", ...)` for auth context (not needed for catalog, since its routes are public — but keep the same `newContext` helper shape used in `lists/transport/http/handler_test.go`).
- `catalog`'s `repository.Repository` interface (6 methods) is smaller and simpler than `lists`'/`ratings`' — the fake for catalog only needs in-memory slices/maps for books, authors, genres, and languages, keyed the same way `catalog/repository/repository_test.go` already sets up its `domain.Book`/`domain.Author` fixtures (reuse those fixture-construction ideas for consistency, but this is a fresh fake, not a DB-backed test).

Frontend:
- `apps/web/vitest.config.ts` + `apps/web/vitest.setup.ts` — copy verbatim (adjust nothing except this being a different app root, paths are already relative).
- `apps/web/package.json` devDependencies block (`@testing-library/jest-dom`, `@testing-library/react`, `@vitejs/plugin-react`, `jsdom`, `vitest`, `@types/node` already present in admin) — add the missing ones at the exact same versions.
- Test style for a pure function: `apps/web/src/shared/lib/safe-redirect.test.ts` (table-driven via `it.each`). Mirror for `apps/admin/src/shared/lib/redirect.ts` (`isSafeInternalRedirect`, `sanitizeRedirect`) — note admin's version takes an explicit `baseOrigin` param (unlike web's single-arg version), so tests differ slightly in call shape but should cover the same cases (safe path, `//evil.com`, `https://evil.com`, backslash variants, control characters, wrong-origin resolution).
- Test style for a Zustand store: `apps/web/src/entities/session/model/auth-store.test.ts` — mocks `@bookhaven/api-client` (`api.auth.*`, `onTokenRefresh`, `scheduleTokenRefresh`, `FALLBACK_ACCESS_TOKEN_TTL_SECONDS`) with `vi.mock` + a hoisted `var` listener capture. Admin's `auth-store.ts` has extra surface (`isSigningOut`, `signOut`, `signOutAndRedirect`, `forbiddenPath`) not present in web's store — tests should cover admin-specific behavior (e.g. `signOutAndRedirect` builds `/login?redirect=...` via `sanitizeRedirect`, `isSigningOut` guard prevents concurrent sign-out) in addition to the login/logout/refresh-scheduling cases mirrored from web.
- Test style for a form component: `apps/web/src/features/auth/ui/login-form.test.tsx` — mocks `next/navigation` (`useRouter`, `useSearchParams`) and the auth store module, uses `@testing-library/react` `render`/`fireEvent`/`waitFor`/`screen`. Admin's `login-form.tsx` has near-identical shape (same redirect-safety logic, same `react-hook-form` usage) — mirror the three test cases (safe redirect, default fallback, unsafe redirect rejected) adjusting the fallback path to admin's default (`redirect: '/'` fallback per `isSafeInternalRedirect`/router usage in admin's `LoginForm` — verify exact fallback literal in the component, currently `'/'` via `redirect = isSafeInternalRedirect(...) ? rawRedirect : '/'`) and the button/label text (`'Войти'`, `'Email'`, `'Пароль'` — same Russian copy as web).

## Tests

Backend (`catalog` module):
- `service/service_test.go`:
  - `ListBooks`: passes filter through, returns items/total; propagates repo error wrapped.
  - `GetBookBySlug`: found → returns book; `gorm.ErrRecordNotFound` → `ErrBookNotFound`; other repo error → wrapped, not `ErrBookNotFound`.
  - `GetAuthorBySlug`: found / not-found → `ErrAuthorNotFound` / other-error-wrapped, same shape as book.
  - `ListGenres`, `ListAuthors` (with/without search, limit), `ListLanguages`: pass-through + error wrap.
  - `ParseUUID`: valid string parses; invalid string returns error.
- `transport/http/handler_test.go`:
  - `ListBooks`: default pagination (page=1, limit=20), limit clamped at 100, invalid `genre_id`/`author_id`/`rating_min` query values are ignored rather than erroring, response shape (`items`, `total`, `page`, `limit`).
  - `GetBook`: 200 with body on found; 404 with `{"error": "book not found"}` on not-found.
  - `GetAuthor`: 200 on found (including nested `books` in response); 404 on not-found.
  - `ListAuthors`: default limit applied when unset; search passed through.
  - `ListGenres`: response includes `items` + `total`.
  - `ListLanguages`: `ru`/`en` codes mapped to display names, other codes pass through as their own name.

Frontend (`apps/admin`):
- `shared/lib/redirect.test.ts` — table-driven, mirrors `safe-redirect.test.ts` cases adapted to the two-arg `(redirect, baseOrigin)` signature, plus `sanitizeRedirect` fallback behavior.
- `entities/session/model/auth-store.test.ts` — mirrors web's proactive-refresh test cases (login/register N/A — admin has no register; fetchUser scheduling; logout cancels timer; onTokenRefresh reschedules) plus admin-only `signOutAndRedirect`/`isSigningOut` behavior.
- `features/auth/ui/login-form.test.tsx` — mirrors web's three redirect-outcome cases with admin's copy/fallback.
- (Optional 4th, only if time-cheap) `shared/ui/pagination.test.tsx` — simple presentational component: renders nothing when `totalPages <= 1`, disables prev/next buttons at bounds, calls `onPageChange` with correct page.

## CI/DevOps

- `.github/workflows/ci.yml`, `frontend` job: add a test step. Since root `pnpm test` → `turbo run test` already picks up any workspace app defining a `test` script (confirmed via `turbo.json`'s `test` task, `dependsOn: ["^test"]`, and root `package.json`'s `"test": "turbo run test"`), the added step is simply `run: pnpm run test` alongside the existing lint/type-check/build steps. No env vars needed for `test` (vitest tests mock network/API boundaries; unlike `build`, they don't need `API_URL`/`NEXT_PUBLIC_API_URL`).
- Order: place `pnpm run test` after `pnpm run type-check` and before `pnpm run build` (fail fast on cheaper checks first) — placement is a minor implementation detail, not a hard requirement.
- No changes to the `backend` CI job (already runs `go test ./...`, which will automatically pick up the new catalog tests).
- No changes to `.github/workflows/deploy.yml` or any prod compose/Caddy files.

## Documentation

- No `docs/ROADMAP.md` change required by this contract (the roadmap item remains open pending real server provisioning) — but once this PR lands, whoever tracks the roadmap may want to reword the bullet to make clear only provisioning remains. Leave that decision to the orchestrator/reviewer, not this pass.
- No `docs/DEPLOYMENT.md` or `docs/PIPELINE.md` changes — those already correctly describe provisioning as the manual remaining step and this contract doesn't touch that.

## Risks

- Admin's `auth-store.ts` has more branching (`isSigningOut`, `forbiddenPath`, `signOutAndRedirect`) than web's equivalent; mirroring web's test file verbatim would under-cover it. Plan calls for extending coverage for admin-specific branches, which is slightly more effort than a pure mirror — acceptable, but flagged so the developer doesn't stop at parity-with-web and call it done.
- `catalog`'s handler currently silently ignores invalid `genre_id`/`author_id`/`rating_min`/`year_from`/`year_to` query params (parse errors are swallowed, filter field just stays unset) rather than returning 400. This is existing behavior, not a bug to fix in this pass — tests should assert the current (lenient) behavior, not introduce stricter validation.
- Adding a test step to the CI `frontend` job increases CI wall-clock time modestly; both `apps/web` (5 files) and `apps/admin` (3-4 new files) are small suites so impact should be negligible, but worth a one-line callout in the PR description.
- If `apps/admin`'s `tsconfig`/path-alias setup differs subtly from `apps/web`'s (e.g. `@/*` mapping), the vitest `resolve.alias` in `apps/admin/vitest.config.ts` must match `apps/admin/tsconfig.json`'s actual alias, not be copy-pasted blindly from web assuming identical structure — worth a quick diff check by the implementing developer.
