import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type { AdminCuratedCollectionDetail, AdminCuratedCollectionItem } from '@bookhaven/api-client';
import { CollectionBooks } from './collection-books';

const moveBookMock = vi.fn();
const addBookMock = vi.fn();
const removeBookMock = vi.fn();
const listBooksMock = vi.fn();

vi.mock('@/entities/curated-collection', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/curated-collection')>()),
  adminCollectionsApi: {
    moveBook: (...a: unknown[]) => moveBookMock(...a),
    addBook: (...a: unknown[]) => addBookMock(...a),
    removeBook: (...a: unknown[]) => removeBookMock(...a),
  },
}));

vi.mock('@/features/ai-collection-draft', () => ({
  AiDraftPanel: ({ collectionId, size, booksCount }: { collectionId: string; size: number; booksCount: number }) => (
    <div data-testid="ai-draft-panel">{`${collectionId}:${size}:${booksCount}`}</div>
  ),
}));

vi.mock('@/entities/book', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/book')>()),
  adminBooksApi: { list: (...a: unknown[]) => listBooksMock(...a) },
}));

function item(
  position: number,
  state: AdminCuratedCollectionItem['state'],
  place: number | null,
  status: 'published' | 'pending' | 'rejected' = 'published',
  id = `b${position}`
): AdminCuratedCollectionItem {
  return {
    position,
    place,
    state,
    score: null,
    movement: null,
    book: { id, title: `Книга ${position}`, slug: `k${position}`, status, cover_url: null, authors: [{ id: 'a', name: 'Автор' }] },
  };
}

function detail(items: AdminCuratedCollectionItem[]): AdminCuratedCollectionDetail {
  return {
    id: 'c1',
    slug: 'top',
    title: 'Топ',
    description: '',
    kind: 'manual',
    size: 2,
    source: null,
    source_url: null,
    genre: null,
    is_published: true,
    published_at: null,
    rule: null,
    computed_at: null,
    baseline_at: null,
    created_at: '',
    updated_at: '',
    books_count: items.length,
    shown_count: 2,
    items,
  };
}

function renderWidget(d: AdminCuratedCollectionDetail) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <CollectionBooks collection={d} />
    </QueryClientProvider>
  );
}

