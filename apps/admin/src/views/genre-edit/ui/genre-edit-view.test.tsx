import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type AdminGenre } from '@bookhaven/api-client';
import { consumeFlash } from '@/shared/lib';
import { GenreEditView } from './genre-edit-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  useParams: () => ({ id: 'g1' }),
  useSearchParams: () => searchParamsValue,
}));

const getMock = vi.fn();
const updateMock = vi.fn();

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: {
    get: (...args: unknown[]) => getMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
  },
}));

const BACK = '/moderation/genres?status=published';

function makeGenre(overrides: Partial<AdminGenre> = {}): AdminGenre {
  return {
    id: 'g1',
    name: 'роман',
    slug: 'roman',
    status: 'published',
    wikidata_qid: 'Q8261',
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
      <GenreEditView />
    </QueryClientProvider>
  );
  return { invalidateSpy, ...utils };
}

describe('GenreEditView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams({ back: BACK });
    pushMock.mockReset();
    getMock.mockReset();
    updateMock.mockReset();
    window.sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows the header and a skeleton while loading', () => {
    getMock.mockReturnValue(new Promise(() => {}));
    renderView();

    expect(screen.getByRole('heading', { level: 1, name: 'Редактирование жанра' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '← Назад' })).toHaveAttribute('href', BACK);
    expect(screen.getByTestId('edit-loading')).toBeInTheDocument();
  });

  it.each([404, 400])('shows «Жанр не найден.» on %i with a «Назад» link', async (status) => {
    searchParamsValue = new URLSearchParams();
    getMock.mockRejectedValue(new ApiError(status, 'Error', 'genre not found'));
    renderView();

    expect(await screen.findByText('Жанр не найден.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '← Назад' })).toHaveAttribute('href', '/catalog/genres');
  });

  it('shows a load error on other errors', async () => {
    getMock.mockRejectedValue(new Error('network'));
    renderView();

    expect(await screen.findByText('Не удалось загрузить данные.')).toBeInTheDocument();
  });

  it('prefills the form from the loaded genre', async () => {
    getMock.mockResolvedValue(makeGenre());
    renderView();

    expect(await screen.findByLabelText('Название *')).toHaveValue('роман');
    expect(getMock).toHaveBeenCalledWith('g1');
    expect(screen.getByLabelText('Slug')).toHaveValue('roman');
    expect(screen.getByLabelText('Wikidata QID')).toHaveValue('Q8261');
    expect((screen.getByLabelText('Статус', { selector: '#status' }) as HTMLSelectElement).value).toBe('published');
  });

  it('saves via update(id, input), sets the flash, invalidates and returns to back', async () => {
    getMock.mockResolvedValue(makeGenre());
    updateMock.mockResolvedValue(makeGenre({ name: 'исторический роман' }));
    const { invalidateSpy } = renderView();

    fireEvent.change(await screen.findByLabelText('Название *'), { target: { value: 'исторический роман' } });
    fireEvent.change(screen.getByLabelText('Wikidata QID'), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith(BACK));
    expect(updateMock).toHaveBeenCalledWith('g1', {
      name: 'исторический роман',
      slug: 'roman',
      status: 'published',
      wikidata_qid: '',
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'genres'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
    expect(consumeFlash('catalog.genres')).toBe('Жанр «исторический роман» сохранён.');
  });

  it('extends a 409 QID conflict with the merged-genre hint and stays on the page', async () => {
    getMock.mockResolvedValue(makeGenre());
    updateMock.mockRejectedValueOnce(new ApiError(409, 'Conflict', 'genre wikidata qid already exists'));
    renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Сохранить' }));

    expect(
      await screen.findByText('genre wikidata qid already exists (возможно, у слитого жанра)')
    ).toBeInTheDocument();
    expect(pushMock).not.toHaveBeenCalled();
  });

  it('«Отмена» returns to back without saving', async () => {
    getMock.mockResolvedValue(makeGenre());
    renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Отмена' }));
    expect(pushMock).toHaveBeenCalledWith(BACK);
    expect(updateMock).not.toHaveBeenCalled();
  });

  it.each(['//evil.com', 'javascript:alert(1)', '/%5Cevil.com', '/catalog/genresX', '/catalog/books'])(
    'a malicious back %j resolves to /catalog/genres',
    async (back) => {
      searchParamsValue = new URLSearchParams({ back });
      getMock.mockResolvedValue(makeGenre());
      updateMock.mockResolvedValue(makeGenre());
      renderView();

      expect(screen.getByRole('link', { name: '← Назад' })).toHaveAttribute('href', '/catalog/genres');
      fireEvent.click(await screen.findByRole('button', { name: 'Отмена' }));
      expect(pushMock).toHaveBeenLastCalledWith('/catalog/genres');

      fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
      await waitFor(() => expect(pushMock).toHaveBeenCalledTimes(2));
      expect(pushMock).toHaveBeenLastCalledWith('/catalog/genres');
    }
  );
});
