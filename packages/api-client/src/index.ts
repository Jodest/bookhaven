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
  reviewsCount: number;
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
  book_id: string;
  user_id: string;
  user: { id: string; name: string | null; avatar_url: string | null };
  rating: number;
  content: string;
  likes_count: number;
  liked_by_me?: boolean;
  created_at: string;
  updated_at: string;
}

export interface ReviewsListResponse {
  items: Review[];
  total: number;
  page: number;
  limit: number;
  my_review?: Review;
}

export interface MyRating {
  value: number;
}

export interface SetRatingResponse {
  book_id: string;
  value: number;
  rating: number;
  ratings_count: number;
}

export interface ToggleLikeResponse {
  likes_count: number;
  liked: boolean;
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

export interface List {
  id: string;
  userId: string;
  title: string;
  description: string;
  isPublic: boolean;
  booksCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface ListDetail {
  list: List;
  items: Book[];
  total: number;
  page: number;
  limit: number;
}

export interface Language {
  code: string;
  name: string;
}

const API_URL =
  typeof window === 'undefined'
    ? process.env.API_URL || 'http://backend:8080'
    : '';

export class ApiError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    message?: string
  ) {
    super(message || `API error: ${status} ${statusText}`);
    this.name = 'ApiError';
  }
}

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
    let message: string | undefined;
    try {
      const body = await response.json();
      if (body && typeof body.error === 'string') {
        message = body.error;
      }
    } catch {
      // ignore
    }
    throw new ApiError(response.status, response.statusText, message);
  }

  return response.json() as Promise<T>;
}

export type PaginatedResponse<T> = {
  items: T[];
  total: number;
  page: number;
  limit: number;
};

// Backend returns snake_case for catalog entities; normalize to the camelCase
// Book/Author/Genre types consumed by the frontend.
type RawBook = Book & {
  cover_url: string | null;
  published_year: number | null;
  page_count: number | null;
  ratings_count: number;
  reviews_count: number;
  authors: RawAuthor[];
  genres: RawGenre[];
  created_at: string;
  updated_at: string;
};

type RawAuthor = Author & {
  photo_url: string | null;
  birth_date: string | null;
  death_date: string | null;
  created_at: string;
  updated_at: string;
};

type RawGenre = Genre & {
  created_at: string;
};

type RawList = List & {
  user_id: string;
  is_public: boolean;
  books_count: number;
  created_at: string;
  updated_at?: string;
};

function normalizeAuthor(raw: RawAuthor): Author {
  return {
    ...raw,
    photoUrl: raw.photo_url,
    birthDate: raw.birth_date,
    deathDate: raw.death_date,
    createdAt: raw.created_at,
    updatedAt: raw.updated_at,
  };
}

function normalizeGenre(raw: RawGenre): Genre {
  return {
    ...raw,
    createdAt: raw.created_at,
  };
}

function normalizeBook(raw: RawBook): Book {
  return {
    ...raw,
    coverUrl: raw.cover_url,
    publishedYear: raw.published_year,
    pageCount: raw.page_count,
    ratingsCount: raw.ratings_count,
    reviewsCount: raw.reviews_count,
    authors: raw.authors.map(normalizeAuthor),
    genres: raw.genres.map(normalizeGenre),
    createdAt: raw.created_at,
    updatedAt: raw.updated_at,
  };
}

