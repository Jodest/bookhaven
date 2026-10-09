import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider, useMutation } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { setFlash } from '@/shared/lib';
import { CatalogList } from './catalog-list';
import type {
  CatalogListProps,
  CatalogListState,
  CatalogStatus,
  CatalogStatusChange,
} from '../model/types';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/catalog/things',
  useSearchParams: () => searchParamsValue,
}));

interface Thing {
  id: string;
  name: string;
  status: CatalogStatus;
}

interface Report {
  errors: { message: string }[];
}

type Props = CatalogListProps<Thing, CatalogListState, Report>;

const queryFnMock = vi.fn();
const bulkMock = vi.fn();
const buildParamsMock = vi.fn((state: CatalogListState) => state);

const things: Thing[] = [
  { id: 't1', name: 'Альфа', status: 'published' },
  { id: 't2', name: 'Бета', status: 'pending' },
  { id: 't3', name: 'Гамма', status: 'rejected' },
];

function Harness(props: Partial<Props>) {
  const statusMutation = useMutation({
    mutationFn: ({ ids, status }: CatalogStatusChange): Promise<Report> => bulkMock(ids, status),
  });
  return (
    <CatalogList<Thing, CatalogListState, Report>
      title="Штуки"
      entityPath="/catalog/things"
      flashKey="catalog.things"
      queryKey={['test', 'things']}
      buildParams={buildParamsMock}
      queryFn={(params) => queryFnMock(params)}
      statusMutation={statusMutation}
      getId={(i) => i.id}
      getName={(i) => i.name}
      getStatus={(i) => i.status}
      statusLabels={{ published: 'Опубликована', pending: 'На модерации', rejected: 'Отклонена' }}
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
      columns={[{ id: 'name', header: 'Имя', cell: (i) => <span className="font-medium">{i.name}</span> }]}
      copy={{
        searchPlaceholder: 'Поиск',
        loadError: 'Ошибка загрузки',
        empty: 'Штуки не найдены.',
        statusErrorFallback: 'Запасная ошибка',
        unpublishConfirm: (name) => `снять ${name}?`,
        unpublishSuccess: (name) => `снято ${name}`,
      }}
      {...props}
    />
  );
}

function renderList(props: Partial<Props> = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <Harness {...props} />
    </QueryClientProvider>
  );
}

function rowOf(name: string) {
  return screen.getByText(name, { selector: 'span.font-medium' }).closest('tr') as HTMLElement;
}

async function loaded() {
  await waitFor(() => expect(screen.getByText('Альфа')).toBeInTheDocument());
}

