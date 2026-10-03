import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LoginView } from './login-view';

const pushMock = vi.fn();
const loginMock = vi.fn();
const resendVerificationMock = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock('@bookhaven/api-client', async () => {
  const actual = await vi.importActual<typeof import('@bookhaven/api-client')>(
    '@bookhaven/api-client'
  );
  return {
    ApiError: actual.ApiError,
    isApiError: actual.isApiError,
    isEmailNotVerifiedError: actual.isEmailNotVerifiedError,
    api: {
      auth: { resendVerification: (email: string) => resendVerificationMock(email) },
    },
  };
});

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ login: loginMock, error: null, clearError: vi.fn() }),
}));

async function notVerifiedError() {
  const { ApiError } = await vi.importActual<typeof import('@bookhaven/api-client')>(
    '@bookhaven/api-client'
  );
  return new ApiError(
    403,
    'Forbidden',
    'email not verified',
    '/api/v1/auth/login',
    'email_not_verified'
  );
}

async function submit(email: string) {
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: email } });
  fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'password123' } });
  fireEvent.click(screen.getByRole('button', { name: 'Войти' }));
  await waitFor(() => expect(loginMock).toHaveBeenCalled());
}

describe('LoginView', () => {
  beforeEach(async () => {
    pushMock.mockReset();
    loginMock.mockReset();
    loginMock.mockRejectedValue(await notVerifiedError());
    resendVerificationMock.mockReset();
    resendVerificationMock.mockResolvedValue({ status: 'sent' });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('shows the not-verified panel with the typed email on 403', async () => {
    render(<LoginView />);

    await submit('anna@example.com');

    const panel = await screen.findByRole('status');
    expect(panel).toHaveTextContent('Email anna@example.com не подтверждён.');
    expect(
      screen.getByRole('button', { name: 'Отправить письмо ещё раз' })
    ).toBeInTheDocument();
    expect(pushMock).not.toHaveBeenCalled();
  });

  it('resends the verification email to that address', async () => {
    render(<LoginView />);

    await submit('anna@example.com');
    fireEvent.click(await screen.findByRole('button', { name: 'Отправить письмо ещё раз' }));

    await waitFor(() => expect(resendVerificationMock).toHaveBeenCalledWith('anna@example.com'));
    expect(
      await screen.findByText(
        'Если этот адрес зарегистрирован и не подтверждён, мы отправили на него новое письмо.'
      )
    ).toBeInTheDocument();
  });

  it('drops the panel when a later attempt fails for another reason', async () => {
    render(<LoginView />);

    await submit('anna@example.com');
    await screen.findByRole('status');

    loginMock.mockReset();
    loginMock.mockRejectedValue(new Error('invalid credentials'));
    await submit('other@example.com');

    await waitFor(() => expect(screen.queryByRole('status')).toBeNull());
  });

  it('shows no panel before a 403', () => {
    render(<LoginView />);

    expect(screen.queryByRole('status')).toBeNull();
  });
});
