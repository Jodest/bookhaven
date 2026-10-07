import { render, screen, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
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
  });

  it('admin on / sees five sections, Обзор active, and the page', () => {
    setUser('admin');
    renderShell('/');

    const links = sidebarLinks();
    expect(links.map((l) => [l.textContent, l.getAttribute('href')])).toEqual([
      ['Обзор', '/'],
      ['Модерация', '/books?visibility=pending_review'],
      ['Каталог', '/books'],
      ['Импорт', '/harvest'],
      ['Пользователи', '/users'],
    ]);
    expect(links[0]).toHaveAttribute('aria-current', 'page');
    expect(links[1]).not.toHaveAttribute('aria-current');
    expect(screen.getByText('page content')).toBeInTheDocument();
  });

  it('admin on /authors/moderation highlights Модерация', () => {
    setUser('admin');
    renderShell('/authors/moderation');

    const active = sidebarLinks().filter((l) => l.getAttribute('aria-current') === 'page');
    expect(active.map((l) => l.textContent)).toEqual(['Модерация']);
  });

  it('admin on /books/123/edit highlights Каталог; /books/import highlights Импорт', () => {
    setUser('admin');
    const { unmount } = renderShell('/books/123/edit');
    expect(screen.getByRole('link', { name: 'Каталог' })).toHaveAttribute('aria-current', 'page');
    unmount();

    renderShell('/books/import');
    expect(screen.getByRole('link', { name: 'Импорт' })).toHaveAttribute('aria-current', 'page');
  });

  it('moderator on /reviews sees only Модерация and the page', () => {
    setUser('moderator');
    renderShell('/reviews');

    const links = sidebarLinks();
    expect(links).toHaveLength(1);
    expect(links[0]).toHaveTextContent('Модерация');
    expect(links[0]).toHaveAttribute('href', '/reviews');
    expect(screen.getByText('page content')).toBeInTheDocument();
  });

  it('moderator on / is redirected to /reviews without AccessDenied', () => {
    setUser('moderator');
    renderShell('/');

    expect(replaceMock).toHaveBeenCalledWith('/reviews');
    expect(screen.queryByText('page content')).not.toBeInTheDocument();
    expect(screen.queryByText('Доступ запрещен')).not.toBeInTheDocument();
  });

  it.each(['/books', '/users', '/unknown', '/import'])(
    'moderator on %s sees admin-only AccessDenied with a link home',
    (pathname) => {
      setUser('moderator');
      renderShell(pathname);

      expect(screen.getByText(ADMIN_ONLY_MESSAGE)).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'К отзывам' })).toHaveAttribute('href', '/reviews');
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
    renderShell('/books');

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
    authState.forbiddenPath = '/reviews';
    renderShell('/reviews');

    expect(
      screen.getByText(
        'Сервер отклонил запрос: у вас недостаточно прав для выполнения этого действия.'
      )
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'К отзывам' })).toHaveAttribute('href', '/reviews');
  });
});
