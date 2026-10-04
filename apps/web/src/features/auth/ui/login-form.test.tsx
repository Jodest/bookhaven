import { ApiError } from '@bookhaven/api-client';
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

  it('links to /forgot-password', () => {
    render(<LoginForm />);

    expect(screen.getByRole('link', { name: 'Забыли пароль?' })).toHaveAttribute(
      'href',
      '/forgot-password'
    );
  });

  it('ignores a redirect back to a guest-only auth page', async () => {
    searchParamsValue = new URLSearchParams({ redirect: '/register' });
    render(<LoginForm />);

    await fillAndSubmit();

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/profile'));
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

  describe('email not verified (403)', () => {
    const notVerified = () =>
      new ApiError(
        403,
        'Forbidden',
        'email not verified',
        '/api/v1/auth/login',
        'email_not_verified'
      );

    it('calls onEmailNotVerified with the typed email and does not navigate', async () => {
      loginMock.mockRejectedValue(notVerified());
      const onEmailNotVerified = vi.fn();
      render(<LoginForm onEmailNotVerified={onEmailNotVerified} />);

      await fillAndSubmit();

      await waitFor(() => expect(onEmailNotVerified).toHaveBeenCalledWith('user@example.com'));
      expect(pushMock).not.toHaveBeenCalled();
      expect(screen.queryByText('Email не подтверждён. Проверьте почту.')).toBeNull();
    });

    it('shows an inline hint when no callback is given', async () => {
      loginMock.mockRejectedValue(notVerified());
      render(<LoginForm />);

      await fillAndSubmit();

      expect(
        await screen.findByText('Email не подтверждён. Проверьте почту.')
      ).toBeInTheDocument();
      expect(pushMock).not.toHaveBeenCalled();
    });

    it('does not call onEmailNotVerified for other errors', async () => {
      loginMock.mockRejectedValue(new ApiError(401, 'Unauthorized', 'invalid credentials'));
      const onEmailNotVerified = vi.fn();
      render(<LoginForm onEmailNotVerified={onEmailNotVerified} />);

      await fillAndSubmit();

      await waitFor(() => expect(loginMock).toHaveBeenCalled());
      expect(onEmailNotVerified).not.toHaveBeenCalled();
    });
  });
});
