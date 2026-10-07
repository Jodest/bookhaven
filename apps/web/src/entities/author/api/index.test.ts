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

  it('normalizes photo_attribution to camelCase (null when absent)', async () => {
    const raw = {
      id: 'a1',
      name: 'Лев Толстой',
      slug: 'lev-tolstoy',
      biography: null,
      photo_url: 'https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/T.jpg/500px-T.jpg',
      birth_date: null,
      death_date: null,
      photo_attribution: {
        author: null,
        license: 'Public domain',
        license_url: null,
        source_url: 'https://commons.wikimedia.org/wiki/File:T.jpg',
      },
      books: [],
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => raw }));

    const author = await authorsApi.get('lev-tolstoy');
    expect(author.photoAttribution).toEqual({
      author: null,
      license: 'Public domain',
      licenseUrl: null,
      sourceUrl: 'https://commons.wikimedia.org/wiki/File:T.jpg',
    });
    expect(author).not.toHaveProperty('photo_attribution');

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ...raw, photo_attribution: null }) }));
    expect((await authorsApi.get('lev-tolstoy')).photoAttribution).toBeNull();
  });
});