function normalizeList(raw: RawList): List {
  return {
    ...raw,
    userId: raw.user_id,
    isPublic: raw.is_public,
    booksCount: raw.books_count,
    createdAt: raw.created_at,
    updatedAt: raw.updated_at ?? raw.created_at,
  };
}

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
    list: async (params?: {
      page?: number;
      limit?: number;
      search?: string;
      genre_id?: string[];
      author_id?: string;
      rating_min?: number;
      language?: string;
      year_from?: number;
      year_to?: number;
      sort?: string;
    }) => {
      const queryParams: Record<string, string> = {};
      const genreSearchParams = new URLSearchParams();
      if (params) {
        if (params.page !== undefined) queryParams.page = String(params.page);
        if (params.limit !== undefined) queryParams.limit = String(params.limit);
        if (params.search) queryParams.search = params.search;
        if (params.genre_id) {
          for (const id of params.genre_id) {
            genreSearchParams.append('genre_id', id);
          }
        }
        if (params.author_id) queryParams.author_id = params.author_id;
        if (params.rating_min !== undefined) queryParams.rating_min = String(params.rating_min);
        if (params.language) queryParams.language = params.language;
        if (params.year_from !== undefined) queryParams.year_from = String(params.year_from);
        if (params.year_to !== undefined) queryParams.year_to = String(params.year_to);
        if (params.sort) queryParams.sort = params.sort;
      }
      // Merge scalar params + genre array params
      const merged = new URLSearchParams(queryParams);
      genreSearchParams.forEach((v, k) => merged.append(k, v));
      const raw = await fetcher<{ items: RawBook[]; total: number; page: number; limit: number }>(
        `/api/v1/books?${merged.toString()}`
      );
      return {
        ...raw,
        items: raw.items.map(normalizeBook),
      };
    },
    get: async (slug: string) => {
      const raw = await fetcher<RawBook>(`/api/v1/books/${slug}`);
      return normalizeBook(raw);
    },
  },
  authors: {
    list: (params?: { search?: string; limit?: number }) => {
      const queryParams: Record<string, string> = {};
      if (params) {
        if (params.search) queryParams.search = params.search;
        if (params.limit !== undefined) queryParams.limit = String(params.limit);
      }
      return fetcher<PaginatedResponse<Author>>(
        `/api/v1/authors?${new URLSearchParams(queryParams)}`
      );
    },
    get: (slug: string) => fetcher<Author>(`/api/v1/authors/${slug}`),
  },
  genres: {
    list: () => fetcher<{ items: Genre[]; total: number }>('/api/v1/genres'),
  },
  languages: {
    list: () => fetcher<{ items: Language[] }>('/api/v1/languages'),
  },
  ratings: {
    set: (bookId: string, value: number) =>
      fetcher<SetRatingResponse>(`/api/v1/books/${bookId}/ratings`, {
        method: 'POST',
        body: JSON.stringify({ value }),
      }),
    my: (bookId: string) =>
      fetcher<MyRating>(`/api/v1/books/${bookId}/ratings/me`).catch(() => null),
  },
  reviews: {
    list: (bookId: string, params?: { page?: number; limit?: number }) => {
      const queryParams = new URLSearchParams();
      if (params?.page !== undefined) queryParams.set('page', String(params.page));
      if (params?.limit !== undefined) queryParams.set('limit', String(params.limit));
      return fetcher<ReviewsListResponse>(
        `/api/v1/books/${bookId}/reviews?${queryParams.toString()}`
      );
    },
    create: (bookId: string, data: { rating: number; content: string }) =>
      fetcher<Review>(`/api/v1/books/${bookId}/reviews`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    update: (id: string, data: { rating?: number; content?: string }) =>
      fetcher<Review>(`/api/v1/reviews/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    remove: (id: string) =>
      fetcher<{ status: string }>(`/api/v1/reviews/${id}`, {
        method: 'DELETE',
      }),
    toggleLike: (id: string) =>
      fetcher<ToggleLikeResponse>(`/api/v1/reviews/${id}/likes`, {
        method: 'POST',
      }),
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
  lists: {
    list: async (params?: { book_id?: string }) => {
      const queryParams: Record<string, string> = {};
      if (params?.book_id) {
        queryParams.book_id = params.book_id;
      }
      const query = new URLSearchParams(queryParams).toString();
      const raw = await fetcher<{
        items: RawList[];
        total: number;
        book_in_lists?: string[];
      }>(
        `/api/v1/lists${query ? `?${query}` : ''}`
      );
      return {
        ...raw,
        items: raw.items.map(normalizeList),
      };
    },
    create: async (data: { title: string; description?: string; is_public?: boolean }) => {
      const raw = await fetcher<RawList>('/api/v1/lists', {
        method: 'POST',
        body: JSON.stringify(data),
      });
      return normalizeList(raw);
    },
    get: async (id: string, params?: { page?: number; limit?: number }) => {
      const queryParams: Record<string, string> = {};
      if (params?.page !== undefined) queryParams.page = String(params.page);
      if (params?.limit !== undefined) queryParams.limit = String(params.limit);
      const query = new URLSearchParams(queryParams).toString();
      const raw = await fetcher<Omit<ListDetail, 'list'> & { list: RawList }>(
        `/api/v1/lists/${id}${query ? `?${query}` : ''}`
      );
      return { ...raw, list: normalizeList(raw.list) };
    },
    update: async (id: string, data: { title?: string; description?: string; is_public?: boolean }) => {
      const raw = await fetcher<RawList>(`/api/v1/lists/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      });
      return normalizeList(raw);
    },
    remove: (id: string) =>
      fetcher<{ status: string }>(`/api/v1/lists/${id}`, { method: 'DELETE' }),
    addBook: (id: string, bookId: string) =>
      fetcher<{ status: string }>(`/api/v1/lists/${id}/books`, {
        method: 'POST',
        body: JSON.stringify({ book_id: bookId }),
      }),
    removeBook: (id: string, bookId: string) =>
      fetcher<{ status: string }>(`/api/v1/lists/${id}/books/${bookId}`, {
        method: 'DELETE',
      }),
    reorder: (id: string, bookIds: string[]) =>
      fetcher<{ status: string }>(`/api/v1/lists/${id}/books/reorder`, {
        method: 'PATCH',
        body: JSON.stringify({ book_ids: bookIds }),
      }),
    getPublic: (id: string, params?: { page?: number; limit?: number }) => {
      const queryParams: Record<string, string> = {};
      if (params?.page !== undefined) queryParams.page = String(params.page);
      if (params?.limit !== undefined) queryParams.limit = String(params.limit);
      const query = new URLSearchParams(queryParams).toString();
      return fetcher<ListDetail>(`/api/v1/lists/public/${id}${query ? `?${query}` : ''}`);
    },
  },
};
