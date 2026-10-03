import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { RegisterView } from './register-view';

const pushMock = vi.fn();
const registerMock = vi.fn();
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
    api: { auth: { resendVerification: vi.fn() } },
  };
});

vi.mock('@/entities/session', () => ({
  useAuthStore: Object.assign(
    () => ({ register: registerMock, error: null, clearError: vi.fn() }),
    { getState: () => ({ fetchUser: vi.fn() }) }
  ),
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
    registerMock.mockResolvedValue(undefined);
    pushMock.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('switches to the check-email panel with a safe continue link', async () => {
    searchParamsValue = new URLSearchParams({ redirect: '/books/abc' });
    render(<RegisterView />);

    await register();

    expect(await screen.findByText('Проверьте почту')).toBeInTheDocument();
    expect(screen.getByText('user@example.com')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Отправить письмо ещё раз' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Продолжить' })).toHaveAttribute('href', '/books/abc');
    expect(pushMock).not.toHaveBeenCalled();
  });

  it('continue link falls back to /profile for an unsafe redirect', async () => {
    searchParamsValue = new URLSearchParams({ redirect: '//evil.com' });
    render(<RegisterView />);

    await register();

    expect(await screen.findByRole('link', { name: 'Продолжить' })).toHaveAttribute(
      'href',
      '/profile'
    );
  });
});
