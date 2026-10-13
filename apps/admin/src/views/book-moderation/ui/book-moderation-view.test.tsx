import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type { AdminAuthor, AdminBook, AdminBookQueueItem, AdminGenre } from '@bookhaven/api-client';
import { consumeFlash, setFlash } from '@/shared/lib';
import { BookModerationView } from './book-moderation-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/moderation/books',
  useSearchParams: () => searchParamsValue,
}));

const queueMock = vi.fn();
const bulkStatusMock = vi.fn();

vi.mock('@/entities/book', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/book')>()),
  adminBooksApi: {
    moderationQueue: (...args: unknown[]) => queueMock(...args),
    bulkStatus: (...args: unknown[]) => bulkStatusMock(...args),
  },
}));

function makeAuthor(overrides: Partial<AdminAuthor> = {}): AdminAuthor {
  return {
    id: 'a1',
    name: 'Лев Толстой',
    slug: 'lev-tolstoy',
    biography: null,
    photo_url: null,
    birth_date: null,
    death_date: null,
    status: 'published',
    external_source: null,
    external_id: null,
    photo_attribution: null,
    auto_harvest: false,
    last_auto_harvested_at: null,
    ...overrides,
  };
}

function makeGenre(overrides: Partial<AdminGenre> = {}): AdminGenre {
  return { id: 'g1', name: 'Роман', slug: 'roman', status: 'published', wikidata_qid: null, ...overrides };
}

function makeBook(overrides: Partial<AdminBook> = {}): AdminBook {
  return {
    id: 'b1',
    title: 'Война и мир',
    slug: 'voyna-i-mir',
    description: null,
    cover_url: null,
    isbn: '978-5-17-000000-1',
    published_year: 1869,
    page_count: null,
    language: 'ru',
    rating: 0,
    ratings_count: 0,
    reviews_count: 0,
    status: 'pending',
    published_at: null,
    external_source: 'wikidata',
    authors: [makeAuthor(), makeAuthor({ id: 'a2', name: 'Неизвестный соавтор', status: 'pending' })],
    genres: [makeGenre({ status: 'pending' })],
    created_at: '2026-10-06T10:00:00Z',
    updated_at: '2026-10-06T10:00:00Z',
    ...overrides,
  };
}

function makeItem(overrides: Partial<AdminBookQueueItem> = {}): AdminBookQueueItem {
  return {
    book: makeBook(),
    external_id: 'Q161531',
    created_at: '2026-10-06T10:00:00Z',
    similar: [
      {
        book: {
          id: 'h1',
          title: 'Война и мир (дубль)',
          slug: 'voyna-i-mir-2',
          status: 'rejected',
          isbn: '978-5-17-000000-1',
          published_year: 1869,
          external_source: null,
        },
        match: 'same_isbn',
      },
    ],
    ...overrides,
  };
}

const secondItem = makeItem({
  book: makeBook({
    id: 'b2',
    title: 'Анна Каренина',
    slug: 'anna',
    isbn: null,
    published_year: null,
    external_source: null,
    authors: [],
    genres: [],
  }),
  external_id: null,
  similar: [],
});

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

function rowOf(title: string) {
  return screen.getByText(title, { selector: 'a.font-medium' }).closest('tr') as HTMLElement;
}

async function loaded() {
  await waitFor(() => expect(screen.getByText('Анна Каренина')).toBeInTheDocument());
}

