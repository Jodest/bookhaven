import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type {
  AdminAiCollectionDraft,
  AdminAiCollectionDraftDetail,
  AdminAiCollectionsStatus,
  AiDraftLine,
} from '@bookhaven/api-client';
import { pasteBook, pasteLine } from '@/shared/lib/paste-fixtures';
import { AiDraftPanel } from './ai-draft-panel';

const statusMock = vi.fn();
const listMock = vi.fn();
const getMock = vi.fn();
const createMock = vi.fn();
const bulkMock = vi.fn();

vi.mock('../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api')>()),
  adminAiCollectionsApi: {
    status: (...a: unknown[]) => statusMock(...a),
    list: (...a: unknown[]) => listMock(...a),
    get: (...a: unknown[]) => getMock(...a),
    create: (...a: unknown[]) => createMock(...a),
  },
}));

vi.mock('@/entities/curated-collection/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/curated-collection/api')>()),
  adminCollectionsApi: { bulkAddBooks: (...a: unknown[]) => bulkMock(...a) },
}));

function status(over: Partial<AdminAiCollectionsStatus> = {}): AdminAiCollectionsStatus {
  return {
    enabled: true,
    model: 'claude-sonnet-5-5',
    prompt_version: 1,
    max_brief_length: 500,
    max_count: 1000,
    daily_limit: 20,
    used_today: 2,
    ...over,
  };
}

function summary(over: Partial<AdminAiCollectionDraft> = {}): AdminAiCollectionDraft {
  return {
    id: 'd1',
    collection_id: 'c1',
    brief: 'Топ научной фантастики',
    count: 120,
    status: 'done',
    error_code: null,
    proposals_count: 5,
    model: 'claude-sonnet-5-5',
    prompt_version: 1,
    input_tokens: 0,
    output_tokens: 0,
    created_by: { id: 'u1', username: 'admin' },
    created_at: '2026-10-15T12:00:00Z',
    finished_at: '2026-10-15T12:01:00Z',
    ...over,
  };
}

function aiLine(n: number, st: AiDraftLine['status'], over: Partial<AiDraftLine> = {}): AiDraftLine {
  return { ...pasteLine(n, st), original_title: null, note: '', ...over };
}

function detail(
  lines: AiDraftLine[],
  over: Partial<AdminAiCollectionDraftDetail> = {},
  free = 100
): AdminAiCollectionDraftDetail {
  const counts = { found: 0, ambiguous: 0, not_found: 0, rejected: 0, in_collection: 0, duplicate: 0, invalid: 0 };
  for (const l of lines) counts[l.status] += 1;
  return { ...summary(), report: { books_count: 1000 - free, free_slots: free, counts, lines }, ...over };
}

function renderPanel(size = 100) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <AiDraftPanel collectionId="c1" size={size} booksCount={3} />
    </QueryClientProvider>
  );
}

const submit = () => screen.getByRole('button', { name: 'Составить черновик' });
const briefField = () => screen.findByLabelText('Описание подборки для ИИ');

