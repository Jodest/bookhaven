# Import Method Sub-pages

Status: done

Tracking: issue #334, a follow-up to F1 #314 / epic #310 ([import-by-entity.md](import-by-entity.md)).
Scope: `apps/admin` only. No backend, api-client, harvester (#318/#319) or catalog/moderation changes.

## Goal

Each Import entity tab (Книги / Авторы / Жанры) is split into real sub-pages, one per import method. The sub-pages replace F1's stacked `PageSection` blocks. A second row of sub-tabs, driven by the navigation config, switches between methods.

## User Stories

- As an admin, I open Импорт → Книги and land directly on the Wikidata harvest. Sub-tabs «Вручную / Wikidata / Из файла» switch the method, and each method has its own URL I can bookmark.
- As an admin, I open Импорт → Авторы or Жанры and land on «Вручную». I can switch to «Wikidata».
- As an admin with an old bookmark, I land on the exact method page.

## Acceptance Criteria

- [x] Static routes exist and render only their own panel:
  - `/import/books/manual`, `/import/books/wikidata`, `/import/books/file`
  - `/import/authors/manual`, `/import/authors/wikidata`
  - `/import/genres/manual`, `/import/genres/wikidata`
- [x] Sub-tab order is always Вручную → Wikidata → Из файла. A method an entity lacks gets no tab, route or placeholder.
- [x] The section tab hrefs are `/import/books/wikidata`, `/import/authors/manual` and `/import/genres/manual`, so there is no redirect hop. «Задачи» stays `/import/jobs` with no sub-tabs.
- [x] Bare `/import/books`, `/import/authors` and `/import/genres` respond 307 to their tab's default sub-page. Each default is one config value: the tab `href`.
- [x] `/import` still goes to the first visible tab's href, which is now `/import/books/wikidata`.
- [x] On every sub-page the section tab of its entity stays active (e.g. Книги on `/import/books/file`), and the matching sub-tab has `aria-current="page"`. The sidebar highlights Импорт.
- [x] Legacy redirects in `next.config.js` (`permanent: false`):
  - `/import/wikidata` → `/import/books/wikidata`
  - `/import/file` → `/import/books/file`
  - `/import/author` → `/import/authors/wikidata`
  - `/catalog/books/new` → `/import/books/manual`
- [x] The Dashboard link «Запустить харвест» points to `/import/books/wikidata`.
- [x] In the Wikidata author/genre panels, the empty-result hint «…в разделе «Вручную».» makes «Вручную» a link to `/import/authors/manual` or `/import/genres/manual`.
- [x] Every panel behaves exactly as in F1: same API calls, banners, invalidations and the form reset after create.
- [ ] `tsc --noEmit`, `next lint` (FSD) and `vitest` pass natively (done locally). The CI build (typedRoutes) is green (pending CI). Do NOT run a native `next build`.
- [x] Smoke test:
  - Run `docker compose restart admin`.
  - With cookie `has_session=1`, `curl` all 7 sub-pages → 200.
  - The 3 bare entity paths and 4 legacy URLs → 307, with the `Location` values above.

## Business Rules

1. Role gating is unchanged. All Import tabs are `roles: ['admin']`, and sub-tabs inherit the tab's roles. Optional `subtab.roles` can only narrow them: a sub-tab is visible/accessible iff `canAccessTab(tab)` && (`!subtab.roles` || `subtab.roles` includes the role). The frontend guard is UX only, and backend checks stay authoritative.
2. The default sub-page is the tab `href`. It must equal one of the tab's `subtabs[].href` (enforced by a test). When #318/#319 land, switching Авторы/Жанры to Wikidata is a one-line `href` edit.
3. Moving code must not change any request payload, error mapping or query key.

## UX

- **Sub-tab row:** rendered directly under the section tabs. It must look clearly secondary: a pill/segmented control (`inline-flex rounded-md bg-muted p-1`; the active pill is `bg-background shadow-sm text-foreground`, inactive pills are `text-muted-foreground`). It has `<nav aria-label="Способ импорта">` and links with `aria-current="page"` on the active one. It renders only when the resolved tab has ≥2 visible sub-tabs.
- **Labels:** «Вручную», «Wikidata», «Из файла». For books, keep «Wikidata» short; the h1 says «харвест».
- **Page header per sub-page:** an h1 plus one muted description line (reuse F1's section descriptions). The `PageSection` wrapper is dropped. Remove the `PageSection` import from these views (keep the component in shared/ui if it is used elsewhere).

| Route | h1 |
|---|---|
| `/import/books/manual` | Импорт книг: вручную |
| `/import/books/wikidata` | Импорт книг: харвест Wikidata |
| `/import/books/file` | Импорт книг: из файла |
| `/import/authors/manual` | Импорт авторов: вручную |
| `/import/authors/wikidata` | Импорт авторов: Wikidata |
| `/import/genres/manual` | Импорт жанров: вручную |
| `/import/genres/wikidata` | Импорт жанров: Wikidata |

## API

None.

## Database

None.

## Frontend

### `shared/config/navigation.ts`

- Add:
  ```ts
  export interface NavSubtab { id: string; label: string; href: string; match: string[]; roles?: StaffRole[] }
  ```
  Add `subtabs?: NavSubtab[]` to `NavTab`.
- Import tabs:
  - `import.books` gets `href: '/import/books/wikidata'`, `match: ['/import/books']` and subtabs `import.books.manual|wikidata|file`. Each subtab's match is its own href.
  - Authors and genres follow the same pattern with `manual|wikidata`, and their `href` is the `/manual` sub-page.
- `NavLocation`: the tab variant gets an optional `subtab?: NavSubtab`. `resolveNavLocation` keeps the existing longest-prefix tab resolution unchanged. It then resolves the longest-prefix match within `best.tab.subtabs` (or none).
- New helpers:
  - `canAccessSubtab(tab, subtab, role)`
  - `getVisibleSubtabs(tab, role)`
  - `getTabById(id)` (or `getTabHref(id)`), used by the bare-path redirect pages.
- `getSectionHref` and `SectionIndexRedirect` stay unchanged, because they already use `tab.href`.

### `widgets/section-tabs`

Extend this slice; do not create a new widget, so no widget imports another widget. Add `ui/section-subtabs.tsx` exporting `SectionSubtabs`:
- It uses `usePathname`, `resolveNavLocation`, `getVisibleSubtabs` and the role from `useAuthStore`.
- Export it from `index.ts`.
- Render it in `views/section-layout/ui/section-layout.tsx` right after `<SectionTabs />`.
- Adjust spacing: the tabs `mb-6` becomes smaller when sub-tabs follow. Alternatively, wrap both components in one container in SectionLayout.
  - Implemented: SectionLayout wraps both rows in `<div className="mb-6 space-y-3 empty:hidden">` and `SectionTabs` lost its own `mb-6`.

### Shell route guard (`widgets/admin-shell`)

If `loc.subtab` exists and `!canAccessSubtab(...)`, show AccessDenied. Today this is a no-op, but it keeps `subtab.roles` honest.

### App routes (`app/import/`)

- `books/page.tsx`, `authors/page.tsx` and `genres/page.tsx` become **server components** that call `redirect(getTabHref('import.books'))` (etc.) from `next/navigation`. This gives a server-side 307 and reads the single config value. The server cannot know the role, but the target page is gated by the same roles, so the Shell guard handles it.
- If the smoke test shows these bare paths returning 200 instead of 307 (e.g. because of streaming), fall back:
  1. Add the 3 bare-path entries to `next.config.js` `redirects()`.
  2. Add a vitest test that `require`s `next.config.js` and asserts each bare-path destination equals the corresponding tab `href`.

  Document whichever mechanism is used.
- **Implemented (#334):** the smoke test showed the server `redirect()` streaming a 200 (dev server, `NEXT_REDIRECT` in the RSC payload under the client root layout), so the fallback is used. The 307 comes from the 3 bare-path entries in `next.config.js` `redirects()`. `src/app/import/entity-redirects.test.tsx` asserts each destination equals `getTabHref(...)` (and checks the 4 legacy destinations). The `books/authors/genres/page.tsx` server `redirect(getTabHref(...))` pages are kept as a second line of defense and are unit-tested with a mocked `redirect`.
- `/import` itself still returns 200 and `SectionIndexRedirect` performs the client-side `router.replace` to the first visible tab href (`/import/books/wikidata` for admins).
- New pages:
  - `books/{manual,wikidata,file}/page.tsx`
  - `authors/{manual,wikidata}/page.tsx`
  - `genres/{manual,wikidata}/page.tsx`

  Each renders its entity view with a `method` prop.

### Views

Keep one slice per entity, so no view imports another view and the create-sections stay local.
- `views/import-books`:
  - `ImportBooksView({ method }: { method: 'manual' | 'wikidata' | 'file' })` renders the h1 + description, then exactly one of `BookCreateSection` / `BookHarvestPanel` / `BookFileImportPanel`.
  - Use a `const METHODS: Record<Method, { title; description; Panel }>` map.
- `views/import-authors`: `ImportAuthorsView({ method: 'manual' | 'wikidata' })` renders `AuthorCreateSection` / `WikidataAuthorImportPanel`.
- `views/import-genres`: `ImportGenresView({ method: 'manual' | 'wikidata' })` renders `GenreCreateSection` / `WikidataGenreImportPanel`.

### Empty-result hint links

- In `features/search-wikidata-author` and `features/search-wikidata-genre`, widen the `emptyHint` type from `string` to `React.ReactNode`.
- `widgets/wikidata-author-import` and `widgets/wikidata-genre-import` then pass a fragment containing `<Link href="/import/authors/manual">«Вручную»</Link>` (or `/import/genres/manual`).

### Link and comment sweep

Update every `/import/books|authors|genres` reference in `apps/admin/src`:
- `views/dashboard/ui/dashboard-view.tsx:146` → `/import/books/wikidata`
- JSDoc comments in:
  - `book-harvest-panel`, `book-file-import-panel`
  - `wikidata-author-import-panel`, `wikidata-genre-import-panel`
  - `book-create-section`, `author-create-section`, `genre-create-section`
  - `book-form.tsx:45`

`/import/jobs` links stay as they are.

### `next.config.js`

Update the 4 legacy destinations as listed in the Acceptance Criteria.

## Existing Patterns

- `widgets/section-tabs/ui/section-tabs.tsx`: the config-driven tab rendering and active state to mirror.
- `widgets/section-tabs/ui/section-index-redirect.tsx` and `views/section-layout`: bare section index handling (unchanged).
- `shared/config/navigation.ts`: longest-prefix `resolveNavLocation`, plus `canAccessTab` / `getVisibleTabs`.
- F1 views and tests under `views/import-*`.

## Tests

- `shared/config/navigation.test.ts`:
  - Each of the 7 sub-paths resolves to the correct tab id and subtab id.
  - `/import/books` resolves to tab `import.books` with no subtab.
  - `/import/books/unknown` resolves to the tab with no subtab.
  - Every tab with `subtabs` has an `href` equal to one of its subtab hrefs.
  - Subtab order is manual → wikidata → file.
  - Sub-tab role filtering works, including with a test-only config that has a narrowed `subtab.roles`.
  - `getSectionHref(import, 'admin')` is `/import/books/wikidata`.
- `widgets/section-tabs/ui/section-tabs.test.tsx`:
  - Update the expected hrefs.
  - On `/import/books/file`, «Книги» is active.
  - `/import` → `replace('/import/books/wikidata')`.
- New `section-subtabs.test.tsx`:
  - Books shows 3 pills in order with the correct hrefs; the active one has `aria-current`.
  - Authors shows 2 pills.
  - `/import/jobs`, catalog and moderation pages render nothing.
  - A moderator, or no user, renders nothing.
  - Role narrowing hides a pill (test-only config, or mock the config).
- `widgets/admin-shell/ui/shell.test.tsx`:
  - Update the Импорт sidebar href to `/import/books/wikidata`.
  - `/import/books/file` highlights Импорт.
- Bare-path redirect pages: unit-test that `books/authors/genres/page.tsx` call a mocked `redirect` with `/import/books/wikidata`, `/import/authors/manual` and `/import/genres/manual`. If the next.config fallback is used, test the config consistency instead.
- `views/import-*/ui/*-view.test.tsx`: rewrite them per method. Each `method` renders the correct h1 and only its panel; mock the panels as F1 tests do. Keep the existing create-section behavior tests.
- `wikidata-author-import-panel.test.tsx` / genre equivalent: the empty hint contains a link «Вручную» to the manual sub-page.
- `dashboard-view.test.tsx`: the «Запустить харвест» href is `/import/books/wikidata`.

## Documentation

- `docs/features/import-by-entity.md`:
  - Add an "Update (#334)" note to UX / Acceptance Criteria saying that stacked sections were replaced by method sub-pages, with a link to this doc.
  - Update the redirect targets.
- `docs/features/admin-navigation-restructure.md`: if it documents the `NavTab` shape, add a short note on `subtabs`.
- This doc: set `Status: done` and tick the criteria after merge.

## Risks

- **Server `redirect()` returning 200 instead of 307.** It may stream instead of returning 307 if an ancestor suspends. There is no `loading.tsx` today, but verify with curl. The fallback is defined above.
- **typedRoutes on the new routes.** New static routes are fine. Keep the `as any` casts used for config-driven hrefs. CI is the only typedRoutes gate.
- **Changing `emptyHint` to ReactNode** touches two features. Keep the string usage working (the test harness passes strings).
- **Spacing/visual regression** between the two tab rows. Keep the sub-tab row clearly secondary so the two rows are not confused.
- **Stale references.** grep `"/import/books'"`, `"/import/authors'"` and `"/import/genres'"` (exact bare hrefs) to make sure only the redirect pages and tests reference bare entity paths.
