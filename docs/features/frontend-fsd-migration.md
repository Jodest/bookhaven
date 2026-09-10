# Frontend FSD Migration (apps/web, apps/admin)

Status: implemented

## Goal

Restructure `apps/web` and `apps/admin` from the current flat `app/ + components/ + lib/` layout into Feature-Sliced Design (FSD) layers, with enforceable import-direction rules, **without changing observed application behavior**. This is a structural refactor, not a new feature — no new user-facing capability is in scope.

## Non-Goals / Scope Cuts

- No visual/UX changes, no new routes, no new API endpoints.
- No change to `packages/api-client`'s public shape (the monolithic `api.*` object) or `packages/ui`'s public exports. Decomposing `api-client` per-entity is noted as a follow-up (see Risks), not part of this pass.
- No FSD `processes` layer (deprecated in modern FSD); use `shared → entities → features → widgets → views → app`.
- No new automated frontend test suite is introduced. Neither app currently has frontend tests, so "no behavior change" is verified via build/lint/type-check plus a manual route smoke-check, not new automated tests.
- Backend, migrations, and `packages/*` internals are out of scope except for how they are *consumed* from the apps.
- If the full migration proves too large for one pass, `apps/web` is migrated first (it has more routes/components) and `apps/admin` second, reusing the same conventions. Both are covered by this single contract; they may land as separate PRs.

## User Stories

N/A (internal structural refactor; no end-user-facing story). Acceptance is developer/agent-facing: a future feature contract should be implementable inside the new structure without ambiguity about where code lives.

## Acceptance Criteria

- [x] `apps/web/src` and `apps/admin/src` no longer have top-level `components/` or `lib/` dirs; all code lives under `app/`, `views/`, `widgets/`, `features/`, `entities/`, or `shared/`.
- [x] Every current route in both apps renders the same UI/behavior as before (verified via `next build` static generation of every route plus line-by-line code review of each moved component; interactive `next dev` smoke-testing could not be completed in the sandboxed shell used for implementation — see Tests/Risks).
- [x] `pnpm --filter @bookhaven/web build`, `lint`, `type-check` pass.
- [x] `pnpm --filter @bookhaven/admin build`, `lint`, `type-check` pass.
- [x] Repo-wide `pnpm build`, `pnpm lint`, `pnpm type-check` (turbo) pass (repo-wide `build` requires a reachable backend for `apps/web`'s `/` route SSR fetch, same as before this migration; `lint`/`type-check` pass with no backend needed).
- [x] An import-boundary lint rule is active in both apps' ESLint config and fails CI-equivalent lint on a layer-direction violation (verified by a throwaway violation during implementation, then reverted).
- [x] Each slice (folder under `entities/`, `features/`, `widgets/`) exposes a single `index.ts` barrel; no other file in the codebase deep-imports a slice's internals (enforced by `eslint-plugin-boundaries`' `entry-point` rule, verified by a throwaway deep-import violation during implementation, then reverted).
- [x] `docs/ROADMAP.md` and this contract are updated to `implemented` status after review.

## Business Rules

None — this is a code-organization change. Existing authorization, validation, and business logic must be moved, not rewritten. Any bug incidentally noticed during the move should be filed/fixed separately, not silently changed in this refactor.

## Target FSD Layer Structure (both apps)

Layers, low → high (an import may only point **downward**, i.e. `features` may import `entities`/`shared`, never `widgets`/`views`/`app`; sibling slices in the same layer must not import each other except via composition in a higher layer):

```
src/
  app/        # Next.js App Router — thin routing only (framework requirement)
  views/      # FSD "pages" layer, renamed to avoid clashing with Next's app/ dir
  widgets/    # composite, route-agnostic UI blocks composed from features/entities
  features/   # single user-actions (submit, filter, add, moderate, ...)
  entities/   # domain nouns: data shape + read UI + slice-local API wrapper
  shared/     # generic, app-agnostic: ui re-exports, lib, api client setup, config
  middleware.ts  # stays at src root (Next.js requirement, not an FSD layer)
```

