import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useMyRatingStore } from '@/entities/rating';
import { ReviewsSection } from './reviews-section';

let authValue: { isAuthenticated: boolean; user: { id: string } | null } = {
  isAuthenticated: false,
  user: null,
};
const authSnapshot = () => authValue;
const listMock = vi.fn();
const removeMock = vi.fn();
const refreshMock = vi.fn();

vi.mock('@/entities/session', () => ({
  useAuthStore: () => authSnapshot(),
}));

vi.mock('next/navigation', () => ({
  useRouter: () => ({ refresh: refreshMock }),
}));

vi.mock('@/entities/review', () => ({
  ReviewCard: ({
    review,
    onDelete,
  }: {
    review: { id: string; rating: number; is_hidden?: boolean };
    onDelete: (r: { id: string; rating: number }) => void;
  }) => (
    <div>
      {review.is_hidden && <span data-testid={`hidden-${review.id}`}>hidden-card</span>}
      <span data-testid={`rating-${review.id}`}>{review.rating}</span>
      <button onClick={() => onDelete(review)}>del-{review.id}</button>
    </div>
  ),
  reviewsApi: {
    list: (...args: unknown[]) => listMock(...args),
    remove: (id: string) => removeMock(id),
  },
}));

vi.mock('@/features/write-review', () => ({ ReviewForm: () => null }));

describe('ReviewsSection', () => {
  beforeEach(() => {
    listMock.mockResolvedValue({ items: [], total: 0 });
  });
  afterEach(() => {
    authValue = { isAuthenticated: false, user: null };
    useMyRatingStore.setState({ ownerId: null, byBook: {} });
    listMock.mockReset();
    removeMock.mockReset();
    refreshMock.mockReset();
    vi.restoreAllMocks();
  });

  it('is an anchor target with the shared heading style', async () => {
    const { container } = render(<ReviewsSection bookId="b1" slug="book-1" />);
    await screen.findByText('Войдите, чтобы написать рецензию');

    expect(container.querySelector('section#reviews')).not.toBeNull();
    expect(screen.getByRole('heading', { name: 'Рецензии' })).toHaveClass(
      'font-display',
      'text-xl',
      'font-semibold'
    );
  });

  it('guest link carries an encoded return path', async () => {
    render(<ReviewsSection bookId="b1" slug="book-1" />);
    expect(await screen.findByRole('link', { name: 'Войдите, чтобы написать рецензию' })).toHaveAttribute(
      'href',
      '/login?redirect=%2Fbooks%2Fbook-1'
    );
  });

  describe('signed in', () => {
    const own = { id: 'r1', user_id: 'u1', rating: 5 };
    const other = { id: 'r2', user_id: 'u2', rating: 3 };
    beforeEach(() => {
      authValue = { isAuthenticated: true, user: { id: 'u1' } };
      listMock.mockResolvedValue({ items: [own, other], total: 2, my_review: own });
    });

    it('own card rating follows a store change, other cards stay', async () => {
      render(<ReviewsSection bookId="b1" slug="book-1" />);
      expect(await screen.findByTestId('rating-r1')).toHaveTextContent('5');
      expect(useMyRatingStore.getState().byBook.b1).toBe(5);

      act(() => useMyRatingStore.getState().setMyRating('b1', 2));

      await waitFor(() => expect(screen.getByTestId('rating-r1')).toHaveTextContent('2'));
      expect(screen.getByTestId('rating-r2')).toHaveTextContent('3');
    });

    it('delete confirmation says the rating stays', async () => {
      const confirmMock = vi.spyOn(window, 'confirm').mockReturnValue(false);
      render(<ReviewsSection bookId="b1" slug="book-1" />);
      fireEvent.click(await screen.findByText('del-r1'));

      expect(confirmMock).toHaveBeenCalledWith('Удалить рецензию? Ваша оценка книги сохранится.');
      expect(removeMock).not.toHaveBeenCalled();
    });

    it('deletes after confirmation', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      removeMock.mockResolvedValue(undefined);
      render(<ReviewsSection bookId="b1" slug="book-1" />);
      fireEvent.click(await screen.findByText('del-r1'));

      await waitFor(() => expect(removeMock).toHaveBeenCalledWith('r1'));
    });

    describe('own hidden review', () => {
      const hidden = { id: 'r1', user_id: 'u1', rating: 5, is_hidden: true };
      beforeEach(() => {
        listMock.mockResolvedValue({ items: [], total: 0, my_review: hidden });
      });

      it('shows the own card with delete and no write/edit buttons', async () => {
        render(<ReviewsSection bookId="b1" slug="book-1" />);
        expect(await screen.findByTestId('hidden-r1')).toBeInTheDocument();
        expect(screen.getByText('del-r1')).toBeInTheDocument();
        expect(screen.queryByText('Редактировать рецензию')).toBeNull();
        expect(screen.queryByText('Написать рецензию')).toBeNull();
      });

      it('after a confirmed delete offers to write a new review', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(true);
        removeMock.mockResolvedValue(undefined);
        render(<ReviewsSection bookId="b1" slug="book-1" />);
        listMock.mockResolvedValue({ items: [], total: 0 });
        fireEvent.click(await screen.findByText('del-r1'));

        await waitFor(() => expect(removeMock).toHaveBeenCalledWith('r1'));
        expect(await screen.findByText('Написать рецензию')).toBeInTheDocument();
        expect(screen.queryByTestId('hidden-r1')).toBeNull();
      });

      it('keeps the card when the confirm is dismissed', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(false);
        render(<ReviewsSection bookId="b1" slug="book-1" />);
        fireEvent.click(await screen.findByText('del-r1'));

        expect(removeMock).not.toHaveBeenCalled();
        expect(screen.getByTestId('hidden-r1')).toBeInTheDocument();
        expect(screen.queryByText('Написать рецензию')).toBeNull();
      });
    });

    it('a visible own review still offers editing', async () => {
      render(<ReviewsSection bookId="b1" slug="book-1" />);
      expect(await screen.findByText('Редактировать рецензию')).toBeInTheDocument();
    });
  });
});
