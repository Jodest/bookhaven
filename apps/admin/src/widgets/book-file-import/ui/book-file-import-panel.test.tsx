import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { BookFileImportPanel } from './book-file-import-panel';

const importMock = vi.fn();

vi.mock('@/entities/book', () => ({
  adminBooksApi: {
    import: (...args: unknown[]) => importMock(...args),
  },
}));

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <BookFileImportPanel />
    </QueryClientProvider>
  );
}

function makeFile(name: string, content = 'title,authors\nBook,Author\n') {
  return new File([content], name, { type: 'text/csv' });
}

describe('BookFileImportPanel', () => {
  beforeEach(() => {
    importMock.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('has no heading and no «К списку книг» link of its own', () => {
    renderView();

    expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'К списку книг' })).not.toBeInTheDocument();
  });

  it('requires a file to be selected before submitting', async () => {
    renderView();

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать' }));

    await waitFor(() =>
      expect(screen.getByText('Выберите файл .csv или .json для импорта.')).toBeInTheDocument()
    );
    expect(importMock).not.toHaveBeenCalled();
  });

  it('submits the selected file and renders the report on success', async () => {
    importMock.mockResolvedValue({
      processed: 3,
      created: 1,
      skipped_existing: 1,
      errors: [{ row: 3, title: '', message: 'title is required' }],
      created_authors: [],
      created_genres: [],
    });

    renderView();

    const file = makeFile('books.csv');
    const input = screen.getByLabelText('Файл (.csv или .json)') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [file] } });

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать' }));

    await waitFor(() => expect(importMock).toHaveBeenCalledWith(file, { status: 'published' }));

    await waitFor(() => expect(screen.getByText('Создано: 1')).toBeInTheDocument());
    expect(screen.getByText('Обработано: 3')).toBeInTheDocument();
    expect(screen.getByText('Пропущено (уже существуют): 1')).toBeInTheDocument();
    expect(screen.getByText('title is required')).toBeInTheDocument();
  });

  it('shows a message pointing at cmd/harvest or cmd/seed on a 422 row-limit error', async () => {
    importMock.mockRejectedValue(
      new ApiError(422, 'Unprocessable Entity', 'too many rows; use cmd/harvest or cmd/seed')
    );

    renderView();

    const file = makeFile('books.csv');
    fireEvent.change(screen.getByLabelText('Файл (.csv или .json)'), {
      target: { files: [file] },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Импортировать' }));

    await waitFor(() =>
      expect(screen.getByText('too many rows; use cmd/harvest or cmd/seed')).toBeInTheDocument()
    );
  });

  it('offers Опубликованы / На модерации and sends the chosen batch status', async () => {
    importMock.mockResolvedValue({
      processed: 1,
      created: 1,
      skipped_existing: 0,
      errors: [],
      created_authors: [],
      created_genres: [],
    });

    renderView();

    const select = screen.getByLabelText('Статус партии') as HTMLSelectElement;
    expect(select.value).toBe('published');
    expect(Array.from(select.options).map((o) => o.textContent)).toEqual([
      'Опубликованы',
      'На модерации',
    ]);

    const file = makeFile('books.csv');
    fireEvent.change(screen.getByLabelText('Файл (.csv или .json)') as HTMLInputElement, {
      target: { files: [file] },
    });
    fireEvent.change(select, { target: { value: 'pending' } });
    fireEvent.click(screen.getByRole('button', { name: 'Импортировать' }));

    await waitFor(() => expect(importMock).toHaveBeenCalledWith(file, { status: 'pending' }));
  });

  async function submitFile() {
    const file = makeFile('books.csv');
    fireEvent.change(screen.getByLabelText('Файл (.csv или .json)'), {
      target: { files: [file] },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Импортировать' }));
    await waitFor(() => expect(importMock).toHaveBeenCalled());
  }

  it('lists the created authors and genres with edit links, row labels and the pending note', async () => {
    importMock.mockResolvedValue({
      processed: 3,
      created: 2,
      skipped_existing: 0,
      errors: [{ row: 3, title: 'Книга', message: 'invalid isbn' }],
      created_authors: [
        { id: 'a-1', name: 'Иван Новиков', row: 1 },
        { id: 'a-2', name: 'Сирота', row: 3 },
      ],
      created_genres: [{ id: 'g-1', name: 'киберпанк', row: 2 }],
    });

    renderView();
    await submitFile();

    await waitFor(() => expect(screen.getByText('Созданные авторы (2)')).toBeInTheDocument());
    expect(screen.getByRole('link', { name: 'Иван Новиков' })).toHaveAttribute(
      'href',
      '/catalog/authors/a-1/edit'
    );
    expect(screen.getByRole('link', { name: 'Сирота' })).toHaveAttribute(
      'href',
      '/catalog/authors/a-2/edit'
    );
    expect(screen.getByText('строка 3')).toBeInTheDocument();

    expect(screen.getByText('Созданные жанры (1)')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'киберпанк' })).toHaveAttribute(
      'href',
      '/catalog/genres/g-1/edit'
    );
    expect(screen.getByText('строка 2')).toBeInTheDocument();
    expect(
      screen.getByText('Новые жанры созданы на модерации и не видны на сайте до публикации.')
    ).toBeInTheDocument();
  });

  it('shows no created lists when nothing was created (or the fields are missing)', async () => {
    importMock.mockResolvedValueOnce({
      processed: 1,
      created: 1,
      skipped_existing: 0,
      errors: [],
      created_authors: [],
      created_genres: [],
    });

    renderView();
    await submitFile();

    await waitFor(() => expect(screen.getByText('Создано: 1')).toBeInTheDocument());
    expect(screen.queryByText(/Созданные авторы/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Созданные жанры/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Новые жанры созданы на модерации/)).not.toBeInTheDocument();
  });

  it('tolerates an older backend report without the created fields', async () => {
    importMock.mockResolvedValueOnce({ processed: 1, created: 1, skipped_existing: 0, errors: [] });

    renderView();
    await submitFile();

    await waitFor(() => expect(screen.getByText('Создано: 1')).toBeInTheDocument());
    expect(screen.queryByText(/Созданные авторы/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Созданные жанры/)).not.toBeInTheDocument();
  });
});
