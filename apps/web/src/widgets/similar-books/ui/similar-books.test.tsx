import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { Book } from '@bookhaven/api-client';
import type { ImgHTMLAttributes } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SimilarBooks } from './similar-books';

const similarMock = vi.fn();

vi.mock('next/image', () => ({
  // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
  default: ({ fill: _fill, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean }) => <img {...props} />,
}));

// Replace only the API call.
vi.mock('@/entities/book', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/entities/book')>();
  return {
    ...actual,
    booksApi: {
      ...actual.booksApi,
      similar: (id: string, params?: { limit?: number }) => similarMock(id, params),
    },
  };
});

function makeBook(n: number): Book {
  return {
    id: `b${n}`,
    title: `Книга ${n}`,
    slug: `book-${n}`,
    description: null,
    coverUrl: n === 1 ? 'https://example.com/c1.jpg' : null,
    isbn: null,
    publishedYear: 2000,
    pageCount: null,
    language: '',
    rating: 4.2,
    ratingsCount: 10,
    reviewsCount: 0,
    authors: [
      {
        id: `a${n}`,
        name: `Автор ${n}`,
        slug: `author-${n}`,
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
}

function renderSimilar() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <SimilarBooks bookId="src-id" />
    </QueryClientProvider>
  );
}

describe('SimilarBooks', () => {
  afterEach(() => {
    similarMock.mockReset();
  });

  it('shows a skeleton while pending', () => {
    similarMock.mockReturnValue(new Promise(() => {}));
    renderSimilar();
    expect(screen.getByRole('heading', { name: 'Похожие книги' })).toBeInTheDocument();
    expect(screen.getAllByTestId('similar-books-skeleton')).toHaveLength(4);
    expect(screen.getByRole('region')).toHaveAttribute('aria-busy', 'true');
  });

  it('renders the heading and a card link per book, requesting limit 8', async () => {
    similarMock.mockResolvedValue({ items: [makeBook(1), makeBook(2), makeBook(3)] });
    renderSimilar();

    await waitFor(() => expect(screen.getAllByRole('link')).toHaveLength(3));
    expect(screen.getByRole('heading', { name: 'Похожие книги' })).toBeInTheDocument();
    const hrefs = screen.getAllByRole('link').map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/books/book-1', '/books/book-2', '/books/book-3']);
    expect(screen.getByRole('img', { name: 'Книга 1' })).toHaveAttribute('sizes', '112px');
    expect(screen.queryByTestId('similar-books-skeleton')).not.toBeInTheDocument();
    const list = screen.getByRole('list');
    expect(list).toHaveAttribute('tabindex', '0');
    expect(list).toHaveAccessibleName('Похожие книги');
    expect(similarMock).toHaveBeenCalledWith('src-id', { limit: 8 });
  });

  it('renders nothing for an empty result', async () => {
    similarMock.mockResolvedValue({ items: [] });
    const { container } = renderSimilar();
    await waitFor(() => expect(similarMock).toHaveBeenCalled());
    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });

  it('renders nothing on error', async () => {
    similarMock.mockRejectedValue(new Error('boom'));
    const { container } = renderSimilar();
    await waitFor(() => expect(similarMock).toHaveBeenCalled());
    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });
});
