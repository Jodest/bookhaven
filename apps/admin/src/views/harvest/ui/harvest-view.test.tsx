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

  it('marks candidates already in BookHaven with a status badge and an open link', async () => {
    searchAuthorsMock.mockResolvedValue({
      items: [
        {
          qid: 'Q7200',
          label: 'Лев Толстой',
          description: 'русский писатель',
          existing: { id: 'a1', name: 'Лев Толстой', slug: 'lev-tolstoy', status: 'pending' },
        },
        { qid: 'Q171976', label: 'Алексей Николаевич Толстой', existing: null },
      ],
    });
    renderView();

    fireEvent.change(screen.getByLabelText('Поиск автора по имени'), {
      target: { value: 'Толстой' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));

    await waitFor(() => expect(screen.getByText('На модерации')).toBeInTheDocument());
    expect(screen.getAllByRole('link', { name: 'Открыть' })).toHaveLength(1);
    // Harvest keeps its own action: no import buttons here.
    expect(screen.getAllByRole('button', { name: 'Добавить' })).toHaveLength(2);
    expect(screen.queryByRole('button', { name: 'Импортировать автора' })).not.toBeInTheDocument();
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
          result: { processed: 5, created: 2, skipped_existing: 3, errors: [], skipped_records: [] },
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
    // Header + the completed job: both link to the author moderation queue.
    const authorLinks = screen.getAllByRole('link', { name: 'К авторам на модерации' });
    expect(authorLinks).toHaveLength(2);
    authorLinks.forEach((link) => expect(link).toHaveAttribute('href', '/authors/moderation'));
  });

  it('shows the author enrichment counters of a completed job when present', async () => {
    listJobsMock.mockResolvedValue({
      items: [
        {
          id: 'job-enriched',
          queue: 'harvest',
          state: 'completed',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
          result: {
            processed: 4,
            total: 4,
            created: 4,
            skipped_existing: 0,
            errors: [],
            skipped_records: [],
            authors_enriched: 3,
            author_enrichment_errors: 1,
          },
        },
        {
          id: 'job-old',
          queue: 'harvest',
          state: 'completed',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
          result: { processed: 1, total: 1, created: 0, skipped_existing: 1, errors: [], skipped_records: [] },
        },
        {
          id: 'job-zero',
          queue: 'harvest',
          state: 'completed',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
          result: {
            processed: 1,
            total: 1,
            created: 1,
            skipped_existing: 0,
            errors: [],
            skipped_records: [],
            authors_enriched: 0,
            author_enrichment_errors: 0,
          },
        },
      ],
    });

    renderView();

    await waitFor(() =>
      expect(screen.getByText('Авторов обогащено из Wikidata: 3, ошибок обогащения: 1')).toBeInTheDocument()
    );
    expect(screen.getByText('Авторов обогащено из Wikidata: 0')).toBeInTheDocument();
    expect(screen.getAllByText(/Авторов обогащено из Wikidata/)).toHaveLength(2);
    expect(screen.getByText('создано 4, пропущено 0, ошибок 0')).toBeInTheDocument();
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
          result: { processed: 37, total: 200, created: 0, skipped_existing: 0, errors: [], skipped_records: [] },
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
          result: { processed: 0, total: 0, created: 0, skipped_existing: 0, errors: [], skipped_records: [] },
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
          result: { processed: 12, total: 50, created: 0, skipped_existing: 0, errors: [], skipped_records: [] },
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

  it('renders no "Подробнее" button for a job with empty errors and skipped_records', async () => {
    listJobsMock.mockResolvedValue({
      items: [
        {
          id: 'job-clean',
          queue: 'harvest',
          state: 'completed',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
          result: { processed: 5, total: 5, created: 5, skipped_existing: 0, errors: [], skipped_records: [] },
        },
      ],
    });

    renderView();

    await waitFor(() => expect(screen.getByText('completed')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: 'Подробнее' })).not.toBeInTheDocument();
  });

  it('expands a completed job to reveal errors/skipped tables, and collapses again', async () => {
    listJobsMock.mockResolvedValue({
      items: [
        {
          id: 'job-report',
          queue: 'harvest',
          state: 'completed',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
          result: {
            processed: 5,
            total: 5,
            created: 2,
            skipped_existing: 2,
            errors: [{ index: 4, title: 'Плохая книга', message: 'invalid ISBN' }],
            skipped_records: [
              { index: 1, title: 'Уже в каталоге 1' },
              { index: 3, title: 'Уже в каталоге 2' },
            ],
          },
        },
      ],
    });

    renderView();

    await waitFor(() => expect(screen.getByText('completed')).toBeInTheDocument());

    const detailsButton = screen.getByRole('button', { name: 'Подробнее' });
    expect(screen.queryByText('Ошибки')).not.toBeInTheDocument();
    expect(screen.queryByText('Пропущено')).not.toBeInTheDocument();

    fireEvent.click(detailsButton);

    expect(screen.getByText('Ошибки')).toBeInTheDocument();
    expect(screen.getByText('Плохая книга')).toBeInTheDocument();
    expect(screen.getByText('invalid ISBN')).toBeInTheDocument();

    expect(screen.getByText('Пропущено')).toBeInTheDocument();
    expect(screen.getByText('Уже в каталоге 1')).toBeInTheDocument();
    expect(screen.getByText('Уже в каталоге 2')).toBeInTheDocument();
    expect(
      screen.getAllByText('уже есть в каталоге (совпадение по внешнему источнику)').length
    ).toBe(2);

    expect(screen.getByRole('button', { name: 'Скрыть' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Скрыть' }));

    expect(screen.queryByText('Ошибки')).not.toBeInTheDocument();
    expect(screen.queryByText('Пропущено')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Подробнее' })).toBeInTheDocument();
  });

  it('renders a working "Подробнее" toggle for an active job with a partial result carrying errors/skips', async () => {
    listJobsMock.mockResolvedValue({
      items: [
        {
          id: 'job-active-report',
          queue: 'harvest',
          state: 'active',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
          result: {
            processed: 10,
            total: 100,
            created: 8,
            skipped_existing: 1,
            errors: [{ index: 2, title: 'Ошибочная запись', message: 'resolve authors: boom' }],
            skipped_records: [{ index: 5, title: 'Дубликат' }],
          },
        },
      ],
    });

    renderView();

    await waitFor(() => expect(screen.getByText('10 / 100 (10%)')).toBeInTheDocument());

    const detailsButton = screen.getByRole('button', { name: 'Подробнее' });
    fireEvent.click(detailsButton);

    expect(screen.getByText('Ошибочная запись')).toBeInTheDocument();
    expect(screen.getByText('resolve authors: boom')).toBeInTheDocument();
    expect(screen.getByText('Дубликат')).toBeInTheDocument();
  });
});
