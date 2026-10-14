import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { reviewErrorText } from '../lib/review-text';
import { ReviewActions } from './review-actions';

const approveMock = vi.fn();
const discardMock = vi.fn();

vi.mock('@/entities/ai-description', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/ai-description')>()),
  adminAiDescriptionsApi: {
    approve: (...args: unknown[]) => approveMock(...args),
    discard: (...args: unknown[]) => discardMock(...args),
  },
}));

function renderActions(status: 'draft' | 'approved' | 'unknown', notify = vi.fn()) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <ReviewActions bookId="b1" status={status} text="Повесть о враче." notify={notify} />
    </QueryClientProvider>
  );
  return notify;
}

describe('reviewErrorText', () => {
  it('maps the 409 own-description conflict', () => {
    expect(reviewErrorText(new ApiError(409, 'x', 'book already has a description'))).toBe(
      'У книги уже есть собственное описание.'
    );
  });
  it('maps another 409, 404 and 422', () => {
    expect(reviewErrorText(new ApiError(409, 'x', 'AI description is not reviewable'))).toMatch(/не черновик/);
    expect(reviewErrorText(new ApiError(404, 'x'))).toBe('Описание ИИ не найдено.');
    expect(reviewErrorText(new ApiError(422, 'x', 'text must be'))).toMatch(/от 1 до 2000 символов/);
  });
});

describe('ReviewActions', () => {
  beforeEach(() => {
    approveMock.mockReset();
    discardMock.mockReset();
  });

  it('approves a draft as is', async () => {
    approveMock.mockResolvedValue({});
    const notify = renderActions('draft');
    fireEvent.click(screen.getByRole('button', { name: 'Одобрить' }));
    await waitFor(() => expect(notify).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'success' })));
    expect(approveMock).toHaveBeenCalledWith('b1', undefined);
  });

  it('edits and approves with the trimmed text; 422 shows the rule', async () => {
    approveMock.mockRejectedValue(new ApiError(422, 'x', 'text must be'));
    const notify = renderActions('draft');
    fireEvent.click(screen.getByRole('button', { name: 'Изменить и одобрить' }));
    const field = screen.getByLabelText('Текст описания');
    expect(field).toHaveValue('Повесть о враче.');
    fireEvent.change(field, { target: { value: '  Исправленный текст.  ' } });
    expect(screen.getByText('23 / 2000')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Одобрить с изменениями' }));
    await waitFor(() =>
      expect(notify).toHaveBeenLastCalledWith({
        type: 'error',
        message: 'Текст должен быть от 1 до 2000 символов и написан в основном кириллицей.',
      })
    );
    expect(approveMock).toHaveBeenCalledWith('b1', 'Исправленный текст.');
  });

  it('maps the own-description 409 on approve', async () => {
    approveMock.mockRejectedValue(new ApiError(409, 'x', 'book already has a description'));
    const notify = renderActions('draft');
    fireEvent.click(screen.getByRole('button', { name: 'Одобрить' }));
    await waitFor(() =>
      expect(notify).toHaveBeenLastCalledWith({ type: 'error', message: 'У книги уже есть собственное описание.' })
    );
  });

  it('discards; an approved record offers no plain approve', async () => {
    discardMock.mockResolvedValue({});
    renderActions('approved');
    expect(screen.queryByRole('button', { name: 'Одобрить' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Отклонить' }));
    await waitFor(() => expect(discardMock).toHaveBeenCalledWith('b1'));
  });

  it('renders nothing for a non-reviewable status', () => {
    renderActions('unknown');
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});
