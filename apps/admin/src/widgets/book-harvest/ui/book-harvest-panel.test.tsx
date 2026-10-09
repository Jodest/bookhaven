import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { BookHarvestPanel } from './book-harvest-panel';

const triggerMock = vi.fn();
const listJobsMock = vi.fn();
const searchAuthorsMock = vi.fn();

vi.mock('@/entities/harvest', () => ({
  HARVEST_JOBS_QUERY_KEY: ['admin', 'harvest', 'jobs'],
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
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <BookHarvestPanel />
    </QueryClientProvider>
  );
  return { ...utils, invalidateSpy };
}

function switchToManualMode() {
  fireEvent.click(screen.getByRole('button', { name: 'Ввести QID вручную' }));
}

async function triggerManually(qids: string) {
  fireEvent.change(screen.getByLabelText('QID авторов Wikidata'), {
    target: { value: qids },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Запустить харвест' }));
  await waitFor(() => expect(screen.getByText(/Харвест запущен/)).toBeInTheDocument());
}

describe('BookHarvestPanel', () => {
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

  it('renders without its own heading and does not fetch the jobs list', () => {
    renderView();

    expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Запустить харвест' })).toBeInTheDocument();
    expect(listJobsMock).not.toHaveBeenCalled();
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

  it('on success invalidates the jobs query, clears the form and shows a confirmation with a link to /import/jobs (manual mode)', async () => {
    triggerMock.mockResolvedValue({ task_id: 'task-1234567890abcdef', queue: 'harvest', state: 'pending' });
    const { invalidateSpy } = renderView();
    switchToManualMode();

    await triggerManually('Q7200');

    expect(triggerMock).toHaveBeenCalled();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'harvest', 'jobs'] });
    expect(screen.getByLabelText('QID авторов Wikidata')).toHaveValue('');
    expect(screen.getByRole('status')).toHaveTextContent(
      'Харвест запущен (задача task-1234567). Ход выполнения — на вкладке «Задачи».'
    );
    expect(screen.getByRole('link', { name: 'К задачам' })).toHaveAttribute('href', '/import/jobs');
    expect(listJobsMock).not.toHaveBeenCalled();
  });

  it('hides the confirmation on "Скрыть"', async () => {
    triggerMock.mockResolvedValue({ task_id: 'task-1', queue: 'harvest', state: 'pending' });
    renderView();
    switchToManualMode();

    await triggerManually('Q7200');
    fireEvent.click(screen.getByRole('button', { name: 'Скрыть' }));

    expect(screen.queryByText(/Харвест запущен/)).not.toBeInTheDocument();
  });

  it('hides the confirmation on mode switch', async () => {
    triggerMock.mockResolvedValue({ task_id: 'task-1', queue: 'harvest', state: 'pending' });
    renderView();
    switchToManualMode();

    await triggerManually('Q7200');
    fireEvent.click(screen.getByRole('button', { name: 'Поиск по имени' }));

    expect(screen.queryByText(/Харвест запущен/)).not.toBeInTheDocument();
  });

  it('hides the confirmation on the next submit', async () => {
    triggerMock.mockResolvedValue({ task_id: 'task-1', queue: 'harvest', state: 'pending' });
    renderView();
    switchToManualMode();

    await triggerManually('Q7200');
    // Empty form: validation error, the previous confirmation is cleared.
    fireEvent.click(screen.getByRole('button', { name: 'Запустить харвест' }));

    await waitFor(() =>
      expect(screen.getByText('Введите хотя бы один QID автора.')).toBeInTheDocument()
    );
    expect(screen.queryByText(/Харвест запущен/)).not.toBeInTheDocument();
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
});
