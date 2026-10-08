import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminUser } from '@bookhaven/api-client';
import { useAuthStore } from '@/entities/session';
import { UsersView } from './users-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/users',
  useSearchParams: () => searchParamsValue,
}));

const listMock = vi.fn();
const changeRoleMock = vi.fn();

vi.mock('@/entities/user', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/user')>()),
  adminUsersApi: {
    list: (...args: unknown[]) => listMock(...args),
    changeRole: (...args: unknown[]) => changeRoleMock(...args),
  },
}));

function makeUser(overrides: Partial<AdminUser> = {}): AdminUser {
  return {
    id: 'u1',
    email: 'ann@example.com',
    name: 'Анна',
    avatar_url: null,
    role: 'user',
    profile_type: 'reader',
    email_verified_at: '2026-09-02T10:00:00Z',
    created_at: '2026-09-01T10:00:00Z',
    ...overrides,
  };
}

function page(items: AdminUser[]) {
  return { items, total: items.length, page: 1, limit: 20 };
}

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  render(
    <QueryClientProvider client={queryClient}>
      <UsersView />
    </QueryClientProvider>
  );
  return { invalidateSpy };
}

function rowOf(email: string): HTMLElement {
  return screen.getByText(email).closest('tr') as HTMLElement;
}

describe('UsersView', () => {
  const confirmSpy = vi.spyOn(window, 'confirm');

  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    listMock.mockReset();
    changeRoleMock.mockReset();
    confirmSpy.mockReset();
    useAuthStore.setState({
      user: {
        id: 'me',
        email: 'me@example.com',
        role: 'admin',
        profileType: 'reader',
      } as never,
    });
  });

  afterEach(() => {
    useAuthStore.setState({ user: null });
  });

  it('renders rows with name fallback, role label and verification', async () => {
    listMock.mockResolvedValue(
      page([
        makeUser(),
        makeUser({ id: 'u2', email: 'bob@example.com', name: null, role: 'moderator', email_verified_at: null }),
      ])
    );
    renderView();

    expect(await screen.findByText('ann@example.com')).toBeInTheDocument();
    const ann = within(rowOf('ann@example.com'));
    expect(ann.getByText('Анна')).toBeInTheDocument();
    expect(ann.getByText('Читатель', { selector: 'td' })).toBeInTheDocument();
    expect(ann.getByText('Да')).toBeInTheDocument();
    expect(ann.getByText('01.09.2026')).toBeInTheDocument();

    const bob = within(rowOf('bob@example.com'));
    expect(bob.getByText('—')).toBeInTheDocument();
    expect(bob.getByText('Нет')).toBeInTheDocument();
    expect(listMock).toHaveBeenCalledWith({ search: undefined, role: undefined, page: 1, limit: 20 });
  });

  it('disables the role select on the own row with an "Это вы" hint', async () => {
    listMock.mockResolvedValue(
      page([makeUser({ id: 'me', email: 'me@example.com', role: 'admin' }), makeUser()])
    );
    renderView();

    await screen.findByText('me@example.com');
    expect(screen.getByLabelText('Роль пользователя me@example.com')).toBeDisabled();
    expect(within(rowOf('me@example.com')).getByText('Это вы')).toBeInTheDocument();
    expect(screen.getByLabelText('Роль пользователя ann@example.com')).not.toBeDisabled();
    expect(within(rowOf('ann@example.com')).queryByText('Это вы')).not.toBeInTheDocument();
  });

  it('reads search/role/page from the URL and writes them back with a page reset', async () => {
    searchParamsValue = new URLSearchParams('search=ann&role=moderator&page=3');
    listMock.mockResolvedValue(page([makeUser({ role: 'moderator' })]));
    renderView();

    await waitFor(() =>
      expect(listMock).toHaveBeenCalledWith({ search: 'ann', role: 'moderator', page: 3, limit: 20 })
    );

    fireEvent.change(screen.getByLabelText('Роль'), { target: { value: 'admin' } });
    expect(pushMock).toHaveBeenLastCalledWith('/users?search=ann&role=admin&page=1');

    fireEvent.change(screen.getByPlaceholderText('Поиск по email или имени'), {
      target: { value: 'bob' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));
    expect(pushMock).toHaveBeenLastCalledWith('/users?search=bob&role=moderator&page=1');

    fireEvent.change(screen.getByLabelText('Роль'), { target: { value: '' } });
    expect(pushMock).toHaveBeenLastCalledWith('/users?search=ann&page=1');
  });

  it('shows the empty state', async () => {
    listMock.mockResolvedValue(page([]));
    renderView();
    expect(await screen.findByText('Пользователи не найдены.')).toBeInTheDocument();
  });

  it('shows the load error', async () => {
    listMock.mockRejectedValue(new Error('boom'));
    renderView();
    expect(await screen.findByText('Не удалось загрузить пользователей.')).toBeInTheDocument();
  });

  it('changes a role, shows a success banner and invalidates the users queries', async () => {
    listMock.mockResolvedValue(page([makeUser()]));
    changeRoleMock.mockResolvedValue(makeUser({ role: 'contributor' }));
    confirmSpy.mockReturnValue(true);
    const { invalidateSpy } = renderView();

    await screen.findByText('ann@example.com');
    fireEvent.change(screen.getByLabelText('Роль пользователя ann@example.com'), {
      target: { value: 'contributor' },
    });

    expect(
      await screen.findByText('Роль пользователя «Анна» изменена: Читатель → Автор контента.')
    ).toBeInTheDocument();
    expect(changeRoleMock).toHaveBeenCalledWith('u1', 'contributor');
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'users'] });
  });

  it('shows the mapped error banner when the change fails', async () => {
    const { ApiError } = await import('@bookhaven/api-client');
    listMock.mockResolvedValue(page([makeUser({ role: 'admin' })]));
    changeRoleMock.mockRejectedValue(new ApiError(409, 'Conflict', 'x', '', 'cannot_change_own_role'));
    confirmSpy.mockReturnValue(true);
    renderView();

    await screen.findByText('ann@example.com');
    fireEvent.change(screen.getByLabelText('Роль пользователя ann@example.com'), {
      target: { value: 'user' },
    });
    expect(await screen.findByText('Нельзя изменить собственную роль.')).toBeInTheDocument();
  });
});
