import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminScreenWork } from '@bookhaven/api-client';
import { ScreenWorksView } from './screen-works-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/catalog/screen-works',
  useSearchParams: () => searchParamsValue,
}));

const listMock = vi.fn();
vi.mock('@/entities/screen-work', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/screen-work')>()),
  adminScreenWorksApi: { list: (...a: unknown[]) => listMock(...a) },
}));

const makeWork = (patch: Partial<AdminScreenWork> = {}): AdminScreenWork => ({
  id: 'w1',
  title: 'Мастер и Маргарита',
  original_title: null,
  kind: 'series',
  year: 2005,
  director: 'Владимир Бортко',
  imdb_id: null,
  kinopoisk_id: null,
  qid: 'Q1',
  manually_edited_at: null,
  book_count: 2,
  author_count: 1,
  ...patch,
});

function renderView() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <ScreenWorksView />
    </QueryClientProvider>
  );
}

describe('ScreenWorksView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    listMock.mockReset();
    window.sessionStorage.clear();
  });

  it('renders rows with counts and an edit link carrying the list location', async () => {
    searchParamsValue = new URLSearchParams({ search: 'мастер' });
    listMock.mockResolvedValue({ items: [makeWork()], total: 1, page: 1, limit: 20 });
    renderView();
    expect(await screen.findByText('Мастер и Маргарита')).toBeTruthy();
    expect(screen.getByText('Книг: 2')).toBeTruthy();
    expect(screen.getByText('Авторов: 1')).toBeTruthy();
    expect(screen.getByText('Сериал')).toBeTruthy();
    expect(listMock).toHaveBeenCalledWith({ search: 'мастер', unlinked: undefined, page: 1, limit: 20 });
    const edit = screen.getByRole('link', { name: 'Изменить' });
    expect(edit.getAttribute('href')).toBe(
      '/catalog/screen-works/w1/edit?back=' + encodeURIComponent('/catalog/screen-works?' + new URLSearchParams({ search: 'мастер' }).toString())
    );
  });

  it('keeps search and filter in the address', async () => {
    listMock.mockResolvedValue({ items: [makeWork()], total: 1, page: 1, limit: 20 });
    renderView();
    await screen.findByText('Мастер и Маргарита');
    fireEvent.change(screen.getByLabelText('Поиск'), { target: { value: ' солярис ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));
    expect(pushMock).toHaveBeenCalledWith('/catalog/screen-works?search=%D1%81%D0%BE%D0%BB%D1%8F%D1%80%D0%B8%D1%81');
    fireEvent.click(screen.getByLabelText('Только без связей'));
    expect(pushMock).toHaveBeenLastCalledWith('/catalog/screen-works?unlinked=true');
  });

  it('shows the empty and error texts', async () => {
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    renderView();
    expect(await screen.findByText('Фильмы не найдены.')).toBeTruthy();
  });

  it('shows the load error', async () => {
    listMock.mockRejectedValue(new Error('boom'));
    renderView();
    expect(await screen.findByText('Не удалось загрузить фильмы.')).toBeTruthy();
  });
});
