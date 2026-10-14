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
  /**
   * Approved AI-generated description fallback (GET /books/:slug only;
   * openspec catalog-admin/ai-descriptions). Set only while the book's own
   * description is blank; null otherwise, undefined on other payloads.
   * Render as plain text with the notice «Описание создано ИИ и может
   * содержать неточности.».
   */
  aiDescription?: BookAIDescription | null;
}

export interface BookAIDescription {
  text: string;
  generatedAt: string;
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
  /**
   * Wikimedia Commons attribution of the photo. Set (object or null) only
   * by GET /authors/:slug; undefined on list items and book embeds.
   */
  photoAttribution?: PhotoAttribution | null;
}

// AuthorPhotoAttribution is the raw (snake_case) Commons attribution of an
// author photo filled by Wikidata enrichment
// (docs/features/author-wikidata-enrichment.md). author may be null
// (public domain / CC0 without a known author).
export interface AuthorPhotoAttribution {
  author: string | null;
  license: string;
  license_url: string | null;
  source_url: string;
}

// PhotoAttribution is AuthorPhotoAttribution normalized to camelCase.
export interface PhotoAttribution {
  author: string | null;
  license: string;
  licenseUrl: string | null;
  sourceUrl: string;
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
  /** All books, every status. */
  books: number;
  /** Books with status 'pending' (docs/features/book-status-model.md). */
  pending_books: number;
  /** Books with status 'rejected'. */
  rejected_books: number;
  authors: number;
  genres: number;
  pending_reviews: number;
  /** Authors with status 'pending' (the author moderation queue). */
  pending_authors: number;
  /** Genres with status 'pending' (docs/features/genre-status.md). */
  pending_genres: number;
}

/** Book moderation status (docs/features/book-status-model.md). */
export type AdminBookStatus = 'pending' | 'published' | 'rejected';

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
  status: AdminBookStatus;
  /** First time the book became published; null if never. Read-only. */
  published_at: string | null;
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
  /** Commons attribution of a Wikidata-enriched photo; null otherwise. */
  photo_attribution: AuthorPhotoAttribution | null;
  /** Opt-in to the worker's scheduled Wikidata harvest (needs a QID). */
  auto_harvest: boolean;
  /** RFC 3339 time of the last scheduled harvest (read-only); null if never. */
  last_auto_harvested_at: string | null;
}

export type AdminGenreStatus = 'pending' | 'published' | 'rejected';

export interface AdminGenre {
  id: string;
  name: string;
  slug: string;
  /** Only 'published' genres are visible on the public site. */
  status: AdminGenreStatus;
  /** Bare Wikidata QID (e.g. 'Q8261'); null when none. */
  wikidata_qid: string | null;
}

// Admin users management (docs/features/admin-users.md).
export type AdminUserRole = User['role'];

