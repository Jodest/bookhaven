# Self-Hosted Fonts (Fix Flaky CI Build Due to Live Google Fonts Fetch)

Status: implemented

## Goal

Eliminate a CI build failure/flakiness caused by `next/font/google` performing a live network fetch to `fonts.gstatic.com` during `next build` in `apps/web` and `apps/admin`. Confirmed root cause (CI run https://github.com/Jodest/bookhaven/actions/runs/36442875686/job/108997592456): the fetch fails on the GitHub Actions runner, and `next/font/google`'s loader crashes ungracefully (`TypeError: Cannot read properties of null (reading '1')`) instead of retrying/failing cleanly, aborting the `apps/web` build. `apps/admin` uses the same live-fetch mechanism (`Inter`) and is equally exposed even though it happened to succeed in that run.

This is a build-reliability fix only — same fonts, same weights, same subsets, no visual/typography change, no CI workflow change.

## Non-Goals

- Any change to `.github/workflows/ci.yml` (no retries/workarounds at the workflow level — root cause is fixed in application code).
- Any visual/design change — same font families (Inter, Literata), same weights (`400/500/600/700` for Literata; default/variable for Inter), same language coverage (Latin + Cyrillic).
- Any change to other build steps, lint/type-check config, or the `backend` CI job.
- Introducing a new CI test step to assert "no network fetch happened" — out of scope per instructions (no CI changes); verification of the fix is a manual/local acceptance check instead (see Acceptance Criteria).

## Decision: Option A — `next/font/local` with self-hosted `.woff2`/`.ttf` files

Chosen over Option B (`@fontsource/inter` + `@fontsource/literata`).

Rationale:

- **Smaller, lower-risk change surface.** `next/font/local` exposes the exact same `.className` / `.variable` API as `next/font/google`. `apps/web/src/app/layout.tsx` and `apps/admin/src/app/layout.tsx` need only their `import` statement and constructor call changed (`Inter(...)` / `Literata(...)` → `localFont({...})`); `inter.className` on `<body>` and `literata.variable` stay exactly as they are.
- **Zero Tailwind changes required.** `apps/web/tailwind.config.ts` maps `fontFamily.display` to `var(--font-display)` (line 19), which is populated by `literata.variable` — `next/font/local`'s `variable` option produces the same CSS custom property mechanism, so all 17 files under `apps/web/src` that use the `font-display` utility class keep working unmodified. `apps/admin/tailwind.config.ts` has no font-family customization at all (just the shared preset from `packages/ui/tailwind.config.ts`), so admin needs no Tailwind changes either.
- **No new live-fetch risk.** All files are committed to the repo; `next build` never touches the network for fonts.
- Option B was rejected because it requires: (a) rewriting both layout files to drop `next/font`'s className/variable mechanism in favor of plain CSS imports + manual `font-family` declarations, (b) manually re-wiring `--font-display` in `globals.css` to point at Fontsource's font-family name instead of a `next/font`-generated one, and (c) auditing all 17 `font-display` usages and the base `<body>` font stack to make sure nothing silently falls back to a default sans-serif during the transition. This is a larger, more mechanical, more error-prone diff for no reliability benefit over Option A (Option B's "no new live-fetch risk" advantage is identical to Option A's, since Option A's files are also committed, not fetched at build time).

### Risk specific to Option A, and how to de-risk it

Do **not** attempt to hand-construct `fonts.gstatic.com` CDN URLs (e.g. by guessing/copying the hashed `.woff2` URLs seen in CI logs) — these are unstable, versioned, undocumented endpoints not meant for direct consumption and are exactly the kind of external dependency this fix is supposed to remove.

Instead, source font files from the fonts' **official upstream OFL repositories**, which ship the full glyph set (including Cyrillic — Google's "subsets" splitting is a browser-download-size optimization, not a content restriction; the canonical font files already contain both Latin and Cyrillic glyphs):

- Inter: [`rsms/inter`](https://github.com/rsms/inter) or the `ofl/inter` directory of [`google/fonts`](https://github.com/google/fonts) — use the variable font file (Inter ships as a single variable `.ttf`/`.woff2` covering weight 100–900), matching the current `Inter({ subsets: ['latin', 'cyrillic'] })` call which (no `weight` specified) implicitly uses Google's variable font.
- Literata: `ofl/literata` directory of [`google/fonts`](https://github.com/google/fonts) — use the 4 static weight files (400/500/600/700) matching the current `weight: ['400','500','600','700']`, or the variable file restricted to that weight range; either works with `next/font/local`.

Both repos already include an `OFL.txt` license file alongside the font sources — copy that file in as the attribution/license record for the self-hosted copies (see Documentation section; this repo has no prior convention for committing third-party binary assets, so this establishes one).

`next/font/local` requires explicit `weight` metadata per `src` entry. For a variable font, this means passing `weight: '100 900'` (a range) in the `localFont({ src: [...] })` config so that Tailwind's `font-bold`/`font-semibold` utilities (which set CSS `font-weight`) render correctly via the variable font's weight axis instead of silently falling back to synthetic/faux bold.

## Acceptance Criteria

- [x] `apps/web/src/fonts/` contains committed Inter (variable, Latin+Cyrillic glyph coverage) and Literata (400/500/600/700, Latin+Cyrillic glyph coverage) font files sourced from the official OFL upstream repos, plus an `OFL.txt`/`LICENSE` file for each font family.
- [x] `apps/admin/src/fonts/` contains the same committed Inter font file(s) (admin only uses Inter) plus its `OFL.txt`/`LICENSE` file.
- [x] `apps/web/src/app/layout.tsx`: `Inter`/`Literata` imports from `next/font/google` replaced with `next/font/local`; `inter.className` still applied to `<body>` exactly as before; `literata` still configured with `variable: '--font-display'` and its resulting class still applied to `<body>`.
- [x] `apps/admin/src/app/layout.tsx`: `Inter` import from `next/font/google` replaced with `next/font/local`; `inter.className` still applied to `<body>` exactly as before.
- [x] No remaining `next/font/google` imports anywhere in `apps/web/src` or `apps/admin/src` (verify via `rg "next/font/google" apps/web/src apps/admin/src` returning no matches).
- [x] `apps/web/tailwind.config.ts` and `apps/admin/tailwind.config.ts` are unchanged (no font-family/plugin changes needed — confirm this holds after the swap, don't change them speculatively).
- [x] `pnpm run build` (root, Turborepo, both apps) succeeds with no network calls to `fonts.gstatic.com`/`fonts.googleapis.com`. Verify locally by one of: (a) blocking those hosts (e.g. hosts-file entries pointing them at `127.0.0.1` or an offline/airplane-mode network) and confirming the build still succeeds, or (b) confirming via build output/tracing that no request to those hosts occurs.
- [x] Visual/typography spot check: `apps/web` renders body text in Inter and headings/display text using the `font-display` utility in Literata, both weights and Cyrillic characters (e.g. a Russian book title) rendering correctly; `apps/admin` renders body text in Inter with Cyrillic rendering correctly. No visual diff versus current production output is expected or intended.
- [x] `pnpm run lint` and `pnpm run type-check` pass for both apps (new local font files/imports don't introduce lint/type errors).
- [x] `.github/workflows/ci.yml` is unchanged.

## Business Rules

- Font family, weights, and language subsets (Latin + Cyrillic) must remain identical in outcome to the current `next/font/google` configuration — this is a sourcing/loading-mechanism change only.

## Existing Patterns

- `next/font/local` API mirrors `next/font/google`: `localFont({ src, variable, weight?, style? })` returns the same `{ className, variable, style }` shape apps already consume — see current usage in `apps/web/src/app/layout.tsx:8-13` and `apps/admin/src/app/layout.tsx:7`.
- `apps/web/tailwind.config.ts:18-20` (`fontFamily.display: ['var(--font-display)']`) is the only Tailwind hook into font loading in either app; it must keep working unmodified.
- No existing repo convention for committing third-party binary assets with license files — this feature establishes one (`src/fonts/<family>/` directory + `OFL.txt` per family), which future icon/asset additions may reuse.

## Frontend

- New files: `apps/web/src/fonts/inter/*.woff2` (or `.ttf`), `apps/web/src/fonts/inter/OFL.txt`; `apps/web/src/fonts/literata/*.woff2` (×4 weights), `apps/web/src/fonts/literata/OFL.txt`; `apps/admin/src/fonts/inter/*.woff2`, `apps/admin/src/fonts/inter/OFL.txt`.
- Modified files: `apps/web/src/app/layout.tsx`, `apps/admin/src/app/layout.tsx` (import + constructor swap only, per Acceptance Criteria above).
- No changes to any component consuming `font-display` or default body font styling.

## Documentation

- Add a short note (README in `apps/web/src/fonts/` and `apps/admin/src/fonts/`, or a single line in each app's top-level README if one exists) recording: font family, version/source commit of the upstream repo the files were pulled from, and that `OFL.txt` is the license. Keep it minimal — a few lines, not a new doc page.
- No `docs/ROADMAP.md` change needed (this is a bug fix, not a roadmap item).

## Risks

- Sourcing the exact right upstream files (variable vs. static, correct weight range metadata) is the main implementation risk flagged in Option A; mitigated by using the canonical OFL upstream repos rather than reverse-engineering CDN URLs, and by the explicit `weight: '100 900'` requirement for the Inter variable font called out above. If the developer agent cannot reliably fetch binary files directly (tooling-dependent), flag this back rather than guessing/fabricating font bytes.
- Self-hosted font files increase repo size modestly (a few hundred KB–1MB per family) — acceptable tradeoff for build reliability, not worth optimizing further in this pass (e.g. no need to hand-split into per-unicode-range chunks like Google does).
- If `apps/admin`'s build "passing" in the referenced CI run was luck rather than a reliable pass, there's no way to regression-test the *old* flakiness disappearing other than the network-blocked local verification in Acceptance Criteria — CI itself won't get a new automated check per the out-of-scope constraint.
