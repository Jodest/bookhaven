import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminAuthor } from '@bookhaven/api-client';
import { AuthorsView } from './authors-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/catalog/authors',
  useSearchParams: () => searchParamsValue,
}));

const listMock = vi.fn();
const updateMock = vi.fn();
const bulkStatusMock = vi.fn();
const enrichMock = vi.fn();

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    list: (...args: unknown[]) => listMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
    bulkStatus: (...args: unknown[]) => bulkStatusMock(...args),
    enrichWikidata: (...args: unknown[]) => enrichMock(...args),
  },
}));

function makeAuthor(overrides: Partial<AdminAuthor> = {}): AdminAuthor {
  return {
    id: 'a1',
    name: 'Лев Толстой',
    slug: 'lev-tolstoy',
    biography: null,
    photo_url: null,
    birth_date: null,
    death_date: null,
    status: 'published',
    external_source: null,
    external_id: null,
    photo_attribution: null,
    auto_harvest: false,
    last_auto_harvested_at: null,
    ...overrides,
  };
}

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <AuthorsView />
    </QueryClientProvider>
  );
  return { invalidateSpy, ...utils };
}

function rowOf(name: string) {
  return screen.getByText(name, { selector: 'span.font-medium' }).closest('tr') as HTMLElement;
}

describe('AuthorsView (catalog list)', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    listMock.mockReset();
    updateMock.mockReset();
    bulkStatusMock.mockReset();
    enrichMock.mockReset();
    window.sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows status badges and a linked QID', async () => {
    listMock.mockResolvedValue({
      items: [
        makeAuthor({ external_source: 'wikidata', external_id: 'Q7243' }),
        makeAuthor({ id: 'a2', name: 'Ожидающий', slug: 'p', status: 'pending' }),
        makeAuthor({ id: 'a3', name: 'Отклонённый', slug: 'r', status: 'rejected' }),
      ],
      total: 3,
      page: 1,
      limit: 20,
    });

    renderView();

    await waitFor(() => expect(screen.getByText('Ожидающий')).toBeInTheDocument());
    expect(screen.getByText('Опубликован', { selector: 'span' })).toBeInTheDocument();
    expect(screen.getByText('На модерации', { selector: 'span' })).toBeInTheDocument();
    expect(screen.getByText('Отклонён', { selector: 'span' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Q7243' })).toHaveAttribute('href', 'https://www.wikidata.org/wiki/Q7243');
  });

  it('builds list params from the URL (search, status, page) and writes the status back', async () => {
    searchParamsValue = new URLSearchParams('status=pending&search=Лев&page=2');
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });

    renderView();

    await waitFor(() =>
      expect(listMock).toHaveBeenCalledWith({ search: 'Лев', status: 'pending', page: 2, limit: 20 })
    );
    const filter = screen.getByLabelText('Статус') as HTMLSelectElement;
    expect(filter.value).toBe('pending');

    fireEvent.change(filter, { target: { value: 'rejected' } });
    expect(pushMock).toHaveBeenCalledWith(
      `/catalog/authors?status=rejected&search=${encodeURIComponent('Лев')}`
    );
  });

  it('links «Редактировать» to the edit page with back; no inline form and no «Слить»', async () => {
    searchParamsValue = new URLSearchParams('status=published');
    listMock.mockResolvedValue({ items: [makeAuthor()], total: 1, page: 1, limit: 20 });

    renderView();

    const edit = await screen.findByRole('link', { name: 'Редактировать' });
    expect(edit).toHaveAttribute(
      'href',
      `/catalog/authors/a1/edit?back=${encodeURIComponent('/catalog/authors?status=published')}`
    );
    fireEvent.click(edit);
    expect(screen.queryByLabelText('Имя *')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Сохранить' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Слить' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Создать' })).not.toBeInTheDocument();
  });

  it('unpublishes a published author → pending and invalidates authors + dashboard', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    listMock.mockResolvedValue({ items: [makeAuthor()], total: 1, page: 1, limit: 20 });
    bulkStatusMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });

    const { invalidateSpy } = renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Снять с публикации' }));
    expect(window.confirm).toHaveBeenCalledWith(
      'Снять автора «Лев Толстой» с публикации? Он вернётся в очередь модерации.'
    );
    await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['a1'], 'pending'));
    expect(
      await screen.findByText('Автор «Лев Толстой» снят с публикации и возвращён на модерацию.')
    ).toBeInTheDocument();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'authors'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
  });

  it('offers «Обогатить из Wikidata» only with a QID and calls enrichWikidata([id])', async () => {
    listMock.mockResolvedValue({
      items: [
        makeAuthor({ external_source: 'wikidata', external_id: 'Q7243' }),
        makeAuthor({ id: 'a2', name: 'Без QID', slug: 'no-qid' }),
      ],
      total: 2,
      page: 1,
      limit: 20,
    });
    enrichMock.mockResolvedValue({
      processed: 1,
      enriched: 0,
      items: [{ author_id: 'a1', name: 'Лев Толстой', outcome: 'unchanged', filled: [], notes: [], message: '' }],
    });

    renderView();

    await waitFor(() => expect(screen.getByText('Без QID')).toBeInTheDocument());
    expect(within(rowOf('Без QID')).queryByRole('button', { name: 'Обогатить из Wikidata' })).not.toBeInTheDocument();
    fireEvent.click(within(rowOf('Лев Толстой')).getByRole('button', { name: 'Обогатить из Wikidata' }));
    await waitFor(() => expect(enrichMock).toHaveBeenCalledWith(['a1']));
    expect(await screen.findByText(/Автор «Лев Толстой»: нечего добавить/)).toBeInTheDocument();
  });

  it('links to the author moderation queue from the header', async () => {
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    renderView();
    expect(screen.getByRole('link', { name: 'Очередь модерации' })).toHaveAttribute('href', '/moderation/authors');
  });
});
