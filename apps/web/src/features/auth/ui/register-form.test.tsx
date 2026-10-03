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

  it('navigates to /login keeping the safe `redirect` param on success', async () => {
    searchParamsValue = new URLSearchParams({ redirect: '/books/abc' });
    render(<RegisterForm />);

    await fillAndSubmit();

    await waitFor(() =>
      expect(pushMock).toHaveBeenCalledWith('/login?redirect=%2Fbooks%2Fabc')
    );
  });

  it('navigates to /login when there is no redirect param', async () => {
    render(<RegisterForm />);

    await fillAndSubmit();

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/login'));
  });

  it('drops an unsafe redirect param', async () => {
    searchParamsValue = new URLSearchParams({ redirect: '//evil.com' });
    render(<RegisterForm />);

    await fillAndSubmit();

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/login'));
  });

  it('calls onRegistered with the email instead of navigating when provided', async () => {
    const onRegistered = vi.fn();
    searchParamsValue = new URLSearchParams({ redirect: '/books/abc' });
    render(<RegisterForm onRegistered={onRegistered} />);

    await fillAndSubmit();

    await waitFor(() => expect(onRegistered).toHaveBeenCalledWith('user@example.com'));
    expect(pushMock).not.toHaveBeenCalled();
  });
});
