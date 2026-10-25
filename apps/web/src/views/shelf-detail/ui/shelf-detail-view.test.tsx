import { fireEvent, render as rtlRender, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ImgHTMLAttributes, ReactElement } from 'react';
import { homeKeys } from '@/entities/home';
import { readingChallengeKeys } from '@/entities/reading-challenge';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Shelf } from '@bookhaven/api-client';
import { ShelfDetailView } from './shelf-detail-view';

let queryClient: QueryClient;

function render(ui: ReactElement) {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return rtlRender(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

const getMock = vi.fn();
const removeBookMock = vi.fn();
const updateMock = vi.fn();

vi.mock('@/entities/shelf', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/shelf')>()),
  shelvesApi: {
    get: (...args: unknown[]) => getMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
    remove: vi.fn(),
    removeBook: (...args: unknown[]) => removeBookMock(...args),
  },
}));

vi.mock('next/navigation', () => ({ useRouter: () => ({ push: vi.fn() }) }));

vi.mock('next/image', () => ({
  default: ({ fill: _fill, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean }) => (
    // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
    <img {...props} />
  ),
}));

function shelf(overrides: Partial<Shelf>): Shelf {
  return {
    id: 's1',
    userId: 'u1',
    name: 'Отпуск',
    type: 'custom',
    description: 'Для пляжа',
    isPublic: true,
    booksCount: 0,
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

function respond(s: Shelf) {
  getMock.mockResolvedValue({ shelf: s, items: [], total: 0, page: 1, limit: 12 });
}

describe('ShelfDetailView', () => {
  beforeEach(() => {
    getMock.mockReset();
    removeBookMock.mockReset().mockResolvedValue(undefined);
    updateMock.mockReset().mockResolvedValue(undefined);
  });

  it('shows the description and edits name and description together', async () => {
    respond(shelf({}));
    render(<ShelfDetailView id="s1" />);

    expect(await screen.findByText('Для пляжа')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Редактировать' }));

    const description = screen.getByLabelText('Описание');
    fireEvent.change(description, { target: { value: 'Новое описание' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() =>
      expect(updateMock).toHaveBeenCalledWith('s1', {
        name: 'Отпуск',
        description: 'Новое описание',
      })
    );
  });

  it('toggles privacy and shows the badge on a private shelf', async () => {
    respond(shelf({ isPublic: false }));
    render(<ShelfDetailView id="s1" />);

    expect(await screen.findByText('Приватная')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Сделать публичной' }));

    await waitFor(() => expect(updateMock).toHaveBeenCalledWith('s1', { is_public: true }));
  });

  it('shows no controls on the default «Прочитано» shelf', async () => {
    respond(shelf({ name: 'Прочитано', type: 'read', description: '' }));
    render(<ShelfDetailView id="s1" />);

    expect(await screen.findByText('Прочитано')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Редактировать' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Сделать/ })).not.toBeInTheDocument();
    expect(screen.queryByText('Приватная')).not.toBeInTheDocument();
  });

  it('"Убрать с полки" removes the book and invalidates challenge and home caches', async () => {
    const book = { id: 'b1', slug: 'b-1', title: 'Книга', authors: [], coverUrl: '' };
    getMock.mockResolvedValue({ shelf: shelf({ name: 'Прочитано', type: 'read' }), items: [book], total: 1, page: 1, limit: 12 });
    const challengeKey = readingChallengeKeys.detail(2026, undefined);
    const homeKey = homeKeys.me(undefined);
    render(<ShelfDetailView id="s1" />);
    queryClient.setQueryData(challengeKey, { booksRead: 3 });
    queryClient.setQueryData(homeKey, {});

    fireEvent.click(await screen.findByRole('button', { name: 'Убрать с полки' }));

    await waitFor(() => expect(removeBookMock).toHaveBeenCalledWith('s1', 'b1'));
    await waitFor(() => expect(queryClient.getQueryState(challengeKey)?.isInvalidated).toBe(true));
    expect(queryClient.getQueryState(homeKey)?.isInvalidated).toBe(true);
  });

  it('pagination "Вперёд" requests page 2', async () => {
    const book = { id: 'b1', slug: 'b-1', title: 'Книга', authors: [], coverUrl: '' };
    getMock.mockResolvedValue({ shelf: shelf({}), items: [book], total: 30, page: 1, limit: 12 });
    render(<ShelfDetailView id="s1" />);

    fireEvent.click(await screen.findByRole('button', { name: 'Вперёд' }));

    await waitFor(() => expect(getMock).toHaveBeenCalledWith('s1', { page: 2, limit: 12 }));
  });

  it('shows "Прочитано N раз" only under books finished at least once', async () => {
    const book = (id: string, title: string, readsCount: number) => ({
      id,
      slug: id,
      title,
      coverUrl: null,
      authors: [],
      genres: [],
      readsCount,
      lastReadAt: readsCount > 0 ? '2025-05-15T12:00:00Z' : null,
    });
    getMock.mockResolvedValue({
      shelf: shelf({ type: 'read', name: 'Прочитано' }),
      items: [book('b1', 'Дюна', 2), book('b2', 'Эмма', 0)],
      total: 2,
      page: 1,
      limit: 12,
    });
    render(<ShelfDetailView id="s1" />);

    expect(await screen.findByText('Прочитано 2 раза')).toBeInTheDocument();
    expect(screen.getAllByText(/Прочитано \d/)).toHaveLength(1);
  });
});
