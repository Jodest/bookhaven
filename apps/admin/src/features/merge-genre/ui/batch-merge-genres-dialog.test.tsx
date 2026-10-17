import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AdminGenre, ApiError } from '@bookhaven/api-client';
import { BatchMergeGenresDialog } from './batch-merge-genres-dialog';

const listMock = vi.fn();
const mergeBatchMock = vi.fn();
const previewMock = vi.fn();

// The preview summary and the genre search live in entities/genre and use its
// api module, so the mock targets that module (the entity index re-exports it).
vi.mock('@/entities/genre/api', () => ({
  adminGenresApi: {
    list: (...args: unknown[]) => listMock(...args),
    mergeBatch: (...args: unknown[]) => mergeBatchMock(...args),
    mergePreview: (...args: unknown[]) => previewMock(...args),
  },
}));

const romany: AdminGenre = { id: 'g1', name: 'романы', slug: 'romany', status: 'pending', wikidata_qid: null };
const roman: AdminGenre = { id: 'g2', name: 'роман', slug: 'roman', status: 'published', wikidata_qid: null };
const epopeya: AdminGenre = { id: 'g3', name: 'роман-эпопея', slug: 'roman-epopeya', status: 'pending', wikidata_qid: null };
const proza: AdminGenre = { id: 'g4', name: 'проза', slug: 'proza', status: 'published', wikidata_qid: null };
const musor: AdminGenre = { id: 'g5', name: 'мусор', slug: 'musor', status: 'rejected', wikidata_qid: null };

function renderDialog(genres: AdminGenre[] = [romany, roman, epopeya]) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  const onClose = vi.fn();
  const onMerged = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <BatchMergeGenresDialog genres={genres} onClose={onClose} onMerged={onMerged} />
    </QueryClientProvider>
  );
  return { onClose, onMerged, invalidateSpy };
}

const confirmName = (n: number, target: string) => `Слить ${n} в «${target}»`;

describe('BatchMergeGenresDialog', () => {
  beforeEach(() => {
    listMock.mockReset();
    mergeBatchMock.mockReset();
    previewMock.mockReset();
    previewMock.mockResolvedValue({ books_reassigned: 3, collections: [] });
    listMock.mockResolvedValue({ items: [romany, roman, proza, musor], total: 4, page: 1, limit: 20 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('lists the selection with radios and keeps confirm disabled until a target is chosen', () => {
    renderDialog();

    expect(screen.getByRole('heading', { name: 'Слить жанры (3)' })).toBeInTheDocument();
    expect(screen.getAllByRole('radio')).toHaveLength(4);
    expect(screen.getByLabelText(/Другой жанр…/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Слить 0 в/ })).toBeDisabled();
    expect(previewMock).not.toHaveBeenCalled();
  });

  it('merges the other selected genres, in order, into the radio-chosen target', async () => {
    mergeBatchMock.mockResolvedValue({
      genre: roman,
      sources_merged: 2,
      books_reassigned: 5,
      collections_switched: 1,
    });
    previewMock.mockResolvedValue({
      books_reassigned: 5,
      collections: [
        {
          id: 'c1',
          title: 'Лучшие романы',
          slug: 'luchshie-romany',
          kind: 'manual',
          is_published: true,
          source_genre: { id: 'g1', name: 'романы' },
        },
      ],
    });
    const { onMerged, invalidateSpy } = renderDialog();

    fireEvent.click(screen.getAllByRole('radio')[1]);
    expect(previewMock).toHaveBeenCalledWith(['g1', 'g3'], 'g2');
    expect(await screen.findByText('Будет перенесено книг: 5.')).toBeInTheDocument();
    expect(screen.getByText('Подборка «Лучшие романы» будет переключена на жанр «роман».')).toBeInTheDocument();

    const confirm = screen.getByRole('button', { name: confirmName(2, 'роман') });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);

    await waitFor(() => expect(onMerged).toHaveBeenCalledWith(expect.objectContaining({ sources_merged: 2 }), 'роман'));
    expect(mergeBatchMock).toHaveBeenCalledWith(['g1', 'g3'], 'g2');
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'genres'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'books'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'collections'] });
  });

  it('merges the whole selection into a genre outside it found via search', async () => {
    mergeBatchMock.mockResolvedValue({ genre: proza, sources_merged: 2, books_reassigned: 1, collections_switched: 0 });
    const { onMerged } = renderDialog([romany, epopeya]);

    fireEvent.click(screen.getByLabelText(/Другой жанр…/));
    fireEvent.change(screen.getByLabelText('Поиск целевого жанра'), { target: { value: 'про' } });
    expect(await screen.findByRole('button', { name: 'проза' })).toBeInTheDocument();
    // Selected genres are never offered as the target.
    expect(screen.queryByRole('button', { name: 'романы' })).not.toBeInTheDocument();
    expect(listMock).toHaveBeenCalledWith({ search: 'про', limit: 20 });
    fireEvent.click(screen.getByRole('button', { name: 'проза' }));

    const confirm = screen.getByRole('button', { name: confirmName(2, 'проза') });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);
    await waitFor(() => expect(onMerged).toHaveBeenCalled());
    expect(mergeBatchMock).toHaveBeenCalledWith(['g1', 'g3'], 'g4');
  });

  it('keeps confirm disabled when the only selected genre is the target', () => {
    renderDialog([romany]);

    fireEvent.click(screen.getAllByRole('radio')[0]);
    expect(screen.getByRole('button', { name: /^Слить 0 в/ })).toBeDisabled();
    expect(previewMock).not.toHaveBeenCalled();
  });

  it('warns when the target is rejected', async () => {
    renderDialog([romany]);

    fireEvent.click(screen.getByLabelText(/Другой жанр…/));
    fireEvent.change(screen.getByLabelText('Поиск целевого жанра'), { target: { value: 'му' } });
    fireEvent.click(await screen.findByRole('button', { name: 'мусор' }));
    expect(screen.getByText('Целевой жанр отклонён — книги окажутся в скрытом жанре.')).toBeInTheDocument();
  });

  it('gates confirm on the preview', async () => {
    let resolvePreview: (v: unknown) => void = () => {};
    previewMock.mockReturnValue(new Promise((resolve) => (resolvePreview = resolve)));
    renderDialog();

    fireEvent.click(screen.getAllByRole('radio')[1]);
    expect(screen.getByText('Считаем последствия…')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: confirmName(2, 'роман') })).toBeDisabled();

    resolvePreview({ books_reassigned: 1, collections: [] });
    await waitFor(() => expect(screen.getByRole('button', { name: confirmName(2, 'роман') })).toBeEnabled());
  });

  it.each([
    [400, 'Некорректный выбор жанров для слияния.'],
    [404, 'Один из жанров не найден'],
    [500, 'Не удалось выполнить слияние. Попробуйте позже.'],
  ])('shows the error text for a failed merge with status %i', async (status, text) => {
    mergeBatchMock.mockRejectedValue(new ApiError(status, 'err', 'err'));
    const { onMerged } = renderDialog();

    fireEvent.click(screen.getAllByRole('radio')[1]);
    const confirm = screen.getByRole('button', { name: confirmName(2, 'роман') });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);

    await waitFor(() => expect(screen.getByText(new RegExp(text))).toBeInTheDocument());
    expect(onMerged).not.toHaveBeenCalled();
  });
});
