import { api, User } from '@bookhaven/api-client';
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
  clearError: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,
  isAuthenticated: false,
  error: null,

  fetchUser: async () => {
    try {
      const user = await api.auth.me();
      set({ user, isAuthenticated: !!user, isLoading: false });
    } catch {
      set({ user: null, isAuthenticated: false, isLoading: false });
    }
  },

  login: async (email, password) => {
    set({ error: null });
    try {
      const response = await api.auth.login({ email, password });
      set({ user: response.user, isAuthenticated: true });
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
      set({ user: response.user, isAuthenticated: true });
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
      set({ user: null, isAuthenticated: false });
    }
  },

  clearError: () => set({ error: null }),
}));
