import { useAuthStore } from '../model/auth-store';
import { ApiError, isApiError } from '@bookhaven/api-client';
import {
  MutationCache,
  QueryCache,
  QueryClient,
} from '@tanstack/react-query';

function isAdminApiError(error: unknown): error is ApiError {
  return isApiError(error) && error.path.startsWith('/api/v1/admin');
}

function handleAdminApiError(error: unknown) {
  if (!isAdminApiError(error)) return;

  const { status } = error;
  const state = useAuthStore.getState();

  if (status === 401) {
    if (state.isSigningOut) return;

    if (typeof window === 'undefined') {
      void state.signOut();
      return;
    }

    const currentPath = window.location.pathname + window.location.search;
    if (window.location.pathname === '/login') {
      void state.signOut();
      return;
    }

    void state.signOutAndRedirect(currentPath);
    return;
  }

  if (status === 403) {
    if (typeof window === 'undefined') return;
    state.setForbiddenPath(window.location.pathname);
  }
}

export function createAdminQueryClient(): QueryClient {
  return new QueryClient({
    queryCache: new QueryCache({
      onError: handleAdminApiError,
    }),
    mutationCache: new MutationCache({
      onError: handleAdminApiError,
    }),
    defaultOptions: {
      queries: {
        staleTime: 60 * 1000,
        refetchOnWindowFocus: false,
        retry: (failureCount, error) => {
          if (
            isAdminApiError(error) &&
            (error.status === 401 || error.status === 403)
          ) {
            return false;
          }
          return failureCount < 3;
        },
      },
    },
  });
}
