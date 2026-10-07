# Admin Navigation Restructure

Status: in progress (phase 1 done)

Epic: #283. Phases (existing sub-issues): #284 shell/tabs/guard -> #285 Catalog -> #286 Moderation -> #287 Import -> #288 Overview + counters -> #289 Users (separate contract) -> #290 Genres moderation/import (needs product decision).

Phases 1-5 are frontend-only (`apps/admin`). The phase 1 section (#284) is the implementable part of this contract; later phases are outlined so that phase 1 does not paint them into a corner.

## Goal

Replace the flat admin sidebar (Обзор / Книги / Авторы / Жанры / Харвестер / Отзывы) and the hidden pages (`/authors/moderation`, `/books/import`, pending-review filter) with five task-oriented sections, each with tabs, driven by a single navigation config that also drives the role guard.

Target structure:

```
Обзор         /
Модерация     /moderation/books | /moderation/authors | /moderation/genres* | /moderation/reviews
Каталог       /catalog/books (+ /new, /[id]/edit) | /catalog/authors | /catalog/genres
Импорт        /import/wikidata | /import/author | /import/file | /import/genres* | /import/jobs
Пользователи  /users*
```

`*` = placeholder page. No redirects from old routes; old routes are deleted in the phase that moves their page.

## User Stories

- As an admin, I see five sections in the sidebar and switch between related pages via tabs inside a section, so hidden pages (author moderation, file import) are discoverable.
- As a moderator, I see only Модерация (with only the Отзывы tab) and land on reviews after login instead of an AccessDenied screen on `/`.
- As a developer, I add/move a page by editing one config entry; sidebar, tabs and guard follow automatically.

## Business Rules

- Roles: `admin` can access everything; `moderator` can access only review moderation. `user`/other roles see the existing "У вас недостаточно прав..." screen (unchanged).
- Authorization is per tab: each tab lists the roles allowed (`roles: ('admin' | 'moderator')[]`), compared against `user.role` exactly. Do NOT use `isModerator()` for config checks (it returns true for admins; an explicit roles list is clearer). A section is visible iff at least one of its tabs is visible to the user.
- Guard fails closed: a path that matches no config entry is admin-only (moderators get AccessDenied). Backend remains the real authority (admin endpoints are already role-checked); the frontend guard is UX only.
- A user's home = href of the first visible tab of the first visible section (admin: `/`; moderator: the reviews tab href — `/reviews` in phases 1-2, `/moderation/reviews` from #286).

---

## Phase 1 — #284 Navigation shell, section tabs, role-based guard

### Acceptance Criteria

- [x] A single config module defines sections -> tabs -> allowed roles -> href/match prefixes; sidebar, section tabs and route guard all read from it. The hardcoded `adminNav`, `moderatorNav` and `adminOnlyPaths` in `shell.tsx` are removed.
- [x] Admin sidebar shows exactly: Обзор, Модерация, Каталог, Импорт, Пользователи (in this order, with icons). Each section link points to the href of its first tab visible to the user.
- [x] Moderator sidebar shows only Модерация, linking to `/reviews`.
- [x] Sidebar highlights the section owning the current pathname (e.g. `/authors/moderation` highlights Модерация, `/books/123/edit` highlights Каталог, `/books/import` highlights Импорт).
- [x] A shared `SectionTabs` component renders the tabs of the current section (only tabs visible to the user), marks the active tab (`aria-current="page"`), and renders nothing when the section has fewer than 2 visible tabs (Обзор, Пользователи, moderator's Модерация).
- [x] Tabs are visible on both new section routes and on the legacy routes that are still in place during the transition (see "Transitional config").
- [x] Moderator opening `/` (including right after login, since login defaults to `redirect=/`) is redirected with `router.replace` to their home (`/reviews`); no AccessDenied flash for `/`.
- [x] Moderator opening any admin-only or unknown path sees AccessDenied with message "Этот раздел доступен только администраторам." and a button to their home (label "К отзывам").
- [x] Moderator can open `/reviews` and moderate as today.
- [x] Bare section paths `/moderation` and `/import` redirect (client-side, `router.replace`) to the first visible tab of that section; a moderator on `/moderation` lands on `/reviews`; a moderator on `/import` gets AccessDenied.
- [x] Placeholder pages exist and are admin-only: `/users`, `/moderation/genres`, `/import/genres` — page title plus "Раздел в разработке" text; no API calls.
- [x] The server-forbidden screen (`forbiddenPath === pathname`) button points to the user's home instead of hardcoded `/` (avoids a moderator loop through `/`).
- [x] Unauthenticated redirect to `/login?redirect=...`, the loading state, the logout button and the non-staff AccessDenied screen behave exactly as before.
- [x] All existing pages keep working at their current URLs; no page moves in this phase.
- [x] New tests listed below pass; existing admin tests still pass; CI frontend build (typedRoutes) passes.

Implementation notes (phase 1):

- The `app` layer may only import `shared`/`views` (ESLint `boundaries`), so section layouts and bare-section index pages go through a thin view slice `views/section-layout` (`SectionLayout` = `<SectionTabs />{children}`, `SectionIndexView` = `SectionIndexRedirect`) instead of importing `widgets/section-tabs` directly.
- Tabs carry an optional `homeLabel` used for the "home" button on AccessDenied/forbidden screens (`На главную` on Обзор, `К отзывам` on Модерация/Отзывы).
- Shell computes the guard before early returns; a staff user on `/` whose home is not `/` (moderator) gets `router.replace(home)` and a "Перенаправление..." placeholder. Bare `/moderation` passes the guard for moderators (they have a visible tab there); the index page then redirects.
- Sidebar links carry `aria-current="page"` on the active section.
- The "CI frontend build (typedRoutes) passes" criterion is verified by CI on the PR (no native `next build` on Windows).

### Config design

New module `apps/admin/src/shared/config/navigation.ts` (new `shared/config` segment, exported via `apps/admin/src/shared/config/index.ts`). It must not import from `entities` (FSD) — helpers take `role: string | undefined`.

Shape (indicative, names may vary):

```ts
type StaffRole = 'admin' | 'moderator';
interface NavTab {
  id: string;            // 'moderation.reviews'
  label: string;         // 'Отзывы'
  href: string;          // where the tab links to (may include a query string)
  match: string[];       // pathname prefixes this tab owns; [] = owns nothing
  roles: StaffRole[];
}
interface NavSection {
  id: 'overview' | 'moderation' | 'catalog' | 'import' | 'users';
  label: string;
  icon: LucideIcon;
  basePath?: string;     // '/moderation', '/import' — bare path redirects to first visible tab
  tabs: NavTab[];
}
```

Pure helpers (all unit-tested):

- `resolveNavLocation(pathname)` -> `{ section, tab } | { section, isBasePath: true } | null`. Matching: prefix `p` matches iff `pathname === p || pathname.startsWith(p + '/')`; prefix `/` matches only `pathname === '/'`. **Longest matching prefix across all tabs wins** (so `/authors/moderation` -> Модерация/Авторы beats Каталог/Авторы `/authors`; `/books/import` -> Импорт/Файл beats Каталог/Книги `/books`). Query strings are ignored (pathname only).
- `canAccessTab(tab, role)`, `getVisibleSections(role)` (sections with tabs filtered by role, empty sections dropped), `getSectionHref(section, role)`, `getHomeHref(role)`.

Icons: Обзор `LayoutDashboard`, Модерация `ShieldCheck`, Каталог `BookOpen`, Импорт `Download`, Пользователи `Users` (lucide-react; adjust if unavailable).

### Transitional config (phase 1 state)

Decision: tabs point to wherever the content lives **today**; each later phase only edits `href`/`match` of the tabs it moves. A tab whose content currently shares one page with another tab is **omitted** until its own page exists (no duplicate hrefs, no ambiguous matches).

| Section | Tab | href (phase 1) | match (phase 1) | roles | Final href (phase) |
|---|---|---|---|---|---|
| Обзор | Обзор | `/` | `['/']` | admin | `/` |
| Модерация | Книги | `/books?visibility=pending_review` | `[]` | admin | `/moderation/books` (#286) |
| Модерация | Авторы | `/authors/moderation` | `['/authors/moderation']` | admin | `/moderation/authors` (#286) |
| Модерация | Жанры | `/moderation/genres` | `['/moderation/genres']` | admin | same (placeholder) |
| Модерация | Отзывы | `/reviews` | `['/reviews']` | admin, moderator | `/moderation/reviews` (#286) |
| Каталог | Книги | `/books` | `['/books']` | admin | `/catalog/books` (#285) |
| Каталог | Авторы | `/authors` | `['/authors']` | admin | `/catalog/authors` (#285) |
| Каталог | Жанры | `/genres` | `['/genres']` | admin | `/catalog/genres` (#285) |
| Импорт | Wikidata | `/harvest` | `['/harvest']` | admin | `/import/wikidata` (#287) |
| Импорт | Автор | — omitted (panel lives on `/authors/moderation`) | | admin | `/import/author` (#287) |
| Импорт | Файл | `/books/import` | `['/books/import']` | admin | `/import/file` (#287) |
| Импорт | Жанры | `/import/genres` | `['/import/genres']` | admin | same (placeholder) |
| Импорт | Задания | — omitted (job history lives on `/harvest`) | | admin | `/import/jobs` (#287) |
| Пользователи | Пользователи | `/users` | `['/users']` | admin | same (placeholder until #289) |

Known transitional quirk (accepted, ends with #286): Модерация/Книги links to the filtered books list, which by pathname belongs to Каталог, so after clicking it the sidebar highlights Каталог and the Каталог tabs show. Do not add search-param-aware matching to work around it (`useSearchParams` in the root-layout Shell would force a Suspense boundary for every page).

Tab order follows the target structure (Книги first), so the admin's Модерация sidebar link goes to `/books?visibility=pending_review` in phase 1. Accepted; do not reorder tabs to work around it.

### Where tabs render

- `apps/admin/src/widgets/section-tabs/` — new widget `SectionTabs` (client). No props required: resolves the current section from `usePathname()` via `resolveNavLocation`, filters tabs by `useAuthStore().user?.role`. Styling: horizontal tab links (border-bottom active style consistent with the sidebar's `bg-primary`/muted palette), `mb-6` above page content. Links use `href={tab.href as any}` like the rest of the app (typedRoutes).
- `layout.tsx` per section segment renders `<SectionTabs />{children}`:
  - new: `app/moderation/layout.tsx`, `app/import/layout.tsx` (Каталог/Пользователи layouts arrive with #285/#289 when they have pages; Пользователи has one tab so no tabs anyway);
  - temporary legacy: `app/books/layout.tsx`, `app/authors/layout.tsx`, `app/genres/layout.tsx`, `app/harvest/layout.tsx`, `app/reviews/layout.tsx`. Because `SectionTabs` resolves by pathname, one layout per legacy segment works even where a segment straddles sections (`/books/import` -> Импорт tabs, `/authors/moderation` -> Модерация tabs). Each legacy layout is deleted together with its route in #285-#287.
- Do not render `SectionTabs` from `Shell` (keeps the human-decided per-section layout pattern).
- Bare section index pages: `app/moderation/page.tsx`, `app/import/page.tsx` render a small client component (e.g. `SectionIndexRedirect` in `widgets/section-tabs`) that `router.replace`s to `getSectionHref(section, role)`; if no visible tab, render nothing (the Shell guard already shows AccessDenied).

### Shell changes (`apps/admin/src/widgets/admin-shell/ui/shell.tsx`)

Keep, in order: `/login` passthrough, login-redirect effect, `forbiddenPath` effect, loading, no-user, non-staff AccessDenied. Then:

1. `role = user.role`; `loc = resolveNavLocation(pathname)`.
2. Allowed if: `loc` has a tab and `canAccessTab(tab, role)`; or `loc` is a section base path and the section has a visible tab; or `loc === null` and role is admin.
3. Moderator on `/` (not allowed, pathname `/`): `useEffect` -> `router.replace(getHomeHref(role))`; render the "Перенаправление..." placeholder meanwhile (hooks must be called unconditionally, before early returns — compute `loc`/allowed before the returns or place the effect at the top).
4. Otherwise not allowed: `<AccessDenied message="Этот раздел доступен только администраторам.">` with `<Link href={getHomeHref(role)}><Button variant="outline">{homeLabel}</Button></Link>` where `homeLabel` = label of the home tab for moderators ("К отзывам") / "На главную" for admins.
5. `forbiddenPath` screen: button href = `getHomeHref(role)`.
6. Sidebar: `getVisibleSections(role)` -> link `getSectionHref`, active iff `loc?.section.id === section.id`.

`login-form.tsx` needs no change (default `/` + shell redirect covers moderators). `middleware.ts` unchanged.

### Placeholder pages

Shared presentational component `apps/admin/src/shared/ui/placeholder-page.tsx` (`PlaceholderPage({ title, description? })`, exported from `@/shared/ui`), used by `app/users/page.tsx` ("Пользователи"), `app/moderation/genres/page.tsx` ("Модерация жанров"), `app/import/genres/page.tsx` ("Импорт жанров"). Text: "Раздел в разработке." Pages may live directly in `app/` (thin) — no `views/*` slice needed for placeholders.

### Tests (phase 1)

Vitest + Testing Library, mocking `next/navigation` like `views/authors/ui/authors-view.test.tsx`; mock `@/entities/session` `useAuthStore` to return `{ user, isLoading, logout, forbiddenPath, clearForbiddenPath }`.

- `shared/config/navigation.test.ts`:
  - resolution: `/` -> overview; `/books`, `/books/new`, `/books/5/edit` -> catalog.books; `/books/import` -> import.file; `/authors` -> catalog.authors; `/authors/moderation` -> moderation.authors; `/reviews` -> moderation.reviews; `/harvest` -> import.wikidata; `/users` -> users; `/moderation` -> moderation base path; `/booksx`, `/unknown` -> null; `/reviewsfoo` does not match `/reviews`.
  - invariants: every tab has non-empty `roles`; no match prefix is claimed by two tabs; every section has >= 1 tab.
  - visibility: admin -> 5 sections in order; moderator -> only moderation with only the reviews tab; `user`/undefined -> none.
  - `getHomeHref('admin') === '/'`, `getHomeHref('moderator') === '/reviews'`; `getSectionHref(moderation, 'moderator') === '/reviews'`.
- `widgets/admin-shell/ui/shell.test.tsx`:
  - admin on `/`: 5 sidebar links with expected labels/hrefs; Обзор active; children rendered.
  - admin on `/authors/moderation`: Модерация active.
  - moderator on `/reviews`: only Модерация link (href `/reviews`), children rendered.
  - moderator on `/`: `replace('/reviews')` called; children and AccessDenied not rendered.
  - moderator on `/books`, `/users`, `/unknown`: AccessDenied with "Этот раздел доступен только администраторам." and a link to `/reviews`; children not rendered.
  - admin on `/unknown`: children rendered.
  - role `user`: existing non-staff AccessDenied.
  - no user, not loading: `push` called with `/login?redirect=...` (regression).
  - `forbiddenPath === pathname` for moderator: button links to `/reviews`.
- `widgets/section-tabs/ui/section-tabs.test.tsx`:
  - admin on `/reviews`: tabs Книги, Авторы, Жанры, Отзывы with correct hrefs; Отзывы has `aria-current="page"`.
  - admin on `/books/import`: Импорт tabs (Wikidata, Файл, Жанры), Файл active.
  - moderator on `/reviews`: renders nothing.
  - admin on `/` and `/users`: renders nothing.
- `SectionIndexRedirect`: admin on `/moderation` -> `replace('/books?visibility=pending_review')`; moderator -> `replace('/reviews')`.

### Documentation (phase 1)

- This contract: tick phase 1 criteria, set `Status: in progress (phase 1 done)`.
- `docs/ROADMAP.md`: epic is listed under "Up Next"; move to Completed when the epic (#284-#288) is done.

---

## Phase 2 — #285 Catalog (outline)

- Add `app/catalog/layout.tsx`, `app/catalog/books/page.tsx`, `books/new`, `books/[id]/edit`, `app/catalog/authors/page.tsx`, `app/catalog/genres/page.tsx` reusing existing views. Update the three Каталог tabs (`href`/`match`) in the config.
- Delete `app/books/page.tsx`, `app/books/new`, `app/books/[id]/edit`, `app/authors/page.tsx`, `app/genres/` (+ their legacy layouts). **Keep** `app/books/import` (moves in #287) and `app/authors/moderation` (moves in #286) and the `app/books/layout.tsx`/`app/authors/layout.tsx` legacy layouts while those children exist.
- Update internal links: `book-form.tsx` (`router.push('/books')` x2), `books-list-view.tsx` (`/books/new`, `/books/${id}/edit`), `author-moderation-view.tsx` (`/books/${id}/edit`, back link `/authors`), `book-import-view.tsx` (`/books`). Grep for `'/books`, `'/authors`, `'/genres` before finishing.
- Remove cross-section buttons: "Импорт" (`/books/import`) on the books list, "Модерация" (`/authors/moderation`) on the authors list.
- Drop `pending_review` from the books-list visibility filter. **Ordering risk:** Модерация/Книги still links to `/books?visibility=pending_review` until #286. Either land #286 first, or drop the filter option in the same PR that creates `/moderation/books`. Do not leave pending books unreachable.

## Phase 3 — #286 Moderation (outline)

- `/moderation/books`: new view listing `visibility=pending_review` books (reuse `bookApi` list with the visibility param) with row selection and bulk publish via the existing `bookApi.bulkPublish` (`entities/book/api`). Check whether this closes ROADMAP item #114.
- `/moderation/authors`: move `author-moderation` view, removing `WikidataImportPanel` (it moves to `/import/author` in #287 — if #287 lands later, keep the panel until then so it stays reachable).
- `/moderation/reviews`: move the reviews view; moderator home becomes `/moderation/reviews` automatically via config.
- Add `app/moderation/*` pages; delete `app/reviews/`, `app/authors/moderation/` and legacy layouts no longer needed. Update `entities/author/lib/moderation-href.ts`, dashboard tile hrefs (`/reviews?visibility=hidden`, `/books?visibility=pending_review`, `/authors/moderation`), harvest-view links.
- Shell/config tests updated for new paths (moderator home).

## Phase 4 — #287 Import (outline)

- Split `views/harvest` into `/import/wikidata` (harvest trigger form) and `/import/jobs` (job history/report). `/import/author`: `WikidataImportPanel` moved out of `views/author-moderation` into its own view (or feature-level widget). `/import/file`: `book-import` view.
- Add Импорт/Автор and Импорт/Задания tabs to the config; update Wikidata/Файл tabs. Delete `app/harvest/`, `app/books/import/` and remaining legacy layouts (`app/books/layout.tsx`, `app/authors/layout.tsx` if empty).
- Harvest-view links to pending books/authors point to `/moderation/books` / `/moderation/authors`. After this phase no legacy route or layout remains; add a config test asserting no `match` prefix outside `/`, `/moderation`, `/catalog`, `/import`, `/users`.

## Phase 5 — #288 Overview + sidebar counters (outline)

- Dashboard tiles link to the new routes; content regrouped by section.
- Sidebar badges on Модерация (and its tabs) from `GET /api/v1/admin/dashboard` (`pending_import_review`, `pending_authors`, `pending_reviews`). The endpoint is admin-only: the query must be `enabled` only for admins; moderators get no badge and no request (otherwise a 403 sets `forbiddenPath`). Share the TanStack Query key with the dashboard view; invalidate after moderation mutations.

## Phase 6 — #289 Users management (outline, separate contract)

Needs its own contract (`docs/features/admin-user-management.md`); relates to ROADMAP #247. Backend: `GET /api/v1/admin/users` (paginated, search, role filter), `PATCH /api/v1/admin/users/:id/role` (admin-only; forbid changing own role; forbid demoting the last admin, checked transactionally). Frontend: replace `/users` placeholder.

## Phase 7 — #290 Genres moderation/import (proposal)

Genres have only Name/Slug/CreatedAt — no status or source. Moderation/import requires a product decision (status lifecycle, import source, harvest behaviour) and a schema migration; plan a separate contract once decided. Placeholders stay until then.

---

## Existing Patterns

- Shell, guard, login redirect: `apps/admin/src/widgets/admin-shell/ui/shell.tsx`, `access-denied.tsx`.
- Roles: `isAdmin`/`isModerator`/`useAuthStore` in `apps/admin/src/entities/session/model/auth-store.ts` (`user.role`).
- `next/navigation` mocking in tests: `apps/admin/src/views/authors/ui/authors-view.test.tsx`.
- `as any` casts for typedRoutes hrefs (project-wide convention).

## Risks

- Hooks ordering in `Shell`: the new redirect effect must be declared before the early returns.
- typedRoutes: `tsc` does not catch route errors; the CI frontend build is the gate (do not run a native `next build` on Windows).
- Legacy layouts stack with nested routes (`app/books/layout.tsx` wraps `/books/import`); correctness relies on `SectionTabs` resolving by pathname, not on the layout's location. Remove legacy layouts in the phase that empties their segment.
- Phase ordering: #285 dropping `pending_review` before #286 exists, or #286 removing the Wikidata panel before #287, would make content unreachable.
- Frontend guard is UX only; backend role checks remain authoritative (reviews endpoints admin+moderator, everything else admin).
