import { BookOpen, Download, LayoutDashboard, ShieldCheck, Users, type LucideIcon } from 'lucide-react';

/**
 * Single source of truth for admin navigation: sidebar sections, section tabs
 * and the client-side route guard all read from this config.
 *
 * Phases 1-4 (#284-#287) applied; no legacy routes remain.
 * See docs/features/admin-navigation-restructure.md.
 *
 * The frontend guard is UX only — backend role checks remain authoritative.
 */

export type StaffRole = 'admin' | 'moderator';

/**
 * A sub-page of a tab (e.g. an import method). Inherits the tab's roles;
 * optional `roles` can only narrow them (see canAccessSubtab).
 */
export interface NavSubtab {
  id: string;
  label: string;
  href: string;
  /** Pathname prefixes this sub-tab owns (longest prefix wins within the tab). */
  match: string[];
  roles?: StaffRole[];
}

export interface NavTab {
  id: string;
  label: string;
  /** Where the tab links to (may include a query string). */
  href: string;
  /** Pathname prefixes this tab owns; [] = owns nothing. */
  match: string[];
  roles: StaffRole[];
  /** Button label used when this tab is a user's home (e.g. on AccessDenied). */
  homeLabel?: string;
  /** Sub-pages; when present, `href` must equal one of `subtabs[].href` (the default). */
  subtabs?: NavSubtab[];
}

export type NavSectionId = 'overview' | 'moderation' | 'catalog' | 'import' | 'users';

export interface NavSection {
  id: NavSectionId;
  label: string;
  icon: LucideIcon;
  /** Bare section path that redirects to the first visible tab. */
  basePath?: string;
  tabs: NavTab[];
}

export const navSections: NavSection[] = [
  {
    id: 'overview',
    label: 'Обзор',
    icon: LayoutDashboard,
    tabs: [
      {
        id: 'overview.dashboard',
        label: 'Обзор',
        href: '/',
        match: ['/'],
        roles: ['admin'],
        homeLabel: 'На главную',
      },
    ],
  },
  {
    id: 'moderation',
    label: 'Модерация',
    icon: ShieldCheck,
    basePath: '/moderation',
    tabs: [
      {
        id: 'moderation.books',
        label: 'Книги',
        href: '/moderation/books',
        match: ['/moderation/books'],
        roles: ['admin'],
      },
      {
        id: 'moderation.authors',
        label: 'Авторы',
        href: '/moderation/authors',
        match: ['/moderation/authors'],
        roles: ['admin'],
      },
      {
        id: 'moderation.genres',
        label: 'Жанры',
        href: '/moderation/genres',
        match: ['/moderation/genres'],
        roles: ['admin'],
      },
      {
        id: 'moderation.reviews',
        label: 'Отзывы',
        href: '/moderation/reviews',
        match: ['/moderation/reviews'],
        roles: ['admin', 'moderator'],
        homeLabel: 'К отзывам',
      },
    ],
  },
  {
    id: 'catalog',
    label: 'Каталог',
    icon: BookOpen,
    basePath: '/catalog',
    tabs: [
      {
        id: 'catalog.books',
        label: 'Книги',
        href: '/catalog/books',
        match: ['/catalog/books'],
        roles: ['admin'],
      },
      {
        id: 'catalog.authors',
        label: 'Авторы',
        href: '/catalog/authors',
        match: ['/catalog/authors'],
        roles: ['admin'],
      },
      {
        id: 'catalog.genres',
        label: 'Жанры',
        href: '/catalog/genres',
        match: ['/catalog/genres'],
        roles: ['admin'],
      },
    ],
  },
  {
    id: 'import',
    label: 'Импорт',
    icon: Download,
    basePath: '/import',
    tabs: [
      {
        id: 'import.books',
        label: 'Книги',
        // Default sub-page; the bare /import/books path redirects here.
        href: '/import/books/wikidata',
        match: ['/import/books'],
        roles: ['admin'],
        subtabs: [
          {
            id: 'import.books.manual',
            label: 'Вручную',
            href: '/import/books/manual',
            match: ['/import/books/manual'],
          },
          {
            id: 'import.books.wikidata',
            label: 'Wikidata',
            href: '/import/books/wikidata',
            match: ['/import/books/wikidata'],
          },
          {
            id: 'import.books.file',
            label: 'Из файла',
            href: '/import/books/file',
            match: ['/import/books/file'],
          },
        ],
      },
      {
        id: 'import.authors',
        label: 'Авторы',
        // Default sub-page; the bare /import/authors path redirects here.
        href: '/import/authors/manual',
        match: ['/import/authors'],
        roles: ['admin'],
        subtabs: [
          {
            id: 'import.authors.manual',
            label: 'Вручную',
            href: '/import/authors/manual',
            match: ['/import/authors/manual'],
          },
          {
            id: 'import.authors.wikidata',
            label: 'Wikidata',
            href: '/import/authors/wikidata',
            match: ['/import/authors/wikidata'],
          },
          {
            // Always visible; the page itself says when AI curation is off.
            id: 'import.authors.ai',
            label: 'Подбор ИИ',
            href: '/import/authors/ai',
            match: ['/import/authors/ai'],
          },
        ],
      },
      {
        id: 'import.genres',
        label: 'Жанры',
        // Default sub-page; the bare /import/genres path redirects here.
        href: '/import/genres/manual',
        match: ['/import/genres'],
        roles: ['admin'],
        subtabs: [
          {
            id: 'import.genres.manual',
            label: 'Вручную',
            href: '/import/genres/manual',
            match: ['/import/genres/manual'],
          },
          {
            id: 'import.genres.wikidata',
            label: 'Wikidata',
            href: '/import/genres/wikidata',
            match: ['/import/genres/wikidata'],
          },
        ],
      },
      { id: 'import.jobs', label: 'Задачи', href: '/import/jobs', match: ['/import/jobs'], roles: ['admin'] },
    ],
  },
  {
    id: 'users',
    label: 'Пользователи',
    icon: Users,
    tabs: [{ id: 'users.list', label: 'Пользователи', href: '/users', match: ['/users'], roles: ['admin'] }],
  },
];

