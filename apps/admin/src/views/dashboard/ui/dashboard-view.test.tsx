import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DashboardView } from './dashboard-view';

const dashboardMock = vi.fn();

vi.mock('@bookhaven/api-client', () => ({
  api: {
    admin: {
      dashboard: (...args: unknown[]) => dashboardMock(...args),
    },
  },
}));

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <DashboardView />
    </QueryClientProvider>
  );
}

describe('DashboardView', () => {
  beforeEach(() => {
    dashboardMock.mockReset();
    dashboardMock.mockResolvedValue({
      books: 20,
      hidden_books: 3,
      authors: 11,
      genres: 5,
      pending_reviews: 2,
      pending_import_review: 4,
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders a card for pending import review with a link to the filtered books list', async () => {
    renderView();

    await waitFor(() => expect(screen.getByText('Импортировано, на проверке')).toBeInTheDocument());
    expect(screen.getByText('4')).toBeInTheDocument();

    const link = screen.getByText('Импортировано, на проверке').closest('a');
    expect(link).toHaveAttribute('href', '/books?visibility=pending_review');
  });
});
