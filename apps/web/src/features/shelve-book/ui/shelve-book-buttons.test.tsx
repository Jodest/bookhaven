import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { Shelf } from '@bookhaven/api-client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ShelveBookButtons } from './shelve-book-buttons';

let isAuthenticatedValue = false;
const listMock = vi.fn();
const addBookMock = vi.fn();

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ isAuthenticated: isAuthenticatedValue }),
}));

vi.mock('@/entities/shelf', () => ({
  shelvesApi: {
    list: (params?: unknown) => listMock(params),
    addBook: (shelfId: string, bookId: string) => addBookMock(shelfId, bookId),
    removeBook: vi.fn(),
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

const allShelves = [
  shelf('s1', 'want_to_read', 'Хочу прочитать'),
  shelf('s2', 'reading', 'Читаю'),
  shelf('s3', 'read', 'Прочитано'),
  shelf('s4', 'custom', 'Отпуск'),
];

describe('ShelveBookButtons', () => {
  afterEach(() => {
    isAuthenticatedValue = false;
    listMock.mockReset();
    addBookMock.mockReset();
  });

  it('guest defaults: three login links redirecting to the book page', () => {
    render(<ShelveBookButtons bookId="b1" slug="book-1" />);
    const links = screen.getAllByRole('link');
    expect(links.map((l) => l.textContent)).toEqual(['Хочу прочитать', 'Читаю', 'Прочитано']);
    for (const link of links) {
      expect(link).toHaveAttribute('href', '/login?redirect=/books/book-1');
    }
  });

  it('guest with types and loginRedirect: one link to /login?redirect=/', () => {
    render(<ShelveBookButtons bookId="b1" slug="book-1" types={['want_to_read']} loginRedirect="/" />);
    const links = screen.getAllByRole('link');
    expect(links).toHaveLength(1);
    expect(links[0]).toHaveTextContent('Хочу прочитать');
    expect(links[0]).toHaveAttribute('href', '/login?redirect=/');
  });

  it('logged-in defaults: all system buttons and custom shelves', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue({ items: allShelves, total: 4 });
    render(<ShelveBookButtons bookId="b1" slug="book-1" />);

    expect(await screen.findByRole('button', { name: 'Хочу прочитать' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Читаю' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Прочитано' })).toBeInTheDocument();
    expect(screen.getByLabelText('Отпуск')).toBeInTheDocument();
  });

  it('logged-in want-to-read-only mode hides other buttons and custom shelves', async () => {
    isAuthenticatedValue = true;
    listMock.mockResolvedValue({ items: allShelves, total: 4 });
    addBookMock.mockResolvedValue({ status: 'ok' });
    render(
      <ShelveBookButtons
        bookId="b1"
        slug="book-1"
        types={['want_to_read']}
        showCustomShelves={false}
        loginRedirect="/"
      />
    );

    const button = await screen.findByRole('button', { name: 'Хочу прочитать' });
    expect(screen.queryByRole('button', { name: 'Читаю' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Прочитано' })).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Отпуск')).not.toBeInTheDocument();

    fireEvent.click(button);
    await waitFor(() => expect(addBookMock).toHaveBeenCalledWith('s1', 'b1'));
  });
});
