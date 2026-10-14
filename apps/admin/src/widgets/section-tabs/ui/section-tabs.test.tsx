import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { NavCounters } from '@/features/nav-counters';
import { SectionTabs } from './section-tabs';
import { SectionIndexRedirect } from './section-index-redirect';

const replaceMock = vi.fn();
let pathnameValue = '/';
let role: string | undefined;

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: replaceMock }),
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

let countersValue: NavCounters = { sections: {}, tabs: {} };

vi.mock('@/features/nav-counters', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/features/nav-counters')>()),
  useNavCounters: () => countersValue,
}));

function renderTabs(pathname: string) {
  pathnameValue = pathname;
  return render(<SectionTabs />);
}

describe('SectionTabs', () => {
  beforeEach(() => {
    role = 'admin';
    countersValue = { sections: {}, tabs: {} };
  });

  it('admin on /moderation/reviews sees moderation tabs with Отзывы active', () => {
    renderTabs('/moderation/reviews');

    const links = screen.getAllByRole('link');
    expect(links.map((l) => [l.textContent, l.getAttribute('href')])).toEqual([
      ['Книги', '/moderation/books'],
      ['Авторы', '/moderation/authors'],
      ['Жанры', '/moderation/genres'],
      ['Описания ИИ', '/moderation/descriptions'],
      ['Отзывы', '/moderation/reviews'],
    ]);
    expect(screen.getByRole('link', { name: 'Отзывы' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Книги' })).not.toHaveAttribute('aria-current');
  });

  it('admin on /import/authors/wikidata sees import tabs (default sub-page hrefs) with Авторы active', () => {
    renderTabs('/import/authors/wikidata');

    const links = screen.getAllByRole('link');
    expect(links.map((l) => [l.textContent, l.getAttribute('href')])).toEqual([
      ['Книги', '/import/books/wikidata'],
      ['Авторы', '/import/authors/manual'],
      ['Жанры', '/import/genres/manual'],
      ['Задачи', '/import/jobs'],
    ]);
    expect(screen.getByRole('link', { name: 'Авторы' })).toHaveAttribute('aria-current', 'page');
  });

  it('admin on /import/books/file sees Книги active', () => {
    renderTabs('/import/books/file');

    expect(screen.getByRole('link', { name: 'Книги' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Задачи' })).not.toHaveAttribute('aria-current');
  });

  it('admin on /import/jobs sees import tabs with Задачи active', () => {
    renderTabs('/import/jobs');

    expect(screen.getByRole('link', { name: 'Задачи' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Книги' })).not.toHaveAttribute('aria-current');
  });

  it('admin on /catalog/books/5/edit sees catalog tabs with Книги active', () => {
    renderTabs('/catalog/books/5/edit');

    const links = screen.getAllByRole('link');
    expect(links.map((l) => [l.textContent, l.getAttribute('href')])).toEqual([
      ['Книги', '/catalog/books'],
      ['Авторы', '/catalog/authors'],
      ['Жанры', '/catalog/genres'],
    ]);
    expect(screen.getByRole('link', { name: 'Книги' })).toHaveAttribute('aria-current', 'page');
  });

  it('moderator on /moderation/reviews renders nothing', () => {
    role = 'moderator';
    const { container } = renderTabs('/moderation/reviews');
    expect(container).toBeEmptyDOMElement();
  });

  it.each(['/', '/users', '/unknown'])('admin on %s renders nothing', (pathname) => {
    const { container } = renderTabs(pathname);
    expect(container).toBeEmptyDOMElement();
  });

  it('moderation tabs show their own counts; zero and other tabs have no badge', () => {
    countersValue = {
      sections: {},
      tabs: { 'moderation.books': 4, 'moderation.authors': 0, 'moderation.genres': 6 },
    };
    renderTabs('/moderation/reviews');

    expect(screen.getByRole('link', { name: 'Книги, ожидают проверки: 4' })).toHaveAttribute(
      'href',
      '/moderation/books'
    );
    expect(screen.getByRole('link', { name: 'Авторы' })).toHaveAccessibleName('Авторы');
    expect(screen.getByRole('link', { name: 'Отзывы' })).toHaveAccessibleName('Отзывы');
    expect(screen.getByRole('link', { name: 'Жанры, ожидают проверки: 6' })).toHaveAttribute(
      'href',
      '/moderation/genres'
    );
  });

  it('import Задачи tab shows the in-flight jobs count', () => {
    countersValue = { sections: {}, tabs: { 'import.jobs': 3 } };
    renderTabs('/import/books/wikidata');

    expect(screen.getByRole('link', { name: 'Задачи, задач в работе: 3' })).toHaveAttribute(
      'href',
      '/import/jobs'
    );
    expect(screen.getByRole('link', { name: 'Книги' })).toHaveAttribute('aria-current', 'page');
  });
});

describe('SectionIndexRedirect', () => {
  beforeEach(() => {
    replaceMock.mockReset();
  });

  it('admin on /moderation goes to the first moderation tab', () => {
    role = 'admin';
    pathnameValue = '/moderation';
    render(<SectionIndexRedirect sectionId="moderation" />);
    expect(replaceMock).toHaveBeenCalledWith('/moderation/books');
  });

  it('moderator on /moderation goes to reviews', () => {
    role = 'moderator';
    pathnameValue = '/moderation';
    render(<SectionIndexRedirect sectionId="moderation" />);
    expect(replaceMock).toHaveBeenCalledWith('/moderation/reviews');
  });

  it('admin on /catalog goes to the first catalog tab', () => {
    role = 'admin';
    pathnameValue = '/catalog';
    render(<SectionIndexRedirect sectionId="catalog" />);
    expect(replaceMock).toHaveBeenCalledWith('/catalog/books');
  });

  it('moderator on /catalog is not redirected', () => {
    role = 'moderator';
    pathnameValue = '/catalog';
    render(<SectionIndexRedirect sectionId="catalog" />);
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it('admin on /import goes to the default books sub-page', () => {
    role = 'admin';
    pathnameValue = '/import';
    render(<SectionIndexRedirect sectionId="import" />);
    expect(replaceMock).toHaveBeenCalledWith('/import/books/wikidata');
  });

  it('moderator on /import is not redirected', () => {
    role = 'moderator';
    pathnameValue = '/import';
    render(<SectionIndexRedirect sectionId="import" />);
    expect(replaceMock).not.toHaveBeenCalled();
  });
});
