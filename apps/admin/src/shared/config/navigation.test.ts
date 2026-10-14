import { describe, expect, it } from 'vitest';
import {
  canAccessSubtab,
  getHomeHref,
  getSectionHref,
  getTabById,
  getTabHref,
  getVisibleSections,
  getVisibleSubtabs,
  navSections,
  resolveNavLocation,
  type NavSection,
} from './navigation';

function tabIdOf(pathname: string): string | null | undefined {
  const loc = resolveNavLocation(pathname);
  return loc ? loc.tab?.id : null;
}

describe('resolveNavLocation', () => {
  it.each([
    ['/', 'overview.dashboard'],
    ['/catalog/books', 'catalog.books'],
    ['/catalog/books/123/edit', 'catalog.books'],
    ['/catalog/books/5/edit', 'catalog.books'],
    ['/catalog/authors', 'catalog.authors'],
    ['/catalog/authors/1/edit', 'catalog.authors'],
    ['/catalog/genres', 'catalog.genres'],
    ['/catalog/genres/1/edit', 'catalog.genres'],
    ['/moderation/books', 'moderation.books'],
    ['/moderation/authors', 'moderation.authors'],
    ['/moderation/genres', 'moderation.genres'],
    ['/moderation/reviews', 'moderation.reviews'],
    ['/import/books', 'import.books'],
    ['/import/authors', 'import.authors'],
    ['/import/genres', 'import.genres'],
    ['/import/jobs', 'import.jobs'],
    ['/import/books/file', 'import.books'],
    ['/users', 'users.list'],
  ])('%s -> %s', (pathname, tabId) => {
    expect(tabIdOf(pathname)).toBe(tabId);
  });

  it('resolves the owning section', () => {
    expect(resolveNavLocation('/')?.section.id).toBe('overview');
    expect(resolveNavLocation('/users')?.section.id).toBe('users');
    expect(resolveNavLocation('/moderation/authors')?.section.id).toBe('moderation');
    expect(resolveNavLocation('/import/books')?.section.id).toBe('import');
  });

  it('resolves bare section paths as base paths', () => {
    const moderation = resolveNavLocation('/moderation');
    expect(moderation?.section.id).toBe('moderation');
    expect(moderation?.isBasePath).toBe(true);
    expect(moderation?.tab).toBeUndefined();

    const imp = resolveNavLocation('/import');
    expect(imp?.section.id).toBe('import');
    expect(imp?.isBasePath).toBe(true);

    const catalog = resolveNavLocation('/catalog');
    expect(catalog?.section.id).toBe('catalog');
    expect(catalog?.isBasePath).toBe(true);
  });

  // Old (pre-#285/#286/#287) routes were deleted and must resolve to nothing.
  it.each([
    '/books',
    '/books/5/edit',
    '/books/import',
    '/authors',
    '/authors/moderation',
    '/genres',
    '/reviews',
    '/harvest',
    '/import/booksx',
    '/import/unknown',
    '/catalog/unknown',
    '/moderation/booksx',
    '/unknown',
    '/moderation/unknown',
  ])(
    '%s -> null',
    (pathname) => {
      expect(resolveNavLocation(pathname)).toBeNull();
    }
  );

  it('ignores query strings', () => {
    expect(tabIdOf('/catalog/books?status=rejected')).toBe('catalog.books');
  });
});

describe('navigation config invariants', () => {
  const tabs = navSections.flatMap((s) => s.tabs);

  it('every section has at least one tab', () => {
    for (const section of navSections) {
      expect(section.tabs.length).toBeGreaterThan(0);
    }
  });

  it('every tab has non-empty roles', () => {
    for (const tab of tabs) {
      expect(tab.roles.length).toBeGreaterThan(0);
    }
  });

  it('no match prefix is claimed by two tabs', () => {
    const prefixes = tabs.flatMap((t) => t.match);
    expect(new Set(prefixes).size).toBe(prefixes.length);
  });

  it('every match prefix lives under a section path', () => {
    const sectionPrefixes = ['/moderation/', '/catalog/', '/import/'];
    for (const prefix of tabs.flatMap((t) => t.match)) {
      const ok =
        prefix === '/' ||
        prefix === '/users' ||
        sectionPrefixes.some((p) => prefix.startsWith(p));
      expect(ok, prefix).toBe(true);
    }
  });

  it('tab ids and hrefs are unique', () => {
    expect(new Set(tabs.map((t) => t.id)).size).toBe(tabs.length);
    expect(new Set(tabs.map((t) => t.href)).size).toBe(tabs.length);
  });
});