describe('BookModerationView', () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    queueMock.mockReset();
    bulkStatusMock.mockReset();
    queueMock.mockResolvedValue({ items: [makeItem(), secondItem], total: 2, page: 1, limit: 20 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the title and requests the pending queue by default', async () => {
    renderView();
    await loaded();
    expect(screen.getByRole('heading', { level: 1, name: 'Модерация книг' })).toBeInTheDocument();
    expect(queueMock).toHaveBeenCalledWith({
      status: 'pending',
      source: undefined,
      search: undefined,
      page: 1,
      limit: 20,
    });
    expect(screen.queryByRole('button', { name: /Опубликовать все на странице/ })).not.toBeInTheDocument();
  });

  it('passes status, source, search and page from the URL', async () => {
    searchParamsValue = new URLSearchParams('status=rejected&source=harvested&search=x&page=2');
    renderView();
    await loaded();
    expect(queueMock).toHaveBeenCalledWith({
      status: 'rejected',
      source: 'harvested',
      search: 'x',
      page: 2,
      limit: 20,
    });
  });

  it('drops an unknown source value', async () => {
    searchParamsValue = new URLSearchParams('source=bogus');
    renderView();
    await loaded();
    expect(queueMock).toHaveBeenCalledWith(expect.objectContaining({ source: undefined }));
  });

  it('pushes the source filter, tab and search to the URL', async () => {
    searchParamsValue = new URLSearchParams('page=3');
    renderView();
    await loaded();

    fireEvent.change(screen.getByRole('combobox', { name: 'Источник' }), { target: { value: 'manual' } });
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/books?source=manual');

    fireEvent.click(screen.getByRole('button', { name: 'Отклонённые' }));
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/books?status=rejected');

    fireEvent.change(screen.getByRole('textbox', { name: 'Поиск' }), { target: { value: ' Толстой ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));
    expect(pushMock).toHaveBeenLastCalledWith(
      `/moderation/books?${new URLSearchParams({ search: 'Толстой' }).toString()}`
    );
  });

  it('title links to the edit page with back to the current queue URL', async () => {
    searchParamsValue = new URLSearchParams('status=rejected&source=manual');
    renderView();
    await loaded();
    expect(screen.getByRole('link', { name: 'Война и мир' })).toHaveAttribute(
      'href',
      '/catalog/books/b1/edit?back=' + encodeURIComponent('/moderation/books?status=rejected&source=manual')
    );
  });

  it('renders the book columns', async () => {
    renderView();
    await loaded();
    const row = rowOf('Война и мир');

    expect(screen.getByRole('link', { name: 'Война и мир' })).toHaveAttribute(
      'href',
      '/catalog/books/b1/edit?back=%2Fmoderation%2Fbooks'
    );
    expect(within(row).getAllByText('978-5-17-000000-1 · 1869').length).toBeGreaterThan(0);

    // Authors: badge only for the non-published one.
    const tolstoy = within(row).getByText('Лев Толстой').closest('li') as HTMLElement;
    expect(within(tolstoy).queryByText('На модерации')).not.toBeInTheDocument();
    const coauthor = within(row).getByText('Неизвестный соавтор').closest('li') as HTMLElement;
    expect(within(coauthor).getByText('На модерации')).toBeInTheDocument();

    // Genres.
    const genre = within(row).getByText('Роман').closest('li') as HTMLElement;
    expect(within(genre).getByText('На модерации')).toBeInTheDocument();

    // Source.
    expect(within(row).getByText('wikidata')).toBeInTheDocument();
    expect(within(row).getByText('Q161531')).toHaveClass('font-mono');
    const second = rowOf('Анна Каренина');
    expect(within(second).getByText('Вручную')).toBeInTheDocument();

    // Added date.
    expect(within(row).getByText(new Date('2026-10-06T10:00:00Z').toLocaleDateString('ru-RU'))).toBeInTheDocument();
  });

  it('renders the duplicate hints', async () => {
    queueMock.mockResolvedValue({
      items: [
        makeItem(),
        {
          ...secondItem,
          similar: [
            {
              book: {
                id: 'h2',
                title: 'Анна Каренина (другая)',
                slug: 'anna-2',
                status: 'published',
                isbn: null,
                published_year: null,
                external_source: null,
              },
              match: 'same_title_author',
            },
          ],
        },
      ],
      total: 2,
      page: 1,
      limit: 20,
    });
    renderView();
    await loaded();

    const row = rowOf('Война и мир');
    const hintLink = within(row).getByRole('link', { name: 'Война и мир (дубль)' });
    expect(hintLink).toHaveAttribute('href', '/catalog/books/h1/edit?back=%2Fmoderation%2Fbooks');
    const hint = hintLink.closest('li') as HTMLElement;
    expect(within(hint).getByText('(тот же ISBN)')).toBeInTheDocument();
    expect(within(hint).getByText('Отклонена')).toBeInTheDocument();

    const second = rowOf('Анна Каренина');
    expect(within(second).getByText('(то же название и автор)')).toBeInTheDocument();
  });

  it('shows a dash when there are no hints, authors or genres', async () => {
    queueMock.mockResolvedValue({ items: [secondItem], total: 1, page: 1, limit: 20 });
    renderView();
    await loaded();
    expect(within(rowOf('Анна Каренина')).getAllByText('—')).toHaveLength(3);
  });

  it('row publish: confirm, bulkStatus with one id, success message', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });
    renderView();
    await loaded();

    fireEvent.click(within(rowOf('Война и мир')).getByRole('button', { name: 'Опубликовать' }));
    expect(confirmSpy).toHaveBeenCalledWith('Опубликовать книгу «Война и мир»?');
    await waitFor(() => expect(screen.getByText('Книга «Война и мир» опубликована.')).toBeInTheDocument());
    expect(bulkStatusMock).toHaveBeenCalledWith(['b1'], 'published');
  });

  it('rejected tab row: return to moderation', async () => {
    searchParamsValue = new URLSearchParams('status=rejected');
    const rejected = makeItem({ book: makeBook({ status: 'rejected' }) });
    queueMock.mockResolvedValue({ items: [rejected, secondItem], total: 2, page: 1, limit: 20 });
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });
    renderView();
    await loaded();

    fireEvent.click(within(rowOf('Война и мир')).getByRole('button', { name: 'Вернуть на модерацию' }));
    expect(confirmSpy).toHaveBeenCalledWith('Вернуть книгу «Война и мир» на модерацию?');
    await waitFor(() =>
      expect(screen.getByText('Книга «Война и мир» возвращена на модерацию.')).toBeInTheDocument()
    );
    expect(bulkStatusMock).toHaveBeenCalledWith(['b1'], 'pending');
  });

  it('row errors: report error and ApiError message', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkStatusMock.mockResolvedValueOnce({
      processed: 1,
      updated: 0,
      errors: [{ book_id: 'b1', title: 'Война и мир', message: 'msg' }],
    });
    renderView();
    await loaded();

    fireEvent.click(within(rowOf('Война и мир')).getByRole('button', { name: 'Отклонить' }));
    await waitFor(() =>
      expect(screen.getByText('Не удалось изменить статус «Война и мир»: msg.')).toBeInTheDocument()
    );

    bulkStatusMock.mockRejectedValueOnce(new ApiError(409, 'Conflict', 'Конфликт статуса'));
    fireEvent.click(within(rowOf('Война и мир')).getByRole('button', { name: 'Отклонить' }));
    await waitFor(() => expect(screen.getByText('Конфликт статуса')).toBeInTheDocument());
  });

  it('bulk publish of the selected page', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({ processed: 2, updated: 2, errors: [] });
    renderView();
    await loaded();

    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать все на странице' }));
    fireEvent.click(screen.getByRole('button', { name: 'Опубликовать выбранные (2)' }));
    expect(confirmSpy).toHaveBeenCalledWith('Опубликовать выбранные книги (2)?');
    await waitFor(() => expect(screen.getByText('Обновлено 2 из 2.')).toBeInTheDocument());
    expect(bulkStatusMock).toHaveBeenCalledWith(['b1', 'b2'], 'published');
  });

  it('bulk partial report shows errors and clears the selection', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({
      processed: 2,
      updated: 1,
      errors: [{ book_id: 'b2', title: 'T', message: 'msg' }],
    });
    renderView();
    await loaded();

    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать все на странице' }));
    fireEvent.click(screen.getByRole('button', { name: 'Отклонить выбранные (2)' }));
    await waitFor(() =>
      expect(screen.getByText('Обновлено 1 из 2. Ошибок: 1 («T»: msg).')).toBeInTheDocument()
    );
    expect(screen.queryByText(/Выбрано:/)).not.toBeInTheDocument();
  });

  it.each([
    ['', 'Нет книг на модерации'],
    ['source=manual', 'Книги не найдены'],
  ])('empty state for "%s"', async (query, text) => {
    searchParamsValue = new URLSearchParams(query);
    queueMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    renderView();
    await waitFor(() => expect(screen.getByText(text)).toBeInTheDocument());
  });

  it('shows the load error', async () => {
    queueMock.mockRejectedValue(new Error('fail'));
    renderView();
    await waitFor(() => expect(screen.getByText('Не удалось загрузить очередь книг.')).toBeInTheDocument());
  });

  it('consumes the book-saved flash left by the edit page', async () => {
    setFlash('catalog.books', 'Книга «Война и мир» сохранена.');
    renderView();
    await loaded();
    expect(screen.getByText('Книга «Война и мир» сохранена.')).toBeInTheDocument();
    expect(consumeFlash('catalog.books')).toBeNull();
  });
});
