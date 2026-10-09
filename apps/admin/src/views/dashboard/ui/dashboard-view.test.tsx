import { render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminHarvestJob } from '@bookhaven/api-client';
import { DashboardView } from './dashboard-view';

const dashboardMock = vi.fn();
const listJobsMock = vi.fn();

vi.mock('@bookhaven/api-client', () => ({
  api: {
    admin: {
      dashboard: (...args: unknown[]) => dashboardMock(...args),
      harvest: {
        trigger: vi.fn(),
        jobs: { list: (...args: unknown[]) => listJobsMock(...args), get: vi.fn() },
        authors: { search: vi.fn() },
      },
    },
  },
}));

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <DashboardView />
    </QueryClientProvider>
  );
}

function group(name: string) {
  return screen.getByRole('region', { name });
}

function job(id: string, state: string, extra: Partial<AdminHarvestJob> = {}): AdminHarvestJob {
  return {
    id,
    queue: 'harvest',
    state,
    type: 'harvest:wikidata_query',
    max_retry: 3,
    retried: 0,
    ...extra,
  };
}

const result = (overrides: Partial<NonNullable<AdminHarvestJob['result']>> = {}) => ({
  processed: 0,
  total: 0,
  created: 0,
  skipped_existing: 0,
  errors: [],
  skipped_records: [],
  ...overrides,
});

