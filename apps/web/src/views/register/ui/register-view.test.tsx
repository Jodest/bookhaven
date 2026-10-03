import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { RegisterView } from './register-view';

const pushMock = vi.fn();
const registerMock = vi.fn();
const resendVerificationMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  useSearchParams: () => searchParamsValue,
}));

vi.mock('@bookhaven/api-client', async () => {
  const actual = await vi.importActual<typeof import('@bookhaven/api-client')>(
    '@bookhaven/api-client'
  );
  return {
    ApiError: actual.ApiError,
    isApiError: actual.isApiError,
    api: {
      auth: { resendVerification: (email: string) => resendVerificationMock(email) },
    },
  };
});

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ register: registerMock, error: null, clearError: vi.fn() }),
}));

async function register() {
  fireEvent.change(screen.getByLabelText('Имя'), { target: { value: 'Test User' } });
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'user@example.com' } });
  fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'password123' } });
  fireEvent.click(screen.getByRole('button', { name: 'Зарегистрироваться' }));
  await waitFor(() => expect(registerMock).toHaveBeenCalled());
}

describe('RegisterView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    registerMock.mockReset();
    registerMock.mockResolvedValue({ email: 'user@example.com' });
    resendVerificationMock.mockReset();
    resendVerificationMock.mockResolvedValue({ status: 'sent' });
    pushMock.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('switches to the check-email panel with a login link keeping a safe redirect', async () => {
    searchParamsValue = new URLSearchParams({ redirect: '/books/abc' });
    render(<RegisterView />);

    await register();

    expect(await screen.findByText('Проверьте почту')).toBeInTheDocument();
    expect(screen.getByText('user@example.com')).toBeInTheDocument();
    expect(screen.getByText(/Войти можно после подтверждения/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Войти' })).toHaveAttribute(
      'href',
      '/login?redirect=%2Fbooks%2Fabc'
    );
    expect(screen.queryByRole('link', { name: 'Продолжить' })).toBeNull();
    expect(pushMock).not.toHaveBeenCalled();
  });

  it('login link drops an unsafe redirect', async () => {
    searchParamsValue = new URLSearchParams({ redirect: '//evil.com' });
    render(<RegisterView />);

    await register();

    expect(await screen.findByRole('link', { name: 'Войти' })).toHaveAttribute(
      'href',
      '/login'
    );
  });

  it('resend uses the registered email', async () => {
    render(<RegisterView />);

    await register();
    fireEvent.click(await screen.findByRole('button', { name: 'Отправить письмо ещё раз' }));

    await waitFor(() => expect(resendVerificationMock).toHaveBeenCalledWith('user@example.com'));
  });
});
