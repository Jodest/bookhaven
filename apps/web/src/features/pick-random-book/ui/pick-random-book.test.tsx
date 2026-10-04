import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { Book } from '@bookhaven/api-client';
import type { ImgHTMLAttributes } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PickRandomBook } from './pick-random-book';

const randomMock = vi.fn();

vi.mock('next/image', () => ({
  // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
  default: ({ fill: _fill, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean }) => <img {...props} />,
}));

vi.mock('@/entities/home', () => ({
  homeApi: {
    random: (params?: unknown) => randomMock(params),
  },
}));

function makeBook(id: string, title: string): Book {
  return {
    id,
    title,
    slug: id,
    description: null,
    coverUrl: null,
    isbn: null,
    publishedYear: 2000,
    pageCount: null,
    language: '',
    rating: 4.5,
    ratingsCount: 3,
    reviewsCount: 0,
    authors: [],
    genres: [],
    createdAt: '',
    updatedAt: '',
  };
}

function renderPicker() {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PickRandomBook genres={[{ id: 'g1', name: 'Классика' }, { id: 'g2', name: 'Роман' }]} />
    </QueryClientProvider>
  );
}

describe('PickRandomBook', () => {
  afterEach(() => {
    randomMock.mockReset();
  });

  it('calls random with the selected genre and shows the book', async () => {
    randomMock.mockResolvedValue({ book: makeBook('b1', 'Первая книга') });
    renderPicker();

    fireEvent.change(screen.getByLabelText('Жанр'), { target: { value: 'g2' } });
    fireEvent.click(screen.getByRole('button', { name: 'Подобрать' }));

    expect(await screen.findByRole('link', { name: 'Первая книга' })).toHaveAttribute('href', '/books/b1');
    expect(randomMock).toHaveBeenCalledWith({ genreId: 'g2', excludeId: undefined });
  });

  it('"Ещё вариант" passes excludeId of the shown book', async () => {
    randomMock
      .mockResolvedValueOnce({ book: makeBook('b1', 'Первая книга') })
      .mockResolvedValueOnce({ book: makeBook('b2', 'Вторая книга') });
    renderPicker();

    fireEvent.click(screen.getByRole('button', { name: 'Подобрать' }));
    await screen.findByText('Первая книга');
    fireEvent.click(screen.getByRole('button', { name: 'Ещё вариант' }));

    expect(await screen.findByText('Вторая книга')).toBeInTheDocument();
    expect(randomMock).toHaveBeenLastCalledWith({ genreId: undefined, excludeId: 'b1' });
  });

  it('shows a message when nothing is found', async () => {
    randomMock.mockResolvedValue({ book: null });
    renderPicker();

    fireEvent.click(screen.getByRole('button', { name: 'Подобрать' }));
    expect(await screen.findByText('Не нашли подходящую книгу.')).toBeInTheDocument();
  });

  it('shows an inline error on failure', async () => {
    randomMock.mockRejectedValue(new Error('boom'));
    renderPicker();

    fireEvent.click(screen.getByRole('button', { name: 'Подобрать' }));
    await waitFor(() =>
      expect(screen.getByText('Не удалось подобрать книгу. Попробуйте ещё раз.')).toBeInTheDocument()
    );
  });
});
