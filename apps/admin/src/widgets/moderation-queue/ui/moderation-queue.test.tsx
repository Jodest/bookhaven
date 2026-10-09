import { act, render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider, useMutation } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { ModerationQueue } from './moderation-queue';
import { moderationCreatedAtColumn } from './created-at-column';
import type {
  ModerationQueueControls,
  ModerationQueueProps,
  ModerationQueueState,
  ModerationStatus,
  ModerationStatusChange,
  ModerationStatusReport,
} from '../model/types';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/moderation/things',
  useSearchParams: () => searchParamsValue,
}));

interface Thing {
  id: string;
  name: string;
  status: ModerationStatus;
  created_at: string;
}

type Props = ModerationQueueProps<Thing, ModerationQueueState, ModerationStatusReport>;

const queryFnMock = vi.fn();
const bulkMock = vi.fn();
const buildParamsMock = vi.fn((state: ModerationQueueState) => state);
const rowConfirmMock = vi.fn((next: ModerationStatus, item: Thing) => `row ${next} ${item.name}`);
const bulkConfirmMock = vi.fn((next: ModerationStatus, items: Thing[]) => `bulk ${next} ${items.length}`);
const bulkSummaryMock = vi.fn(
  (report: ModerationStatusReport, _names: Record<string, string>) => `summary ${report.updated}`
);

const things: Thing[] = [
  { id: 't1', name: 'Альфа', status: 'pending', created_at: '2026-10-08T10:00:00Z' },
  { id: 't2', name: 'Бета', status: 'pending', created_at: '2026-10-08T10:00:00Z' },
];

function Harness(props: Partial<Props>) {
  const statusMutation = useMutation({
    mutationFn: ({ ids, status }: ModerationStatusChange): Promise<ModerationStatusReport> =>
      bulkMock(ids, status),
  });
  return (
    <ModerationQueue<Thing, ModerationQueueState, ModerationStatusReport>
      queryKey={['test', 'things']}
      buildParams={buildParamsMock}
      queryFn={(params) => queryFnMock(params)}
      statusMutation={statusMutation}
      getId={(i) => i.id}
      getName={(i) => i.name}
      getStatus={(i) => i.status}
      filters={[
        {
          param: 'x',
          label: 'Икс',
          options: [
            { value: '', label: 'Икс: любой' },
            { value: 'a', label: 'A' },
          ],
        },
      ]}
      columns={[
        { id: 'name', header: 'Имя', cell: (i) => <span className="font-medium">{i.name}</span> },
        moderationCreatedAtColumn((i: Thing) => i.created_at),
      ]}
      copy={{
        title: 'Модерация штук',
        searchPlaceholder: 'Поиск',
        loadError: 'Ошибка загрузки',
        emptyPending: 'Пусто на модерации',
        emptyFiltered: 'Ничего не найдено',
        statusErrorFallback: 'Запасная ошибка',
        rowConfirm: rowConfirmMock,
        bulkConfirm: bulkConfirmMock,
        rowSuccess: (next, name) => `ok ${next} ${name}`,
        bulkSummary: bulkSummaryMock,
      }}
      {...props}
    />
  );
}

function renderQueue(props: Partial<Props> = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <Harness {...props} />
    </QueryClientProvider>
  );
  return { queryClient, ...utils };
}

function rowOf(name: string) {
  return screen.getByText(name, { selector: 'span.font-medium' }).closest('tr') as HTMLElement;
}

async function loaded() {
  await waitFor(() => expect(screen.getByText('Альфа')).toBeInTheDocument());
}

