import { render, screen, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { NavCounters } from '@/features/nav-counters';
import { Shell } from './shell';

const pushMock = vi.fn();
const replaceMock = vi.fn();
let pathnameValue = '/';

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock, replace: replaceMock }),
  usePathname: () => pathnameValue,
}));

interface MockAuthState {
  user: { id: string; email: string; role: string } | null;
  isLoading: boolean;
  forbiddenPath: string | null;
}

let authState: MockAuthState;
const clearForbiddenPathMock = vi.fn();

vi.mock('@/entities/session', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/session')>()),
  useAuthStore: () => ({
    ...authState,
    logout: vi.fn(),
    clearForbiddenPath: clearForbiddenPathMock,
  }),
}));

let countersValue: NavCounters = { sections: {}, tabs: {} };

vi.mock('@/features/nav-counters', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/features/nav-counters')>()),
  useNavCounters: () => countersValue,
}));

function setUser(role: string | null) {
  authState.user = role ? { id: 'u1', email: 'staff@example.com', role } : null;
}

function renderShell(pathname: string) {
  pathnameValue = pathname;
  return render(
    <Shell>
      <div>page content</div>
    </Shell>
  );
}

function sidebarLinks() {
  return within(screen.getByRole('navigation')).getAllByRole('link');
}

const ADMIN_ONLY_MESSAGE = 'Этот раздел доступен только администраторам.';

describe('Shell', () => {
  beforeEach(() => {
    pushMock.mockReset();
    replaceMock.mockReset();
    clearForbiddenPathMock.mockReset();
    authState = { user: null, isLoading: false, forbiddenPath: null };
    countersValue = { sections: {}, tabs: {} };
  });

  it('admin on / sees seven sections, Обзор active, and the page', () => {
    setUser('admin');
    renderShell('/');

    const links = sidebarLinks();
    expect(links.map((l) => [l.textContent, l.getAttribute('href')])).toEqual([
      ['Обзор', '/'],
      ['Модерация', '/moderation/books'],
      ['Каталог', '/catalog/books'],
      ['Подборки', '/collections'],
      ['Импорт', '/import/books/wikidata'],
      ['Пользователи', '/users'],
      ['Справка', '/help'],
    ]);
    expect(links[0]).toHaveAttribute('aria-current', 'page');
    expect(links[1]).not.toHaveAttribute('aria-current');
    expect(screen.getByText('page content')).toBeInTheDocument();
  });

  it.each(['/moderation/authors', '/moderation/books'])('admin on %s highlights Модерация', (pathname) => {
    setUser('admin');
    renderShell(pathname);

    const active = sidebarLinks().filter((l) => l.getAttribute('aria-current') === 'page');
    expect(active.map((l) => l.textContent)).toEqual(['Модерация']);
  });

  it('admin on /catalog/books/123/edit highlights Каталог; /import/authors highlights Импорт', () => {
    setUser('admin');
    const { unmount } = renderShell('/catalog/books/123/edit');
    expect(screen.getByRole('link', { name: 'Каталог' })).toHaveAttribute('aria-current', 'page');
    unmount();

    renderShell('/import/authors');
    expect(screen.getByRole('link', { name: 'Импорт' })).toHaveAttribute('aria-current', 'page');
  });

  it.each(['/import/jobs', '/import/books/file', '/import/genres/wikidata'])(
    'admin on %s highlights Импорт',
    (pathname) => {
      setUser('admin');
      renderShell(pathname);

      const active = sidebarLinks().filter((l) => l.getAttribute('aria-current') === 'page');
      expect(active.map((l) => l.textContent)).toEqual(['Импорт']);
    }
  );

  it('moderator on /moderation/reviews sees only Модерация, Справка and the page', () => {
    setUser('moderator');
    renderShell('/moderation/reviews');

    const links = sidebarLinks();
    expect(links.map((l) => [l.textContent, l.getAttribute('href')])).toEqual([
      ['Модерация', '/moderation/reviews'],
      ['Справка', '/help'],
    ]);
    expect(screen.getByText('page content')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Подборки' })).not.toBeInTheDocument();
  });

  it('moderator on / is redirected to /moderation/reviews without AccessDenied', () => {
    setUser('moderator');
    renderShell('/');

    expect(replaceMock).toHaveBeenCalledWith('/moderation/reviews');
    expect(screen.queryByText('page content')).not.toBeInTheDocument();
    expect(screen.queryByText('Доступ запрещен')).not.toBeInTheDocument();
  });

  it.each(['/catalog/books', '/moderation/books', '/users', '/unknown', '/import', '/import/books', '/import/books/file', '/catalog'])(
    'moderator on %s sees admin-only AccessDenied with a link home',
    (pathname) => {
      setUser('moderator');
      renderShell(pathname);

      expect(screen.getByText(ADMIN_ONLY_MESSAGE)).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'К отзывам' })).toHaveAttribute(
        'href',
        '/moderation/reviews'
      );
      expect(screen.queryByText('page content')).not.toBeInTheDocument();
      expect(replaceMock).not.toHaveBeenCalled();
    }
  );

  it('moderator on bare /moderation is allowed (index page redirects)', () => {
    setUser('moderator');
    renderShell('/moderation');

    expect(screen.queryByText(ADMIN_ONLY_MESSAGE)).not.toBeInTheDocument();
    expect(screen.getByText('page content')).toBeInTheDocument();
  });

  it('admin on an unknown path sees the page', () => {
    setUser('admin');
    renderShell('/unknown');

    expect(screen.getByText('page content')).toBeInTheDocument();
  });

  it('non-staff user sees the existing AccessDenied', () => {
    setUser('user');
    renderShell('/');

    expect(
      screen.getByText('У вас недостаточно прав для доступа к панели администратора.')
    ).toBeInTheDocument();
    expect(screen.queryByText('page content')).not.toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it('redirects to login when there is no user', () => {
    setUser(null);
    renderShell('/catalog/books');

    expect(pushMock).toHaveBeenCalledTimes(1);
    const url = new URL(pushMock.mock.calls[0][0] as string);
    expect(url.pathname).toBe('/login');
    expect(url.searchParams.get('redirect')).not.toBeNull();
  });

  it('shows the loading state', () => {
    authState.isLoading = true;
    renderShell('/');

    expect(screen.getByText('Загрузка...')).toBeInTheDocument();
    expect(pushMock).not.toHaveBeenCalled();
  });

  it('server-forbidden screen links the moderator home', () => {
    setUser('moderator');
    authState.forbiddenPath = '/moderation/reviews';
    renderShell('/moderation/reviews');

    expect(
      screen.getByText(
        'Сервер отклонил запрос: у вас недостаточно прав для выполнения этого действия.'
      )
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'К отзывам' })).toHaveAttribute(
      'href',
      '/moderation/reviews'
    );
  });

  describe('sidebar counters', () => {
    it('admin sees moderation and import badges in the link names', () => {
      setUser('admin');
      countersValue = { sections: { moderation: 11, import: 2 }, tabs: {} };
      renderShell('/');

      expect(
        screen.getByRole('link', { name: 'Модерация, ожидают проверки: 11' })
      ).toHaveAttribute('href', '/moderation/books');
      expect(screen.getByRole('link', { name: 'Импорт, задач в работе: 2' })).toHaveAttribute(
        'href',
        '/import/books/wikidata'
      );
      expect(screen.getByRole('link', { name: 'Каталог' })).toBeInTheDocument();
    });

    it('a zero count renders no badge', () => {
      setUser('admin');
      countersValue = { sections: { moderation: 0 }, tabs: {} };
      renderShell('/');

      const link = screen.getByRole('link', { name: 'Модерация' });
      expect(link).toHaveAccessibleName('Модерация');
      expect(link.textContent).toBe('Модерация');
    });

    it('caps the visual value at 99+', () => {
      setUser('admin');
      countersValue = { sections: { moderation: 150 }, tabs: {} };
      renderShell('/');

      const link = screen.getByRole('link', { name: 'Модерация, ожидают проверки: 150' });
      expect(link.querySelector('[aria-hidden="true"]')).toHaveTextContent('99+');
    });

    it('moderator sidebar link name is exactly Модерация', () => {
      setUser('moderator');
      renderShell('/moderation/reviews');

      const links = sidebarLinks();
      expect(links).toHaveLength(2);
      expect(links[0]).toHaveAccessibleName('Модерация');
    });
  });
});

