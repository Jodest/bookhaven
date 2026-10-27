import { render, screen } from '@testing-library/react';
import type { Author } from '@bookhaven/api-client';
import type { ImgHTMLAttributes } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AuthorDetailView } from './author-detail-view';

const getAuthor = vi.fn();
const getFilms = vi.fn();

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  authorsApi: { get: (slug: string) => getAuthor(slug) },
}));

vi.mock('@/entities/screen-work', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/screen-work')>()),
  screenWorksApi: { forAuthor: (id: string) => getFilms(id) },
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
  beforeEach(() => {
    getFilms.mockReset();
    getFilms.mockResolvedValue({ items: [], total: 0 });
  });

  it('renders the films block and requests films by author id', async () => {
    getAuthor.mockResolvedValue(base);
    getFilms.mockResolvedValue({
      items: [
        {
          id: 'w1',
          title: 'Толкин',
          original_title: null,
          kind: 'film',
          relation: 'biopic',
          year: 2019,
          director: null,
          imdb_url: null,
          kinopoisk_url: null,
          poster: null,
        },
      ],
      total: 1,
    });

    render(await AuthorDetailView({ slug: 'lev-tolstoy' }));

    expect(getFilms).toHaveBeenCalledWith('a1');
    expect(screen.getByRole('heading', { name: 'Фильмы об авторе' })).toBeInTheDocument();
    expect(screen.getByText('Толкин')).toBeInTheDocument();
  });

  it('renders the page without the films block when the films request fails', async () => {
    getAuthor.mockResolvedValue(base);
    getFilms.mockRejectedValue(new Error('boom'));

    render(await AuthorDetailView({ slug: 'lev-tolstoy' }));

    expect(screen.getByRole('heading', { name: 'Лев Толстой' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Фильмы об авторе' })).not.toBeInTheDocument();
  });

  it('renders no films block for an empty list', async () => {
    getAuthor.mockResolvedValue(base);

    render(await AuthorDetailView({ slug: 'lev-tolstoy' }));

    expect(screen.queryByRole('heading', { name: 'Фильмы об авторе' })).not.toBeInTheDocument();
  });

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