// AdminUser is an item of GET /api/v1/admin/users and the response of
// PATCH /api/v1/admin/users/:id/role. Never carries the password hash.
export interface AdminUser {
  id: string;
  email: string;
  name: string | null;
  avatar_url: string | null;
  role: AdminUserRole;
  profile_type: User['profileType'];
  email_verified_at: string | null;
  created_at: string;
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
  status: AdminBookStatus;
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
  /** Linked books with status = published. */
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

// Genre moderation queue (GET /api/v1/admin/genres/moderation,
// docs/features/genre-moderation-queue.md). Merged genres (aliases) are
// never returned.
export type AdminGenreSampleBook = AdminAuthorSampleBook;

/** Display-only "possibly the same genre" suggestion. */
export interface AdminGenreHint {
  genre: AdminGenre;
  match: 'exact_name' | 'same_stem' | 'shared_word';
}

export interface AdminGenreQueueItem {
  genre: AdminGenre;
  created_at: string;
  /** All linked books. */
  books_total: number;
  /** Linked books with status = published. */
  books_visible: number;
  /** Up to 3 linked books, ordered by title. */
  sample_books: AdminGenreSampleBook[];
  similar: AdminGenreHint[];
}

export interface AdminGenreQueueParams {
  /** Defaults to 'pending' on the server. */
  status?: AdminGenreStatus;
  qid?: 'with' | 'without';
  search?: string;
  page?: number;
  limit?: number;
}

// Book moderation queue (GET /api/v1/admin/books/moderation,
// docs/features/book-moderation-queue.md).

/** Compact book of a "possibly the same book" hint (no authors/genres). */
export interface AdminBookHintBook {
  id: string;
  title: string;
  slug: string;
  status: AdminBookStatus;
  isbn: string | null;
  published_year: number | null;
  external_source: string | null;
}

/** Display-only "possibly the same book" suggestion. */
export interface AdminBookHint {
  book: AdminBookHintBook;
  match: 'same_isbn' | 'same_title_author';
}

export type AdminBookAIVerdictValue = 'publish' | 'reject' | 'needs_human';

/**
 * AI pre-moderation verdict of a book (openspec capability
 * catalog-moderation/ai-verdicts). Only classifies: nothing is published
 * automatically.
 */
export interface AdminBookAIVerdict {
  verdict: AdminBookAIVerdictValue;
  /** Machine reason codes; never null ([] for publish). */
  reason_codes: string[];
  /** Short Russian explanation; render as plain text only. */
  reason: string;
  /** 0..1 */
  confidence: number;
  model: string;
  prompt_version: number;
  created_at: string;
  /** The prompt version or the book's data changed since the verdict. */
  stale: boolean;
}

export interface AdminBookQueueItem {
  /** Authors and genres carry their own status. */
  book: AdminBook;
  external_id: string | null;
  created_at: string;
  similar: AdminBookHint[];
  /** null when the book has no AI verdict. */
  ai_verdict: AdminBookAIVerdict | null;
}

export interface AdminBookQueueParams {
  /** Defaults to 'pending' on the server. */
  status?: AdminBookStatus;
  source?: 'manual' | 'harvested';
  /** AI verdict filter; 'none' = books without a verdict. */
  ai?: AdminBookAIVerdictValue | 'none';
  search?: string;
  page?: number;
  limit?: number;
}

/** Body of POST /admin/books/ai-review: exactly one form. */
export type AdminBookAIReviewRequest = { book_ids: string[] } | { all_unreviewed: true };

export interface AdminBookAIReviewResponse {
  task_id: string;
}

export interface AdminBookAIReviewStatus {
  /** AI moderation is switched on and has an API key. */
  enabled: boolean;
  model: string;
  prompt_version: number;
  /** Pending books the next sweep would pick. */
  needs_review: number;
}

export interface AdminGenreBulkStatusRowError {
  genre_id: string;
  name: string;
  message: string;
}

// Response of POST /api/v1/admin/genres/bulk-status (HTTP 200 regardless
// of per-row outcomes).
export interface AdminGenreBulkStatusReport {
  processed: number;
  updated: number;
  errors: AdminGenreBulkStatusRowError[];
}

// Response of POST /api/v1/admin/genres/:id/merge. genre is the target
// (with a carried QID); books_reassigned is how many books moved to it.
export interface AdminGenreMergeResult {
  genre: AdminGenre;
  books_reassigned: number;
}

/** Fields enrichment can fill. */
export type AdminAuthorEnrichmentField = 'birth_date' | 'death_date' | 'photo';

// One author of POST /api/v1/admin/authors/enrich-wikidata. notes are codes
// such as 'no_qid', 'entity_not_found', 'birth_date_unusable',
// 'death_date_unusable', 'photo_unavailable', 'photo_license_not_free',
// 'photo_attribution_missing', 'wikidata_unavailable', 'commons_unavailable'.
export interface AdminAuthorEnrichmentItem {
  author_id: string;
  name: string;
  outcome: 'enriched' | 'unchanged' | 'skipped' | 'error';
  filled: AdminAuthorEnrichmentField[];
  notes: string[];
  message: string;
}

// Response of POST /api/v1/admin/authors/enrich-wikidata (HTTP 200
// regardless of per-item outcomes). See
// docs/features/author-wikidata-enrichment.md.
export interface AdminAuthorEnrichmentReport {
  processed: number;
  enriched: number;
  items: AdminAuthorEnrichmentItem[];
}

/** Enrichment applied to an author created by a direct import. */
export interface AdminAuthorImportEnrichment {
  filled: AdminAuthorEnrichmentField[];
  notes: string[];
  error: string;
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
  /** Create: omitted means published. Update: omitted means unchanged. */
  status?: AdminBookStatus;
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
  /**
   * Scheduled-harvest opt-in. Applied on update only (omitted = unchanged);
   * ignored on create. true on an author without a Wikidata QID is a 422.
   */
  auto_harvest?: boolean;
}

export interface AdminGenreInput {
  name: string;
  slug?: string;
  /** Applied on update only (omitted = unchanged); ignored on create (published). */
  status?: AdminGenreStatus;
  /**
   * Omitted/null = unchanged, "" = clear, otherwise a bare QID (^Q[0-9]+$).
   * Errors: 400 invalid QID, 409 QID already used by another genre.
   */
  wikidata_qid?: string | null;
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
  // Authors/genres this import created, in creation order, each at most
  // once - including those created by rows that then failed or were skipped.
  created_authors: ImportCreatedEntity[];
  created_genres: ImportCreatedEntity[];
}

// ImportCreatedEntity is one author or genre created by a book file import:
// its ID, stored name and the 1-based data row that created it.
export interface ImportCreatedEntity {
  id: string;
  name: string;
  row: number;
}

export interface AdminBookBulkStatusRowError {
  book_id: string;
  title: string;
  message: string;
}

// Response of POST /api/v1/admin/books/bulk-status (HTTP 200 regardless of
// per-row outcomes).
export interface AdminBookBulkStatusReport {
  processed: number;
  updated: number;
  errors: AdminBookBulkStatusRowError[];
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
export type AdminHarvestJobTrigger = 'manual' | 'scheduled';

export interface AdminHarvestJob {
  id: string;
  queue: string;
  state: string;
  type: string;
  max_retry: number;
  retried: number;
  last_error?: string;
  /** How the job was started; legacy and CLI jobs report 'manual'. */
  trigger: AdminHarvestJobTrigger;
  /** Harvested author QIDs; [] when unknown. */
  author_qids: string[];
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
    // End-of-job Wikidata enrichment of the authors the job created
    // (docs/features/author-wikidata-enrichment.md). Absent while the job
    // runs, in older job results and when enrichment is not configured.
    authors_enriched?: number;
    author_enrichment_errors?: number;
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
  // The BookHaven author already carrying this QID (QID match only), or
  // null. Also null when the server-side lookup failed. See
  // docs/features/author-direct-import.md.
  existing: AdminHarvestExistingAuthor | null;
}

export interface AdminHarvestExistingAuthor {
  id: string;
  name: string;
  slug: string;
  status: AdminAuthorStatus;
}

// AdminAuthorImportResult is the response of
// POST /api/v1/admin/authors/import-wikidata (201 created, 200 existing /
// rejected). match is null for created. See
// docs/features/author-direct-import.md.
export interface AdminAuthorImportResult {
  outcome: 'created' | 'existing' | 'rejected';
  match: 'qid' | 'slug' | null;
  qid_backfilled: boolean;
  namesake_of: AdminAuthor | null;
  author: AdminAuthor;
  /** Enrichment of a created author; null for existing / rejected. */
  enrichment: AdminAuthorImportEnrichment | null;
}

// AI curator of author import lists (/api/v1/admin/ai-curator, openspec
// capability catalog-import/ai-author-curation). The model proposes author
// names only; every option comes from Wikidata. Nothing is imported or
// published automatically.

export type AdminCurationStatusValue = 'queued' | 'running' | 'done' | 'failed';

export type AdminCurationMatchStatus = 'pending' | 'matched' | 'ambiguous' | 'not_found' | 'error';

export type AdminCurationErrorCode =
  | 'model_refusal'
  | 'model_truncated'
  | 'invalid_output'
  | 'model_error'
  | 'enqueue_failed'
  | 'timeout';

export interface AdminCurationStatus {
  /** AI curation is switched on and has an API key. */
  enabled: boolean;
  model: string;
  prompt_version: number;
  max_prompt_length: number;
  max_names: number;
  default_names: number;
  daily_limit: number;
  /** The caller's requests in the last 24 hours. */
  used_today: number;
  /** The caller's queued/running request, or null. */
  active_request_id: string | null;
}

export interface AdminCurationCreateInput {
  prompt: string;
  /** 1..30; omitted means 20. */
  max_names?: number;
}

export interface AdminCurationRequestSummary {
  id: string;
  prompt: string;
  max_names: number;
  status: AdminCurationStatusValue;
  error_code: AdminCurationErrorCode | null;
  model: string;
  prompt_version: number;
  created_by: { id: string; username: string } | null;
  created_at: string;
  finished_at: string | null;
  candidates_total: number;
  candidates_resolved: number;
}

// AdminCurationOption is structurally an AdminHarvestAuthorCandidate, so the
// Wikidata author import hook accepts it as is.
export interface AdminCurationOption extends AdminHarvestAuthorCandidate {
  description: string;
}

export interface AdminCurationCandidate {
  id: string;
  position: number;
  /** Proposed by the model; display-only plain text. */
  name: string;
  /** The model's short note; display-only plain text. */
  note: string;
  match_status: AdminCurationMatchStatus;
  options: AdminCurationOption[];
}

export interface AdminCurationRequest extends AdminCurationRequestSummary {
  input_tokens: number;
  output_tokens: number;
  candidates: AdminCurationCandidate[];
}

// AI book descriptions (/api/v1/admin/ai-descriptions, openspec capability
// catalog-admin/ai-descriptions). Generated texts are drafts kept apart from
// the book's own description; an admin approves or discards each one. The
// text is display-only plain text.

export type AIDescriptionStatus = 'draft' | 'approved' | 'discarded' | 'unknown' | 'failed';

export type AIDescriptionErrorCode = 'model_refusal' | 'model_truncated' | 'invalid_output';

export interface AdminAIDescriptionStatus {
  /** Generation is switched on and has an API key. */
  enabled: boolean;
  model: string;
  prompt_version: number;
  /** Books an all_missing run would pick (without the cap). */
  missing: number;
  /** Drafts whose book still has no own description. */
  drafts: number;
  max_bulk: number;
}

export interface AdminAIDescription {
  status: AIDescriptionStatus;
  /** Display-only plain text; empty for unknown and failed. */
  text: string;
  error_code: AIDescriptionErrorCode | null;
  model: string;
  prompt_version: number;
  generated_at: string;
  reviewed_at: string | null;
  edited: boolean;
  /** Prompt version or the book's model input changed since generation. */
  stale: boolean;
  /** The book now has its own description (the AI one is not public). */
  superseded: boolean;
}

export interface AdminAIDescriptionItem extends AdminAIDescription {
  book: {
    id: string;
    title: string;
    authors: string[];
    published_year: number | null;
    status: string;
  };
}

export type AdminAIDescriptionGenerateInput = { book_ids: string[] } | { all_missing: true };

export interface AdminAIDescriptionListParams {
  status?: AIDescriptionStatus;
  page?: number;
  limit?: number;
}

// Genre import from Wikidata (docs/features/genre-wikidata-import.md).

// AdminGenreExistingMarker is the "already in BookHaven" marker of a genre
// search candidate (QID match only): the canonical genre, plus the merged
// alias carrying the QID when there is one.
export interface AdminGenreExistingMarker {
  id: string;
  name: string;
  slug: string;
  status: AdminGenreStatus;
  via_alias: { id: string; name: string; slug: string } | null;
}

// AdminGenreWikidataCandidate is one item of
// GET /api/v1/admin/genres/wikidata-search. ru_label and literary_genre are
// null when class_check is 'unavailable' (ru_label also when the item has
// no Russian label).
export interface AdminGenreWikidataCandidate {
  qid: string;
  label: string;
  description: string;
  ru_label: string | null;
  literary_genre: boolean | null;
  existing: AdminGenreExistingMarker | null;
}

export interface AdminGenreWikidataSearchResult {
  class_check: 'ok' | 'unavailable';
  items: AdminGenreWikidataCandidate[];
}

// AdminGenreImportResult is the response of
// POST /api/v1/admin/genres/import-wikidata (201 created, 200 existing /
// rejected). genre is the canonical genre; via_alias the merged genre the
// match went through; qid_mismatch the QID of a same-named genre that
// carries a different QID.
export interface AdminGenreImportResult {
  outcome: 'created' | 'existing' | 'rejected';
  match: 'qid' | 'slug' | null;
  qid_backfilled: boolean;
  qid_mismatch: string | null;
  via_alias: AdminGenre | null;
  genre: AdminGenre;
}

const API_URL =
  typeof window === 'undefined'
    ? process.env.API_URL || 'http://backend:8080'
    : process.env.NEXT_PUBLIC_API_URL || '';

let accessToken: string | null = null;

// ---- Curated collections (openspec collections/*) ----

export type CuratedCollectionKind = 'manual' | 'auto';

export type AutoMetric = 'weighted_rating' | 'most_read' | 'most_wanted' | 'trending';

/** Normalized rule of an automatic collection (defaults filled in). */
export interface AutoRule {
  metric: AutoMetric;
  /** null means all time. */
  period_days: number | null;
  /** Weighted rating only, null for other metrics. */
  min_ratings: number | null;
  /** The m of the weighted rating, null for other metrics. */
  confidence: number | null;
}

/** Position change between two snapshots; null when there is no baseline. */
export interface CollectionMovement {
  direction: 'up' | 'down' | 'same' | 'new';
  by: number;
}

export interface CuratedCollectionGenre {
  id: string;
  name: string;
  slug: string;
}

/** Public collection summary (index item and detail header). */
export interface CuratedCollection {
  id: string;
  slug: string;
  title: string;
  description: string;
  kind: CuratedCollectionKind;
  size: number;
  /** Number of public places, at most `size`. */
  shown_count: number;
  source: string | null;
  source_url: string | null;
  genre: CuratedCollectionGenre | null;
  cover_urls: string[];
  /** Set for automatic collections, null for manual. */
  rule: AutoRule | null;
  /** Time of the current snapshot; null for manual or before the first computation. */
  computed_at: string | null;
}

export interface CuratedCollectionPlace {
  /** Global 1-based place, stable across pages. */
  place: number;
  /** Null for manual collections and without a baseline. */
  movement: CollectionMovement | null;
  book: Book;
}

export interface CuratedCollectionDetail {
  collection: CuratedCollection;
  items: CuratedCollectionPlace[];
  total: number;
  page: number;
  limit: number;
}

export interface AdminCuratedCollection {
  id: string;
  slug: string;
  title: string;
  description: string;
  kind: CuratedCollectionKind;
  size: number;
  source: string | null;
  source_url: string | null;
  genre: { id: string; name: string; slug: string; status: AdminGenreStatus } | null;
  is_published: boolean;
  published_at: string | null;
  rule: AutoRule | null;
  computed_at: string | null;
  /** Time of the snapshot the movement refers to; null without a baseline. */
  baseline_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface AdminCuratedCollectionListItem {
  id: string;
  slug: string;
  title: string;
  kind: CuratedCollectionKind;
  size: number;
  is_published: boolean;
  books_count: number;
  shown_count: number;
  computed_at: string | null;
  updated_at: string;
}

export type AdminCuratedItemState = 'ranked' | 'reserve' | 'hidden';

export interface AdminCuratedCollectionItem {
  position: number;
  /** 1..size for ranked items, null otherwise. */
  place: number | null;
  state: AdminCuratedItemState;
  /** Snapshot score of an automatic collection, null for manual. */
  score: number | null;
  movement: CollectionMovement | null;
  book: {
    id: string;
    title: string;
    slug: string;
    status: AdminBookStatus;
    cover_url: string | null;
    authors: { id: string; name: string }[];
  };
}

export interface AdminCuratedCollectionDetail extends AdminCuratedCollection {
  books_count: number;
  shown_count: number;
  items: AdminCuratedCollectionItem[];
}

export type PasteLineStatus =
  | 'found'
  | 'ambiguous'
  | 'not_found'
  | 'rejected'
  | 'in_collection'
  | 'duplicate'
  | 'invalid';

/** A catalog book in a bulk-paste preview. */
export interface PasteBook {
  id: string;
  title: string;
  slug: string;
  status: AdminBookStatus;
  cover_url: string | null;
  published_year: number | null;
  authors: { id: string; name: string }[];
  in_collection: boolean;
}

export interface PasteLine {
  /** 1-based line number in the pasted text (blank lines counted). */
  line: number;
  text: string;
  title: string;
  author: string | null;
  status: PasteLineStatus;
  /** Set for found and for a single-match in_collection. */
  book: PasteBook | null;
  /** Only for ambiguous and rejected lines. */
  candidates: PasteBook[];
  duplicate_of: number | null;
}

export interface PastePreview {
  books_count: number;
  free_slots: number;
  counts: Record<PasteLineStatus, number>;
  lines: PasteLine[];
}

export type BulkSkipReason = 'not_found' | 'rejected' | 'already_in_collection' | 'duplicate';

export interface BulkAddResult {
  added: { book_id: string; position: number }[];
  skipped: { book_id: string; reason: BulkSkipReason }[];
  books_count: number;
}

export interface AutoRuleInput {
  metric: AutoMetric;
  period_days?: number | null;
  min_ratings?: number | null;
  confidence?: number | null;
}

export interface AdminCuratedCollectionInput {
  /** Create only; `manual` by default. The kind cannot be changed afterwards. */
  kind?: CuratedCollectionKind;
  /** Required for `auto`, forbidden for `manual`. Replaced as a whole on update. */
  rule?: AutoRuleInput;
  title?: string;
  size?: number;
  slug?: string;
  description?: string;
  /** Empty string clears. */
  source?: string;
  /** Empty string clears. */
  source_url?: string;
  /** null clears on update. */
  genre_id?: string | null;
  is_published?: boolean;
}

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
  ai_description?: { text: string; generated_at: string } | null;
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
  photo_attribution?: AuthorPhotoAttribution | null;
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
    aiDescription: raw.ai_description
      ? { text: raw.ai_description.text, generatedAt: raw.ai_description.generated_at }
      : null,
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

function normalizePhotoAttribution(raw: AuthorPhotoAttribution | null | undefined): PhotoAttribution | null {
  if (!raw) return null;
  return {
    author: raw.author ?? null,
    license: raw.license,
    licenseUrl: raw.license_url ?? null,
    sourceUrl: raw.source_url,
  };
}

function normalizeAuthorDetail(raw: RawAuthorDetail): Author {
  const { books, photo_attribution, ...authorFields } = raw;
  return {
    ...normalizeAuthor(authorFields),
    books: books.map(normalizeBookListItem),
    photoAttribution: normalizePhotoAttribution(photo_attribution),
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
  collections: {
    list: () => fetcher<{ items: CuratedCollection[]; total: number }>('/api/v1/collections'),
    get: async (slug: string, params?: { page?: number; limit?: number }) => {
      const queryParams: Record<string, string> = {};
      if (params?.page !== undefined) queryParams.page = String(params.page);
      if (params?.limit !== undefined) queryParams.limit = String(params.limit);
      const query = new URLSearchParams(queryParams).toString();
      const raw = await fetcher<Omit<CuratedCollectionDetail, 'items'> & {
        items: { place: number; movement?: CollectionMovement | null; book: RawBookListItem }[];
      }>(`/api/v1/collections/${encodeURIComponent(slug)}${query ? `?${query}` : ''}`);
      return {
        ...raw,
        items: raw.items.map((it) => ({
          place: it.place,
          movement: it.movement ?? null,
          book: normalizeBookListItem(it.book),
        })),
      } as CuratedCollectionDetail;
    },
  },
  admin: {
    dashboard: () => fetcher<AdminDashboard>('/api/v1/admin/dashboard'),
    books: {
      list: async (params?: {
        page?: number;
        limit?: number;
        search?: string;
        status?: AdminBookStatus | '';
        genreId?: string;
        authorId?: string;
        source?: 'manual' | 'harvested' | '';
      }) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.search) queryParams.search = params.search;
        if (params?.status) queryParams.status = params.status;
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
      import: (
        file: File,
        options?: { format?: 'csv' | 'json'; status?: 'pending' | 'published' }
      ) => {
        const formData = new FormData();
        formData.append('file', file);
        if (options?.format) {
          formData.append('format', options.format);
        }
        if (options?.status) {
          formData.append('status', options.status);
        }
        return fetcher<ImportReport>('/api/v1/admin/books/import', {
          method: 'POST',
          body: formData,
        });
      },
      bulkStatus: (ids: string[], status: AdminBookStatus) =>
        fetcher<AdminBookBulkStatusReport>('/api/v1/admin/books/bulk-status', {
          method: 'POST',
          body: JSON.stringify({ ids, status }),
        }),
      moderationQueue: (params?: AdminBookQueueParams) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.status) queryParams.status = params.status;
        if (params?.source) queryParams.source = params.source;
        if (params?.ai) queryParams.ai = params.ai;
        if (params?.search) queryParams.search = params.search;
        return fetcher<PaginatedResponse<AdminBookQueueItem>>(
          `/api/v1/admin/books/moderation?${new URLSearchParams(queryParams)}`
        );
      },
      // Errors: 400 bad body, 409 already queued, 503 AI moderation disabled.
      aiReview: (body: AdminBookAIReviewRequest) =>
        fetcher<AdminBookAIReviewResponse>('/api/v1/admin/books/ai-review', {
          method: 'POST',
          body: JSON.stringify(body),
        }),
      aiReviewStatus: () => fetcher<AdminBookAIReviewStatus>('/api/v1/admin/books/ai-review/status'),
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
      // Any status. Errors: 400 invalid id, 404 unknown (or merged) author.
      get: (id: string) => fetcher<AdminAuthor>(`/api/v1/admin/authors/${id}`),
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
      // Errors: 422 carries ApiError.code 'no_ru_label' | 'not_a_person'.
      importWikidata: (qid: string) =>
        fetcher<AdminAuthorImportResult>('/api/v1/admin/authors/import-wikidata', {
          method: 'POST',
          body: JSON.stringify({ qid }),
        }),
      // Fill-empty-only enrichment of 1..50 authors. Errors: 502 Wikidata
      // unavailable, 503 not configured.
      enrichWikidata: (ids: string[]) =>
        fetcher<AdminAuthorEnrichmentReport>('/api/v1/admin/authors/enrich-wikidata', {
          method: 'POST',
          body: JSON.stringify({ ids }),
        }),
    },
    genres: {
      list: async (params?: {
        page?: number;
        limit?: number;
        search?: string;
        status?: AdminGenreStatus;
      }) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.search) queryParams.search = params.search;
        if (params?.status) queryParams.status = params.status;
        return fetcher<PaginatedResponse<AdminGenre>>(
          `/api/v1/admin/genres?${new URLSearchParams(queryParams)}`
        );
      },
      // Any status. Errors: 400 invalid id, 404 unknown genre or merge alias.
      get: (id: string) => fetcher<AdminGenre>(`/api/v1/admin/genres/${id}`),
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
      moderationQueue: (params?: AdminGenreQueueParams) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.status) queryParams.status = params.status;
        if (params?.qid) queryParams.qid = params.qid;
        if (params?.search) queryParams.search = params.search;
        return fetcher<PaginatedResponse<AdminGenreQueueItem>>(
          `/api/v1/admin/genres/moderation?${new URLSearchParams(queryParams)}`
        );
      },
      // At most 100 ids; unknown or merged ids are row errors.
      bulkStatus: (ids: string[], status: AdminGenreStatus) =>
        fetcher<AdminGenreBulkStatusReport>('/api/v1/admin/genres/bulk-status', {
          method: 'POST',
          body: JSON.stringify({ ids, status }),
        }),
      // Irreversible: sourceId becomes an alias of canonicalId. Errors: 400
      // self-merge, 404 missing or already merged genre.
      merge: (sourceId: string, canonicalId: string) =>
        fetcher<AdminGenreMergeResult>(`/api/v1/admin/genres/${sourceId}/merge`, {
          method: 'POST',
          body: JSON.stringify({ canonical_id: canonicalId }),
        }),
      // Literary-genre candidates from Wikidata. Errors: 400 empty q, 502
      // Wikidata unavailable, 503 not configured.
      searchWikidata: (q: string) =>
        fetcher<AdminGenreWikidataSearchResult>(
          `/api/v1/admin/genres/wikidata-search?${new URLSearchParams({ q })}`
        ),
      // Errors: 422 carries ApiError.code 'no_ru_label' |
      // 'not_a_literary_genre'; 409 concurrent create; 502 / 503.
      importWikidata: (qid: string) =>
        fetcher<AdminGenreImportResult>('/api/v1/admin/genres/import-wikidata', {
          method: 'POST',
          body: JSON.stringify({ qid }),
        }),
    },
    users: {
      // Errors: 400 ApiError.code 'invalid_role' (bad role filter).
      list: async (params?: {
        page?: number;
        limit?: number;
        search?: string;
        role?: AdminUserRole | '';
      }) => {
        const queryParams: Record<string, string> = {};
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        if (params?.search) queryParams.search = params.search;
        if (params?.role) queryParams.role = params.role;
        return fetcher<PaginatedResponse<AdminUser>>(
          `/api/v1/admin/users?${new URLSearchParams(queryParams)}`
        );
      },
      // Errors: 400 'invalid_role', 404 'user_not_found', 409
      // 'cannot_change_own_role' | 'last_admin' (ApiError.code); 403 only
      // when the acting admin lost the admin role.
      changeRole: (id: string, role: AdminUserRole) =>
        fetcher<AdminUser>(`/api/v1/admin/users/${id}/role`, {
          method: 'PATCH',
          body: JSON.stringify({ role }),
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
    aiCurator: {
      status: () => fetcher<AdminCurationStatus>('/api/v1/admin/ai-curator/status'),
      // Errors: 400 bad prompt/max_names, 409 active request, 429 daily
      // limit, 502 queueing failed, 503 AI curation disabled.
      create: (body: AdminCurationCreateInput) =>
        fetcher<AdminCurationRequestSummary>('/api/v1/admin/ai-curator/requests', {
          method: 'POST',
          body: JSON.stringify(body),
        }),
      list: () => fetcher<{ items: AdminCurationRequestSummary[] }>('/api/v1/admin/ai-curator/requests'),
      get: (id: string) => fetcher<AdminCurationRequest>(`/api/v1/admin/ai-curator/requests/${id}`),
    },
    aiDescriptions: {
      status: () => fetcher<AdminAIDescriptionStatus>('/api/v1/admin/ai-descriptions/status'),
      // Errors: 400 bad input, 409 an all_missing run already queued, 502
      // queueing failed, 503 AI descriptions disabled.
      generate: (body: AdminAIDescriptionGenerateInput) =>
        fetcher<{ task_id: string }>('/api/v1/admin/ai-descriptions/generate', {
          method: 'POST',
          body: JSON.stringify(body),
        }),
      list: (params?: AdminAIDescriptionListParams) => {
        const queryParams: Record<string, string> = {};
        if (params?.status) queryParams.status = params.status;
        if (params?.page !== undefined) queryParams.page = String(params.page);
        if (params?.limit !== undefined) queryParams.limit = String(params.limit);
        return fetcher<PaginatedResponse<AdminAIDescriptionItem>>(
          `/api/v1/admin/ai-descriptions?${new URLSearchParams(queryParams)}`
        );
      },
      get: (bookId: string) =>
        fetcher<{ ai_description: AdminAIDescription | null }>(`/api/v1/admin/ai-descriptions/${bookId}`),
      // Errors: 404 no record, 409 not reviewable or the book already has a
      // description, 422 invalid text.
      approve: (bookId: string, text?: string) =>
        fetcher<AdminAIDescriptionItem>(`/api/v1/admin/ai-descriptions/${bookId}/approve`, {
          method: 'POST',
          body: JSON.stringify(text === undefined ? {} : { text }),
        }),
      discard: (bookId: string) =>
        fetcher<AdminAIDescriptionItem>(`/api/v1/admin/ai-descriptions/${bookId}/discard`, {
          method: 'POST',
          body: JSON.stringify({}),
        }),
    },
    collections: {
      list: () =>
        fetcher<{ items: AdminCuratedCollectionListItem[]; total: number }>('/api/v1/admin/collections'),
      get: (id: string) => fetcher<AdminCuratedCollectionDetail>(`/api/v1/admin/collections/${id}`),
      // Errors: 400 field, 409 slug already exists.
      create: (data: AdminCuratedCollectionInput) =>
        fetcher<AdminCuratedCollection>('/api/v1/admin/collections', {
          method: 'POST',
          body: JSON.stringify(data),
        }),
      update: (id: string, data: AdminCuratedCollectionInput) =>
        fetcher<AdminCuratedCollection>(`/api/v1/admin/collections/${id}`, {
          method: 'PATCH',
          body: JSON.stringify(data),
        }),
      remove: (id: string) =>
        fetcher<{ status: string }>(`/api/v1/admin/collections/${id}`, { method: 'DELETE' }),
      // Recomputes an automatic collection now. Errors: 404, 422 collection is not automatic.
      recompute: (id: string) =>
        fetcher<AdminCuratedCollectionDetail>(`/api/v1/admin/collections/${id}/recompute`, {
          method: 'POST',
        }),
      // Errors: 404 book/collection, 409 book already in collection,
      // 422 book is rejected / collection is full.
      addBook: (id: string, bookId: string, position?: number) =>
        fetcher<{ status: string }>(`/api/v1/admin/collections/${id}/books`, {
          method: 'POST',
          body: JSON.stringify(position === undefined ? { book_id: bookId } : { book_id: bookId, position }),
        }),
      removeBook: (id: string, bookId: string) =>
        fetcher<{ status: string }>(`/api/v1/admin/collections/${id}/books/${bookId}`, {
          method: 'DELETE',
        }),
      // Read-only. Errors: 400 text, 404 collection, 422 too many lines (max 500).
      previewPaste: (id: string, text: string) =>
        fetcher<PastePreview>(`/api/v1/admin/collections/${id}/books/paste/preview`, {
          method: 'POST',
          body: JSON.stringify({ text }),
        }),
      // Errors: 400 book_ids / position, 404 collection, 422 collection is full (nothing added).
      bulkAddBooks: (id: string, bookIds: string[], position?: number) =>
        fetcher<BulkAddResult>(`/api/v1/admin/collections/${id}/books/bulk`, {
          method: 'POST',
          body: JSON.stringify(position === undefined ? { book_ids: bookIds } : { book_ids: bookIds, position }),
        }),
      moveBook: (id: string, bookId: string, position: number) =>
        fetcher<{ status: string }>(`/api/v1/admin/collections/${id}/books/${bookId}/position`, {
          method: 'PUT',
          body: JSON.stringify({ position }),
        }),
    },
  },
};

export const collectionsApi = api.collections;
export const adminCollectionsApi = api.admin.collections;
