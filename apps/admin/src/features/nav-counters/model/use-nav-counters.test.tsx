import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useNavCounters } from './use-nav-counters';

const dashboardMock = vi.fn();
const listJobsMock = vi.fn();

vi.mock('@bookhaven/api-client', () => ({
  api: {
    admin: {
      dashboard: (...args: unknown[]) => dashboardMock(...args),
      harvest: {
        trigger: vi.fn(),
        jobs: { list: (...args: unknown[]) => listJobsMock(...args), get: vi.fn() },
        authors: { search: vi.fn() },
      },
    },
  },
}));

let currentUser: { id: string; email: string; role: string } | null = null;

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ user: currentUser }),
  isAdmin: (user: { role: string } | null) => user?.role === 'admin',
}));

function renderCounters() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return renderHook(() => useNavCounters(), { wrapper });
}

// Lets any (wrongly) enabled query run its queryFn before asserting.
const flush = () => new Promise((resolve) => setTimeout(resolve, 20));

describe('useNavCounters', () => {
  beforeEach(() => {
    dashboardMock.mockReset();
    listJobsMock.mockReset();
    dashboardMock.mockResolvedValue({
      books: 20,
      hidden_books: 3,
      authors: 11,
      genres: 5,
      pending_reviews: 9,
      pending_import_review: 4,
      pending_authors: 7,
    });
    listJobsMock.mockResolvedValue({
      items: [
        { id: 'a', queue: 'harvest', state: 'active', type: 't', max_retry: 3, retried: 0 },
        { id: 'b', queue: 'harvest', state: 'completed', type: 't', max_retry: 3, retried: 0 },
      ],
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('admin: fetches dashboard and jobs once each and returns counters', async () => {
    currentUser = { id: 'u1', email: 'admin@example.com', role: 'admin' };
    const { result } = renderCounters();

    await waitFor(() =>
      expect(result.current).toEqual({
        sections: { moderation: 11, import: 1 },
        tabs: { 'moderation.books': 4, 'moderation.authors': 7, 'import.jobs': 1 },
      })
    );
    expect(dashboardMock).toHaveBeenCalledTimes(1);
    expect(listJobsMock).toHaveBeenCalledTimes(1);
  });

  it('moderator: no request to either endpoint, empty counters', async () => {
    currentUser = { id: 'u2', email: 'mod@example.com', role: 'moderator' };
    const { result } = renderCounters();
    await flush();

    expect(dashboardMock).not.toHaveBeenCalled();
    expect(listJobsMock).not.toHaveBeenCalled();
    expect(result.current).toEqual({ sections: {}, tabs: {} });
  });

  it('loading session (user null): no request to either endpoint', async () => {
    currentUser = null;
    const { result } = renderCounters();
    await flush();

    expect(dashboardMock).not.toHaveBeenCalled();
    expect(listJobsMock).not.toHaveBeenCalled();
    expect(result.current).toEqual({ sections: {}, tabs: {} });
  });
});
