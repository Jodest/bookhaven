import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LoginForm } from './login-form';

const pushMock = vi.fn();
const loginMock = vi.fn();
const clearErrorMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  useSearchParams: () => searchParamsValue,
}));

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({
    login: loginMock,
    error: null,
    clearError: clearErrorMock,
  }),
}));

async function fillAndSubmit() {
  fireEvent.change(screen.getByLabelText('Email'), {
    target: { value: 'user@example.com' },
  });
  fireEvent.change(screen.getByLabelText('Пароль'), {
    target: { value: 'password123' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Войти' }));

  await waitFor(() => expect(loginMock).toHaveBeenCalled());
}

describe('LoginForm', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    loginMock.mockReset();
    pushMock.mockReset();
    clearErrorMock.mockReset();
    loginMock.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('redirects to the safe `redirect` param on success', async () => {
    searchParamsValue = new URLSearchParams({ redirect: '/books/abc' });
    render(<LoginForm />);

    await fillAndSubmit();

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/books/abc'));
  });

  it('falls back to /profile when there is no redirect param', async () => {
    render(<LoginForm />);

    await fillAndSubmit();

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/profile'));
  });

  it('falls back to /profile when the redirect param is unsafe', async () => {
    searchParamsValue = new URLSearchParams({ redirect: '//evil.com' });
    render(<LoginForm />);

    await fillAndSubmit();

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/profile'));
  });
});
