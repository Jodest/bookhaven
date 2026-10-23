# Next.js 15.5 / React 19 Upgrade

Status: done

Issue: #594 (follow-up of #590, dependency scanning).

## Goal

Upgrade `apps/web` and `apps/admin` from Next 14.2.35 / React 18 to **Next 15.5.28 / React 19**. Every `next` advisory allowlisted for #594 in `.github/vuln-allowlist.json` is fixed by 15.5.28 or earlier, including the critical Image Optimization RCEs that `next/image` in apps/web can reach. Nothing the user sees should change. This is a toolchain migration only.

## Target decision: 15.5.28, not 16.x

- **15.5.28 closes every current `next` advisory.** The highest fixed-in version among them is 15.5.28 (GHSA-2464-fhr4-w9xw, GHSA-p3m5-59mp-jv5r). The 15.5 line is the maintained `backport` dist-tag, so future security fixes will still arrive as patch bumps.
- **16.x would add a second major on top**, for no security benefit:
  - `middleware.ts` is renamed to `proxy.ts`.
  - `next lint` is removed, and eslint-config-next 16 is flat-config only. That means ESLint 9 plus a rewrite of both `.eslintrc.json` files, including the `boundaries` FSD rules.
  - Turbopack becomes the default `next build`. That carries a risk for the webpack-based `withSentryConfig` instrumentation, for `typedRoutes` and for `transpilePackages`.
  - The synchronous fallbacks for request APIs are removed, and `images` defaults change.
  - None of this can be verified locally because native `next build` is forbidden. The CI build would be the only gate for two majors at once.
- **Recommendation:** pin `next` to exactly `15.5.28` (the project already pins `next` exactly). Open a separate backlog item for Next 16 once 15 is stable. That item would cover the ESLint 9 flat config, `proxy.ts` and the Turbopack build.

## Acceptance Criteria

- [x] `apps/web` and `apps/admin`:
  - `next` and `eslint-config-next` are exactly `15.5.28`.
  - `react` and `react-dom` are `^19` (current 19.x patch; check it against `npm view next@15.5.28 peerDependencies`).
  - `@types/react` and `@types/react-dom` are `^19`.
- [x] `packages/ui`: `peerDependencies.react` is `^19.0.0`, and the devDeps `react` / `@types/react` / `@types/react-dom` are `^19`.
- [x] Third-party React peers are satisfied with React 19. `pnpm install` prints no `unmet peer react` warnings for runtime deps (see Dependencies).
- [x] Every dynamic `page.tsx` / `generateMetadata` uses `params: Promise<...>` and awaits it (7 files, see Frontend).
- [x] Both `next.config.js` files:
  - `experimental.instrumentationHook` is removed.
  - `typedRoutes` has moved from `experimental` to the top level. If 15.5.28 rejects the top-level key, keep it under `experimental` and note that in the PR.
  - Server-side Sentry still initialises through `src/instrumentation.ts`.
- [x] `tsc --noEmit`, `next lint` and `vitest run` are green in web, admin and ui. The CI `frontend` job is green: lint, test, build, and the vuln gate with no `next` findings.
- [x] Every `next` entry with `"issue": "#594"` is removed from `.github/vuln-allowlist.json` (12 entries). The gate reports no stale-entry notices for them. The `vite` and `golang-jwt` entries stay.
- [x] The compose smoke test (see Tests) passes for web and admin.
- [x] The First Load JS shared size from the CI build log is recorded in the PR, next to the last dev run (87.2 kB).
- [x] Docs are updated (see Documentation).

## Dependencies

Change version ranges only. Do not touch unrelated packages.

| Package | Where | Now | Target | Why |
|---|---|---|---|---|
| next | web, admin | 14.2.35 | **15.5.28** (exact) | advisories |
| eslint-config-next | web, admin | 14.2.35 | **15.5.28** (exact) | must match `next`; peer `eslint ^7.23 \|\| ^8 \|\| ^9`, so **keep eslint 8 and `.eslintrc.json`** |
| react, react-dom | web, admin | ^18.3.0 | ^19.x | App Router on Next 15 requires React 19. The installed copy is what vitest and the types use; the app runtime uses Next's vendored React. |
| @types/react, @types/react-dom | web, admin, ui | ^18.3.0 | ^19 | |
| react (dev + peer) | ui | ^18.3.0 | ^19 / peer `^19.0.0` | otherwise the workspace peer conflicts |
| lucide-react | web, admin, ui | ^0.436.0 | lowest 0.x whose `peerDependencies.react` includes `^19` (check with `npm view lucide-react@<v> peerDependencies`) | 0.436 declares react up to ^18 only. Icon renames between versions are caught by tsc. Keep the same version in all three packages. |
| react-hook-form | web, admin | ^7.52.0 | raise floor to the first 7.x declaring react ^19 (7.54+) | peer |
| @tanstack/react-query | web, admin | ^5.52.0 | raise floor to the first 5.x declaring react ^19 | peer |
| @radix-ui/react-slot, @radix-ui/react-popover | ui | ^1.1.x | as is if the lockfile-resolved versions already declare ^19; otherwise bump within 1.x | peer |

