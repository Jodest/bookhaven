import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AdminAuthor, AdminBook, ApiError } from '@bookhaven/api-client';
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

const authorsListMock = vi.fn(() => Promise.resolve({ items: [] as AdminAuthor[], total: 0, page: 1, limit: 100 }));

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    list: () => authorsListMock(),
  },
}));

vi.mock('@/entities/genre', () => ({
  adminGenresApi: {
    list: () => Promise.resolve({ items: [], total: 0, page: 1, limit: 100 }),
  },
}));

function renderForm(
  initial?: AdminBook,
  onSubmit: ReturnType<typeof vi.fn> = vi.fn().mockResolvedValue(undefined)
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <BookForm initial={initial} onSubmit={onSubmit} />
    </QueryClientProvider>
  );
  return { onSubmit, ...utils };
}

const latinBook: AdminBook = {
  id: 'b1',
  title: 'Empire V',
  slug: 'empire-v',
  description: null,
  cover_url: null,
  isbn: null,
  published_year: 2006,
  page_count: null,
  language: 'ru',
  rating: 0,
  ratings_count: 0,
  reviews_count: 0,
  is_hidden: false,
  external_source: null,
  authors: [],
  genres: [],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

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

  it('does not crash when the lookup response has null authors/genres', async () => {
    // Regression test: production crashed with "Cannot read properties of
    // null (reading 'length')" at this file's genres.length check when the
    // backend's lookup response had authors/genres as null instead of []
    // (a real bug in the Open Library search-doc mapping, now fixed
    // server-side too - this guards the frontend independently in case a
    // future source client reintroduces the same nil-slice mistake).
    lookupMock.mockResolvedValue({
      title: 'Земля обетованная',
      authors: null,
      genres: null,
      source_name: 'openlibrary',
      source_id: 'OL1',
    });

    renderForm();

    fireEvent.change(screen.getByLabelText('Название *'), {
      target: { value: 'Земля обетованная' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Найти по ISBN/названию' }));

    await waitFor(() => expect(lookupMock).toHaveBeenCalled());

    // Renders without throwing; the "from search result" hints are simply
    // absent since there's nothing to show.
    await waitFor(() =>
      expect((screen.getByLabelText('Название *') as HTMLInputElement).value).toBe(
        'Земля обетованная'
      )
    );
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

describe('BookForm Latin-script title opt-in', () => {
  const checkboxLabel = 'Название на латинице допустимо';

  beforeEach(() => {
    pushMock.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders the checkbox unchecked in create mode', () => {
    renderForm();
    expect((screen.getByLabelText(checkboxLabel) as HTMLInputElement).checked).toBe(false);
  });

  it('renders the checkbox unchecked in edit mode, even for a Latin title', () => {
    renderForm(latinBook);
    expect((screen.getByLabelText(checkboxLabel) as HTMLInputElement).checked).toBe(false);
  });

  it('submits allow_non_russian_title: true when the checkbox is ticked', async () => {
    const { onSubmit } = renderForm();

    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Empire V' } });
    fireEvent.click(screen.getByLabelText(checkboxLabel));
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalled());
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      title: 'Empire V',
      allow_non_russian_title: true,
    });
  });

  it('shows a Russian hint pointing at the checkbox on a 422 "title must be in Russian"', async () => {
    const onSubmit = vi
      .fn()
      .mockRejectedValue(new ApiError(422, 'Unprocessable Entity', 'title must be in Russian'));
    renderForm(undefined, onSubmit);

    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Empire V' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() =>
      expect(
        screen.getByText(
          'Название должно быть на русском языке. Если официальное название книги пишется латиницей, отметьте «Название на латинице допустимо».'
        )
      ).toBeInTheDocument()
    );
    expect(screen.queryByText('title must be in Russian')).not.toBeInTheDocument();
  });

  it('shows other 422 messages as is', async () => {
    const onSubmit = vi
      .fn()
      .mockRejectedValue(
        new ApiError(422, 'Unprocessable Entity', 'description must be in Russian')
      );
    renderForm(undefined, onSubmit);

    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Книга' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() =>
      expect(screen.getByText('description must be in Russian')).toBeInTheDocument()
    );
  });
});

describe('BookForm author picker status', () => {
  const author = (id: string, name: string, status: AdminAuthor['status']): AdminAuthor => ({
    id,
    name,
    slug: id,
    biography: null,
    photo_url: null,
    birth_date: null,
    death_date: null,
    status,
    external_source: null,
    external_id: null,
  });

  afterEach(() => {
    authorsListMock.mockReset();
    authorsListMock.mockImplementation(() =>
      Promise.resolve({ items: [] as AdminAuthor[], total: 0, page: 1, limit: 100 })
    );
  });

  it('marks non-published authors in the picker and badges selected ones', async () => {
    const pending = author('a2', 'Ожидающий Автор', 'pending');
    authorsListMock.mockResolvedValue({
      items: [author('a1', 'Опубликованный Автор', 'published'), pending, author('a3', 'Отклонённый Автор', 'rejected')],
      total: 3,
      page: 1,
      limit: 100,
    });

    renderForm({ ...latinBook, authors: [pending] });

    await waitFor(() =>
      expect(screen.getByRole('option', { name: 'Ожидающий Автор · На модерации' })).toBeInTheDocument()
    );
    expect(screen.getByRole('option', { name: 'Опубликованный Автор' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Отклонённый Автор · Отклонён' })).toBeInTheDocument();

    const selectedList = screen.getByRole('list', { name: 'Неопубликованные авторы' });
    expect(selectedList).toHaveTextContent('На модерации');
    expect(selectedList).toHaveTextContent('Ожидающий Автор');
    expect(selectedList).not.toHaveTextContent('Опубликованный Автор');
  });
});