describe('ModerationQueue', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    queryFnMock.mockReset();
    bulkMock.mockReset();
    buildParamsMock.mockClear();
    rowConfirmMock.mockClear();
    bulkConfirmMock.mockClear();
    bulkSummaryMock.mockClear();
    queryFnMock.mockResolvedValue({ items: things, total: 2, page: 1, limit: 20 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('builds default params and appends them to the query key', async () => {
    const { queryClient } = renderQueue();
    await loaded();
    const expected = { status: 'pending', page: 1, limit: 20, filters: { x: undefined } };
    expect(buildParamsMock).toHaveBeenCalledWith(expected);
    expect(queryFnMock).toHaveBeenCalledWith(expected);
    expect(queryClient.getQueryData(['test', 'things', expected])).toBeDefined();
    expect(screen.getByRole('heading', { name: 'Модерация штук' })).toBeInTheDocument();
  });

  it('parses the URL: invalid status/filter/page fall back to defaults', async () => {
    searchParamsValue = new URLSearchParams('status=bogus&x=zzz&page=0');
    renderQueue();
    await loaded();
    expect(buildParamsMock).toHaveBeenLastCalledWith({
      status: 'pending',
      page: 1,
      limit: 20,
      filters: { x: undefined },
    });
  });

  it('parses the URL: valid values pass through', async () => {
    searchParamsValue = new URLSearchParams('status=rejected&x=a&search=аб&page=3');
    renderQueue();
    await loaded();
    expect(buildParamsMock).toHaveBeenLastCalledWith({
      status: 'rejected',
      search: 'аб',
      page: 3,
      limit: 20,
      filters: { x: 'a' },
    });
  });

  it('treats a non-numeric page as 1', async () => {
    searchParamsValue = new URLSearchParams('page=abc');
    renderQueue();
    await loaded();
    expect(buildParamsMock).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1 }));
  });

  it('pushes URLs for filter, tab, search and pagination; filter change drops page and selection', async () => {
    searchParamsValue = new URLSearchParams('page=2');
    queryFnMock.mockResolvedValue({ items: things, total: 60, page: 2, limit: 20 });
    renderQueue();
    await loaded();

    fireEvent.click(screen.getByLabelText('Выбрать «Альфа»'));
    expect(screen.getByText('Выбрано: 1')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Икс'), { target: { value: 'a' } });
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/things?x=a');
    expect(screen.getByLabelText('Выбрать «Альфа»')).not.toBeChecked();
    expect(screen.queryByText(/Выбрано:/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Опубликованные' }));
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/things?status=published');

    fireEvent.click(screen.getByRole('button', { name: 'На модерации' }));
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/things?');

    fireEvent.change(screen.getByLabelText('Поиск'), { target: { value: '  бе  ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));
    expect(pushMock).toHaveBeenLastCalledWith(`/moderation/things?search=${encodeURIComponent('бе')}`);

    fireEvent.click(screen.getByRole('button', { name: 'Вперед' }));
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/things?page=3');
  });

  it('toggles select-all and single rows; bulk bar only with a selection', async () => {
    renderQueue();
    await loaded();
    const selectAll = screen.getByLabelText('Выбрать все на странице');

    expect(screen.queryByText(/Выбрано:/)).not.toBeInTheDocument();
    fireEvent.click(selectAll);
    expect(screen.getByLabelText('Выбрать «Альфа»')).toBeChecked();
    expect(screen.getByLabelText('Выбрать «Бета»')).toBeChecked();
    expect(selectAll).toBeChecked();
    expect(screen.getByText('Выбрано: 2')).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Выбрать «Бета»'));
    expect(selectAll).not.toBeChecked();
    expect(screen.getByText('Выбрано: 1')).toBeInTheDocument();

    fireEvent.click(selectAll);
    expect(selectAll).toBeChecked();
    fireEvent.click(selectAll);
    expect(screen.getByLabelText('Выбрать «Альфа»')).not.toBeChecked();
    expect(screen.queryByText(/Выбрано:/)).not.toBeInTheDocument();
  });

  it.each([
    ['', ['Опубликовать выбранные (1)', 'Отклонить выбранные (1)']],
    ['status=published', ['Отклонить выбранные (1)']],
    ['status=rejected', ['Вернуть на модерацию (1)', 'Опубликовать выбранные (1)']],
  ])('bulk buttons for tab "%s"', async (query, expected) => {
    searchParamsValue = new URLSearchParams(query);
    renderQueue();
    await loaded();
    fireEvent.click(screen.getByLabelText('Выбрать «Альфа»'));
    const bar = screen.getByText('Выбрано: 1').parentElement as HTMLElement;
    expect(within(bar).getAllByRole('button').map((b) => b.textContent)).toEqual(expected);
  });

  it('cancelled bulk confirm does not mutate; confirm receives the selected items', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    renderQueue();
    await loaded();
    fireEvent.click(screen.getByLabelText('Выбрать «Бета»'));
    fireEvent.click(screen.getByRole('button', { name: 'Отклонить выбранные (1)' }));
    expect(bulkConfirmMock).toHaveBeenCalledWith('rejected', [things[1]]);
    expect(bulkMock).not.toHaveBeenCalled();
  });

  it('bulk success clears the selection and shows the summary (error banner when report has errors)', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const report = { processed: 2, updated: 1, errors: [{ message: 'boom' }] };
    bulkMock.mockResolvedValue(report);
    renderQueue();
    await loaded();
    fireEvent.click(screen.getByLabelText('Выбрать все на странице'));
    fireEvent.click(screen.getByRole('button', { name: 'Опубликовать выбранные (2)' }));

    await waitFor(() => expect(bulkMock).toHaveBeenCalledWith(['t1', 't2'], 'published'));
    await waitFor(() => expect(screen.getByText('summary 1')).toBeInTheDocument());
    expect(bulkSummaryMock).toHaveBeenCalledWith(report, { t1: 'Альфа', t2: 'Бета' });
    expect(screen.getByText('summary 1').className).toContain('text-destructive');
    expect(screen.getByLabelText('Выбрать «Альфа»')).not.toBeChecked();
    expect(screen.queryByText(/Выбрано:/)).not.toBeInTheDocument();
  });

  it('bulk success without errors shows a success banner', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });
    renderQueue();
    await loaded();
    fireEvent.click(screen.getByLabelText('Выбрать «Альфа»'));
    fireEvent.click(screen.getByRole('button', { name: 'Отклонить выбранные (1)' }));
    await waitFor(() => expect(screen.getByText('summary 1')).toBeInTheDocument());
    expect(screen.getByText('summary 1').className).toContain('bg-green-100');
  });

  it('renders per-row status buttons by item status', async () => {
    queryFnMock.mockResolvedValue({
      items: [
        things[0],
        { ...things[1], id: 'p', name: 'Пуб', status: 'published' },
        { ...things[1], id: 'r', name: 'Отк', status: 'rejected' },
      ],
      total: 3,
      page: 1,
      limit: 20,
    });
    renderQueue();
    await loaded();
    const names = (row: HTMLElement) =>
      within(row)
        .getAllByRole('button')
        .map((b) => b.textContent);
    expect(names(rowOf('Альфа'))).toEqual(['Опубликовать', 'Отклонить']);
    expect(names(rowOf('Пуб'))).toEqual(['Отклонить']);
    expect(names(rowOf('Отк'))).toEqual(['Вернуть на модерацию', 'Опубликовать']);
  });

  it('row status: confirm, mutate and rowSuccess; cancel does nothing', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    bulkMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });
    renderQueue();
    await loaded();

    fireEvent.click(within(rowOf('Альфа')).getByRole('button', { name: 'Опубликовать' }));
    expect(rowConfirmMock).toHaveBeenCalledWith('published', things[0]);
    expect(confirmSpy).toHaveBeenCalledWith('row published Альфа');
    expect(bulkMock).not.toHaveBeenCalled();

    confirmSpy.mockReturnValue(true);
    fireEvent.click(within(rowOf('Альфа')).getByRole('button', { name: 'Опубликовать' }));
    await waitFor(() => expect(bulkMock).toHaveBeenCalledWith(['t1'], 'published'));
    await waitFor(() => expect(screen.getByText('ok published Альфа')).toBeInTheDocument());
  });

  it('row status: report errors and request errors', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkMock.mockResolvedValueOnce({ processed: 1, updated: 0, errors: [{ message: 'нельзя' }] });
    renderQueue();
    await loaded();

    fireEvent.click(within(rowOf('Альфа')).getByRole('button', { name: 'Отклонить' }));
    await waitFor(() =>
      expect(screen.getByText('Не удалось изменить статус «Альфа»: нельзя.')).toBeInTheDocument()
    );

    bulkMock.mockRejectedValueOnce(new ApiError(409, 'Conflict', 'Конфликт статуса'));
    fireEvent.click(within(rowOf('Бета')).getByRole('button', { name: 'Отклонить' }));
    await waitFor(() => expect(screen.getByText('Конфликт статуса')).toBeInTheDocument());

    bulkMock.mockRejectedValueOnce(new Error('network'));
    fireEvent.click(within(rowOf('Бета')).getByRole('button', { name: 'Отклонить' }));
    await waitFor(() => expect(screen.getByText('Запасная ошибка')).toBeInTheDocument());
  });

  it('renders slots: row actions after status buttons, bulk actions get the selection, overlay controls', async () => {
    const bulkSlot = vi.fn((selected: Thing[]) => <span>extra {selected.length}</span>);
    let overlayControls: ModerationQueueControls | null = null;
    renderQueue({
      renderRowActions: (item, ctx) => (
        <button type="button" onClick={() => ctx.changeStatus('rejected')}>
          Доп {item.name}
        </button>
      ),
      renderBulkActions: bulkSlot,
      renderOverlay: (controls) => {
        overlayControls = controls;
        return null;
      },
    });
    await loaded();

    const buttons = within(rowOf('Альфа'))
      .getAllByRole('button')
      .map((b) => b.textContent);
    expect(buttons).toEqual(['Опубликовать', 'Отклонить', 'Доп Альфа']);

    fireEvent.click(screen.getByLabelText('Выбрать «Бета»'));
    expect(bulkSlot).toHaveBeenLastCalledWith([things[1]], expect.anything());
    expect(screen.getByText('extra 1')).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Выбрать «Альфа»'));
    expect(overlayControls).not.toBeNull();
    // TS narrows the let to null; it is assigned inside the render callback.
    const controls = overlayControls as unknown as ModerationQueueControls;
    act(() => {
      controls.notify({ type: 'success', message: 'Готово' });
      controls.clearSelection();
    });
    expect(screen.getByText('Готово')).toBeInTheDocument();
    expect(screen.getByLabelText('Выбрать «Альфа»')).not.toBeChecked();
    expect(screen.getByLabelText('Выбрать «Бета»')).not.toBeChecked();

    fireEvent.click(screen.getByRole('button', { name: 'Скрыть' }));
    expect(screen.queryByText('Готово')).not.toBeInTheDocument();
  });

  it('shows the skeleton while loading and the load error on rejection', async () => {
    let reject: (err: Error) => void = () => {};
    queryFnMock.mockReturnValue(
      new Promise((_, r) => {
        reject = r;
      })
    );
    const { container } = renderQueue();
    expect(container.querySelector('.animate-pulse')).not.toBeNull();

    reject(new Error('fail'));
    await waitFor(() => expect(screen.getByText('Ошибка загрузки')).toBeInTheDocument());
    expect(container.querySelector('.animate-pulse')).toBeNull();
    expect(screen.queryByText('Пусто на модерации')).not.toBeInTheDocument();
  });

  it.each([
    ['', 'Пусто на модерации'],
    ['search=q', 'Ничего не найдено'],
    ['x=a', 'Ничего не найдено'],
    ['status=published', 'Ничего не найдено'],
  ])('empty state for "%s"', async (query, text) => {
    searchParamsValue = new URLSearchParams(query);
    queryFnMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    renderQueue();
    await waitFor(() => expect(screen.getByText(text)).toBeInTheDocument());
  });
});
