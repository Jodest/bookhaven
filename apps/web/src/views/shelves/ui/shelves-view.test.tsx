import { fireEvent, render as rtlRender, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactElement } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Shelf } from '@bookhaven/api-client';
import { ShelvesView } from './shelves-view';

let queryClient: QueryClient;

function render(ui: ReactElement) {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return rtlRender(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

const listMock = vi.fn();
const updateMock = vi.fn();
const createMock = vi.fn();

vi.mock('@/entities/shelf', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/shelf')>()),
  shelvesApi: {
    list: () => listMock(),
    update: (...args: unknown[]) => updateMock(...args),
    create: (...args: unknown[]) => createMock(...args),
    remove: vi.fn(),
  },
}));

function shelf(overrides: Partial<Shelf>): Shelf {
  return {
    id: 's1',
    userId: 'u1',
    name: 'Полка',
    type: 'custom',
    description: '',
    isPublic: true,
    booksCount: 0,
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('ShelvesView privacy', () => {
  beforeEach(() => {
    listMock.mockReset();
    updateMock.mockReset().mockResolvedValue(undefined);
    createMock.mockReset().mockResolvedValue(undefined);
  });

  it('shows the badge and «Сделать публичной» for a private custom shelf', async () => {
    listMock.mockResolvedValue({
      items: [shelf({ id: 'c1', name: 'Тайная', isPublic: false, description: 'Для себя' })],
      total: 1,
    });
    render(<ShelvesView />);

    expect(await screen.findByText('Приватная')).toBeInTheDocument();
    expect(screen.getByText('Для себя')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Сделать публичной' })).toBeInTheDocument();
  });

  it('toggles privacy of a public custom shelf', async () => {
    listMock.mockResolvedValue({ items: [shelf({ id: 'c1', name: 'Открытая' })], total: 1 });
    render(<ShelvesView />);

    fireEvent.click(await screen.findByRole('button', { name: 'Сделать приватной' }));

    await waitFor(() => expect(updateMock).toHaveBeenCalledWith('c1', { is_public: false }));
  });

  it('shows no badge and no privacy toggle on default shelves', async () => {
    listMock.mockResolvedValue({
      items: [shelf({ id: 'd1', name: 'Читаю', type: 'reading' })],
      total: 1,
    });
    render(<ShelvesView />);

    expect(await screen.findByText('Читаю')).toBeInTheDocument();
    expect(screen.queryByText('Приватная')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Сделать/ })).not.toBeInTheDocument();
  });

  it('creates a shelf with an optional description', async () => {
    listMock.mockResolvedValue({ items: [], total: 0 });
    render(<ShelvesView />);

    fireEvent.change(await screen.findByPlaceholderText('Название новой полки'), { target: { value: 'Отпуск' } });
    fireEvent.change(screen.getByPlaceholderText('Описание (необязательно)'), { target: { value: 'Для пляжа' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() =>
      expect(createMock).toHaveBeenCalledWith({ name: 'Отпуск', description: 'Для пляжа' })
    );
  });
});
