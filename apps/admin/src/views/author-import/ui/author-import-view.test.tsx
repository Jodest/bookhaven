import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type { AdminAuthor, AdminAuthorImportResult, AdminHarvestAuthorCandidate } from '@bookhaven/api-client';
import { AuthorImportView } from './author-import-view';

const importMock = vi.fn();
const searchMock = vi.fn();
const triggerMock = vi.fn();

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    importWikidata: (...args: unknown[]) => importMock(...args),
  },
}));

vi.mock('@/entities/harvest', () => ({
  HARVEST_JOBS_QUERY_KEY: ['admin', 'harvest', 'jobs'],
  adminHarvestApi: {
    searchAuthors: (...args: unknown[]) => searchMock(...args),
    trigger: (...args: unknown[]) => triggerMock(...args),
  },
}));

function author(overrides: Partial<AdminAuthor> = {}): AdminAuthor {
  return {
    id: 'a1',
    name: 'Лев Толстой',
    slug: 'lev-tolstoy',
    biography: null,
    photo_url: null,
    birth_date: null,
    death_date: null,
    status: 'pending',
    external_source: 'wikidata',
    external_id: 'Q7200',
    photo_attribution: null,
    ...overrides,
  };
}

function importResult(overrides: Partial<AdminAuthorImportResult> = {}): AdminAuthorImportResult {
  return { outcome: 'created', match: null, qid_backfilled: false, namesake_of: null, enrichment: null, author: author(), ...overrides };
}

const freshCandidate: AdminHarvestAuthorCandidate = {
  qid: 'Q7200',
  label: 'Лев Толстой',
  description: 'русский писатель',
  existing: null,
};

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  render(
    <QueryClientProvider client={queryClient}>
      <AuthorImportView />
    </QueryClientProvider>
  );
  return { invalidateSpy };
}

async function searchWith(candidates: AdminHarvestAuthorCandidate[]) {
  searchMock.mockResolvedValue({ items: candidates });
  fireEvent.change(screen.getByLabelText('Имя автора в Wikidata'), { target: { value: 'Толстой' } });
  fireEvent.click(screen.getByRole('button', { name: 'Найти' }));
  await waitFor(() => expect(screen.getByText(candidates[0].label)).toBeInTheDocument());
}

