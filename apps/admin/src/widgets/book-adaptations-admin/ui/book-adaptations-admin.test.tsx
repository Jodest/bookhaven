import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminBookAdaptation } from '@bookhaven/api-client';
import { BookAdaptationsAdmin } from './book-adaptations-admin';

const adaptationsMock = vi.fn();
const deleteMock = vi.fn();

vi.mock('next/navigation', () => ({
  usePathname: () => '/catalog/books/b1/edit',
  useSearchParams: () => new URLSearchParams('back=%2Fcatalog%2Fbooks'),
}));
vi.mock('@/entities/book', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/book')>()),
  adminBooksApi: {
    adaptations: (...a: unknown[]) => adaptationsMock(...a),
    deleteAdaptation: (...a: unknown[]) => deleteMock(...a),
    setAdaptationHidden: vi.fn(),
    syncAdaptations: vi.fn(),
  },
}));

const item = (id: string, source: 'wikidata' | 'manual', title: string): AdminBookAdaptation => ({
  id,
  title,
  original_title: null,
  kind: 'film',
  year: 1990,
  director: null,
  imdb_url: null,
  kinopoisk_url: null,
  poster: null,
  hidden: false,
  source,
  qid: source === 'wikidata' ? 'Q1' : null,
  imdb_id: null,
  kinopoisk_id: null,
});

function renderWidget(hasQid = false) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <BookAdaptationsAdmin bookId="b1" bookTitle="Книга" hasQid={hasQid} />
    </QueryClientProvider>
  );
}

describe('BookAdaptationsAdmin', () => {
  beforeEach(() => {
    adaptationsMock.mockReset();
    deleteMock.mockReset();
    vi.restoreAllMocks();
  });

  it('offers delete only for manual rows and links edit with back', async () => {
    adaptationsMock.mockResolvedValue({
      items: [item('w1', 'wikidata', 'Из Wikidata'), item('w2', 'manual', 'Ручной')],
      total: 2,
    });
    renderWidget();
    await screen.findByText('Ручной');
    expect(screen.getAllByRole('button', { name: 'Удалить' })).toHaveLength(1);
    expect(screen.getAllByText('Вручную')).toHaveLength(1);
    expect(screen.getAllByRole('button', { name: 'Скрыть' })).toHaveLength(2);
    const edit = screen.getAllByRole('link', { name: 'Изменить' })[0];
    expect(edit.getAttribute('href')).toBe(
      '/catalog/screen-works/w1/edit?back=' + encodeURIComponent('/catalog/books/b1/edit?back=%2Fcatalog%2Fbooks')
    );
  });

  it('shows the add button without a QID and no Wikidata search', async () => {
    adaptationsMock.mockResolvedValue({ items: [], total: 0 });
    renderWidget(false);
    expect(await screen.findByText('Экранизаций пока нет.')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Добавить экранизацию' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Найти экранизации в Wikidata' })).toBeNull();
  });

  it('deletes a manual link after confirmation', async () => {
    adaptationsMock.mockResolvedValue({ items: [item('w2', 'manual', 'Ручной')], total: 1 });
    deleteMock.mockResolvedValue(undefined);
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderWidget();
    fireEvent.click(await screen.findByRole('button', { name: 'Удалить' }));
    expect(confirmSpy).toHaveBeenCalledWith('Удалить связь с «Ручной»? Сам фильм останется в каталоге.');
    await waitFor(() => expect(deleteMock).toHaveBeenCalledWith('b1', 'w2'));
  });

  it('shows the delete error', async () => {
    adaptationsMock.mockResolvedValue({ items: [item('w2', 'manual', 'Ручной')], total: 1 });
    deleteMock.mockRejectedValue(new Error('x'));
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderWidget();
    fireEvent.click(await screen.findByRole('button', { name: 'Удалить' }));
    expect(await screen.findByText('Не удалось удалить связь.')).toBeTruthy();
  });
});
