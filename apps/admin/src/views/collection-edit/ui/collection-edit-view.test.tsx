import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminCuratedCollectionDetail } from '@bookhaven/api-client';
import { CollectionEditView } from './collection-edit-view';

const pushMock = vi.fn();
let paramsValue: Record<string, string> = { id: 'c1' };

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  useParams: () => paramsValue,
}));

const getMock = vi.fn();
const createMock = vi.fn();
const updateMock = vi.fn();
const removeMock = vi.fn();

vi.mock('@/entities/curated-collection', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/curated-collection')>()),
  adminCollectionsApi: {
    get: (...a: unknown[]) => getMock(...a),
    create: (...a: unknown[]) => createMock(...a),
    update: (...a: unknown[]) => updateMock(...a),
    remove: (...a: unknown[]) => removeMock(...a),
  },
}));

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: { list: vi.fn().mockResolvedValue({ items: [{ id: 'g1', name: 'Фантастика' }], total: 1 }) },
}));

// The book panel has its own tests.
vi.mock('@/widgets/collection-books', () => ({
  CollectionBooks: () => <div data-testid="books-panel" />,
}));

function detail(overrides: Partial<AdminCuratedCollectionDetail> = {}): AdminCuratedCollectionDetail {
  return {
    id: 'c1',
    slug: 'top',
    title: 'Топ-100',
    description: '',
    kind: 'manual',
    size: 100,
    source: null,
    source_url: null,
    genre: null,
    is_published: true,
    published_at: null,
    created_at: '',
    updated_at: '',
    books_count: 100,
    shown_count: 100,
    items: [],
    ...overrides,
  };
}

function renderView(mode?: 'new') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <CollectionEditView mode={mode} />
    </QueryClientProvider>
  );
}

describe('CollectionEditView', () => {
  beforeEach(() => {
    pushMock.mockReset();
    getMock.mockReset();
    createMock.mockReset();
    updateMock.mockReset();
    removeMock.mockReset();
    paramsValue = { id: 'c1' };
  });

  it('new mode creates and redirects to the edit page', async () => {
    createMock.mockResolvedValue(detail({ id: 'new-id' }));
    renderView('new');

    expect(screen.queryByTestId('books-panel')).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Название'), { target: { value: 'Топ 100 фантастики' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/collections/new-id'));
    expect(createMock).toHaveBeenCalledWith({ title: 'Топ 100 фантастики', size: 100, is_published: false });
  });

  it('shows the slug conflict under the slug field', async () => {
    const { ApiError } = await import('@bookhaven/api-client');
    createMock.mockRejectedValue(new ApiError(409, 'Conflict', 'slug already exists'));
    renderView('new');
    fireEvent.change(screen.getByLabelText('Название'), { target: { value: 'Топ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
    await waitFor(() => expect(screen.getByText('Такой адрес уже занят')).toBeInTheDocument());
  });

  it('warns when fewer books are shown than places', async () => {
    getMock.mockResolvedValue(detail({ shown_count: 98, books_count: 101 }));
    renderView();
    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent('В подборке видно 98 из 100 книг — пополните резерв')
    );
    expect(screen.getByTestId('books-panel')).toBeInTheDocument();
  });

  it('has no warning when the collection is full', async () => {
    getMock.mockResolvedValue(detail());
    renderView();
    await waitFor(() => expect(screen.getByTestId('books-panel')).toBeInTheDocument());
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('saves with cleared fields', async () => {
    getMock.mockResolvedValue(detail());
    updateMock.mockResolvedValue(detail());
    renderView();
    await waitFor(() => expect(screen.getByLabelText('Название')).toHaveValue('Топ-100'));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    await waitFor(() =>
      expect(updateMock).toHaveBeenCalledWith('c1', expect.objectContaining({ genre_id: null, source: '', title: 'Топ-100' }))
    );
    await waitFor(() => expect(screen.getByText('Подборка сохранена.')).toBeInTheDocument());
  });

  it('delete asks for confirmation first', async () => {
    getMock.mockResolvedValue(detail());
    removeMock.mockResolvedValue({ status: 'ok' });
    renderView();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Удалить подборку' })).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Удалить подборку' }));
    expect(removeMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Да, удалить' }));
    await waitFor(() => expect(removeMock).toHaveBeenCalledWith('c1'));
    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/collections'));
  });

  it('shows not found', async () => {
    const { ApiError } = await import('@bookhaven/api-client');
    getMock.mockRejectedValue(new ApiError(404, 'Not Found', 'collection not found'));
    renderView();
    await waitFor(() => expect(screen.getByText('Подборка не найдена.')).toBeInTheDocument());
  });
});