describe('visibility', () => {
  it('admin sees all five sections in order', () => {
    expect(getVisibleSections('admin').map((s) => s.label)).toEqual([
      'Обзор',
      'Модерация',
      'Каталог',
      'Импорт',
      'Пользователи',
    ]);
  });

  it('moderator sees only moderation with only the reviews tab', () => {
    const sections = getVisibleSections('moderator');
    expect(sections.map((s) => s.id)).toEqual(['moderation']);
    expect(sections[0].tabs.map((t) => t.id)).toEqual(['moderation.reviews']);
  });

  it('non-staff and anonymous users see nothing', () => {
    expect(getVisibleSections('user')).toEqual([]);
    expect(getVisibleSections(undefined)).toEqual([]);
  });

  it('does not mutate the config when filtering', () => {
    getVisibleSections('moderator');
    expect(navSections.find((s) => s.id === 'moderation')?.tabs).toHaveLength(4);
  });
});

describe('home and section hrefs', () => {
  const moderation = navSections.find((s) => s.id === 'moderation')!;
  const importSection = navSections.find((s) => s.id === 'import')!;
  const catalog = navSections.find((s) => s.id === 'catalog')!;

  it('getHomeHref', () => {
    expect(getHomeHref('admin')).toBe('/');
    expect(getHomeHref('moderator')).toBe('/moderation/reviews');
  });

  it('getSectionHref', () => {
    expect(getSectionHref(moderation, 'moderator')).toBe('/moderation/reviews');
    expect(getSectionHref(moderation, 'admin')).toBe('/moderation/books');
    expect(getSectionHref(catalog, 'admin')).toBe('/catalog/books');
    expect(getSectionHref(catalog, 'moderator')).toBeNull();
    expect(getSectionHref(importSection, 'admin')).toBe('/import/books/wikidata');
    expect(getSectionHref(importSection, 'moderator')).toBeNull();
  });
});

function subtabIdOf(pathname: string, sections?: NavSection[]): string | undefined {
  const loc = resolveNavLocation(pathname, sections);
  return loc && !loc.isBasePath ? loc.subtab?.id : undefined;
}

