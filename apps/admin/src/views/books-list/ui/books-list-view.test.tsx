import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BooksListView } from './books-list-view';
import type { AdminBook } from '@bookhaven/api-client';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/catalog/books',
  useSearchParams: () => searchParamsValue,
}));

const listMock = vi.fn();
const genresListMock = vi.fn();
const authorsListMock = vi.fn();

vi.mock('@/entities/book', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/book')>()),
  adminBooksApi: {
    list: (...args: unknown[]) => listMock(...args),
  },
}));

vi.mock('@/entities/genre', () => ({
  adminGenresApi: {
    list: (...args: unknown[]) => genresListMock(...args),
  },
}));

vi.mock('@/entities/author', () => ({
  adminAuthorsApi: {
    list: (...args: unknown[]) => authorsListMock(...args),
  },
}));

function makeBook(overrides: Partial<AdminBook> = {}): AdminBook {
  return {
    id: 'book-1',
    title: 'Тестовая книга',
    slug: 'test-book',
    description: null,
    cover_url: null,
    isbn: null,
    published_year: null,
    page_count: null,
    language: 'ru',
    rating: 0,
    ratings_count: 0,
    reviews_count: 0,
    status: 'published',
    external_source: null,
    authors: [],
    genres: [],
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <BooksListView />
    </QueryClientProvider>
  );
}

