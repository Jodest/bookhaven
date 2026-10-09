import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { navSections, type NavSection } from '@/shared/config';
import { SectionSubtabs } from './section-subtabs';

let pathnameValue = '/';
let role: string | undefined;
// When set, resolveNavLocation reads this test-only config instead of the real one.
let customSections: NavSection[] | undefined;

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  usePathname: () => pathnameValue,
}));

vi.mock('@/entities/session', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/session')>()),
  useAuthStore: () => ({
    user: role ? { id: 'u1', email: 'staff@example.com', role } : null,
    isLoading: false,
    forbiddenPath: null,
    logout: vi.fn(),
    clearForbiddenPath: vi.fn(),
  }),
}));

vi.mock('@/shared/config', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/shared/config')>();
  return {
    ...actual,
    resolveNavLocation: (pathname: string) => actual.resolveNavLocation(pathname, customSections),
  };
});

function renderSubtabs(pathname: string) {
  pathnameValue = pathname;
  return render(<SectionSubtabs />);
}

function pills() {
  return screen.getAllByRole('link').map((l) => [l.textContent, l.getAttribute('href')]);
}

describe('SectionSubtabs', () => {
  beforeEach(() => {
    role = 'admin';
    customSections = undefined;
  });

  it('books shows three method pills in order with the active one marked', () => {
    renderSubtabs('/import/books/file');

    expect(screen.getByRole('navigation', { name: 'Способ импорта' })).toBeInTheDocument();
    expect(pills()).toEqual([
      ['Вручную', '/import/books/manual'],
      ['Wikidata', '/import/books/wikidata'],
      ['Из файла', '/import/books/file'],
    ]);
    expect(screen.getByRole('link', { name: 'Из файла' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Вручную' })).not.toHaveAttribute('aria-current');
    expect(screen.getByRole('link', { name: 'Wikidata' })).not.toHaveAttribute('aria-current');
  });

  it.each([
    ['/import/authors/manual', 'authors'],
    ['/import/genres/wikidata', 'genres'],
  ])('%s shows two pills (no file import)', (pathname, entity) => {
    renderSubtabs(pathname);

    expect(pills()).toEqual([
      ['Вручную', `/import/${entity}/manual`],
      ['Wikidata', `/import/${entity}/wikidata`],
    ]);
    const active = screen.getAllByRole('link').filter((l) => l.getAttribute('aria-current') === 'page');
    expect(active.map((l) => l.getAttribute('href'))).toEqual([pathname]);
  });

  it.each(['/import', '/import/jobs', '/catalog/books', '/moderation/books', '/', '/users', '/unknown'])(
    'admin on %s renders nothing',
    (pathname) => {
      const { container } = renderSubtabs(pathname);
      expect(container).toBeEmptyDOMElement();
    }
  );

  it.each([['moderator'], [undefined]])('role %s renders nothing on an import sub-page', (r) => {
    role = r;
    const { container } = renderSubtabs('/import/books/wikidata');
    expect(container).toBeEmptyDOMElement();
  });

  it('subtab.roles narrowing hides a pill and drops the row below two pills', () => {
    customSections = [
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
              { id: 't.b', label: 'B', href: '/import/t/b', match: ['/import/t/b'] },
              { id: 't.c', label: 'C', href: '/import/t/c', match: ['/import/t/c'], roles: ['admin'] },
            ],
          },
        ],
      },
    ];

    const { unmount } = renderSubtabs('/import/t/a');
    expect(pills().map(([label]) => label)).toEqual(['A', 'B', 'C']);
    unmount();

    role = 'moderator';
    renderSubtabs('/import/t/a');
    expect(pills().map(([label]) => label)).toEqual(['A', 'B']);

    customSections[0].tabs[0].subtabs![1].roles = ['admin'];
    const { container } = renderSubtabs('/import/t/a');
    // Only one visible pill left: the row is not rendered at all.
    expect(container).toBeEmptyDOMElement();
  });
});
