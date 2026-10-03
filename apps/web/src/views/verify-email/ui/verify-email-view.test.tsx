import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { VerifyEmailView } from './verify-email-view';

const verifyEmailMock = vi.fn();
const fetchUserMock = vi.fn();
let searchParamsValue = new URLSearchParams();

type AuthSnapshot = {
  user: { id: string; email: string; emailVerified: boolean } | null;
  isAuthenticated: boolean;
  isLoading: boolean;
};
let authState: AuthSnapshot = { user: null, isAuthenticated: false, isLoading: false };

vi.mock('next/navigation', () => ({
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
      auth: {
        verifyEmail: (token: string) => verifyEmailMock(token),
        resendVerification: vi.fn(),
      },
    },
  };
});

vi.mock('@/entities/session', () => ({
  useAuthStore: Object.assign(() => authState, {
    getState: () => ({ fetchUser: fetchUserMock }),
  }),
}));

async function expiredTokenError() {
  const { ApiError } = await vi.importActual<typeof import('@bookhaven/api-client')>(
    '@bookhaven/api-client'
  );
  return new ApiError(400, 'Bad Request', 'expired', '/api/v1/auth/verify-email', 'token_expired');
}

const RESEND = { name: 'Отправить письмо ещё раз' };

describe('VerifyEmailView', () => {
  beforeEach(async () => {
    searchParamsValue = new URLSearchParams({ token: 'abc' });
    verifyEmailMock.mockReset();
    verifyEmailMock.mockRejectedValue(await expiredTokenError());
    fetchUserMock.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('token_expired + logged-in unverified user -> resend button', async () => {
    authState = {
      user: { id: 'u1', email: 'a@example.com', emailVerified: false },
      isAuthenticated: true,
      isLoading: false,
    };
    render(<VerifyEmailView />);

    expect(await screen.findByText('Срок действия ссылки истёк')).toBeInTheDocument();
    expect(screen.getByRole('button', RESEND)).toBeInTheDocument();
    expect(verifyEmailMock).toHaveBeenCalledWith('abc');
  });

  it('token_expired + logged out -> login link', async () => {
    authState = { user: null, isAuthenticated: false, isLoading: false };
    render(<VerifyEmailView />);

    expect(await screen.findByText('Срок действия ссылки истёк')).toBeInTheDocument();
    expect(screen.getByText('Войдите, чтобы получить новое письмо.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Войти' })).toHaveAttribute('href', '/login');
    expect(screen.queryByRole('button', RESEND)).toBeNull();
  });

  it('token_expired + logged-in verified user -> no resend button', async () => {
    authState = {
      user: { id: 'u1', email: 'a@example.com', emailVerified: true },
      isAuthenticated: true,
      isLoading: false,
    };
    render(<VerifyEmailView />);

    expect(await screen.findByText('Срок действия ссылки истёк')).toBeInTheDocument();
    expect(screen.getByText('Ваш email уже подтверждён.')).toBeInTheDocument();
    expect(screen.queryByRole('button', RESEND)).toBeNull();
  });

  it('blank token -> no API call and missing-token copy', () => {
    searchParamsValue = new URLSearchParams({ token: '   ' });
    authState = { user: null, isAuthenticated: false, isLoading: false };
    render(<VerifyEmailView />);

    expect(screen.getByText('Ссылка неполная')).toBeInTheDocument();
    expect(verifyEmailMock).not.toHaveBeenCalled();
  });
});
