import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminReview } from '@bookhaven/api-client';
import { ReviewsView } from './reviews-view';

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => '/moderation/reviews',
  useSearchParams: () => new URLSearchParams(),
}));

const listMock = vi.fn();
const moderateMock = vi.fn();

vi.mock('@/entities/review', () => ({
  adminReviewsApi: {
    list: (...args: unknown[]) => listMock(...args),
    moderate: (...args: unknown[]) => moderateMock(...args),
  },
}));

function makeReview(overrides: Partial<AdminReview> = {}): AdminReview {
  return {
    id: 'review-1',
    book_id: 'book-1',
    book_title: 'Тестовая книга',
    user_id: 'user-1',
    user: { id: 'user-1', name: 'Читатель', avatar_url: null },
    rating: 4,
    content: 'Хорошая книга',
    likes_count: 0,
    is_hidden: false,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  render(
    <QueryClientProvider client={queryClient}>
      <ReviewsView />
    </QueryClientProvider>
  );
  return { invalidateSpy };
}

describe('ReviewsView', () => {
  beforeEach(() => {
    listMock.mockReset();
    moderateMock.mockReset();
    listMock.mockResolvedValue({ items: [makeReview()], total: 1, page: 1, limit: 20 });
    moderateMock.mockResolvedValue(undefined);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('hiding a review invalidates the reviews list and the dashboard stats', async () => {
    const { invalidateSpy } = renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Скрыть' }));

    await waitFor(() => expect(moderateMock).toHaveBeenCalledWith('review-1', true));
    await waitFor(() =>
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'reviews'] })
    );
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
    expect(await screen.findByText(/Отзыв скрыт/)).toBeInTheDocument();
  });

  it('restoring a review also invalidates the dashboard stats', async () => {
    listMock.mockResolvedValue({
      items: [makeReview({ is_hidden: true })],
      total: 1,
      page: 1,
      limit: 20,
    });
    const { invalidateSpy } = renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Восстановить' }));

    await waitFor(() => expect(moderateMock).toHaveBeenCalledWith('review-1', false));
    await waitFor(() =>
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] })
    );
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'reviews'] });
  });
});