describe('CollectionBooks', () => {
  beforeEach(() => {
    moveBookMock.mockReset().mockResolvedValue({ status: 'ok' });
    addBookMock.mockReset().mockResolvedValue({ status: 'ok' });
    removeBookMock.mockReset().mockResolvedValue({ status: 'ok' });
    listBooksMock.mockReset();
  });

  const items = [
    item(1, 'ranked', 1),
    item(2, 'hidden', null, 'pending'),
    item(3, 'ranked', 2),
    item(4, 'reserve', null),
  ];

  it('shows places, markers and the reserve divider before the first reserve item', () => {
    renderWidget(detail(items));

    expect(screen.getByText('№ 1')).toBeInTheDocument();
    expect(screen.getByText('№ 2')).toBeInTheDocument();
    expect(screen.getByText('Скрыта: не опубликована')).toBeInTheDocument();
    expect(screen.getByText('Резерв')).toBeInTheDocument();

    const divider = screen.getByTestId('reserve-divider');
    const rows = Array.from(divider.parentElement!.children);
    expect(rows.indexOf(divider)).toBe(rows.length - 2);
    expect(rows[rows.length - 1]).toHaveTextContent('Книга 4');
  });

  it('has no divider without reserve and disables edge moves', () => {
    renderWidget(detail([item(1, 'ranked', 1), item(2, 'ranked', 2)]));
    expect(screen.queryByTestId('reserve-divider')).not.toBeInTheDocument();
    const ups = screen.getAllByRole('button', { name: 'Выше' });
    const downs = screen.getAllByRole('button', { name: 'Ниже' });
    expect(ups[0]).toBeDisabled();
    expect(ups[1]).toBeEnabled();
    expect(downs[0]).toBeEnabled();
    expect(downs[1]).toBeDisabled();
  });

  it('moves a book to the typed position', async () => {
    renderWidget(detail(items));
    const row = screen.getByText('Книга 3').closest('tr')!;
    fireEvent.change(within(row).getByLabelText('Переместить на'), { target: { value: '1' } });
    fireEvent.click(within(row).getByRole('button', { name: 'Переместить' }));
    await waitFor(() => expect(moveBookMock).toHaveBeenCalledWith('c1', 'b3', 1));
  });

  it('up/down move by one and remove calls the API', async () => {
    renderWidget(detail(items));
    const row = screen.getByText('Книга 3').closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Выше' }));
    await waitFor(() => expect(moveBookMock).toHaveBeenCalledWith('c1', 'b3', 2));
    fireEvent.click(within(row).getByRole('button', { name: 'Убрать' }));
    await waitFor(() => expect(removeBookMock).toHaveBeenCalledWith('c1', 'b3'));
  });

  it('search results: Добавить disabled for present and rejected books', async () => {
    listBooksMock.mockResolvedValue({
      items: [
        { id: 'b1', title: 'Уже тут', status: 'published', authors: [{ name: 'А' }] },
        { id: 'x2', title: 'Отклонённая', status: 'rejected', authors: [] },
        { id: 'x3', title: 'Новая', status: 'pending', authors: [] },
      ],
      total: 3,
      page: 1,
      limit: 10,
    });
    renderWidget(detail(items));

    fireEvent.change(screen.getByLabelText('Найти книгу в каталоге'), { target: { value: 'книга' } });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));

    await waitFor(() => expect(screen.getByText('Новая')).toBeInTheDocument());
    expect(listBooksMock).toHaveBeenCalledWith({ search: 'книга', limit: 10 });
    const addButton = (title: string) =>
      within(screen.getByText(title).closest('li')!).getByRole('button', { name: 'Добавить' });
    expect(addButton('Уже тут')).toBeDisabled();
    expect(screen.getByText('Уже в подборке')).toBeInTheDocument();
    expect(addButton('Отклонённая')).toBeDisabled();
    expect(addButton('Новая')).toBeEnabled();

    fireEvent.click(addButton('Новая'));
    await waitFor(() => expect(addBookMock).toHaveBeenCalledWith('c1', 'x3', undefined));
  });

  it('shows the mapped error of a failed add', async () => {
    addBookMock.mockRejectedValue(new ApiError(409, 'Conflict', 'book already in collection'));
    listBooksMock.mockResolvedValue({
      items: [{ id: 'x3', title: 'Новая', status: 'pending', authors: [] }],
      total: 1,
      page: 1,
      limit: 10,
    });
    renderWidget(detail(items));
    fireEvent.change(screen.getByLabelText('Найти книгу в каталоге'), { target: { value: 'н' } });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));
    await waitFor(() => expect(screen.getByText('Новая')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Добавить' }));
    await waitFor(() => expect(screen.getByText('Книга уже в подборке')).toBeInTheDocument());
  });

  it('toggles the paste panel behind the Вставить список button', () => {
    renderWidget(detail(items));
    expect(screen.queryByLabelText('Список книг')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Вставить список' }));
    expect(screen.getByLabelText('Список книг')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Найти в каталоге' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Вставить список' }));
    expect(screen.queryByLabelText('Список книг')).not.toBeInTheDocument();
  });

  it('toggles the AI draft panel behind the Черновик ИИ button, closed for a non-empty collection', () => {
    renderWidget(detail(items));
    const toggle = screen.getByRole('button', { name: 'Черновик ИИ' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByTestId('ai-draft-panel')).not.toBeInTheDocument();

    fireEvent.click(toggle);
    expect(screen.getByTestId('ai-draft-panel')).toHaveTextContent('c1:2:4');

    fireEvent.click(toggle);
    expect(screen.queryByTestId('ai-draft-panel')).not.toBeInTheDocument();
  });

  it('opens the AI draft panel by default for an empty collection', () => {
    renderWidget(detail([]));
    expect(screen.getByRole('button', { name: 'Черновик ИИ' })).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByTestId('ai-draft-panel')).toBeInTheDocument();
  });
});
