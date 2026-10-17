import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type AdminGenreAISuggestion, type AdminGenreAIStatus } from '@bookhaven/api-client';
import { GenreAiSuggestionsView } from './genre-ai-suggestions-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/moderation/genres/ai-suggestions',
  useSearchParams: () => searchParamsValue,
}));

const listMock = vi.fn();
const statusMock = vi.fn();
const analyzeMock = vi.fn();
const applyMock = vi.fn();
const bulkApplyMock = vi.fn();
const dismissMock = vi.fn();
const bulkDismissMock = vi.fn();
const previewMock = vi.fn();

vi.mock('@/entities/genre/api', () => ({
  adminGenresApi: {
    list: vi.fn(),
    mergePreview: (...args: unknown[]) => previewMock(...args),
  },
}));

vi.mock('@/entities/genre-ai-suggestion/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre-ai-suggestion/api')>()),
  adminGenreAiSuggestionsApi: {
    list: (...args: unknown[]) => listMock(...args),
    status: (...args: unknown[]) => statusMock(...args),
    analyze: (...args: unknown[]) => analyzeMock(...args),
    apply: (...args: unknown[]) => applyMock(...args),
    bulkApply: (...args: unknown[]) => bulkApplyMock(...args),
    dismiss: (...args: unknown[]) => dismissMock(...args),
    bulkDismiss: (...args: unknown[]) => bulkDismissMock(...args),
  },
}));

function genre(id: string, name: string, over: Partial<AdminGenreAISuggestion['genres'][number]> = {}) {
  return {
    id,
    name,
    status: 'published' as const,
    merged: false,
    exists: true,
    wikidata_qid: null,
    books_total: 12,
    sample_books: [{ id: `b-${id}`, title: `Книга ${name}`, status: 'published' }],
    ...over,
  };
}

function suggestion(over: Partial<AdminGenreAISuggestion> = {}): AdminGenreAISuggestion {
  return {
    id: 's1',
    kind: 'duplicate',
    status: 'open',
    reason_codes: ['spelling', 'language'],
    reason: 'Это один и тот же жанр.',
    confidence: 0.934,
    model: 'claude-sonnet-5-5',
    prompt_version: 1,
    created_at: '2026-10-15T10:00:00Z',
    decided_at: null,
    stale: false,
    target: { id: 'g1', name: 'Фэнтези' },
    genres: [genre('g1', 'Фэнтези'), genre('g2', 'Фентези', { status: 'pending' })],
    applied_target: null,
    ...over,
  };
}

const junk = suggestion({
  id: 's2',
  kind: 'junk',
  reason_codes: ['date'],
  reason: 'Год, а не жанр.',
  confidence: 0.8,
  target: null,
  genres: [genre('g3', 'Книги 2020', { status: 'pending', books_total: 7 })],
});

const stale = suggestion({
  id: 's3',
  kind: 'narrow',
  stale: true,
  reason_codes: ['too_narrow'],
  target: { id: 'g4', name: 'Киберпанк' },
  genres: [genre('g4', 'Киберпанк'), genre('g5', 'Киберпанк-нуар', { exists: false, status: '', books_total: 0, sample_books: [] })],
});

function aiStatus(over: Partial<AdminGenreAIStatus> = {}): AdminGenreAIStatus {
  return {
    enabled: true,
    model: 'claude-sonnet-5-5',
    prompt_version: 1,
    open_count: 3,
    last_run: {
      id: 'r1',
      trigger: 'manual',
      status: 'done',
      batches_total: 1,
      batches_done: 1,
      suggestions_created: 3,
      error_code: null,
      created_at: '2026-10-15T10:00:00Z',
      finished_at: '2026-10-15T10:05:00Z',
    },
    ...over,
  };
}

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <GenreAiSuggestionsView />
    </QueryClientProvider>
  );
}

function rowOf(text: string) {
  return screen.getByText(text, { selector: 'span.font-medium, span.font-medium.line-through' }).closest('tr') as HTMLElement;
}