describe('AiDraftPanel', () => {
  beforeEach(() => {
    statusMock.mockReset().mockResolvedValue(status());
    listMock.mockReset().mockResolvedValue({ items: [] });
    getMock.mockReset();
    createMock.mockReset();
    bulkMock.mockReset();
  });

  it('disabled state: says so, no submit, earlier drafts stay readable', async () => {
    statusMock.mockResolvedValue(status({ enabled: false }));
    listMock.mockResolvedValue({ items: [summary()] });
    getMock.mockResolvedValue(detail([aiLine(1, 'found', { book: pasteBook('a') })]));
    renderPanel();

    expect(await screen.findByText('Черновики ИИ выключены')).toBeInTheDocument();
    expect(
      screen.getByText('Включите AI_COLLECTIONS_ENABLED и задайте ANTHROPIC_API_KEY для api и воркера.')
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Составить черновик' })).not.toBeInTheDocument();
    expect(await screen.findByTestId('ai-draft-counts')).toBeInTheDocument();
  });

  it('prefills the default count and disables submit for an empty brief or invalid count', async () => {
    renderPanel(100);
    const count = await screen.findByLabelText('Сколько книг предложить');
    expect(count).toHaveValue(120);
    expect(submit()).toBeDisabled();

    fireEvent.change(await briefField(), { target: { value: 'Топ фантастики' } });
    expect(screen.getByText('14 / 500')).toBeInTheDocument();
    expect(submit()).toBeEnabled();

    fireEvent.change(count, { target: { value: '0' } });
    expect(submit()).toBeDisabled();
    fireEvent.change(count, { target: { value: '1001' } });
    expect(submit()).toBeDisabled();
    fireEvent.change(count, { target: { value: '50' } });
    expect(submit()).toBeEnabled();
  });

  it('disables submit at the daily limit', async () => {
    statusMock.mockResolvedValue(status({ used_today: 20 }));
    renderPanel();
    fireEvent.change(await briefField(), { target: { value: 'x' } });
    expect(submit()).toBeDisabled();
    expect(screen.getByText('Лимит черновиков на сегодня исчерпан')).toBeInTheDocument();
  });

  it('disables submit while the collection has a running draft and shows its progress', async () => {
    listMock.mockResolvedValue({ items: [summary({ status: 'running', proposals_count: 40 })] });
    getMock.mockResolvedValue({ ...summary({ status: 'running', proposals_count: 40 }), report: null });
    renderPanel();
    fireEvent.change(await briefField(), { target: { value: 'x' } });
    await waitFor(() => expect(submit()).toBeDisabled());
    expect(await screen.findByText('ИИ составляет черновик… предложено 40 из 120')).toBeInTheDocument();
  });

  it('creates a draft and shows its progress', async () => {
    createMock.mockResolvedValue(summary({ id: 'd2', status: 'queued', proposals_count: 0 }));
    getMock.mockResolvedValue({ ...summary({ id: 'd2', status: 'queued', proposals_count: 0 }), report: null });
    renderPanel();

    fireEvent.change(await briefField(), { target: { value: '  Топ фантастики ' } });
    fireEvent.change(screen.getByLabelText('Сколько книг предложить'), { target: { value: '50' } });
    fireEvent.click(submit());

    expect(await screen.findByText('ИИ составляет черновик… предложено 0 из 120')).toBeInTheDocument();
    expect(createMock).toHaveBeenCalledWith({ collection_id: 'c1', brief: 'Топ фантастики', count: 50 });
    expect(getMock).toHaveBeenCalledWith('d2');
  });

  it('polls a running draft and switches to the report when it is done', async () => {
    listMock.mockResolvedValue({ items: [summary({ status: 'running', proposals_count: 10 })] });
    getMock
      .mockResolvedValueOnce({ ...summary({ status: 'running', proposals_count: 10 }), report: null })
      .mockResolvedValue(detail([aiLine(1, 'found', { book: pasteBook('a') })]));
    renderPanel();

    expect(await screen.findByText('ИИ составляет черновик… предложено 10 из 120')).toBeInTheDocument();
    expect(await screen.findByRole('checkbox', { name: 'Добавить строку 1' }, { timeout: 6000 })).toBeChecked();
    expect(screen.queryByText(/ИИ составляет черновик/)).not.toBeInTheDocument();
    // A finished draft is not polled again.
    const calls = getMock.mock.calls.length;
    await new Promise((r) => setTimeout(r, 3500));
    expect(getMock.mock.calls.length).toBe(calls);
  }, 15000);

  it('maps create errors to readable messages', async () => {
    createMock.mockRejectedValue(new ApiError(429, 'Too Many Requests', 'daily AI draft limit reached'));
    renderPanel();
    fireEvent.change(await briefField(), { target: { value: 'x' } });
    fireEvent.click(submit());
    expect(await screen.findByRole('alert')).toHaveTextContent('Лимит черновиков на сегодня исчерпан');
  });

  it('a failed draft shows a readable reason', async () => {
    listMock.mockResolvedValue({ items: [summary({ status: 'failed', error_code: 'model_refusal', proposals_count: 0 })] });
    getMock.mockResolvedValue({ ...summary({ status: 'failed', error_code: 'model_refusal' }), report: null });
    renderPanel();
    expect(await screen.findByText('ИИ отказался составлять список')).toBeInTheDocument();
  });

  it('a done draft with an error code says it is partial', async () => {
    listMock.mockResolvedValue({ items: [summary({ error_code: 'model_truncated' })] });
    getMock.mockResolvedValue(
      detail([aiLine(1, 'found', { book: pasteBook('a') })], { error_code: 'model_truncated' })
    );
    renderPanel();
    expect(await screen.findByText(/Черновик составлен не полностью/)).toHaveTextContent('Ответ ИИ оборвался');
  });

  it('report: found ticked, ambiguous needs a pick, not_found listed without checkboxes', async () => {
    listMock.mockResolvedValue({ items: [summary()] });
    getMock.mockResolvedValue(
      detail([
        aiLine(1, 'found', { book: pasteBook('a'), text: 'Гиперион — Дэн Симмонс', note: 'эпос о паломниках' }),
        aiLine(2, 'ambiguous', { candidates: [pasteBook('b1'), pasteBook('b2')], text: 'Дюна' }),
        aiLine(3, 'not_found', {
          title: 'Солярис',
          author: 'Станислав Лем',
          text: 'Солярис — Станислав Лем',
          original_title: 'Solaris',
        }),
        aiLine(4, 'not_found', { title: 'Эдем', author: null, text: 'Эдем' }),
      ])
    );
    renderPanel();

    expect(await screen.findByRole('checkbox', { name: 'Добавить строку 1' })).toBeChecked();
    expect(screen.getByText('эпос о паломниках')).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Добавить строку 2' })).toBeDisabled();
    expect(screen.queryByRole('checkbox', { name: 'Добавить строку 3' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Добавить выбранные (1)' })).toBeEnabled();

    const notFound = screen.getByRole('region', { name: 'Нет в каталоге' });
    expect(within(notFound).getByText('Нет в каталоге (2)')).toBeInTheDocument();
    expect(within(notFound).getByText('Солярис — Станислав Лем')).toBeInTheDocument();
    const links = within(notFound).getAllByRole('link', { name: 'Добавить в каталог' });
    expect(links).toHaveLength(2);
    expect(links[0]).toHaveAttribute('href', '/import/books/manual');
    expect(links[0]).toHaveAttribute('target', '_blank');
    expect(within(notFound).queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('copies the not-found entries as «Название — Автор» lines', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    listMock.mockResolvedValue({ items: [summary()] });
    getMock.mockResolvedValue(
      detail([
        aiLine(1, 'not_found', { title: 'Солярис', author: 'Станислав Лем' }),
        aiLine(2, 'not_found', { title: 'Эдем', author: null }),
      ])
    );
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'Скопировать ненайденные' }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('Солярис — Станислав Лем\nЭдем'));
  });

  it('apply sends the rank-ordered ids with the position and shows the result', async () => {
    listMock.mockResolvedValue({ items: [summary()] });
    getMock.mockResolvedValue(
      detail([
        aiLine(1, 'found', { book: pasteBook('a'), text: 'Первая', title: 'Первая' }),
        aiLine(2, 'ambiguous', { candidates: [pasteBook('b1'), pasteBook('b2')], text: 'Вторая' }),
        aiLine(3, 'found', { book: pasteBook('c'), text: 'Третья', title: 'Третья' }),
      ])
    );
    bulkMock.mockResolvedValue({
      added: [
        { book_id: 'a', position: 4 },
        { book_id: 'b2', position: 5 },
      ],
      skipped: [{ book_id: 'c', reason: 'already_in_collection' }],
      books_count: 5,
    });
    renderPanel();

    await screen.findByRole('checkbox', { name: 'Добавить строку 1' });
    fireEvent.click(screen.getByRole('radio', { name: /Выбрать: Книга b2/ }));
    fireEvent.click(screen.getByRole('radio', { name: 'На позицию' }));
    fireEvent.change(screen.getByLabelText('Позиция'), { target: { value: '2' } });
    fireEvent.click(screen.getByRole('button', { name: 'Добавить выбранные (3)' }));

    await waitFor(() => expect(bulkMock).toHaveBeenCalledWith('c1', ['a', 'b2', 'c'], 2));
    const result = await screen.findByTestId('ai-draft-result');
    expect(result).toHaveTextContent('Добавлено: 2');
    expect(result).toHaveTextContent('Пропущено: 1');
    expect(result).toHaveTextContent('Третья — уже в подборке');
  });

  it('keeps the result after the report is refetched and marks added books as already in', async () => {
    listMock.mockResolvedValue({ items: [summary()] });
    getMock
      .mockResolvedValueOnce(detail([aiLine(1, 'found', { book: pasteBook('a') })]))
      .mockResolvedValue(
        detail([aiLine(1, 'in_collection', { book: pasteBook('a', { in_collection: true }) })])
      );
    bulkMock.mockResolvedValue({ added: [{ book_id: 'a', position: 4 }], skipped: [], books_count: 4 });
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'Добавить выбранные (1)' }));

    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.getAllByText(/Уже в подборке/).length).toBeGreaterThan(0));
    expect(screen.getByTestId('ai-draft-result')).toHaveTextContent('Добавлено: 1');
  });

  it('default mode appends: no position is sent', async () => {
    listMock.mockResolvedValue({ items: [summary()] });
    getMock.mockResolvedValue(detail([aiLine(1, 'found', { book: pasteBook('a') })]));
    bulkMock.mockResolvedValue({ added: [{ book_id: 'a', position: 4 }], skipped: [], books_count: 4 });
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'Добавить выбранные (1)' }));
    await waitFor(() => expect(bulkMock).toHaveBeenCalledWith('c1', ['a'], undefined));
  });

  it('blocks apply when more books are ticked than free places', async () => {
    listMock.mockResolvedValue({ items: [summary()] });
    getMock.mockResolvedValue(
      detail([aiLine(1, 'found', { book: pasteBook('a') }), aiLine(2, 'found', { book: pasteBook('b') })], {}, 1)
    );
    renderPanel();
    expect(await screen.findByRole('button', { name: 'Добавить выбранные (2)' })).toBeDisabled();
    expect(screen.getByText('Свободно мест: 1 — снимите лишние отметки')).toBeInTheDocument();
  });
});
