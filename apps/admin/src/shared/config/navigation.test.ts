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
    ['/books', 'catalog.books'],
    ['/books/new', 'catalog.books'],
    ['/books/5/edit', 'catalog.books'],
    ['/books/import', 'import.file'],
    ['/authors', 'catalog.authors'],
    ['/authors/moderation', 'moderation.authors'],
    ['/genres', 'catalog.genres'],
    ['/reviews', 'moderation.reviews'],
    ['/harvest', 'import.wikidata'],
    ['/users', 'users.list'],
    ['/moderation/genres', 'moderation.genres'],
    ['/import/genres', 'import.genres'],
  ])('%s -> %s', (pathname, tabId) => {
    expect(tabIdOf(pathname)).toBe(tabId);
  });

  it('resolves the owning section', () => {
    expect(resolveNavLocation('/')?.section.id).toBe('overview');
    expect(resolveNavLocation('/users')?.section.id).toBe('users');
    expect(resolveNavLocation('/authors/moderation')?.section.id).toBe('moderation');
    expect(resolveNavLocation('/books/import')?.section.id).toBe('import');
  });

  it('resolves bare section paths as base paths', () => {
    const moderation = resolveNavLocation('/moderation');
    expect(moderation?.section.id).toBe('moderation');
    expect(moderation?.isBasePath).toBe(true);
    expect(moderation?.tab).toBeUndefined();

    const imp = resolveNavLocation('/import');
    expect(imp?.section.id).toBe('import');
    expect(imp?.isBasePath).toBe(true);
  });

  it.each(['/booksx', '/unknown', '/reviewsfoo', '/moderation/unknown'])(
    '%s -> null',
    (pathname) => {
      expect(resolveNavLocation(pathname)).toBeNull();
    }
  );

  it('ignores query strings', () => {
    expect(tabIdOf('/books?visibility=pending_review')).toBe('catalog.books');
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

  it('getHomeHref', () => {
    expect(getHomeHref('admin')).toBe('/');
    expect(getHomeHref('moderator')).toBe('/reviews');
  });

  it('getSectionHref', () => {
    expect(getSectionHref(moderation, 'moderator')).toBe('/reviews');
    expect(getSectionHref(moderation, 'admin')).toBe('/books?visibility=pending_review');
    expect(getSectionHref(importSection, 'moderator')).toBeNull();
  });
});
