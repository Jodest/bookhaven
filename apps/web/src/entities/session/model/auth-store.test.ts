import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const meMock = vi.fn();
const loginMock = vi.fn();
const registerMock = vi.fn();
const logoutMock = vi.fn();
const refreshMock = vi.fn();
const scheduleTokenRefreshMock = vi.fn();
const cancelMock = vi.fn();

type RefreshTokens = { accessToken: string; refreshToken: string; expiresIn: number };
// `var` (not `let`/`const`) and no initializer: vitest hoists `vi.mock`/`import`
// above this statement, so the mocked `onTokenRefresh` assigns to this binding
// (hoisted as `undefined`) before this line's own (re-)declaration would run.
var onTokenRefreshListener: ((tokens: RefreshTokens) => void) | null;

vi.mock('@bookhaven/api-client', async () => {
  const actual = await vi.importActual<typeof import('@bookhaven/api-client')>(
    '@bookhaven/api-client'
  );
  return {
    ApiError: actual.ApiError,
    isApiError: actual.isApiError,
    isEmailNotVerifiedError: actual.isEmailNotVerifiedError,
    api: {
      auth: {
        me: (...args: unknown[]) => meMock(...args),
        login: (...args: unknown[]) => loginMock(...args),
        register: (...args: unknown[]) => registerMock(...args),
        logout: (...args: unknown[]) => logoutMock(...args),
        refresh: (...args: unknown[]) => refreshMock(...args),
      },
    },
    onTokenRefresh: (listener: (tokens: RefreshTokens) => void) => {
      onTokenRefreshListener = listener;
      return () => {
        onTokenRefreshListener = null;
      };
    },
    scheduleTokenRefresh: (expiresIn: number, onDue: () => void) => {
      scheduleTokenRefreshMock(expiresIn, onDue);
      return cancelMock;
    },
    FALLBACK_ACCESS_TOKEN_TTL_SECONDS: 900,
  };
});

import { ApiError } from '@bookhaven/api-client';
import { useAuthStore } from './auth-store';

