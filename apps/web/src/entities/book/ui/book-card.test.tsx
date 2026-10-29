import type { Book } from '@bookhaven/api-client';
import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { BookCard } from './book-card';

const FANTLAB = 'https://fantlab.ru/images/editions/big/16469';

function book(coverUrl: string): Book {
  return {
    id: 'b1',
    slug: 'khobbit',
    title: 'Хоббит',
    authors: [],
    genres: [],
    coverUrl,
  } as unknown as Book;
}

describe('BookCard cover', () => {
  it('hotlinks a FantLab cover: the src is the link itself, not the optimizer', () => {
    const { container } = render(<BookCard book={book(FANTLAB)} />);
    expect(container.querySelector('img')?.getAttribute('src')).toBe(FANTLAB);
  });

  it('sends other covers through the optimizer', () => {
    const { container } = render(<BookCard book={book('https://covers.openlibrary.org/b/id/1-L.jpg')} />);
    expect(container.querySelector('img')?.getAttribute('src')).toContain('/_next/image');
  });
});
