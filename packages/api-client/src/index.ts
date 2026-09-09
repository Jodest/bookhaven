export interface User {
  id: string;
  email: string;
  name: string | null;
  avatarUrl: string | null;
  role: 'user' | 'contributor' | 'moderator' | 'admin';
  profileType: 'reader' | 'librarian' | 'bookseller' | 'other';
  createdAt: string;
  updatedAt: string;
}

export interface PublicProfile {
  user: {
    id: string;
    name: string | null;
    avatarUrl: string | null;
    profileType: 'reader' | 'librarian' | 'bookseller' | 'other';
    joinedAt: string;
  };
  lists: {
    id: string;
    title: string;
    description: string;
    isPublic: boolean;
    booksCount: number;
    createdAt: string;
    updatedAt: string;
  }[];
}

export interface UpdateProfileInput {
  name?: string;
  avatarUrl?: string | null;
  profileType?: 'reader' | 'librarian' | 'bookseller' | 'other';
}

type RawUser = {
  id: string;
  email: string;
  name: string | null;
  avatar_url: string | null;
  role: 'user' | 'contributor' | 'moderator' | 'admin';
  profile_type: 'reader' | 'librarian' | 'bookseller' | 'other';
  created_at: string;
  updated_at: string;
};

function normalizeUser(raw: RawUser): User {
  return {
    id: raw.id,
    email: raw.email,
    name: raw.name,
    avatarUrl: raw.avatar_url,
    role: raw.role,
    profileType: raw.profile_type,
    createdAt: raw.created_at,
    updatedAt: raw.updated_at,
  };
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

export interface PublicList {
  id: string;
  title: string;
  description: string;
  isPublic: boolean;
  booksCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface Language {
  code: string;
  name: string;
}

export interface AdminDashboard {
  books: number;
  hidden_books: number;
  authors: number;
  genres: number;
  pending_reviews: number;
}

export interface AdminBook {
  id: string;
  title: string;
  slug: string;
  description: string | null;
  cover_url: string | null;
  isbn: string | null;
  published_year: number | null;
  page_count: number | null;
  language: string;
  rating: number;
  ratings_count: number;
  reviews_count: number;
  is_hidden: boolean;
  authors: AdminAuthor[];
  genres: AdminGenre[];
  created_at: string;
  updated_at: string;
}

export interface AdminAuthor {
  id: string;
  name: string;
  slug: string;
  biography: string | null;
  photo_url: string | null;
  birth_date: string | null;
  death_date: string | null;
}

export interface AdminGenre {
  id: string;
  name: string;
  slug: string;
}

export interface AdminReview {
  id: string;
  book_id: string;
  book_title: string;
  user_id: string;
  user: { id: string; name: string | null; avatar_url: string | null } | null;
  rating: number;
  content: string;
  likes_count: number;
  is_hidden: boolean;
  created_at: string;
  updated_at: string;
}

export interface AdminBookInput {
  title: string;
  slug?: string;
  description?: string | null;
  cover_url?: string | null;
  isbn?: string | null;
  published_year?: number | null;
  page_count?: number | null;
  language: string;
  author_ids: string[];
  genre_ids: string[];
  is_hidden: boolean;
}

export interface AdminAuthorInput {
  name: string;
  slug?: string;
  biography?: string | null;
  photo_url?: string | null;
  birth_date?: string | null;
  death_date?: string | null;
}

export interface AdminGenreInput {
  name: string;
  slug?: string;
}

const API_URL =
  typeof window === 'undefined'
    ? process.env.API_URL || 'http://backend:8080'
    : process.env.NEXT_PUBLIC_API_URL || '';

let accessToken: string | null = null;

export function setAccessToken(token: string | null): void {
  accessToken = token;
}

export function getAccessToken(): string | null {
  return accessToken;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    message?: string,
    public path: string = ''
  ) {
    super(message || `API error: ${status} ${statusText}`);
    this.name = 'ApiError';
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

async function fetcher<T>(path: string, options?: RequestInit): Promise<T> {
  const headers = new Headers(options?.headers);
  headers.set('Content-Type', 'application/json');
  // Attach bearer token to state-changing admin requests. Reads continue to
  // rely on the HttpOnly cookie so cookie auth for admin reads is preserved.
  const method = (options?.method ?? 'GET').toUpperCase();
  if (
    accessToken &&
    path.startsWith('/api/v1/admin') &&
    method !== 'GET' &&
    method !== 'HEAD'
  ) {
    headers.set('Authorization', `Bearer ${accessToken}`);
  }

  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    headers,
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
    throw new ApiError(response.status, response.statusText, message, path);
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

type RawPublicList = PublicList & {
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

function normalizePublicList(raw: RawPublicList): PublicList {
  return {
    ...raw,
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

type RawAuthResponse = {
  user: RawUser;
  access_token: string;
  refresh_token: string;
  expires_in: number;
};

function normalizeAuthResponse(raw: RawAuthResponse): AuthResponse {
  return {
    user: normalizeUser(raw.user),
    accessToken: raw.access_token,
    refreshToken: raw.refresh_token,
    expiresIn: raw.expires_in,
  };
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
        const raw = await fetcher<RawUser>('/api/v1/auth/me');
        return normalizeUser(raw);
      } catch {
        return null;
      }
    },
    login: async (data: LoginInput) => {
      const raw = await fetcher<RawAuthResponse>('/api/v1/auth/login', {
        method: 'POST',
        body: JSON.stringify(data),
      });
      return normalizeAuthResponse(raw);
    },
    register: async (data: RegisterInput) => {
      const raw = await fetcher<RawAuthResponse>('/api/v1/auth/register', {
        method: 'POST',
        body: JSON.stringify(data),
      });
      return normalizeAuthResponse(raw);
    },
    logout: () =>
      fetcher<{ status: string }>('/api/v1/auth/logout', {
        method: 'POST',
      }),
    refresh: async () => {
      const raw = await fetcher<{
        access_token: string;
        refresh_token: string;
        expires_in: number;
      }>('/api/v1/auth/refresh', {
        method: 'POST',
      });
      return {
        accessToken: raw.access_token,
        refreshToken: raw.refresh_token,
        expiresIn: raw.expires_in,
      };
    },
  },
  users: {
    getPublic: async (id: string) => {
      const raw = await fetcher<{
        user: {
          id: string;
          name: string | null;
          avatar_url: string | null;
          profile_type: 'reader' | 'librarian' | 'bookseller' | 'other';
          joined_at: string;
        };
        lists: RawPublicList[];
      }>(`/api/v1/users/${id}`);
      return {
        user: {
          id: raw.user.id,
          name: raw.user.name,
          avatarUrl: raw.user.avatar_url,
          profileType: raw.user.profile_type,
          joinedAt: raw.user.joined_at,
        },
        lists: raw.lists.map(normalizePublicList),
      };
    },
    updateMe: async (data: UpdateProfileInput) => {
      const body: Record<string, unknown> = {};
      if (data.name !== undefined) body.name = data.name;
      if (data.avatarUrl !== undefined) body.avatar_url = data.avatarUrl;
      if (data.profileType !== undefined) body.profile_type = data.profileType;
      const raw = await fetcher<RawUser>('/api/v1/users/me', {
        method: 'PATCH',
        body: JSON.stringify(body),
      });
      return normalizeUser(raw);
    },
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
      const raw = await fetcher<Omit<ListDetail, 'list' | 'items'> & { list: RawList; items: RawBook[] }>(
        `/api/v1/lists/${id}${query ? `?${query}` : ''}`
      );
      return {
        ...raw,
        list: normalizeList(raw.list),
        items: raw.items.map(normalizeBook),
      };
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
    getPublic: async (id: string, params?: { page?: number; limit?: number }) => {
      const queryParams: Record<string, string> = {};
      if (params?.page !== undefined) queryParams.page = String(params.page);
      if (params?.limit !== undefined) queryParams.limit = String(params.limit);
      const query = new URLSearchParams(queryParams).toString();
      const raw = await fetcher<Omit<ListDetail, 'list' | 'items'> & { list: RawPublicList; items: RawBook[] }>(
        `/api/v1/lists/public/${id}${query ? `?${query}` : ''}`
      );
      return {
        ...raw,
        list: normalizePublicList(raw.list),
        items: raw.items.map(normalizeBook),
      };
    },
  },
  admin: {
    dashboard: () => fetcher<AdminDashboard>('/api/v1/admin/dashboard'),
    books: {
      list: async (params?: {
        page?: number;
        limit?: number;
        search?: string;
        visibility?: 'visible' | 'hidden' | '';
      }) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.search) queryParams.search = params.search;
        if (params?.visibility) queryParams.visibility = params.visibility;
        return fetcher<PaginatedResponse<AdminBook>>(
          `/api/v1/admin/books?${new URLSearchParams(queryParams)}`
        );
      },
      get: (id: string) => fetcher<AdminBook>(`/api/v1/admin/books/${id}`),
      create: (data: AdminBookInput) =>
        fetcher<AdminBook>('/api/v1/admin/books', {
          method: 'POST',
          body: JSON.stringify(data),
        }),
      update: (id: string, data: AdminBookInput) =>
        fetcher<AdminBook>(`/api/v1/admin/books/${id}`, {
          method: 'PATCH',
          body: JSON.stringify(data),
        }),
    },
    authors: {
      list: async (params?: {
        page?: number;
        limit?: number;
        search?: string;
      }) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.search) queryParams.search = params.search;
        return fetcher<PaginatedResponse<AdminAuthor>>(
          `/api/v1/admin/authors?${new URLSearchParams(queryParams)}`
        );
      },
      create: (data: AdminAuthorInput) =>
        fetcher<AdminAuthor>('/api/v1/admin/authors', {
          method: 'POST',
          body: JSON.stringify(data),
        }),
      update: (id: string, data: AdminAuthorInput) =>
        fetcher<AdminAuthor>(`/api/v1/admin/authors/${id}`, {
          method: 'PATCH',
          body: JSON.stringify(data),
        }),
    },
    genres: {
      list: async (params?: {
        page?: number;
        limit?: number;
        search?: string;
      }) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.search) queryParams.search = params.search;
        return fetcher<PaginatedResponse<AdminGenre>>(
          `/api/v1/admin/genres?${new URLSearchParams(queryParams)}`
        );
      },
      create: (data: AdminGenreInput) =>
        fetcher<AdminGenre>('/api/v1/admin/genres', {
          method: 'POST',
          body: JSON.stringify(data),
        }),
      update: (id: string, data: AdminGenreInput) =>
        fetcher<AdminGenre>(`/api/v1/admin/genres/${id}`, {
          method: 'PATCH',
          body: JSON.stringify(data),
        }),
    },
    reviews: {
      list: async (params?: {
        page?: number;
        limit?: number;
        visibility?: 'visible' | 'hidden' | '';
      }) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.visibility) queryParams.visibility = params.visibility;
        return fetcher<PaginatedResponse<AdminReview>>(
          `/api/v1/admin/reviews?${new URLSearchParams(queryParams)}`
        );
      },
      moderate: (id: string, isHidden: boolean) =>
        fetcher<AdminReview>(`/api/v1/admin/reviews/${id}`, {
          method: 'PATCH',
          body: JSON.stringify({ is_hidden: isHidden }),
        }),
    },
  },
};