describe('Shell help', () => {
  beforeEach(() => {
    authState = { user: null, isLoading: false, forbiddenPath: null };
    countersValue = { sections: {}, tabs: {} };
  });

  it('admin on /import/jobs sees the help link to the import jobs chapter', () => {
    setUser('admin');
    renderShell('/import/jobs');
    expect(screen.getByRole('link', { name: 'Справка по разделу' })).toHaveAttribute('href', '/help/import-jobs');
  });

  it('moderator on /moderation/reviews sees the moderation chapter link', () => {
    setUser('moderator');
    renderShell('/moderation/reviews');
    expect(screen.getByRole('link', { name: 'Справка по разделу' })).toHaveAttribute('href', '/help/moderation');
  });

  it('shows no help link inside the help', () => {
    setUser('admin');
    renderShell('/help/moderation');
    expect(screen.queryByRole('link', { name: 'Справка по разделу' })).not.toBeInTheDocument();
    expect(screen.getByText('page content')).toBeInTheDocument();
  });

  it('moderator on an admin-only chapter gets AccessDenied with the reviews button', () => {
    setUser('moderator');
    renderShell('/help/adding-content');
    expect(screen.getByText(ADMIN_ONLY_MESSAGE)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'К отзывам' })).toBeInTheDocument();
    expect(screen.queryByText('page content')).not.toBeInTheDocument();
  });

  it.each(['/help', '/help/moderation'])('moderator on %s is allowed', (pathname) => {
    setUser('moderator');
    renderShell(pathname);
    expect(screen.getByText('page content')).toBeInTheDocument();
    expect(screen.queryByText(ADMIN_ONLY_MESSAGE)).not.toBeInTheDocument();
  });
});
