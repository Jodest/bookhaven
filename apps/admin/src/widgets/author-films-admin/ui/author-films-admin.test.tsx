import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminAuthorFilm } from '@bookhaven/api-client';
import { AuthorFilmsAdmin } from './author-films-admin';

const screenWorksMock = vi.fn();
const updateMock = vi.fn();
const deleteMock = vi.fn();

vi.mock('next/navigation', () => ({
  usePathname: () => '/catalog/authors/a1/edit',
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    screenWorks: (...a: unknown[]) => screenWorksMock(...a),
    updateScreenWork: (...a: unknown[]) => updateMock(...a),
    setScreenWorkHidden: vi.fn(),
    deleteScreenWork: (...a: unknown[]) => deleteMock(...a),
    syncScreenWorks: vi.fn(),
  },
}));

const film = (id: string, source: 'wikidata' | 'manual', relation: 'biopic' | 'documentary'): AdminAuthorFilm => ({
  id,
  title: `Фильм ${id}`,
  original_title: null,
  kind: 'film',
  year: 2010,
  director: null,
  imdb_url: null,
  kinopoisk_url: null,
  poster: null,
  relation,
  hidden: false,
  source,
  qid: null,
  imdb_id: null,
  kinopoisk_id: null,
});

function renderWidget(hasQid = false) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <AuthorFilmsAdmin authorId="a1" authorName="Автор" hasQid={hasQid} />
    </QueryClientProvider>
  );
}

describe('AuthorFilmsAdmin', () => {
  beforeEach(() => {
    screenWorksMock.mockReset();
    updateMock.mockReset();
    deleteMock.mockReset();
    vi.restoreAllMocks();
  });

  it('has no delete on a Wikidata row and marks the manual one', async () => {
    screenWorksMock.mockResolvedValue({
      items: [film('w1', 'wikidata', 'biopic'), film('w2', 'manual', 'documentary')],
      total: 2,
    });
    renderWidget();
    await screen.findByText('Фильм w2');
    expect(screen.getAllByRole('button', { name: 'Удалить' })).toHaveLength(1);
    expect(screen.getAllByText('Вручную')).toHaveLength(1);
    expect(screen.getAllByRole('link', { name: 'Изменить' })).toHaveLength(2);
  });

  it('shows the add button for an author without a QID', async () => {
    screenWorksMock.mockResolvedValue({ items: [], total: 0 });
    renderWidget(false);
    expect(await screen.findByText('Фильмов об авторе пока нет.')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Добавить фильм об авторе' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Найти фильмы об авторе в Wikidata' })).toBeNull();
  });

  it('patches the relation from the switch', async () => {
    screenWorksMock.mockResolvedValue({ items: [film('w1', 'wikidata', 'biopic')], total: 1 });
    updateMock.mockResolvedValue(film('w1', 'wikidata', 'documentary'));
    renderWidget();
    fireEvent.change(await screen.findByLabelText('Тип связи'), { target: { value: 'documentary' } });
    await waitFor(() => expect(updateMock).toHaveBeenCalledWith('a1', 'w1', { relation: 'documentary' }));
  });

  it('shows the relation error', async () => {
    screenWorksMock.mockResolvedValue({ items: [film('w1', 'wikidata', 'biopic')], total: 1 });
    updateMock.mockRejectedValue(new Error('x'));
    renderWidget();
    fireEvent.change(await screen.findByLabelText('Тип связи'), { target: { value: 'documentary' } });
    expect(await screen.findByText('Не удалось изменить тип связи.')).toBeTruthy();
  });

  it('deletes a manual link after confirmation', async () => {
    screenWorksMock.mockResolvedValue({ items: [film('w2', 'manual', 'biopic')], total: 1 });
    deleteMock.mockResolvedValue(undefined);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderWidget();
    fireEvent.click(await screen.findByRole('button', { name: 'Удалить' }));
    await waitFor(() => expect(deleteMock).toHaveBeenCalledWith('a1', 'w2'));
  });
});
