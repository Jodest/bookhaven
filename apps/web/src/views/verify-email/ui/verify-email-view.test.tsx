import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { VerifyEmailView } from './verify-email-view';

const verifyEmailMock = vi.fn();
const resendVerificationMock = vi.fn();
let searchParamsValue = new URLSearchParams();

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
        resendVerification: (email: string) => resendVerificationMock(email),
      },
    },
  };
});

async function expiredTokenError() {
  const { ApiError } = await vi.importActual<typeof import('@bookhaven/api-client')>(
    '@bookhaven/api-client'
  );
  return new ApiError(400, 'Bad Request', 'expired', '/api/v1/auth/verify-email', 'token_expired');
}

const RESEND = { name: 'Отправить письмо ещё раз' };
const SENT =
  'Если этот адрес зарегистрирован и не подтверждён, мы отправили на него новое письмо.';

describe('VerifyEmailView', () => {
  beforeEach(async () => {
    searchParamsValue = new URLSearchParams({ token: 'abc' });
    verifyEmailMock.mockReset();
    verifyEmailMock.mockRejectedValue(await expiredTokenError());
    resendVerificationMock.mockReset();
    resendVerificationMock.mockResolvedValue({ status: 'sent' });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('verified -> login and catalog links', async () => {
    verifyEmailMock.mockResolvedValue({ status: 'verified' });
    render(<VerifyEmailView />);

    expect(await screen.findByText('Email подтверждён')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Войти' })).toHaveAttribute('href', '/login');
    expect(screen.getByRole('link', { name: 'К каталогу' })).toHaveAttribute('href', '/books');
    expect(verifyEmailMock).toHaveBeenCalledWith('abc');
  });

  it('already_verified -> login link', async () => {
    verifyEmailMock.mockResolvedValue({ status: 'already_verified' });
    render(<VerifyEmailView />);

    expect(await screen.findByText('Email уже подтверждён')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Войти' })).toHaveAttribute('href', '/login');
  });

  it('token_expired -> email form that resends to the typed address', async () => {
    render(<VerifyEmailView />);

    expect(await screen.findByText('Срок действия ссылки истёк')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'a@example.com' } });
    fireEvent.click(screen.getByRole('button', RESEND));

    await waitFor(() => expect(resendVerificationMock).toHaveBeenCalledWith('a@example.com'));
    expect(await screen.findByText(SENT)).toBeInTheDocument();
  });

  it('resend form rejects an empty email without calling the API', async () => {
    render(<VerifyEmailView />);

    expect(await screen.findByText('Срок действия ссылки истёк')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', RESEND));

    expect(await screen.findByText('Введите корректный email')).toBeInTheDocument();
    expect(resendVerificationMock).not.toHaveBeenCalled();
  });

  it('blank token -> no API call, missing-token copy and resend form', () => {
    searchParamsValue = new URLSearchParams({ token: '   ' });
    render(<VerifyEmailView />);

    expect(screen.getByText('Ссылка неполная')).toBeInTheDocument();
    expect(screen.getByRole('button', RESEND)).toBeInTheDocument();
    expect(verifyEmailMock).not.toHaveBeenCalled();
  });
});
