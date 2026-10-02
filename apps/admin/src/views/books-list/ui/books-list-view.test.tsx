import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BooksListView } from './books-list-view';
import type { AdminBook } from '@bookhaven/api-client';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/books',
  useSearchParams: () => searchParamsValue,
}));

const listMock = vi.fn();
const updateMock = vi.fn();
const bulkPublishMock = vi.fn();
const genresListMock = vi.fn();
const authorsListMock = vi.fn();

vi.mock('@/entities/book', () => ({
  adminBooksApi: {
    list: (...args: unknown[]) => listMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
    bulkPublish: (...args: unknown[]) => bulkPublishMock(...args),
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
    is_hidden: false,
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
    updateMock.mockReset();
    bulkPublishMock.mockReset();
    genresListMock.mockReset();
    authorsListMock.mockReset();
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    updateMock.mockResolvedValue(makeBook());
    bulkPublishMock.mockResolvedValue({ processed: 0, published: 0, errors: [] });
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

  it('renders the pending_review filter option and updates the URL when selected', async () => {
    renderView();

    await waitFor(() => expect(listMock).toHaveBeenCalled());

    const option = screen.getByRole('option', {
      name: 'Импортировано (на проверке)',
    });
    expect(option).toBeInTheDocument();
    const select = option.closest('select') as HTMLSelectElement;

    fireEvent.change(select, { target: { value: 'pending_review' } });

    await waitFor(() =>
      expect(pushMock).toHaveBeenCalledWith(
        expect.stringContaining('visibility=pending_review')
      )
    );
  });

  it('shows an "Импортировано" badge when external_source is present', async () => {
    listMock.mockResolvedValue({
      items: [
        makeBook({
          id: 'imported-1',
          title: 'Импортированная книга',
          is_hidden: true,
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

  it('publishes a hidden imported book after confirmation, reusing full-replace update with is_hidden: false', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const book = makeBook({
      id: 'imported-2',
      title: 'На проверке',
      is_hidden: true,
      external_source: 'wikidata',
      authors: [{ id: 'a1', name: 'Автор', slug: 'a1', biography: null, photo_url: null, birth_date: null, death_date: null }],
      genres: [{ id: 'g1', name: 'Жанр', slug: 'g1' }],
    });
    listMock.mockResolvedValue({ items: [book], total: 1, page: 1, limit: 20 });

    renderView();

    const publishButton = await screen.findByRole('button', { name: 'Опубликовать' });
    fireEvent.click(publishButton);

    expect(confirmSpy).toHaveBeenCalled();

    await waitFor(() =>
      expect(updateMock).toHaveBeenCalledWith(
        'imported-2',
        expect.objectContaining({
          is_hidden: false,
          author_ids: ['a1'],
          genre_ids: ['g1'],
        })
      )
    );
  });

  it('does not publish when the confirmation is cancelled', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    const book = makeBook({
      id: 'imported-3',
      title: 'Отменить публикацию',
      is_hidden: true,
      external_source: 'wikidata',
    });
    listMock.mockResolvedValue({ items: [book], total: 1, page: 1, limit: 20 });

    renderView();

    const publishButton = await screen.findByRole('button', { name: 'Опубликовать' });
    fireEvent.click(publishButton);

    expect(updateMock).not.toHaveBeenCalled();
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

  describe('bulk publish', () => {
    it('does not render the "Опубликовать все" button when the pending_review filter is not active, even with a nonzero total', async () => {
      listMock.mockResolvedValue({
        items: [makeBook()],
        total: 5,
        page: 1,
        limit: 20,
      });

      renderView();

      await waitFor(() => expect(listMock).toHaveBeenCalled());
      expect(
        screen.queryByRole('button', { name: /Опубликовать все/ })
      ).not.toBeInTheDocument();
    });

    it('does not render the button when pending_review is active but the total is zero', async () => {
      searchParamsValue = new URLSearchParams('visibility=pending_review');
      listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });

      renderView();

      await waitFor(() => expect(listMock).toHaveBeenCalled());
      expect(
        screen.queryByRole('button', { name: /Опубликовать все/ })
      ).not.toBeInTheDocument();
    });

    it('renders "Опубликовать все (N)" with the already-fetched total when pending_review is active and total > 0', async () => {
      searchParamsValue = new URLSearchParams('visibility=pending_review');
      listMock.mockResolvedValue({
        items: [
          makeBook({ id: 'p1', is_hidden: true, external_source: 'wikidata' }),
        ],
        total: 7,
        page: 1,
        limit: 20,
      });

      renderView();

      expect(
        await screen.findByRole('button', { name: 'Опубликовать все (7)' })
      ).toBeInTheDocument();
    });

    it('confirms, then calls bulkPublish with the current search value, then shows a success summary', async () => {
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
      searchParamsValue = new URLSearchParams('visibility=pending_review&search=Толстой');
      listMock.mockResolvedValue({
        items: [
          makeBook({ id: 'p1', is_hidden: true, external_source: 'wikidata' }),
        ],
        total: 3,
        page: 1,
        limit: 20,
      });
      bulkPublishMock.mockResolvedValue({ processed: 3, published: 3, errors: [] });

      renderView();

      const button = await screen.findByRole('button', { name: 'Опубликовать все (3)' });
      fireEvent.click(button);

      expect(confirmSpy).toHaveBeenCalled();
      await waitFor(() =>
        expect(bulkPublishMock).toHaveBeenCalledWith({ search: 'Толстой' })
      );
      await waitFor(() =>
        expect(screen.getByText('Опубликовано 3 из 3.')).toBeInTheDocument()
      );
    });

    it('does not call bulkPublish when the confirmation is cancelled', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(false);
      searchParamsValue = new URLSearchParams('visibility=pending_review');
      listMock.mockResolvedValue({
        items: [
          makeBook({ id: 'p1', is_hidden: true, external_source: 'wikidata' }),
        ],
        total: 2,
        page: 1,
        limit: 20,
      });

      renderView();

      const button = await screen.findByRole('button', { name: 'Опубликовать все (2)' });
      fireEvent.click(button);

      expect(bulkPublishMock).not.toHaveBeenCalled();
    });

    it('shows an error summary with a sample of row errors when the report contains failures', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      searchParamsValue = new URLSearchParams('visibility=pending_review');
      listMock.mockResolvedValue({
        items: [
          makeBook({ id: 'p1', is_hidden: true, external_source: 'wikidata' }),
        ],
        total: 2,
        page: 1,
        limit: 20,
      });
      bulkPublishMock.mockResolvedValue({
        processed: 2,
        published: 1,
        errors: [{ book_id: 'p2', title: 'Плохая книга', message: 'slug already exists' }],
      });

      renderView();

      const button = await screen.findByRole('button', { name: 'Опубликовать все (2)' });
      fireEvent.click(button);

      await waitFor(() =>
        expect(screen.getByText(/Опубликовано 1 из 2\. Ошибок: 1/)).toBeInTheDocument()
      );
    });
  });
});
