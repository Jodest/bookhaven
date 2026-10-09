import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminAuthor, AdminAuthorQueueItem } from '@bookhaven/api-client';
import { AuthorModerationView } from './author-moderation-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/moderation/authors',
  useSearchParams: () => searchParamsValue,
}));

const queueMock = vi.fn();
const bulkStatusMock = vi.fn();
const enrichMock = vi.fn();

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    moderationQueue: (...args: unknown[]) => queueMock(...args),
    bulkStatus: (...args: unknown[]) => bulkStatusMock(...args),
    enrichWikidata: (...args: unknown[]) => enrichMock(...args),
  },
}));

vi.mock('@/features/merge-author', () => ({
  MergeAuthorDialog: (props: {
    duplicate: AdminAuthor;
    initialCanonical?: Pick<AdminAuthor, 'id' | 'name'>;
  }) => (
    <div data-testid="merge-dialog">
      {props.duplicate.name}|{props.initialCanonical?.name ?? ''}
    </div>
  ),
}));

function makeAuthor(overrides: Partial<AdminAuthor> = {}): AdminAuthor {
  return {
    id: 'a1',
    name: 'Л. Н. Толстой',
    slug: 'l-n-tolstoy',
    biography: null,
    photo_url: null,
    birth_date: null,
    death_date: null,
    status: 'pending',
    external_source: 'wikidata',
    external_id: 'Q7243',
    photo_attribution: null,
    ...overrides,
  };
}

function makeItem(overrides: Partial<AdminAuthorQueueItem> = {}): AdminAuthorQueueItem {
  return {
    author: makeAuthor(),
    created_at: '2026-10-06T10:00:00Z',
    books_total: 3,
    books_visible: 1,
    sample_books: [
      { id: 'b1', title: 'Война и мир', status: 'published' },
      { id: 'b2', title: 'Анна Каренина', status: 'pending' },
      { id: 'b3', title: 'Воскресение', status: 'rejected' },
    ],
    similar: [
      {
        author: makeAuthor({
          id: 'h1',
          name: 'Лев Толстой',
          slug: 'lev-tolstoy',
          status: 'published',
          external_id: 'Q1',
        }),
        match: 'surname_initial',
      },
    ],
    ...overrides,
  };
}

const secondItem = makeItem({
  author: makeAuthor({ id: 'a2', name: 'Гомер', slug: 'gomer', external_source: null, external_id: null }),
  books_total: 0,
  books_visible: 0,
  sample_books: [],
  similar: [],
});

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <AuthorModerationView />
    </QueryClientProvider>
  );
  return { invalidateSpy, ...utils };
}

function rowOf(name: string) {
  return screen.getByText(name, { selector: 'span.font-medium' }).closest('tr') as HTMLElement;
}

