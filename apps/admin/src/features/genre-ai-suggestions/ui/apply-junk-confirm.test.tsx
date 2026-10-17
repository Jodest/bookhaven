import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { ApplyJunkConfirm } from './apply-junk-confirm';
import { junkSuggestion } from '../test-fixtures';

const applyMock = vi.fn();

vi.mock('@/entities/genre-ai-suggestion/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre-ai-suggestion/api')>()),
  adminGenreAiSuggestionsApi: {
    apply: (...args: unknown[]) => applyMock(...args),
  },
}));

function renderConfirm() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const onClose = vi.fn();
  const onApplied = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <ApplyJunkConfirm suggestion={junkSuggestion()} onClose={onClose} onApplied={onApplied} />
    </QueryClientProvider>
  );
  return { onClose, onApplied };
}

describe('ApplyJunkConfirm', () => {
  beforeEach(() => {
    applyMock.mockReset();
  });

  it('asks before rejecting and keeps the book links promise in the text', () => {
    renderConfirm();
    expect(
      screen.getByText('Отклонить жанр «Книги 2020»? Связи с книгами (7) сохранятся, жанр исчезнет со страниц книг.')
    ).toBeInTheDocument();
    expect(applyMock).not.toHaveBeenCalled();
  });

  it('applies without a target and reports the genre name', async () => {
    const result = { suggestion: junkSuggestion({ status: 'applied' }), merge: null, status_report: { processed: 1, updated: 1, errors: [] } };
    applyMock.mockResolvedValue(result);
    const { onApplied } = renderConfirm();

    fireEvent.click(screen.getByRole('button', { name: 'Отклонить жанр' }));

    await waitFor(() => expect(onApplied).toHaveBeenCalledWith(result, 'Книги 2020'));
    expect(applyMock).toHaveBeenCalledWith('s-junk', undefined);
  });

  it('shows the stale text on 409', async () => {
    applyMock.mockRejectedValue(new ApiError(409, 'Conflict', 'suggestion is stale'));
    const { onApplied } = renderConfirm();
    fireEvent.click(screen.getByRole('button', { name: 'Отклонить жанр' }));
    expect(await screen.findByText('Предложение устарело: жанры изменились после анализа.')).toBeInTheDocument();
    expect(onApplied).not.toHaveBeenCalled();
  });

  it('closes on cancel', () => {
    const { onClose } = renderConfirm();
    fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));
    expect(onClose).toHaveBeenCalled();
  });
});
