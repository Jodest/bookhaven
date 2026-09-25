import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { HarvestView } from './harvest-view';

const triggerMock = vi.fn();
const listJobsMock = vi.fn();
const searchAuthorsMock = vi.fn();

vi.mock('@/entities/harvest', () => ({
  adminHarvestApi: {
    trigger: (...args: unknown[]) => triggerMock(...args),
    listJobs: (...args: unknown[]) => listJobsMock(...args),
    getJob: vi.fn(),
    searchAuthors: (...args: unknown[]) => searchAuthorsMock(...args),
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

function switchToManualMode() {
  fireEvent.click(screen.getByRole('button', { name: 'Ввести QID вручную' }));
}

describe('HarvestView', () => {
  beforeEach(() => {
    triggerMock.mockReset();
    listJobsMock.mockReset();
    searchAuthorsMock.mockReset();
    listJobsMock.mockResolvedValue({ items: [] });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('defaults to name-search mode, with a toggle to manual QID entry', async () => {
    renderView();

    expect(screen.getByLabelText('Поиск автора по имени')).toBeInTheDocument();
    expect(screen.queryByLabelText('QID авторов Wikidata')).not.toBeInTheDocument();

    switchToManualMode();

    expect(screen.getByLabelText('QID авторов Wikidata')).toBeInTheDocument();
    expect(screen.queryByLabelText('Поиск автора по имени')).not.toBeInTheDocument();
  });

  it('requires at least one selected author before submitting in name-search mode', async () => {
    renderView();

    fireEvent.click(screen.getByRole('button', { name: 'Запустить харвест' }));

    await waitFor(() =>
      expect(screen.getByText('Выберите хотя бы одного автора.')).toBeInTheDocument()
    );
    expect(triggerMock).not.toHaveBeenCalled();
  });

  it('requires at least one author QID before submitting in manual mode', async () => {
    renderView();
    switchToManualMode();

    fireEvent.click(screen.getByRole('button', { name: 'Запустить харвест' }));

    await waitFor(() =>
      expect(screen.getByText('Введите хотя бы один QID автора.')).toBeInTheDocument()
    );
    expect(triggerMock).not.toHaveBeenCalled();
  });

  it('parses comma/newline-separated QIDs into an author_qids array (manual mode)', async () => {
    triggerMock.mockResolvedValue({ task_id: 'task-1', queue: 'harvest', state: 'pending' });
    renderView();
    switchToManualMode();

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

  it('refetches the jobs list after a successful trigger (manual mode)', async () => {
    triggerMock.mockResolvedValue({ task_id: 'task-1', queue: 'harvest', state: 'pending' });
    renderView();
    switchToManualMode();

    await waitFor(() => expect(listJobsMock).toHaveBeenCalledTimes(1));

    fireEvent.change(screen.getByLabelText('QID авторов Wikidata'), {
      target: { value: 'Q7200' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Запустить харвест' }));

    await waitFor(() => expect(triggerMock).toHaveBeenCalled());
    await waitFor(() => expect(listJobsMock.mock.calls.length).toBeGreaterThan(1));
  });

  it('surfaces the message from a 400 ApiError on trigger (manual mode)', async () => {
    triggerMock.mockRejectedValue(new ApiError(400, 'Bad Request', 'invalid author QID "bad"'));
    renderView();
    switchToManualMode();

    fireEvent.change(screen.getByLabelText('QID авторов Wikidata'), {
      target: { value: 'bad' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Запустить харвест' }));

    await waitFor(() =>
      expect(screen.getByText('invalid author QID "bad"')).toBeInTheDocument()
    );
  });

  it('searches by name, renders candidates, adds one to the selected list, and submits its qid', async () => {
    searchAuthorsMock.mockResolvedValue({
      items: [
        { qid: 'Q7200', label: 'Лев Толстой', description: 'русский писатель' },
        { qid: 'Q171976', label: 'Алексей Николаевич Толстой', description: 'русский писатель' },
      ],
    });
    triggerMock.mockResolvedValue({ task_id: 'task-1', queue: 'harvest', state: 'pending' });
    renderView();

    fireEvent.change(screen.getByLabelText('Поиск автора по имени'), {
      target: { value: 'Толстой' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));

    await waitFor(() => expect(searchAuthorsMock).toHaveBeenCalled());
    expect(searchAuthorsMock.mock.calls[0][0]).toBe('Толстой');
    await waitFor(() => expect(screen.getByText('Лев Толстой')).toBeInTheDocument());
    expect(screen.getByText('Алексей Николаевич Толстой')).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole('button', { name: 'Добавить' })[0]);

    await waitFor(() => expect(screen.getByText('Выбранные авторы')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: 'Запустить харвест' }));

    await waitFor(() => expect(triggerMock).toHaveBeenCalled());
    expect(triggerMock.mock.calls[0][0]).toEqual({ author_qids: ['Q7200'] });
  });

  it('shows the empty-state message pointing at manual QID entry when a name search has zero results', async () => {
    searchAuthorsMock.mockResolvedValue({ items: [] });
    renderView();

    fireEvent.change(screen.getByLabelText('Поиск автора по имени'), {
      target: { value: 'Несуществующий Автор' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));

    await waitFor(() =>
      expect(
        screen.getByText('Совпадений не найдено. Проверьте написание или используйте ручной ввод QID.')
      ).toBeInTheDocument()
    );
  });

  it('surfaces the message from a 502 ApiError on name search', async () => {
    searchAuthorsMock.mockRejectedValue(
      new ApiError(502, 'Bad Gateway', 'search source is unavailable; retry later or enter the QID manually')
    );
    renderView();

    fireEvent.change(screen.getByLabelText('Поиск автора по имени'), {
      target: { value: 'Толстой' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));

    await waitFor(() =>
      expect(
        screen.getByText('search source is unavailable; retry later or enter the QID manually')
      ).toBeInTheDocument()
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

  it('renders a progress bar with N / M (P%) text for an active job with a partial result', async () => {
    listJobsMock.mockResolvedValue({
      items: [
        {
          id: 'job-active',
          queue: 'harvest',
          state: 'active',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
          result: { processed: 37, total: 200, created: 0, skipped_existing: 0, errors: [] },
        },
      ],
    });

    renderView();

    await waitFor(() => expect(screen.getByText('37 / 200 (19%)')).toBeInTheDocument());
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '19');
  });

  it('renders "Выполняется…" instead of a bar for an active job with no result yet or total === 0', async () => {
    listJobsMock.mockResolvedValue({
      items: [
        {
          id: 'job-active-no-result',
          queue: 'harvest',
          state: 'active',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
        },
        {
          id: 'job-active-zero-total',
          queue: 'harvest',
          state: 'active',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
          result: { processed: 0, total: 0, created: 0, skipped_existing: 0, errors: [] },
        },
      ],
    });

    renderView();

    await waitFor(() => expect(screen.getAllByText('Выполняется…').length).toBe(2));
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
  });

  it('does not render a stale progress bar for a retry job, keeping the last_error/— summary', async () => {
    listJobsMock.mockResolvedValue({
      items: [
        {
          id: 'job-retry-stale',
          queue: 'harvest',
          state: 'retry',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 1,
          last_error: 'wikidata: source unavailable',
          result: { processed: 12, total: 50, created: 0, skipped_existing: 0, errors: [] },
        },
      ],
    });

    renderView();

    await waitFor(() =>
      expect(screen.getByText('wikidata: source unavailable')).toBeInTheDocument()
    );
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    expect(screen.queryByText('12 / 50 (24%)')).not.toBeInTheDocument();
  });
});
