import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type AdminBook, type AdminBookAdaptation } from '@bookhaven/api-client';
import { consumeFlash } from '@/shared/lib';
import { BookEditView } from './book-edit-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  useParams: () => ({ id: 'b1' }),
  useSearchParams: () => searchParamsValue,
  usePathname: () => '/catalog/x/b1/edit',
}));

const getMock = vi.fn();
const updateMock = vi.fn();
const adaptationsMock = vi.fn();
const setHiddenMock = vi.fn();
const syncMock = vi.fn();

vi.mock('@/entities/book', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/book')>()),
  adminBooksApi: {
    get: (...args: unknown[]) => getMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
    adaptations: (...args: unknown[]) => adaptationsMock(...args),
    setAdaptationHidden: (...args: unknown[]) => setHiddenMock(...args),
    syncAdaptations: (...args: unknown[]) => syncMock(...args),
  },
}));

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    list: () => Promise.resolve({ items: [], total: 0, page: 1, limit: 100 }),
  },
}));

// The AI description panel under the form (its own tests live in
// widgets/ai-description-panel): no record, feature off.
vi.mock('@/entities/ai-description', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/ai-description')>()),
  adminAiDescriptionsApi: {
    get: () => Promise.resolve({ ai_description: null }),
    status: () =>
      Promise.resolve({ enabled: false, model: 'm', prompt_version: 1, missing: 0, drafts: 0, max_bulk: 200 }),
  },
}));

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: {
    list: () => Promise.resolve({ items: [], total: 0, page: 1, limit: 100 }),
  },
}));

const BACK = '/catalog/books?author_id=a1&page=2';

function makeBook(overrides: Partial<AdminBook> = {}): AdminBook {
  return {
    id: 'b1',
    title: 'Война и мир',
    slug: 'voyna-i-mir',
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
    published_at: null,
    external_source: null,
    authors: [],
    genres: [],
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

function makeAdaptation(overrides: Partial<AdminBookAdaptation> = {}): AdminBookAdaptation {
  return {
    id: 'w1',
    title: 'Мастер и Маргарита',
    original_title: null,
    kind: 'series',
    year: 2005,
    director: 'Владимир Бортко',
    imdb_url: null,
    kinopoisk_url: null,
    hidden: false,
    source: 'wikidata',
    qid: 'Q10',
    imdb_id: null,
    kinopoisk_id: null,
    ...overrides,
  };
}

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <BookEditView />
    </QueryClientProvider>
  );
  return { invalidateSpy, ...utils };
}

