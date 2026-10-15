import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type AdminAuthor, type AdminAuthorFilm } from '@bookhaven/api-client';
import { consumeFlash } from '@/shared/lib';
import { AuthorEditView } from './author-edit-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  useParams: () => ({ id: 'a1' }),
  useSearchParams: () => searchParamsValue,
  usePathname: () => '/catalog/x/a1/edit',
}));

const getMock = vi.fn();
const updateMock = vi.fn();
const filmsMock = vi.fn();
const setHiddenMock = vi.fn();
const syncFilmsMock = vi.fn();

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    get: (...args: unknown[]) => getMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
    screenWorks: (...args: unknown[]) => filmsMock(...args),
    setScreenWorkHidden: (...args: unknown[]) => setHiddenMock(...args),
    syncScreenWorks: (...args: unknown[]) => syncFilmsMock(...args),
  },
}));

const BACK = '/catalog/authors?status=pending&page=2';

function makeAuthor(overrides: Partial<AdminAuthor> = {}): AdminAuthor {
  return {
    id: 'a1',
    name: 'Лев Толстой',
    slug: 'lev-tolstoy',
    biography: 'Писатель',
    photo_url: null,
    birth_date: '1828-09-09',
    death_date: null,
    status: 'pending',
    external_source: null,
    external_id: null,
    photo_attribution: null,
    auto_harvest: false,
    last_auto_harvested_at: null,
    ...overrides,
  };
}

function makeFilm(overrides: Partial<AdminAuthorFilm> = {}): AdminAuthorFilm {
  return {
    id: 'w1',
    title: 'Толкин',
    original_title: null,
    kind: 'film',
    relation: 'biopic',
    year: 2019,
    director: 'Доме Каруко',
    imdb_url: null,
    kinopoisk_url: null,
    hidden: false,
    source: 'wikidata',
    qid: 'Q42689885',
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
      <AuthorEditView />
    </QueryClientProvider>
  );
  return { invalidateSpy, ...utils };
}

