import { render, screen } from '@testing-library/react';
import type { Book } from '@bookhaven/api-client';
import type { ImgHTMLAttributes } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { BookOfTheDay } from './book-of-the-day';

const shelveProps = vi.fn();

vi.mock('next/image', () => ({
  // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
  default: ({ fill: _fill, priority: _priority, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean; priority?: boolean }) => <img {...props} />,
}));

vi.mock('@/features/shelve-book', () => ({
  ShelveBookButtons: (props: Record<string, unknown>) => {
    shelveProps(props);
    return <div data-testid="shelve-buttons" />;
  },
}));

const book: Book = {
  id: 'b1',
  title: 'Отцы и дети',
  slug: 'fathers-and-sons',
  description: null,
  coverUrl: 'https://example.com/c.jpg',
  isbn: null,
  publishedYear: 1862,
  pageCount: null,
  language: '',
  rating: 4.6,
  ratingsCount: 12,
  reviewsCount: 0,
  authors: [
    {
      id: 'a1',
      name: 'Иван Тургенев',
      slug: 'ivan-turgenev',
      biography: null,
      photoUrl: null,
      birthDate: null,
      deathDate: null,
      createdAt: '',
      updatedAt: '',
    },
  ],
  genres: [],
  createdAt: '',
  updatedAt: '',
};

describe('BookOfTheDay', () => {
  it('renders nothing for null', () => {
    const { container } = render(<BookOfTheDay data={null} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders the book, rating, excerpt and a want-to-read-only toggle', () => {
    render(<BookOfTheDay data={{ book, descriptionExcerpt: 'Роман о конфликте поколений…' }} />);

    expect(screen.getByRole('heading', { name: 'Книга дня' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Отцы и дети' })).toHaveAttribute('href', '/books/fathers-and-sons');
    expect(screen.getByText('4,6 · 12 оценок')).toBeInTheDocument();
    expect(screen.getByText('Роман о конфликте поколений…')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Подробнее' })).toHaveAttribute('href', '/books/fathers-and-sons');
    expect(shelveProps).toHaveBeenCalledWith(
      expect.objectContaining({
        bookId: 'b1',
        types: ['want_to_read'],
        showCustomShelves: false,
        loginRedirect: '/',
      })
    );
  });

  it('hides the rating line without ratings', () => {
    render(<BookOfTheDay data={{ book: { ...book, rating: 0, ratingsCount: 0 }, descriptionExcerpt: null }} />);
    expect(screen.queryByText(/оцен/)).not.toBeInTheDocument();
  });
});