describe('BookEditView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams({ back: BACK });
    pushMock.mockReset();
    getMock.mockReset();
    updateMock.mockReset();
    adaptationsMock.mockReset();
    adaptationsMock.mockResolvedValue({ items: [], total: 0 });
    setHiddenMock.mockReset();
    syncMock.mockReset();
    window.sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the header with «Назад» to the back href and the title as subtitle', async () => {
    getMock.mockResolvedValue(makeBook());
    renderView();

    expect(screen.getByRole('heading', { level: 1, name: 'Редактирование книги' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '← Назад' })).toHaveAttribute('href', BACK);
    expect(await screen.findByText('Война и мир', { selector: 'p' })).toBeInTheDocument();
  });

  it('shows «Книга не найдена.» on 404 with «Назад» to the fallback', async () => {
    searchParamsValue = new URLSearchParams({ back: 'https://evil.com' });
    getMock.mockRejectedValue(new ApiError(404, 'Not Found', 'book not found'));
    renderView();

    expect(await screen.findByText('Книга не найдена.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '← Назад' })).toHaveAttribute('href', '/catalog/books');
  });

  it('shows a load error on other errors', async () => {
    getMock.mockRejectedValue(new ApiError(500, 'Server Error', 'boom'));
    renderView();

    expect(await screen.findByText('Не удалось загрузить данные.')).toBeInTheDocument();
  });

  it('passes the back href to BookForm: «Отмена» goes back', async () => {
    getMock.mockResolvedValue(makeBook());
    renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Отмена' }));
    expect(pushMock).toHaveBeenCalledWith(BACK);
    expect(updateMock).not.toHaveBeenCalled();
  });

  it('saves, invalidates books + dashboard, sets the flash and returns to back', async () => {
    getMock.mockResolvedValue(makeBook());
    updateMock.mockResolvedValue(makeBook({ title: 'Война и мир (том 1)' }));
    const { invalidateSpy } = renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Сохранить' }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith(BACK));
    expect(updateMock).toHaveBeenCalledWith('b1', expect.objectContaining({ title: 'Война и мир' }));
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'books'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
    expect(consumeFlash('catalog.books')).toBe('Книга «Война и мир (том 1)» сохранена.');
  });

  it('a malicious back falls back to /catalog/books', async () => {
    searchParamsValue = new URLSearchParams({ back: '//evil.com/catalog/books' });
    getMock.mockResolvedValue(makeBook());
    renderView();

    expect(screen.getByRole('link', { name: '← Назад' })).toHaveAttribute('href', '/catalog/books');
    fireEvent.click(await screen.findByRole('button', { name: 'Отмена' }));
    expect(pushMock).toHaveBeenCalledWith('/catalog/books');
  });

  it('lists adaptations with «Скрыта» marks and switches hide / show', async () => {
    getMock.mockResolvedValue(makeBook());
    adaptationsMock.mockResolvedValue({
      items: [makeAdaptation(), makeAdaptation({ id: 'w2', title: 'Мастер (фильм)', kind: 'film', year: 1994, hidden: true })],
      total: 2,
    });
    setHiddenMock.mockResolvedValue(makeAdaptation({ hidden: true }));
    renderView();

    const section = (await screen.findByRole('heading', { name: 'Экранизации' })).closest('section') as HTMLElement;
    expect(await within(section).findByText('Мастер и Маргарита')).toBeInTheDocument();
    expect(within(section).getByText('2005 · Сериал · Владимир Бортко')).toBeInTheDocument();
    expect(within(section).getAllByText('Скрыта')).toHaveLength(1);
    expect(within(section).getByRole('button', { name: 'Показать' })).toBeInTheDocument();

    adaptationsMock.mockResolvedValue({
      items: [makeAdaptation({ hidden: true }), makeAdaptation({ id: 'w2', title: 'Мастер (фильм)', kind: 'film', year: 1994, hidden: true })],
      total: 2,
    });
    fireEvent.click(within(section).getByRole('button', { name: 'Скрыть' }));
    await waitFor(() => expect(setHiddenMock).toHaveBeenCalledWith('b1', 'w1', true));
    await waitFor(() => expect(within(section).getAllByText('Скрыта')).toHaveLength(2));
    expect(within(section).queryByRole('button', { name: 'Скрыть' })).not.toBeInTheDocument();
    expect(within(section).getAllByRole('button', { name: 'Показать' })).toHaveLength(2);
  });

  it('shows the empty text, the load error and the visibility error', async () => {
    getMock.mockResolvedValue(makeBook());
    const { unmount } = renderView();
    expect(await screen.findByText('Экранизаций пока нет.')).toBeInTheDocument();
    unmount();

    adaptationsMock.mockRejectedValue(new ApiError(500, 'Server Error', 'boom'));
    const second = renderView();
    expect(await screen.findByText('Не удалось загрузить экранизации.')).toBeInTheDocument();
    second.unmount();

    adaptationsMock.mockResolvedValue({ items: [makeAdaptation()], total: 1 });
    setHiddenMock.mockRejectedValue(new ApiError(500, 'Server Error', 'boom'));
    renderView();
    fireEvent.click(await screen.findByRole('button', { name: 'Скрыть' }));
    expect(await screen.findByText('Не удалось изменить видимость экранизации.')).toBeInTheDocument();
  });

  it('offers the sync only for a Wikidata QID book and shows the result', async () => {
    getMock.mockResolvedValue(makeBook());
    const first = renderView();
    await screen.findByText('Экранизаций пока нет.');
    expect(screen.queryByRole('button', { name: 'Найти экранизации в Wikidata' })).not.toBeInTheDocument();
    first.unmount();

    getMock.mockResolvedValue(makeBook({ external_source: 'wikidata', external_id: 'Q161531' }));
    syncMock.mockResolvedValue({
      processed: 1,
      updated: 1,
      added: 2,
      items: [{ book_id: 'b1', title: 'Война и мир', outcome: 'added', found: 2, added: 2, notes: [], message: '' }],
    });
    renderView();
    fireEvent.click(await screen.findByRole('button', { name: 'Найти экранизации в Wikidata' }));
    await waitFor(() => expect(syncMock).toHaveBeenCalledWith(['b1']));
    expect(await screen.findByText('Книга «Война и мир»: добавлено экранизаций — 2.')).toBeInTheDocument();
  });
});
