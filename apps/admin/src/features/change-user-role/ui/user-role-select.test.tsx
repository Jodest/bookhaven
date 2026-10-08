import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type { AdminUser } from '@bookhaven/api-client';
import { UserRoleSelect } from './user-role-select';

const changeRoleMock = vi.fn();

vi.mock('@/entities/user', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/user')>()),
  adminUsersApi: {
    list: vi.fn(),
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
    email_verified_at: null,
    created_at: '2026-09-01T10:00:00Z',
    ...overrides,
  };
}

function renderSelect(props: Partial<React.ComponentProps<typeof UserRoleSelect>> = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const user = props.user ?? makeUser();
  render(
    <QueryClientProvider client={queryClient}>
      <UserRoleSelect user={user} {...props} />
    </QueryClientProvider>
  );
  return screen.getByLabelText(`Роль пользователя ${user.email}`) as HTMLSelectElement;
}

describe('UserRoleSelect', () => {
  const confirmSpy = vi.spyOn(window, 'confirm');

  beforeEach(() => {
    changeRoleMock.mockReset();
    confirmSpy.mockReset();
  });

  afterEach(() => {
    confirmSpy.mockReset();
  });

  it('does nothing and reverts the value when the confirm is cancelled', () => {
    confirmSpy.mockReturnValue(false);
    const select = renderSelect();

    fireEvent.change(select, { target: { value: 'moderator' } });

    expect(confirmSpy).toHaveBeenCalledWith('Изменить роль пользователя «Анна»: Читатель → Модератор?');
    expect(changeRoleMock).not.toHaveBeenCalled();
    expect(select.value).toBe('user');
  });

  it('calls changeRole(id, role) after confirmation and reports success', async () => {
    confirmSpy.mockReturnValue(true);
    changeRoleMock.mockResolvedValue(makeUser({ role: 'moderator' }));
    const onChanged = vi.fn();
    const select = renderSelect({ onChanged });

    fireEvent.change(select, { target: { value: 'moderator' } });

    await waitFor(() => expect(onChanged).toHaveBeenCalled());
    expect(changeRoleMock).toHaveBeenCalledWith('u1', 'moderator');
    expect(onChanged.mock.calls[0][1]).toBe('user');
    expect(onChanged.mock.calls[0][2]).toBe('moderator');
    expect(select.value).toBe('moderator');
  });

  it('respects the disabled prop', () => {
    const select = renderSelect({ disabled: true });
    expect(select).toBeDisabled();
  });

  it('surfaces last_admin via onError and reverts the value', async () => {
    confirmSpy.mockReturnValue(true);
    changeRoleMock.mockRejectedValue(new ApiError(409, 'Conflict', 'x', '', 'last_admin'));
    const onError = vi.fn();
    const select = renderSelect({ user: makeUser({ role: 'admin' }), onError });

    fireEvent.change(select, { target: { value: 'user' } });

    await waitFor(() =>
      expect(onError).toHaveBeenCalledWith('Нельзя снять роль с последнего администратора.')
    );
    await waitFor(() => expect(select.value).toBe('admin'));
  });
});
