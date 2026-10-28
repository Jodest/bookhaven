import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type AdminScreenWorkDetail } from '@bookhaven/api-client';
import { consumeFlash } from '@/shared/lib';
import { resolveBackHref, ScreenWorkEditView } from './screen-work-edit-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  useParams: () => ({ id: 'w1' }),
  useSearchParams: () => searchParamsValue,
}));

const getMock = vi.fn();
const updateMock = vi.fn();
const deleteMock = vi.fn();

vi.mock('@/entities/screen-work', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/screen-work')>()),
  adminScreenWorksApi: {
    get: (...a: unknown[]) => getMock(...a),
    update: (...a: unknown[]) => updateMock(...a),
    delete: (...a: unknown[]) => deleteMock(...a),
  },
}));

const makeWork = (patch: Partial<AdminScreenWorkDetail> = {}): AdminScreenWorkDetail => ({
  id: 'w1',
  title: 'Солярис',
  original_title: null,
  kind: 'film',
  year: 1972,
  director: null,
  imdb_id: null,
  kinopoisk_id: null,
  qid: null,
  manually_edited_at: null,
  book_count: 0,
  author_count: 0,
  books: [],
  authors: [],
  poster: null,
  poster_removed: false,
  ...patch,
});

function renderView() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <ScreenWorkEditView />
    </QueryClientProvider>
  );
}

describe('resolveBackHref', () => {
  it.each([
    ['/catalog/screen-works?search=%D1%81', '/catalog/screen-works?search=%D1%81'],
    ['/catalog/books/abc-1/edit?back=%2Fcatalog%2Fbooks', '/catalog/books/abc-1/edit?back=%2Fcatalog%2Fbooks'],
    ['/catalog/authors/a1/edit', '/catalog/authors/a1/edit'],
  ])('keeps %s', (raw, expected) => {
    expect(resolveBackHref(raw)).toBe(expected);
  });

  it.each([
    'https://evil.example/catalog/screen-works',
    '//evil.example',
    '/\\evil.example',
    '/catalog/genres',
    '/catalog/books/a/b/edit',
    '/catalog/books/edit',
    null,
  ])('falls back for %s', (raw) => {
    expect(resolveBackHref(raw)).toBe('/catalog/screen-works');
  });
});

describe('ScreenWorkEditView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams({ back: '/catalog/screen-works?search=x' });
    pushMock.mockReset();
    getMock.mockReset();
    updateMock.mockReset();
    deleteMock.mockReset();
    vi.restoreAllMocks();
    window.sessionStorage.clear();
  });

  it('shows the poster section apart from the data form', async () => {
    getMock.mockResolvedValue(makeWork({ poster_removed: true }));
    renderView();
    expect(await screen.findByRole('heading', { name: 'Постер' })).toBeTruthy();
    expect(screen.getByText('Постера нет.')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Загрузить постер' })).toBeTruthy();
  });

  it('falls back to the list for a malicious back', async () => {
    searchParamsValue = new URLSearchParams({ back: 'https://evil.example/' });
    getMock.mockResolvedValue(makeWork());
    renderView();
    const back = await screen.findByRole('link', { name: '← Назад' });
    expect(back.getAttribute('href')).toBe('/catalog/screen-works');
  });

  it('lists linked books and authors and offers no delete', async () => {
    getMock.mockResolvedValue(
      makeWork({
        manually_edited_at: '2026-10-01T10:00:00Z',
        books: [{ id: 'b1', title: 'Книга', status: 'published', hidden: true, source: 'wikidata' }],
        authors: [{ id: 'a1', name: 'Автор', status: 'published', hidden: false, source: 'manual', relation: 'documentary' }],
      })
    );
    renderView();
    expect(await screen.findByRole('link', { name: 'Книга' })).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Автор' })).toBeTruthy();
    expect(screen.getByText('Скрыта')).toBeTruthy();
    expect(screen.getByText('Вручную')).toBeTruthy();
    expect(screen.getByText('Документальный')).toBeTruthy();
    expect(screen.getByText(/Изменён вручную/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Удалить фильм' })).toBeNull();
    expect(screen.getByText('Фильм можно удалить, только когда у него нет связей с книгами и авторами.')).toBeTruthy();
  });

  it('deletes an unlinked work after confirmation and returns to the list', async () => {
    getMock.mockResolvedValue(makeWork());
    deleteMock.mockResolvedValue(undefined);
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderView();
    fireEvent.click(await screen.findByRole('button', { name: 'Удалить фильм' }));
    expect(confirmSpy).toHaveBeenCalledWith('Удалить фильм «Солярис» из каталога?');
    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/catalog/screen-works?search=x'));
    expect(consumeFlash('catalog.screen-works')).toBe('Фильм «Солярис» удалён.');
  });

  it('shows the delete failure', async () => {
    getMock.mockResolvedValue(makeWork());
    deleteMock.mockRejectedValue(new ApiError(409, 'Conflict', 'screen work has links'));
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderView();
    fireEvent.click(await screen.findByRole('button', { name: 'Удалить фильм' }));
    expect(await screen.findByText('Не удалось удалить фильм.')).toBeTruthy();
    expect(pushMock).not.toHaveBeenCalled();
  });

  it('saves and returns to where the admin came from', async () => {
    searchParamsValue = new URLSearchParams({ back: '/catalog/books/b1/edit' });
    getMock.mockResolvedValue(makeWork());
    updateMock.mockResolvedValue(makeWork({ title: 'Новое' }));
    renderView();
    fireEvent.click(await screen.findByRole('button', { name: 'Сохранить' }));
    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/catalog/books/b1/edit'));
  });

  it('shows not found', async () => {
    getMock.mockRejectedValue(new ApiError(404, 'Not Found', 'screen work not found'));
    renderView();
    expect(await screen.findByText('Фильм не найден.')).toBeTruthy();
  });
});
