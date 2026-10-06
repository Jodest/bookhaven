import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminAuthor } from '@bookhaven/api-client';
import { AuthorsView } from './authors-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/authors',
  useSearchParams: () => searchParamsValue,
}));

const listMock = vi.fn();
const createMock = vi.fn();
const updateMock = vi.fn();

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    list: (...args: unknown[]) => listMock(...args),
    create: (...args: unknown[]) => createMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
  },
}));

vi.mock('@/features/merge-author', () => ({ MergeAuthorDialog: () => null }));

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
    ...overrides,
  };
}

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthorsView />
    </QueryClientProvider>
  );
}

describe('AuthorsView status', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    pushMock.mockReset();
    listMock.mockReset();
    createMock.mockReset();
    updateMock.mockReset();
  });

  it('shows a status badge per author', async () => {
    listMock.mockResolvedValue({
      items: [
        makeAuthor(),
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
  });

  it('reads the status filter from the URL and writes it back on change', async () => {
    searchParamsValue = new URLSearchParams('status=pending');
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });

    renderView();

    await waitFor(() => expect(listMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'pending' })));
    const filter = screen.getByLabelText('Статус') as HTMLSelectElement;
    expect(filter.value).toBe('pending');

    fireEvent.change(filter, { target: { value: 'rejected' } });
    expect(pushMock).toHaveBeenCalledWith('/authors?status=rejected&page=1');
  });

  it('sends status on edit only, never on create', async () => {
    listMock.mockResolvedValue({ items: [makeAuthor({ status: 'pending' })], total: 1, page: 1, limit: 20 });
    createMock.mockResolvedValue(makeAuthor());
    updateMock.mockResolvedValue(makeAuthor());

    renderView();

    // Create form: no status select.
    expect(screen.queryByLabelText('Статус', { selector: '#status' })).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Имя *'), { target: { value: 'Новый автор' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
    await waitFor(() => expect(createMock).toHaveBeenCalled());
    expect(createMock.mock.calls[0][0]).not.toHaveProperty('status');

    // Edit form: status select pre-filled and sent.
    fireEvent.click(await screen.findByRole('button', { name: 'Редактировать' }));
    const statusSelect = screen.getByLabelText('Статус', { selector: '#status' }) as HTMLSelectElement;
    expect(statusSelect.value).toBe('pending');
    fireEvent.change(statusSelect, { target: { value: 'published' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() =>
      expect(updateMock).toHaveBeenCalledWith('a1', expect.objectContaining({ status: 'published' }))
    );
  });

  it('links to the author moderation queue from the header', async () => {
    listMock.mockResolvedValue({ items: [makeAuthor()], total: 1, page: 1, limit: 20 });

    renderView();

    const link = screen.getByRole('link', { name: 'Очередь модерации' });
    expect(link).toHaveAttribute('href', '/authors/moderation');
    await waitFor(() => expect(listMock).toHaveBeenCalled());
  });
});
