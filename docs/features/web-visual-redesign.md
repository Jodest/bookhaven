# Web Visual Redesign

Status: implemented

## Goal

Give `apps/web` (the public reader-facing storefront) a coherent, modern visual
identity — inspired loosely by a supplied "Books Online" e-commerce mockup
(dark header, prominent search, category nav, card-grid catalog, tabs,
footer) but reinterpreted with contemporary design tokens, and constrained to
BookHaven's actual feature set. No shopping/commerce UI. `apps/admin` must be
visually unaffected.

## Non-Goals / Scope Cuts

- `apps/admin` — no visual or structural changes. Regression-checked, not touched.
- No new backend endpoints, no schema changes. Every visual element must map
  to data/behavior that already exists (browsing/filtering, ratings/reviews,
  shelves, lists, reading challenge, profile, auth).
- No cart, checkout, wishlist, prices, discount badges, "deal of the month"
  promo, or any other commerce affordance from the reference image.
- No pixel-for-pixel replication of the reference mockup — it is style/layout
  inspiration only.
- No new automated visual-regression tooling (no Percy/Chromatic) — out of
  scope; smoke-check is manual, same limitation as
  `docs/features/frontend-fsd-migration.md`.
- No change to business logic, data fetching, auth flow, or FSD layer
  boundaries established in the FSD migration — this is a restyle (+ two
  small additive widgets: header search, footer), not a re-architecture.

## User Stories

- As a visitor, I see a cohesive, modern-looking storefront (not a generic
  admin-tool aesthetic) that makes me want to browse.
- As a visitor, I can search and jump into genre browsing from anywhere via
  the header, and from a quick genre-nav on the home page.
- As a user, every visual affordance I see corresponds to something I can
  actually do (filter, rate, review, shelve, list, track a challenge) — no
  dead/fake UI.

## Acceptance Criteria

- [ ] `apps/web`'s `:root` CSS variables in `apps/web/src/app/globals.css` are
      replaced with the palette in "Visual Design Direction" below.
      `apps/admin/src/app/globals.css` is untouched (byte-for-byte).
- [ ] `packages/ui/src/globals.css` and `packages/ui/tailwind.config.ts` are
      **not modified**. Any web-only token (display font, card shadow,
      highlight color) is added via `theme.extend` in `apps/web/tailwind.config.ts`
      (which already uses the shared config as a `preset`), not the shared config.
- [ ] Any change to a shared `packages/ui` component (`Button`, `Card`,
      `Badge`, `Input`, `Select`, `Table`, `Label`, `Textarea`) is **additive
      only** — new variant keys, no edits to existing variants' class strings
      or to any component's default (unspecified-variant) output. Verified by
      reading the diff: every hunk in `packages/ui/src/**` either adds a new
      `variants.*` key/branch or is a no-op.
- [ ] All FSD layer-direction rules from `docs/features/frontend-fsd-migration.md`
      continue to hold (widgets may compose entities/features; no reaching
      into another slice's internals; new `widgets/footer` and header changes
      follow the same slice conventions as `widgets/header`).
- [ ] Every restyled/added element maps to a real capability — no
      cart/wishlist/price/checkout/promo-carousel strings, icons, or badges
      anywhere in `apps/web`.
- [ ] `pnpm --filter @bookhaven/web build`, `lint`, `type-check` pass.
- [ ] `pnpm --filter @bookhaven/admin build`, `lint`, `type-check` pass (no
      regression from shared-package changes).
- [ ] Repo-wide `pnpm build`, `pnpm lint`, `pnpm type-check` (turbo) pass.
- [ ] Manual `next dev` smoke-check (see Tests) of every route below at both
      ~1440px and ~375px viewport widths, with screenshots kept/attached to
      the PR: home, `/books` (catalog, filters open + mobile drawer),
      `/books/[slug]`, `/authors/[slug]`, `/shelves`, `/shelves/[id]`,
      `/lists`, `/lists/[id]`, `/lists/public/[id]`, `/profile`,
      `/profile/[id]`, `/challenge`, `/login`, `/register`.
- [ ] `apps/admin` manual smoke-check (dashboard + books list) shows **no
      visual change** versus current `master` (same colors, radius, spacing) —
      confirms the shared-package/token isolation actually holds in practice,
      not just in the diff.