export type NavLocation =
  | { section: NavSection; tab: NavTab; subtab?: NavSubtab; isBasePath?: false }
  | { section: NavSection; tab?: undefined; isBasePath: true };

/** Prefix `p` matches iff pathname === p or pathname starts with `p/`; `/` matches only `/`. */
export function matchesPrefix(pathname: string, prefix: string): boolean {
  if (prefix === '/') return pathname === '/';
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

function stripQuery(pathname: string): string {
  const idx = pathname.search(/[?#]/);
  return idx === -1 ? pathname : pathname.slice(0, idx);
}

/**
 * Resolves the section/tab owning `pathname`. The longest matching prefix
 * across all tabs wins; an exact bare section path resolves to that section's
 * base path. Returns null for unknown paths.
 */
export function resolveNavLocation(
  pathname: string,
  sections: NavSection[] = navSections
): NavLocation | null {
  const path = stripQuery(pathname);
  let best: { section: NavSection; tab: NavTab; length: number } | null = null;

  for (const section of sections) {
    for (const tab of section.tabs) {
      for (const prefix of tab.match) {
        if (matchesPrefix(path, prefix) && (!best || prefix.length > best.length)) {
          best = { section, tab, length: prefix.length };
        }
      }
    }
  }

  if (best) {
    let subtab: { subtab: NavSubtab; length: number } | null = null;
    for (const candidate of best.tab.subtabs ?? []) {
      for (const prefix of candidate.match) {
        if (matchesPrefix(path, prefix) && (!subtab || prefix.length > subtab.length)) {
          subtab = { subtab: candidate, length: prefix.length };
        }
      }
    }
    return subtab
      ? { section: best.section, tab: best.tab, subtab: subtab.subtab }
      : { section: best.section, tab: best.tab };
  }

  const baseSection = sections.find((s) => s.basePath && s.basePath === path);
  if (baseSection) return { section: baseSection, isBasePath: true };

  return null;
}

export function canAccessTab(tab: NavTab, role: string | undefined): boolean {
  return !!role && (tab.roles as string[]).includes(role);
}

export function getVisibleTabs(section: NavSection, role: string | undefined): NavTab[] {
  return section.tabs.filter((tab) => canAccessTab(tab, role));
}

/** A sub-tab is accessible iff its tab is and `subtab.roles` (if set) includes the role. */
export function canAccessSubtab(tab: NavTab, subtab: NavSubtab, role: string | undefined): boolean {
  return canAccessTab(tab, role) && (!subtab.roles || (subtab.roles as string[]).includes(role!));
}

export function getVisibleSubtabs(tab: NavTab, role: string | undefined): NavSubtab[] {
  return (tab.subtabs ?? []).filter((subtab) => canAccessSubtab(tab, subtab, role));
}

export function getTabById(id: string, sections: NavSection[] = navSections): NavTab | null {
  for (const section of sections) {
    const tab = section.tabs.find((t) => t.id === id);
    if (tab) return tab;
  }
  return null;
}

/** Href of the tab with `id` (its default sub-page when it has sub-tabs). Throws on unknown ids. */
export function getTabHref(id: string, sections: NavSection[] = navSections): string {
  const tab = getTabById(id, sections);
  if (!tab) throw new Error(`Unknown nav tab: ${id}`);
  return tab.href;
}

/** Sections with tabs filtered by role; sections with no visible tabs are dropped. */
export function getVisibleSections(
  role: string | undefined,
  sections: NavSection[] = navSections
): NavSection[] {
  return sections
    .map((section) => ({ ...section, tabs: getVisibleTabs(section, role) }))
    .filter((section) => section.tabs.length > 0);
}

/** Href of the first tab of `section` visible to `role`, or null if none. */
export function getSectionHref(section: NavSection, role: string | undefined): string | null {
  return getVisibleTabs(section, role)[0]?.href ?? null;
}

/** The user's home tab: first visible tab of the first visible section. */
export function getHomeTab(
  role: string | undefined,
  sections: NavSection[] = navSections
): NavTab | null {
  return getVisibleSections(role, sections)[0]?.tabs[0] ?? null;
}

/** The user's home href; falls back to `/` when the role sees nothing. */
export function getHomeHref(role: string | undefined, sections: NavSection[] = navSections): string {
  return getHomeTab(role, sections)?.href ?? '/';
}