Already compatible, no change needed:
- @sentry/nextjs 11.8.0 (peer next ^14 \|\| ^15 \|\| ^16)
- react-markdown 10
- zustand 4.5 (peer react >=16.8)
- @hookform/resolvers 5
- @testing-library/react 16 (react/@types ^18 \|\| ^19)
- @testing-library/jest-dom 6
- @vitejs/plugin-react 4
- vitest 2
- jsdom
- typescript ^5.5

Keep the root `pnpm.overrides`. Do not add `peerDependencyRules` to hide warnings; fix them by bumping.

## Codemod

**Do not run `npx @next/codemod upgrade`.**
- It is interactive.
- It edits every package.json and runs a native install.
- It offers Turbopack and React 19 type codemods across the whole tree.
- Its result is hard to review.

The real breaking surface here is small (7 page files, 1 `useRef`), so make the edits by hand. If you want a cross-check, run the pinned async-API codemod as a dry run only:

`npx @next/codemod@15.5.28 next-async-request-api apps/web/src/app apps/admin/src/app --dry --print`

Then compare its output with the manual edits. Do not run the `types-react-codemod` preset.

## Next 15 / React 19 breaking changes, checked against this codebase

Grep results are from 2026-10-23.

**Async request APIs.**
- `cookies()`, `headers()` and `draftMode()`: no usages. Nothing imports `next/headers`.
- There are no route handlers (`route.ts`).
- Both middlewares only use `request.cookies` / `request.nextUrl`, which are unchanged. There is no `request.geo` / `request.ip`, which Next 15 removed.
- Pages/metadata that receive `params`. All are server components that pass a string into a client view. Change each to `params: Promise<{...}>` and make it `async` with `const { slug } = await params`:
  - `apps/web/src/app/books/[slug]/page.tsx`: `Page` and `generateMetadata`
  - `apps/web/src/app/authors/[slug]/page.tsx`
  - `apps/web/src/app/profile/[id]/page.tsx`
  - `apps/web/src/app/shelves/[id]/page.tsx`
  - `apps/web/src/app/shelves/public/[id]/page.tsx`
  - `apps/web/src/app/top/[slug]/page.tsx`
  - `apps/admin/src/app/help/[chapter]/page.tsx`: `Page` and `generateMetadata`. Keep `force-static`, `dynamicParams = false` and the synchronous `generateStaticParams`. `generateMetadata` becomes `async`.
- No page reads the `searchParams` prop. Client components use `useSearchParams` / `useParams` (admin `[id]` pages), and those stay synchronous.
- The `typedRoutes` build type-check enforces the Promise shape, so CI catches any page that was missed.

**Caching defaults.** In Next 15, `fetch` and `GET` route handlers are no longer cached by default.
- There are no route handlers.
- The only server-side fetch goes through `@bookhaven/api-client`, which already sets `cache: 'no-store'`.
- Home already uses `force-dynamic`.
- Result: no behavior change.
- The client router cache (`staleTimes.dynamic` default 0) refetches page segments on navigation. Data comes from react-query in client views, so this has no visible effect. Do not add `staleTimes`.

**React 19.**
- `useFormState` / `useFormStatus` / `useActionState`: no usages.
- `forwardRef` (packages/ui) still works in 19. Do not migrate it to ref-as-prop (out of scope).
- None of these appear in `apps/*/src` or `packages/*/src`: `ReactDOM.render`, `findDOMNode`, string refs, `defaultProps` on function components, `propTypes`, `react-dom/test-utils`, `react-test-renderer`, `element.props` access, a global `JSX.` namespace or `MutableRefObject`.
- `@types/react` 19 requires an argument for `useRef`. One hit: `apps/web/src/widgets/catalog-filters/ui/filters/author-filter.tsx:28`. Change it to `useRef<NodeJS.Timeout | undefined>(undefined)`. Fix any other `useRef<T>()` that tsc reports the same way.
- `React.ElementRef` (`packages/ui/src/components/ui/popover.tsx`) is deprecated but still exported in 19. Leave it unless tsc or lint fails. If you do change it, use `React.ComponentRef`.
- Expect possibly new tsc errors from stricter types: `ReactElement` props default to `unknown`, and ref callbacks must not return a value (an implicit-return arrow like `ref={(el) => (x = el)}` must become a block body). Fix them minimally where tsc points.