describe('auth-store proactive refresh', () => {
  beforeEach(() => {
    meMock.mockReset();
    loginMock.mockReset();
    registerMock.mockReset();
    logoutMock.mockReset();
    refreshMock.mockReset();
    scheduleTokenRefreshMock.mockReset();
    cancelMock.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('schedules a refresh timer on successful login using the real expires_in', async () => {
    loginMock.mockResolvedValue({
      user: { id: '1', email: 'user@example.com' },
      accessToken: 'access',
      refreshToken: 'refresh',
      expiresIn: 900,
    });

    await useAuthStore.getState().login('user@example.com', 'password123');

    expect(scheduleTokenRefreshMock).toHaveBeenCalledWith(900, expect.any(Function));
  });

  it('register does not create a session', async () => {
    useAuthStore.setState({ user: null, isAuthenticated: false, error: null });
    registerMock.mockResolvedValue({ email: 'user@example.com', verificationRequired: true });

    const result = await useAuthStore
      .getState()
      .register('user@example.com', 'password123', 'User');

    expect(result).toEqual({ email: 'user@example.com' });
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    expect(useAuthStore.getState().user).toBeNull();
    expect(scheduleTokenRefreshMock).not.toHaveBeenCalled();
  });

  it('login with email_not_verified rethrows without setting error', async () => {
    useAuthStore.setState({ user: null, isAuthenticated: false, error: null });
    const err = new ApiError(
      403,
      'Forbidden',
      'email not verified',
      '/api/v1/auth/login',
      'email_not_verified'
    );
    loginMock.mockRejectedValue(err);

    await expect(
      useAuthStore.getState().login('user@example.com', 'password123')
    ).rejects.toBe(err);

    expect(useAuthStore.getState().error).toBeNull();
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    expect(scheduleTokenRefreshMock).not.toHaveBeenCalled();
  });

  it('login with other errors sets error', async () => {
    useAuthStore.setState({ error: null });
    loginMock.mockRejectedValue(
      new ApiError(401, 'Unauthorized', 'invalid credentials', '/api/v1/auth/login')
    );

    await expect(useAuthStore.getState().login('user@example.com', 'bad')).rejects.toBeTruthy();

    expect(useAuthStore.getState().error).toBe('invalid credentials');
  });

  it('maps 429 rate_limited to a message with the formatted wait', async () => {
    useAuthStore.setState({ error: null });
    loginMock.mockRejectedValue(
      new ApiError(429, 'Too Many Requests', 'too many requests', '/api/v1/auth/login', 'rate_limited', 600)
    );

    await expect(useAuthStore.getState().login('user@example.com', 'bad')).rejects.toBeTruthy();

    expect(useAuthStore.getState().error).toBe(
      'Слишком много попыток входа. Попробуйте снова через 10 мин.'
    );
  });

  it('falls back to 15 мин when a 429 carries no retry_after_seconds', async () => {
    useAuthStore.setState({ error: null });
    loginMock.mockRejectedValue(
      new ApiError(429, 'Too Many Requests', 'too many requests', '/api/v1/auth/login')
    );

    await expect(useAuthStore.getState().login('user@example.com', 'bad')).rejects.toBeTruthy();

    expect(useAuthStore.getState().error).toBe(
      'Слишком много попыток входа. Попробуйте снова через 15 мин.'
    );
  });

  it('maps a register 429 to a message with the formatted wait', async () => {
    useAuthStore.setState({ error: null });
    registerMock.mockRejectedValue(
      new ApiError(429, 'Too Many Requests', 'too many requests', '/api/v1/auth/register', 'rate_limited', 600)
    );

    await expect(
      useAuthStore.getState().register('user@example.com', 'password123', 'User')
    ).rejects.toBeTruthy();

    expect(useAuthStore.getState().error).toBe(
      'Слишком много попыток регистрации. Попробуйте снова через 10 мин.'
    );
  });

  it('falls back to 1 ч when a register 429 carries no retry_after_seconds', async () => {
    useAuthStore.setState({ error: null });
    registerMock.mockRejectedValue(
      new ApiError(429, 'Too Many Requests', 'too many requests', '/api/v1/auth/register')
    );

    await expect(
      useAuthStore.getState().register('user@example.com', 'password123', 'User')
    ).rejects.toBeTruthy();

    expect(useAuthStore.getState().error).toBe(
      'Слишком много попыток регистрации. Попробуйте снова через 1 ч.'
    );
  });

  it('schedules a fallback refresh timer when fetchUser finds an existing session', async () => {
    meMock.mockResolvedValue({ id: '1', email: 'user@example.com' });

    await useAuthStore.getState().fetchUser();

    expect(scheduleTokenRefreshMock).toHaveBeenCalledWith(900, expect.any(Function));
  });

  it('does not schedule a refresh timer when fetchUser finds no session', async () => {
    meMock.mockResolvedValue(null);

    await useAuthStore.getState().fetchUser();

    expect(scheduleTokenRefreshMock).not.toHaveBeenCalled();
  });

  it('cancels the refresh timer on logout', async () => {
    loginMock.mockResolvedValue({
      user: { id: '1', email: 'user@example.com' },
      accessToken: 'access',
      refreshToken: 'refresh',
      expiresIn: 900,
    });
    await useAuthStore.getState().login('user@example.com', 'password123');

    logoutMock.mockResolvedValue(undefined);
    await useAuthStore.getState().logout();

    expect(cancelMock).toHaveBeenCalled();
  });

  it('reschedules using the new expires_in whenever onTokenRefresh fires', () => {
    expect(onTokenRefreshListener).not.toBeNull();

    onTokenRefreshListener?.({
      accessToken: 'rotated-access',
      refreshToken: 'rotated-refresh',
      expiresIn: 1200,
    });

    expect(scheduleTokenRefreshMock).toHaveBeenCalledWith(1200, expect.any(Function));
  });
});
