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
const aiReviewMock = vi.fn();
const aiReviewStatusMock = vi.fn();
const syncMock = vi.fn();

vi.mock('@/entities/book', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/book')>()),
  adminBooksApi: {
    moderationQueue: (...args: unknown[]) => queueMock(...args),
    bulkStatus: (...args: unknown[]) => bulkStatusMock(...args),
    aiReview: (...args: unknown[]) => aiReviewMock(...args),
    aiReviewStatus: (...args: unknown[]) => aiReviewStatusMock(...args),
    syncAdaptations: (...args: unknown[]) => syncMock(...args),
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
    ai_verdict: null,
    ...overrides,
  };
}

function makeVerdict(overrides: Partial<NonNullable<AdminBookQueueItem['ai_verdict']>> = {}) {
  return {
    verdict: 'publish' as const,
    reason_codes: [],
    reason: 'Проблем не найдено.',
    confidence: 0.95,
    model: 'claude-haiku-5-5',
    prompt_version: 1,
    created_at: '2026-10-13T10:00:00Z',
    stale: false,
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
    aiReviewMock.mockReset();
    aiReviewStatusMock.mockReset();
    syncMock.mockReset();
    aiReviewStatusMock.mockResolvedValue({ enabled: true, model: 'claude-haiku-5-5', prompt_version: 1, needs_review: 7 });
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
      ai: undefined,
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
      ai: undefined,
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
    // Hints, authors, genres and the «ИИ» column (no verdict).
    expect(within(rowOf('Анна Каренина')).getAllByText('—')).toHaveLength(4);
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
  it('passes the ai filter from the URL and pushes it on change', async () => {
    searchParamsValue = new URLSearchParams('ai=needs_human');
    renderView();
    await loaded();
    expect(queueMock).toHaveBeenCalledWith(expect.objectContaining({ ai: 'needs_human' }));

    const select = screen.getByRole('combobox', { name: 'ИИ' });
    expect(within(select).getAllByRole('option').map((o) => o.textContent)).toEqual([
      'ИИ: любой',
      'ИИ: опубликовать',
      'ИИ: отклонить',
      'ИИ: нужен человек',
      'ИИ: без проверки',
    ]);
    fireEvent.change(select, { target: { value: 'none' } });
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/books?ai=none');
  });

  it('drops an unknown ai value', async () => {
    searchParamsValue = new URLSearchParams('ai=maybe');
    renderView();
    await loaded();
    expect(queueMock).toHaveBeenCalledWith(expect.objectContaining({ ai: undefined }));
  });

  it('renders the AI verdict column', async () => {
    queueMock.mockResolvedValue({
      items: [
        makeItem({
          ai_verdict: makeVerdict({
            verdict: 'reject',
            reason_codes: ['duplicate'],
            reason: 'Уже есть.',
            confidence: 0.81,
            stale: true,
          }),
        }),
        secondItem,
      ],
      total: 2,
      page: 1,
      limit: 20,
    });
    renderView();
    await loaded();
    const row = rowOf('Война и мир');
    const cell = within(row).getByText('81%').closest('td') as HTMLElement;
    expect(within(cell).getByText('Отклонить')).toBeInTheDocument();
    expect(within(cell).getByText('дубликат')).toBeInTheDocument();
    expect(within(cell).getByText('Уже есть.')).toBeInTheDocument();
    expect(within(cell).getByText('устарел')).toBeInTheDocument();
  });

  it('confident selection only selects the right rows and publishes nothing', async () => {
    const third = makeItem({
      book: makeBook({ id: 'b3', title: 'Детство', isbn: null }),
      similar: [],
      ai_verdict: makeVerdict({ stale: true }),
    });
    const fourth = makeItem({
      book: makeBook({ id: 'b4', title: 'Отрочество', isbn: null }),
      similar: [],
      ai_verdict: makeVerdict({ confidence: 0.5 }),
    });
    queueMock.mockResolvedValue({
      items: [
        makeItem({ ai_verdict: makeVerdict() }), // has a duplicate hint
        { ...secondItem, ai_verdict: makeVerdict() },
        third,
        fourth,
      ],
      total: 4,
      page: 1,
      limit: 20,
    });
    const confirmSpy = vi.spyOn(window, 'confirm');
    renderView();
    await loaded();

    fireEvent.click(await screen.findByRole('button', { name: 'Выбрать уверенные «Опубликовать» (1)' }));
    expect(screen.getByRole('checkbox', { name: 'Выбрать «Анна Каренина»' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Выбрать «Война и мир»' })).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Выбрать «Детство»' })).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Выбрать «Отрочество»' })).not.toBeChecked();
    expect(screen.getByRole('button', { name: 'Опубликовать выбранные (1)' })).toBeInTheDocument();
    expect(bulkStatusMock).not.toHaveBeenCalled();
    expect(aiReviewMock).not.toHaveBeenCalled();
    expect(confirmSpy).not.toHaveBeenCalled();
  });

  it('«Проверить ИИ (N)» posts the selected ids and shows the queued message', async () => {
    aiReviewMock.mockResolvedValue({ task_id: 't1' });
    renderView();
    await loaded();
    expect(await screen.findByText('Без проверки ИИ: 7')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать все на странице' }));
    fireEvent.click(screen.getByRole('button', { name: 'Проверить ИИ (2)' }));
    await waitFor(() =>
      expect(
        screen.getByText('Проверка ИИ поставлена в очередь. Вердикты появятся через несколько минут.')
      ).toBeInTheDocument()
    );
    expect(aiReviewMock).toHaveBeenCalledWith({ book_ids: ['b1', 'b2'] });
    expect(bulkStatusMock).not.toHaveBeenCalled();
  });

  it('«Проверить ИИ все непроверенные» confirms, queues and maps 409', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    aiReviewMock.mockRejectedValueOnce(new ApiError(409, 'Conflict', 'AI review already queued'));
    renderView();
    await loaded();

    fireEvent.click(await screen.findByRole('button', { name: 'Проверить ИИ все непроверенные' }));
    expect(confirmSpy).toHaveBeenCalledWith('Отправить на проверку ИИ все непроверенные книги (7)?');
    await waitFor(() => expect(screen.getByText('Проверка ИИ уже в очереди.')).toBeInTheDocument());
    expect(aiReviewMock).toHaveBeenCalledWith({ all_unreviewed: true });

    confirmSpy.mockReturnValue(false);
    fireEvent.click(screen.getByRole('button', { name: 'Проверить ИИ все непроверенные' }));
    expect(aiReviewMock).toHaveBeenCalledTimes(1);
  });

  it('hides the check controls when AI moderation is disabled; verdicts and the filter remain', async () => {
    aiReviewStatusMock.mockResolvedValue({
      enabled: false,
      model: 'claude-haiku-5-5',
      prompt_version: 1,
      needs_review: 3,
    });
    queueMock.mockResolvedValue({
      items: [
        makeItem({ ai_verdict: makeVerdict({ verdict: 'needs_human', reason_codes: ['insufficient_data'] }) }),
        { ...secondItem, ai_verdict: makeVerdict() },
      ],
      total: 2,
      page: 1,
      limit: 20,
    });
    renderView();
    await loaded();
    await waitFor(() => expect(aiReviewStatusMock).toHaveBeenCalled());

    expect(screen.getByText('Нужен человек')).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'ИИ' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Выбрать уверенные «Опубликовать» (1)' })).toBeInTheDocument();
    expect(screen.queryByText(/Без проверки ИИ/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Проверить ИИ все непроверенные' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать все на странице' }));
    expect(screen.queryByRole('button', { name: /Проверить ИИ \(/ })).not.toBeInTheDocument();
  });

  it('shows no AI controls outside the pending tab', async () => {
    searchParamsValue = new URLSearchParams('status=published');
    queueMock.mockResolvedValue({
      items: [{ ...secondItem, book: { ...secondItem.book, status: 'published' }, ai_verdict: makeVerdict() }],
      total: 1,
      page: 1,
      limit: 20,
    });
    renderView();
    await loaded();
    await waitFor(() => expect(aiReviewStatusMock).toHaveBeenCalled());
    expect(screen.queryByText(/Без проверки ИИ/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Выбрать уверенные/ })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать все на странице' }));
    expect(screen.queryByRole('button', { name: /Проверить ИИ \(/ })).not.toBeInTheDocument();
  });

  it('offers the adaptation sync row action only for QID books and syncs one book', async () => {
    syncMock.mockResolvedValue({
      processed: 1,
      updated: 0,
      added: 0,
      items: [{ book_id: 'b1', title: 'Война и мир', outcome: 'unchanged', found: 0, added: 0, notes: [], message: '' }],
    });
    renderView();
    await loaded();

    expect(
      within(rowOf('Анна Каренина')).queryByRole('button', { name: 'Найти экранизации в Wikidata' })
    ).not.toBeInTheDocument();
    fireEvent.click(within(rowOf('Война и мир')).getByRole('button', { name: 'Найти экранизации в Wikidata' }));
    await waitFor(() => expect(syncMock).toHaveBeenCalledWith(['b1']));
    expect(await screen.findByText('Книга «Война и мир»: новых экранизаций нет.')).toBeInTheDocument();
  });

  it('bulk sync counts and sends only QID rows and reports the skipped ones', async () => {
    syncMock.mockResolvedValue({
      processed: 1,
      updated: 1,
      added: 2,
      items: [{ book_id: 'b1', title: 'Война и мир', outcome: 'added', found: 2, added: 2, notes: [], message: '' }],
    });
    renderView();
    await loaded();

    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать все на странице' }));
    fireEvent.click(screen.getByRole('button', { name: 'Найти экранизации в Wikidata (1)' }));
    await waitFor(() => expect(syncMock).toHaveBeenCalledWith(['b1']));
    expect(
      await screen.findByText('Обновлено книг: 1 из 1, добавлено экранизаций: 2. Без QID пропущено: 1.')
    ).toBeInTheDocument();
  });

  it('shows the Wikidata-unavailable text when the bulk sync fails with 502', async () => {
    syncMock.mockRejectedValue(new ApiError(502, 'Bad Gateway', 'x'));
    renderView();
    await loaded();

    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать все на странице' }));
    fireEvent.click(screen.getByRole('button', { name: 'Найти экранизации в Wikidata (1)' }));
    expect(await screen.findByText('Wikidata недоступна, попробуйте позже.')).toBeInTheDocument();
  });
});
