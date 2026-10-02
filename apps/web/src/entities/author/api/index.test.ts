import { afterEach, describe, expect, it, vi } from 'vitest';
import { authorsApi } from './index';

describe('authorsApi.get', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('normalizes the snake_case author-detail response to camelCase', async () => {
    const rawResponse = {
      id: 'author-1',
      name: 'Ursula K. Le Guin',
      slug: 'ursula-k-le-guin',
      biography: 'An American author.',
      photo_url: 'https://example.com/photo.jpg',
      birth_date: '1929-10-21',
      death_date: '2018-01-22',
      books: [
        {
          id: 'book-1',
          title: 'The Left Hand of Darkness',
          slug: 'the-left-hand-of-darkness',
          cover_url: 'https://example.com/cover.jpg',
          published_year: 1969,
          rating: 4.5,
          ratings_count: 120,
          authors: [
            {
              id: 'author-1',
              name: 'Ursula K. Le Guin',
              slug: 'ursula-k-le-guin',
              photo_url: 'https://example.com/photo.jpg',
              birth_date: '1929-10-21',
            },
          ],
          genres: [{ id: 'genre-1', name: 'Science Fiction', slug: 'sci-fi' }],
        },
      ],
    };

    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => rawResponse,
    });
    vi.stubGlobal('fetch', fetchMock);

    const author = await authorsApi.get('ursula-k-le-guin');

    expect(author.photoUrl).toBe('https://example.com/photo.jpg');
    expect(author.birthDate).toBe('1929-10-21');
    expect(author.deathDate).toBe('2018-01-22');

    expect(author.books).toHaveLength(1);
    const book = author.books?.[0];
    expect(book?.coverUrl).toBe('https://example.com/cover.jpg');
    expect(book?.publishedYear).toBe(1969);
    expect(book?.authors[0].photoUrl).toBe('https://example.com/photo.jpg');
  });
});