- [ ] Chosen display/heading font renders Cyrillic correctly (no
      tofu/fallback-to-generic-serif) on a Russian-text heading — verified by
      screenshot; `next/font/google` `subsets` includes a Cyrillic subset for
      that font, or a Cyrillic-supporting fallback font is used instead.
- [ ] Contrast check (WebAIM or equivalent contrast checker) recorded for:
      `foreground`-on-`background`, `primary-foreground`-on-`primary`,
      `muted-foreground`-on-`background`, and the new `highlight-foreground`-
      on-`highlight` badge combo. Normal text ≥ 4.5:1, large text (≥18px, or
      ≥14px bold) / UI components ≥ 3:1. Adjust HSL lightness if a pairing
      fails and re-record.
- [ ] `docs/ROADMAP.md` gets an entry for this feature once implemented.

## Business Rules

None — purely presentational. No authorization, validation, or data-flow
changes. (Header search reuses existing public book-search; no new
permission surface.)

## Visual Design Direction

Operationalized tokens — treat these as the literal target, not a vibe.

### Palette (HSL CSS custom properties, `apps/web/src/app/globals.css` only)

Replace the current shadcn-default `:root` block with a warm "literary"
palette (paper + ink + one deep accent), keeping the same variable *names*
so all existing Tailwind utility classes (`bg-background`, `text-foreground`,
`bg-primary`, etc.) keep working unchanged:

```
--background:        40 38% 97%   /* warm paper */
--foreground:         222 39% 11%  /* ink */
--card:                0 0% 100%   /* white card on paper bg for lift */
--card-foreground:     222 39% 11%
--popover:             0 0% 100%
--popover-foreground:  222 39% 11%
--primary:             158 64% 24% /* deep emerald */
--primary-foreground:  40 38% 97%
--secondary:           32 25% 92%  /* warm sand */
--secondary-foreground:222 39% 11%
--muted:               32 20% 93%
--muted-foreground:    222 15% 40%
--accent:              32 25% 90%  /* hover tint, neutral */
--accent-foreground:   222 39% 11%
--destructive:         0 72% 42%
--destructive-foreground: 40 38% 97%
--border:              32 20% 87%
--input:               32 20% 87%
--ring:                158 64% 24%
--radius:              0.75rem     /* up from 0.5rem: softer, more modern */
```

