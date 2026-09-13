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

vi.mock('@bookhaven/api-client', () => ({
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
}));

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

  it('schedules a refresh timer on successful register using the real expires_in', async () => {
    registerMock.mockResolvedValue({
      user: { id: '1', email: 'user@example.com' },
      accessToken: 'access',
      refreshToken: 'refresh',
      expiresIn: 1234,
    });

    await useAuthStore.getState().register('user@example.com', 'password123', 'User');

    expect(scheduleTokenRefreshMock).toHaveBeenCalledWith(1234, expect.any(Function));
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