describe('DashboardView', () => {
  beforeEach(() => {
    dashboardMock.mockReset();
    listJobsMock.mockReset();
    dashboardMock.mockResolvedValue({
      books: 20,
      rejected_books: 3,
      authors: 11,
      genres: 5,
      pending_reviews: 2,
      pending_books: 4,
      pending_authors: 7,
      pending_genres: 6,
    });
    listJobsMock.mockResolvedValue({ items: [] });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders the Модерация / Каталог / Импорт groups in order', async () => {
    renderView();

    await waitFor(() => expect(screen.getByText('Всего книг')).toBeInTheDocument());
    const headings = screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent);
    expect(headings).toEqual(['Модерация', 'Каталог', 'Импорт']);
  });

  it('renders a card for pending books with a link to the moderation queue', async () => {
    renderView();

    await waitFor(() => expect(screen.getByText('Книги на модерации')).toBeInTheDocument());
    const moderation = group('Модерация');
    expect(within(moderation).getByText('4')).toBeInTheDocument();
    expect(screen.queryByText('Импортировано, на проверке')).not.toBeInTheDocument();
    expect(within(moderation).getByText('Книги на модерации').closest('a')).toHaveAttribute(
      'href',
      '/moderation/books'
    );
  });

  it('renders a card for pending authors linking to the author moderation queue', async () => {
    renderView();

    await waitFor(() => expect(screen.getByText('Авторы на модерации')).toBeInTheDocument());
    const moderation = group('Модерация');
    expect(within(moderation).getByText('7')).toBeInTheDocument();
    expect(within(moderation).getByText('Авторы на модерации').closest('a')).toHaveAttribute(
      'href',
      '/moderation/authors'
    );
  });

  it('renders a card for pending genres linking to the genre moderation queue', async () => {
    renderView();

    await waitFor(() => expect(screen.getByText('Жанры на модерации')).toBeInTheDocument());
    const moderation = group('Модерация');
    expect(within(moderation).getByText('6')).toBeInTheDocument();
    expect(within(moderation).getByText('Жанры на модерации').closest('a')).toHaveAttribute(
      'href',
      '/moderation/genres'
    );
  });

  it('labels the reviews tile "Скрытых отзывов" inside Модерация', async () => {
    renderView();

    await waitFor(() => expect(screen.getByText('Скрытых отзывов')).toBeInTheDocument());
    expect(screen.queryByText('Отзывов на модерации')).not.toBeInTheDocument();
    expect(within(group('Модерация')).getByText('Скрытых отзывов').closest('a')).toHaveAttribute(
      'href',
      '/moderation/reviews?visibility=hidden'
    );
  });

  it('renders the catalog tiles inside Каталог', async () => {
    renderView();

    await waitFor(() => expect(screen.getByText('Всего книг')).toBeInTheDocument());
    const catalog = group('Каталог');
    expect(within(catalog).getByText('Всего книг').closest('a')).toHaveAttribute(
      'href',
      '/catalog/books'
    );
    expect(within(catalog).getByText('Отклонённых книг').closest('a')).toHaveAttribute(
      'href',
      '/catalog/books?status=rejected'
    );
    expect(within(catalog).getByText('3')).toBeInTheDocument();
    expect(screen.queryByText('Скрытых книг')).not.toBeInTheDocument();
    expect(within(catalog).getByText('Авторов').closest('a')).toHaveAttribute(
      'href',
      '/catalog/authors'
    );
    expect(within(catalog).getByText('Жанров').closest('a')).toHaveAttribute(
      'href',
      '/catalog/genres'
    );
  });

  it('shows at most 5 harvest jobs with id prefix, state and one-line status', async () => {
    listJobsMock.mockResolvedValue({
      items: [
        job('active-job-1234567890', 'active', { result: result({ processed: 3, total: 10 }) }),
        job('retry-job-1234567890', 'retry', { last_error: 'wikidata timeout' }),
        job('completed-job-123456', 'completed', { result: result({ created: 6 }) }),
        job('job-4', 'completed', { result: result({ created: 1 }) }),
        job('job-5', 'archived', { last_error: 'boom' }),
        job('job-6', 'archived'),
        job('job-7', 'completed'),
      ],
    });
    renderView();

    const importGroup = group('Импорт');
    const list = await within(importGroup).findByRole('list', { name: 'Задачи харвеста' });
    const rows = within(list).getAllByRole('listitem');
    expect(rows).toHaveLength(5);
    expect(rows[0]).toHaveTextContent('active-job-1');
    expect(rows[0]).toHaveTextContent('active');
    expect(rows[0]).toHaveTextContent('3 / 10');
    expect(rows[1]).toHaveTextContent('retry');
    expect(within(rows[1]).getByText('wikidata timeout')).toHaveAttribute(
      'title',
      'wikidata timeout'
    );
    expect(rows[2]).toHaveTextContent('completed-jo');
    expect(rows[2]).toHaveTextContent('создано 6');
    expect(within(importGroup).queryByText('job-6')).not.toBeInTheDocument();
    expect(within(importGroup).getByText('в работе: 2')).toBeInTheDocument();
  });

  it('shows "Выполняется…" for an active job without a known total', async () => {
    listJobsMock.mockResolvedValue({ items: [job('active-1', 'active')] });
    renderView();

    expect(await within(group('Импорт')).findByText('Выполняется…')).toBeInTheDocument();
  });

  it('links to the jobs page and the harvest trigger', async () => {
    renderView();

    const importGroup = group('Импорт');
    expect(within(importGroup).getByRole('link', { name: 'Все задачи →' })).toHaveAttribute(
      'href',
      '/import/jobs'
    );
    expect(within(importGroup).getByRole('link', { name: 'Запустить харвест' })).toHaveAttribute(
      'href',
      '/import/books'
    );
  });

  it('shows "Нет задач" and no in-flight counter when the jobs list is empty', async () => {
    renderView();

    const importGroup = group('Импорт');
    expect(await within(importGroup).findByText('Нет задач')).toBeInTheDocument();
    expect(within(importGroup).queryByText(/в работе/)).not.toBeInTheDocument();
  });

  it('a jobs error does not hide the counters', async () => {
    listJobsMock.mockRejectedValue(new Error('boom'));
    renderView();

    expect(
      await within(group('Импорт')).findByText('Не удалось загрузить задачи харвеста.')
    ).toBeInTheDocument();
    expect(await screen.findByText('Всего книг')).toBeInTheDocument();
    expect(
      screen.queryByText('Не удалось загрузить данные панели управления.')
    ).not.toBeInTheDocument();
  });

  it('a stats error does not hide the jobs block', async () => {
    dashboardMock.mockRejectedValue(new Error('boom'));
    listJobsMock.mockResolvedValue({ items: [job('pending-1', 'pending')] });
    renderView();

    expect(
      await screen.findByText('Не удалось загрузить данные панели управления.')
    ).toBeInTheDocument();
    const importGroup = group('Импорт');
    expect(await within(importGroup).findByText('pending-1')).toBeInTheDocument();
    expect(within(importGroup).getByText('в работе: 1')).toBeInTheDocument();
    expect(screen.queryByText('Всего книг')).not.toBeInTheDocument();
  });
});