describe('GenreAiSuggestionsView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    for (const m of [pushMock, listMock, statusMock, analyzeMock, applyMock, bulkApplyMock, dismissMock, bulkDismissMock, previewMock]) {
      m.mockReset();
    }
    statusMock.mockResolvedValue(aiStatus());
    listMock.mockResolvedValue({ items: [suggestion(), junk, stale], total: 3 });
    previewMock.mockResolvedValue({ books_reassigned: 3, collections: [] });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('requests the open tab and shows the rows with kind, genres, action, reason and confidence', async () => {
    renderView();

    expect(screen.getByRole('heading', { name: 'Предложения ИИ по жанрам' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getAllByText('Это один и тот же жанр.').length).toBeGreaterThan(0));
    expect(listMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'open', page: 1, limit: 20 }));

    const row = rowOf('Фентези');
    expect(within(row).getByText('Дубликат')).toBeInTheDocument();
    expect(within(row).getByText('Слить в «Фэнтези»')).toBeInTheDocument();
    expect(within(row).getByText('Книга Фентези')).toBeInTheDocument();
    expect(within(row).getAllByText('книг: 12').length).toBeGreaterThan(0);
    expect(within(row).getByText(/написание, другой язык · уверенность 93%/)).toBeInTheDocument();

    const junkRow = rowOf('Книги 2020');
    expect(within(junkRow).getByText('Не жанр')).toBeInTheDocument();
    expect(within(junkRow).getByText('Отклонить жанр')).toBeInTheDocument();
    expect(within(junkRow).getByText(/дата · уверенность 80%/)).toBeInTheDocument();
  });

  it('shows the last run line', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText(/^Последний анализ: /)).toBeInTheDocument());

    statusMock.mockResolvedValue(aiStatus({ last_run: { ...aiStatus().last_run!, status: 'running', finished_at: null } }));
    renderView();
    expect(await screen.findByText(/идёт…/)).toBeInTheDocument();
  });

  it('marks a stale open row, disables «Применить» and keeps «Отклонить»', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText('Киберпанк-нуар')).toBeInTheDocument());

    const row = rowOf('Киберпанк-нуар');
    expect(within(row).getByText('устарело')).toBeInTheDocument();
    expect(within(row).getByRole('button', { name: 'Применить' })).toBeDisabled();
    expect(within(row).getByRole('button', { name: 'Отклонить' })).toBeEnabled();
    expect(within(row).getByRole('checkbox')).toBeDisabled();
    expect(within(row).getByText('удалён')).toBeInTheDocument();
  });

  it('hides «Запустить анализ» and explains when the analysis is switched off, keeping the actions', async () => {
    statusMock.mockResolvedValue(aiStatus({ enabled: false }));
    renderView();
    await waitFor(() => expect(screen.getByText('ИИ-анализ жанров выключен.')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: 'Запустить анализ' })).not.toBeInTheDocument();
    expect(within(rowOf('Фентези')).getByRole('button', { name: 'Применить' })).toBeEnabled();
  });

  it('queues an analysis after confirming and reports it', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    analyzeMock.mockResolvedValue({ run_id: 'r2' });
    renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Запустить анализ' }));
    expect(confirmSpy).toHaveBeenCalledWith('Запустить ИИ-анализ жанров? Это займёт несколько минут.');
    expect(
      await screen.findByText('Анализ поставлен в очередь. Предложения появятся через несколько минут.')
    ).toBeInTheDocument();
  });

  it('shows «Анализ уже идёт.» on 409 and does nothing when the confirmation is declined', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValueOnce(false).mockReturnValue(true);
    analyzeMock.mockRejectedValue(new ApiError(409, 'Conflict', 'AI genre analysis already running'));
    renderView();

    const button = await screen.findByRole('button', { name: 'Запустить анализ' });
    fireEvent.click(button);
    expect(analyzeMock).not.toHaveBeenCalled();

    fireEvent.click(button);
    expect(await screen.findByText('Анализ уже идёт.')).toBeInTheDocument();
    expect(confirmSpy).toHaveBeenCalledTimes(2);
  });

  it('applies a junk suggestion after the confirmation and reports it', async () => {
    applyMock.mockResolvedValue({ suggestion: { ...junk, status: 'applied' }, merge: null, status_report: { processed: 1, updated: 1, errors: [] } });
    renderView();
    await waitFor(() => expect(screen.getByText('Книги 2020')).toBeInTheDocument());

    fireEvent.click(within(rowOf('Книги 2020')).getByRole('button', { name: 'Применить' }));
    expect(
      screen.getByText('Отклонить жанр «Книги 2020»? Связи с книгами (7) сохранятся, жанр исчезнет со страниц книг.')
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Отклонить жанр' }));

    expect(await screen.findByText('Жанр «Книги 2020» отклонён.')).toBeInTheDocument();
    expect(applyMock).toHaveBeenCalledWith('s2', undefined);
  });

  it('applies a merge suggestion through the dialog and reports books and switched collections', async () => {
    applyMock.mockResolvedValue({
      suggestion: { ...suggestion(), status: 'applied' },
      merge: { genre: { id: 'g1', name: 'Фэнтези', slug: 'f', status: 'published', wikidata_qid: null }, sources_merged: 1, books_reassigned: 5, collections_switched: 1 },
      status_report: null,
    });
    renderView();
    await waitFor(() => expect(screen.getByText('Фентези')).toBeInTheDocument());

    fireEvent.click(within(rowOf('Фентези')).getByRole('button', { name: 'Применить' }));
    const confirm = await screen.findByRole('button', { name: 'Слить 1 в «Фэнтези»' });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);

    expect(
      await screen.findByText('Жанры слиты в «Фэнтези». Перенесено книг: 5. Переключено подборок: 1.')
    ).toBeInTheDocument();
  });

  it('applies the selection through the bulk dialog and reports the counts', async () => {
    bulkApplyMock.mockResolvedValue({ processed: 2, applied: 1, stale: 1, errors: [] });
    renderView();
    await waitFor(() => expect(screen.getByText('Фентези')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /Применить выбранные/ })).not.toBeInTheDocument();

    fireEvent.click(within(rowOf('Фентези')).getByRole('checkbox'));
    fireEvent.click(within(rowOf('Книги 2020')).getByRole('checkbox'));
    fireEvent.click(screen.getByRole('button', { name: 'Применить выбранные (2)' }));

    const confirm = await screen.findByRole('button', { name: 'Применить (2)' });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);

    expect(await screen.findByText('Применено: 1. Устарело: 1. Ошибок: 0.')).toBeInTheDocument();
    expect(bulkApplyMock).toHaveBeenCalledWith([{ id: 's1' }, { id: 's2' }]);
    expect(screen.queryByRole('button', { name: /Применить выбранные/ })).not.toBeInTheDocument();
  });

  it('selects only the non-stale rows with «Выбрать все»', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText('Фентези')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать все на странице' }));
    expect(screen.getByRole('button', { name: 'Применить выбранные (2)' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Отклонить выбранные (2)' })).toBeInTheDocument();
  });

  it('dismisses the selection after confirming and reports the count', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkDismissMock.mockResolvedValue({ processed: 2, dismissed: 2, errors: [] });
    renderView();
    await waitFor(() => expect(screen.getByText('Фентези')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать все на странице' }));
    fireEvent.click(screen.getByRole('button', { name: 'Отклонить выбранные (2)' }));

    expect(confirmSpy).toHaveBeenCalledWith(
      'Отклонить выбранные предложения (2)? ИИ больше не предложит их для тех же жанров.'
    );
    expect(await screen.findByText('Отклонено предложений: 2.')).toBeInTheDocument();
    expect(bulkDismissMock).toHaveBeenCalledWith(['s1', 's2']);
  });

  it('dismisses one row after confirming', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    dismissMock.mockResolvedValue({ ...stale, status: 'dismissed' });
    renderView();
    await waitFor(() => expect(screen.getByText('Киберпанк-нуар')).toBeInTheDocument());

    fireEvent.click(within(rowOf('Киберпанк-нуар')).getByRole('button', { name: 'Отклонить' }));
    expect(confirmSpy).toHaveBeenCalledWith('Отклонить предложение? ИИ больше не предложит его для тех же жанров.');
    expect(await screen.findByText('Предложение отклонено.')).toBeInTheDocument();
    expect(dismissMock).toHaveBeenCalledWith('s3');
  });

  it('keeps tab, kind filter and page in the URL', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText('Фентези')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: 'Устаревшие' }));
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/genres/ai-suggestions?status=stale');

    fireEvent.change(screen.getByLabelText('Тип предложения'), { target: { value: 'junk' } });
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/genres/ai-suggestions?kind=junk');
  });

  it('reads tab, kind and page from the URL', async () => {
    searchParamsValue = new URLSearchParams('status=applied&kind=junk&page=2');
    listMock.mockResolvedValue({ items: [], total: 0 });
    renderView();
    await waitFor(() =>
      expect(listMock).toHaveBeenCalledWith({ status: 'applied', kind: 'junk', page: 2, limit: 20 })
    );
    expect(screen.getByRole('button', { name: 'Применённые' })).toHaveAttribute('aria-pressed', 'true');
    expect(await screen.findByText('Предложений нет')).toBeInTheDocument();
  });

  it('shows the empty open tab text', async () => {
    listMock.mockResolvedValue({ items: [], total: 0 });
    renderView();
    expect(await screen.findByText('Открытых предложений нет')).toBeInTheDocument();
  });

  it('shows a load error', async () => {
    listMock.mockRejectedValue(new Error('boom'));
    renderView();
    expect(await screen.findByText('Не удалось загрузить предложения ИИ по жанрам.')).toBeInTheDocument();
  });
});