describe('AuthorImportView', () => {
  let confirmSpy: MockInstance<(message?: string) => boolean>;

  beforeEach(() => {
    importMock.mockReset();
    searchMock.mockReset();
    triggerMock.mockReset();
    confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the page heading', () => {
    renderView();

    expect(screen.getByRole('heading', { level: 1, name: 'Импорт автора из Wikidata' })).toBeInTheDocument();
  });

  it('imports only the author without triggering a harvest, then refreshes search and caches', async () => {
    importMock.mockResolvedValue(importResult());
    const { invalidateSpy } = renderView();
    await searchWith([freshCandidate]);

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать автора' }));

    await waitFor(() =>
      expect(screen.getByText('Автор «Лев Толстой» импортирован и ждёт модерации.')).toBeInTheDocument()
    );
    expect(importMock).toHaveBeenCalledWith('Q7200');
    expect(triggerMock).not.toHaveBeenCalled();
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'authors'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
    await waitFor(() => expect(searchMock).toHaveBeenCalledTimes(2));
    expect(searchMock.mock.calls[1][0]).toBe('Толстой');
  });

  it('imports with books: import first, then trigger with the stored QID', async () => {
    importMock.mockResolvedValue(importResult({ author: author({ external_id: 'Q999' }) }));
    triggerMock.mockResolvedValue({ task_id: 't1', queue: 'harvest', state: 'pending' });
    const { invalidateSpy } = renderView();
    await searchWith([freshCandidate]);

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать с книгами' }));

    await waitFor(() => expect(triggerMock).toHaveBeenCalled());
    expect(confirmSpy).toHaveBeenCalledWith(
      'Запустить харвест книг автора «Лев Толстой»? Книги появятся скрытыми в очереди на проверку.'
    );
    expect(importMock.mock.invocationCallOrder[0]).toBeLessThan(triggerMock.mock.invocationCallOrder[0]);
    expect(triggerMock).toHaveBeenCalledWith({ author_qids: ['Q999'] });
    await waitFor(() =>
      expect(screen.getByText(/Харвест книг запущен/)).toBeInTheDocument()
    );
    expect(screen.getByRole('link', { name: 'К задачам харвеста' })).toHaveAttribute('href', '/import/jobs');
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'harvest', 'jobs'] });
  });

  it('does not trigger a harvest for a rejected outcome and links to the author', async () => {
    importMock.mockResolvedValue(
      importResult({ outcome: 'rejected', match: 'slug', author: author({ status: 'rejected' }) })
    );
    renderView();
    await searchWith([freshCandidate]);

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать с книгами' }));

    await waitFor(() =>
      expect(
        screen.getByText(
          'Автор «Лев Толстой» был отклонён ранее и повторно не создаётся. Книги не импортированы.'
        )
      ).toBeInTheDocument()
    );
    expect(triggerMock).not.toHaveBeenCalled();
    expect(screen.getByRole('link', { name: 'Открыть' })).toHaveAttribute(
      'href',
      `/moderation/authors?${new URLSearchParams({ status: 'rejected', search: 'Лев Толстой' })}`
    );
  });

  it('shows the no_ru_label message and does not trigger a harvest', async () => {
    importMock.mockRejectedValue(new ApiError(422, 'Unprocessable Entity', 'no ru', '', 'no_ru_label'));
    renderView();
    await searchWith([freshCandidate]);

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать с книгами' }));

    await waitFor(() =>
      expect(
        screen.getByText('У элемента Wikidata нет русского названия — импорт невозможен. Создайте автора вручную.')
      ).toBeInTheDocument()
    );
    expect(triggerMock).not.toHaveBeenCalled();
  });

  it('reports a harvest failure after a successful import', async () => {
    importMock.mockResolvedValue(importResult({ outcome: 'existing', match: 'qid' }));
    triggerMock.mockRejectedValue(new ApiError(500, 'Internal Server Error', 'failed to enqueue'));
    renderView();
    await searchWith([
      { ...freshCandidate, existing: { id: 'a1', name: 'Лев Толстой', slug: 'lev-tolstoy', status: 'pending' } },
    ]);

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать книги' }));

    await waitFor(() =>
      expect(
        screen.getByText(
          'Автор импортирован, но харвест не запущен: failed to enqueue. Нажмите «Импортировать книги» ещё раз.'
        )
      ).toBeInTheDocument()
    );
    expect(importMock).toHaveBeenCalledWith('Q7200');
  });

  it('existing candidates offer only "Импортировать книги"; rejected ones offer no import', async () => {
    renderView();
    await searchWith([
      { ...freshCandidate, existing: { id: 'a1', name: 'Лев Толстой', slug: 'lev-tolstoy', status: 'published' } },
      {
        qid: 'Q1',
        label: 'Мусор',
        existing: { id: 'a2', name: 'Мусор', slug: 'musor', status: 'rejected' },
      },
    ]);

    expect(screen.getAllByRole('button', { name: 'Импортировать книги' })).toHaveLength(1);
    expect(screen.queryByRole('button', { name: 'Импортировать автора' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Импортировать с книгами' })).not.toBeInTheDocument();
    expect(screen.getByText('Отклонён ранее')).toBeInTheDocument();
    expect(screen.getAllByRole('link', { name: 'Открыть' })).toHaveLength(2);
  });

  it('cancelling the confirm calls nothing', async () => {
    confirmSpy.mockReturnValue(false);
    renderView();
    await searchWith([freshCandidate]);

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать с книгами' }));

    expect(confirmSpy).toHaveBeenCalled();
    expect(importMock).not.toHaveBeenCalled();
    expect(triggerMock).not.toHaveBeenCalled();
  });
});
