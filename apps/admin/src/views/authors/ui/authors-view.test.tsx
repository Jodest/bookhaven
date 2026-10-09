import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
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

vi.mock('@/features/merge-author', () => ({
  MergeAuthorDialog: ({
    duplicate,
    onMerged,
  }: {
    duplicate: AdminAuthor;
    onMerged: (booksReassigned: number) => void;
  }) => (
    <div role="dialog" aria-label={`Слияние ${duplicate.name}`}>
      <button type="button" onClick={() => onMerged(2)}>
        Подтвердить слияние
      </button>
    </div>
  ),
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
    expect(pushMock).toHaveBeenCalledWith('/catalog/authors?status=rejected&page=1');
  });

  it('has no create form: the form appears only on «Редактировать»', async () => {
    listMock.mockResolvedValue({ items: [makeAuthor()], total: 1, page: 1, limit: 20 });

    renderView();

    await screen.findByRole('button', { name: 'Редактировать' });
    expect(screen.queryByRole('button', { name: 'Создать' })).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Имя *')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Редактировать' }));
    expect(screen.getByRole('button', { name: 'Сохранить' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));
    expect(screen.queryByLabelText('Имя *')).not.toBeInTheDocument();
    expect(createMock).not.toHaveBeenCalled();
  });

  it('sends status on edit, hides the form after save and shows «Автор обновлен»', async () => {
    listMock.mockResolvedValue({ items: [makeAuthor({ status: 'pending' })], total: 1, page: 1, limit: 20 });
    updateMock.mockResolvedValue(makeAuthor());

    renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Редактировать' }));
    const statusSelect = screen.getByLabelText('Статус', { selector: '#status' }) as HTMLSelectElement;
    expect(statusSelect.value).toBe('pending');
    fireEvent.change(statusSelect, { target: { value: 'published' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() =>
      expect(updateMock).toHaveBeenCalledWith('a1', expect.objectContaining({ status: 'published' }))
    );
    await waitFor(() => expect(screen.getByText('Автор обновлен')).toBeInTheDocument());
    expect(screen.queryByLabelText('Имя *')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Сохранить' })).not.toBeInTheDocument();
  });

  it('shows an update error inline in the edit form', async () => {
    listMock.mockResolvedValue({ items: [makeAuthor()], total: 1, page: 1, limit: 20 });
    updateMock.mockRejectedValue(new Error('slug already exists'));

    renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Редактировать' }));
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => expect(screen.getByText('slug already exists')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: 'Сохранить' })).toBeInTheDocument();
  });

  it('still opens the merge dialog and shows the merge result', async () => {
    listMock.mockResolvedValue({ items: [makeAuthor()], total: 1, page: 1, limit: 20 });

    renderView();

    fireEvent.click(await screen.findByRole('button', { name: 'Слить' }));
    expect(screen.getByRole('dialog', { name: 'Слияние Лев Толстой' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить слияние' }));

    expect(
      screen.getByText('Автор «Лев Толстой» слит с каноническим автором. Перенесено книг: 2.')
    ).toBeInTheDocument();
  });

  it('has no "Очередь модерации" link in the header', async () => {
    listMock.mockResolvedValue({ items: [makeAuthor()], total: 1, page: 1, limit: 20 });

    renderView();

    await waitFor(() => expect(listMock).toHaveBeenCalled());
    expect(screen.queryByRole('link', { name: 'Очередь модерации' })).not.toBeInTheDocument();
  });
});
