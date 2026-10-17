import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AdminGenre, ApiError } from '@bookhaven/api-client';
import { MergeGenreDialog } from './merge-genre-dialog';

const listMock = vi.fn();
const mergeMock = vi.fn();
const previewMock = vi.fn();

// The preview summary and the genre search live in entities/genre and use its
// api module, so the mock targets that module (the entity index re-exports it).
vi.mock('@/entities/genre/api', () => ({
  adminGenresApi: {
    list: (...args: unknown[]) => listMock(...args),
    merge: (...args: unknown[]) => mergeMock(...args),
    mergePreview: (...args: unknown[]) => previewMock(...args),
  },
}));

const source: AdminGenre = { id: 'src-1', name: 'романы', slug: 'romany', status: 'pending', wikidata_qid: 'Q123' };
const target: AdminGenre = { id: 'tgt-1', name: 'роман', slug: 'roman', status: 'published', wikidata_qid: null };
const rejectedTarget: AdminGenre = { id: 'rej-1', name: 'мусор', slug: 'musor', status: 'rejected', wikidata_qid: null };

function renderDialog(initialCanonical?: AdminGenre) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  const onClose = vi.fn();
  const onMerged = vi.fn();
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <MergeGenreDialog
        source={source}
        initialCanonical={initialCanonical}
        onClose={onClose}
        onMerged={onMerged}
      />
    </QueryClientProvider>
  );
  return { onClose, onMerged, invalidateSpy, ...utils };
}

async function pickTarget(name: string) {
  fireEvent.change(screen.getByLabelText('Поиск целевого жанра'), { target: { value: 'ром' } });
  fireEvent.click(await screen.findByRole('button', { name }));
}

describe('MergeGenreDialog', () => {
  beforeEach(() => {
    listMock.mockReset();
    mergeMock.mockReset();
    previewMock.mockReset();
    previewMock.mockResolvedValue({ books_reassigned: 3, collections: [] });
    listMock.mockResolvedValue({ items: [source, target, rejectedTarget], total: 3, page: 1, limit: 20 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('keeps confirm disabled until a target is picked and explains the alias behavior', async () => {
    renderDialog();

    expect(screen.getByRole('heading', { name: 'Слить жанр «романы» в …' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Слить «романы» в/ })).toBeDisabled();

    await pickTarget('роман');
    expect(listMock).toHaveBeenCalledWith({ search: 'ром', limit: 20 });

    await waitFor(() => expect(screen.getByRole('button', { name: 'Слить «романы» в «роман»' })).toBeEnabled());
    expect(
      screen.getByText(
        'Книги жанра «романы» будут перенесены в «роман». «романы» исчезнет из списков, но его название и QID продолжат указывать на «роман» при будущих импортах. Слияние можно будет отменить в «Истории слияний».'
      )
    ).toBeInTheDocument();
    expect(screen.queryByText(/Целевой жанр отклонён/)).not.toBeInTheDocument();
  });

  it('never offers the source itself as a target and shows candidate status badges', async () => {
    renderDialog();
    fireEvent.change(screen.getByLabelText('Поиск целевого жанра'), { target: { value: 'ром' } });

    const candidate = await screen.findByRole('button', { name: 'роман' });
    expect(candidate).toHaveTextContent('Опубликован');
    expect(screen.getByRole('button', { name: 'мусор' })).toHaveTextContent('Отклонён');
    expect(screen.queryByRole('button', { name: 'романы' })).not.toBeInTheDocument();
  });

  it('warns when the target is rejected', async () => {
    renderDialog();
    await pickTarget('мусор');
    expect(
      screen.getByText('Целевой жанр отклонён — книги окажутся в скрытом жанре.')
    ).toBeInTheDocument();
  });

  it('preselects initialCanonical, merges and invalidates genres, books and dashboard', async () => {
    mergeMock.mockResolvedValue({
      genre: { ...target, wikidata_qid: 'Q123' },
      books_reassigned: 2,
      collections_switched: 1,
      merge_id: 'merge-1',
    });
    const { onMerged, invalidateSpy } = renderDialog(target);

    expect(screen.queryByLabelText('Поиск целевого жанра')).not.toBeInTheDocument();
    const confirmButton = screen.getByRole('button', { name: 'Слить «романы» в «роман»' });
    await waitFor(() => expect(confirmButton).toBeEnabled());

    fireEvent.click(confirmButton);
    await waitFor(() => expect(onMerged).toHaveBeenCalledWith(2, 1, 'merge-1'));
    expect(mergeMock).toHaveBeenCalledWith('src-1', 'tgt-1');
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'collections'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'genres'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'books'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
  });

  it('previews books and switched collections and gates confirm on the preview', async () => {
    let resolvePreview: (v: unknown) => void = () => {};
    previewMock.mockReturnValue(new Promise((resolve) => (resolvePreview = resolve)));
    renderDialog(target);

    expect(screen.getByText('Считаем последствия…')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Слить «романы» в «роман»' })).toBeDisabled();
    expect(previewMock).toHaveBeenCalledWith(['src-1'], 'tgt-1');

    resolvePreview({
      books_reassigned: 4,
      collections: [
        {
          id: 'c1',
          title: 'Лучшие романы',
          slug: 'luchshie-romany',
          kind: 'auto',
          is_published: true,
          source_genre: { id: 'src-1', name: 'романы' },
        },
      ],
    });
    expect(await screen.findByText('Будет перенесено книг: 4.')).toBeInTheDocument();
    expect(screen.getByText('Подборка «Лучшие романы» будет переключена на жанр «роман».')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Слить «романы» в «роман»' })).toBeEnabled());
  });

  it('shows the merge error text for a failed preview and keeps confirm disabled', async () => {
    previewMock.mockRejectedValue(new ApiError(404, 'Not Found', 'genre not found'));
    renderDialog(target);

    expect(await screen.findByText(/Один из жанров не найден/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Слить «романы» в «роман»' })).toBeDisabled();
  });

  it('lets the admin replace a preselected target', () => {
    renderDialog(target);
    fireEvent.click(screen.getByRole('button', { name: 'Изменить' }));
    expect(screen.getByLabelText('Поиск целевого жанра')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Слить «романы» в/ })).toBeDisabled();
  });

  it('shows an inline error on 400 and does not call onMerged', async () => {
    mergeMock.mockRejectedValue(new ApiError(400, 'Bad Request', 'cannot merge a genre into itself'));
    const { onMerged } = renderDialog(target);

    const confirmButton = screen.getByRole('button', { name: 'Слить «романы» в «роман»' });
    await waitFor(() => expect(confirmButton).toBeEnabled());
    fireEvent.click(confirmButton);
    await waitFor(() => expect(screen.getByText('Нельзя слить жанр с самим собой.')).toBeInTheDocument());
    expect(onMerged).not.toHaveBeenCalled();
  });

  it('shows an inline error on 404 (missing or already merged genre)', async () => {
    mergeMock.mockRejectedValue(new ApiError(404, 'Not Found', 'genre not found'));
    const { onMerged } = renderDialog(target);

    const confirmButton = screen.getByRole('button', { name: 'Слить «романы» в «роман»' });
    await waitFor(() => expect(confirmButton).toBeEnabled());
    fireEvent.click(confirmButton);
    await waitFor(() => expect(screen.getByText(/Один из жанров не найден/)).toBeInTheDocument());
    expect(onMerged).not.toHaveBeenCalled();
  });
});
