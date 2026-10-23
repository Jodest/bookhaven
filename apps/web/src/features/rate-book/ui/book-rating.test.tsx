import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { act } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useMyRatingStore } from '@/entities/rating';
import { BookRating } from './book-rating';

let isAuthenticatedValue = true;
const myMock = vi.fn();
const setMock = vi.fn();
const refreshMock = vi.fn();

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ isAuthenticated: isAuthenticatedValue }),
}));

vi.mock('next/navigation', () => ({
  useRouter: () => ({ refresh: refreshMock }),
}));

vi.mock('@bookhaven/api-client', () => ({
  api: {
    ratings: {
      my: (id: string) => myMock(id),
      set: (id: string, value: number) => setMock(id, value),
    },
  },
}));

describe('BookRating', () => {
  afterEach(() => {
    isAuthenticatedValue = true;
    useMyRatingStore.setState({ ownerId: null, byBook: {} });
    myMock.mockReset();
    setMock.mockReset();
    refreshMock.mockReset();
  });

  it('saves a rating, shows it and refreshes the page data', async () => {
    myMock.mockResolvedValue(null);
    setMock.mockResolvedValue({ rating: 4, ratings_count: 1 });
    render(<BookRating bookId="b1" slug="book-1" />);

    fireEvent.click(screen.getByRole('button', { name: 'Оценить на 4' }));

    expect(await screen.findByText('Моя оценка: 4')).toBeInTheDocument();
    expect(setMock).toHaveBeenCalledWith('b1', 4);
    await waitFor(() => expect(refreshMock).toHaveBeenCalled());
  });

  it('restores the previous value and shows an error on failure', async () => {
    myMock.mockResolvedValue({ value: 2 });
    setMock.mockRejectedValue('boom');
    render(<BookRating bookId="b1" slug="book-1" />);
    expect(await screen.findByText('Моя оценка: 2')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Оценить на 5' }));

    expect(await screen.findByText('Ошибка сохранения оценки')).toBeInTheDocument();
    expect(screen.getByText('Моя оценка: 2')).toBeInTheDocument();
    expect(refreshMock).not.toHaveBeenCalled();
  });

  it('guest gets an encoded login link', () => {
    isAuthenticatedValue = false;
    render(<BookRating bookId="b1" slug="book-1" />);
    expect(screen.getByRole('link', { name: 'Войдите, чтобы оценить' })).toHaveAttribute(
      'href',
      '/login?redirect=%2Fbooks%2Fbook-1'
    );
  });

  it('derives star clip ids from React.useId (deterministic, no colons)', () => {
    myMock.mockResolvedValue(null);
    render(<BookRating bookId="b1" slug="book-1" />);
    const ids = Array.from(document.querySelectorAll('clipPath')).map((el) => el.getAttribute('id'));
    expect(ids).toHaveLength(5);
    expect(new Set(ids).size).toBe(5);
    for (const id of ids) expect(id).toMatch(/^[A-Za-z0-9_]+-star-clip$/);
  });

  it('prompts to rate when there is no rating and previews stars on hover', async () => {
    myMock.mockResolvedValue(null);
    render(<BookRating bookId="b1" slug="book-1" />);
    expect(screen.getByText('Оцените книгу')).toBeInTheDocument();

    const filled = () =>
      Array.from(document.querySelectorAll('clipPath rect')).filter(
        (r) => r.getAttribute('width') === '100%'
      ).length;
    expect(filled()).toBe(0);
    fireEvent.mouseEnter(screen.getByRole('button', { name: 'Оценить на 3' }));
    expect(filled()).toBe(3);
    fireEvent.mouseLeave(screen.getByRole('button', { name: 'Оценить на 3' }).parentElement as HTMLElement);
    expect(filled()).toBe(0);
  });

  it('writes the store on a successful save but not on failure', async () => {
    myMock.mockResolvedValue({ value: 2 });
    setMock.mockResolvedValueOnce({ rating: 4, ratings_count: 1 });
    render(<BookRating bookId="b1" slug="book-1" />);
    expect(await screen.findByText('Моя оценка: 2')).toBeInTheDocument();
    expect(useMyRatingStore.getState().byBook.b1).toBe(2);

    fireEvent.click(screen.getByRole('button', { name: 'Оценить на 4' }));
    await waitFor(() => expect(useMyRatingStore.getState().byBook.b1).toBe(4));

    setMock.mockRejectedValueOnce(new Error('boom'));
    fireEvent.click(screen.getByRole('button', { name: 'Оценить на 1' }));
    expect(await screen.findByText('boom')).toBeInTheDocument();
    expect(useMyRatingStore.getState().byBook.b1).toBe(4);
    expect(screen.getByText('Моя оценка: 4')).toBeInTheDocument();
  });

  it('follows an external store change', async () => {
    myMock.mockResolvedValue(null);
    render(<BookRating bookId="b1" slug="book-1" />);
    expect(screen.getByText('Оцените книгу')).toBeInTheDocument();
    act(() => useMyRatingStore.getState().setMyRating('b1', 5));
    expect(await screen.findByText('Моя оценка: 5')).toBeInTheDocument();
  });
});
