import type { Book, Author, Review, User } from '@bookhaven/types';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

async function fetcher<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    credentials: 'include',
  });

  if (!response.ok) {
    throw new Error(`API error: ${response.status} ${response.statusText}`);
  }

  return response.json() as Promise<T>;
}

export const api = {
  books: {
    list: (params?: { page?: number; limit?: number; search?: string }) =>
      fetcher<{ items: Book[]; total: number }>(`/api/v1/books?${new URLSearchParams(params as Record<string, string>)}`),
    get: (slug: string) => fetcher<Book>(`/api/v1/books/${slug}`),
  },
  authors: {
    get: (slug: string) => fetcher<Author>(`/api/v1/authors/${slug}`),
  },
  reviews: {
    list: (bookId: string) => fetcher<Review[]>(`/api/v1/books/${bookId}/reviews`),
  },
  auth: {
    me: () => fetcher<User | null>('/api/v1/auth/me'),
  },
};

export type { Book, Author, Review, User } from '@bookhaven/types';
