import { ApiError } from '@bookhaven/api-client';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ForgotPasswordView } from './forgot-password-view';

const forgotPasswordMock = vi.fn();

vi.mock('@bookhaven/api-client', async () => {
  const actual = await vi.importActual<typeof import('@bookhaven/api-client')>(
    '@bookhaven/api-client'
  );
  return {
    ApiError: actual.ApiError,
    isApiError: actual.isApiError,
    api: {
      auth: {
        forgotPassword: (email: string) => forgotPasswordMock(email),
      },
    },
  };
});

const SENT =
  'Если аккаунт с таким email существует, мы отправили на него письмо со ссылкой для сброса пароля. Ссылка действительна 1 час.';
const SUBMIT = { name: 'Отправить ссылку' };

function submitEmail(value: string) {
  fireEvent.change(screen.getByLabelText('Email'), { target: { value } });
  fireEvent.click(screen.getByRole('button', SUBMIT));
}

describe('ForgotPasswordView', () => {
  beforeEach(() => {
    forgotPasswordMock.mockReset();
    forgotPasswordMock.mockResolvedValue({ status: 'sent' });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('renders the form with a link back to /login', () => {
    render(<ForgotPasswordView />);

    expect(screen.getByText('Восстановление пароля')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Вернуться ко входу' })).toHaveAttribute(
      'href',
      '/login'
    );
  });

  it('202 -> neutral confirmation, form replaced', async () => {
    render(<ForgotPasswordView />);
    submitEmail('anna@example.com');

    await waitFor(() => expect(forgotPasswordMock).toHaveBeenCalledWith('anna@example.com'));
    expect(await screen.findByText(SENT)).toBeInTheDocument();
    expect(screen.queryByRole('button', SUBMIT)).toBeNull();
    expect(screen.getByRole('button', { name: 'Отправить ещё раз' })).toBeDisabled();
    expect(screen.getByRole('link', { name: 'Вернуться ко входу' })).toHaveAttribute(
      'href',
      '/login'
    );
  });

  it('"Отправить ещё раз" returns to the form after the cool-down', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    render(<ForgotPasswordView />);
    submitEmail('anna@example.com');
    expect(await screen.findByText(SENT)).toBeInTheDocument();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    const again = screen.getByRole('button', { name: 'Отправить ещё раз' });
    await waitFor(() => expect(again).toBeEnabled());
    fireEvent.click(again);

    expect(screen.getByRole('button', SUBMIT)).toBeInTheDocument();
  });

  it('429 -> retry text, form stays', async () => {
    forgotPasswordMock.mockRejectedValue(
      new ApiError(429, 'Too Many Requests', 'x', '/api/v1/auth/forgot-password', 'rate_limited', 42)
    );
    render(<ForgotPasswordView />);
    submitEmail('anna@example.com');

    expect(
      await screen.findByText('Слишком много запросов. Попробуйте снова через 42 сек.')
    ).toBeInTheDocument();
    expect(screen.getByRole('button', SUBMIT)).toBeInTheDocument();
  });

  it('500 -> generic error', async () => {
    forgotPasswordMock.mockRejectedValue(
      new ApiError(500, 'Internal Server Error', 'x', '/p', 'email_send_failed')
    );
    render(<ForgotPasswordView />);
    submitEmail('anna@example.com');

    expect(
      await screen.findByText('Не удалось отправить письмо. Попробуйте позже.')
    ).toBeInTheDocument();
  });

  it('400 invalid_email -> field error', async () => {
    forgotPasswordMock.mockRejectedValue(new ApiError(400, 'Bad Request', 'x', '/p', 'invalid_email'));
    render(<ForgotPasswordView />);
    submitEmail('anna@example.com');

    expect(await screen.findByText('Введите корректный email')).toBeInTheDocument();
  });

  it('invalid email -> no API call', async () => {
    render(<ForgotPasswordView />);
    submitEmail('not-an-email');

    expect(await screen.findByText('Введите корректный email')).toBeInTheDocument();
    expect(forgotPasswordMock).not.toHaveBeenCalled();
  });
});
