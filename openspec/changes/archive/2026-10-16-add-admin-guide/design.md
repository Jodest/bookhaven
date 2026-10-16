# Design

## Context

The admin app (`apps/admin`, Next.js 14.2.6 App Router, FSD with eslint-plugin-boundaries, `typedRoutes` on, `output: 'standalone'`) has no help. Navigation, sidebar and the client-side role guard are all driven by `src/shared/config/navigation.ts`; `widgets/admin-shell/ui/shell.tsx` renders the sidebar and the guard (AccessDenied «Этот раздел доступен только администраторам.» + home button, which is «К отзывам» for a moderator). Only catalog/import/moderation use `views/section-layout` (tabs); overview, collections and users do not, so a per-section «?» must live in the Shell, not in `SectionLayout`.

Docker: `infrastructure/docker/Dockerfile.node` builder does `COPY . .` from the repo-root context, then `pnpm run build` (turbo → `next build` with cwd `apps/admin`). `docs/` is not in `.dockerignore`, so `docs/admin-guide/` is available at build time. The `admin-production` stage copies only `.next/standalone`, `public`, `.next/static` — so the markdown must be consumed at build time, not read at runtime. The dev compose service mounts the whole repo at `/app`, so `next dev` can read the files directly.

No backend changes.

## Decisions

1. **Guide location: `docs/admin-guide/*.md`** (readable on GitHub, single source). File name = chapter slug: `getting-started.md`, `adding-content.md`, `import-jobs.md`, `collections.md`, `catalog-review.md`, `moderation.md`, `duplicates.md`, `screen-works.md`, `users-roles.md`, plus `README.md` (table of contents for repo readers with relative links; not a help chapter, not rendered by `/help`).
2. **Build-time loading via a statically generated server route.** `app/help/[chapter]/page.tsx` is a server component with `export const dynamicParams = false`, `export const dynamic = 'force-static'` and `generateStaticParams()` from the chapter registry. It reads the file with `node:fs` (`fs.readFileSync(path.join(process.cwd(), '..', '..', 'docs', 'admin-guide', `${slug}.md`), 'utf8')`) and throws a clear error if the file is missing, so a missing chapter fails the build instead of shipping an empty page. Pages are prerendered into the standalone output; nothing reads `docs/` at runtime. Do not move the guide under `apps/admin` and do not add a webpack raw-loader.
3. **Renderer: `react-markdown` + `remark-gfm`** (tables), pinned to exact versions (`pnpm add --save-exact`, current majors at install time; react-markdown escapes/ignores raw HTML by default — do not add `rehype-raw`). Rendering happens in the server component, so the library is not shipped to the browser. Custom `components`:
   - `a`: a relative `*.md` href (optionally with `#fragment`) whose basename is a registered slug → `next/link` to `/help/<slug>[#fragment]`; `http(s)://` → `<a target="_blank" rel="noopener noreferrer">`; `#fragment` → plain anchor; anything else → plain text-styled anchor as-is.
   - `h1`–`h3`, `ul/ol/li`, `table/th/td`, `blockquote`, `code/pre`: Tailwind classes (no `@tailwindcss/typography` dependency).
   - `img`: not rendered (no screenshots in this change; keeps the image pipeline out of scope).
   The chapter page renders the markdown's own `# H1` as its title and shows no separate registry title (each file starts with `# <registry title>`).
4. **Chapter registry in `shared/config/help.ts`** (exported via `shared/config/index.ts`): ordered array `{ slug, title, roles: StaffRole[] }` with the nine chapters from the spec; `moderation` has `['admin','moderator']`, all others `['admin']`. Type `HelpChapterSlug` is a union of the slugs. Helpers: `getHelpChapter(slug)`, `getVisibleHelpChapters(role)`, `getHelpHref(slug)` → `/help/${slug}`.
5. **Help as a nav section.** Add `NavSectionId` `'help'` and a last section `{ id: 'help', label: 'Справка', icon: CircleHelp, tabs }` in `navSections`, with tabs generated from the registry: `help.index` (`href: '/help'`, `match: ['/help']`, roles `['admin','moderator']`) followed by one tab per chapter (`id: help.<slug>`, `href/match: /help/<slug>`, roles from the registry). Longest-prefix matching makes `/help/<slug>` resolve to the chapter tab, so the existing Shell guard shows AccessDenied for a moderator on admin-only chapters without new guard code; `/help/<unknown>` resolves to `help.index` (allowed) and then 404s. The section is last, so `getHomeTab`/`getHomeHref` are unchanged for both roles. Help tabs must not render as a tab bar: `app/help/layout.tsx` does not use `SectionLayout`.
6. **Section → chapter mapping lives in the nav config**: add required `helpChapter: HelpChapterSlug | null` to `NavTab` (null only for `help.*` tabs) and optional `helpChapter` on `NavSubtab` (override; none needed now). Mapping per the spec requirement «Section help link». Helper `getHelpHrefForPath(pathname, role)` in `shared/config`: resolve location → subtab override ?? tab chapter → return `/help/<slug>` only if that chapter is visible to the role, else null; base paths (`/moderation`, `/catalog`, `/import`) return null (they redirect anyway).
7. **«?» link**: `shared/ui/help-link.tsx` (`HelpLink({ href })`, lucide `CircleHelp`, `aria-label`/`title` «Справка по разделу», ghost icon styling). Shell renders, inside `<main>` above `{children}`, a right-aligned row with `HelpLink` when `getHelpHrefForPath(pathname, role)` is non-null (so never on `/help…`, `/login`, AccessDenied screens). Keep it compact (`flex justify-end`, small negative bottom margin if needed) so page headers do not shift noticeably.
8. **Help UI**: `views/help` slice exporting
   - `HelpLayout` (client, used by `app/help/layout.tsx`): two columns on `md+` — chapter list (`getVisibleHelpChapters(role)` from `entities/session` user role; current chapter `aria-current="page"` by pathname) and content; stacks on mobile.
   - `HelpIndex` (client, `app/help/page.tsx`): heading «Справка», short intro, list of visible chapters as links.
   - `HelpChapter` (server-safe, no `'use client'`): takes `markdown: string`, renders via react-markdown with the components above.
   App files may import only `shared` and `views` (boundaries rule), so the fs read is a small helper in `shared/lib/admin-guide.ts` (`readGuideChapter(slug)`), imported only by the server route. Do not import it from client components (it pulls `node:fs`).
