import {
  api,
  FALLBACK_ACCESS_TOKEN_TTL_SECONDS,
  isApiError,
  isEmailNotVerifiedError,
  onTokenRefresh,
  scheduleTokenRefresh,
  User,
} from '@bookhaven/api-client';
import { create } from 'zustand';
import { formatRetryAfter } from '@/shared/lib/format-retry-after';

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  error: string | null;
  fetchUser: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  // Creates the account; no session is issued (the email must be verified
  // before logging in). Resolves with the registered email.
  register: (email: string, password: string, name: string) => Promise<{ email: string }>;
  logout: () => Promise<void>;
  setUser: (user: User) => void;
  clearError: () => void;
}

const LOGIN_RATE_LIMIT_FALLBACK_SECONDS = 900;
const REGISTER_RATE_LIMIT_FALLBACK_SECONDS = 3600;

function rateLimitMessage(error: unknown, prefix: string, fallbackSeconds: number): string | null {
  if (!isApiError(error) || !(error.status === 429 || error.code === 'rate_limited')) {
    return null;
  }
  const retry = error.retryAfterSeconds;
  const seconds =
    typeof retry === 'number' && Number.isFinite(retry) && retry > 0 ? retry : fallbackSeconds;
  return `${prefix} Попробуйте снова через ${formatRetryAfter(seconds)}.`;
}

function loginRateLimitMessage(error: unknown): string | null {
  return rateLimitMessage(error, 'Слишком много попыток входа.', LOGIN_RATE_LIMIT_FALLBACK_SECONDS);
}

function registerRateLimitMessage(error: unknown): string | null {
  return rateLimitMessage(
    error,
    'Слишком много попыток регистрации.',
    REGISTER_RATE_LIMIT_FALLBACK_SECONDS,
  );
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
// using the accurate `expires_in` returned by the backend.
onTokenRefresh((tokens) => scheduleRefresh(tokens.expiresIn));

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,
  isAuthenticated: false,
  error: null,

  fetchUser: async () => {
    try {
      const user = await api.auth.me();
      set({ user, isAuthenticated: !!user, isLoading: false });
      if (user) {
        scheduleRefresh(FALLBACK_ACCESS_TOKEN_TTL_SECONDS);
      }
    } catch {
      set({ user: null, isAuthenticated: false, isLoading: false });
    }
  },

  login: async (email, password) => {
    set({ error: null });
    try {
      const response = await api.auth.login({ email, password });
      set({ user: response.user, isAuthenticated: true, isLoading: false });
      scheduleRefresh(response.expiresIn);
    } catch (error) {
      // email_not_verified is shown by the login view's own panel.
      if (!isEmailNotVerifiedError(error)) {
        const message =
          loginRateLimitMessage(error) ??
          (error instanceof Error ? error.message : 'Ошибка входа');
        set({ error: message });
      }
      throw error;
    }
  },

  register: async (email, password, name) => {
    set({ error: null });
    try {
      const response = await api.auth.register({ email, password, name });
      return { email: response.email };
    } catch (error) {
      const message =
        registerRateLimitMessage(error) ??
        (error instanceof Error ? error.message : 'Ошибка регистрации');
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
      set({ user: null, isAuthenticated: false, isLoading: false });
    }
  },

  setUser: (user) => set({ user, isAuthenticated: true, isLoading: false }),

  clearError: () => set({ error: null }),
}));
