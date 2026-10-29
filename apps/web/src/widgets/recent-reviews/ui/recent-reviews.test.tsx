import type { HomeReview } from '@bookhaven/api-client';
import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { RecentReviews } from './recent-reviews';

const FANTLAB = 'https://fantlab.ru/images/editions/big/16469';

function review(coverUrl: string): HomeReview {
  return {
    id: 'r1',
    rating: 5,
    excerpt: 'Отлично',
    createdAt: '2026-01-02T10:00:00Z',
    book: { slug: 'khobbit', title: 'Хоббит', authors: [], coverUrl },
    user: { name: 'Аня', avatarUrl: null },
  } as unknown as HomeReview;
}

describe('RecentReviews cover', () => {
  it('hotlinks a FantLab cover: the src is the link itself, not the optimizer', () => {
    const { container } = render(<RecentReviews reviews={[review(FANTLAB)]} />);
    expect(container.querySelector('img')?.getAttribute('src')).toBe(FANTLAB);
  });

  it('sends other covers through the optimizer', () => {
    const { container } = render(
      <RecentReviews reviews={[review('https://covers.openlibrary.org/b/id/1-L.jpg')]} />,
    );
    expect(container.querySelector('img')?.getAttribute('src')).toContain('/_next/image');
  });
});