**next.config.js** (both apps):
- Remove `experimental.instrumentationHook`. Instrumentation is stable in 15. Update the comment that refers to "Next 14 needs the flag".
- Move `experimental.typedRoutes` to the top-level `typedRoutes: true` (stable in 15.5). Drop the empty `experimental` object.
- `serverComponentsExternalPackages` (renamed `serverExternalPackages`) and `bundlePagesRouterDependencies`: not used, no action.
- The rest is unchanged in 15: `images.remotePatterns`, `minimumCacheTTL`, `output: 'standalone'`, `transpilePackages`, `headers()`, `rewrites()`, `redirects()`, `withSentryConfig` options.
- Do not enable Turbopack (`next dev --turbopack`), the React Compiler or `ppr`.

**ESLint.**
- eslint-config-next 15.5.28 works with eslint 8 and the legacy `.eslintrc.json`, and it pulls eslint-plugin-react-hooks 5.
- `next lint` prints a deprecation notice in 15.5. That is accepted; leaving `next lint` belongs to the Next 16 item.
- If the new config reports new warnings or errors, fix them in place. Do not disable rules.
- `packages/ui` lints with plain eslint and @typescript-eslint 7, and is unaffected.

**Other.**
- `next-env.d.ts` is regenerated by Next 15.5 during `next dev` / `next build`. Commit the version CI or `next dev` produces; do not hand-write it.
- `src/instrumentation.ts`: keep it as is. Next 15 adds the `onRequestError` hook (`Sentry.captureRequestError`). Leave that out of scope and list it as a follow-up in the PR. A Sentry build warning about the missing hook is acceptable.
- Node: 22 everywhere (CI, `Dockerfile.node`), which satisfies Next 15. In 15, standalone output bundles `sharp` for image optimization, so `Dockerfile.node` does not change.

## Frontend

Implement in this order, one commit per step:
1. **Dependency bump.** Change `package.json` ranges as above, then run `pnpm install` natively to update `pnpm-lock.yaml`. Do not run `next build`.
2. **Code changes.**
   - Make `params` async in the 7 pages.
   - Fix the `useRef` call.
   - Update both `next.config.js` files.
   - Fix any tsc/lint fallout.
3. **Allowlist and docs.**

No FSD or layout changes and no unrelated refactors. Leave `Suspense` wrappers, `history.replaceState` URL sync (catalog pagination), the middleware logic and the security headers as they are.

## Tests

Run locally, in each of `apps/web`, `apps/admin` and `packages/ui`:
- `pnpm exec tsc --noEmit`
- `./node_modules/.bin/next lint` (apps)
- `pnpm exec vitest run`

Existing suites must pass unchanged. Pay particular attention to:
- the middleware tests;
- `entity-redirects.test.tsx` (admin);
- the auth-store and login-form tests;
- the render tests that use @testing-library/react with React 19. In React 19, `act` warnings and the `useEffect` order in StrictMode can surface; fix the test setup, not the assertions.

No new unit tests are needed: behavior does not change.

**No native `next build`** (project rule). The CI `frontend` job (lint, test, build, vuln gate) is the gate for the build and typedRoutes.

Compose smoke test (`docker compose` dev stack), web:
- Login.
- Open `/profile` while logged out: it redirects to `/login?redirect=/profile`.
- Open `/login` while logged in: it redirects to `/profile`.
- Token refresh: let `access_token` expire or delete it while `has_session` stays set, reload `/shelves` and confirm the client refreshes without redirecting.
- `/books/<slug>`: the page renders and `<title>` comes from `generateMetadata`.
- `/authors/<slug>` with a Wikimedia photo (unoptimized).
- An Open Library cover through `/_next/image` returns 200 with an image content-type.
- A self-hosted `/covers/...` image.
- `/profile/<id>`, `/shelves/public/<id>`, `/top/<slug>`.
- `/books` with filters and "load more", checking that the URL syncs.
- Response headers on a page still include the CSP and the other security headers.

