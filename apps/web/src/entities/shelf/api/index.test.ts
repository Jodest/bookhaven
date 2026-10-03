import { beforeEach, describe, expect, it, vi } from 'vitest';

import { shelvesApi } from './index';

// These tests exercise the real @bookhaven/api-client fetch/normalize path
// (no mocking of the client itself) by stubbing global fetch, to guard
// against the snake_case/camelCase mismatch regression where
// api.shelves.list/get/create returned the raw backend shape unnormalized.
function jsonResponse(body: unknown): Response {
  return {
    ok: true,
    status: 200,
    statusText: 'OK',
    json: async () => body,
  } as Response;
}

describe('shelvesApi normalization', () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
  });

  it('normalizes list() items from snake_case to camelCase', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        items: [
          {
            id: 'shelf-1',
            name: 'Reading',
            type: 'reading',
            books_count: 3,
            has_book: true,
            created_at: '2024-01-01T00:00:00Z',
          },
        ],
        total: 1,
      })
    );
    vi.stubGlobal('fetch', fetchMock);

    const result = await shelvesApi.list();

    expect(result.items[0]).toEqual({
      id: 'shelf-1',
      userId: '',
      name: 'Reading',
      type: 'reading',
      booksCount: 3,
      hasBook: true,
      createdAt: '2024-01-01T00:00:00Z',
      updatedAt: '2024-01-01T00:00:00Z',
    });
  });

  it('normalizes get() shelf (and nested books) from snake_case to camelCase', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        shelf: {
          id: 'shelf-1',
          user_id: 'user-1',
          name: 'Reading',
          type: 'reading',
          books_count: 1,
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-02T00:00:00Z',
        },
        items: [],
        total: 0,
        page: 1,
        limit: 20,
      })
    );
    vi.stubGlobal('fetch', fetchMock);

    const result = await shelvesApi.get('shelf-1');

    expect(result.shelf).toEqual({
      id: 'shelf-1',
      userId: 'user-1',
      name: 'Reading',
      type: 'reading',
      booksCount: 1,
      hasBook: undefined,
      createdAt: '2024-01-01T00:00:00Z',
      updatedAt: '2024-01-02T00:00:00Z',
    });
  });

  it('normalizes getPublic() shelf (and nested books) from snake_case to camelCase', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        shelf: {
          id: 'shelf-1',
          user_id: 'user-1',
          name: 'Reading',
          type: 'reading',
          books_count: 1,
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-02T00:00:00Z',
        },
        items: [],
        total: 0,
        page: 1,
        limit: 20,
      })
    );
    vi.stubGlobal('fetch', fetchMock);

    const result = await shelvesApi.getPublic('shelf-1');

    expect(result.shelf).toEqual({
      id: 'shelf-1',
      userId: 'user-1',
      name: 'Reading',
      type: 'reading',
      booksCount: 1,
      hasBook: undefined,
      createdAt: '2024-01-01T00:00:00Z',
      updatedAt: '2024-01-02T00:00:00Z',
    });
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/api/v1/shelves/public/shelf-1'),
      expect.anything()
    );
  });

  it('normalizes create() response from snake_case to camelCase', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        id: 'shelf-2',
        user_id: 'user-1',
        name: 'Favorites',
        type: 'custom',
        books_count: 0,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      })
    );
    vi.stubGlobal('fetch', fetchMock);

    const result = await shelvesApi.create({ name: 'Favorites' });

    expect(result).toEqual({
      id: 'shelf-2',
      userId: 'user-1',
      name: 'Favorites',
      type: 'custom',
      booksCount: 0,
      hasBook: undefined,
      createdAt: '2024-01-01T00:00:00Z',
      updatedAt: '2024-01-01T00:00:00Z',
    });
  });
});
