import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ReviewsSection } from './reviews-section';

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ isAuthenticated: false, user: null }),
}));

vi.mock('@/entities/review', () => ({
  ReviewCard: () => null,
  reviewsApi: { list: vi.fn().mockResolvedValue({ items: [], total: 0 }) },
}));

vi.mock('@/features/write-review', () => ({ ReviewForm: () => null }));

describe('ReviewsSection', () => {
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
});
