import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminAuthor, AdminBook, AdminGenre } from '@bookhaven/api-client';
import { ImportBooksView, type ImportBooksMethod } from './import-books-view';

const pushMock = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
}));

const createMock = vi.fn();

vi.mock('@/entities/book', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/book')>()),
  adminBooksApi: {
    create: (...args: unknown[]) => createMock(...args),
    lookup: vi.fn(),
  },
}));

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    list: () => Promise.resolve({ items: [] as AdminAuthor[], total: 0, page: 1, limit: 100 }),
  },
}));

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: {
    list: () => Promise.resolve({ items: [] as AdminGenre[], total: 0, page: 1, limit: 100 }),
  },
}));

vi.mock('@/widgets/book-harvest', () => ({ BookHarvestPanel: () => <div>harvest panel</div> }));
vi.mock('@/widgets/book-file-import', () => ({ BookFileImportPanel: () => <div>file panel</div> }));

function makeBook(overrides: Partial<AdminBook> = {}): AdminBook {
  return {
    id: 'b42',
    title: 'Новая книга',
    slug: 'novaya-kniga',
    description: null,
    cover_url: null,
    isbn: null,
    published_year: null,
    page_count: null,
    language: 'ru',
    rating: 0,
    ratings_count: 0,
    reviews_count: 0,
    status: 'published',
    external_source: null,
    authors: [],
    genres: [],
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

function renderView(method: ImportBooksMethod = 'manual') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  render(
    <QueryClientProvider client={queryClient}>
      <ImportBooksView method={method} />
    </QueryClientProvider>
  );
  return { invalidateSpy };
}

describe('ImportBooksView', () => {
  beforeEach(() => {
    pushMock.mockReset();
    createMock.mockReset();
  });

  it.each([
    ['manual', 'Импорт книг: вручную', 'Создайте книгу через форму.'],
    ['wikidata', 'Импорт книг: харвест Wikidata', 'Запустите фоновый харвест книг выбранных авторов из Wikidata.'],
    ['file', 'Импорт книг: из файла', 'Загрузите партию книг из файла .csv или .json.'],
  ] as const)('method %s renders its h1, description and only its panel', (method, title, description) => {
    renderView(method);

    expect(screen.getByRole('heading', { level: 1, name: title })).toBeInTheDocument();
    expect(screen.getByText(description)).toBeInTheDocument();
    expect(screen.queryByRole('region')).not.toBeInTheDocument();
    expect(screen.queryByText('harvest panel') !== null).toBe(method === 'wikidata');
    expect(screen.queryByText('file panel') !== null).toBe(method === 'file');
    expect(screen.queryByLabelText('Название *') !== null).toBe(method === 'manual');
  });

  it('creates a book, shows a banner with a link, resets the form and stays on the page', async () => {
    createMock.mockResolvedValue(makeBook());
    const { invalidateSpy } = renderView();

    expect(screen.queryByRole('button', { name: 'Отмена' })).not.toBeInTheDocument();
    const title = screen.getByLabelText('Название *') as HTMLInputElement;
    fireEvent.change(title, { target: { value: 'Новая книга' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => expect(createMock).toHaveBeenCalled());
    expect(createMock.mock.calls[0][0]).toMatchObject({ title: 'Новая книга', status: 'published' });

    const banner = await screen.findByRole('status');
    expect(banner).toHaveTextContent('Книга «Новая книга» создана');
    expect(screen.getByRole('link', { name: 'Открыть' })).toHaveAttribute(
      'href',
      '/catalog/books/b42/edit'
    );
    await waitFor(() => expect(title.value).toBe(''));
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'books'] });
    expect(pushMock).not.toHaveBeenCalled();
  });

  it('shows a create error inline and keeps the input', async () => {
    createMock.mockRejectedValue(new Error('boom'));
    renderView();

    const title = screen.getByLabelText('Название *') as HTMLInputElement;
    fireEvent.change(title, { target: { value: 'Книга' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() =>
      expect(screen.getByText('Не удалось сохранить книгу. Проверьте введенные данные.')).toBeInTheDocument()
    );
    expect(title.value).toBe('Книга');
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });
});
