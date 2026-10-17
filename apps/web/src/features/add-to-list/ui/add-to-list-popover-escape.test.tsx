import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { AddToListPopover } from './add-to-list-popover';

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ isAuthenticated: true }),
}));

vi.mock('@/entities/list', () => ({
  listsApi: {
    list: vi.fn().mockResolvedValue({ items: [], book_in_lists: [] }),
    create: vi.fn(),
    addBook: vi.fn(),
    removeBook: vi.fn(),
  },
}));

describe('AddToListPopover Escape handling', () => {
  it('first Escape cancels the new-list field, second closes the popover', async () => {
    render(<AddToListPopover bookId="b1" slug="book-1" />);

    fireEvent.click(screen.getByRole('button', { name: /В список/ }));
    fireEvent.click(await screen.findByRole('button', { name: /Новый список/ }));
    const input = screen.getByPlaceholderText('Название нового списка');

    fireEvent.keyDown(input, { key: 'Escape' });

    expect(screen.queryByPlaceholderText('Название нового списка')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Новый список/ })).toBeInTheDocument();

    fireEvent.keyDown(screen.getByRole('button', { name: /Новый список/ }), { key: 'Escape' });
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /Новый список/ })).not.toBeInTheDocument()
    );
  });
});
