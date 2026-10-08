import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BookModerationView } from './book-moderation-view';
import type { AdminBook } from '@bookhaven/api-client';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/moderation/books',
  useSearchParams: () => searchParamsValue,
}));

const listMock = vi.fn();
const updateMock = vi.fn();
const bulkPublishMock = vi.fn();

vi.mock('@/entities/book', () => ({
  adminBooksApi: {
    list: (...args: unknown[]) => listMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
    bulkPublish: (...args: unknown[]) => bulkPublishMock(...args),
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
    is_hidden: true,
    external_source: 'wikidata',
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
      <BookModerationView />
    </QueryClientProvider>
  );
}

describe('BookModerationView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    listMock.mockReset();
    updateMock.mockReset();
    bulkPublishMock.mockReset();
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    updateMock.mockResolvedValue(makeBook({ is_hidden: false }));
    bulkPublishMock.mockResolvedValue({ processed: 0, published: 0, errors: [] });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('lists pending_review books with search and page from the URL', async () => {
    searchParamsValue = new URLSearchParams('search=Толстой&page=2');
    renderView();

    await waitFor(() =>
      expect(listMock).toHaveBeenCalledWith({
        visibility: 'pending_review',
        search: 'Толстой',
        page: 2,
        limit: 20,
      })
    );
    expect(screen.getByRole('heading', { name: 'Книги на проверке' })).toBeInTheDocument();
  });

  it('shows the empty state', async () => {
    renderView();

    expect(await screen.findByText('Нет книг на проверке.')).toBeInTheDocument();
  });

  it('shows the error state', async () => {
    listMock.mockRejectedValue(new Error('boom'));
    renderView();

    expect(await screen.findByText('Не удалось загрузить книги.')).toBeInTheDocument();
  });

  it('search submit pushes ?search=…&page=1 on the current pathname', async () => {
    renderView();

    await waitFor(() => expect(listMock).toHaveBeenCalled());
    fireEvent.change(screen.getByPlaceholderText('Поиск по названию, ISBN, описанию'), {
      target: { value: 'Пушкин' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));

    expect(pushMock).toHaveBeenCalledWith(
      `/moderation/books?${new URLSearchParams({ search: 'Пушкин', page: '1' }).toString()}`
    );
  });

  it('row edit links to the catalog edit page', async () => {
    listMock.mockResolvedValue({
      items: [makeBook({ id: 'p1', title: 'На проверке' })],
      total: 1,
      page: 1,
      limit: 20,
    });
    renderView();

    const edit = await screen.findByRole('link', { name: 'Редактировать' });
    expect(edit).toHaveAttribute('href', '/catalog/books/p1/edit');
  });

  describe('row publish', () => {
    it('publishes after confirmation, reusing full-replace update with is_hidden: false', async () => {
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
      const book = makeBook({
        id: 'imported-2',
        title: 'На проверке',
        authors: [
          {
            id: 'a1',
            name: 'Автор',
            slug: 'a1',
            biography: null,
            photo_url: null,
            birth_date: null,
            death_date: null,
            status: 'published',
            external_source: null,
            external_id: null,
            photo_attribution: null,
          },
        ],
        genres: [{ id: 'g1', name: 'Жанр', slug: 'g1', status: 'published', wikidata_qid: null }],
      });
      listMock.mockResolvedValue({ items: [book], total: 1, page: 1, limit: 20 });

      renderView();

      const publishButton = await screen.findByRole('button', { name: 'Опубликовать' });
      fireEvent.click(publishButton);

      expect(confirmSpy).toHaveBeenCalledWith('Опубликовать книгу «На проверке»?');
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
      expect(await screen.findByText('Книга опубликована.')).toBeInTheDocument();
    });

    it('does not publish when the confirmation is cancelled', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(false);
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'imported-3', title: 'Отменить публикацию' })],
        total: 1,
        page: 1,
        limit: 20,
      });

      renderView();

      const publishButton = await screen.findByRole('button', { name: 'Опубликовать' });
      fireEvent.click(publishButton);

      expect(updateMock).not.toHaveBeenCalled();
    });
  });

  describe('bulk publish', () => {
    it('does not render the button when the total is zero', async () => {
      renderView();

      await screen.findByText('Нет книг на проверке.');
      expect(screen.queryByRole('button', { name: /Опубликовать все/ })).not.toBeInTheDocument();
    });

    it('renders "Опубликовать все (N)" with the already-fetched total', async () => {
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'p1' })],
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
      searchParamsValue = new URLSearchParams('search=Толстой');
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'p1' })],
        total: 3,
        page: 1,
        limit: 20,
      });
      bulkPublishMock.mockResolvedValue({ processed: 3, published: 3, errors: [] });

      renderView();

      const button = await screen.findByRole('button', { name: 'Опубликовать все (3)' });
      fireEvent.click(button);

      expect(confirmSpy).toHaveBeenCalledWith('Опубликовать все книги на проверке (3)?');
      await waitFor(() => expect(bulkPublishMock).toHaveBeenCalledWith({ search: 'Толстой' }));
      expect(await screen.findByText('Опубликовано 3 из 3.')).toBeInTheDocument();
    });

    it('passes search: undefined when no search is active', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'p1' })],
        total: 1,
        page: 1,
        limit: 20,
      });

      renderView();

      fireEvent.click(await screen.findByRole('button', { name: 'Опубликовать все (1)' }));
      await waitFor(() => expect(bulkPublishMock).toHaveBeenCalledWith({ search: undefined }));
    });

    it('does not call bulkPublish when the confirmation is cancelled', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(false);
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'p1' })],
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
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'p1' })],
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
