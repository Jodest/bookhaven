import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type AdminGenre } from '@bookhaven/api-client';
import { GenresView } from './genres-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/catalog/genres',
  useSearchParams: () => searchParamsValue,
}));

const listMock = vi.fn();
const createMock = vi.fn();
const updateMock = vi.fn();

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: {
    list: (...args: unknown[]) => listMock(...args),
    create: (...args: unknown[]) => createMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
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
  return render(
    <QueryClientProvider client={queryClient}>
      <GenresView />
    </QueryClientProvider>
  );
}

describe('GenresView status and QID', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    listMock.mockReset();
    createMock.mockReset();
    updateMock.mockReset();
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

  it('reads the status filter from the URL and writes it back on change', async () => {
    searchParamsValue = new URLSearchParams('status=pending');
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });

    renderView();

    await waitFor(() =>
      expect(listMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'pending' }))
    );
    const filter = screen.getByLabelText('Статус') as HTMLSelectElement;
    expect(filter.value).toBe('pending');

    fireEvent.change(filter, { target: { value: 'rejected' } });
    expect(pushMock).toHaveBeenCalledWith('/catalog/genres?status=rejected&page=1');
  });

  it('sends QID on create (no status) and status + QID on edit', async () => {
    listMock.mockResolvedValue({
      items: [makeGenre({ status: 'pending', wikidata_qid: 'Q8261' })],
      total: 1,
      page: 1,
      limit: 20,
    });
    createMock.mockResolvedValue(makeGenre());
    updateMock.mockResolvedValue(makeGenre());

    renderView();

    // Create form: no status select, QID sent.
    expect(screen.queryByLabelText('Статус', { selector: '#status' })).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'повесть' } });
    fireEvent.change(screen.getByLabelText('Wikidata QID'), { target: { value: 'Q49084' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
    await waitFor(() => expect(createMock).toHaveBeenCalled());
    expect(createMock.mock.calls[0][0]).toEqual(
      expect.objectContaining({ name: 'повесть', wikidata_qid: 'Q49084' })
    );
    expect(createMock.mock.calls[0][0]).not.toHaveProperty('status');
    await waitFor(() => expect(screen.getByText('Жанр создан')).toBeInTheDocument());

    // Edit form: status + QID pre-filled; clearing the QID sends "".
    fireEvent.click(await screen.findByRole('button', { name: 'Редактировать' }));
    const statusSelect = screen.getByLabelText('Статус', { selector: '#status' }) as HTMLSelectElement;
    expect(statusSelect.value).toBe('pending');
    const qidInput = screen.getByLabelText('Wikidata QID') as HTMLInputElement;
    expect(qidInput.value).toBe('Q8261');
    fireEvent.change(statusSelect, { target: { value: 'published' } });
    fireEvent.change(qidInput, { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() =>
      expect(updateMock).toHaveBeenCalledWith(
        'g1',
        expect.objectContaining({ status: 'published', wikidata_qid: '' })
      )
    );
  });

  it('links to the genre moderation queue from the header', async () => {
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    renderView();
    expect(screen.getByRole('link', { name: 'Очередь модерации' })).toHaveAttribute(
      'href',
      '/moderation/genres'
    );
  });

  it('extends a 409 QID conflict with a merged-genre hint and keeps other messages', async () => {
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    createMock.mockRejectedValueOnce(new ApiError(409, 'Conflict', 'genre wikidata qid already exists'));
    renderView();

    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'повесть' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
    await waitFor(() =>
      expect(
        screen.getByText('genre wikidata qid already exists (возможно, у слитого жанра)')
      ).toBeInTheDocument()
    );

    createMock.mockRejectedValueOnce(new ApiError(409, 'Conflict', 'slug already exists'));
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
    await waitFor(() => expect(screen.getByText('slug already exists')).toBeInTheDocument());
  });
});
