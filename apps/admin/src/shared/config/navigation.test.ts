import { describe, expect, it } from 'vitest';
import {
  getHomeHref,
  getSectionHref,
  getVisibleSections,
  navSections,
  resolveNavLocation,
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
    expect(getSectionHref(importSection, 'admin')).toBe('/import/books');
    expect(getSectionHref(importSection, 'moderator')).toBeNull();
  });
});