describe('AuthorModerationView', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    queueMock.mockReset();
    bulkStatusMock.mockReset();
    enrichMock.mockReset();
    queueMock.mockResolvedValue({ items: [makeItem(), secondItem], total: 2, page: 1, limit: 20 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('defaults to the pending queue and renders QID link, books summary and hints', async () => {
    renderView();

    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());
    expect(queueMock).toHaveBeenCalledWith(
      expect.objectContaining({ status: 'pending', page: 1, limit: 20 })
    );

    const row = rowOf('Л. Н. Толстой');
    const qidLink = within(row).getByRole('link', { name: 'Q7243' });
    expect(qidLink).toHaveAttribute('href', 'https://www.wikidata.org/wiki/Q7243');
    expect(qidLink).toHaveAttribute('target', '_blank');
    expect(qidLink).toHaveAttribute('rel', 'noopener noreferrer');

    expect(within(row).getByText('3 (видимых 1)')).toBeInTheDocument();
    expect(within(row).getByRole('link', { name: 'Война и мир' })).toHaveAttribute(
      'href',
      '/catalog/books/b1/edit'
    );
    expect(within(row).getByText('(на модерации)')).toBeInTheDocument();
    expect(within(row).getByText('(отклонена)')).toBeInTheDocument();
    expect(within(row).getAllByText(/^\((на модерации|отклонена)\)$/)).toHaveLength(2);

    expect(within(row).getByText('Лев Толстой')).toBeInTheDocument();
    expect(within(row).getByText('(фамилия и инициал)')).toBeInTheDocument();
    expect(within(row).getByText('разные QID — вероятно, тёзки')).toBeInTheDocument();

    const second = rowOf('Гомер');
    expect(within(second).getByText('0 (видимых 0)')).toBeInTheDocument();
  });

  it('shows status-dependent per-row actions', async () => {
    queueMock.mockResolvedValue({
      items: [
        makeItem({ similar: [] }),
        makeItem({ author: makeAuthor({ id: 'r1', name: 'Отклонённый', status: 'rejected' }), similar: [] }),
        makeItem({ author: makeAuthor({ id: 'p1', name: 'Опубликованный', status: 'published' }), similar: [] }),
      ],
      total: 3,
      page: 1,
      limit: 20,
    });
    renderView();
    await waitFor(() => expect(screen.getByText('Отклонённый')).toBeInTheDocument());

    const pendingRow = rowOf('Л. Н. Толстой');
    expect(within(pendingRow).getByRole('button', { name: 'Опубликовать' })).toBeInTheDocument();
    expect(within(pendingRow).getByRole('button', { name: 'Отклонить' })).toBeInTheDocument();
    expect(within(pendingRow).getByRole('button', { name: 'Слить' })).toBeInTheDocument();

    const rejectedRow = rowOf('Отклонённый');
    expect(within(rejectedRow).getByRole('button', { name: 'Вернуть на модерацию' })).toBeInTheDocument();
    expect(within(rejectedRow).getByRole('button', { name: 'Опубликовать' })).toBeInTheDocument();
    expect(within(rejectedRow).queryByRole('button', { name: 'Отклонить' })).not.toBeInTheDocument();
    expect(within(rejectedRow).queryByRole('button', { name: 'Слить' })).not.toBeInTheDocument();

    const publishedRow = rowOf('Опубликованный');
    expect(within(publishedRow).queryByRole('button', { name: 'Опубликовать' })).not.toBeInTheDocument();
    expect(within(publishedRow).getByRole('button', { name: 'Отклонить' })).toBeInTheDocument();
    expect(within(publishedRow).getByRole('button', { name: 'Слить' })).toBeInTheDocument();
  });

  it('publishes a single author after confirmation and invalidates authors + dashboard', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });
    const { invalidateSpy } = renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    fireEvent.click(within(rowOf('Л. Н. Толстой')).getByRole('button', { name: 'Опубликовать' }));

    expect(confirmSpy).toHaveBeenCalledWith('Опубликовать автора «Л. Н. Толстой»?');
    await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['a1'], 'published'));
    await waitFor(() =>
      expect(screen.getByText('Автор «Л. Н. Толстой» опубликован.')).toBeInTheDocument()
    );
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'authors'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
  });

  it('reject confirm mentions visible books, and cancelling does nothing', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    fireEvent.click(within(rowOf('Л. Н. Толстой')).getByRole('button', { name: 'Отклонить' }));

    const text = confirmSpy.mock.calls[0][0] as string;
    expect(text).toContain('«Л. Н. Толстой»');
    expect(text).toContain('опубликованных книг (1)');
    expect(bulkStatusMock).not.toHaveBeenCalled();

    confirmSpy.mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });
    fireEvent.click(within(rowOf('Гомер')).getByRole('button', { name: 'Отклонить' }));
    expect(confirmSpy.mock.calls[1][0]).not.toContain('опубликованных книг');
    await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['a2'], 'rejected'));
  });

  it('bulk-publishes the selected rows and reports "Обновлено X из N" with errors', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({
      processed: 2,
      updated: 1,
      errors: [{ author_id: 'a2', name: '', message: 'author not found' }],
    });
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    expect(screen.queryByRole('button', { name: /Опубликовать выбранные/ })).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Выбрать все на странице'));
    expect(screen.getByLabelText('Выбрать «Л. Н. Толстой»')).toBeChecked();
    expect(screen.getByLabelText('Выбрать «Гомер»')).toBeChecked();

    fireEvent.click(screen.getByRole('button', { name: 'Опубликовать выбранные (2)' }));

    await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['a1', 'a2'], 'published'));
    await waitFor(() =>
      expect(
        screen.getByText('Обновлено 1 из 2. Ошибок: 1 («Гомер»: author not found).')
      ).toBeInTheDocument()
    );
    expect(screen.getByLabelText('Выбрать «Л. Н. Толстой»')).not.toBeChecked();
    expect(screen.queryByRole('button', { name: /Опубликовать выбранные/ })).not.toBeInTheDocument();
  });

  it('offers "Вернуть на модерацию (N)" on the rejected tab', async () => {
    searchParamsValue = new URLSearchParams('status=rejected');
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    bulkStatusMock.mockResolvedValue({ processed: 1, updated: 1, errors: [] });
    queueMock.mockResolvedValue({
      items: [makeItem({ author: makeAuthor({ status: 'rejected' }), similar: [] })],
      total: 1,
      page: 1,
      limit: 20,
    });
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());
    expect(queueMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'rejected' }));

    fireEvent.click(screen.getByLabelText('Выбрать «Л. Н. Толстой»'));
    fireEvent.click(screen.getByRole('button', { name: 'Вернуть на модерацию (1)' }));

    await waitFor(() => expect(bulkStatusMock).toHaveBeenCalledWith(['a1'], 'pending'));
    await waitFor(() => expect(screen.getByText('Обновлено 1 из 1.')).toBeInTheDocument());
  });

  it('clears the selection and updates URL params on filter change', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText('Выбрать «Гомер»'));
    expect(screen.getByRole('button', { name: 'Отклонить выбранные (1)' })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('QID'), { target: { value: 'without' } });
    expect(pushMock).toHaveBeenCalledWith('/moderation/authors?qid=without');
    expect(screen.getByLabelText('Выбрать «Гомер»')).not.toBeChecked();
    expect(screen.queryByRole('button', { name: /выбранные/ })).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Книги'), { target: { value: 'none' } });
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/authors?books=none');

    fireEvent.click(screen.getByRole('button', { name: 'Отклонённые' }));
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/authors?status=rejected');

    fireEvent.change(screen.getByLabelText('Поиск'), { target: { value: 'Толстой' } });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));
    expect(pushMock).toHaveBeenLastCalledWith(
      `/moderation/authors?search=${encodeURIComponent('Толстой')}`
    );
  });

  it('opens the merge dialog plain from "Слить" and prefilled from a hint', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    fireEvent.click(within(rowOf('Л. Н. Толстой')).getByRole('button', { name: 'Это он — слить' }));
    expect(screen.getByTestId('merge-dialog')).toHaveTextContent('Л. Н. Толстой|Лев Толстой');
  });

  it('opens the merge dialog without a canonical from the row action', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    fireEvent.click(within(rowOf('Гомер')).getByRole('button', { name: 'Слить' }));
    expect(screen.getByTestId('merge-dialog').textContent).toBe('Гомер|');
  });

  it('shows the pending empty state', async () => {
    queueMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    renderView();
    await waitFor(() => expect(screen.getByText('Нет авторов на модерации')).toBeInTheDocument());
  });

  it('shows the generic empty state with filters', async () => {
    searchParamsValue = new URLSearchParams('status=published&books=none');
    queueMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    renderView();
    await waitFor(() => expect(screen.getByText('Авторы не найдены')).toBeInTheDocument());
    expect(queueMock).toHaveBeenCalledWith(
      expect.objectContaining({ status: 'published', books: 'none' })
    );
  });
  it('has no "Импорт из Wikidata" button and no import panel (moved to /import/authors)', async () => {
    renderView();
    await waitFor(() => expect(screen.getByText('Война и мир')).toBeInTheDocument());

    expect(screen.queryByRole('button', { name: 'Импорт из Wikidata' })).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Имя автора в Wikidata')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Все авторы' })).not.toBeInTheDocument();
  });

  it('offers "Обогатить из Wikidata" only on rows with a QID and shows the result', async () => {
    enrichMock.mockResolvedValue({
      processed: 1,
      enriched: 1,
      items: [
        {
          author_id: 'a1',
          name: 'Л. Н. Толстой',
          outcome: 'enriched',
          filled: ['birth_date', 'photo'],
          notes: ['death_date_unusable'],
          message: '',
        },
      ],
    });
    const { invalidateSpy } = renderView();
    await waitFor(() => expect(rowOf('Л. Н. Толстой')).toBeInTheDocument());

    expect(within(rowOf('Гомер')).queryByRole('button', { name: 'Обогатить из Wikidata' })).not.toBeInTheDocument();
    fireEvent.click(within(rowOf('Л. Н. Толстой')).getByRole('button', { name: 'Обогатить из Wikidata' }));

    await waitFor(() =>
      expect(
        screen.getByText(
          'Автор «Л. Н. Толстой»: добавлены дата рождения, фото. Примечания: дата смерти в Wikidata неточная или противоречивая — пропущена.'
        )
      ).toBeInTheDocument()
    );
    expect(enrichMock).toHaveBeenCalledWith(['a1']);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'authors'] });
  });

  it('bulk-enriches only selected rows with a QID', async () => {
    enrichMock.mockResolvedValue({
      processed: 1,
      enriched: 0,
      items: [
        { author_id: 'a1', name: 'Л. Н. Толстой', outcome: 'unchanged', filled: [], notes: [], message: '' },
      ],
    });
    renderView();
    await waitFor(() => expect(rowOf('Л. Н. Толстой')).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText('Выбрать «Гомер»'));
    expect(screen.getByRole('button', { name: 'Обогатить из Wikidata (0)' })).toBeDisabled();

    fireEvent.click(screen.getByLabelText('Выбрать «Л. Н. Толстой»'));
    fireEvent.click(screen.getByRole('button', { name: 'Обогатить из Wikidata (1)' }));

    await waitFor(() =>
      expect(screen.getByText('Обогащено 0 из 1. Без QID пропущено: 1.')).toBeInTheDocument()
    );
    expect(enrichMock).toHaveBeenCalledWith(['a1']);
  });

  it('shows the 502 message when Wikidata is unavailable', async () => {
    const { ApiError } = await import('@bookhaven/api-client');
    enrichMock.mockRejectedValue(new ApiError(502, 'Bad Gateway', 'wikidata is unavailable; retry later'));
    renderView();
    await waitFor(() => expect(rowOf('Л. Н. Толстой')).toBeInTheDocument());

    fireEvent.click(within(rowOf('Л. Н. Толстой')).getByRole('button', { name: 'Обогатить из Wikidata' }));
    await waitFor(() => expect(screen.getByText('Wikidata недоступна, попробуйте позже.')).toBeInTheDocument());
  });
});
