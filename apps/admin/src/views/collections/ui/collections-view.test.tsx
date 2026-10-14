import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { CollectionsView } from './collections-view';

const listMock = vi.fn();

vi.mock('@/entities/curated-collection', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/curated-collection')>()),
  adminCollectionsApi: { list: (...a: unknown[]) => listMock(...a) },
}));

function renderView() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <CollectionsView />
    </QueryClientProvider>
  );
}

describe('CollectionsView', () => {
  beforeEach(() => listMock.mockReset());

  it('lists collections with status, shown count and a warning when short', async () => {
    listMock.mockResolvedValue({
      items: [
        { id: 'c1', slug: 'a', title: 'Топ-100', kind: 'manual', size: 100, is_published: true, books_count: 102, shown_count: 98, updated_at: '' },
        { id: 'c2', slug: 'b', title: 'Новая идея', kind: 'manual', size: 10, is_published: false, books_count: 12, shown_count: 10, updated_at: '' },
      ],
      total: 2,
    });
    renderView();

    await waitFor(() => expect(screen.getByText('Топ-100')).toBeInTheDocument());
    expect(screen.getByText('Опубликована')).toBeInTheDocument();
    expect(screen.getByText('Черновик')).toBeInTheDocument();
    expect(screen.getByText(/98 из 100/)).toBeInTheDocument();
    expect(screen.getAllByTestId('reserve-warning')).toHaveLength(1);
    expect(screen.getByLabelText('В подборке видно 98 из 100 книг — пополните резерв')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Новая подборка' })).toHaveAttribute('href', '/collections/new');
    expect(screen.getAllByRole('link', { name: 'Редактировать' })[0]).toHaveAttribute('href', '/collections/c1');
  });

  it('shows the empty state', async () => {
    listMock.mockResolvedValue({ items: [], total: 0 });
    renderView();
    await waitFor(() => expect(screen.getByText('Подборок пока нет.')).toBeInTheDocument());
  });
});