describe('AuthorEditView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams({ back: BACK });
    pushMock.mockReset();
    getMock.mockReset();
    updateMock.mockReset();
    filmsMock.mockReset();
    filmsMock.mockResolvedValue({ items: [], total: 0 });
    setHiddenMock.mockReset();
    syncFilmsMock.mockReset();
    window.sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows the header and a skeleton while loading', () => {
    getMock.mockReturnValue(new Promise(() => {}));
    renderView();

    expect(screen.getByRole('heading', { level: 1, name: 'Редактирование автора' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '← Назад' })).toHaveAttribute('href', BACK);
    expect(screen.getByTestId('edit-loading')).toBeInTheDocument();
  });

  it.each([404, 400])('shows «Автор не найден.» on %i with a «Назад» link', async (status) => {
    searchParamsValue = new URLSearchParams();
    getMock.mockRejectedValue(new ApiError(status, 'Error', 'author not found'));
    renderView();

    expect(await screen.findByText('Автор не найден.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '← Назад' })).toHaveAttribute('href', '/catalog/authors');
  });

  it('shows a load error on other errors', async () => {
    getMock.mockRejectedValue(new ApiError(500, 'Server Error', 'boom'));
    renderView();

    expect(await screen.findByText('Не удалось загрузить данные.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '← Назад' })).toHaveAttribute('href', BACK);
  });

  it('prefills the form from the loaded author', async () => {
    getMock.mockResolvedValue(makeAuthor());
    renderView();

    expect(await screen.findByLabelText('Имя *')).toHaveValue('Лев Толстой');
    expect(getMock).toHaveBeenCalledWith('a1');
    expect(screen.getByLabelText('Slug')).toHaveValue('lev-tolstoy');
    expect(screen.getByLabelText('Биография')).toHaveValue('Писатель');
    expect(screen.getByLabelText('Дата рождения')).toHaveValue('1828-09-09');
    expect((screen.getByLabelText('Статус', { selector: '#status' }) as HTMLSelectElement).value).toBe('pending');
    expect(screen.getByText('Лев Толстой', { selector: 'p' })).toBeInTheDocument();
  });

  it('saves via update(id, input), sets the flash, invalidates and returns to back', async () => {
    getMock.mockResolvedValue(makeAuthor());
    updateMock.mockResolvedValue(makeAuthor({ name: 'Лев Николаевич Толстой' }));
    const { invalidateSpy } = renderView();

    fireEvent.change(await screen.findByLabelText('Имя *'), { target: { value: 'Лев Николаевич Толстой' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith(BACK));
    expect(updateMock).toHaveBeenCalledWith('a1', {
      name: 'Лев Николаевич Толстой',
      slug: 'lev-tolstoy',
      biography: 'Писатель',
      photo_url: '',
      birth_date: '1828-09-09',
      death_date: '',
      status: 'pending',
    });
    expect('auto_harvest' in updateMock.mock.calls[0][1]).toBe(false);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'authors'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
    expect(consumeFlash('catalog.authors')).toBe('Автор «Лев Николаевич Толстой» сохранён.');
  });

  it('sends auto_harvest: true after ticking the checkbox on an author with a QID', async () => {
    const author = makeAuthor({ external_source: 'wikidata', external_id: 'Q7243' });
    getMock.mockResolvedValue(author);
    updateMock.mockResolvedValue({ ...author, auto_harvest: true });
    renderView();

    fireEvent.click(await screen.findByLabelText('Автоматически пополнять книги из Wikidata'));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => expect(updateMock).toHaveBeenCalled());
    expect(updateMock.mock.calls[0][0]).toBe('a1');
    expect(updateMock.mock.calls[0][1]).toMatchObject({ auto_harvest: true });
  });

  it('sends no auto_harvest when only the biography of a flagged author changed', async () => {
    getMock.mockResolvedValue(makeAuthor({ auto_harvest: true }));
    updateMock.mockResolvedValue(makeAuthor({ auto_harvest: true }));
    renderView();

    fireEvent.change(await screen.findByLabelText('Биография'), { target: { value: 'Новая биография' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => expect(updateMock).toHaveBeenCalled());
    expect(updateMock.mock.calls[0][1]).toMatchObject({ biography: 'Новая биография' });
    expect(JSON.stringify(updateMock.mock.calls[0][1])).not.toContain('auto_harvest');
  });

  it('clears the flag of a flagged author without a QID', async () => {
    const author = makeAuthor({ external_source: null, external_id: null, auto_harvest: true });
    getMock.mockResolvedValue(author);
    updateMock.mockResolvedValue({ ...author, auto_harvest: false });
    renderView();

    fireEvent.click(await screen.findByLabelText('Автоматически пополнять книги из Wikidata'));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => expect(updateMock).toHaveBeenCalled());
    expect(updateMock.mock.calls[0][1]).toMatchObject({ auto_harvest: false });
  });

  it('shows a save error inline and stays on the page', async () => {
    getMock.mockResolvedValue(makeAuthor());
    updateMock.mockRejectedValue(new ApiError(409, 'Conflict', 'author slug already exists'));
    renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Сохранить' }));

    expect(await screen.findByText('author slug already exists')).toBeInTheDocument();
    expect(pushMock).not.toHaveBeenCalled();
    expect(consumeFlash('catalog.authors')).toBeNull();
  });

  it('lists films with «Документальный» and «Скрыт» marks and switches hide / show', async () => {
    getMock.mockResolvedValue(makeAuthor());
    filmsMock.mockResolvedValue({
      items: [
        makeFilm(),
        makeFilm({ id: 'w2', title: 'Волшебство слов', kind: 'tv_film', relation: 'documentary', year: 2011, director: null, hidden: true }),
      ],
      total: 2,
    });
    setHiddenMock.mockResolvedValue(makeFilm({ hidden: true }));
    renderView();

    const section = (await screen.findByRole('heading', { name: 'Фильмы об авторе' })).closest('section') as HTMLElement;
    expect(await within(section).findByText('Толкин')).toBeInTheDocument();
    expect(within(section).getByText('2019 · Фильм · Доме Каруко')).toBeInTheDocument();
    expect(within(section).getAllByText('Документальный', { ignore: 'option' })).toHaveLength(1);
    expect(within(section).getAllByText('Скрыт')).toHaveLength(1);
    expect(within(section).getByRole('button', { name: 'Показать' })).toBeInTheDocument();

    filmsMock.mockResolvedValue({
      items: [makeFilm({ hidden: true }), makeFilm({ id: 'w2', title: 'Волшебство слов', relation: 'documentary', hidden: true })],
      total: 2,
    });
    fireEvent.click(within(section).getByRole('button', { name: 'Скрыть' }));
    await waitFor(() => expect(setHiddenMock).toHaveBeenCalledWith('a1', 'w1', true));
    await waitFor(() => expect(within(section).getAllByText('Скрыт')).toHaveLength(2));
    expect(within(section).queryByRole('button', { name: 'Скрыть' })).not.toBeInTheDocument();
    expect(within(section).getAllByRole('button', { name: 'Показать' })).toHaveLength(2);
  });

  it('shows the films empty text, the load error and the visibility error', async () => {
    getMock.mockResolvedValue(makeAuthor());
    const first = renderView();
    expect(await screen.findByText('Фильмов об авторе пока нет.')).toBeInTheDocument();
    first.unmount();

    filmsMock.mockRejectedValue(new ApiError(500, 'Server Error', 'boom'));
    const second = renderView();
    expect(await screen.findByText('Не удалось загрузить фильмы об авторе.')).toBeInTheDocument();
    second.unmount();

    filmsMock.mockResolvedValue({ items: [makeFilm()], total: 1 });
    setHiddenMock.mockRejectedValue(new ApiError(500, 'Server Error', 'boom'));
    renderView();
    fireEvent.click(await screen.findByRole('button', { name: 'Скрыть' }));
    expect(await screen.findByText('Не удалось изменить видимость фильма.')).toBeInTheDocument();
  });

  it('offers the film sync only for a QID author and shows the result', async () => {
    getMock.mockResolvedValue(makeAuthor());
    const first = renderView();
    await screen.findByText('Фильмов об авторе пока нет.');
    expect(screen.queryByRole('button', { name: 'Найти фильмы об авторе в Wikidata' })).not.toBeInTheDocument();
    first.unmount();

    getMock.mockResolvedValue(makeAuthor({ external_source: 'wikidata', external_id: 'Q892' }));
    syncFilmsMock.mockResolvedValue({
      processed: 1,
      updated: 1,
      added: 2,
      items: [{ author_id: 'a1', name: 'Лев Толстой', outcome: 'added', found: 2, added: 2, notes: [], message: '' }],
    });
    const { invalidateSpy } = renderView();
    fireEvent.click(await screen.findByRole('button', { name: 'Найти фильмы об авторе в Wikidata' }));
    await waitFor(() => expect(syncFilmsMock).toHaveBeenCalledWith(['a1']));
    expect(await screen.findByText('Автор «Лев Толстой»: добавлено фильмов — 2.')).toBeInTheDocument();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'authors'] });
  });

  it('«Отмена» returns to back without saving', async () => {
    getMock.mockResolvedValue(makeAuthor());
    renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Отмена' }));
    expect(pushMock).toHaveBeenCalledWith(BACK);
    expect(updateMock).not.toHaveBeenCalled();
  });

  it.each(['//evil.com', 'https://evil.com/catalog/authors', '/\\evil.com', '/catalog/authors/a1/edit', '/login'])(
    'a malicious back %j resolves to /catalog/authors',
    async (back) => {
      searchParamsValue = new URLSearchParams({ back });
      getMock.mockResolvedValue(makeAuthor());
      updateMock.mockResolvedValue(makeAuthor());
      renderView();

      expect(screen.getByRole('link', { name: '← Назад' })).toHaveAttribute('href', '/catalog/authors');
      fireEvent.click(await screen.findByRole('button', { name: 'Отмена' }));
      expect(pushMock).toHaveBeenLastCalledWith('/catalog/authors');

      fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
      await waitFor(() => expect(updateMock).toHaveBeenCalled());
      await waitFor(() => expect(pushMock).toHaveBeenCalledTimes(2));
      expect(pushMock).toHaveBeenLastCalledWith('/catalog/authors');
    }
  );
});
