import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AddToListButton } from './add-to-list-button';

const listMock = vi.fn();
const createMock = vi.fn();

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ isAuthenticated: true }),
}));

vi.mock('@/entities/list', () => ({
  listsApi: {
    list: (p?: unknown) => listMock(p),
    create: (p: unknown) => createMock(p),
    addBook: vi.fn(),
    removeBook: vi.fn(),
  },
}));

describe('AddToListButton create row', () => {
  afterEach(() => {
    listMock.mockReset();
    createMock.mockReset();
  });

  it('creates a list on Enter and ignores an empty title', async () => {
    listMock.mockResolvedValue({ items: [], book_in_lists: [] });
    createMock.mockResolvedValue({ id: 'l1', title: 'Новый', booksCount: 0 });
    render(<AddToListButton bookId="b1" slug="book-1" />);

    fireEvent.click(await screen.findByRole('button', { name: /Новый список/ }));
    const input = screen.getByPlaceholderText('Название нового списка');
    expect(screen.queryByRole('button', { name: 'Создать' })).not.toBeInTheDocument();

    fireEvent.submit(input.closest('form') as HTMLFormElement);
    expect(createMock).not.toHaveBeenCalled();

    fireEvent.change(input, { target: { value: 'Мои любимые' } });
    fireEvent.submit(input.closest('form') as HTMLFormElement);
    await waitFor(() => expect(createMock).toHaveBeenCalledWith({ title: 'Мои любимые', is_public: true }));
    expect(await screen.findByLabelText(/Новый/)).toBeInTheDocument();
  });

  it('Escape cancels the field and clears the input', async () => {
    listMock.mockResolvedValue({ items: [], book_in_lists: [] });
    render(<AddToListButton bookId="b1" slug="book-1" />);

    fireEvent.click(await screen.findByRole('button', { name: /Новый список/ }));
    const input = screen.getByPlaceholderText('Название нового списка');
    fireEvent.change(input, { target: { value: 'abc' } });
    fireEvent.keyDown(input, { key: 'Escape' });

    expect(screen.queryByPlaceholderText('Название нового списка')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Новый список/ }));
    expect(screen.getByPlaceholderText('Название нового списка')).toHaveValue('');
  });
});
