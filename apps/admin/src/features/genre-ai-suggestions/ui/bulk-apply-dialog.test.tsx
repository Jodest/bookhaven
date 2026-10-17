import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { BulkApplyDialog } from './bulk-apply-dialog';
import { fantasy, fentezi, junkSuggestion, mergeSuggestion, suggestionGenre } from '../test-fixtures';

const previewMock = vi.fn();
const bulkApplyMock = vi.fn();

vi.mock('@/entities/genre/api', () => ({
  adminGenresApi: {
    list: vi.fn(),
    mergePreview: (...args: unknown[]) => previewMock(...args),
  },
}));

vi.mock('@/entities/genre-ai-suggestion/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre-ai-suggestion/api')>()),
  adminGenreAiSuggestionsApi: {
    bulkApply: (...args: unknown[]) => bulkApplyMock(...args),
  },
}));

const otherTarget = suggestionGenre({ id: 'g-scifi', name: 'Фантастика' });
const otherSource = suggestionGenre({ id: 'g-scifi-ru', name: 'Научная фантастика', status: 'pending' });
const second = mergeSuggestion({
  id: 's-second',
  kind: 'narrow',
  target: { id: otherTarget.id, name: otherTarget.name },
  genres: [otherTarget, otherSource],
});

function renderDialog(suggestions = [mergeSuggestion(), second, junkSuggestion()]) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const onClose = vi.fn();
  const onApplied = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <BulkApplyDialog suggestions={suggestions} onClose={onClose} onApplied={onApplied} />
    </QueryClientProvider>
  );
  return { onClose, onApplied };
}

describe('BulkApplyDialog', () => {
  beforeEach(() => {
    previewMock.mockReset();
    bulkApplyMock.mockReset();
    previewMock.mockResolvedValue({ books_reassigned: 3, collections: [] });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('lists every suggestion with its action and previews the merge ones for their suggested targets', async () => {
    renderDialog();

    expect(screen.getByRole('heading', { name: 'Применить выбранные предложения (3)' })).toBeInTheDocument();
    expect(screen.getByText('Слить в «Фэнтези»')).toBeInTheDocument();
    expect(screen.getByText('Слить в «Фантастика»')).toBeInTheDocument();
    expect(screen.getByText('Отклонить жанр')).toBeInTheDocument();
    await waitFor(() => expect(previewMock).toHaveBeenCalledTimes(2));
    expect(previewMock).toHaveBeenCalledWith(expect.arrayContaining([fentezi.id]), fantasy.id);
    expect(previewMock).toHaveBeenCalledWith([otherSource.id], otherTarget.id);
  });

  it('enables confirm only once every preview has loaded', async () => {
    let resolveFirst: (v: unknown) => void = () => {};
    previewMock.mockImplementation((_sources: string[], target: string) =>
      target === fantasy.id
        ? new Promise((resolve) => (resolveFirst = resolve))
        : Promise.resolve({ books_reassigned: 1, collections: [] })
    );
    renderDialog();

    const confirm = screen.getByRole('button', { name: 'Применить (3)' });
    await waitFor(() => expect(previewMock).toHaveBeenCalledTimes(2));
    expect(confirm).toBeDisabled();

    resolveFirst({ books_reassigned: 9, collections: [] });
    await waitFor(() => expect(confirm).toBeEnabled());
  });

  it('shows a failed preview on its item and still allows confirming', async () => {
    previewMock.mockImplementation((_sources: string[], target: string) =>
      target === fantasy.id
        ? Promise.reject(new ApiError(404, 'Not Found', 'genre not found'))
        : Promise.resolve({ books_reassigned: 1, collections: [] })
    );
    renderDialog();

    expect(
      await screen.findByText('Жанр не найден — возможно, его уже слили. Обновите список и попробуйте снова.')
    ).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Применить (3)' })).toBeEnabled());
  });

  it('is enabled at once for a junk-only selection and sends ids without targets', async () => {
    const report = { processed: 1, applied: 1, stale: 0, errors: [] };
    bulkApplyMock.mockResolvedValue(report);
    const { onApplied } = renderDialog([junkSuggestion()]);

    const confirm = screen.getByRole('button', { name: 'Применить (1)' });
    expect(confirm).toBeEnabled();
    expect(previewMock).not.toHaveBeenCalled();
    fireEvent.click(confirm);

    await waitFor(() => expect(onApplied).toHaveBeenCalledWith(report));
    expect(bulkApplyMock).toHaveBeenCalledWith([{ id: 's-junk' }]);
  });

  it('sends the selected ids in table order', async () => {
    bulkApplyMock.mockResolvedValue({ processed: 3, applied: 2, stale: 1, errors: [] });
    renderDialog();

    const confirm = screen.getByRole('button', { name: 'Применить (3)' });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);

    await waitFor(() =>
      expect(bulkApplyMock).toHaveBeenCalledWith([{ id: 's-merge' }, { id: 's-second' }, { id: 's-junk' }])
    );
  });

  it('shows a request failure', async () => {
    bulkApplyMock.mockRejectedValue(new ApiError(400, 'Bad Request', 'invalid request'));
    const { onApplied } = renderDialog([junkSuggestion()]);
    fireEvent.click(screen.getByRole('button', { name: 'Применить (1)' }));
    expect(await screen.findByText('Не удалось применить предложения. Попробуйте позже.')).toBeInTheDocument();
    expect(onApplied).not.toHaveBeenCalled();
  });
});
