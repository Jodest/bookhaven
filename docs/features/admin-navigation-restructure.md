# Admin Navigation Restructure

Status: in progress (phases 1-3 done)

Epic: #283. Phases (existing sub-issues): #284 shell/tabs/guard -> #285 Catalog -> #286 Moderation -> #287 Import -> #288 Overview + counters -> #289 Users (separate contract) -> #290 Genres moderation/import (needs product decision).

Phases 1-5 are frontend-only (`apps/admin`). Phase 1 (#284) is done. Phases 2+3 (#285 + #286) are specified at implementation level below and ship **together in one branch/PR** (human decision: avoids a window where pending-review books have no page). Phases 4-7 stay outlines.

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
- A user's home = href of the first visible tab of the first visible section (admin: `/`; moderator: the reviews tab href — `/reviews` in phase 1, `/moderation/reviews` from #285+#286).

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

Known transitional quirk (accepted; removed by #285+#286, see the phase 2+3 table): Модерация/Книги links to the filtered books list, which by pathname belongs to Каталог, so after clicking it the sidebar highlights Каталог and the Каталог tabs show. Do not add search-param-aware matching to work around it (`useSearchParams` in the root-layout Shell would force a Suspense boundary for every page).

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

## Phases 2+3 — #285 Catalog + #286 Moderation (one PR)

Frontend only (`apps/admin`). Backend, `packages/api-client`, `middleware.ts`, `login-form.tsx` unchanged. Branch closes both issues (remember: GitHub auto-closes only the first `Closes #` — close the second manually after merge).

### Decisions

- **No redirects** from old routes; old pages are deleted (404 afterwards). Accepted: admin is internal, bookmarks break.
- **`/books/import` moves early to `/import/file`** (was planned for #287). Rationale — least churn: after this PR the `books` segment would otherwise contain only `import`, forcing us to keep `app/books/layout.tsx` and a `/books/import` match for one more phase while `/books` itself 404s. Moving it now is one thin page + one config edit, deletes the whole `app/books` segment, and #287 no longer touches it. `BookImportView` itself is unchanged except its back link.
- **Wikidata single-author import panel stays on `/moderation/authors`** (toggle button "Импортировать из Wikidata" + `WikidataImportPanel` inside `views/author-moderation`). It moves to `/import/author` in #287. Removing it here would make it unreachable. Импорт/Автор tab stays omitted.
- **`/harvest` stays** (with its legacy layout) until #287; only its outbound links are updated.
- **Publish actions leave the catalog.** The per-row "Опубликовать" button in the catalog list is shown only for `is_hidden && external_source`, i.e. exactly the `pending_review` set, so both per-row publish and "Опубликовать все (N)" move (not copy) to `/moderation/books`. Catalog books become pure CRUD + filters. Hidden harvested books still appear under catalog `visibility=hidden` and can be unhidden through the edit form.
- **No row selection for books.** `adminBooksApi.bulkPublish({ search })` publishes everything matching the server-side `pending_review` predicate (+ search); there is no ids-based endpoint and the backend is out of scope. The moderation view therefore exposes **only the search filter**, so the bulk scope always equals the displayed list.
- Cross-section header buttons are removed: "Импортировать" (books list), "Очередь модерации" (authors list), "Все авторы" (author moderation header). Sidebar + tabs replace them. Buttons that point into the *same task flow* stay and are repointed: harvest "К очереди на проверку"/"К авторам на модерации", book import "К списку книг".
- After saving a book opened from `/moderation/books`, `BookForm` still returns to `/catalog/books` (accepted; no `returnTo` param in this PR).

### Acceptance Criteria

Catalog (#285):

- [x] `/catalog/books`, `/catalog/books/new`, `/catalog/books/[id]/edit`, `/catalog/authors`, `/catalog/genres` render the existing views (`BooksListView`, `BookNewView`, `BookEditView`, `AuthorsView`, `GenresView`) with Каталог highlighted and Каталог tabs (Книги / Авторы / Жанры) shown; the correct tab is active, including on `/new` and `/[id]/edit` (Книги).
- [x] Bare `/catalog` redirects (`router.replace`) to the first visible tab (`/catalog/books` for admin); a moderator on `/catalog` gets AccessDenied.
- [x] Old routes `/books`, `/books/new`, `/books/[id]/edit`, `/authors`, `/genres` are deleted (404). Moderator on any of them: AccessDenied (unknown path = admin-only), as before.
- [x] Book create/update success and the form "Отмена" button navigate to `/catalog/books`.
- [x] Books list: "Добавить книгу" -> `/catalog/books/new`; row "Редактировать" -> `/catalog/books/{id}/edit`; filter/pagination URLs stay on `/catalog/books` (built from `usePathname()` — no change needed).
- [x] Books list header has no "Импортировать" button; authors list header has no "Очередь модерации" button.
- [x] Books list visibility filter options are exactly Все / Видимые / Скрытые; any other `visibility` URL value (e.g. legacy `pending_review`) is treated as `''` (not sent to the API, select shows "Все").
- [x] Books list has no "Опубликовать" row button and no "Опубликовать все" button.

Moderation (#286):

- [x] `/moderation/books` (admin only): title "Книги на проверке"; lists `adminBooksApi.list({ visibility: 'pending_review', search, page, limit: 20 })`; columns Название / Авторы / Жанры / Действия; search form (`?search=`), pagination (`?page=`) via URL params on the current pathname; empty state "Нет книг на проверке."; error state "Не удалось загрузить книги.".
- [x] Each row: "Опубликовать" (confirm `Опубликовать книгу «{title}»?`, same full-payload `adminBooksApi.update(..., is_hidden: false)` as today) and "Редактировать" -> `/catalog/books/{id}/edit`.
- [x] "Опубликовать все (N)" is shown when `total > 0`; confirm `Опубликовать все книги на проверке (N)?`; calls `adminBooksApi.bulkPublish({ search: search || undefined })`; feedback text identical to today (`Опубликовано X из Y.` / errors sample). Both mutations invalidate `['admin','books']` and `['admin','dashboard']`.
- [x] `/moderation/authors` (admin only) renders `AuthorModerationView` unchanged in behaviour, **including** the "Импортировать из Wikidata" toggle and `WikidataImportPanel`; status/filter URL pushes stay on `/moderation/authors` (built from `usePathname()`); author-books links -> `/catalog/books/{id}/edit`; "Все авторы" button removed.
- [x] `/moderation/reviews` (admin + moderator) renders `ReviewsView` unchanged in behaviour.
- [x] Moderator: sidebar Модерация links to `/moderation/reviews`; login / `/` / `/moderation` land on `/moderation/reviews`; AccessDenied and forbidden-screen buttons ("К отзывам") link to `/moderation/reviews`; `/moderation/books` and `/moderation/authors` give AccessDenied.
- [x] Admin: sidebar Модерация and bare `/moderation` go to `/moderation/books`; on any `/moderation/*` page the sidebar highlights Модерация and tabs Книги / Авторы / Жанры / Отзывы show with the right one active (the phase-1 "Модерация/Книги highlights Каталог" quirk is gone).
- [x] Old routes `/reviews`, `/authors/moderation` are deleted.
- [x] `/import/file` renders `BookImportView` with Импорт highlighted and tab Файл active; `/books/import` is deleted.

Links and cleanup:

- [x] Every internal link listed in "Link inventory" points to the new routes; a grep of `apps/admin/src` (code and tests) for string/template literals starting with `/books`, `/authors`, `/genres` or `/reviews` returns no hits (only `/harvest` legacy links remain, by design; `shared/lib/redirect.test.ts` sample paths are arbitrary inputs and are exempt).
- [x] Legacy layouts `app/books/layout.tsx`, `app/authors/layout.tsx`, `app/genres/layout.tsx`, `app/reviews/layout.tsx` are deleted with their segments; only `app/harvest/layout.tsx` remains.
- [x] Tests listed below pass; existing admin tests pass; CI frontend build (typedRoutes) passes (verified by CI, no native `next build`).

Implementation notes (phases 2+3):

- `BookModerationView` reads `search` for the query and for `bulkPublish` from the URL (`?search=`), not from the input state, so the bulk scope always equals the displayed list; row publish now also invalidates `['admin','dashboard']` (it previously invalidated only books).
- The author moderation toggle keeps its existing label "Импорт из Wikidata".
- `shared/config/navigation.test.ts` intentionally keeps the deleted old routes (`/books`, `/authors/moderation`, ...) as null-resolution cases (required by the test list); they are inputs, not links, and are exempt from the grep criterion like `redirect.test.ts`.
- `tsc --noEmit` may report stale `.next/types/app/{books,authors,genres,reviews}` artifacts from a previous local build; they disappear on the next `next build`/`next dev` (CI starts clean).

### Route moves

| New file | Content | Deleted file |
|---|---|---|
| `app/catalog/layout.tsx` | `SectionLayout` (copy `app/moderation/layout.tsx`) | — |
| `app/catalog/page.tsx` | `<SectionIndexView sectionId="catalog" />` | — |
| `app/catalog/books/page.tsx` | `BooksListView` | `app/books/page.tsx` |
| `app/catalog/books/new/page.tsx` | `BookNewView` | `app/books/new/page.tsx` |
| `app/catalog/books/[id]/edit/page.tsx` | `BookEditView` | `app/books/[id]/edit/page.tsx` |
| `app/catalog/authors/page.tsx` | `AuthorsView` | `app/authors/page.tsx` |
| `app/catalog/genres/page.tsx` | `GenresView` | `app/genres/page.tsx` |
| `app/moderation/books/page.tsx` | `BookModerationView` (new) | — |
| `app/moderation/authors/page.tsx` | `AuthorModerationView` | `app/authors/moderation/page.tsx` |
| `app/moderation/reviews/page.tsx` | `ReviewsView` | `app/reviews/page.tsx` |
| `app/import/file/page.tsx` | `BookImportView` | `app/books/import/page.tsx` |
| — | — | `app/books/layout.tsx`, `app/authors/layout.tsx`, `app/genres/layout.tsx`, `app/reviews/layout.tsx` (whole `app/books`, `app/authors`, `app/genres`, `app/reviews` dirs gone) |

`app/moderation/layout.tsx`, `app/import/layout.tsx`, `app/moderation/page.tsx`, `app/import/page.tsx` already exist and need no change. Pages stay thin (`app` imports only `views`/`shared`). Use `git mv` for moved page files.

### Config (phase 2+3 state)

Edit `apps/admin/src/shared/config/navigation.ts` (also update the header comment: phase 1 transitional -> "phases 2+3 applied; Импорт still transitional until #287"):

| Section | Tab | href | match | roles |
|---|---|---|---|---|
| Обзор | Обзор | `/` | `['/']` | admin |
| Модерация | Книги | `/moderation/books` | `['/moderation/books']` | admin |
| Модерация | Авторы | `/moderation/authors` | `['/moderation/authors']` | admin |
| Модерация | Жанры | `/moderation/genres` | `['/moderation/genres']` | admin |
| Модерация | Отзывы (`homeLabel: 'К отзывам'`) | `/moderation/reviews` | `['/moderation/reviews']` | admin, moderator |
| Каталог (add `basePath: '/catalog'`) | Книги | `/catalog/books` | `['/catalog/books']` | admin |
| Каталог | Авторы | `/catalog/authors` | `['/catalog/authors']` | admin |
| Каталог | Жанры | `/catalog/genres` | `['/catalog/genres']` | admin |
| Импорт | Wikidata | `/harvest` | `['/harvest']` | admin — still legacy until #287 |
| Импорт | Автор | — omitted (panel lives on `/moderation/authors`) | | admin — `/import/author` in #287 |
| Импорт | Файл | `/import/file` | `['/import/file']` | admin — final |
| Импорт | Жанры | `/import/genres` | `['/import/genres']` | admin |
| Импорт | Задания | — omitted (job history on `/harvest`) | | admin — `/import/jobs` in #287 |
| Пользователи | Пользователи | `/users` | `['/users']` | admin |

Known quirks after this PR: none from phase 1 (Модерация/Книги now owns a real path). Remaining transitional bits: `/harvest` legacy route/layout, and the Wikidata author import panel living under Модерация/Авторы until #287.

### Link inventory (grep result on `main` after #284; update all)

| File | Current | New |
|---|---|---|
| `features/manage-book/ui/book-form.tsx` (post-save push, "Отмена") | `/books` x2 | `/catalog/books` |
| `views/books-list/ui/books-list-view.tsx` | `/books/import` button | remove |
| same | `/books/new` | `/catalog/books/new` |
| same | `/books/${id}/edit` | `/catalog/books/${id}/edit` |
| same | `pending_review` select option, publish/bulk-publish mutations and buttons | remove (moved to `views/book-moderation`) |
| `views/authors/ui/authors-view.tsx` | `/authors/moderation` "Очередь модерации" | remove |
| `views/author-moderation/ui/author-moderation-view.tsx` | `/authors` "Все авторы" | remove |
| same | `/books/${b.id}/edit` | `/catalog/books/${b.id}/edit` |
| `views/book-import/ui/book-import-view.tsx` | `/books` "К списку книг" | `/catalog/books` |
| `views/dashboard/ui/dashboard-view.tsx` | `/books`, `/books?visibility=hidden`, `/authors`, `/genres` | `/catalog/books`, `/catalog/books?visibility=hidden`, `/catalog/authors`, `/catalog/genres` |
| same | `/reviews?visibility=hidden`, `/books?visibility=pending_review`, `/authors/moderation` | `/moderation/reviews?visibility=hidden`, `/moderation/books`, `/moderation/authors` |
| `views/harvest/ui/harvest-view.tsx` (header + completed-job row, 2 each) | `/books?visibility=pending_review`, `/authors/moderation` | `/moderation/books`, `/moderation/authors` |
| `entities/author/lib/moderation-href.ts` | `/authors/moderation?status=…&search=…` | `/moderation/authors?…` (used by Wikidata search feature and import-result feedback) |
| `features/import-wikidata-author/lib/result-text.ts` | `/harvest` "К харвесту" | unchanged until #287 |
| `shared/config/navigation.ts` | see Config table | |

Views that build URLs from `usePathname()` (`books-list`, `authors`, `genres`, `reviews`, `author-moderation`) need no change; only their tests' mocked pathname should be updated for realism.

### New view `views/book-moderation`

- `apps/admin/src/views/book-moderation/{index.ts, ui/book-moderation-view.tsx, ui/book-moderation-view.test.tsx}`, export `BookModerationView`.
- Move (cut, not duplicate) from `BooksListView`: `publishMutation` + `handlePublish`, `bulkPublishMutation` + `handleBulkPublish`, feedback banner. Query key: `['admin', 'books', 'pending_review', { search, page }]` (still under `['admin','books']` so existing invalidations hit it). Header: `<h1>Книги на проверке</h1>` + "Опубликовать все (N)" button on the right when `total > 0` (disabled while pending).
- Imports allowed: `entities/book`, `@bookhaven/api-client` types, `@bookhaven/ui`, `shared/ui` (`Pagination`). No new abstraction / shared hook; if the publish payload builder is needed twice it is not — catalog no longer publishes.
- Badges: none needed (every row is hidden + imported); optionally show `external_source` as a small muted text. Keep it minimal.

### Tests (phases 2+3)

Update:

- `shared/config/navigation.test.ts`: resolution table -> `/catalog/books`, `/catalog/books/new`, `/catalog/books/5/edit` -> `catalog.books`; `/catalog/authors`, `/catalog/genres`; `/moderation/books`, `/moderation/authors`, `/moderation/reviews`, `/moderation/genres`; `/import/file` -> `import.file`; `/harvest` -> `import.wikidata`. Section asserts: `/moderation/authors` -> moderation, `/import/file` -> import. Base paths: add `/catalog`. Null cases: add old routes `/books`, `/books/5/edit`, `/authors`, `/authors/moderation`, `/genres`, `/reviews`, `/books/import`, plus `/catalog/unknown`, `/moderation/booksx`. Query test: `/catalog/books?visibility=hidden` -> `catalog.books`. Home: `getHomeHref('moderator') === '/moderation/reviews'`; `getSectionHref(moderation, 'admin') === '/moderation/books'`, moderator -> `/moderation/reviews`; `getSectionHref(catalog, 'admin') === '/catalog/books'`. New invariant: every non-placeholder match prefix starts with `/moderation/`, `/catalog/`, `/import/`, `/users` or is `/`, **except** the allow-listed legacy `/harvest` (comment: remove in #287).
- `widgets/admin-shell/ui/shell.test.tsx`: admin sidebar hrefs `Модерация -> /moderation/books`, `Каталог -> /catalog/books`, `Импорт -> /harvest`; active-section cases `/moderation/authors` -> Модерация, `/catalog/books/123/edit` -> Каталог, `/import/file` -> Импорт, **new** `/moderation/books` -> Модерация; moderator on `/moderation/reviews` sees only Модерация (href `/moderation/reviews`) and children; moderator on `/` -> `replace('/moderation/reviews')`; moderator AccessDenied paths `['/catalog/books', '/moderation/books', '/users', '/unknown', '/import', '/catalog']` with "К отзывам" -> `/moderation/reviews`; forbidden-screen case on `/moderation/reviews`; admin non-moderator path case `/books` -> use `/catalog/books`.
- `widgets/section-tabs/ui/section-tabs.test.tsx`: admin on `/moderation/reviews` -> tabs Книги `/moderation/books`, Авторы `/moderation/authors`, Жанры `/moderation/genres`, Отзывы `/moderation/reviews` (active); admin on `/import/file` -> Wikidata, Файл (active), Жанры; **new** admin on `/catalog/books/5/edit` -> Каталог tabs, Книги active; moderator on `/moderation/reviews` -> nothing; `SectionIndexRedirect` moderation: admin -> `/moderation/books`, moderator -> `/moderation/reviews`; **new** catalog: admin -> `/catalog/books`, moderator -> no `replace`.
- `views/books-list/ui/books-list-view.test.tsx`: pathname mock `/catalog/books`; replace the "renders the pending_review filter option" test with "visibility options are Все/Видимые/Скрытые" + "legacy `visibility=pending_review` is not sent to the API"; move the per-row publish tests (~L160-210) and the whole "Опубликовать все" describe (~L284-400) to the new view test; add asserts: no "Импортировать" link, "Добавить книгу" href `/catalog/books/new`, row edit href `/catalog/books/{id}/edit`, no "Опубликовать" button even for a hidden imported row.
- `views/authors/ui/authors-view.test.tsx`: pathname `/catalog/authors`; the "Очередь модерации" link test (~L130-135) becomes "no 'Очередь модерации' link"; push expectation `'/catalog/authors?status=rejected&page=1'`.
- `views/author-moderation/ui/author-moderation-view.test.tsx`: pathname `/moderation/authors`; book link `'/catalog/books/b1/edit'`; push expectations `/moderation/authors?...` (L279-292); assert "Все авторы" absent and the "Импортировать из Wikidata" toggle still present.
- `views/author-moderation/ui/wikidata-import-panel.test.tsx` (L148), `features/import-wikidata-author/lib/result-text.test.ts` (L37), `features/search-wikidata-author/ui/wikidata-author-search.test.tsx` (L78): `/moderation/authors?…` (follows `moderation-href.ts`); "К харвесту" `/harvest` unchanged.
- `views/dashboard/ui/dashboard-view.test.tsx`: pending import tile -> `/moderation/books`, pending authors tile -> `/moderation/authors`; add one catalog tile assertion (`Всего книг` -> `/catalog/books`) and reviews tile (`/moderation/reviews?visibility=hidden`).
- `views/harvest/ui/harvest-view.test.tsx` (~L252-258): "К авторам на модерации" -> `/moderation/authors`; add "К очереди на проверку" -> `/moderation/books`.
- `views/book-import/ui/book-import-view.test.tsx`: add "К списку книг" href `/catalog/books` (if not asserted already).
- A `book-form` test, if one exists, for the post-save push -> `/catalog/books`; otherwise covered by grep criterion (no new test required).

Add:

- `views/book-moderation/ui/book-moderation-view.test.tsx` (moved + new): calls list with `visibility: 'pending_review'`; search submit pushes `?search=…&page=1` on the mocked pathname `/moderation/books`; row publish (confirm text, payload `is_hidden: false`, success feedback "Книга опубликована."); bulk button hidden when total 0, shown as "Опубликовать все (N)" otherwise, passes `search`, report/error feedback; edit link `/catalog/books/{id}/edit`; empty state text.

### Documentation (phases 2+3)

- This contract: tick phase 2+3 criteria, `Status: in progress (phases 1-3 done)`; phase 4 outline already reflects the early `/import/file` move.
- `docs/ROADMAP.md`: #114 (bulk publish for pending-review books) — bulk publish already exists in code (`adminBooksApi.bulkPublish`); verify the issue state and tick/close it in this PR if it is still open.
- PR body: `Closes #285` and `Closes #286`; close the second issue manually after merge.

## Phase 4 — #287 Import (outline)

- Split `views/harvest` into `/import/wikidata` (harvest trigger form) and `/import/jobs` (job history/report). `/import/author`: `WikidataImportPanel` + its toggle moved out of `views/author-moderation` (still there after #285+#286) into its own view (or feature-level widget). `/import/file` already done in #285+#286.
- Add Импорт/Автор and Импорт/Задания tabs to the config; update the Wikidata tab. Delete `app/harvest/` and its legacy layout (the last one). Update `result-text.ts` "К харвесту" link.
- Harvest links to `/moderation/books` / `/moderation/authors` were already updated in #285+#286. After this phase no legacy route or layout remains; drop the `/harvest` allow-list entry from the config invariant test.

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
- Pending-review list + publish/bulk publish (source for `views/book-moderation`): `apps/admin/src/views/books-list/ui/books-list-view.tsx` and its test.
- Thin section pages: `apps/admin/src/app/moderation/{layout,page}.tsx` (`SectionLayout`, `SectionIndexView`).

## Risks

- Hooks ordering in `Shell`: the new redirect effect must be declared before the early returns.
- typedRoutes: `tsc` does not catch route errors; the CI frontend build is the gate (do not run a native `next build` on Windows).
- Legacy layouts stack with nested routes (`app/books/layout.tsx` wraps `/books/import`); correctness relies on `SectionTabs` resolving by pathname, not on the layout's location. Remove legacy layouts in the phase that empties their segment (after #285+#286 only `app/harvest/layout.tsx` remains).
- Phase ordering: resolved for #285/#286 by shipping them in one PR. The Wikidata author import panel must NOT be removed from `/moderation/authors` until #287 creates `/import/author`.
- Missed links (phases 2+3): a stale `/books…`/`/authors…`/`/reviews` href becomes a 404 (no redirects). Mitigation: link inventory + the grep acceptance criterion; template-literal hrefs (`` `/books/${id}/edit` ``) are easy to miss.
- Moving publish out of `BooksListView`: the existing publish/bulk tests must be moved, not deleted, so coverage of the publish payload and bulk report handling is preserved.
- Bulk publish scope: `bulkPublish` ignores any filter except `search`; adding genre/author filters to `/moderation/books` later would make "Опубликовать все" publish more than is displayed. Keep search-only unless the backend endpoint gains filters.
- Moderator bookmarks of `/reviews` break (404 -> AccessDenied for unknown path, with a "К отзывам" button to the new home) — acceptable, no redirects by design.
- Frontend guard is UX only; backend role checks remain authoritative (reviews endpoints admin+moderator, everything else admin).
