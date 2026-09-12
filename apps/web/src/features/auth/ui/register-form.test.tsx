import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { RegisterForm } from './register-form';

const pushMock = vi.fn();
const registerMock = vi.fn();
const clearErrorMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  useSearchParams: () => searchParamsValue,
}));

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({
    register: registerMock,
    error: null,
    clearError: clearErrorMock,
  }),
}));

async function fillAndSubmit() {
  fireEvent.change(screen.getByLabelText('Имя'), {
    target: { value: 'Test User' },
  });
  fireEvent.change(screen.getByLabelText('Email'), {
    target: { value: 'user@example.com' },
  });
  fireEvent.change(screen.getByLabelText('Пароль'), {
    target: { value: 'password123' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Зарегистрироваться' }));

  await waitFor(() => expect(registerMock).toHaveBeenCalled());
}

describe('RegisterForm', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    registerMock.mockReset();
    pushMock.mockReset();
    clearErrorMock.mockReset();
    registerMock.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('redirects to the safe `redirect` param on success', async () => {
    searchParamsValue = new URLSearchParams({ redirect: '/books/abc' });
    render(<RegisterForm />);

    await fillAndSubmit();

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/books/abc'));
  });

  it('falls back to /profile when there is no redirect param', async () => {
    render(<RegisterForm />);

    await fillAndSubmit();

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/profile'));
  });

  it('falls back to /profile when the redirect param is unsafe', async () => {
    searchParamsValue = new URLSearchParams({ redirect: '//evil.com' });
    render(<RegisterForm />);

    await fillAndSubmit();

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/profile'));
  });
});