New, web-local-only tokens (not part of the shared shadcn set — add to
`apps/web/src/app/globals.css` `:root` and reference from
`apps/web/tailwind.config.ts`'s `theme.extend.colors.highlight`):

```
--highlight:            38 92% 50%   /* amber — rating/genre badge */
--highlight-foreground: 222 39% 11%
```

Exact HSL numbers above are a concrete starting point; the developer may
nudge lightness/saturation to satisfy the contrast acceptance criterion, but
must stay within the same hue family (paper/ink/emerald/amber) — do not
introduce a different color scheme ad hoc.

### Typography

- Body/UI font: keep `Inter` (`next/font/google`, `subsets: ['latin','cyrillic']`) — already configured in `apps/web/src/app/layout.tsx`.
- Display/heading font: add one serif Google font with confirmed Cyrillic
  support for page/section headings and book/author names — `Literata`
  (verify `cyrillic` subset availability at implementation time) with
  `PT Serif` as the fallback choice if Literata's Cyrillic coverage is
  insufficient. Wire as `font-display` via `theme.extend.fontFamily.display`
  in `apps/web/tailwind.config.ts`, loaded with `next/font/google` in
  `layout.tsx` alongside Inter (CSS variable, not replacing Inter as body font).
- Scale (apply consistently, don't invent new sizes per page):
  - Hero H1 (home only): `font-display text-5xl md:text-6xl font-semibold tracking-tight`
  - Section H2: `font-display text-2xl md:text-3xl font-semibold tracking-tight`
  - Card/list item title: `text-base font-semibold leading-tight` (body font — serif at small sizes hurts legibility)
  - Body copy / descriptions: `text-sm md:text-base text-muted-foreground`
  - Nav links / buttons: `text-sm font-medium` (existing shadcn Button already sets this)

### Spacing, radius, shadow

- Page container: `container mx-auto px-4 sm:px-6 lg:px-8` (keep existing
  `container mx-auto px-4` pattern, add responsive gutter).
- Section vertical rhythm: `py-12 md:py-16` between major page sections;
  `gap-6 md:gap-8` inside grids.
- Catalog/home card grid: `grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-6`.
- Radius: rely on the raised `--radius: 0.75rem` (cards/buttons/inputs
  already consume `rounded-lg/md/sm` derived from this var — no per-component
  radius overrides needed).
- Card shadow/hover: add a `boxShadow.card` token in
  `apps/web/tailwind.config.ts` (`theme.extend.boxShadow.card`, e.g.
  `'0 1px 2px rgba(0,0,0,.04), 0 8px 24px -8px rgba(0,0,0,.12)'`); book/author/
  list/shelf cards use `shadow-sm hover:shadow-card hover:-translate-y-0.5
  transition` instead of the current `shadow-sm hover:shadow-md`.

### Component style

- **Header** (`widgets/header`): becomes a dark bar — `bg-foreground
  text-background` (ink) instead of the current `bg-background border-b` —
  echoing the reference's dark header without copying it literally. Contains:
  logo (`font-display`), primary nav links (catalog/shelves/lists/challenge/
  profile, auth-gated as today), and a new compact inline search input (see
  Frontend below). Auth buttons keep shadcn `Button` variants but on dark bg
  use `variant="secondary"`/`variant="ghost"` with sufficient contrast
  (verify, don't assume).
- **Footer** (new `widgets/footer`): same dark ink background as header.
  Columns: "Каталог" (link to `/books` + a handful of genre links from
  `genresApi.list()`), "Аккаунт" (профиль/полки/списки/вызов — auth-aware,
  same links as header nav), "О проекте" (static one-paragraph description,
  no payment/commerce badges). One-line copyright row. Rendered from
  `RootLayout` alongside `<Header />`.
- **Cards** (book/author/shelf/list cards across catalog, home, shelves,
  lists): cover image top, rounded-xl overflow-hidden, hover lift per above.
  Replace the reference's discount badge with a real **rating badge**
  (amber `bg-highlight text-highlight-foreground`, star icon + `book.rating`)
  overlaid top-left on the cover when `book.rating > 0`, matching current
  conditional. If `book.genres` is non-empty, show up to 2 genre chips
  (`Badge` with a new additive `highlight`/`outline`-style variant) below the
  title — real data already present on the `Book` type, no new fetch.
- **Catalog filter sidebar** (`widgets/catalog-filters`): restyle as a
  bordered card panel (`rounded-xl border bg-card p-4`) echoing the
  reference's nested-category sidebar, no structural/behavioral change to
  the filter logic itself.
- **Sort/tabs**: `SortSelector` may be restyled toward a segmented-control
  look (echoing the reference's Best sellers/New Arrivals tabs) but must
  keep the exact same sort options it has today — no fabricated tabs like
  "New Arrivals"/"Used Books" unless they map to a real existing sort value.
- **Home page genre quick-nav**: horizontal scrollable row of genre chips
  (from `genresApi.list()`, first N, existing endpoint — no new backend
  work) linking to `/books?genre_id=<id>`, placed below the hero. This is
  the modernized, real-data stand-in for the reference's horizontal category
  bar.
- **Buttons/inputs/badges**: use shared `@bookhaven/ui` components as today;
  only add new `cva` variant branches where a genuinely new visual role is
  needed (e.g. a `highlight` `Badge` variant for rating/genre chips) —
  existing variants (`default`, `outline`, `ghost`, etc.) keep their current
  class strings so `apps/admin` is unaffected.

## API

None. No backend/API changes.

## Database

None.

## Frontend

- `apps/web/src/app/globals.css`: replace `:root` palette (above). No `.dark`
  block needed (dark mode is not in scope; the app is single-theme today).
- `apps/web/tailwind.config.ts`: add `theme.extend.fontFamily.display`,
  `theme.extend.boxShadow.card`, `theme.extend.colors.highlight` (+`.DEFAULT`/
  `.foreground` per the shadcn color-object convention already used in
  `packages/ui/tailwind.config.ts`). Do not touch the `presets` import.
- `apps/web/src/app/layout.tsx`: add the display font via `next/font/google`
  (CSS variable, e.g. `variable: '--font-display'`), apply its variable class
  alongside `inter.className` on `<body>` or `<html>`; render `<Footer />`
  next to the existing `<Header />`.
- `widgets/header/ui/header.tsx`: restyle (dark bar) + add inline search
  input. Reuse the existing search-submit pattern from
  `widgets/catalog-filters/ui/search-bar.tsx` (extract the minimal
  submit-on-enter/navigate-to-`/books?search=`logic to avoid duplicating two
  divergent implementations — developer's call whether that becomes a tiny
  shared hook in `widgets/catalog-filters` exported for reuse, or a
  header-local minimal version; do not fork business logic silently).
- New `widgets/footer/ui/footer.tsx` + `index.ts`, following the same
  slice/barrel convention as `widgets/header`.
- Restyle only (tokens/classNames, no logic change) in: `views/home`,
  `views/catalog`, `views/book-detail`, `views/author-detail`,
  `views/shelves`, `views/shelf-detail`, `views/lists`, `views/list-detail`,
  `views/public-list-detail`, `views/profile`, `views/public-profile`,
  `views/challenge`, `views/login`, `views/register`, and the card markup
  currently duplicated inline in `views/home` and `views/catalog` (extracting
  a shared `BookCard` presentational component is encouraged but optional —
  if done, it belongs in `entities/book/ui`, consistent with the FSD mapping
  note that entity read-UI wasn't extracted originally "since none existed as
  a separately reusable component before").
- `packages/ui/src/components/ui/badge.tsx` (and/or `button.tsx`): additive
  new variant(s) only, per Acceptance Criteria.

## Existing Patterns

- `apps/web/src/widgets/header/ui/header.tsx` — current header structure/auth
  logic to preserve while restyling.
- `apps/web/src/views/catalog/ui/catalog-view.tsx` — card markup, grid,
  mobile filter drawer, pagination pattern to restyle in place.
- `packages/ui/src/components/ui/button.tsx` — `cva` variant pattern to
  follow when adding the `highlight` badge variant.
- `apps/web/tailwind.config.ts` — already the correct, established extension
  point (`presets: [packages/ui/tailwind.config.ts]`) for web-only tokens;
  this is exactly how admin/web already diverge safely today (each app has
  its own `tailwind.config.ts` and `globals.css`).
- `docs/features/frontend-fsd-migration.md` — layer/slice conventions this
  redesign must keep obeying.

## Tests

- No new automated frontend tests (none exist today, per the FSD migration
  contract's precedent). This is a visual change; verification is
  build/lint/type-check + manual smoke-check.
- Manual smoke-check: `pnpm --filter @bookhaven/web dev`, visit every route
  listed in Acceptance Criteria at 1440px and 375px widths, screenshot each,
  confirm: header/footer render with new palette, no layout overflow/broken
  wrapping/text clipping, card grid reflows correctly at mobile width, mobile
  filter drawer still opens/closes, no console errors/hydration warnings.
- Exercise at least one write action end-to-end post-restyle to confirm no
  behavioral regression (e.g. submit a rating, add a book to a shelf).
- `apps/admin` smoke-check: `pnpm --filter @bookhaven/admin dev`, visit
  dashboard + books list, confirm pixel-similar to current `master` (same
  primary color, radius, card shadow) — this is the actual proof that the
  shared-package isolation held, not just the config diff.

## Documentation

- Add an entry to `docs/ROADMAP.md` once implemented, linking this contract.
- If the developer deviates from a specific token value (e.g. adjusts an HSL
  lightness to pass the contrast check, or swaps `Literata`→`PT Serif`),
  update this contract's "Visual Design Direction" section to match reality,
  consistent with how `frontend-fsd-migration.md` documents its own
  deviations.

## Implementation Notes / Deviations

- **All exact token values from "Visual Design Direction" were used as-is** —
  no HSL nudging was needed. Contrast was computed (WCAG relative-luminance
  formula) for the four required pairs and all pass comfortably above the
  required thresholds:
  - `foreground` on `background`: **16.81:1**
  - `primary-foreground` on `primary`: **6.70:1**
  - `muted-foreground` on `background`: **5.96:1**
  - `highlight-foreground` on `highlight`: **8.32:1**
- **`Literata` was used as specified**, no fallback to `PT Serif` needed —
  confirmed at implementation time that Google Fonts serves a genuine
  `cyrillic`/`cyrillic-ext` subset for `Literata` (checked via the
  `fonts.googleapis.com/css2` endpoint with a modern-browser `User-Agent`,
  which returns distinct `/* cyrillic */ @font-face` blocks with their own
  Cyrillic-range `unicode-range`, not just a latin/latin-ext fallback).
  Loaded via `next/font/google` with `subsets: ['latin', 'cyrillic']` and
  `variable: '--font-display'` in `apps/web/src/app/layout.tsx`.
- **Header/catalog search dedup landed in `apps/web/src/shared/ui/search-input-form.tsx`**,
  not in `widgets/catalog-filters` as the contract's "Frontend" section
  initially suggested. `apps/web/.eslintrc.json`'s `boundaries/element-types`
  rule only allows a `widgets/*` slice to import from **its own** slice
  (`["widgets", { "slice": "${from.slice}" }]`), not sibling widget slices —
  so `widgets/header` importing from `widgets/catalog-filters` would fail
  `next lint`. `shared/ui` is importable from any layer, is the correct home
  for a generic, business-logic-free icon+input+form component, and still
  satisfies the acceptance criterion (both `SearchBar` and the new
  `HeaderSearch` share the exact same markup/submit behavior, no fork).
- **Genre chips on `BookCard` use the existing `Badge` `outline` variant**,
  not the new `highlight` variant. The new `highlight` variant (amber) is
  reserved for the rating badge overlaid on the cover, so the two real,
  distinct signals (rating vs. genre) stay visually distinct instead of both
  being amber. This is within the contract's own phrasing ("Badge with a new
  additive `highlight`/`outline`-style variant") — the `highlight` variant
  was still added additively to `packages/ui`, it's just not the one used
  for genre chips.
- **Extracted `entities/book/ui/book-card.tsx` (`BookCard`)**, used by
  `views/home`, `views/catalog`, and `views/author-detail`, removing three
  near-identical copies of the book-card markup. This follows the same
  precedent as `entities/review`'s `ReviewCard` noted in
  `frontend-fsd-migration.md`.
- **`SortSelector` was restyled into a real segmented control** (button
  group with `role="radiogroup"`), replacing the native `<select>`, while
  keeping the exact same three sort values/labels (`rating`, `new`, `title`).
- Repo `package.json`s pin `next@14.2.6` / `react@^18.3.0` (not the Next 15 /
  React 19 mentioned in this contract's preamble) — pre-existing, unrelated
  to this feature; no framework upgrade was performed.
- Manual `next dev` screenshot smoke-check (1440px/375px per route, plus the
  `apps/admin` no-regression check) was not executed as part of this
  automated change — `pnpm --filter @bookhaven/web build`/`lint`/`type-check`
  and `pnpm --filter @bookhaven/admin build`/`lint`/`type-check` were run and
  pass (see PR/commit for verification), but the actual manual
  browser-screenshot pass called for in "Tests" is left for human/QA
  follow-up before merge, same limitation noted in
  `frontend-fsd-migration.md`.

## Risks

- **Subjectivity of "modern"**: mitigated by making the palette/type
  scale/spacing/radius/shadow concrete and checkable above; residual
  aesthetic judgment (e.g. exact hero copy/layout balance) is left to the
  developer's discretion but must stay within these tokens — reviewers
  should check *consistency and token adherence*, not personal taste.
- **Font Cyrillic coverage**: `Literata`'s Cyrillic support must be verified
  at implementation time (Google Fonts subset availability changes); `PT
  Serif` is the specified fallback. Do not ship a heading font that falls
  back to a mismatched system serif for Russian text.
- **Shared-package blast radius**: even "additive-only" changes to
  `packages/ui` carry risk if implemented carelessly (e.g. editing a `cva`
  base class instead of adding a variant branch). The diff-review + admin
  smoke-check acceptance criteria exist specifically to catch this.
- **No visual-regression tooling**: relies on manual screenshot smoke-checks,
  same known gap as the FSD migration; a subtle admin regression could slip
  through if the smoke-check is skipped.
- **Header search duplication**: risk of two divergent search
  implementations (header vs. catalog `SearchBar`) if the developer forks
  logic instead of extracting/reusing it.
- **Home page is an async server component** (`HomeView`) — restyling must
  not disturb its `booksApi.list()` server-side data fetch.