describe('sub-tabs', () => {
  const tabs = navSections.flatMap((s) => s.tabs);
  const importBooks = getTabById('import.books')!;

  it.each([
    ['/import/books/manual', 'import.books', 'import.books.manual'],
    ['/import/books/wikidata', 'import.books', 'import.books.wikidata'],
    ['/import/books/file', 'import.books', 'import.books.file'],
    ['/import/authors/manual', 'import.authors', 'import.authors.manual'],
    ['/import/authors/wikidata', 'import.authors', 'import.authors.wikidata'],
    ['/import/authors/ai', 'import.authors', 'import.authors.ai'],
    ['/import/genres/manual', 'import.genres', 'import.genres.manual'],
    ['/import/genres/wikidata', 'import.genres', 'import.genres.wikidata'],
  ])('%s -> %s / %s', (pathname, tabId, subtabId) => {
    expect(tabIdOf(pathname)).toBe(tabId);
    expect(subtabIdOf(pathname)).toBe(subtabId);
  });

  it.each([
    ['/import/books', 'import.books'],
    ['/import/books/unknown', 'import.books'],
    ['/import/authors/file', 'import.authors'],
    ['/import/books/ai', 'import.books'],
    ['/import/genres/ai', 'import.genres'],
    ['/import/jobs', 'import.jobs'],
  ])('%s resolves to %s with no subtab', (pathname, tabId) => {
    expect(tabIdOf(pathname)).toBe(tabId);
    expect(subtabIdOf(pathname)).toBeUndefined();
  });

  it('every tab with sub-tabs defaults (href) to one of its sub-tabs', () => {
    const withSubtabs = tabs.filter((t) => t.subtabs);
    expect(withSubtabs.map((t) => t.id)).toEqual(['import.books', 'import.authors', 'import.genres']);
    for (const tab of withSubtabs) {
      expect(tab.subtabs!.map((s) => s.href), tab.id).toContain(tab.href);
    }
  });

  it('sub-tab order is manual -> wikidata -> file -> ai and sub-tabs live under their tab', () => {
    const order = ['manual', 'wikidata', 'file', 'ai'];
    for (const tab of tabs.filter((t) => t.subtabs)) {
      const methods = tab.subtabs!.map((s) => s.href.split('/').pop()!);
      expect(methods, tab.id).toEqual(order.filter((m) => methods.includes(m)));
      for (const subtab of tab.subtabs!) {
        const nested = subtab.match.every((p) => tab.match.some((tp) => p.startsWith(`${tp}/`)));
        expect(nested, subtab.id).toBe(true);
      }
    }
    expect(importBooks.subtabs!.map((s) => s.label)).toEqual(['Вручную', 'Wikidata', 'Из файла']);
    expect(getTabById('import.authors')!.subtabs!.map((s) => s.label)).toEqual(['Вручную', 'Wikidata', 'Подбор ИИ']);
    expect(getTabById('import.genres')!.subtabs!.map((s) => s.label)).toEqual(['Вручную', 'Wikidata']);
  });

  it('sub-tabs inherit the tab roles', () => {
    expect(getVisibleSubtabs(importBooks, 'admin').map((s) => s.id)).toEqual([
      'import.books.manual',
      'import.books.wikidata',
      'import.books.file',
    ]);
    expect(getVisibleSubtabs(importBooks, 'moderator')).toEqual([]);
    expect(getVisibleSubtabs(importBooks, undefined)).toEqual([]);
    expect(getVisibleSubtabs(getTabById('import.jobs')!, 'admin')).toEqual([]);
  });

  it('subtab.roles can only narrow the tab roles', () => {
    const sections: NavSection[] = [
      {
        id: 'import',
        label: 'Импорт',
        icon: navSections[0].icon,
        tabs: [
          {
            id: 't',
            label: 'T',
            href: '/import/t/a',
            match: ['/import/t'],
            roles: ['admin', 'moderator'],
            subtabs: [
              { id: 't.a', label: 'A', href: '/import/t/a', match: ['/import/t/a'] },
              { id: 't.b', label: 'B', href: '/import/t/b', match: ['/import/t/b'], roles: ['admin'] },
            ],
          },
          {
            id: 'u',
            label: 'U',
            href: '/import/u/a',
            match: ['/import/u'],
            roles: ['admin'],
            subtabs: [
              { id: 'u.a', label: 'A', href: '/import/u/a', match: ['/import/u/a'], roles: ['moderator'] },
            ],
          },
        ],
      },
    ];
    const [t, u] = sections[0].tabs;
    expect(getVisibleSubtabs(t, 'admin').map((s) => s.id)).toEqual(['t.a', 't.b']);
    expect(getVisibleSubtabs(t, 'moderator').map((s) => s.id)).toEqual(['t.a']);
    expect(canAccessSubtab(t, t.subtabs![1], 'moderator')).toBe(false);
    // A sub-tab cannot widen access beyond its tab.
    expect(canAccessSubtab(u, u.subtabs![0], 'moderator')).toBe(false);
    expect(canAccessSubtab(u, u.subtabs![0], 'admin')).toBe(false);
    expect(subtabIdOf('/import/t/b', sections)).toBe('t.b');
  });

  it('getTabHref returns the tab default sub-page and throws on unknown ids', () => {
    expect(getTabHref('import.books')).toBe('/import/books/wikidata');
    expect(getTabHref('import.authors')).toBe('/import/authors/manual');
    expect(getTabHref('import.genres')).toBe('/import/genres/manual');
    expect(getTabById('nope')).toBeNull();
    expect(() => getTabHref('nope')).toThrow();
  });
});