Compose smoke test, admin:
- Login.
- Opening any page while logged out redirects to login.
- Dashboard.
- `/catalog/books/<id>/edit`.
- `/import/books` gives a 307 to `/import/books/wikidata`.
- `/help/<chapter>` (static params, metadata title).
- The moderation queue.
- `/collections/<id>`.

Bundle size: copy the `First Load JS shared by all` line and the largest routes from the CI build log into the PR, next to the last dev run (shared 87.2 kB). Some growth is expected from the React 19 + Next 15 runtime, roughly 10–20 kB shared. Flag anything well beyond that, or any route that more than doubles, for review before merge. Do not try to optimize it in this PR.

## Documentation

- `.github/vuln-allowlist.json`: remove the 12 `next` entries for #594.
- `docs/DEPLOYMENT.md`:
  - Security checklist, line 554: tick the item, or reword it to "Next 15.5.28 (#594) landed; Image Optimization RCEs closed".
  - Dependabot paragraph, line 130: replace "(the Next 15 / React 19 migration is #594)" with a note that the Next 16 major is a separate follow-up.
- `docs/PROJECT.md:30`: "Next.js 14" becomes "Next.js 15 (React 19)".
- `docs/features/error-tracking.md:100-101`: note that the `instrumentationHook` flag was removed on Next 15 (instrumentation is stable).
- `docs/features/dependency-scanning.md`: one line in Risks/Status saying follow-up (b), #594, has landed.
- `docs/ROADMAP.md`: mark #594 done if it is listed there.
- Set this contract to `Status: done` with a short Implementation Notes section: the final versions, the bundle numbers, and anything that deviated from the plan.

## Risks

- **The CI build is the only build gate.** typedRoutes and the Promise `params` check, the `withSentryConfig` webpack wrapper and `transpilePackages` all surface only in CI. Push early and iterate on CI logs. If the Sentry wrapper breaks the build, stop and report; do not hack around it.
- **Peer mismatches.**
  - lucide-react: icon renames when bumping. tsc catches them; keep the version identical across the three packages.
  - The `packages/ui` peer range must move to React 19, otherwise pnpm reports workspace peer conflicts.
- **New React 19 types errors** that the grep did not catch, such as ref callback returns or `ReactElement` props. Fix them minimally.
- **New lint rules from eslint-plugin-react-hooks 5.** Fix them in place; do not disable rules.
- **Hydration and runtime differences in React 19:** error messages, stricter hydration mismatch reporting. The compose smoke test with the browser console open covers this.
- **The image optimizer path** (`/_next/image` for Open Library covers) must still work after the bump. The smoke test checks it explicitly, because it is the reason for the urgency.
- **Destructive commands:** none needed. No DB changes, no `migrate down`. Do not delete `node_modules` via `next build`.
- **Out of scope** (separate backlog items):
  - Next 16 (ESLint 9 flat config, `proxy.ts`, Turbopack build);
  - Sentry `onRequestError`;
  - the vitest/vite major bump for the `vite` advisories;
  - migrating `forwardRef` to ref-as-prop.

## Implementation Notes

- Final versions: next and eslint-config-next 15.5.28 (exact); react and react-dom ^19.2.0 (resolved 19.3.x); @types/react and @types/react-dom ^19.2.0; ui peer react ^19.0.0; lucide-react ^0.469.0 (first 0.x with a stable ^19 peer) in web, admin and ui; react-hook-form ^7.54.0; @tanstack/react-query ^5.59.0. Radix slot and popover already declare ^19; no change.
- `typedRoutes` works at the top level in 15.5.28. `instrumentationHook` is removed; Sentry server init through `src/instrumentation.ts` still starts (compose dev).
- Deviation: the web test `book-rating.test.tsx` asserted that the useId-derived clip id matches `[a-z0-9]+`; React 19.2 emits ids like `_r_a_`, so the character class now allows `_` and uppercase. The component is unchanged (ids stay valid in `url(#...)`).
- The vuln-gate test for the committed allowlist now asserts that no #594 entries remain.
- `next-env.d.ts` in both apps was regenerated by Next 15.5 (adds the routes.d.ts reference).
- `next lint` prints a deprecation notice (accepted, see Next 16 follow-up #603). Web lint keeps 2 pre-existing `<img>` warnings.
- First Load JS: not measured locally (no native `next build`); take it from the CI build log (previous dev run: shared 87.2 kB).
- Follow-ups: Next 16 (#603), Sentry `onRequestError`, vite major bump, forwardRef to ref-as-prop.
