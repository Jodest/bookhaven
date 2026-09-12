import {
  api,
  FALLBACK_ACCESS_TOKEN_TTL_SECONDS,
  onTokenRefresh,
  scheduleTokenRefresh,
  User,
} from '@bookhaven/api-client';
import { create } from 'zustand';

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  error: string | null;
  fetchUser: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;
  logout: () => Promise<void>;
  setUser: (user: User) => void;
  clearError: () => void;
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
      const message = error instanceof Error ? error.message : 'Ошибка входа';
      set({ error: message });
      throw error;
    }
  },

  register: async (email, password, name) => {
    set({ error: null });
    try {
      const response = await api.auth.register({ email, password, name });
      set({ user: response.user, isAuthenticated: true, isLoading: false });
      scheduleRefresh(response.expiresIn);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Ошибка регистрации';
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
