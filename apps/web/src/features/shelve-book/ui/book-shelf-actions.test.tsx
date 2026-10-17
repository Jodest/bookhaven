import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { Shelf } from '@bookhaven/api-client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { BookShelfActions } from './book-shelf-actions';

let isAuthenticatedValue = false;
const listMock = vi.fn();
const addBookMock = vi.fn();
const removeBookMock = vi.fn();

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ isAuthenticated: isAuthenticatedValue }),
}));

vi.mock('@/entities/shelf', () => ({
  shelvesApi: {
    list: (params?: unknown) => listMock(params),
    addBook: (shelfId: string, bookId: string) => addBookMock(shelfId, bookId),
    removeBook: (shelfId: string, bookId: string) => removeBookMock(shelfId, bookId),
  },
}));

function shelf(id: string, type: Shelf['type'], name: string, hasBook = false): Shelf {
  return {
    id,
    userId: 'u1',
    name,
    type,
    booksCount: 0,
    hasBook,
    createdAt: '',
    updatedAt: '',
  } as Shelf;
}

function shelves(opts: { active?: string; custom?: boolean } = {}) {
  const items = [
    shelf('s1', 'want_to_read', 'Хочу прочитать', opts.active === 's1'),
    shelf('s2', 'reading', 'Читаю', opts.active === 's2'),
    shelf('s3', 'read', 'Прочитано', opts.active === 's3'),
  ];
  if (opts.custom !== false) items.push(shelf('s4', 'custom', 'Отпуск'));
  return { items, total: items.length };
}

describe('BookShelfActions', () => {
  afterEach(() => {
    isAuthenticatedValue = false;
    listMock.mockReset();
    addBookMock.mockReset();
    removeBookMock.mockReset();
  });

  it('adds to want_to_read from the primary button when there is no status', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue(shelves());
    addBookMock.mockResolvedValue({});
    render(<BookShelfActions bookId="b1" slug="book-1" />);

    fireEvent.click(await screen.findByRole('button', { name: 'Хочу прочитать' }));
    await waitFor(() => expect(addBookMock).toHaveBeenCalledWith('s1', 'b1'));
    expect(await screen.findByRole('button', { name: 'Статус: Хочу прочитать. Изменить' })).toBeInTheDocument();
  });

  it('picks another status from the chooser and closes it', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue(shelves());
    addBookMock.mockResolvedValue({});
    render(<BookShelfActions bookId="b1" slug="book-1" />);

    fireEvent.click(await screen.findByRole('button', { name: 'Выбрать статус' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Читаю' }));

    await waitFor(() => expect(addBookMock).toHaveBeenCalledWith('s2', 'b1'));
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Прочитано' })).not.toBeInTheDocument()
    );
  });

  it('removes the status with "Убрать статус"', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue(shelves({ active: 's2' }));
    removeBookMock.mockResolvedValue({});
    render(<BookShelfActions bookId="b1" slug="book-1" />);

    fireEvent.click(await screen.findByRole('button', { name: 'Статус: Читаю. Изменить' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Убрать статус' }));

    await waitFor(() => expect(removeBookMock).toHaveBeenCalledWith('s2', 'b1'));
  });

  it('picking the current status removes it', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue(shelves({ active: 's2' }));
    removeBookMock.mockResolvedValue({});
    render(<BookShelfActions bookId="b1" slug="book-1" />);

    fireEvent.click(await screen.findByRole('button', { name: 'Статус: Читаю. Изменить' }));
    const current = await screen.findByRole('button', { name: 'Читаю', pressed: true });
    fireEvent.click(current);

    await waitFor(() => expect(removeBookMock).toHaveBeenCalledWith('s2', 'b1'));
  });

  it('"На полку" opens the checklist and stays open after a toggle', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue(shelves());
    addBookMock.mockResolvedValue({});
    render(<BookShelfActions bookId="b1" slug="book-1" />);

    fireEvent.click(await screen.findByRole('button', { name: 'На полку' }));
    fireEvent.click(await screen.findByLabelText('Отпуск'));

    await waitFor(() => expect(addBookMock).toHaveBeenCalledWith('s4', 'b1'));
    expect(screen.getByLabelText('Отпуск')).toBeChecked();
    expect(screen.getByText('Добавить на полку')).toBeInTheDocument();
  });

  it('has no "На полку" without custom shelves', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue(shelves({ custom: false }));
    render(<BookShelfActions bookId="b1" slug="book-1" />);

    await screen.findByRole('button', { name: 'Выбрать статус' });
    expect(screen.queryByRole('button', { name: /На полку/ })).not.toBeInTheDocument();
  });

  it('rolls back and shows an error when the update fails', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue(shelves());
    addBookMock.mockRejectedValue(new Error('x'));
    render(<BookShelfActions bookId="b1" slug="book-1" />);

    fireEvent.click(await screen.findByRole('button', { name: 'Хочу прочитать' }));

    expect(await screen.findByText('x')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Выбрать статус' })).toBeInTheDocument();
  });

  it('shows the default error text for a non-Error rejection', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue(shelves());
    addBookMock.mockRejectedValue('boom');
    render(<BookShelfActions bookId="b1" slug="book-1" />);

    fireEvent.click(await screen.findByRole('button', { name: 'Хочу прочитать' }));
    expect(await screen.findByText('Ошибка обновления полки')).toBeInTheDocument();
  });

  it('shows a skeleton while loading', () => {
    isAuthenticatedValue = true;
    listMock.mockReturnValue(new Promise(() => {}));
    const { container } = render(<BookShelfActions bookId="b1" slug="book-1" />);
    expect(container.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('guest sees one encoded login link and no "На полку"', () => {
    render(<BookShelfActions bookId="b1" slug="book-1" />);
    const links = screen.getAllByRole('link');
    expect(links).toHaveLength(1);
    expect(links[0]).toHaveAttribute('href', '/login?redirect=%2Fbooks%2Fbook-1');
    expect(screen.queryByText(/На полку/)).not.toBeInTheDocument();
  });

  it('keeps the dropdown arrow visible when a status is set', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue(shelves({ active: 's2' }));
    render(<BookShelfActions bookId="b1" slug="book-1" />);

    const trigger = await screen.findByRole('button', { name: 'Статус: Читаю. Изменить' });
    expect(trigger.querySelector('svg.lucide-chevron-down')).not.toBeNull();
  });

  it('shows the current status icon on the trigger and a check on the selected item', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue(shelves({ active: 's2' }));
    render(<BookShelfActions bookId="b1" slug="book-1" />);

    const trigger = await screen.findByRole('button', { name: 'Статус: Читаю. Изменить' });
    expect(trigger.querySelector('svg.lucide-book-open')).not.toBeNull();

    fireEvent.click(trigger);
    const selected = await screen.findByRole('button', { name: 'Читаю', pressed: true });
    expect(selected.querySelector('svg.lucide-check')).not.toBeNull();
    const other = screen.getByRole('button', { name: 'Прочитано' });
    expect(other.querySelector('svg.lucide-check')).toBeNull();
  });
});
