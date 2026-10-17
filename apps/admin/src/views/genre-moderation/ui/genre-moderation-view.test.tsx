import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminGenre, AdminGenreQueueItem } from '@bookhaven/api-client';
import { GenreModerationView } from './genre-moderation-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/moderation/genres',
  useSearchParams: () => searchParamsValue,
}));

const queueMock = vi.fn();
const bulkStatusMock = vi.fn();

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: {
    moderationQueue: (...args: unknown[]) => queueMock(...args),
    bulkStatus: (...args: unknown[]) => bulkStatusMock(...args),
  },
}));

vi.mock('@/features/merge-genre', () => ({
  BatchMergeGenresDialog: (props: {
    genres: AdminGenre[];
    onMerged: (result: unknown, targetName: string) => void;
  }) => (
    <div data-testid="batch-merge-dialog">
      {props.genres.map((g) => g.name).join(',')}
      <button
        type="button"
        onClick={() =>
          props.onMerged(
            { genre: props.genres[0], sources_merged: 2, books_reassigned: 5, collections_switched: 1 },
            'роман'
          )
        }
      >
        finish-batch
      </button>
    </div>
  ),
  MergeGenreDialog: (props: { source: AdminGenre; initialCanonical?: AdminGenre }) => (
    <div data-testid="merge-dialog">
      {props.source.name}|{props.initialCanonical?.name ?? ''}
    </div>
  ),
}));

function makeGenre(overrides: Partial<AdminGenre> = {}): AdminGenre {
  return {
    id: 'g1',
    name: 'романы',
    slug: 'romany',
    status: 'pending',
    wikidata_qid: 'Q123',
    ...overrides,
  };
}

function makeItem(overrides: Partial<AdminGenreQueueItem> = {}): AdminGenreQueueItem {
  return {
    genre: makeGenre(),
    created_at: '2026-10-08T10:00:00Z',
    books_total: 3,
    books_visible: 1,
    sample_books: [
      { id: 'b1', title: 'Война и мир', status: 'published' },
      { id: 'b2', title: 'Анна Каренина', status: 'pending' },
      { id: 'b3', title: 'Воскресение', status: 'rejected' },
    ],
    similar: [
      {
        genre: makeGenre({ id: 'h1', name: 'роман', slug: 'roman', status: 'published', wikidata_qid: 'Q8261' }),
        match: 'same_stem',
      },
    ],
    ...overrides,
  };
}

const secondItem = makeItem({
  genre: makeGenre({ id: 'g2', name: 'эпос', slug: 'epos', wikidata_qid: null }),
  books_total: 0,
  books_visible: 0,
  sample_books: [],
  similar: [],
});

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <GenreModerationView />
    </QueryClientProvider>
  );
  return { invalidateSpy, ...utils };
}

function rowOf(name: string) {
  return screen.getByText(name, { selector: 'span.font-medium' }).closest('tr') as HTMLElement;
}

