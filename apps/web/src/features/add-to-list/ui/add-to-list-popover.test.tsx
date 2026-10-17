import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AddToListPopover } from './add-to-list-popover';

let isAuthenticatedValue = false;

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ isAuthenticated: isAuthenticatedValue }),
}));

vi.mock('./add-to-list-button', () => ({
  AddToListButton: () => <div>add-to-list-content</div>,
}));

describe('AddToListPopover', () => {
  afterEach(() => {
    isAuthenticatedValue = false;
  });

  it('guest gets an encoded login link and no popover', () => {
    render(<AddToListPopover bookId="b1" slug="book-1" />);
    expect(screen.getByRole('link', { name: 'Добавить в список' })).toHaveAttribute(
      'href',
      '/login?redirect=%2Fbooks%2Fbook-1'
    );
    expect(screen.queryByRole('button', { name: /В список/ })).not.toBeInTheDocument();
  });

  it('signed-in reader opens AddToListButton from «В список»', async () => {
    isAuthenticatedValue = true;
    render(<AddToListPopover bookId="b1" slug="book-1" />);
    expect(screen.queryByText('add-to-list-content')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /В список/ }));
    expect(await screen.findByText('add-to-list-content')).toBeInTheDocument();
  });
});
