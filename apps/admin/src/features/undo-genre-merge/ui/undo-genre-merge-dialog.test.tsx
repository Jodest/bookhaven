import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AdminGenreMergeEntry, ApiError } from '@bookhaven/api-client';
import { UndoGenreMergeDialog } from './undo-genre-merge-dialog';

const previewMock = vi.fn();
const undoMock = vi.fn();

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: {
    undoMergePreview: (...args: unknown[]) => previewMock(...args),
    undoMerge: (...args: unknown[]) => undoMock(...args),
  },
}));

const merge: AdminGenreMergeEntry = {
  id: 'merge-1',
  created_at: '2026-10-01T10:00:00Z',
  performed_by: { id: 'u1', name: 'Админ' },
  target: { id: 'tgt', name: 'роман' },
  sources: [{ id: 'src', name: 'романы' }],
  books_reassigned: 4,
  collections_switched: 1,
  undone_at: null,
  undone_by: null,
};

const preview = { merge, books_returned: 4, books_relinked: 0, collections: [] };

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  const onClose = vi.fn();
  const onUndone = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <UndoGenreMergeDialog mergeId="merge-1" onClose={onClose} onUndone={onUndone} />
    </QueryClientProvider>
  );
  return { onClose, onUndone, invalidateSpy };
}

const undoButton = () => screen.getByRole('button', { name: 'Отменить слияние' });

describe('UndoGenreMergeDialog', () => {
  beforeEach(() => {
    previewMock.mockReset();
    undoMock.mockReset();
    previewMock.mockResolvedValue(preview);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows the loading text and keeps the button disabled until the preview loads', async () => {
    renderDialog();
    expect(screen.getByText('Считаем последствия…')).toBeInTheDocument();
    expect(undoButton()).toBeDisabled();
    await waitFor(() => expect(undoButton()).toBeEnabled());
    expect(screen.getByRole('heading', { name: 'Отменить слияние в «роман»' })).toBeInTheDocument();
    expect(previewMock).toHaveBeenCalledWith('merge-1');
  });

  it('shows the returned books and hides the relinked line when it is zero', async () => {
    renderDialog();
    expect(await screen.findByText('Вернётся книг в исходные жанры: 4.')).toBeInTheDocument();
    expect(screen.queryByText(/Книг, которые снова получат исходный жанр/)).not.toBeInTheDocument();
  });

  it('shows relinked books and the collections that return', async () => {
    previewMock.mockResolvedValue({
      ...preview,
      books_relinked: 2,
      collections: [
        {
          id: 'c1',
          title: 'Лучшие романы',
          slug: 'best',
          kind: 'manual',
          is_published: true,
          source_genre: { id: 'src', name: 'романы' },
        },
      ],
    });
    renderDialog();
    expect(
      await screen.findByText('Книг, которые снова получат исходный жанр, оставаясь в «роман»: 2.')
    ).toBeInTheDocument();
    expect(screen.getByText('Подборка «Лучшие романы» вернётся к жанру «романы».')).toBeInTheDocument();
  });

  it('undoes the merge, invalidates the caches and reports the result', async () => {
    const result = { genre: {}, sources_restored: 1, books_returned: 4, books_relinked: 0, collections_switched_back: 1 };
    undoMock.mockResolvedValue(result);
    const { onUndone, invalidateSpy } = renderDialog();
    await waitFor(() => expect(undoButton()).toBeEnabled());

    fireEvent.click(undoButton());
    await waitFor(() => expect(onUndone).toHaveBeenCalledWith(result));
    expect(undoMock).toHaveBeenCalledWith('merge-1');
    const keys = invalidateSpy.mock.calls.map((c) => JSON.stringify((c[0] as { queryKey: unknown }).queryKey));
    expect(keys).toContain(JSON.stringify(['admin', 'genres']));
    expect(keys).toContain(JSON.stringify(['admin', 'books']));
  });

  it('closes with "Закрыть"', async () => {
    const { onClose } = renderDialog();
    await waitFor(() => expect(undoButton()).toBeEnabled());
    fireEvent.click(screen.getByRole('button', { name: 'Закрыть' }));
    expect(onClose).toHaveBeenCalled();
  });

  const conflicts: [string, string][] = [
    ['already_undone', 'Это слияние уже отменено.'],
    ['target_missing', 'Целевой жанр удалён — отменить слияние нельзя.'],
    ['target_merged', 'Целевой жанр с тех пор слит с другим жанром — сначала отмените более позднее слияние.'],
    ['source_missing', 'Один из исходных жанров удалён — отменить слияние нельзя.'],
    ['source_not_alias', 'Один из исходных жанров больше не связан с целевым — отменить слияние нельзя.'],
    ['qid_conflict', 'QID одного из исходных жанров теперь занят другим жанром — отменить слияние нельзя.'],
  ];

  it.each(conflicts)('shows the text of a blocked preview (%s) and keeps the button disabled', async (code, text) => {
    previewMock.mockRejectedValue(new ApiError(409, 'Conflict', 'blocked', '', code));
    renderDialog();
    expect(await screen.findByText(text)).toBeInTheDocument();
    expect(undoButton()).toBeDisabled();
  });

  it.each(conflicts)('shows the text of a blocked undo (%s)', async (code, text) => {
    undoMock.mockRejectedValue(new ApiError(409, 'Conflict', 'blocked', '', code));
    const { onUndone } = renderDialog();
    await waitFor(() => expect(undoButton()).toBeEnabled());
    fireEvent.click(undoButton());
    expect(await screen.findByText(text)).toBeInTheDocument();
    expect(onUndone).not.toHaveBeenCalled();
  });

  it('shows the 404 text', async () => {
    previewMock.mockRejectedValue(new ApiError(404, 'Not Found', 'genre merge not found'));
    renderDialog();
    expect(await screen.findByText('Слияние не найдено. Обновите список.')).toBeInTheDocument();
  });

  it('shows the generic text for other errors', async () => {
    undoMock.mockRejectedValue(new ApiError(500, 'Server Error', 'boom'));
    renderDialog();
    await waitFor(() => expect(undoButton()).toBeEnabled());
    fireEvent.click(undoButton());
    expect(await screen.findByText('Не удалось отменить слияние. Попробуйте позже.')).toBeInTheDocument();
  });
});