describe('GenreModerationView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    queueMock.mockReset();
    bulkStatusMock.mockReset();
    queueMock.mockResolvedValue({ items: [makeItem(), secondItem], total: 2, page: 1, limit: 20 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('defaults to the pending queue and renders QID link, books summary and hints', async () => {
    renderView();

    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());
    expect(queueMock).toHaveBeenCalledWith(
      expect.objectContaining({ status: 'pending', page: 1, limit: 20 })
    );

    const row = rowOf('романы');
    expect(within(row).getByText('romany')).toBeInTheDocument();
    const qidLink = within(row).getByRole('link', { name: 'Q123' });
    expect(qidLink).toHaveAttribute('href', 'https://www.wikidata.org/wiki/Q123');
    expect(qidLink).toHaveAttribute('target', '_blank');
    expect(qidLink).toHaveAttribute('rel', 'noopener noreferrer');

    expect(within(row).getByText('3 (видимых 1)')).toBeInTheDocument();
    expect(within(row).getByRole('link', { name: 'Война и мир' })).toHaveAttribute(
      'href',
      '/catalog/books/b1/edit'
    );
    expect(within(row).getByText('(на модерации)')).toBeInTheDocument();
    expect(within(row).getByText('(отклонена)')).toBeInTheDocument();
    expect(within(row).getAllByText(/^\((на модерации|отклонена)\)$/)).toHaveLength(2);

    expect(within(row).getByText('роман')).toBeInTheDocument();
    expect(within(row).getByText('(та же основа)')).toBeInTheDocument();
    expect(within(row).getByRole('link', { name: 'Q8261' })).toBeInTheDocument();
    expect(within(row).getByText(/разные QID/)).toBeInTheDocument();

    const second = rowOf('эпос');
    expect(within(second).getByText('0 (видимых 0)')).toBeInTheDocument();
    expect(within(second).getAllByText('—', { selector: 'span' })).toHaveLength(2);
  });

  it('shows status-dependent per-row actions', async () => {
    queueMock.mockResolvedValue({
      items: [
        makeItem({ similar: [] }),
        makeItem({ genre: makeGenre({ id: 'r1', name: 'отклонённый', status: 'rejected' }), similar: [] }),
        makeItem({ genre: makeGenre({ id: 'p1', name: 'опубликованный', status: 'published' }), similar: [] }),
      ],
      total: 3,
      page: 1,
      limit: 20,
    });
    renderView();
    await waitFor(() => expect(screen.getByText('отклонённый')).toBeInTheDocument());

    const pendingRow = rowOf('романы');
    expect(within(pendingRow).getByRole('button', { name: 'Опубликовать' })).toBeInTheDocument();
    expect(within(pendingRow).getByRole('button', { name: 'Отклонить' })).toBeInTheDocument();
    expect(within(pendingRow).getByRole('button', { name: 'Слить' })).toBeInTheDocument();

    const rejectedRow = rowOf('отклонённый');
    expect(within(rejectedRow).getByRole('button', { name: 'Вернуть на модерацию' })).toBeInTheDocument();
    expect(within(rejectedRow).getByRole('button', { name: 'Опубликовать' })).toBeInTheDocument();
    expect(within(rejectedRow).getByRole('button', { name: 'Слить' })).toBeInTheDocument();
    expect(within(rejectedRow).queryByRole('button', { name: 'Отклонить' })).not.toBeInTheDocument();

    const publishedRow = rowOf('опубликованный');
    expect(within(publishedRow).queryByRole('button', { name: 'Опубликовать' })).not.toBeInTheDocument();
    expect(within(publishedRow).getByRole('button', { name: 'Отклонить' })).toBeInTheDocument();
    expect(within(publishedRow).getByRole('button', { name: 'Слить' })).toBeInTheDocument();
  });

  it('publishes a single genre after confirmation and invalidates genres + dashboard', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });
    const { invalidateSpy } = renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    fireEvent.click(within(rowOf('романы')).getByRole('button', { name: 'Опубликовать' }));

    expect(confirmSpy).toHaveBeenCalledWith('Опубликовать жанр «романы»?');
    await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['g1'], 'published'));
    await waitFor(() => expect(screen.getByText('Жанр «романы» опубликован.')).toBeInTheDocument());
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'genres'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
  });

  it('reject confirm names the genre and mentions visible books; cancelling does nothing', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    fireEvent.click(within(rowOf('романы')).getByRole('button', { name: 'Отклонить' }));

    const text = confirmSpy.mock.calls[0][0] as string;
    expect(text).toContain('«романы»');
    expect(text).toContain('опубликованных книг (1)');
    expect(bulkStatusMock).not.toHaveBeenCalled();

    confirmSpy.mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });
    fireEvent.click(within(rowOf('эпос')).getByRole('button', { name: 'Отклонить' }));
    expect(confirmSpy.mock.calls[1][0]).not.toContain('опубликованных книг');
    await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['g2'], 'rejected'));
  });

  it('bulk-publishes the selected rows and reports "Обновлено X из N" with errors', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({
      processed: 2,
      updated: 1,
      errors: [{ genre_id: 'g2', name: '', message: 'genre not found' }],
    });
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    expect(screen.queryByRole('button', { name: /Опубликовать выбранные/ })).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Выбрать все на странице'));
    expect(screen.getByLabelText('Выбрать «романы»')).toBeChecked();
    expect(screen.getByLabelText('Выбрать «эпос»')).toBeChecked();

    fireEvent.click(screen.getByRole('button', { name: 'Опубликовать выбранные (2)' }));

    await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['g1', 'g2'], 'published'));
    await waitFor(() =>
      expect(
        screen.getByText('Обновлено 1 из 2. Ошибок: 1 («эпос»: genre not found).')
      ).toBeInTheDocument()
    );
    expect(screen.getByLabelText('Выбрать «романы»')).not.toBeChecked();
    expect(screen.queryByRole('button', { name: /Опубликовать выбранные/ })).not.toBeInTheDocument();
  });

  it('bulk reject confirm sums visible books', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText('Выбрать все на странице'));
    fireEvent.click(screen.getByRole('button', { name: 'Отклонить выбранные (2)' }));
    expect(confirmSpy.mock.calls[0][0]).toContain('опубликованных книг (1)');
    expect(bulkStatusMock).not.toHaveBeenCalled();
  });

  it('offers "Вернуть на модерацию (N)" on the rejected tab', async () => {
    searchParamsValue = new URLSearchParams('status=rejected');
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });
    queueMock.mockResolvedValue({
      items: [makeItem({ genre: makeGenre({ status: 'rejected' }), similar: [] })],
      total: 1,
      page: 1,
      limit: 20,
    });
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());
    expect(queueMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'rejected' }));

    fireEvent.click(screen.getByLabelText('Выбрать «романы»'));
    expect(screen.queryByRole('button', { name: /Отклонить выбранные/ })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Вернуть на модерацию (1)' }));

    await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['g1'], 'pending'));
    await waitFor(() => expect(screen.getByText('Обновлено 1 из 1.')).toBeInTheDocument());
  });

  it('clears the selection and updates URL params on filter change', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText('Выбрать «эпос»'));
    expect(screen.getByRole('button', { name: 'Отклонить выбранные (1)' })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('QID'), { target: { value: 'without' } });
    expect(pushMock).toHaveBeenCalledWith('/moderation/genres?qid=without');
    expect(screen.getByLabelText('Выбрать «эпос»')).not.toBeChecked();
    expect(screen.queryByRole('button', { name: /выбранные/ })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Отклонённые' }));
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/genres?status=rejected');

    fireEvent.change(screen.getByLabelText('Поиск'), { target: { value: 'роман' } });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));
    expect(pushMock).toHaveBeenLastCalledWith(`/moderation/genres?search=${encodeURIComponent('роман')}`);
  });

  it('reads status, qid, search and page from the URL', async () => {
    searchParamsValue = new URLSearchParams('status=published&qid=with&search=ром&page=2');
    queueMock.mockResolvedValue({ items: [], total: 0, page: 2, limit: 20 });
    renderView();
    await waitFor(() => expect(screen.getByText('Жанры не найдены')).toBeInTheDocument());
    expect(queueMock).toHaveBeenCalledWith({
      status: 'published',
      qid: 'with',
      search: 'ром',
      page: 2,
      limit: 20,
    });
  });

  it('opens the merge dialog prefilled from a hint', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    fireEvent.click(within(rowOf('романы')).getByRole('button', { name: 'Это он — слить' }));
    expect(screen.getByTestId('merge-dialog')).toHaveTextContent('романы|роман');
  });

  it('opens the merge dialog without a target from the row action', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    fireEvent.click(within(rowOf('эпос')).getByRole('button', { name: 'Слить' }));
    expect(screen.getByTestId('merge-dialog').textContent).toBe('эпос|');
  });

  it('offers "Слить выбранные" for the selection and opens the batch dialog in queue order', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /Слить выбранные/ })).not.toBeInTheDocument();

    // Select in reverse order: the dialog still gets the queue order.
    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать «эпос»' }));
    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать «романы»' }));
    fireEvent.click(screen.getByRole('button', { name: 'Слить выбранные (2)' }));
    expect(screen.getByTestId('batch-merge-dialog')).toHaveTextContent('романы,эпос');

    fireEvent.click(screen.getByRole('button', { name: 'finish-batch' }));
    await waitFor(() =>
      expect(
        screen.getByText('Слито жанров: 2 в «роман». Перенесено книг: 5. Переключено подборок: 1.')
      ).toBeInTheDocument()
    );
    expect(screen.queryByTestId('batch-merge-dialog')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Слить выбранные/ })).not.toBeInTheDocument();
  });

  it('shows the pending empty state', async () => {
    queueMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    renderView();
    await waitFor(() => expect(screen.getByText('Нет жанров на модерации')).toBeInTheDocument());
  });
});
