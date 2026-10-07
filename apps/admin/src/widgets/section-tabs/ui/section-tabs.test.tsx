import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
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

function renderTabs(pathname: string) {
  pathnameValue = pathname;
  return render(<SectionTabs />);
}

describe('SectionTabs', () => {
  beforeEach(() => {
    role = 'admin';
  });

  it('admin on /reviews sees moderation tabs with Отзывы active', () => {
    renderTabs('/reviews');

    const links = screen.getAllByRole('link');
    expect(links.map((l) => [l.textContent, l.getAttribute('href')])).toEqual([
      ['Книги', '/books?visibility=pending_review'],
      ['Авторы', '/authors/moderation'],
      ['Жанры', '/moderation/genres'],
      ['Отзывы', '/reviews'],
    ]);
    expect(screen.getByRole('link', { name: 'Отзывы' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Книги' })).not.toHaveAttribute('aria-current');
  });

  it('admin on /books/import sees import tabs with Файл active', () => {
    renderTabs('/books/import');

    const links = screen.getAllByRole('link');
    expect(links.map((l) => l.textContent)).toEqual(['Wikidata', 'Файл', 'Жанры']);
    expect(screen.getByRole('link', { name: 'Файл' })).toHaveAttribute('aria-current', 'page');
  });

  it('moderator on /reviews renders nothing', () => {
    role = 'moderator';
    const { container } = renderTabs('/reviews');
    expect(container).toBeEmptyDOMElement();
  });

  it.each(['/', '/users', '/unknown'])('admin on %s renders nothing', (pathname) => {
    const { container } = renderTabs(pathname);
    expect(container).toBeEmptyDOMElement();
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
    expect(replaceMock).toHaveBeenCalledWith('/books?visibility=pending_review');
  });

  it('moderator on /moderation goes to reviews', () => {
    role = 'moderator';
    pathnameValue = '/moderation';
    render(<SectionIndexRedirect sectionId="moderation" />);
    expect(replaceMock).toHaveBeenCalledWith('/reviews');
  });

  it('moderator on /import is not redirected', () => {
    role = 'moderator';
    pathnameValue = '/import';
    render(<SectionIndexRedirect sectionId="import" />);
    expect(replaceMock).not.toHaveBeenCalled();
  });
});
