import { BookOpen, Download, LayoutDashboard, ShieldCheck, Users, type LucideIcon } from 'lucide-react';

/**
 * Single source of truth for admin navigation: sidebar sections, section tabs
 * and the client-side route guard all read from this config.
 *
 * Phase 1 (#284) is transitional: tabs point to wherever the content lives
 * today; later phases only edit `href`/`match` of the tabs they move.
 * See docs/features/admin-navigation-restructure.md.
 *
 * The frontend guard is UX only — backend role checks remain authoritative.
 */

export type StaffRole = 'admin' | 'moderator';

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
        href: '/books?visibility=pending_review',
        match: [],
        roles: ['admin'],
      },
      {
        id: 'moderation.authors',
        label: 'Авторы',
        href: '/authors/moderation',
        match: ['/authors/moderation'],
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
        href: '/reviews',
        match: ['/reviews'],
        roles: ['admin', 'moderator'],
        homeLabel: 'К отзывам',
      },
    ],
  },
  {
    id: 'catalog',
    label: 'Каталог',
    icon: BookOpen,
    tabs: [
      { id: 'catalog.books', label: 'Книги', href: '/books', match: ['/books'], roles: ['admin'] },
      { id: 'catalog.authors', label: 'Авторы', href: '/authors', match: ['/authors'], roles: ['admin'] },
      { id: 'catalog.genres', label: 'Жанры', href: '/genres', match: ['/genres'], roles: ['admin'] },
    ],
  },
  {
    id: 'import',
    label: 'Импорт',
    icon: Download,
    basePath: '/import',
    tabs: [
      { id: 'import.wikidata', label: 'Wikidata', href: '/harvest', match: ['/harvest'], roles: ['admin'] },
      { id: 'import.file', label: 'Файл', href: '/books/import', match: ['/books/import'], roles: ['admin'] },
      {
        id: 'import.genres',
        label: 'Жанры',
        href: '/import/genres',
        match: ['/import/genres'],
        roles: ['admin'],
      },
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
  | { section: NavSection; tab: NavTab; isBasePath?: false }
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

  if (best) return { section: best.section, tab: best.tab };

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
