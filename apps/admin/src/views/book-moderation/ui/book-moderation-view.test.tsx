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
const bulkStatusMock = vi.fn();

vi.mock('@/entities/book', () => ({
  adminBooksApi: {
    list: (...args: unknown[]) => listMock(...args),
    bulkStatus: (...args: unknown[]) => bulkStatusMock(...args),
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
    status: 'pending',
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
    bulkStatusMock.mockReset();
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    bulkStatusMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('lists status=pending books (all sources) with search and page from the URL', async () => {
    searchParamsValue = new URLSearchParams('search=Толстой&page=2');
    renderView();

    await waitFor(() =>
      expect(listMock).toHaveBeenCalledWith({
        status: 'pending',
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

  describe('row actions', () => {
    it('publishes after confirmation via bulkStatus([id], published)', async () => {
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'imported-2', title: 'На проверке' })],
        total: 1,
        page: 1,
        limit: 20,
      });

      renderView();

      fireEvent.click(await screen.findByRole('button', { name: 'Опубликовать' }));

      expect(confirmSpy).toHaveBeenCalledWith('Опубликовать книгу «На проверке»?');
      await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['imported-2'], 'published'));
      expect(await screen.findByText('Опубликовано 1 из 1.')).toBeInTheDocument();
    });

    it('rejects after confirmation via bulkStatus([id], rejected)', async () => {
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'manual-1', title: 'Ручная книга', external_source: null })],
        total: 1,
        page: 1,
        limit: 20,
      });

      renderView();

      fireEvent.click(await screen.findByRole('button', { name: 'Отклонить' }));

      expect(confirmSpy).toHaveBeenCalledWith('Отклонить книгу «Ручная книга»?');
      await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['manual-1'], 'rejected'));
      expect(await screen.findByText('Отклонено 1 из 1.')).toBeInTheDocument();
    });

    it('does nothing when the confirmation is cancelled', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(false);
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'imported-3', title: 'Отменить публикацию' })],
        total: 1,
        page: 1,
        limit: 20,
      });

      renderView();

      fireEvent.click(await screen.findByRole('button', { name: 'Опубликовать' }));
      fireEvent.click(screen.getByRole('button', { name: 'Отклонить' }));

      expect(bulkStatusMock).not.toHaveBeenCalled();
    });
  });

  describe('publish the page', () => {
    it('does not render the button when the page is empty', async () => {
      renderView();

      await screen.findByText('Нет книг на проверке.');
      expect(screen.queryByRole('button', { name: /Опубликовать все/ })).not.toBeInTheDocument();
    });

    it('renders "Опубликовать все на странице (N)" with the number of rows on the page', async () => {
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'p1' }), makeBook({ id: 'p2' })],
        total: 45,
        page: 1,
        limit: 20,
      });

      renderView();

      expect(
        await screen.findByRole('button', { name: 'Опубликовать все на странице (2)' })
      ).toBeInTheDocument();
    });

    it('confirms, then calls bulkStatus with the page ids, then shows a success summary', async () => {
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
      searchParamsValue = new URLSearchParams('search=Толстой');
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'p1' }), makeBook({ id: 'p2' }), makeBook({ id: 'p3' })],
        total: 3,
        page: 1,
        limit: 20,
      });
      bulkStatusMock.mockResolvedValue({ processed: 3, updated: 3, errors: [] });

      renderView();

      fireEvent.click(
        await screen.findByRole('button', { name: 'Опубликовать все на странице (3)' })
      );

      expect(confirmSpy).toHaveBeenCalledWith('Опубликовать все книги на этой странице (3)?');
      await waitFor(() =>
        expect(bulkStatusMock).toHaveBeenCalledWith(['p1', 'p2', 'p3'], 'published')
      );
      expect(await screen.findByText('Опубликовано 3 из 3.')).toBeInTheDocument();
    });

    it('does not call bulkStatus when the confirmation is cancelled', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(false);
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'p1' })],
        total: 1,
        page: 1,
        limit: 20,
      });

      renderView();

      fireEvent.click(
        await screen.findByRole('button', { name: 'Опубликовать все на странице (1)' })
      );

      expect(bulkStatusMock).not.toHaveBeenCalled();
    });

    it('shows an error summary with a sample of row errors when the report contains failures', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      listMock.mockResolvedValue({
        items: [makeBook({ id: 'p1' }), makeBook({ id: 'p2' })],
        total: 2,
        page: 1,
        limit: 20,
      });
      bulkStatusMock.mockResolvedValue({
        processed: 2,
        updated: 1,
        errors: [{ book_id: 'p2', title: '', message: 'book not found' }],
      });

      renderView();

      fireEvent.click(
        await screen.findByRole('button', { name: 'Опубликовать все на странице (2)' })
      );

      await waitFor(() =>
        expect(
          screen.getByText(/Опубликовано 1 из 2\. Ошибок: 1 \(«p2»: book not found\)/)
        ).toBeInTheDocument()
      );
    });
  });
});
