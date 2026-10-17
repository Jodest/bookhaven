import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminGenreMergeEntry, AdminGenreMergeUndoResult } from '@bookhaven/api-client';
import { GenreMergeHistoryView } from './genre-merge-history-view';

const replaceMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ replace: replaceMock, push: vi.fn() }),
  usePathname: () => '/moderation/genres/merges',
  useSearchParams: () => searchParamsValue,
}));

const mergesMock = vi.fn();

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: {
    merges: (...args: unknown[]) => mergesMock(...args),
  },
}));

vi.mock('@/features/undo-genre-merge', () => ({
  UndoGenreMergeDialog: (props: {
    mergeId: string;
    onClose: () => void;
    onUndone: (result: AdminGenreMergeUndoResult) => void;
  }) => (
    <div data-testid="undo-dialog">
      {props.mergeId}
      <button type="button" onClick={props.onClose}>
        close-undo
      </button>
      <button
        type="button"
        onClick={() =>
          props.onUndone({
            genre: {} as AdminGenreMergeUndoResult['genre'],
            sources_restored: 2,
            books_returned: 5,
            books_relinked: 0,
            collections_switched_back: props.mergeId === 'm-active' ? 0 : 1,
          })
        }
      >
        finish-undo
      </button>
    </div>
  ),
}));

const active: AdminGenreMergeEntry = {
  id: 'm-active',
  created_at: '2026-10-05T10:00:00Z',
  performed_by: { id: 'u1', name: 'Админ' },
  target: { id: 't1', name: 'роман' },
  sources: [
    { id: 's1', name: 'романы' },
    { id: 's2', name: 'роман-эпопея' },
  ],
  books_reassigned: 7,
  collections_switched: 0,
  undone_at: null,
  undone_by: null,
};

const undone: AdminGenreMergeEntry = {
  id: 'm-undone',
  created_at: '2026-10-01T10:00:00Z',
  performed_by: null,
  target: { id: 't2', name: 'проза' },
  sources: [{ id: 's3', name: 'прозы' }],
  books_reassigned: 1,
  collections_switched: 1,
  undone_at: '2026-10-02T10:00:00Z',
  undone_by: { id: 'u1', name: 'Админ' },
};

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <GenreMergeHistoryView />
    </QueryClientProvider>
  );
}

describe('GenreMergeHistoryView', () => {
  beforeEach(() => {
    replaceMock.mockReset();
    mergesMock.mockReset();
    searchParamsValue = new URLSearchParams();
    mergesMock.mockResolvedValue({ items: [active, undone], total: 2, page: 1, limit: 20 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('lists merges with the merge, performer and status columns', async () => {
    renderView();
    expect(await screen.findByText('«романы», «роман-эпопея» → «роман»')).toBeInTheDocument();
    expect(mergesMock).toHaveBeenCalledWith({ page: 1, limit: 20 });
    expect(screen.getByRole('heading', { name: 'История слияний жанров' })).toBeInTheDocument();

    const activeRow = screen.getByText('«романы», «роман-эпопея» → «роман»').closest('tr') as HTMLElement;
    expect(within(activeRow).getByText('Админ')).toBeInTheDocument();
    expect(within(activeRow).getByText('7')).toBeInTheDocument();
    expect(within(activeRow).getByText('Активно')).toBeInTheDocument();

    const undoneRow = screen.getByText('«прозы» → «проза»').closest('tr') as HTMLElement;
    expect(within(undoneRow).getByText('—')).toBeInTheDocument();
    const undoneDate = new Date(undone.undone_at as string).toLocaleDateString('ru-RU');
    expect(within(undoneRow).getByText(`Отменено ${undoneDate}`)).toBeInTheDocument();
  });

  it('offers the undo button only for merges that are not undone', async () => {
    renderView();
    await screen.findByText('«прозы» → «проза»');
    expect(screen.getAllByRole('button', { name: 'Отменить слияние' })).toHaveLength(1);
    const undoneRow = screen.getByText('«прозы» → «проза»').closest('tr') as HTMLElement;
    expect(within(undoneRow).queryByRole('button', { name: 'Отменить слияние' })).not.toBeInTheDocument();
  });

  it('puts ?undo=<id> in the URL from the row button', async () => {
    renderView();
    fireEvent.click(await screen.findByRole('button', { name: 'Отменить слияние' }));
    expect(replaceMock).toHaveBeenCalledWith('/moderation/genres/merges?undo=m-active');
  });

  it('shows the empty state', async () => {
    mergesMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    renderView();
    expect(await screen.findByText('Слияний пока нет.')).toBeInTheDocument();
  });

  it('shows the load error', async () => {
    mergesMock.mockRejectedValue(new Error('boom'));
    renderView();
    expect(await screen.findByText('Не удалось загрузить историю слияний.')).toBeInTheDocument();
  });

  it('opens the undo dialog for ?undo=<id> and clears the param on close', async () => {
    searchParamsValue = new URLSearchParams('undo=m-active');
    renderView();
    expect(await screen.findByTestId('undo-dialog')).toHaveTextContent('m-active');

    fireEvent.click(screen.getByRole('button', { name: 'close-undo' }));
    expect(replaceMock).toHaveBeenCalledWith('/moderation/genres/merges');
  });

  it('shows the success banner without the collections suffix', async () => {
    searchParamsValue = new URLSearchParams('undo=m-active');
    renderView();
    fireEvent.click(await screen.findByRole('button', { name: 'finish-undo' }));
    expect(
      await screen.findByText('Слияние отменено. Восстановлено жанров: 2. Вернулось книг: 5.')
    ).toBeInTheDocument();
    expect(replaceMock).toHaveBeenCalledWith('/moderation/genres/merges');
  });

  it('shows the success banner with the collections suffix', async () => {
    searchParamsValue = new URLSearchParams('undo=m-other');
    renderView();
    fireEvent.click(await screen.findByRole('button', { name: 'finish-undo' }));
    await waitFor(() =>
      expect(
        screen.getByText('Слияние отменено. Восстановлено жанров: 2. Вернулось книг: 5. Возвращено подборок: 1.')
      ).toBeInTheDocument()
    );
  });
});
