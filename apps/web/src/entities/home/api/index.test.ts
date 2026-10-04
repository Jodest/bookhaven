import { beforeEach, describe, expect, it, vi } from 'vitest';

import { homeApi } from './index';

// Exercises the real @bookhaven/api-client fetch/normalize path by stubbing
// global fetch, guarding against returning snake_case data typed as
// camelCase (see #129/#131).
function jsonResponse(body: unknown): Response {
  return {
    ok: true,
    status: 200,
    statusText: 'OK',
    json: async () => body,
  } as Response;
}

const rawBook = {
  id: 'b1',
  title: 'Герой нашего времени',
  slug: 'geroy',
  cover_url: 'https://example.com/c.jpg',
  published_year: 1840,
  rating: 4.6,
  ratings_count: 12,
  authors: [
    { id: 'a1', name: 'Михаил Лермонтов', slug: 'mikhail-lermontov', photo_url: 'https://example.com/a.jpg', birth_date: '1814-10-15' },
  ],
  genres: [{ id: 'g1', name: 'Классика', slug: 'classics' }],
};

describe('homeApi normalization', () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
  });

  it('normalizes get() including nested calendar authors and review user/book', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        date: '2026-10-04',
        book_of_the_day: { book: rawBook, description_excerpt: 'Роман…' },
        calendar: {
          date: '2026-10-15',
          is_today: false,
          events: [
            {
              type: 'birth',
              years: 212,
              author: {
                id: 'a1',
                name: 'Михаил Лермонтов',
                slug: 'mikhail-lermontov',
                photo_url: 'https://example.com/a.jpg',
                birth_date: '1814-10-15',
                death_date: '1841-07-27',
              },
              books: [rawBook],
            },
          ],
          anniversaries: [{ years: 100, book: rawBook }],
        },
        new_books: [rawBook],
        top_rated: [],
        recent_reviews: [
          {
            id: 'r1',
            excerpt: 'Отличная книга',
            rating: 5,
            likes_count: 2,
            created_at: '2026-10-03T10:00:00Z',
            user: { id: 'u1', name: 'Анна', avatar_url: null },
            book: {
              id: 'b1',
              slug: 'geroy',
              title: 'Герой нашего времени',
              cover_url: 'https://example.com/c.jpg',
              authors: [{ name: 'Михаил Лермонтов', slug: 'mikhail-lermontov' }],
            },
          },
        ],
      })
    );
    vi.stubGlobal('fetch', fetchMock);

    const home = await homeApi.get();

    expect(fetchMock.mock.calls[0][0]).toContain('/api/v1/home');
    expect(home.date).toBe('2026-10-04');
    expect(home.bookOfTheDay?.descriptionExcerpt).toBe('Роман…');
    expect(home.bookOfTheDay?.book).toMatchObject({
      coverUrl: 'https://example.com/c.jpg',
      publishedYear: 1840,
      ratingsCount: 12,
    });
    expect(home.calendar).toMatchObject({ date: '2026-10-15', isToday: false });
    expect(home.calendar?.events[0].author).toEqual({
      id: 'a1',
      name: 'Михаил Лермонтов',
      slug: 'mikhail-lermontov',
      photoUrl: 'https://example.com/a.jpg',
      birthDate: '1814-10-15',
      deathDate: '1841-07-27',
    });
    expect(home.calendar?.events[0].books[0].ratingsCount).toBe(12);
    expect(home.calendar?.anniversaries[0]).toMatchObject({ years: 100, book: { slug: 'geroy' } });
    expect(home.newBooks[0].coverUrl).toBe('https://example.com/c.jpg');
    expect(home.topRated).toEqual([]);
    expect(home.recentReviews[0]).toEqual({
      id: 'r1',
      excerpt: 'Отличная книга',
      rating: 5,
      likesCount: 2,
      createdAt: '2026-10-03T10:00:00Z',
      user: { id: 'u1', name: 'Анна', avatarUrl: null },
      book: {
        id: 'b1',
        slug: 'geroy',
        title: 'Герой нашего времени',
        coverUrl: 'https://example.com/c.jpg',
        authors: [{ name: 'Михаил Лермонтов', slug: 'mikhail-lermontov' }],
      },
    });
  });

  it('keeps null blocks null', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse({
          date: '2026-10-04',
          book_of_the_day: null,
          calendar: null,
          new_books: [],
          top_rated: [],
          recent_reviews: [],
        })
      )
    );
    const home = await homeApi.get();
    expect(home.bookOfTheDay).toBeNull();
    expect(home.calendar).toBeNull();
  });

  it('random() sends genre/exclude and normalizes the book or null', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ book: rawBook }))
      .mockResolvedValueOnce(jsonResponse({ book: null }));
    vi.stubGlobal('fetch', fetchMock);

    const first = await homeApi.random({ genreId: 'g1', excludeId: 'b0' });
    expect(fetchMock.mock.calls[0][0]).toContain('/api/v1/home/random?genre_id=g1&exclude_id=b0');
    expect(first.book).toMatchObject({ id: 'b1', coverUrl: 'https://example.com/c.jpg', ratingsCount: 12 });

    const second = await homeApi.random();
    expect(fetchMock.mock.calls[1][0]).toMatch(/\/api\/v1\/home\/random$/);
    expect(second.book).toBeNull();
  });
});