describe('CatalogList', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    queryFnMock.mockReset();
    bulkMock.mockReset();
    buildParamsMock.mockClear();
    window.sessionStorage.clear();
    queryFnMock.mockResolvedValue({ items: things, total: 3, page: 1, limit: 20 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('URL state', () => {
    it('treats a bogus status as all and page 0 as 1; passes filter values through', async () => {
      searchParamsValue = new URLSearchParams('status=hidden&page=0&x=zzz&search=%20foo%20');
      renderList();
      await loaded();

      expect(buildParamsMock).toHaveBeenCalledWith({
        status: undefined,
        search: 'foo',
        page: 1,
        limit: 20,
        filters: { x: 'zzz' },
      });
      expect((screen.getByLabelText('Статус') as HTMLSelectElement).value).toBe('');
      expect((screen.getByLabelText('Поиск') as HTMLInputElement).value).toBe('foo');
    });

    it('parses page=abc as 1 and a valid status', async () => {
      searchParamsValue = new URLSearchParams('status=rejected&page=abc');
      renderList();
      await loaded();

      expect(buildParamsMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'rejected', page: 1 }));
      expect(queryFnMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'rejected' }));
    });

    it('status select offers all / published / pending / rejected', async () => {
      renderList();
      await loaded();
      const select = screen.getByLabelText('Статус') as HTMLSelectElement;
      expect(Array.from(select.options).map((o) => o.textContent)).toEqual([
        'Все статусы',
        'Опубликована',
        'На модерации',
        'Отклонена',
      ]);
    });

    it('drops page when status, filter or search change', async () => {
      searchParamsValue = new URLSearchParams('page=3&x=a');
      renderList();
      await loaded();

      fireEvent.change(screen.getByLabelText('Статус'), { target: { value: 'pending' } });
      expect(pushMock).toHaveBeenLastCalledWith('/catalog/things?x=a&status=pending');

      fireEvent.change(screen.getByLabelText('Икс'), { target: { value: '' } });
      expect(pushMock).toHaveBeenLastCalledWith('/catalog/things');

      fireEvent.change(screen.getByLabelText('Поиск'), { target: { value: '  лев ' } });
      fireEvent.click(screen.getByRole('button', { name: 'Найти' }));
      expect(pushMock).toHaveBeenLastCalledWith('/catalog/things?x=a&search=%D0%BB%D0%B5%D0%B2');
    });

    it('sets page on pagination', async () => {
      searchParamsValue = new URLSearchParams('status=published');
      queryFnMock.mockResolvedValue({ items: things, total: 45, page: 1, limit: 20 });
      renderList();
      await loaded();

      fireEvent.click(screen.getByRole('button', { name: 'Вперед' }));
      expect(pushMock).toHaveBeenLastCalledWith('/catalog/things?status=published&page=2');
    });

    it('re-syncs the search input when the URL changes', async () => {
      searchParamsValue = new URLSearchParams('search=old');
      const { rerender } = renderList();
      await loaded();
      expect((screen.getByLabelText('Поиск') as HTMLInputElement).value).toBe('old');

      searchParamsValue = new URLSearchParams('search=new');
      rerender(
        <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
          <Harness />
        </QueryClientProvider>
      );
      await waitFor(() => expect((screen.getByLabelText('Поиск') as HTMLInputElement).value).toBe('new'));
    });
  });

  it('edit link carries the encoded current pathname?qs as back', async () => {
    searchParamsValue = new URLSearchParams('status=published&page=2');
    renderList();
    await loaded();

    expect(within(rowOf('Альфа')).getByRole('link', { name: 'Редактировать' })).toHaveAttribute(
      'href',
      `/catalog/things/t1/edit?back=${encodeURIComponent('/catalog/things?status=published&page=2')}`
    );
  });

  describe('unpublish', () => {
    it('is offered on published rows only', async () => {
      renderList();
      await loaded();

      expect(within(rowOf('Альфа')).getByRole('button', { name: 'Снять с публикации' })).toBeInTheDocument();
      expect(within(rowOf('Бета')).queryByRole('button', { name: 'Снять с публикации' })).not.toBeInTheDocument();
      expect(within(rowOf('Гамма')).queryByRole('button', { name: 'Снять с публикации' })).not.toBeInTheDocument();
      expect(within(rowOf('Бета')).getByRole('link', { name: 'Редактировать' })).toBeInTheDocument();
    });

    it('does nothing when the confirm is cancelled', async () => {
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
      renderList();
      await loaded();

      fireEvent.click(within(rowOf('Альфа')).getByRole('button', { name: 'Снять с публикации' }));
      expect(confirmSpy).toHaveBeenCalledWith('снять Альфа?');
      expect(bulkMock).not.toHaveBeenCalled();
    });

    it('sends [id] → pending and shows the success banner; «Скрыть» hides it', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      bulkMock.mockResolvedValue({ errors: [] });
      renderList();
      await loaded();

      fireEvent.click(within(rowOf('Альфа')).getByRole('button', { name: 'Снять с публикации' }));
      await waitFor(() => expect(bulkMock).toHaveBeenCalledWith(['t1'], 'pending'));
      expect(await screen.findByText('снято Альфа')).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: 'Скрыть' }));
      expect(screen.queryByText('снято Альфа')).not.toBeInTheDocument();
    });

    it('shows a report error', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      bulkMock.mockResolvedValue({ errors: [{ message: 'not found' }] });
      renderList();
      await loaded();

      fireEvent.click(within(rowOf('Альфа')).getByRole('button', { name: 'Снять с публикации' }));
      expect(await screen.findByText('Не удалось снять с публикации «Альфа»: not found.')).toBeInTheDocument();
    });

    it('shows an ApiError message', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      bulkMock.mockRejectedValue(new ApiError(403, 'Forbidden', 'forbidden'));
      renderList();
      await loaded();

      fireEvent.click(within(rowOf('Альфа')).getByRole('button', { name: 'Снять с публикации' }));
      expect(await screen.findByText('forbidden')).toBeInTheDocument();
    });

    it('falls back to the copy text on other errors', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      bulkMock.mockRejectedValue(new Error('network'));
      renderList();
      await loaded();

      fireEvent.click(within(rowOf('Альфа')).getByRole('button', { name: 'Снять с публикации' }));
      expect(await screen.findByText('Запасная ошибка')).toBeInTheDocument();
    });
  });

  it('shows the flash once', async () => {
    setFlash('catalog.things', 'Штука «Альфа» сохранена.');
    const { unmount } = renderList();
    expect(await screen.findByText('Штука «Альфа» сохранена.')).toBeInTheDocument();
    unmount();

    renderList();
    await loaded();
    expect(screen.queryByText('Штука «Альфа» сохранена.')).not.toBeInTheDocument();
  });

  describe('states', () => {
    it('loading', () => {
      queryFnMock.mockReturnValue(new Promise(() => {}));
      renderList();
      expect(screen.getByTestId('catalog-list-loading')).toBeInTheDocument();
    });

    it('error', async () => {
      queryFnMock.mockRejectedValue(new Error('boom'));
      renderList();
      expect(await screen.findByText('Ошибка загрузки')).toBeInTheDocument();
      expect(screen.queryByText('Штуки не найдены.')).not.toBeInTheDocument();
    });

    it('empty', async () => {
      queryFnMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
      renderList();
      expect(await screen.findByText('Штуки не найдены.')).toBeInTheDocument();
    });

    it('table with the actions column', async () => {
      renderList();
      await loaded();
      expect(screen.getByRole('columnheader', { name: 'Имя' })).toBeInTheDocument();
      expect(screen.getByRole('columnheader', { name: 'Действия' })).toBeInTheDocument();
    });
  });

  it('renders renderRowActions after the built-in actions', async () => {
    renderList({
      renderRowActions: (item) => <button type="button">Экстра {item.name}</button>,
    });
    await loaded();

    const buttons = within(rowOf('Альфа'))
      .getAllByRole('button')
      .map((b) => b.textContent);
    expect(buttons).toEqual(['Снять с публикации', 'Экстра Альфа']);
    const cell = within(rowOf('Альфа')).getByRole('link', { name: 'Редактировать' }).parentElement as HTMLElement;
    expect(cell.lastElementChild).toHaveTextContent('Экстра Альфа');
  });

  it('renders headerExtra next to the title', async () => {
    renderList({ headerExtra: <a href="/moderation/things">Очередь модерации</a> });
    await loaded();
    expect(screen.getByRole('heading', { level: 1, name: 'Штуки' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Очередь модерации' })).toBeInTheDocument();
  });
});
