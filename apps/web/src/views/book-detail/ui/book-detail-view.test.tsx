import { render, screen } from '@testing-library/react';
import type { Book } from '@bookhaven/api-client';
import type { ImgHTMLAttributes } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { BookDetailView } from './book-detail-view';

const getBook = vi.fn();

vi.mock('@/entities/book', () => ({
  booksApi: { get: (slug: string) => getBook(slug) },
}));

vi.mock('next/image', () => ({
  // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
  default: ({ fill: _fill, priority: _priority, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean; priority?: boolean }) => <img {...props} />,
}));

vi.mock('next/navigation', () => ({ notFound: vi.fn() }));
vi.mock('@/features/add-to-list', () => ({ AddToListButton: () => null }));
vi.mock('@/features/rate-book', () => ({ BookRating: () => null }));
vi.mock('@/features/shelve-book', () => ({ ShelveBookButtons: () => null }));
vi.mock('@/widgets/reviews-section', () => ({ ReviewsSection: () => null }));
vi.mock('@/widgets/similar-books', () => ({ SimilarBooks: () => null }));

const author = {
  biography: null,
  photoUrl: null,
  birthDate: null,
  deathDate: null,
  createdAt: '',
  updatedAt: '',
};

const book: Book = {
  id: 'b1',
  title: 'Отцы и дети',
  slug: 'fathers-and-sons',
  description: null,
  coverUrl: null,
  isbn: null,
  publishedYear: 1862,
  pageCount: null,
  language: 'ru',
  rating: 0,
  ratingsCount: 0,
  reviewsCount: 0,
  authors: [
    { ...author, id: 'a1', name: 'Иван Тургенев', slug: 'ivan-turgenev' },
    { ...author, id: 'a2', name: 'Неизвестный автор', slug: null },
  ],
  genres: [],
  createdAt: '',
  updatedAt: '',
};

describe('BookDetailView', () => {
  it('links published authors and renders non-published ones as plain text', async () => {
    getBook.mockResolvedValue(book);

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.getByRole('link', { name: 'Иван Тургенев' })).toHaveAttribute('href', '/authors/ivan-turgenev');
    expect(screen.getByText('Неизвестный автор')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Неизвестный автор' })).not.toBeInTheDocument();
    expect(document.querySelector('a[href="/authors/null"]')).toBeNull();
  });
});
