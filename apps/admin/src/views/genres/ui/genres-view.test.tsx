import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminGenre } from '@bookhaven/api-client';
import { GenresView } from './genres-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/catalog/genres',
  useSearchParams: () => searchParamsValue,
}));

const listMock = vi.fn();
const bulkStatusMock = vi.fn();

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: {
    list: (...args: unknown[]) => listMock(...args),
    bulkStatus: (...args: unknown[]) => bulkStatusMock(...args),
  },
}));

function makeGenre(overrides: Partial<AdminGenre> = {}): AdminGenre {
  return {
    id: 'g1',
    name: 'роман',
    slug: 'roman',
    status: 'published',
    wikidata_qid: null,
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
      <GenresView />
    </QueryClientProvider>
  );
  return { invalidateSpy, ...utils };
}

describe('GenresView (catalog list)', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    listMock.mockReset();
    bulkStatusMock.mockReset();
    window.sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows a status badge and a linked QID per genre', async () => {
    listMock.mockResolvedValue({
      items: [
        makeGenre({ wikidata_qid: 'Q8261' }),
        makeGenre({ id: 'g2', name: 'ожидающий', slug: 'p', status: 'pending' }),
        makeGenre({ id: 'g3', name: 'отклонённый', slug: 'r', status: 'rejected' }),
      ],
      total: 3,
      page: 1,
      limit: 20,
    });

    renderView();

    await waitFor(() => expect(screen.getByText('ожидающий')).toBeInTheDocument());
    expect(screen.getByText('Опубликован', { selector: 'span' })).toBeInTheDocument();
    expect(screen.getByText('На модерации', { selector: 'span' })).toBeInTheDocument();
    expect(screen.getByText('Отклонён', { selector: 'span' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Q8261' })).toHaveAttribute(
      'href',
      'https://www.wikidata.org/wiki/Q8261'
    );
  });

  it('builds list params from the URL and writes the status back', async () => {
    searchParamsValue = new URLSearchParams('status=pending&page=3');
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });

    renderView();

    await waitFor(() =>
      expect(listMock).toHaveBeenCalledWith({ search: undefined, status: 'pending', page: 3, limit: 20 })
    );
    const filter = screen.getByLabelText('Статус') as HTMLSelectElement;
    expect(filter.value).toBe('pending');

    fireEvent.change(filter, { target: { value: 'rejected' } });
    expect(pushMock).toHaveBeenCalledWith('/catalog/genres?status=rejected');
  });

  it('links «Редактировать» to the edit page with back and has no inline form', async () => {
    listMock.mockResolvedValue({ items: [makeGenre()], total: 1, page: 1, limit: 20 });

    renderView();

    const edit = await screen.findByRole('link', { name: 'Редактировать' });
    expect(edit).toHaveAttribute('href', `/catalog/genres/g1/edit?back=${encodeURIComponent('/catalog/genres')}`);
    fireEvent.click(edit);
    expect(screen.queryByLabelText('Название *')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Сохранить' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Создать' })).not.toBeInTheDocument();
  });

  it('unpublishes a published genre → pending and invalidates genres + dashboard', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    listMock.mockResolvedValue({ items: [makeGenre()], total: 1, page: 1, limit: 20 });
    bulkStatusMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });

    const { invalidateSpy } = renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Снять с публикации' }));
    expect(window.confirm).toHaveBeenCalledWith(
      'Снять жанр «роман» с публикации? Он вернётся в очередь модерации.'
    );
    await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['g1'], 'pending'));
    expect(
      await screen.findByText('Жанр «роман» снят с публикации и возвращён на модерацию.')
    ).toBeInTheDocument();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'genres'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
  });

  it('links to the genre moderation queue from the header', async () => {
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    renderView();
    expect(screen.getByRole('link', { name: 'Очередь модерации' })).toHaveAttribute(
      'href',
      '/moderation/genres'
    );
  });
});
