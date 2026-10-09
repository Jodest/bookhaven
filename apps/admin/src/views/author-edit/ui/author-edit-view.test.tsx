import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type AdminAuthor } from '@bookhaven/api-client';
import { consumeFlash } from '@/shared/lib';
import { AuthorEditView } from './author-edit-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  useParams: () => ({ id: 'a1' }),
  useSearchParams: () => searchParamsValue,
}));

const getMock = vi.fn();
const updateMock = vi.fn();

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    get: (...args: unknown[]) => getMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
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
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'authors'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
    expect(consumeFlash('catalog.authors')).toBe('Автор «Лев Николаевич Толстой» сохранён.');
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
