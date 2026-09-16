import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';

const meMock = vi.fn();
const loginMock = vi.fn();
const logoutMock = vi.fn();
const refreshMock = vi.fn();
const setAccessTokenMock = vi.fn();
const scheduleTokenRefreshMock = vi.fn();
const cancelMock = vi.fn();
const assignMock = vi.fn();

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
      logout: (...args: unknown[]) => logoutMock(...args),
      refresh: (...args: unknown[]) => refreshMock(...args),
    },
  },
  setAccessToken: (...args: unknown[]) => setAccessTokenMock(...args),
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

describe('auth-store', () => {
  beforeAll(() => {
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { origin: 'https://admin.example.com', assign: assignMock },
    });
  });

  beforeEach(() => {
    meMock.mockReset();
    loginMock.mockReset();
    logoutMock.mockReset();
    refreshMock.mockReset();
    setAccessTokenMock.mockReset();
    scheduleTokenRefreshMock.mockReset();
    cancelMock.mockReset();
    assignMock.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('proactive refresh', () => {
    it('schedules a refresh timer on successful login using the real expires_in', async () => {
      loginMock.mockResolvedValue({
        user: { id: '1', email: 'admin@example.com', role: 'admin' },
        accessToken: 'access',
        refreshToken: 'refresh',
        expiresIn: 900,
      });

      await useAuthStore.getState().login('admin@example.com', 'password123');

      expect(scheduleTokenRefreshMock).toHaveBeenCalledWith(900, expect.any(Function));
    });

    it('schedules a fallback refresh timer when fetchUser finds an existing session', async () => {
      meMock.mockResolvedValue({ id: '1', email: 'admin@example.com', role: 'admin' });

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
        user: { id: '1', email: 'admin@example.com', role: 'admin' },
        accessToken: 'access',
        refreshToken: 'refresh',
        expiresIn: 900,
      });
      await useAuthStore.getState().login('admin@example.com', 'password123');

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

  describe('signOutAndRedirect', () => {
    it('logs out and redirects to /login with the sanitized redirect param', async () => {
      logoutMock.mockResolvedValue(undefined);

      await useAuthStore.getState().signOutAndRedirect('/dashboard');

      expect(logoutMock).toHaveBeenCalled();
      expect(assignMock).toHaveBeenCalledWith('/login?redirect=%2Fdashboard');
      expect(useAuthStore.getState().isSigningOut).toBe(false);
      expect(useAuthStore.getState().isAuthenticated).toBe(false);
    });

    it('falls back to "/" when the redirect path is unsafe', async () => {
      logoutMock.mockResolvedValue(undefined);

      await useAuthStore.getState().signOutAndRedirect('https://evil.com');

      expect(assignMock).toHaveBeenCalledWith('/login?redirect=%2F');
    });

    it('still redirects even if the logout API call fails', async () => {
      logoutMock.mockRejectedValue(new Error('network error'));

      await useAuthStore.getState().signOutAndRedirect('/dashboard');

      expect(assignMock).toHaveBeenCalledWith('/login?redirect=%2Fdashboard');
      expect(useAuthStore.getState().isSigningOut).toBe(false);
    });

    it('isSigningOut guard prevents a concurrent sign-out from calling logout again', async () => {
      logoutMock.mockResolvedValue(undefined);

      const first = useAuthStore.getState().signOutAndRedirect('/dashboard');
      const second = useAuthStore.getState().signOutAndRedirect('/dashboard');

      await Promise.all([first, second]);

      expect(logoutMock).toHaveBeenCalledTimes(1);
      expect(useAuthStore.getState().isSigningOut).toBe(false);
    });
  });
});