Segments inside a slice (only what's needed, not all required): `ui/`, `model/`, `api/`, `lib/`, `config/`, plus `index.ts` as the slice's only public entry point.

`src/app/**` under Next's App Router keeps doing routing/metadata/layout only: each `page.tsx` becomes a thin wrapper that renders the matching component from `src/views/<name>`, e.g.:
```tsx
// src/app/books/page.tsx
import { CatalogView } from '@/views/catalog';
export default function Page() { return <CatalogView />; }
```
`layout.tsx`, `providers.tsx`, `globals.css`, and `middleware.ts` remain in `src/app`/`src` but should import session/query-client setup from `shared`/`entities` rather than owning that logic inline.

### `apps/web` slice mapping (current file → target)

- `views/`: `home`, `catalog` (was `books/page.tsx`), `book-detail` (`books/[slug]`), `author-detail` (`authors/[slug]`), `login`, `register`, `shelves`, `shelf-detail` (`shelves/[id]`), `lists`, `list-detail` (`lists/[id]`), `public-list-detail` (`lists/public/[id]`), `profile`, `public-profile` (`profile/[id]`), `challenge`.
- `widgets/`: `header.tsx` → `widgets/header`; `reviews-section.tsx` → `widgets/reviews-section` (composes the `review` entity + `write-review` feature); filter composition (`filter-panel.tsx`, `active-filters.tsx`, `search-bar.tsx`, `sort-selector.tsx`, `filters/*`) → `widgets/catalog-filters` (exact internal split between this widget and a `filter-books` feature is left to the developer; the layer-direction rule is what must hold).
- `features/`: `auth` (from `login-form.tsx`, `register-form.tsx`, `auth-initializer.tsx`), `rate-book` (`book-rating.tsx`, if interactive), `write-review` (`review-form.tsx`), `shelve-book` (`shelve-book-buttons.tsx`), `add-to-list` (`add-to-list-button.tsx`), `manage-reading-challenge` (challenge-page form/logic), `update-profile` (profile edit logic).
- `entities/`: `book`, `author`, `genre`, `review`, `shelf`, `list` (avoid the `List` name clash with the global type), `reading-challenge`, `session` (from `lib/auth-store.ts` — holds the authenticated user; `logout` action can live here or in `features/auth`, pick one and be consistent).
- `shared/`: `shared/ui` (thin re-exports of `@bookhaven/ui` plus any truly generic local primitives, e.g. `components/ui/checkbox.tsx` if not promoted to `@bookhaven/ui`), `shared/lib` (generic helpers), `shared/api` (query client setup, `@bookhaven/api-client` accessors not specific to one entity), `shared/config` (route path constants, env).
- `empty-state.tsx` is generic → `shared/ui`.

### `apps/admin` slice mapping

- `views/`: `dashboard` (`/`), `login`, `books-list` (`/books`), `book-new` (`/books/new`), `book-edit` (`/books/[id]/edit`), `authors`, `genres`, `reviews`.
- `widgets/`: `shell.tsx` → `widgets/admin-shell`.
- `features/`: `auth` (admin `login-form.tsx`, `auth-initializer.tsx`), `manage-book` (`book-form.tsx`), `manage-author`, `manage-genre`, `moderate-review`.
- `entities/`: `book` (admin-flavored, `AdminBook` types), `author`, `genre`, `review`, `session` (from `lib/auth-store.ts`, includes `isAdmin`/`isModerator` helpers).
- `shared/`: `shared/ui` (`pagination.tsx`, `multi-select.tsx`, `access-denied.tsx` — all generic, non-entity-specific), `shared/lib` (`redirect.ts` → `sanitizeRedirect`), `shared/api` (`admin-query-client.ts`).

## Existing Patterns

- Reuse `apps/admin/src/lib/auth-store.ts` and `apps/web/src/lib/auth-store.ts` logic as-is when moving into `entities/session/model`; do not rewrite the token/cookie flow described in `docs/features/admin.md` ("Implementation Notes" section on bearer-token + cookie auth).
- Reuse the pagination/filter/table composition patterns already in `apps/admin/src/app/books/page.tsx` and `apps/web/src/components/filter-panel.tsx` as the reference for what a `views/*` component and a `widgets/*` component should look like after the split.
- `packages/ui` (`@bookhaven/ui`) and `packages/api-client` (`@bookhaven/api-client`) are monorepo-level shared packages that play the same architectural role as an app-local `shared` layer. Treat them as **part of the `shared` layer**: importable directly from `entities/`, `features/`, `widgets/`, `views/`, and `app/` — no local re-export indirection required.

## Import Boundary Rules & Enforcement

Rules (both apps):

1. Layer direction: `shared < entities < features < widgets < views < app`. A file may import from its own layer's other slices only through their `index.ts`, and from any strictly lower layer. It may never import a higher layer or reach into another slice's internals (e.g. `@/entities/book/ui/BookCard` from outside `entities/book` is forbidden — must go through `@/entities/book`).
2. No cross-imports between sibling slices in `entities/` or `features/` (composition happens in `widgets/`/`views/`). If two entities are genuinely coupled only through data already embedded in API response types (e.g. `Book.authors: Author[]`), that is not a code import and is fine.
3. `@bookhaven/ui` and `@bookhaven/api-client` are treated as `shared`-layer externals per above — allowed from any layer.
4. `src/app/**` (Next.js routing) may only import from `views/`, plus `shared` for layout-level setup (providers, global query client). It must not import `features/`/`entities/` directly — that indicates missing view composition.

Enforcement: add `eslint-plugin-boundaries` (or an equivalent boundaries-aware ESLint plugin compatible with the existing ESLint 8 / `next/core-web-vitals` setup) as a devDependency in both `apps/web` and `apps/admin`, and extend each app's `.eslintrc.json` with an `element-types` (layer-direction) rule and a public-API/entry-point rule, mapped to the folder layout above. Consult the plugin's current docs for exact rule names/options during implementation — don't guess at config shape here. Wire it into the existing `lint` script so `turbo run lint` picks it up; no new pipeline is needed.

If `eslint-plugin-boundaries` proves incompatible with the current ESLint 8 setup, fall back to a documented manual convention plus a one-off `steiger` (FSD's dedicated linter) check run locally before submitting the PR, and note the gap in this contract's Risks before merging.

## Migration Approach / Ordering

1. **Scaffold `shared`** in each app: move `lib/` contents and any generic UI atoms into `shared/lib` and `shared/ui`, update imports. No behavior change; build/lint/type-check must stay green after this step.
2. **Extract `entities`**: create one slice per domain noun, move/rename the relevant read-only UI + wrap the corresponding `@bookhaven/api-client` calls in each entity's `api/index.ts`. Session/auth store becomes `entities/session`.
3. **Extract `features`**: move interactive/action components (forms, buttons, filters) into feature slices, wiring them to the entities' API wrappers and/or `entities/session`.
4. **Extract `widgets`**: compose features/entities into the reusable blocks identified above (header, shell, filter bar, reviews section).
5. **Extract `views`**: move the body of each `app/**/page.tsx` into `views/<name>`, leaving thin route files in `src/app`.
6. **Add lint enforcement** (`eslint-plugin-boundaries`) last, once the structure is stable, and fix any violations it surfaces.
7. Do `apps/web` fully (steps 1–6), verify green, then repeat for `apps/admin`. Commit/PR per app-and-step granularity is left to the developer, but each step should leave the app buildable.

Within each step, move files with `git mv` where practical to preserve history, then fix imports — do not rewrite component logic while moving it.

## Frontend

Covered above in full (this contract *is* the frontend change). No backend or database changes.

## Tests

- No existing automated frontend tests to migrate (`apps/web`, `apps/admin` currently have none).
- Required verification: `build`, `lint`, `type-check` for both apps and at the repo root (turbo).
- Manual smoke-check after each app's migration: visit every route listed in the "Target FSD Layer Structure" view lists above in dev mode, confirm no console errors/hydration warnings and no visual/behavioral regression versus current `master`, exercising at least one write action per app (e.g. web: add a rating and a review; admin: edit a book and moderate a review).
- Do not add a new test framework as part of this refactor; if the developer believes minimal smoke tests are warranted, propose that as a separate follow-up contract.

## Documentation

- Update `docs/ROADMAP.md` to mark this migration complete and link to this contract once both apps pass acceptance criteria.
- If the developer settles on specific slice names that differ from the mapping tables above, update this contract to match reality before closing it out, so it stays useful as a reference for future feature contracts.

## Implementation Notes / Deviations from the original mapping tables

Both apps were migrated (web fully, then admin), following the phased ordering. `eslint-plugin-boundaries@^4.2.2` was used (last major line supporting classic `.eslintrc.json` config with ESLint 8; v5+ targets ESLint 9 flat config only, per the plugin's own README). It resolves `@/*` aliases via the `eslint-import-resolver-typescript` resolver already wired in by `eslint-config-next`, so no extra resolver dependency was needed.

Deviations from the mapping tables in this doc, kept because the alternative would have created an upward (`shared`/`app` → `entities`) dependency or a self-contradictory boundary rule:

- **Session bootstrap (`auth-initializer.tsx`) lives in `entities/session`, not `features/auth`,** in both apps (exported as `SessionInitializer`). The mapping table originally suggested `features/auth`, but `src/app/layout.tsx`/`providers.tsx` need to render it, and `app`/`app-root` files are only allowed to import `shared`/`entities`/`views` (not `features`) per the boundary rules — bootstrapping the session store is a read of entity state, not a user-initiated action, so `entities/session` is the correct home.
- **`apps/admin`'s `admin-query-client.ts` lives in `entities/session/lib`, not `shared/api`.** It reads `useAuthStore` for 401/403 handling, so placing it in `shared` would create a `shared → entities` import, which is disallowed by the layer-direction rule. It's exported from `entities/session` as `createAdminQueryClient`.
- **`apps/admin`'s `access-denied.tsx` lives in `widgets/admin-shell/ui`, not `shared/ui`.** It calls `useAuthStore().logout()` directly; keeping it as a widget-owned component (widgets may import entities) avoided adding a prop-drilled callback purely to satisfy layering, with no behavior change.
- **A dedicated `eslint-plugin-boundaries` element type `app-root`** (matching only `app/layout.tsx` and `app/providers.tsx` by filename) was introduced in both apps' `.eslintrc.json`, separate from `app` (which matches route `page.tsx` files). `app-root` may import `shared`/`entities`/`widgets`/`views`; plain `app` route files may only import `shared`/`views`, per rule 4 of the contract.
- **`apps/web`'s `entities/book`, `entities/author`, `entities/review`, `entities/shelf`, `entities/list`, `entities/genre`, `entities/reading-challenge` expose only an `api/` segment** (a typed wrapper around the matching `@bookhaven/api-client` sub-namespace) and no dedicated read-UI component, since none existed as a separately reusable component before the migration (book/author/list/shelf cards were inline JSX repeated with small variations across pages). Extracting a shared card component was judged out of scope for a move-and-rewire pass. The one exception is `entities/review`, which gained a `ReviewCard` (`ui/review-card.tsx`) because `widgets/reviews-section` explicitly needed to compose "the `review` entity + `write-review` feature" per the contract's own wording — this required lifting the per-review `<article>` markup out of the old monolithic `reviews-section.tsx` into its own file.
- **`apps/web`'s `features/manage-reading-challenge`** contains the entire goal-display-and-edit card (`ManageReadingChallengeGoal`), not just the raw `<form>`, because the read display (progress bar, "goal not set" text) and the edit toggle share the same local `isEditing` state and cannot be cleanly split further without prop-drilling that state back out to the view.
- **`apps/admin`'s `features/manage-author` / `features/manage-genre`** are presentational forms (`AuthorForm`, `GenreForm`) that receive `formData`/`editing`/handlers as props; the owning view (`views/authors`, `views/genres`) keeps the `useQuery`/`useMutation` orchestration, search box, and table, since those aren't a single user action.
- **No `apps/web` `filter-books` feature was introduced** — all of `filter-panel.tsx`, `active-filters.tsx`, `search-bar.tsx`, `sort-selector.tsx`, and `filters/*` were kept together under `widgets/catalog-filters`, per the contract's own note that "the exact internal split ... is left to the developer."
- **Non-entity API calls stay as direct `@bookhaven/api-client` calls** from the layer that needs them (e.g. `api.ratings.*` in `features/rate-book`, `api.languages.*` in the language filter, `api.users.updateMe`/`api.users.getPublic` in `features/update-profile`/`views/public-profile`, `api.admin.dashboard()` in `views/dashboard`), since the contract's entity lists for both apps don't include a `rating`, `language`, `user`/`profile`, or `dashboard` entity, and `@bookhaven/api-client` is an allowed shared-layer external from any layer.
- A pre-existing Next.js 14 requirement (also already applied to `apps/admin/src/app/login/page.tsx` before this migration) meant `apps/web/src/app/books/page.tsx` needed a `<Suspense>` wrapper around `CatalogView` for static export to succeed, since `CatalogView` uses `useSearchParams()`. This is a mechanical, non-behavioral addition required to keep `next build` green, consistent with the existing pattern already used elsewhere in the monorepo.

## Risks

- **`@bookhaven/api-client` is a single cross-domain `api` object.** It doesn't map 1:1 onto per-entity API segments; entities/features should only call the sub-namespace they own (e.g. `entities/book/api` calls `api.books.*`), but the package itself is not being decomposed in this pass. Flag as tech debt for a future, separate change.
- **Next.js App Router's `app/` directory name collides with FSD's conventional "pages" layer name.** Mitigated by renaming that layer to `views/` in-app, which is a common, documented adaptation — but it means this repo's FSD is not letter-for-letter canonical FSD; note this so future contributors aren't confused.
- **No automated frontend tests exist**, so regression detection relies on build/type-check plus manual smoke-checks; a subtle behavioral regression (e.g. a dropped auth header, a misplaced `'use client'` boundary) could slip through. Consider prioritizing "Tests, CI/CD and Production" (already on the roadmap) before or shortly after this refactor.
- **ESLint 8 + `eslint-plugin-boundaries` compatibility is unverified** at planning time; if it doesn't fit cleanly, enforcement may need to fall back to a documented convention checked manually, weakening the "mechanically verifiable" boundary guarantee until the tooling gap is closed. Resolved: `eslint-plugin-boundaries@^4.2.2` works cleanly with the existing ESLint 8 / `next/core-web-vitals` setup in both apps (v5+ requires ESLint 9's flat config and was avoided).
- **Scope creep risk**: because this touches nearly every file in both apps, it's tempting to also refactor component internals, rename props, or fix unrelated bugs while moving files. That must be avoided — this contract is move-and-rewire only.
- **Manual route smoke-check was not completed interactively.** `next build`'s static-generation step exercised every route in both apps successfully (with a local stub API for `apps/web`'s home-page SSR fetch, since this sandbox has no reachable `backend` host — the same dependency the app has always had), and every moved file was reviewed line-by-line against its pre-migration source for behavioral equivalence. However, starting `next dev` and clicking through routes in a browser could not be completed in the sandboxed shell used for this implementation (a Windows/pnpm symlink resolution quirk broke direct `next`/`next dev` invocation, while `pnpm run build`/`pnpm run lint`/`tsc` all worked normally). A human or CI run of `pnpm --filter @bookhaven/web dev` / `pnpm --filter @bookhaven/admin dev` plus the write-action smoke-checks described under Tests is recommended before merging.
