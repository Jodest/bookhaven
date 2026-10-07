import { render, screen } from '@testing-library/react';
import type { Author } from '@bookhaven/api-client';
import type { ImgHTMLAttributes } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { AuthorDetailView } from './author-detail-view';

const getAuthor = vi.fn();

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  authorsApi: { get: (slug: string) => getAuthor(slug) },
}));

vi.mock('@/entities/book', () => ({ BookCard: () => null }));

vi.mock('next/image', () => ({
  default: ({
    fill: _fill,
    priority: _priority,
    unoptimized,
    ...props
  }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean; priority?: boolean; unoptimized?: boolean }) => (
    // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
    <img {...props} data-unoptimized={unoptimized ? 'true' : 'false'} />
  ),
}));

vi.mock('next/navigation', () => ({ notFound: vi.fn() }));

const base: Author = {
  id: 'a1',
  name: 'Лев Толстой',
  slug: 'lev-tolstoy',
  biography: null,
  photoUrl: 'https://upload.wikimedia.org/wikipedia/commons/thumb/c/c6/T.jpg/500px-T.jpg',
  birthDate: '1828-09-09',
  deathDate: '1910-11-20',
  createdAt: '',
  updatedAt: '',
  books: [],
};

describe('AuthorDetailView', () => {
  it('renders the Commons attribution and an unoptimized Wikimedia photo', async () => {
    getAuthor.mockResolvedValue({
      ...base,
      photoAttribution: {
        author: 'Sergei Prokudin-Gorskii',
        license: 'Public domain',
        licenseUrl: null,
        sourceUrl: 'https://commons.wikimedia.org/wiki/File:T.jpg',
      },
    });

    render(await AuthorDetailView({ slug: 'lev-tolstoy' }));

    expect(screen.getByText(/Фото: Sergei Prokudin-Gorskii, Public domain/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Wikimedia Commons' })).toHaveAttribute(
      'href',
      'https://commons.wikimedia.org/wiki/File:T.jpg',
    );
    expect(screen.getByRole('img', { name: 'Лев Толстой' })).toHaveAttribute('data-unoptimized', 'true');
  });

  it('renders no attribution without one and keeps other photos optimized', async () => {
    getAuthor.mockResolvedValue({ ...base, photoUrl: 'https://covers.openlibrary.org/a/id/1-L.jpg', photoAttribution: null });

    render(await AuthorDetailView({ slug: 'lev-tolstoy' }));

    expect(screen.queryByText(/Фото:/)).not.toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Лев Толстой' })).toHaveAttribute('data-unoptimized', 'false');
  });
});
