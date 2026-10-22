import {
  api,
  FALLBACK_ACCESS_TOKEN_TTL_SECONDS,
  isApiError,
  isEmailNotVerifiedError,
  onTokenRefresh,
  scheduleTokenRefresh,
  setAccessToken,
  User,
} from '@bookhaven/api-client';
import { create } from 'zustand';
import { formatRetryAfter, sanitizeRedirect } from '@/shared/lib';

const ACCESS_TOKEN_KEY = 'admin_access_token';

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isSigningOut: boolean;
  error: string | null;
  forbiddenPath: string | null;
  fetchUser: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  signOut: () => Promise<void>;
  signOutAndRedirect: (redirectPath?: string) => Promise<void>;
  setForbiddenPath: (path: string | null) => void;
  clearForbiddenPath: () => void;
  clearError: () => void;
}

function getStoredToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem(ACCESS_TOKEN_KEY);
}

function setStoredToken(token: string | null): void {
  if (typeof window === 'undefined') return;
  if (token) {
    localStorage.setItem(ACCESS_TOKEN_KEY, token);
  } else {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
  }
}

// Restore any persisted bearer token on module load so admin mutations work
// after a page refresh while reads continue to use the HttpOnly cookie.
const initialToken = getStoredToken();
if (initialToken) {
  setAccessToken(initialToken);
}

const LOGIN_RATE_LIMIT_FALLBACK_SECONDS = 900;

function loginRateLimitMessage(error: unknown): string | null {
  if (!isApiError(error) || !(error.status === 429 || error.code === 'rate_limited')) {
    return null;
  }
  const retry = error.retryAfterSeconds;
  const seconds =
    typeof retry === 'number' && Number.isFinite(retry) && retry > 0
      ? retry
      : LOGIN_RATE_LIMIT_FALLBACK_SECONDS;
  return `Слишком много попыток входа. Попробуйте снова через ${formatRetryAfter(seconds)}.`;
}

let cancelRefreshTimer: (() => void) | null = null;

function scheduleRefresh(expiresIn: number): void {
  cancelRefreshTimer?.();
  cancelRefreshTimer = scheduleTokenRefresh(expiresIn, () => {
    api.auth.refresh().catch(() => {
      // Proactive refresh failed — the reactive 401-retry path is the
      // correctness backstop for the next real request.
    });
  });
}

// Any successful refresh — proactive or reactive, triggered from inside
// `fetcher()` or from this store's own timer — reschedules the next one
// using the accurate `expires_in`, and persists the rotated bearer token.
// This is the piece that would otherwise be missed for reactive refreshes
// triggered deep inside `fetcher()` (invisible to this store otherwise).
onTokenRefresh((tokens) => {
  setStoredToken(tokens.accessToken);
  scheduleRefresh(tokens.expiresIn);
});

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,
  isAuthenticated: false,
  isSigningOut: false,
  error: null,
  forbiddenPath: null,

  fetchUser: async () => {
    try {
      const user = await api.auth.me();
      set({
        user,
        isAuthenticated: !!user,
        isLoading: false,
        forbiddenPath: null,
      });
      if (user) {
        scheduleRefresh(FALLBACK_ACCESS_TOKEN_TTL_SECONDS);
      } else {
        setAccessToken(null);
        setStoredToken(null);
      }
    } catch {
      set({
        user: null,
        isAuthenticated: false,
        isLoading: false,
      });
      cancelRefreshTimer?.();
      cancelRefreshTimer = null;
      setAccessToken(null);
      setStoredToken(null);
    }
  },

  login: async (email, password) => {
    set({ error: null });
    try {
      const response = await api.auth.login({ email, password });
      setAccessToken(response.accessToken);
      setStoredToken(response.accessToken);
      set({
        user: response.user,
        isAuthenticated: true,
        isLoading: false,
        isSigningOut: false,
        forbiddenPath: null,
      });
      scheduleRefresh(response.expiresIn);
    } catch (error) {
      const message =
        loginRateLimitMessage(error) ??
        (isEmailNotVerifiedError(error)
          ? 'Email не подтверждён. Подтвердите его по ссылке из письма.'
          : error instanceof Error
            ? error.message
            : 'Ошибка входа');
      set({ error: message });
      throw error;
    }
  },

  logout: async () => {
    try {
      await api.auth.logout();
    } finally {
      cancelRefreshTimer?.();
      cancelRefreshTimer = null;
      setAccessToken(null);
      setStoredToken(null);
      set({
        user: null,
        isAuthenticated: false,
        isLoading: false,
        isSigningOut: false,
        forbiddenPath: null,
      });
    }
  },

  signOut: async () => {
    const state = useAuthStore.getState();
    if (state.isSigningOut) return;
    set({ isSigningOut: true });
    try {
      await api.auth.logout();
    } catch {
      // ignore: token may already be invalid
    } finally {
      cancelRefreshTimer?.();
      cancelRefreshTimer = null;
      setAccessToken(null);
      setStoredToken(null);
      set({
        user: null,
        isAuthenticated: false,
        isLoading: false,
        isSigningOut: false,
        forbiddenPath: null,
      });
    }
  },

  signOutAndRedirect: async (redirectPath?: string) => {
    const state = useAuthStore.getState();
    if (state.isSigningOut) return;
    set({ isSigningOut: true });
    try {
      await api.auth.logout();
    } catch {
      // ignore: token may already be invalid
    } finally {
      cancelRefreshTimer?.();
      cancelRefreshTimer = null;
      setAccessToken(null);
      setStoredToken(null);
      set({
        user: null,
        isAuthenticated: false,
        isLoading: false,
        isSigningOut: false,
        forbiddenPath: null,
      });
      if (typeof window !== 'undefined') {
        const origin = window.location.origin;
        const safeRedirect = sanitizeRedirect(redirectPath, origin, '/');
        window.location.assign(
          `/login?redirect=${encodeURIComponent(safeRedirect)}`
        );
      }
    }
  },

  setForbiddenPath: (path: string | null) => set({ forbiddenPath: path }),

  clearForbiddenPath: () => set({ forbiddenPath: null }),

  clearError: () => set({ error: null }),
}));

export function isAdmin(user: User | null): boolean {
  return user?.role === 'admin';
}

export function isModerator(user: User | null): boolean {
  return user?.role === 'moderator' || user?.role === 'admin';
}