describe('BooksListView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    listMock.mockReset();
    genresListMock.mockReset();
    authorsListMock.mockReset();
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    genresListMock.mockResolvedValue({
      items: [{ id: 'g1', name: 'Фантастика', slug: 'fantastika' }],
      total: 1,
      page: 1,
      limit: 100,
    });
    authorsListMock.mockResolvedValue({
      items: [
        {
          id: 'a1',
          name: 'Автор',
          slug: 'avtor',
          biography: null,
          photo_url: null,
          birth_date: null,
          death_date: null,
        },
      ],
      total: 1,
      page: 1,
      limit: 100,
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('status filter options are Все статусы / Опубликована / На модерации / Отклонена', async () => {
    searchParamsValue = new URLSearchParams('page=3');
    renderView();

    const select = (await screen.findByLabelText('Статус')) as HTMLSelectElement;
    expect(Array.from(select.options).map((o) => o.textContent)).toEqual([
      'Все статусы',
      'Опубликована',
      'На модерации',
      'Отклонена',
    ]);

    fireEvent.change(select, { target: { value: 'rejected' } });
    await waitFor(() =>
      expect(pushMock).toHaveBeenCalledWith(expect.stringContaining('status=rejected'))
    );
    expect(pushMock).toHaveBeenCalledWith(expect.stringContaining('page=1'));
  });

  it('sends ?status= to the API', async () => {
    searchParamsValue = new URLSearchParams('status=pending');
    renderView();

    await waitFor(() => expect(listMock).toHaveBeenCalled());
    expect(listMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'pending' }));
    expect((screen.getByLabelText('Статус') as HTMLSelectElement).value).toBe('pending');
  });

  it('ignores an invalid status and the legacy visibility param', async () => {
    searchParamsValue = new URLSearchParams('status=hidden&visibility=hidden');
    renderView();

    await waitFor(() => expect(listMock).toHaveBeenCalled());
    const args = listMock.mock.calls[0][0];
    expect(args.status).toBe('');
    expect(args).not.toHaveProperty('visibility');
    expect((screen.getByLabelText('Статус') as HTMLSelectElement).value).toBe('');
  });

  it('shows a status badge per row', async () => {
    listMock.mockResolvedValue({
      items: [
        makeBook({ id: 'b1', title: 'Опубликованная', status: 'published' }),
        makeBook({ id: 'b2', title: 'Ожидающая', status: 'pending' }),
        makeBook({ id: 'b3', title: 'Отклонённая', status: 'rejected' }),
      ],
      total: 3,
      page: 1,
      limit: 20,
    });
    renderView();

    await waitFor(() => expect(screen.getByText('Ожидающая')).toBeInTheDocument());
    const rowOf = (title: string) => screen.getByText(title).closest('tr') as HTMLElement;
    expect(rowOf('Опубликованная')).toHaveTextContent('Опубликована');
    expect(rowOf('Ожидающая')).toHaveTextContent('На модерации');
    expect(rowOf('Отклонённая')).toHaveTextContent('Отклонена');
  });

  it('header has "Добавить книгу" to the catalog and no "Импортировать" link', async () => {
    renderView();

    await waitFor(() => expect(listMock).toHaveBeenCalled());
    expect(screen.getByRole('link', { name: 'Добавить книгу' })).toHaveAttribute(
      'href',
      '/catalog/books/new'
    );
    expect(screen.queryByRole('link', { name: 'Импортировать' })).not.toBeInTheDocument();
  });

  it('shows an "Импортировано" badge when external_source is present', async () => {
    listMock.mockResolvedValue({
      items: [
        makeBook({
          id: 'imported-1',
          title: 'Импортированная книга',
          status: 'pending',
          external_source: 'wikidata',
        }),
      ],
      total: 1,
      page: 1,
      limit: 20,
    });

    renderView();

    await waitFor(() => expect(screen.getByText('Импортированная книга')).toBeInTheDocument());
    expect(screen.getByText('Импортировано', { selector: 'span' })).toBeInTheDocument();
  });

  it('has no publish actions, even for a hidden imported row; edit links to the catalog', async () => {
    listMock.mockResolvedValue({
      items: [
        makeBook({ id: 'imported-2', title: 'На проверке', status: 'pending', external_source: 'wikidata' }),
      ],
      total: 1,
      page: 1,
      limit: 20,
    });

    renderView();

    const edit = await screen.findByRole('link', { name: 'Редактировать' });
    expect(edit).toHaveAttribute('href', '/catalog/books/imported-2/edit');
    expect(screen.queryByRole('button', { name: /Опубликовать/ })).not.toBeInTheDocument();
  });

  describe('genre/author/source filters', () => {
    it('selecting a genre updates the URL with genre_id and resets page to 1', async () => {
      searchParamsValue = new URLSearchParams('page=2');
      renderView();

      const option = await screen.findByRole('option', { name: 'Фантастика' });
      const select = option.closest('select') as HTMLSelectElement;
      fireEvent.change(select, { target: { value: 'g1' } });

      await waitFor(() =>
        expect(pushMock).toHaveBeenCalledWith(
          expect.stringContaining('genre_id=g1')
        )
      );
      expect(pushMock).toHaveBeenCalledWith(expect.stringContaining('page=1'));
    });

    it('selecting an author updates the URL with author_id and resets page to 1', async () => {
      searchParamsValue = new URLSearchParams('page=2');
      renderView();

      const option = await screen.findByRole('option', { name: 'Автор' });
      const select = option.closest('select') as HTMLSelectElement;
      fireEvent.change(select, { target: { value: 'a1' } });

      await waitFor(() =>
        expect(pushMock).toHaveBeenCalledWith(
          expect.stringContaining('author_id=a1')
        )
      );
      expect(pushMock).toHaveBeenCalledWith(expect.stringContaining('page=1'));
    });

    it('selecting a source updates the URL and passes it through to adminBooksApi.list', async () => {
      renderView();

      const option = await screen.findByRole('option', { name: 'Добавлено вручную' });
      const sourceSelect = option.closest('select') as HTMLSelectElement;
      fireEvent.change(sourceSelect, { target: { value: 'harvested' } });

      await waitFor(() =>
        expect(pushMock).toHaveBeenCalledWith(
          expect.stringContaining('source=harvested')
        )
      );
    });

    it('passes genre_id/author_id/source from the URL through to adminBooksApi.list', async () => {
      searchParamsValue = new URLSearchParams('genre_id=g1&author_id=a1&source=manual');
      renderView();

      await waitFor(() =>
        expect(listMock).toHaveBeenCalledWith(
          expect.objectContaining({
            genreId: 'g1',
            authorId: 'a1',
            source: 'manual',
          })
        )
      );
    });

    it('clearing a filter removes the corresponding query param', async () => {
      searchParamsValue = new URLSearchParams('genre_id=g1');
      renderView();

      const select = await screen.findByDisplayValue('Фантастика');
      fireEvent.change(select, { target: { value: '' } });

      await waitFor(() => expect(pushMock).toHaveBeenCalled());
      const lastCall = pushMock.mock.calls[pushMock.mock.calls.length - 1][0] as string;
      expect(lastCall).not.toContain('genre_id');
    });
  });
});
