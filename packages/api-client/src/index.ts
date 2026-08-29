export interface User {
  id: string;
  email: string;
  name: string;
  avatarUrl: string | null;
  role: 'user' | 'contributor' | 'moderator' | 'admin';
  profileType: 'reader' | 'librarian' | 'bookseller' | 'other';
}

export interface Book {
  id: string;
  title: string;
  slug: string;
  description: string | null;
  coverUrl: string | null;
  isbn: string | null;
  publishedYear: number | null;
  pageCount: number | null;
  language: string;
  rating: number;
  ratingsCount: number;
  authors: Author[];
  genres: Genre[];
  createdAt: string;
  updatedAt: string;
}

export interface Author {
  id: string;
  name: string;
  slug: string;
  biography: string | null;
  photoUrl: string | null;
  birthDate: string | null;
  deathDate: string | null;
  createdAt: string;
  updatedAt: string;
  books?: Book[];
}

export interface Genre {
  id: string;
  name: string;
  slug: string;
  createdAt: string;
}

export interface Review {
  id: string;
  bookId: string;
  userId: string;
  user: Pick<User, 'id' | 'name' | 'avatarUrl'>;
  rating: number;
  content: string;
  likesCount: number;
  isHidden: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface Shelf {
  id: string;
  userId: string;
  name: string;
  type: 'reading' | 'read' | 'want_to_read' | 'abandoned' | 'favorite' | 'custom';
  booksCount: number;
  hasBook?: boolean;
  books?: Book[];
  createdAt: string;
  updatedAt: string;
}

export interface BookList {
  id: string;
  userId: string;
  user: Pick<User, 'id' | 'name' | 'avatarUrl'>;
  title: string;
  description: string | null;
  isPublic: boolean;
  books: Book[];
  createdAt: string;
  updatedAt: string;
}

const API_URL =
  typeof window === 'undefined'
    ? process.env.API_URL || 'http://backend:8080'
    : process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

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

export type PaginatedResponse<T> = {
  items: T[];
  total: number;
  page: number;
  limit: number;
};

export interface LoginInput {
  email: string;
  password: string;
}

export interface RegisterInput {
  email: string;
  password: string;
  name: string;
}

export interface AuthResponse {
  user: User;
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

export const api = {
  books: {
    list: (params?: { page?: number; limit?: number; search?: string; genre_id?: string; author_id?: string }) => {
      const queryParams: Record<string, string> = {};
      if (params) {
        if (params.page !== undefined) queryParams.page = String(params.page);
        if (params.limit !== undefined) queryParams.limit = String(params.limit);
        if (params.search) queryParams.search = params.search;
        if (params.genre_id) queryParams.genre_id = params.genre_id;
        if (params.author_id) queryParams.author_id = params.author_id;
      }
      return fetcher<PaginatedResponse<Book>>(`/api/v1/books?${new URLSearchParams(queryParams)}`);
    },
    get: (slug: string) => fetcher<Book>(`/api/v1/books/${slug}`),
  },
  authors: {
    get: (slug: string) => fetcher<Author>(`/api/v1/authors/${slug}`),
  },
  genres: {
    list: () => fetcher<{ items: Genre[]; total: number }>('/api/v1/genres'),
  },
  reviews: {
    list: (bookId: string) => fetcher<Review[]>(`/api/v1/books/${bookId}/reviews`),
  },
  auth: {
    me: async () => {
      try {
        return await fetcher<User>('/api/v1/auth/me');
      } catch {
        return null;
      }
    },
    login: (data: LoginInput) =>
      fetcher<AuthResponse>('/api/v1/auth/login', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    register: (data: RegisterInput) =>
      fetcher<AuthResponse>('/api/v1/auth/register', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    logout: () =>
      fetcher<{ status: string }>('/api/v1/auth/logout', {
        method: 'POST',
      }),
    refresh: () =>
      fetcher<AuthResponse>('/api/v1/auth/refresh', {
        method: 'POST',
      }),
  },
  shelves: {
    list: (params?: { book_id?: string }) => {
      const queryParams: Record<string, string> = {};
      if (params?.book_id) {
        queryParams.book_id = params.book_id;
      }
      const query = new URLSearchParams(queryParams).toString();
      return fetcher<{ items: Shelf[]; total: number }>(
        `/api/v1/shelves${query ? `?${query}` : ''}`
      );
    },
    get: (
      id: string,
      params?: { page?: number; limit?: number }
    ) => {
      const queryParams: Record<string, string> = {};
      if (params?.page !== undefined) queryParams.page = String(params.page);
      if (params?.limit !== undefined) queryParams.limit = String(params.limit);
      const query = new URLSearchParams(queryParams).toString();
      return fetcher<{
        shelf: Shelf;
        items: Book[];
        total: number;
        page: number;
        limit: number;
      }>(`/api/v1/shelves/${id}${query ? `?${query}` : ''}`);
    },
    create: (data: { name: string }) =>
      fetcher<Shelf>('/api/v1/shelves', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    rename: (id: string, data: { name: string }) =>
      fetcher<Shelf>(`/api/v1/shelves/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    remove: (id: string) =>
      fetcher<{ status: string }>(`/api/v1/shelves/${id}`, {
        method: 'DELETE',
      }),
    addBook: (id: string, bookId: string) =>
      fetcher<{ status: string }>(`/api/v1/shelves/${id}/books`, {
        method: 'POST',
        body: JSON.stringify({ book_id: bookId }),
      }),
    removeBook: (id: string, bookId: string) =>
      fetcher<{ status: string }>(`/api/v1/shelves/${id}/books/${bookId}`, {
        method: 'DELETE',
      }),
  },
};
