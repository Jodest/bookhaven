import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { HarvestView } from './harvest-view';

const triggerMock = vi.fn();
const listJobsMock = vi.fn();

vi.mock('@/entities/harvest', () => ({
  adminHarvestApi: {
    trigger: (...args: unknown[]) => triggerMock(...args),
    listJobs: (...args: unknown[]) => listJobsMock(...args),
    getJob: vi.fn(),
  },
}));

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <HarvestView />
    </QueryClientProvider>
  );
}

describe('HarvestView', () => {
  beforeEach(() => {
    triggerMock.mockReset();
    listJobsMock.mockReset();
    listJobsMock.mockResolvedValue({ items: [] });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('requires at least one author QID before submitting', async () => {
    renderView();

    fireEvent.click(screen.getByRole('button', { name: 'Запустить харвест' }));

    await waitFor(() =>
      expect(screen.getByText('Введите хотя бы один QID автора.')).toBeInTheDocument()
    );
    expect(triggerMock).not.toHaveBeenCalled();
  });

  it('parses comma/newline-separated QIDs into an author_qids array', async () => {
    triggerMock.mockResolvedValue({ task_id: 'task-1', queue: 'harvest', state: 'pending' });
    renderView();

    fireEvent.change(screen.getByLabelText('QID авторов Wikidata'), {
      target: { value: 'Q7200, Q1183296\nQ7200\n' },
    });
    fireEvent.change(screen.getByLabelText('Лимит (необязательно)'), {
      target: { value: '250' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Запустить харвест' }));

    await waitFor(() => expect(triggerMock).toHaveBeenCalled());
    expect(triggerMock.mock.calls[0][0]).toEqual({
      author_qids: ['Q7200', 'Q1183296'],
      limit: 250,
    });
  });

  it('refetches the jobs list after a successful trigger', async () => {
    triggerMock.mockResolvedValue({ task_id: 'task-1', queue: 'harvest', state: 'pending' });
    renderView();

    await waitFor(() => expect(listJobsMock).toHaveBeenCalledTimes(1));

    fireEvent.change(screen.getByLabelText('QID авторов Wikidata'), {
      target: { value: 'Q7200' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Запустить харвест' }));

    await waitFor(() => expect(triggerMock).toHaveBeenCalled());
    await waitFor(() => expect(listJobsMock.mock.calls.length).toBeGreaterThan(1));
  });

  it('surfaces the message from a 400 ApiError', async () => {
    triggerMock.mockRejectedValue(new ApiError(400, 'Bad Request', 'invalid author QID "bad"'));
    renderView();

    fireEvent.change(screen.getByLabelText('QID авторов Wikidata'), {
      target: { value: 'bad' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Запустить харвест' }));

    await waitFor(() =>
      expect(screen.getByText('invalid author QID "bad"')).toBeInTheDocument()
    );
  });

  it('renders state badges and a pending-review link for a completed job with created books', async () => {
    listJobsMock.mockResolvedValue({
      items: [
        {
          id: 'job-1234567890',
          queue: 'harvest',
          state: 'completed',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
          result: { processed: 5, created: 2, skipped_existing: 3, errors: [] },
        },
        {
          id: 'job-2',
          queue: 'harvest',
          state: 'active',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
        },
      ],
    });

    renderView();

    await waitFor(() => expect(screen.getByText('completed')).toBeInTheDocument());
    expect(screen.getByText('active')).toBeInTheDocument();
    expect(screen.getByText('создано 2, пропущено 3, ошибок 0')).toBeInTheDocument();
    expect(
      screen.getAllByRole('link', { name: 'К очереди на проверку' }).length
    ).toBeGreaterThan(0);
  });
});
