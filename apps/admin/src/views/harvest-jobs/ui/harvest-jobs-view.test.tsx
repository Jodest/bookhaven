import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { HarvestJobsView } from './harvest-jobs-view';

const listJobsMock = vi.fn();
const triggerSyncMock = vi.fn();

vi.mock('@bookhaven/api-client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@bookhaven/api-client')>()),
  api: {
    admin: {
      harvest: {
        trigger: vi.fn(),
        jobs: { list: (...args: unknown[]) => listJobsMock(...args), get: vi.fn() },
        authors: { search: vi.fn() },
      },
      books: { triggerCatalogAdaptationSync: (...args: unknown[]) => triggerSyncMock(...args) },
    },
  },
}));

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <HarvestJobsView />
    </QueryClientProvider>
  );
}

describe('HarvestJobsView', () => {
  beforeEach(() => {
    listJobsMock.mockReset();
    triggerSyncMock.mockReset();
    listJobsMock.mockResolvedValue({ items: [] });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('renders the page title and header links to the moderation queues', async () => {
    renderView();

    expect(screen.getByRole('heading', { name: 'Задачи харвеста' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'К очереди на проверку' })).toHaveAttribute(
      'href',
      '/moderation/books'
    );
    expect(screen.getByRole('link', { name: 'К авторам на модерации' })).toHaveAttribute(
      'href',
      '/moderation/authors'
    );
    await waitFor(() => expect(listJobsMock).toHaveBeenCalled());
  });

  it('shows the "Нет задач" empty state when there are no jobs', async () => {
    renderView();

    await waitFor(() => expect(listJobsMock).toHaveBeenCalled());
    expect(await screen.findByText('Нет задач')).toBeInTheDocument();
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
    // Header + the completed job: both link to the moderation queues.
    const bookLinks = screen.getAllByRole('link', { name: 'К очереди на проверку' });
    expect(bookLinks).toHaveLength(2);
    bookLinks.forEach((link) => expect(link).toHaveAttribute('href', '/moderation/books'));
    const authorLinks = screen.getAllByRole('link', { name: 'К авторам на модерации' });
    expect(authorLinks).toHaveLength(2);
    authorLinks.forEach((link) => expect(link).toHaveAttribute('href', '/moderation/authors'));
  });

  it('shows the «Запуск» column: «Авто» with the author count for scheduled jobs, «Вручную» otherwise', async () => {
    listJobsMock.mockResolvedValue({
      items: [
        {
          id: 'job-scheduled',
          queue: 'harvest',
          state: 'pending',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
          trigger: 'scheduled',
          author_qids: ['Q1', 'Q2', 'Q3', 'Q4', 'Q5', 'Q6', 'Q7', 'Q8', 'Q9', 'Q10'],
        },
        {
          id: 'job-manual',
          queue: 'harvest',
          state: 'pending',
          type: 'harvest:wikidata_query',
          max_retry: 3,
          retried: 0,
          trigger: 'manual',
          author_qids: [],
        },
      ],
    });

    renderView();

    expect(await screen.findByRole('columnheader', { name: 'Запуск' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText('Авто')).toBeInTheDocument());
    expect(screen.getByText('Авторов: 10')).toBeInTheDocument();
    expect(screen.getByText('Вручную')).toBeInTheDocument();
    expect(screen.getAllByText(/^Авторов:/)).toHaveLength(1);
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

  describe('catalog adaptation sync', () => {
    const syncJob = (over: Record<string, unknown> = {}) => ({
      id: 'sync-job-123456',
      queue: 'harvest',
      state: 'completed',
      type: 'screenworks:sync_catalog',
      max_retry: 3,
      retried: 0,
      kind: 'adaptation_sync',
      trigger: 'manual',
      author_qids: [],
      result: null,
      adaptation_sync: {
        total: 683,
        processed: 683,
        updated: 40,
        added: 75,
        unchanged: 640,
        skipped: 3,
        failed: 0,
        errors: [],
      },
      ...over,
    });

    it('starts the sync after confirming', async () => {
      triggerSyncMock.mockResolvedValue({ task_id: 't', queue: 'harvest', state: 'pending' });
      renderView();

      fireEvent.click(screen.getByRole('button', { name: 'Найти экранизации для всего каталога' }));
      expect(
        screen.getByText(
          'Найти экранизации в Wikidata для всех опубликованных книг с QID? Задача выполнится в фоне.'
        )
      ).toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: 'Запустить' }));

      expect(
        await screen.findByText('Поиск экранизаций по каталогу поставлен в очередь.')
      ).toBeInTheDocument();
      expect(triggerSyncMock).toHaveBeenCalledTimes(1);
    });

    it('shows the running banner on 409', async () => {
      triggerSyncMock.mockRejectedValue(new ApiError(409, 'Conflict'));
      renderView();

      fireEvent.click(screen.getByRole('button', { name: 'Найти экранизации для всего каталога' }));
      fireEvent.click(screen.getByRole('button', { name: 'Запустить' }));

      expect(
        await screen.findByText('Поиск экранизаций по каталогу уже выполняется.')
      ).toBeInTheDocument();
    });

    it('shows the failure banner on any other error', async () => {
      triggerSyncMock.mockRejectedValue(new ApiError(502, 'Bad Gateway'));
      renderView();

      fireEvent.click(screen.getByRole('button', { name: 'Найти экранизации для всего каталога' }));
      fireEvent.click(screen.getByRole('button', { name: 'Запустить' }));

      expect(await screen.findByText('Не удалось запустить поиск экранизаций.')).toBeInTheDocument();
    });

    it('sends nothing when the confirmation is cancelled', async () => {
      renderView();

      fireEvent.click(screen.getByRole('button', { name: 'Найти экранизации для всего каталога' }));
      fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));

      expect(triggerSyncMock).not.toHaveBeenCalled();
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      await waitFor(() => expect(listJobsMock).toHaveBeenCalled());
    });

    it('disables the button while a catalog sync job is active', async () => {
      listJobsMock.mockResolvedValue({
        items: [
          syncJob({
            state: 'active',
            adaptation_sync: { total: 683, processed: 100, updated: 1, added: 2, unchanged: 97, skipped: 0, failed: 0, errors: [] },
          }),
        ],
      });
      renderView();

      await waitFor(() => expect(screen.getByText('100 / 683 (15%)')).toBeInTheDocument());
      expect(screen.getByRole('button', { name: 'Найти экранизации для всего каталога' })).toBeDisabled();
    });

    it('renders a completed catalog sync row', async () => {
      listJobsMock.mockResolvedValue({ items: [syncJob()] });
      renderView();

      expect(
        await screen.findByText(
          'Книг: 683 из 683, обновлено: 40, добавлено экранизаций: 75, ошибок: 0'
        )
      ).toBeInTheDocument();
      expect(screen.getByText('Вручную')).toBeInTheDocument();
      expect(screen.getByText('Поиск экранизаций')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Подробнее' })).not.toBeInTheDocument();
    });

    it('lists errors and the count beyond the list', async () => {
      const errors = Array.from({ length: 50 }, (_, i) => ({
        book_id: `b${i}`,
        title: `Книга ${i}`,
        reason: i === 0 ? 'save_failed' : 'wikidata_unavailable',
      }));
      listJobsMock.mockResolvedValue({
        items: [
          syncJob({
            adaptation_sync: { total: 683, processed: 683, updated: 0, added: 0, unchanged: 563, skipped: 0, failed: 120, errors },
          }),
        ],
      });
      renderView();

      fireEvent.click(await screen.findByRole('button', { name: 'Подробнее' }));

      expect(screen.getByText('Книга 0')).toBeInTheDocument();
      expect(screen.getByText('Не удалось сохранить экранизации')).toBeInTheDocument();
      expect(screen.getAllByText('Wikidata недоступна')).toHaveLength(49);
      expect(screen.getByText('Ещё ошибок: 70')).toBeInTheDocument();
    });

    it('leaves harvest rows unchanged', async () => {
      listJobsMock.mockResolvedValue({
        items: [
          {
            id: 'harvest-job-1234',
            queue: 'harvest',
            state: 'completed',
            type: 'harvest:wikidata_query',
            max_retry: 3,
            retried: 0,
            kind: 'harvest',
            trigger: 'scheduled',
            author_qids: ['Q1', 'Q2'],
            adaptation_sync: null,
            result: { processed: 5, total: 5, created: 2, skipped_existing: 3, errors: [], skipped_records: [] },
          },
        ],
      });
      renderView();

      expect(await screen.findByText('создано 2, пропущено 3, ошибок 0')).toBeInTheDocument();
      expect(screen.getByText('Авторов: 2')).toBeInTheDocument();
      expect(screen.queryByText('Поиск экранизаций')).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Найти экранизации для всего каталога' })).toBeEnabled();
    });
  });
});
