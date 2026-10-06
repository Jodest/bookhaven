export interface User {
  id: string;
  email: string;
  name: string | null;
  avatarUrl: string | null;
  role: 'user' | 'contributor' | 'moderator' | 'admin';
  profileType: 'reader' | 'librarian' | 'bookseller' | 'other';
  emailVerified: boolean;
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
  shelves: {
    id: string;
    name: string;
    type: Shelf['type'];
    booksCount: number;
    createdAt: string;
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
  email_verified: boolean;
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
    emailVerified: raw.email_verified,
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
  /**
   * null when the author is embedded in a public book payload but is not
   * published (pending/rejected): render the name as plain text, no link.
   * Always set for GET /authors and GET /authors/:slug (published only).
   */
  slug: string | null;
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

// Notification mirrors the backend's notificationResponse shape as-is
// (snake_case, nested actor/review kept close to wire shape) - same
// no-over-normalization choice as Review.user above. `review_like` is the
// only `type` today; `review` is present whenever `type` is review-related.
export interface Notification {
  id: string;
  type: 'review_like';
  actor: { id: string; name: string | null; avatar_url: string | null };
  review?: { id: string; book_id: string; book_slug: string; book_title: string };
  is_read: boolean;
  created_at: string;
}

export interface NotificationsListResponse {
  items: Notification[];
  total: number;
  page: number;
  limit: number;
}

export interface UnreadCountResponse {
  count: number;
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

export interface ReadingChallenge {
  year: number;
  goal: number | null;
  booksRead: number;
  progressPercent: number | null;
  completed: boolean;
}

export interface UpsertReadingChallengeInput {
  goal: number;
}

type RawReadingChallenge = {
  year: number;
  goal: number | null;
  books_read: number;
  progress_percent: number | null;
  completed: boolean;
};

function normalizeReadingChallenge(raw: RawReadingChallenge): ReadingChallenge {
  return {
    year: raw.year,
    goal: raw.goal,
    booksRead: raw.books_read,
    progressPercent: raw.progress_percent,
    completed: raw.completed,
  };
}

export interface AdminDashboard {
  books: number;
  hidden_books: number;
  authors: number;
  genres: number;
  pending_reviews: number;
  pending_import_review: number;
  /** Authors with status 'pending' (the author moderation queue). */
  pending_authors: number;
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
  external_source: string | null;
  authors: AdminAuthor[];
  genres: AdminGenre[];
  created_at: string;
  updated_at: string;
}

export type AdminAuthorStatus = 'pending' | 'published' | 'rejected';

export interface AdminAuthor {
  id: string;
  name: string;
  slug: string;
  biography: string | null;
  photo_url: string | null;
  birth_date: string | null;
  death_date: string | null;
  status: AdminAuthorStatus;
  /** e.g. 'wikidata'; null for manual/legacy authors. */
  external_source: string | null;
  /** Bare Wikidata QID (e.g. 'Q7243') when external_source is 'wikidata'. */
  external_id: string | null;
}

export interface AdminGenre {
  id: string;
  name: string;
  slug: string;
}

// AdminAuthorMergeResult is the response of
// POST /api/v1/admin/authors/:id/merge. author is the canonical author
// (fields unchanged by the merge); books_reassigned is how many books
// were moved from the duplicate to the canonical author.
export interface AdminAuthorMergeResult {
  author: AdminAuthor;
  books_reassigned: number;
}

// Author moderation queue (GET /api/v1/admin/authors/moderation).
export interface AdminAuthorSampleBook {
  id: string;
  title: string;
  is_hidden: boolean;
}

/** Display-only "possibly the same person" suggestion. */
export interface AdminAuthorHint {
  author: AdminAuthor;
  match: 'exact_name' | 'surname_initial' | 'surname';
}

export interface AdminAuthorQueueItem {
  author: AdminAuthor;
  created_at: string;
  /** All linked books. */
  books_total: number;
  /** Linked books with is_hidden = false. */
  books_visible: number;
  /** Up to 3 linked books, ordered by title. */
  sample_books: AdminAuthorSampleBook[];
  similar: AdminAuthorHint[];
}

export interface AdminAuthorQueueParams {
  /** Defaults to 'pending' on the server. */
  status?: AdminAuthorStatus;
  qid?: 'with' | 'without';
  books?: 'none';
  search?: string;
  page?: number;
  limit?: number;
}

export interface AdminAuthorBulkStatusRowError {
  author_id: string;
  name: string;
  message: string;
}

// Response of POST /api/v1/admin/authors/bulk-status (HTTP 200 regardless
// of per-row outcomes).
export interface AdminAuthorBulkStatusReport {
  processed: number;
  updated: number;
  errors: AdminAuthorBulkStatusRowError[];
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
  /**
   * Opt-in: lifts only the server's Russian-title check for books whose
   * official title is written in Latin script (e.g. «Empire V»). Input-only,
   * not persisted. Not needed when the title is unchanged on update.
   */
  allow_non_russian_title?: boolean;
}

export interface AdminAuthorInput {
  name: string;
  slug?: string;
  biography?: string | null;
  photo_url?: string | null;
  birth_date?: string | null;
  death_date?: string | null;
  /** Applied on update only (omitted = unchanged); ignored on create. */
  status?: AdminAuthorStatus;
}

export interface AdminGenreInput {
  name: string;
  slug?: string;
}

// LookupRecord is the normalized record returned by
// GET /api/v1/admin/books/lookup (the "Найти по ISBN/названию" assistant).
// Fields are only present when the source actually reported them - the
// caller (book-form.tsx) must never overwrite a form field with an absent
// one.
export interface LookupRecord {
  title: string;
  authors: string[];
  genres: string[];
  isbn?: string | null;
  description?: string | null;
  cover_url?: string | null;
  published_year?: number | null;
  page_count?: number | null;
  language?: string;
  source_name: string;
  source_id: string;
}

// ImportRowError describes one failed row of a POST /api/v1/admin/books/import
// batch. row is 1-based over data rows (a CSV header row, if any, is not
// counted).
export interface ImportRowError {
  row: number;
  title: string;
  message: string;
}

// ImportReport is the response of POST /api/v1/admin/books/import, returned
// with HTTP 200 regardless of per-row outcomes.
export interface ImportReport {
  processed: number;
  created: number;
  skipped_existing: number;
  errors: ImportRowError[];
}

// BulkPublishRowError describes one book that failed to publish during a
// POST /api/v1/admin/books/bulk-publish batch.
export interface BulkPublishRowError {
  book_id: string;
  title: string;
  message: string;
}

// BulkPublishReport is the response of POST /api/v1/admin/books/bulk-publish,
// returned with HTTP 200 regardless of per-row outcomes (the batch-too-large
// guard is the only case that responds with a non-200 status instead).
export interface BulkPublishReport {
  processed: number;
  published: number;
  errors: BulkPublishRowError[];
}

// AdminHarvestTriggerInput is the request body of POST /api/v1/admin/harvest.
// author_qids are curated Wikidata author QIDs (e.g. "Q7200") - the backend
// passes them straight into bookdata.BuildAuthorListQuery, which validates
// format and dedupes; there is no free-text/raw-SPARQL mode in the admin UI.
export interface AdminHarvestTriggerInput {
  author_qids: string[];
  limit?: number;
}

// AdminHarvestTriggerResponse is the 201 response of POST /api/v1/admin/harvest.
export interface AdminHarvestTriggerResponse {
  task_id: string;
  queue: string;
  state: string;
}

// AdminHarvestJob mirrors backend harvestJobResponse (both GET /jobs and
// GET /jobs/:id). result decodes from the task's stored result whenever one
// is present and decodes cleanly - this is no longer completed-only: an
// "active" job can carry a partial result (processed < total) reported
// incrementally while the job runs (see docs/features/harvest-progress.md).
// For pending/retry jobs, result is typically absent (nothing has been
// written yet, or - for retry - reflects the previous failed attempt, which
// the admin UI intentionally does not surface as progress).
export interface AdminHarvestJob {
  id: string;
  queue: string;
  state: string;
  type: string;
  max_retry: number;
  retried: number;
  last_error?: string;
  result?: {
    processed: number;
    total: number;
    created: number;
    skipped_existing: number;
    errors: { index: number; title: string; message: string }[];
    // Per-record detail for skipped_existing (added by
    // docs/features/harvest-job-report.md) - one entry per skipped record,
    // no message field: today there is exactly one skip cause (idempotency
    // on external_source/external_id), so the frontend renders a fixed
    // label rather than a per-record backend-supplied reason.
    skipped_records: { index: number; title: string }[];
  };
}

// AdminHarvestAuthorCandidate is one item returned by
// GET /api/v1/admin/harvest/authors/search - an unconfirmed Wikidata item
// the admin must visually vet (via description) before adding its qid to
// the harvest set. See docs/features/harvest-author-search.md.
export interface AdminHarvestAuthorCandidate {
  qid: string;
  label: string;
  description?: string;
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
    public path: string = '',
    // Machine-readable error code from the response body (`code`), e.g.
    // a VerificationErrorCode. Not every endpoint sends one.
    public code?: string,
    // From the body's `retry_after_seconds` (429 responses). The
    // Retry-After header itself is not readable cross-origin.
    public retryAfterSeconds?: number
  ) {
    super(message || `API error: ${status} ${statusText}`);
    this.name = 'ApiError';
  }
}

// Machine-readable `code` values of the email verification endpoints
// (POST /auth/verify-email, POST /auth/resend-verification).
export type VerificationErrorCode =
  | 'invalid_token'
  | 'token_expired'
  | 'token_used'
  | 'invalid_email'
  | 'rate_limited'
  | 'email_send_failed';

// Machine-readable `code` values of the password reset endpoints
// (POST /auth/forgot-password, POST /auth/reset-password).
export type PasswordResetErrorCode =
  | 'invalid_token'
  | 'token_expired'
  | 'token_used'
  | 'invalid_password'
  | 'invalid_email'
  | 'rate_limited'
  | 'email_send_failed';

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

// `code` sent when the email of the account is not verified: 403 from
// POST /auth/login (correct password), 401 from POST /auth/refresh and from
// every authenticated endpoint (stale session of an unverified user).
export const EMAIL_NOT_VERIFIED_CODE = 'email_not_verified';

export function isEmailNotVerifiedError(error: unknown): boolean {
  return isApiError(error) && error.code === EMAIL_NOT_VERIFIED_CODE;
}

export interface RefreshedTokens {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

// Paths that must never trigger an auto-refresh-and-retry: a 401 from these
// means "bad credentials" / "refresh itself is dead" / "already logging out",
// not "access token expired" — retrying would be wasteful or recursive.
// verify-email, resend-verification, forgot-password and reset-password are
// public (no session involved), so a refresh there is pointless.
const AUTH_REFRESH_EXCLUDED_PATHS = [
  '/api/v1/auth/login',
  '/api/v1/auth/register',
  '/api/v1/auth/refresh',
  '/api/v1/auth/logout',
  '/api/v1/auth/verify-email',
  '/api/v1/auth/resend-verification',
  '/api/v1/auth/forgot-password',
  '/api/v1/auth/reset-password',
];

// Mirrors backend/cmd/api/main.go's `AccessTokenTTL` (currently 15 minutes).
// Used as a fallback TTL when scheduling a proactive refresh from a context
// without a real `expires_in` (e.g. after `GET /api/v1/auth/me`). Keep this in
// sync manually if the backend TTL ever changes.
export const FALLBACK_ACCESS_TOKEN_TTL_SECONDS = 15 * 60;

let refreshPromise: Promise<RefreshedTokens> | null = null;
const refreshListeners = new Set<(tokens: RefreshedTokens) => void>();

export function onTokenRefresh(listener: (tokens: RefreshedTokens) => void): () => void {
  refreshListeners.add(listener);
  return () => {
    refreshListeners.delete(listener);
  };
}

async function rawRefresh(): Promise<RefreshedTokens> {
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
}

// Dedups concurrent refresh attempts (reactive 401s and the proactive timer
// alike) so only one `/auth/refresh` call is ever in flight at a time — the
// backend rotates the refresh token on every call, so a second parallel call
// would legitimately fail.
function performRefresh(): Promise<RefreshedTokens> {
  if (refreshPromise) {
    return refreshPromise;
  }
  refreshPromise = rawRefresh()
    .then((tokens) => {
      setAccessToken(tokens.accessToken);
      refreshListeners.forEach((listener) => listener(tokens));
      return tokens;
    })
    .finally(() => {
      refreshPromise = null;
    });
  return refreshPromise;
}

// Schedules `onDue` at ~80% of the access-token TTL (clamped to a small
// minimum) so both apps can proactively refresh identically. Best-effort:
// the reactive 401-retry path in `fetcher()` is the correctness backstop if
// this timer fires late, or not at all (e.g. a throttled background tab).
export function scheduleTokenRefresh(expiresInSeconds: number, onDue: () => void): () => void {
  const MIN_DELAY_MS = 5000;
  const delayMs = Math.max(expiresInSeconds * 1000 * 0.8, MIN_DELAY_MS);
  const timeoutId = setTimeout(onDue, delayMs);
  return () => clearTimeout(timeoutId);
}

// Attach bearer token to state-changing admin requests. Reads continue to
// rely on the HttpOnly cookie so cookie auth for admin reads is preserved.
// Extracted so the 401-retry path below can rebuild headers with a
// freshly-rotated access token before retrying.
function buildRequestHeaders(path: string, options?: RequestInit): Headers {
  const headers = new Headers(options?.headers);
  // A FormData body (used by the admin bulk-import upload) must NOT get a
  // manually-set Content-Type - the browser needs to set its own
  // "multipart/form-data; boundary=..." value, which it only does when
  // Content-Type is left unset. Every other caller passes a JSON string
  // body and keeps getting the header it always got, so this is additive
  // and does not change behavior for any existing caller.
  if (!(options?.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json');
  }
  const method = (options?.method ?? 'GET').toUpperCase();
  if (
    accessToken &&
    path.startsWith('/api/v1/admin') &&
    method !== 'GET' &&
    method !== 'HEAD'
  ) {
    headers.set('Authorization', `Bearer ${accessToken}`);
  }
  return headers;
}

async function fetcher<T>(path: string, options?: RequestInit, isRetry = false): Promise<T> {
  const headers = buildRequestHeaders(path, options);

  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    headers,
    credentials: 'include',
    cache: 'no-store',
  });

  if (!response.ok) {
    if (
      response.status === 401 &&
      !isRetry &&
      !AUTH_REFRESH_EXCLUDED_PATHS.includes(path)
    ) {
      let refreshed = false;
      try {
        await performRefresh();
        refreshed = true;
      } catch {
        // Refresh failed — fall through and surface the *original* 401
        // below so existing "treat 401 as logged out" logic in callers is
        // unaffected.
      }
      if (refreshed) {
        // Rebuild headers (via the recursive call) so a rotated access
        // token is used, and retry exactly once. Any failure here
        // (including a second 401) is surfaced as-is — no second refresh.
        return fetcher<T>(path, options, true);
      }
    }

    let message: string | undefined;
    let code: string | undefined;
    let retryAfterSeconds: number | undefined;
    try {
      const body = await response.json();
      if (body && typeof body.error === 'string') {
        message = body.error;
      }
      if (body && typeof body.code === 'string') {
        code = body.code;
      }
      if (body && typeof body.retry_after_seconds === 'number') {
        retryAfterSeconds = body.retry_after_seconds;
      }
    } catch {
      // ignore
    }
    throw new ApiError(response.status, response.statusText, message, path, code, retryAfterSeconds);
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

// The lighter generic `bookListItemResponse` shape (see catalog handler's
// `toBookListItemResponse`), used by author detail's nested `books` and by
// similar books — it omits description/isbn/page_count/language/
// reviews_count/timestamps that the full book-detail response includes.
type RawBookListItem = {
  id: string;
  title: string;
  slug: string;
  cover_url: string | null;
  published_year: number | null;
  rating: number;
  ratings_count: number;
  authors: RawAuthor[];
  genres: RawGenre[];
};

type RawAuthorDetail = Omit<RawAuthor, 'books'> & {
  books: RawBookListItem[];
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

// Book list items (author detail's nested books, similar books) don't carry
// the fields full Book objects have (see RawBookListItem above); fill those
// in with neutral defaults since BookCard (the only consumer) never reads them.
function normalizeBookListItem(raw: RawBookListItem): Book {
  return {
    id: raw.id,
    title: raw.title,
    slug: raw.slug,
    description: null,
    coverUrl: raw.cover_url,
    isbn: null,
    publishedYear: raw.published_year,
    pageCount: null,
    language: '',
    rating: raw.rating,
    ratingsCount: raw.ratings_count,
    reviewsCount: 0,
    authors: raw.authors.map(normalizeAuthor),
    genres: raw.genres.map(normalizeGenre),
    createdAt: '',
    updatedAt: '',
  };
}

// Home page (see docs/features/home-page-content.md). All fields are
// normalized to camelCase, nested review user/book and calendar authors
// included.
export interface HomeBookOfTheDay {
  book: Book;
  descriptionExcerpt: string | null;
}

export interface HomeCalendarAuthor {
  id: string;
  name: string;
  slug: string;
  photoUrl: string | null;
  birthDate: string | null;
  deathDate: string | null;
}

export interface HomeCalendarEvent {
  type: 'birth' | 'death';
  years: number;
  author: HomeCalendarAuthor;
  books: Book[];
}

export interface HomeAnniversary {
  years: number;
  book: Book;
}

export interface HomeCalendar {
  /** YYYY-MM-DD of the events: today, or the nearest day with events. */
  date: string;
  isToday: boolean;
  events: HomeCalendarEvent[];
  anniversaries: HomeAnniversary[];
}

export interface HomeReview {
  id: string;
  excerpt: string;
  rating: number;
  likesCount: number;
  createdAt: string;
  user: { id: string; name: string | null; avatarUrl: string | null };
  book: {
    id: string;
    slug: string;
    title: string;
    coverUrl: string | null;
    /** slug is null for a non-published author (plain text, no link). */
    authors: { name: string; slug: string | null }[];
  };
}

export interface HomePayload {
  /** Moscow date (YYYY-MM-DD) the payload was built for. */
  date: string;
  bookOfTheDay: HomeBookOfTheDay | null;
  calendar: HomeCalendar | null;
  newBooks: Book[];
  topRated: Book[];
  recentReviews: HomeReview[];
}

type RawHomeCalendarEvent = {
  type: 'birth' | 'death';
  years: number;
  author: {
    id: string;
    name: string;
    slug: string;
    photo_url: string | null;
    birth_date: string | null;
    death_date: string | null;
  };
  books: RawBookListItem[];
};

type RawHomeReview = {
  id: string;
  excerpt: string;
  rating: number;
  likes_count: number;
  created_at: string;
  user: { id: string; name: string | null; avatar_url: string | null };
  book: {
    id: string;
    slug: string;
    title: string;
    cover_url: string | null;
    authors: { name: string; slug: string | null }[] | null;
  };
};

type RawHomePayload = {
  date: string;
  book_of_the_day: { book: RawBookListItem; description_excerpt: string | null } | null;
  calendar: {
    date: string;
    is_today: boolean;
    events: RawHomeCalendarEvent[] | null;
    anniversaries: { years: number; book: RawBookListItem }[] | null;
  } | null;
  new_books: RawBookListItem[] | null;
  top_rated: RawBookListItem[] | null;
  recent_reviews: RawHomeReview[] | null;
};

function normalizeHomeReview(raw: RawHomeReview): HomeReview {
  return {
    id: raw.id,
    excerpt: raw.excerpt,
    rating: raw.rating,
    likesCount: raw.likes_count,
    createdAt: raw.created_at,
    user: {
      id: raw.user.id,
      name: raw.user.name ?? null,
      avatarUrl: raw.user.avatar_url ?? null,
    },
    book: {
      id: raw.book.id,
      slug: raw.book.slug,
      title: raw.book.title,
      coverUrl: raw.book.cover_url ?? null,
      authors: (raw.book.authors ?? []).map((a) => ({ name: a.name, slug: a.slug })),
    },
  };
}

function normalizeHomePayload(raw: RawHomePayload): HomePayload {
  const cal = raw.calendar;
  return {
    date: raw.date,
    bookOfTheDay: raw.book_of_the_day
      ? {
          book: normalizeBookListItem(raw.book_of_the_day.book),
          descriptionExcerpt: raw.book_of_the_day.description_excerpt ?? null,
        }
      : null,
    calendar: cal
      ? {
          date: cal.date,
          isToday: cal.is_today,
          events: (cal.events ?? []).map((e) => ({
            type: e.type,
            years: e.years,
            author: {
              id: e.author.id,
              name: e.author.name,
              slug: e.author.slug,
              photoUrl: e.author.photo_url ?? null,
              birthDate: e.author.birth_date ?? null,
              deathDate: e.author.death_date ?? null,
            },
            books: (e.books ?? []).map(normalizeBookListItem),
          })),
          anniversaries: (cal.anniversaries ?? []).map((a) => ({
            years: a.years,
            book: normalizeBookListItem(a.book),
          })),
        }
      : null,
    newBooks: (raw.new_books ?? []).map(normalizeBookListItem),
    topRated: (raw.top_rated ?? []).map(normalizeBookListItem),
    recentReviews: (raw.recent_reviews ?? []).map(normalizeHomeReview),
  };
}

// Personal home blocks (GET /api/v1/home/me). continueReading and
// wantToReadPick are null until the system shelf is materialized.
export interface PersonalHome {
  continueReading: { shelfId: string; total: number; books: Book[] } | null;
  recommendations: Book[];
  wantToReadPick: { shelfId: string; total: number; book: Book | null } | null;
}

type RawPersonalHome = {
  continue_reading: { shelf_id: string; total: number; books: RawBookListItem[] | null } | null;
  recommendations: RawBookListItem[] | null;
  want_to_read_pick: { shelf_id: string; total: number; book: RawBookListItem | null } | null;
};

function normalizePersonalHome(raw: RawPersonalHome): PersonalHome {
  const cr = raw.continue_reading;
  const wtr = raw.want_to_read_pick;
  return {
    continueReading: cr
      ? { shelfId: cr.shelf_id, total: cr.total, books: (cr.books ?? []).map(normalizeBookListItem) }
      : null,
    recommendations: (raw.recommendations ?? []).map(normalizeBookListItem),
    wantToReadPick: wtr
      ? { shelfId: wtr.shelf_id, total: wtr.total, book: wtr.book ? normalizeBookListItem(wtr.book) : null }
      : null,
  };
}

function normalizeAuthorDetail(raw: RawAuthorDetail): Author {
  const { books, ...authorFields } = raw;
  return {
    ...normalizeAuthor(authorFields),
    books: books.map(normalizeBookListItem),
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

// Backend returns snake_case for shelves; normalize to the camelCase Shelf
// type. The list endpoint's items (shelfListItemResponse) omit user_id and
// updated_at - they're not needed there - so those fields are optional on
// the raw shape and defaulted when normalizing.
type RawShelf = {
  id: string;
  user_id?: string;
  name: string;
  type: Shelf['type'];
  books_count: number;
  has_book?: boolean;
  created_at: string;
  updated_at?: string;
};

function normalizeShelf(raw: RawShelf): Shelf {
  return {
    id: raw.id,
    userId: raw.user_id ?? '',
    name: raw.name,
    type: raw.type,
    booksCount: raw.books_count,
    hasBook: raw.has_book,
    createdAt: raw.created_at,
    updatedAt: raw.updated_at ?? raw.created_at,
  };
}

// Shape of a shelf entry in PublicProfile.shelves (publicShelfActivityResponse
// on the backend) - no user_id/updated_at, mirroring PublicList above.
type RawPublicShelf = {
  id: string;
  name: string;
  type: Shelf['type'];
  books_count: number;
  created_at: string;
};

function normalizePublicShelf(raw: RawPublicShelf): PublicProfile['shelves'][number] {
  return {
    id: raw.id,
    name: raw.name,
    type: raw.type,
    booksCount: raw.books_count,
    createdAt: raw.created_at,
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

// POST /auth/register: the account is created but no session is issued -
// the user must verify the email and then log in.
export interface RegisterResponse {
  email: string;
  verificationRequired: true;
}

type RawRegisterResponse = {
  email: string;
  verification_required: true;
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
    similar: async (id: string, params?: { limit?: number }) => {
      const qs = params?.limit !== undefined ? `?limit=${params.limit}` : '';
      const raw = await fetcher<{ items: RawBookListItem[] }>(`/api/v1/books/${id}/similar${qs}`);
      return { items: raw.items.map(normalizeBookListItem) };
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
    get: async (slug: string) => {
      const raw = await fetcher<RawAuthorDetail>(`/api/v1/authors/${slug}`);
      return normalizeAuthorDetail(raw);
    },
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
    // No tokens/cookies: the user logs in after verifying the email.
    register: async (data: RegisterInput): Promise<RegisterResponse> => {
      const raw = await fetcher<RawRegisterResponse>('/api/v1/auth/register', {
        method: 'POST',
        body: JSON.stringify(data),
      });
      return { email: raw.email, verificationRequired: raw.verification_required };
    },
    logout: () =>
      fetcher<{ status: string }>('/api/v1/auth/logout', {
        method: 'POST',
      }),
    refresh: (): Promise<RefreshedTokens> => performRefresh(),
    // Public: works logged out. Errors carry ApiError.code
    // ('invalid_token' | 'token_expired' | 'token_used').
    verifyEmail: (token: string) =>
      fetcher<{ status: 'verified' | 'already_verified' }>('/api/v1/auth/verify-email', {
        method: 'POST',
        body: JSON.stringify({ token }),
      }),
    // Public. Always 'sent' for a well-formed email (also unknown or already
    // verified ones - the response does not reveal registered addresses).
    // Errors carry ApiError.code ('invalid_email' | 'rate_limited'
    // (+ retryAfterSeconds) | 'email_send_failed').
    resendVerification: (email: string) =>
      fetcher<{ status: 'sent' }>('/api/v1/auth/resend-verification', {
        method: 'POST',
        body: JSON.stringify({ email }),
      }),
    // Public. Always 'sent' for a well-formed email (also unknown ones - the
    // response does not reveal registered addresses). Errors carry
    // ApiError.code (PasswordResetErrorCode: 'invalid_email' | 'rate_limited'
    // (+ retryAfterSeconds) | 'email_send_failed').
    forgotPassword: (email: string) =>
      fetcher<{ status: 'sent' }>('/api/v1/auth/forgot-password', {
        method: 'POST',
        body: JSON.stringify({ email }),
      }),
    // Public. Sets a new password with the emailed token; revokes all
    // sessions and clears auth cookies, does not log in. Errors carry
    // ApiError.code (PasswordResetErrorCode: 'invalid_password' |
    // 'invalid_token' | 'token_expired' | 'token_used').
    resetPassword: (token: string, password: string) =>
      fetcher<{ status: 'password_reset' }>('/api/v1/auth/reset-password', {
        method: 'POST',
        body: JSON.stringify({ token, password }),
      }),
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
        shelves: RawPublicShelf[];
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
        shelves: raw.shelves.map(normalizePublicShelf),
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
    list: async (params?: { book_id?: string }) => {
      const queryParams: Record<string, string> = {};
      if (params?.book_id) {
        queryParams.book_id = params.book_id;
      }
      const query = new URLSearchParams(queryParams).toString();
      const raw = await fetcher<{ items: RawShelf[]; total: number }>(
        `/api/v1/shelves${query ? `?${query}` : ''}`
      );
      return {
        ...raw,
        items: raw.items.map(normalizeShelf),
      };
    },
    get: async (
      id: string,
      params?: { page?: number; limit?: number }
    ) => {
      const queryParams: Record<string, string> = {};
      if (params?.page !== undefined) queryParams.page = String(params.page);
      if (params?.limit !== undefined) queryParams.limit = String(params.limit);
      const query = new URLSearchParams(queryParams).toString();
      const raw = await fetcher<{
        shelf: RawShelf;
        items: RawBook[];
        total: number;
        page: number;
        limit: number;
      }>(`/api/v1/shelves/${id}${query ? `?${query}` : ''}`);
      return {
        ...raw,
        shelf: normalizeShelf(raw.shelf),
        items: raw.items.map(normalizeBook),
      };
    },
    create: async (data: { name: string }) => {
      const raw = await fetcher<RawShelf>('/api/v1/shelves', {
        method: 'POST',
        body: JSON.stringify(data),
      });
      return normalizeShelf(raw);
    },
    rename: async (id: string, data: { name: string }) => {
      const raw = await fetcher<RawShelf>(`/api/v1/shelves/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      });
      return normalizeShelf(raw);
    },
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
    getPublic: async (id: string, params?: { page?: number; limit?: number }) => {
      const queryParams: Record<string, string> = {};
      if (params?.page !== undefined) queryParams.page = String(params.page);
      if (params?.limit !== undefined) queryParams.limit = String(params.limit);
      const query = new URLSearchParams(queryParams).toString();
      const raw = await fetcher<{
        shelf: RawShelf;
        items: RawBook[];
        total: number;
        page: number;
        limit: number;
      }>(`/api/v1/shelves/public/${id}${query ? `?${query}` : ''}`);
      return {
        ...raw,
        shelf: normalizeShelf(raw.shelf),
        items: raw.items.map(normalizeBook),
      };
    },
  },
  home: {
    get: async (): Promise<HomePayload> => {
      const raw = await fetcher<RawHomePayload>('/api/v1/home');
      return normalizeHomePayload(raw);
    },
    random: async (params?: { genreId?: string; excludeId?: string }): Promise<{ book: Book | null }> => {
      const query = new URLSearchParams();
      if (params?.genreId) query.set('genre_id', params.genreId);
      if (params?.excludeId) query.set('exclude_id', params.excludeId);
      const qs = query.toString();
      const raw = await fetcher<{ book: RawBookListItem | null }>(
        `/api/v1/home/random${qs ? `?${qs}` : ''}`
      );
      return { book: raw.book ? normalizeBookListItem(raw.book) : null };
    },
    me: async (): Promise<PersonalHome> => {
      const raw = await fetcher<RawPersonalHome>('/api/v1/home/me');
      return normalizePersonalHome(raw);
    },
  },
  readingChallenges: {
    list: async () => {
      const raw = await fetcher<{ items: RawReadingChallenge[]; total: number }>(
        '/api/v1/reading-challenges'
      );
      return {
        ...raw,
        items: raw.items.map(normalizeReadingChallenge),
      };
    },
    get: async (year: number) => {
      const raw = await fetcher<RawReadingChallenge>(`/api/v1/reading-challenges/${year}`);
      return normalizeReadingChallenge(raw);
    },
    upsert: async (year: number, data: UpsertReadingChallengeInput) => {
      const raw = await fetcher<RawReadingChallenge>(`/api/v1/reading-challenges/${year}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      });
      return normalizeReadingChallenge(raw);
    },
  },
  notifications: {
    list: (params?: { page?: number; limit?: number }) => {
      const queryParams = new URLSearchParams();
      if (params?.page !== undefined) queryParams.set('page', String(params.page));
      if (params?.limit !== undefined) queryParams.set('limit', String(params.limit));
      return fetcher<NotificationsListResponse>(
        `/api/v1/notifications?${queryParams.toString()}`
      );
    },
    unreadCount: () => fetcher<UnreadCountResponse>('/api/v1/notifications/unread-count'),
    markRead: (id: string) =>
      fetcher<{ status: string }>(`/api/v1/notifications/${id}/read`, {
        method: 'PATCH',
      }),
    markAllRead: () =>
      fetcher<{ status: string }>('/api/v1/notifications/read-all', {
        method: 'POST',
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
        visibility?: 'visible' | 'hidden' | 'pending_review' | '';
        genreId?: string;
        authorId?: string;
        source?: 'manual' | 'harvested' | '';
      }) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.search) queryParams.search = params.search;
        if (params?.visibility) queryParams.visibility = params.visibility;
        if (params?.genreId) queryParams.genre_id = params.genreId;
        if (params?.authorId) queryParams.author_id = params.authorId;
        if (params?.source) queryParams.source = params.source;
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
      lookup: (params: { isbn?: string; q?: string; author?: string }) => {
        const queryParams: Record<string, string> = {};
        if (params.isbn) queryParams.isbn = params.isbn;
        if (params.q) queryParams.q = params.q;
        if (params.author) queryParams.author = params.author;
        return fetcher<LookupRecord>(
          `/api/v1/admin/books/lookup?${new URLSearchParams(queryParams)}`
        );
      },
      import: (file: File, options?: { format?: 'csv' | 'json'; isHidden?: boolean }) => {
        const formData = new FormData();
        formData.append('file', file);
        if (options?.format) {
          formData.append('format', options.format);
        }
        if (options?.isHidden !== undefined) {
          formData.append('is_hidden', String(options.isHidden));
        }
        return fetcher<ImportReport>('/api/v1/admin/books/import', {
          method: 'POST',
          body: formData,
        });
      },
      bulkPublish: (params?: { search?: string }) =>
        fetcher<BulkPublishReport>('/api/v1/admin/books/bulk-publish', {
          method: 'POST',
          body: JSON.stringify({ search: params?.search }),
        }),
    },
    authors: {
      list: async (params?: {
        page?: number;
        limit?: number;
        search?: string;
        status?: AdminAuthorStatus;
      }) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.search) queryParams.search = params.search;
        if (params?.status) queryParams.status = params.status;
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
      merge: (duplicateId: string, canonicalId: string) =>
        fetcher<AdminAuthorMergeResult>(`/api/v1/admin/authors/${duplicateId}/merge`, {
          method: 'POST',
          body: JSON.stringify({ canonical_id: canonicalId }),
        }),
      moderationQueue: (params?: AdminAuthorQueueParams) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.status) queryParams.status = params.status;
        if (params?.qid) queryParams.qid = params.qid;
        if (params?.books) queryParams.books = params.books;
        if (params?.search) queryParams.search = params.search;
        return fetcher<PaginatedResponse<AdminAuthorQueueItem>>(
          `/api/v1/admin/authors/moderation?${new URLSearchParams(queryParams)}`
        );
      },
      bulkStatus: (ids: string[], status: AdminAuthorStatus) =>
        fetcher<AdminAuthorBulkStatusReport>('/api/v1/admin/authors/bulk-status', {
          method: 'POST',
          body: JSON.stringify({ ids, status }),
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
    harvest: {
      trigger: (data: AdminHarvestTriggerInput) =>
        fetcher<AdminHarvestTriggerResponse>('/api/v1/admin/harvest', {
          method: 'POST',
          body: JSON.stringify(data),
        }),
      jobs: {
        list: () => fetcher<{ items: AdminHarvestJob[] }>('/api/v1/admin/harvest/jobs'),
        get: (id: string) => fetcher<AdminHarvestJob>(`/api/v1/admin/harvest/jobs/${id}`),
      },
      authors: {
        search: (q: string) =>
          fetcher<{ items: AdminHarvestAuthorCandidate[] }>(
            `/api/v1/admin/harvest/authors/search?${new URLSearchParams({ q })}`
          ),
      },
    },
  },
};