9. **Content authoring**: chapters are written in Russian by reading the actual pages/features/widgets in `apps/admin/src` and the living specs in `openspec/specs/` (catalog-import, catalog-moderation, catalog-admin, collections, screen-works, reviews/review-moderation, users) — not from memory or the frozen `docs/features/*.md`. UI labels in «» exactly as rendered; menu paths «Импорт → Книги → Wikidata». No secrets, internal hostnames, tokens or credentials (the prerendered pages are reachable without sign-in; see Risks). Suggested chapter contents:
   - `getting-started`: signing in, roles in one paragraph, sidebar map (Обзор, Модерация, Каталог, Подборки, Импорт, Пользователи, Справка) with one line each, the «?» link, status model pending/published/rejected and why imported items land in moderation.
   - `adding-content`: decision table «что добавляю / способ / когда»; Wikidata (books harvest, author, genre import), «Подбор ИИ» for authors (and that the page says when AI curation is off), «Из файла» for books (format, row cap, preview/errors as implemented), «Вручную» for each entity; editing in «Каталог» afterwards.
   - `import-jobs`: «Импорт → Задачи», job states as rendered, what a failed job/row means, what to do (retry/re-run, check Wikidata id, fall back to manual), when to escalate to the maintainer.
   - `collections`: curated collections list/new/edit, adding books (incl. paste), recompute/snapshot, AI drafts («ИИ-кураторы») and reviewing them before publishing.
   - `catalog-review`: book/author/genre queues, AI verdicts, duplicate hints, bulk status, «Описания ИИ» review; link to `duplicates.md`.
   - `moderation`: self-contained for moderators — finding reviews, filters, hide/restore and its effects.
   - `duplicates`: author merge and genre merge — where, what is moved, irreversibility as implemented.
   - `screen-works`: adaptations/author films sync from Wikidata, hide/show a link, manual editing (#449).
   - `users-roles`: user list, the four roles and what each can do in the admin app, changing a role, who should get `moderator`.

## Frontend

- `apps/admin/package.json`: `react-markdown`, `remark-gfm` (exact versions); lockfile updated with `pnpm install` from the repo root.
- `src/shared/config/help.ts` (new), `navigation.ts` (help section, `helpChapter` on tabs, `getHelpHrefForPath`), `index.ts` exports.
- `src/shared/lib/admin-guide.ts` (new, server-only by convention; not re-exported from `shared/lib/index.ts` if that index is imported by client code — import it by path).
- `src/shared/ui/help-link.tsx` + export.
- `src/views/help/` (new slice: `index.ts`, `ui/help-layout.tsx`, `ui/help-index.tsx`, `ui/help-chapter.tsx`).
- `src/app/help/layout.tsx`, `src/app/help/page.tsx`, `src/app/help/[chapter]/page.tsx` (`generateStaticParams`, `dynamicParams = false`, `dynamic = 'force-static'`, `generateMetadata` → «<title> — Справка»).
- `src/widgets/admin-shell/ui/shell.tsx`: render `HelpLink`.
- typedRoutes: `/help` and `/help/[chapter]` are real routes; follow the existing `as any` pattern for config-driven hrefs.

## Existing Patterns

- Config-driven nav + guard: `src/shared/config/navigation.ts`, tests `navigation.test.ts`, `widgets/admin-shell/ui/shell.test.tsx`.
- Role read on the client: `useAuthStore` from `@/entities/session` (as in `widgets/section-tabs/ui/section-tabs.tsx`).
- Active-link styling: sidebar links in `shell.tsx`, tabs in `section-tabs.tsx`.

## Risks

- **Static content is public.** Prerendered help HTML/RSC payload is served without authentication (the guard is client-side UX). The guide must never contain secrets; it is the same content as the repo file.
- **cwd assumption.** The fs path relies on `next build`/`next dev`/vitest running with cwd `apps/admin` (true for turbo, `pnpm --filter admin`, and Docker). The loader throws a descriptive error if the directory is missing, so a wrong cwd fails loudly.
- **Turbo cache.** `docs/admin-guide/**` is outside the admin package, so a local turbo cache could serve a stale admin build after a guide-only edit. Add a per-task input in `turbo.json` (`"@bookhaven/admin#build"` with `inputs: ["$TURBO_DEFAULT$", "$TURBO_ROOT$/docs/admin-guide/**"]`, keeping `dependsOn`/`outputs`) if the installed turbo (see `pnpm-lock.yaml`) supports `$TURBO_ROOT$`; otherwise leave turbo.json unchanged and note it in the PR. CI and Docker builds start without a turbo cache, so production is unaffected either way.
- **ESM-only renderer in vitest/Next 14.** react-markdown/remark-gfm are ESM-only; Next 14 server components handle them. If vitest fails to load them, add them to `test.server.deps.inline` in `vitest.config.ts` rather than switching renderers.
- **typedRoutes.** Only the CI build catches route-type errors; no native `next build` locally.
- **Content drift.** Mitigated by the new PIPELINE rule; a chapter that describes behavior incorrectly is a docs bug, not a spec violation.
