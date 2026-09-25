import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { BookImportView } from './book-import-view';

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
      <BookImportView />
    </QueryClientProvider>
  );
}

function makeFile(name: string, content = 'title,authors\nBook,Author\n') {
  return new File([content], name, { type: 'text/csv' });
}

describe('BookImportView', () => {
  beforeEach(() => {
    importMock.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
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
    });

    renderView();

    const file = makeFile('books.csv');
    const input = screen.getByLabelText('Файл (.csv или .json)') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [file] } });

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать' }));

    await waitFor(() => expect(importMock).toHaveBeenCalledWith(file, { isHidden: false }));

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
});
