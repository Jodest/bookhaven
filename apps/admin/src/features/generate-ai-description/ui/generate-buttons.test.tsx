import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { generateErrorText, generateMissingConfirmText } from '../lib/generate-text';
import { GenerateButton, GenerateMissingButton } from './generate-buttons';

const generateMock = vi.fn();

vi.mock('@/entities/ai-description', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/ai-description')>()),
  adminAiDescriptionsApi: {
    generate: (...args: unknown[]) => generateMock(...args),
  },
}));

function renderWithClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

describe('generateErrorText', () => {
  it.each([
    [409, 'Генерация уже в очереди.'],
    [503, 'Генерация описаний ИИ выключена.'],
    [502, 'Не удалось поставить генерацию в очередь. Попробуйте ещё раз.'],
  ])('%i -> %s', (status, text) => {
    expect(generateErrorText(new ApiError(status, 'x', 'server text'))).toBe(text);
  });

  it('confirm text of the bulk run', () => {
    expect(generateMissingConfirmText(134, 200)).toBe(
      'Сгенерировать описания ИИ для книг без описания (до 200 из 134)?'
    );
  });
});

describe('GenerateButton', () => {
  const confirmSpy = vi.spyOn(window, 'confirm');

  beforeEach(() => {
    generateMock.mockReset();
    confirmSpy.mockReset();
  });
  afterEach(() => confirmSpy.mockReset());

  it('queues one book and reports it', async () => {
    generateMock.mockResolvedValue({ task_id: 't1' });
    const notify = vi.fn();
    const onQueued = vi.fn();
    renderWithClient(<GenerateButton bookId="b1" currentStatus={null} notify={notify} onQueued={onQueued} />);
    fireEvent.click(screen.getByRole('button', { name: 'Сгенерировать' }));
    await waitFor(() => expect(onQueued).toHaveBeenCalled());
    expect(generateMock).toHaveBeenCalledWith({ book_ids: ['b1'] });
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(notify).toHaveBeenLastCalledWith({ type: 'success', message: 'Генерация поставлена в очередь.' });
  });

  it('asks before regenerating an approved description; cancel queues nothing', () => {
    confirmSpy.mockReturnValue(false);
    renderWithClient(<GenerateButton bookId="b1" currentStatus="approved" notify={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'Сгенерировать заново' }));
    expect(confirmSpy).toHaveBeenCalledWith('Одобренное описание будет снято с сайта до проверки нового. Продолжить?');
    expect(generateMock).not.toHaveBeenCalled();
  });

  it('maps 503 to the disabled message', async () => {
    generateMock.mockRejectedValue(new ApiError(503, 'x', 'AI descriptions are disabled'));
    const notify = vi.fn();
    renderWithClient(<GenerateButton bookId="b1" currentStatus="draft" notify={notify} />);
    fireEvent.click(screen.getByRole('button', { name: 'Сгенерировать заново' }));
    await waitFor(() =>
      expect(notify).toHaveBeenLastCalledWith({ type: 'error', message: 'Генерация описаний ИИ выключена.' })
    );
  });
});

describe('GenerateMissingButton', () => {
  const confirmSpy = vi.spyOn(window, 'confirm');

  beforeEach(() => {
    generateMock.mockReset();
    confirmSpy.mockReset();
  });

  it('confirms, queues all_missing and maps 409', async () => {
    confirmSpy.mockReturnValue(true);
    generateMock.mockRejectedValue(new ApiError(409, 'x', 'AI description generation already queued'));
    const notify = vi.fn();
    renderWithClient(<GenerateMissingButton missing={5} maxBulk={200} notify={notify} />);
    fireEvent.click(screen.getByRole('button', { name: 'Сгенерировать для книг без описания' }));
    expect(confirmSpy).toHaveBeenCalledWith('Сгенерировать описания ИИ для книг без описания (до 200 из 5)?');
    await waitFor(() =>
      expect(notify).toHaveBeenLastCalledWith({ type: 'error', message: 'Генерация уже в очереди.' })
    );
    expect(generateMock).toHaveBeenCalledWith({ all_missing: true });
  });

  it('is disabled when no book is missing a description', () => {
    renderWithClient(<GenerateMissingButton missing={0} maxBulk={200} notify={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Сгенерировать для книг без описания' })).toBeDisabled();
  });
});
