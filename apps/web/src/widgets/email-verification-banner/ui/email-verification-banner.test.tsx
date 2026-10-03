import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { EmailVerificationBanner } from './email-verification-banner';

const resendMock = vi.fn();
const fetchUserMock = vi.fn();
let pathnameValue = '/books';

type AuthSnapshot = {
  user: { id: string; email: string; emailVerified: boolean } | null;
  isAuthenticated: boolean;
  isLoading: boolean;
};
let authState: AuthSnapshot;

vi.mock('next/navigation', () => ({
  usePathname: () => pathnameValue,
}));

vi.mock('@bookhaven/api-client', async () => {
  const actual = await vi.importActual<typeof import('@bookhaven/api-client')>(
    '@bookhaven/api-client'
  );
  return {
    ApiError: actual.ApiError,
    isApiError: actual.isApiError,
    api: { auth: { resendVerification: () => resendMock() } },
  };
});

vi.mock('@/entities/session', () => ({
  useAuthStore: Object.assign(() => authState, {
    getState: () => ({ fetchUser: fetchUserMock }),
  }),
}));

async function apiError(status: number, code: string, retryAfterSeconds?: number) {
  const { ApiError } = await vi.importActual<typeof import('@bookhaven/api-client')>(
    '@bookhaven/api-client'
  );
  return new ApiError(
    status,
    '',
    code,
    '/api/v1/auth/resend-verification',
    code,
    retryAfterSeconds
  );
}

const unverified = (id = 'u1'): AuthSnapshot => ({
  user: { id, email: `${id}@example.com`, emailVerified: false },
  isAuthenticated: true,
  isLoading: false,
});

const REGION = { name: 'Подтверждение email' };
const RESEND = { name: 'Отправить письмо ещё раз' };

describe('EmailVerificationBanner', () => {
  beforeEach(() => {
    pathnameValue = '/books';
    authState = unverified();
    resendMock.mockReset();
    fetchUserMock.mockReset();
    fetchUserMock.mockResolvedValue(undefined);
    window.sessionStorage.clear();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('is hidden while the session is loading', () => {
    authState = { user: null, isAuthenticated: false, isLoading: true };
    render(<EmailVerificationBanner />);
    expect(screen.queryByRole('region', REGION)).toBeNull();
  });

  it('is hidden for guests', () => {
    authState = { user: null, isAuthenticated: false, isLoading: false };
    render(<EmailVerificationBanner />);
    expect(screen.queryByRole('region', REGION)).toBeNull();
  });

  it('is hidden for verified users', () => {
    authState = {
      user: { id: 'u1', email: 'u1@example.com', emailVerified: true },
      isAuthenticated: true,
      isLoading: false,
    };
    render(<EmailVerificationBanner />);
    expect(screen.queryByRole('region', REGION)).toBeNull();
  });

  it.each(['/verify-email', '/register'])('is hidden on %s', (path) => {
    pathnameValue = path;
    render(<EmailVerificationBanner />);
    expect(screen.queryByRole('region', REGION)).toBeNull();
  });

  it('is shown with the email for an unverified user', async () => {
    render(<EmailVerificationBanner />);
    expect(await screen.findByRole('region', REGION)).toBeInTheDocument();
    expect(screen.getByText('u1@example.com')).toBeInTheDocument();
  });

  it('shows the success message after a 202 and disables the button', async () => {
    resendMock.mockResolvedValue({ status: 'sent' });
    render(<EmailVerificationBanner />);

    fireEvent.click(await screen.findByRole('button', RESEND));

    expect(await screen.findByText('Письмо отправлено. Проверьте почту.')).toBeInTheDocument();
    expect(screen.getByRole('button', RESEND)).toBeDisabled();
  });

  it('shows the rate-limit wait and disables the button on 429', async () => {
    resendMock.mockRejectedValue(await apiError(429, 'rate_limited', 90));
    render(<EmailVerificationBanner />);

    fireEvent.click(await screen.findByRole('button', RESEND));

    expect(
      await screen.findByText('Слишком много запросов. Попробуйте снова через 2 мин.')
    ).toBeInTheDocument();
    expect(screen.getByRole('button', RESEND)).toBeDisabled();
  });

  it('re-fetches the user on 409 already_verified', async () => {
    resendMock.mockRejectedValue(await apiError(409, 'already_verified'));
    render(<EmailVerificationBanner />);

    fireEvent.click(await screen.findByRole('button', RESEND));

    await waitFor(() => expect(fetchUserMock).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('Email уже подтверждён.')).toBeInTheDocument();
  });

  it('re-enables the button after a generic error', async () => {
    resendMock.mockRejectedValue(await apiError(500, 'email_send_failed'));
    render(<EmailVerificationBanner />);

    fireEvent.click(await screen.findByRole('button', RESEND));

    expect(
      await screen.findByText('Не удалось отправить письмо. Попробуйте позже.')
    ).toBeInTheDocument();
    expect(screen.getByRole('button', RESEND)).toBeEnabled();
  });

  it('dismiss hides it for this user only (sessionStorage per user id)', async () => {
    const first = render(<EmailVerificationBanner />);

    fireEvent.click(await screen.findByRole('button', { name: 'Скрыть' }));

    expect(screen.queryByRole('region', REGION)).toBeNull();
    expect(window.sessionStorage.getItem('bookhaven:verify-banner-dismissed:u1')).toBe('1');
    first.unmount();

    // Same user, fresh mount in the same tab session: stays hidden.
    const second = render(<EmailVerificationBanner />);
    await waitFor(() => expect(screen.queryByRole('region', REGION)).toBeNull());
    second.unmount();

    // A different user in the same tab: shown.
    authState = unverified('u2');
    render(<EmailVerificationBanner />);
    expect(await screen.findByRole('region', REGION)).toBeInTheDocument();
    expect(screen.getByText('u2@example.com')).toBeInTheDocument();
  });
});
