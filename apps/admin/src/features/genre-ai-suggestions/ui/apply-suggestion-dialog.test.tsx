import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type AdminGenre } from '@bookhaven/api-client';
import { ApplySuggestionDialog } from './apply-suggestion-dialog';
import { fantasy, fantasyEn, fentezi, mergeSuggestion, suggestionGenre } from '../test-fixtures';

const listMock = vi.fn();
const previewMock = vi.fn();
const applyMock = vi.fn();

vi.mock('@/entities/genre/api', () => ({
  adminGenresApi: {
    list: (...args: unknown[]) => listMock(...args),
    mergePreview: (...args: unknown[]) => previewMock(...args),
  },
}));

vi.mock('@/entities/genre-ai-suggestion/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre-ai-suggestion/api')>()),
  adminGenreAiSuggestionsApi: {
    apply: (...args: unknown[]) => applyMock(...args),
  },
}));

const scifi: AdminGenre = { id: 'g-scifi', name: 'Фантастика', slug: 'fantastika', status: 'published', wikidata_qid: null };

function renderDialog(suggestion = mergeSuggestion()) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const onClose = vi.fn();
  const onApplied = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <ApplySuggestionDialog suggestion={suggestion} onClose={onClose} onApplied={onApplied} />
    </QueryClientProvider>
  );
  return { onClose, onApplied };
}

const applyResult = {
  suggestion: mergeSuggestion({ status: 'applied' }),
  merge: { genre: scifi, sources_merged: 2, books_reassigned: 5, collections_switched: 1 },
  status_report: null,
};

describe('ApplySuggestionDialog', () => {
  beforeEach(() => {
    listMock.mockReset();
    previewMock.mockReset();
    applyMock.mockReset();
    previewMock.mockResolvedValue({ books_reassigned: 3, collections: [] });
    listMock.mockResolvedValue({ items: [scifi, fantasy], total: 2, page: 1, limit: 20 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('opens with the suggested target preselected and the title, radios and warning text', async () => {
    renderDialog();

    expect(screen.getByRole('heading', { name: 'Применить предложение: слить жанры (3)' })).toBeInTheDocument();
    expect(screen.getAllByRole('radio')).toHaveLength(4);
    expect(screen.getByRole('radio', { name: /Фэнтези/ })).toBeChecked();
    expect(screen.getByLabelText('Другой жанр…')).not.toBeChecked();
    expect(screen.getByText(/Действие необратимо/)).toBeInTheDocument();
    await waitFor(() =>
      expect(previewMock).toHaveBeenCalledWith([fentezi.id, fantasyEn.id], fantasy.id)
    );
  });

  it('keeps confirm disabled until the preview has loaded', async () => {
    let resolvePreview: (v: unknown) => void = () => {};
    previewMock.mockReturnValue(new Promise((resolve) => (resolvePreview = resolve)));
    renderDialog();

    const confirm = screen.getByRole('button', { name: 'Слить 2 в «Фэнтези»' });
    expect(confirm).toBeDisabled();

    resolvePreview({
      books_reassigned: 4,
      collections: [
        { id: 'c1', title: 'Лучшее фэнтези', slug: 'luchshee', kind: 'manual', is_published: true, source_genre: { id: fentezi.id, name: 'Фентези' } },
      ],
    });
    await waitFor(() => expect(confirm).toBeEnabled());
    expect(screen.getByText('Подборка «Лучшее фэнтези» будет переключена на жанр «Фэнтези».')).toBeInTheDocument();
  });

  it('applies with the suggested target without sending target_id', async () => {
    applyMock.mockResolvedValue(applyResult);
    const { onApplied } = renderDialog();

    const confirm = screen.getByRole('button', { name: 'Слить 2 в «Фэнтези»' });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);

    await waitFor(() => expect(onApplied).toHaveBeenCalledWith(applyResult, 'Фэнтези'));
    expect(applyMock).toHaveBeenCalledWith('s-merge', undefined);
  });

  it('sends target_id when another genre of the suggestion is chosen', async () => {
    applyMock.mockResolvedValue(applyResult);
    const { onApplied } = renderDialog();

    fireEvent.click(screen.getByRole('radio', { name: /Fantasy/ }));
    await waitFor(() => expect(previewMock).toHaveBeenCalledWith([fantasy.id, fentezi.id], fantasyEn.id));
    const confirm = screen.getByRole('button', { name: 'Слить 2 в «Fantasy»' });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);

    await waitFor(() => expect(onApplied).toHaveBeenCalledWith(applyResult, 'Fantasy'));
    expect(applyMock).toHaveBeenCalledWith('s-merge', fantasyEn.id);
  });

  it('recomputes the preview with every genre as a source for an outside target and sends its id', async () => {
    applyMock.mockResolvedValue(applyResult);
    const { onApplied } = renderDialog();

    fireEvent.click(screen.getByLabelText('Другой жанр…'));
    fireEvent.change(screen.getByLabelText('Поиск целевого жанра'), { target: { value: 'фант' } });
    fireEvent.click(await screen.findByRole('button', { name: 'Фантастика' }));

    await waitFor(() =>
      expect(previewMock).toHaveBeenCalledWith([fantasy.id, fentezi.id, fantasyEn.id], scifi.id)
    );
    // Genres of the suggestion are not offered as "another" genre.
    expect(screen.queryByRole('button', { name: 'Фэнтези' })).not.toBeInTheDocument();

    const confirm = screen.getByRole('button', { name: 'Слить 3 в «Фантастика»' });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);

    await waitFor(() => expect(onApplied).toHaveBeenCalledWith(applyResult, 'Фантастика'));
    expect(applyMock).toHaveBeenCalledWith('s-merge', scifi.id);
  });

  it('warns about a rejected target', async () => {
    renderDialog(
      mergeSuggestion({
        genres: [suggestionGenre({ id: 'g-rej', name: 'Мусор', status: 'rejected' }), fentezi],
        target: { id: 'g-rej', name: 'Мусор' },
      })
    );
    expect(screen.getByText('Целевой жанр отклонён — книги окажутся в скрытом жанре.')).toBeInTheDocument();
  });

  it.each([
    ['409 stale', new ApiError(409, 'Conflict', 'suggestion is stale'), 'Предложение устарело: жанры изменились после анализа.'],
    ['404 target', new ApiError(404, 'Not Found', 'genre not found'), 'Жанр не найден — возможно, его уже слили. Обновите список и попробуйте снова.'],
    ['500', new ApiError(500, 'Server Error', 'boom'), 'Не удалось применить предложение. Попробуйте позже.'],
  ])('shows the error text for %s and does not report success', async (_name, error, text) => {
    applyMock.mockRejectedValue(error);
    const { onApplied } = renderDialog();

    const confirm = screen.getByRole('button', { name: 'Слить 2 в «Фэнтези»' });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);

    expect(await screen.findByText(text)).toBeInTheDocument();
    expect(onApplied).not.toHaveBeenCalled();
  });

  it('closes on cancel', () => {
    const { onClose } = renderDialog();
    fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));
    expect(onClose).toHaveBeenCalled();
  });
});
