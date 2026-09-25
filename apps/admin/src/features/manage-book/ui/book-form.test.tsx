import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { BookForm } from './book-form';

const pushMock = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
}));

const lookupMock = vi.fn();

vi.mock('@/entities/book', () => ({
  adminBooksApi: {
    lookup: (...args: unknown[]) => lookupMock(...args),
  },
}));

vi.mock('@/entities/author', () => ({
  adminAuthorsApi: {
    list: () => Promise.resolve({ items: [], total: 0, page: 1, limit: 100 }),
  },
}));

vi.mock('@/entities/genre', () => ({
  adminGenresApi: {
    list: () => Promise.resolve({ items: [], total: 0, page: 1, limit: 100 }),
  },
}));

function renderForm() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <BookForm onSubmit={onSubmit} />
    </QueryClientProvider>
  );
  return { onSubmit, ...utils };
}

describe('BookForm lookup assistant', () => {
  beforeEach(() => {
    pushMock.mockReset();
    lookupMock.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('pre-fills scalar fields from a successful lookup, without touching author/genre selections', async () => {
    lookupMock.mockResolvedValue({
      title: 'Война и мир',
      authors: ['Лев Толстой'],
      genres: ['Роман'],
      isbn: '9785171067770',
      description: 'Эпический роман',
      cover_url: 'https://example.com/cover.jpg',
      published_year: 1869,
      page_count: 1225,
      language: 'ru',
      source_name: 'openlibrary',
      source_id: 'OL123',
    });

    renderForm();

    fireEvent.change(screen.getByLabelText('Название *'), {
      target: { value: 'Война и мир' },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Найти по ISBN/названию' }));

    await waitFor(() => expect(lookupMock).toHaveBeenCalledWith({ q: 'Война и мир' }));

    await waitFor(() =>
      expect((screen.getByLabelText('ISBN') as HTMLInputElement).value).toBe('9785171067770')
    );
    expect((screen.getByLabelText('Описание') as HTMLInputElement).value).toBe(
      'Эпический роман'
    );
    expect((screen.getByLabelText('Год издания') as HTMLInputElement).value).toBe('1869');
    expect((screen.getByLabelText('Количество страниц') as HTMLInputElement).value).toBe('1225');
    expect((screen.getByLabelText('URL обложки') as HTMLInputElement).value).toBe(
      'https://example.com/cover.jpg'
    );

    // Author/genre names are shown as read-only hints, never auto-selected.
    expect(screen.getByText(/Лев Толстой/)).toBeInTheDocument();
    expect(screen.getByText(/Роман/)).toBeInTheDocument();
    const authorSelect = screen.getAllByRole('listbox')[0] as HTMLSelectElement;
    expect(Array.from(authorSelect.selectedOptions)).toHaveLength(0);
  });

  it('uses the ISBN field over the title when both are filled in', async () => {
    lookupMock.mockResolvedValue({
      title: 'Anna Karenina',
      authors: [],
      genres: [],
      source_name: 'openlibrary',
      source_id: 'OL456',
    });

    renderForm();

    fireEvent.change(screen.getByLabelText('Название *'), {
      target: { value: 'Some Title' },
    });
    fireEvent.change(screen.getByLabelText('ISBN'), {
      target: { value: '9780000000000' },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Найти по ISBN/названию' }));

    await waitFor(() => expect(lookupMock).toHaveBeenCalledWith({ isbn: '9780000000000' }));
  });

  it('shows an inline message on a 404 miss and does not block manual entry', async () => {
    lookupMock.mockRejectedValue(new ApiError(404, 'Not Found', 'no matching book found'));

    renderForm();

    fireEvent.change(screen.getByLabelText('Название *'), {
      target: { value: 'Unknown Book' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Найти по ISBN/названию' }));

    await waitFor(() =>
      expect(screen.getByText('Ничего не найдено. Введите данные вручную.')).toBeInTheDocument()
    );

    // The title field remains editable/unaffected.
    fireEvent.change(screen.getByLabelText('Название *'), {
      target: { value: 'Manually typed title' },
    });
    expect((screen.getByLabelText('Название *') as HTMLInputElement).value).toBe(
      'Manually typed title'
    );
  });

  it('shows an inline message on a 502 source-unavailable error', async () => {
    lookupMock.mockRejectedValue(
      new ApiError(502, 'Bad Gateway', 'lookup source is unavailable; retry later or enter the data manually')
    );

    renderForm();

    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Book' } });
    fireEvent.click(screen.getByRole('button', { name: 'Найти по ISBN/названию' }));

    await waitFor(() =>
      expect(
        screen.getByText('lookup source is unavailable; retry later or enter the data manually')
      ).toBeInTheDocument()
    );
  });

  it('shows a local validation message when neither ISBN nor title is filled in', async () => {
    renderForm();

    fireEvent.click(screen.getByRole('button', { name: 'Найти по ISBN/названию' }));

    await waitFor(() =>
      expect(screen.getByText('Введите ISBN или название для поиска.')).toBeInTheDocument()
    );
    expect(lookupMock).not.toHaveBeenCalled();
  });
});
