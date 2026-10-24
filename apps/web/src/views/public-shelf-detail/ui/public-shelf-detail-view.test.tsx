import { render, screen } from '@testing-library/react';
import type { ImgHTMLAttributes } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PublicShelfDetailView } from './public-shelf-detail-view';

const getPublicMock = vi.fn();

vi.mock('@/entities/shelf', () => ({
  shelvesApi: { getPublic: (...args: unknown[]) => getPublicMock(...args) },
}));

vi.mock('next/image', () => ({
  default: ({ fill: _fill, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean }) => (
    // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
    <img {...props} />
  ),
}));

const book = (id: string, title: string, readsCount: number) => ({
  id,
  slug: id,
  title,
  coverUrl: null,
  authors: [],
  genres: [],
  readsCount,
  lastReadAt: readsCount > 0 ? '2025-05-15T12:00:00Z' : null,
});

describe('PublicShelfDetailView', () => {
  beforeEach(() => getPublicMock.mockReset());

  it('titles the abandoned shelf and shows the owner read counter', async () => {
    getPublicMock.mockResolvedValue({
      shelf: { id: 's1', userId: 'u2', name: 'Брошено', type: 'abandoned', description: '', isPublic: true },
      items: [book('b1', 'Дюна', 1), book('b2', 'Эмма', 0)],
      total: 2,
      page: 1,
      limit: 12,
    });
    render(<PublicShelfDetailView id="s1" />);

    expect(await screen.findByRole('heading', { level: 1, name: 'Брошено' })).toBeInTheDocument();
    expect(screen.getByText('Прочитано 1 раз')).toBeInTheDocument();
    expect(screen.getAllByText(/Прочитано \d/)).toHaveLength(1);
  });
});
