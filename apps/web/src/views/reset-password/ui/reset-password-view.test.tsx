import { ApiError } from '@bookhaven/api-client';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ResetPasswordView } from './reset-password-view';

const resetPasswordMock = vi.fn();
const logoutMock = vi.fn();
let isAuthenticated = false;
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useSearchParams: () => searchParamsValue,
}));

vi.mock('@/entities/session', () => ({
  useAuthStore: {
    getState: () => ({ isAuthenticated, logout: logoutMock }),
  },
}));

vi.mock('@bookhaven/api-client', async () => {
  const actual = await vi.importActual<typeof import('@bookhaven/api-client')>(
    '@bookhaven/api-client'
  );
  return {
    ApiError: actual.ApiError,
    isApiError: actual.isApiError,
    api: {
      auth: {
        resetPassword: (token: string, password: string) => resetPasswordMock(token, password),
      },
    },
  };
});

const TOKEN = 'a'.repeat(64);
const SAVE = { name: 'Сохранить пароль' };
const REQUEST_NEW = { name: 'Запросить новую ссылку' };

function fillAndSubmit(password: string, confirm = password) {
  fireEvent.change(screen.getByLabelText('Новый пароль'), { target: { value: password } });
  fireEvent.change(screen.getByLabelText('Повторите пароль'), { target: { value: confirm } });
  fireEvent.click(screen.getByRole('button', SAVE));
}

function apiError(code: string) {
  return new ApiError(400, 'Bad Request', 'x', '/api/v1/auth/reset-password', code);
}

describe('ResetPasswordView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams({ token: TOKEN });
    isAuthenticated = false;
    resetPasswordMock.mockReset();
    resetPasswordMock.mockResolvedValue({ status: 'password_reset' });
    logoutMock.mockReset();
    logoutMock.mockRejectedValue(new ApiError(401, 'Unauthorized'));
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('missing token -> missing state with a link to /forgot-password', () => {
    searchParamsValue = new URLSearchParams({ token: '  ' });
    render(<ResetPasswordView />);

    expect(screen.getByText('Ссылка неполная')).toBeInTheDocument();
    expect(screen.getByRole('link', REQUEST_NEW)).toHaveAttribute('href', '/forgot-password');
    expect(screen.queryByRole('button', SAVE)).toBeNull();
  });

  it('does not call the API on page load', () => {
    render(<ResetPasswordView />);

    expect(screen.getByText('Новый пароль', { selector: 'h1' })).toBeInTheDocument();
    expect(resetPasswordMock).not.toHaveBeenCalled();
  });

  it('short password -> field error, no API call', async () => {
    render(<ResetPasswordView />);
    fillAndSubmit('short');

    expect(await screen.findByText('Пароль должен быть не короче 8 символов')).toBeInTheDocument();
    expect(resetPasswordMock).not.toHaveBeenCalled();
  });

  it('mismatched passwords -> field error, no API call', async () => {
    render(<ResetPasswordView />);
    fillAndSubmit('new-password-456', 'other-password-456');

    expect(await screen.findByText('Пароли не совпадают')).toBeInTheDocument();
    expect(resetPasswordMock).not.toHaveBeenCalled();
  });

  it('success -> "Пароль изменён" with a login link; no logout when logged out', async () => {
    render(<ResetPasswordView />);
    fillAndSubmit('new-password-456');

    expect(await screen.findByText('Пароль изменён')).toBeInTheDocument();
    expect(resetPasswordMock).toHaveBeenCalledWith(TOKEN, 'new-password-456');
    expect(screen.getByRole('link', { name: 'Войти' })).toHaveAttribute('href', '/login');
    expect(logoutMock).not.toHaveBeenCalled();
  });

  it('success while logged in -> drops the session, ignoring the logout error', async () => {
    isAuthenticated = true;
    render(<ResetPasswordView />);
    fillAndSubmit('new-password-456');

    expect(await screen.findByText('Пароль изменён')).toBeInTheDocument();
    expect(logoutMock).toHaveBeenCalledTimes(1);
  });

  it.each([
    ['token_expired', 'Срок действия ссылки истёк'],
    ['invalid_token', 'Ссылка недействительна'],
    ['token_used', 'Ссылка уже использована'],
  ])('%s -> its copy and a link to /forgot-password', async (code, title) => {
    resetPasswordMock.mockRejectedValue(apiError(code));
    render(<ResetPasswordView />);
    fillAndSubmit('new-password-456');

    expect(await screen.findByText(title)).toBeInTheDocument();
    expect(screen.getByRole('link', REQUEST_NEW)).toHaveAttribute('href', '/forgot-password');
    if (code === 'token_used') {
      expect(screen.getByRole('link', { name: 'Войти' })).toHaveAttribute('href', '/login');
    } else {
      expect(screen.queryByRole('link', { name: 'Войти' })).toBeNull();
    }
    expect(screen.queryByRole('button', SAVE)).toBeNull();
  });

  it('invalid_password -> field error, form stays', async () => {
    resetPasswordMock.mockRejectedValue(apiError('invalid_password'));
    render(<ResetPasswordView />);
    fillAndSubmit('new-password-456');

    expect(await screen.findByText('Пароль должен быть не короче 8 символов')).toBeInTheDocument();
    expect(screen.getByRole('button', SAVE)).toBeInTheDocument();
  });

  it('500 / network -> inline error, form stays', async () => {
    resetPasswordMock.mockRejectedValue(new TypeError('Failed to fetch'));
    render(<ResetPasswordView />);
    fillAndSubmit('new-password-456');

    expect(
      await screen.findByText('Не удалось сменить пароль. Попробуйте снова.')
    ).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', SAVE)).toBeEnabled());
  });
});
