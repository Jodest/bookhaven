package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Service defines the admin operations used by the handler.
type Service interface {
	DashboardCounts(ctx context.Context) (repository.DashboardStats, error)
	ListBooks(ctx context.Context, filter repository.BookFilter) ([]catalogDomain.Book, int64, error)
	GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error)
	CreateBook(ctx context.Context, input adminService.BookInput) (*catalogDomain.Book, error)
	UpdateBook(ctx context.Context, id uuid.UUID, input adminService.BookInput) (*catalogDomain.Book, error)
	ListAuthors(ctx context.Context, search, status string, page, limit int) ([]catalogDomain.Author, int64, error)
	GetAuthor(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error)
	CreateAuthor(ctx context.Context, input adminService.AuthorInput) (*catalogDomain.Author, error)
	UpdateAuthor(ctx context.Context, id uuid.UUID, input adminService.AuthorInput) (*catalogDomain.Author, error)
	MergeAuthors(ctx context.Context, duplicateID, canonicalID uuid.UUID) (*catalogDomain.Author, int64, error)
	ListAuthorModerationQueue(ctx context.Context, filter repository.AuthorModerationFilter, page, limit int) ([]adminService.AuthorQueueItem, int64, error)
	BulkSetAuthorStatus(ctx context.Context, ids []uuid.UUID, status string) (adminService.BulkStatusReport, error)
	ImportWikidataAuthor(ctx context.Context, qid string) (*adminService.AuthorImportResult, error)
	EnrichAuthorsFromWikidata(ctx context.Context, ids []uuid.UUID) (*adminService.AuthorEnrichmentReport, error)
	ListGenres(ctx context.Context, search, status string, page, limit int) ([]catalogDomain.Genre, int64, error)
	GetGenre(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error)
	CreateGenre(ctx context.Context, input adminService.GenreInput) (*catalogDomain.Genre, error)
	UpdateGenre(ctx context.Context, id uuid.UUID, input adminService.GenreInput) (*catalogDomain.Genre, error)
	ListGenreModerationQueue(ctx context.Context, filter repository.GenreModerationFilter, page, limit int) ([]adminService.GenreQueueItem, int64, error)
	BulkSetGenreStatus(ctx context.Context, ids []uuid.UUID, status string) (adminService.GenreBulkStatusReport, error)
	MergeGenres(ctx context.Context, sourceID, targetID uuid.UUID) (*catalogDomain.Genre, int64, error)
	SearchWikidataGenres(ctx context.Context, query string) (*adminService.GenreSearchResult, error)
	ImportWikidataGenre(ctx context.Context, qid string) (*adminService.GenreImportResult, error)
	ListReviews(ctx context.Context, filter repository.ReviewFilter) ([]reviewsDomain.Review, int64, error)
	ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error)
	LookupBook(ctx context.Context, isbn, query, author string) (*bookdata.ExternalBookRecord, error)
	ImportBooks(ctx context.Context, rows []adminService.ImportRowInput, defaultStatus string) adminService.ImportReport
	BulkSetBookStatus(ctx context.Context, ids []uuid.UUID, status string) (adminService.BookBulkStatusReport, error)
	ListBookModerationQueue(ctx context.Context, filter repository.BookModerationFilter, page, limit int) ([]adminService.BookQueueItem, int64, error)
	ListUsers(ctx context.Context, search, role string, page, limit int) ([]usersDomain.User, int64, error)
	ChangeUserRole(ctx context.Context, actorID, targetID uuid.UUID, role string) (*usersDomain.User, error)
}

// Handler handles admin HTTP requests.
type Handler struct {
	service Service
}

// NewHandler creates a new admin Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Request/response types.

type authorResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	Biography *string `json:"biography,omitempty"`
	PhotoURL  *string `json:"photo_url,omitempty"`
	BirthDate *string `json:"birth_date,omitempty"`
	DeathDate *string `json:"death_date,omitempty"`
	// Status is pending | published | rejected (admin payloads always
	// include it, embedded book authors too).
	Status         string  `json:"status"`
	ExternalSource *string `json:"external_source"`
	ExternalID     *string `json:"external_id"`
	// PhotoAttribution is the Wikimedia Commons attribution of a photo
	// filled by Wikidata enrichment; null for manual photos.
	PhotoAttribution *photoAttributionResponse `json:"photo_attribution"`
	// AutoHarvest is the scheduled-harvest opt-in.
	AutoHarvest bool `json:"auto_harvest"`
	// LastAutoHarvestedAt (RFC 3339, read-only) is when the worker last
	// queued a scheduled harvest covering the author; null if never.
	LastAutoHarvestedAt *string `json:"last_auto_harvested_at"`
}

// photoAttributionResponse is the Commons attribution of an author photo.
type photoAttributionResponse struct {
	Author     *string `json:"author"`
	License    string  `json:"license"`
	LicenseURL *string `json:"license_url"`
	SourceURL  string  `json:"source_url"`
}

// toPhotoAttributionResponse returns the attribution when the author has a
// photo with a Commons source page, else nil.
func toPhotoAttributionResponse(a catalogDomain.Author) *photoAttributionResponse {
	if a.PhotoURL == nil || *a.PhotoURL == "" || a.PhotoSourceURL == nil || *a.PhotoSourceURL == "" {
		return nil
	}
	resp := &photoAttributionResponse{
		Author:     a.PhotoAuthor,
		LicenseURL: a.PhotoLicenseURL,
		SourceURL:  *a.PhotoSourceURL,
	}
	if a.PhotoLicense != nil {
		resp.License = *a.PhotoLicense
	}
	return resp
}

type genreResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	// Status is pending | published | rejected (docs/features/genre-status.md).
	Status string `json:"status"`
	// WikidataQID is the bare Wikidata QID, null when none.
	WikidataQID *string `json:"wikidata_qid"`
}

// bookResponse is the admin book payload. PublishedAt is the first time the
// book became published (null if never); it is read-only and ignored in
// request bodies (#245).
type bookResponse struct {
	ID             string           `json:"id"`
	Title          string           `json:"title"`
	Slug           string           `json:"slug"`
	Description    *string          `json:"description,omitempty"`
	CoverURL       *string          `json:"cover_url,omitempty"`
	ISBN           *string          `json:"isbn,omitempty"`
	PublishedYear  *int             `json:"published_year,omitempty"`
	PageCount      *int             `json:"page_count,omitempty"`
	Language       string           `json:"language"`
	Rating         float64          `json:"rating"`
	RatingsCount   int              `json:"ratings_count"`
	ReviewsCount   int              `json:"reviews_count"`
	Status         string           `json:"status"`
	PublishedAt    *time.Time       `json:"published_at"`
	ExternalSource *string          `json:"external_source,omitempty"`
	ExternalID     *string          `json:"external_id,omitempty"`
	Authors        []authorResponse `json:"authors"`
	Genres         []genreResponse  `json:"genres"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type listBooksResponse struct {
	Items []bookResponse `json:"items"`
	Total int64          `json:"total"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
}

type listAuthorsResponse struct {
	Items []authorResponse `json:"items"`
	Total int64            `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

type listGenresResponse struct {
	Items []genreResponse `json:"items"`
	Total int64           `json:"total"`
	Page  int             `json:"page"`
	Limit int             `json:"limit"`
}

type dashboardResponse struct {
	Books          int64 `json:"books"`
	PendingBooks   int64 `json:"pending_books"`
	RejectedBooks  int64 `json:"rejected_books"`
	Authors        int64 `json:"authors"`
	Genres         int64 `json:"genres"`
	PendingReviews int64 `json:"pending_reviews"`
	PendingAuthors int64 `json:"pending_authors"`
	PendingGenres  int64 `json:"pending_genres"`
}

type authorSampleBookResponse struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type authorHintResponse struct {
	Author authorResponse `json:"author"`
	// Match is exact_name | surname_initial | surname.
	Match string `json:"match"`
}

type authorQueueItemResponse struct {
	Author       authorResponse             `json:"author"`
	CreatedAt    time.Time                  `json:"created_at"`
	BooksTotal   int64                      `json:"books_total"`
	BooksVisible int64                      `json:"books_visible"`
	SampleBooks  []authorSampleBookResponse `json:"sample_books"`
	Similar      []authorHintResponse       `json:"similar"`
}

type listAuthorQueueResponse struct {
	Items []authorQueueItemResponse `json:"items"`
	Total int64                     `json:"total"`
	Page  int                       `json:"page"`
	Limit int                       `json:"limit"`
}

type bulkAuthorStatusRequest struct {
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}

type createUpdateBookRequest struct {
	Title         string   `json:"title"`
	Slug          string   `json:"slug"`
	Description   *string  `json:"description,omitempty"`
	CoverURL      *string  `json:"cover_url,omitempty"`
	ISBN          *string  `json:"isbn,omitempty"`
	PublishedYear *int     `json:"published_year,omitempty"`
	PageCount     *int     `json:"page_count,omitempty"`
	Language      string   `json:"language"`
	AuthorIDs     []string `json:"author_ids"`
	GenreIDs      []string `json:"genre_ids"`
	// Status (pending | published | rejected): on POST omitted means
	// published; on PATCH omitted means unchanged. Invalid -> 400.
	Status *string `json:"status,omitempty"`
	// AllowNonRussianTitle lifts only the Russian-title check for official
	// Latin-script titles. Input-only, not persisted.
	AllowNonRussianTitle bool `json:"allow_non_russian_title"`
}

type createUpdateAuthorRequest struct {
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	Biography *string `json:"biography,omitempty"`
	PhotoURL  *string `json:"photo_url,omitempty"`
	BirthDate *string `json:"birth_date,omitempty"`
	DeathDate *string `json:"death_date,omitempty"`
	// Status (pending | published | rejected) is applied on PATCH only;
	// omitted = unchanged. Ignored on POST (new authors are published).
	Status *string `json:"status,omitempty"`
	// AutoHarvest opts the author into the scheduled Wikidata harvest; it
	// is applied on PATCH only (omitted = unchanged) and needs a Wikidata
	// QID when true (422 otherwise). Ignored on POST.
	AutoHarvest *bool `json:"auto_harvest,omitempty"`
}

// mergeAuthorsRequest is the body of POST /admin/authors/:id/merge.
// CanonicalID is the author that survives the merge; the path's :id is
// the duplicate/source author that is deleted.
type mergeAuthorsRequest struct {
	CanonicalID string `json:"canonical_id"`
}

// mergeAuthorsResponse is returned on a successful merge: the canonical
// author (fields unchanged by the merge) plus how many books were
// reassigned, so the admin UI can show a "N books reassigned" message
// without a second request.
type mergeAuthorsResponse struct {
	Author          authorResponse `json:"author"`
	BooksReassigned int64          `json:"books_reassigned"`
}

type createUpdateGenreRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	// Status (pending | published | rejected) is applied on PATCH only;
	// omitted = unchanged. Ignored on POST (new genres are published).
	Status *string `json:"status,omitempty"`
	// WikidataQID: omitted/null = unchanged, "" = clear, otherwise a bare
	// QID (^Q[0-9]+$ after trim/uppercase; 400 invalid, 409 taken).
	WikidataQID *string `json:"wikidata_qid,omitempty"`
}

type genreSampleBookResponse struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type genreHintResponse struct {
	Genre genreResponse `json:"genre"`
	// Match is exact_name | same_stem | shared_word.
	Match string `json:"match"`
}

type genreQueueItemResponse struct {
	Genre        genreResponse             `json:"genre"`
	CreatedAt    time.Time                 `json:"created_at"`
	BooksTotal   int64                     `json:"books_total"`
	BooksVisible int64                     `json:"books_visible"`
	SampleBooks  []genreSampleBookResponse `json:"sample_books"`
	Similar      []genreHintResponse       `json:"similar"`
}

type listGenreQueueResponse struct {
	Items []genreQueueItemResponse `json:"items"`
	Total int64                    `json:"total"`
	Page  int                      `json:"page"`
	Limit int                      `json:"limit"`
}

// bookHintBookResponse is the compact book of a "possibly the same book"
// hint (docs/features/book-moderation-queue.md); hint candidates carry no
// authors or genres.
type bookHintBookResponse struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Slug           string  `json:"slug"`
	Status         string  `json:"status"`
	ISBN           *string `json:"isbn"`
	PublishedYear  *int    `json:"published_year"`
	ExternalSource *string `json:"external_source"`
}

type bookHintResponse struct {
	Book bookHintBookResponse `json:"book"`
	// Match is same_isbn | same_title_author.
	Match string `json:"match"`
}

// bookQueueItemResponse is one book moderation queue row. ExternalID sits
// on the item (not in book) so bookResponse stays unchanged.
type bookQueueItemResponse struct {
	Book       bookResponse       `json:"book"`
	ExternalID *string            `json:"external_id"`
	CreatedAt  time.Time          `json:"created_at"`
	Similar    []bookHintResponse `json:"similar"`
	// AIVerdict is null when the book has no AI verdict.
	AIVerdict *bookAIVerdictResponse `json:"ai_verdict"`
}

// bookAIVerdictResponse is a book's AI pre-moderation verdict (openspec
// capability catalog-moderation/ai-verdicts). ReasonCodes is never null.
type bookAIVerdictResponse struct {
	// Verdict is publish | reject | needs_human.
	Verdict       string    `json:"verdict"`
	ReasonCodes   []string  `json:"reason_codes"`
	Reason        string    `json:"reason"`
	Confidence    float64   `json:"confidence"`
	Model         string    `json:"model"`
	PromptVersion int       `json:"prompt_version"`
	CreatedAt     time.Time `json:"created_at"`
	// Stale is true when the prompt version or the book's data changed
	// since the verdict was computed.
	Stale bool `json:"stale"`
}

func toBookAIVerdictResponse(v *adminService.BookAIVerdictView) *bookAIVerdictResponse {
	if v == nil {
		return nil
	}
	codes := v.ReasonCodes
	if codes == nil {
		codes = []string{}
	}
	return &bookAIVerdictResponse{
		Verdict:       v.Verdict,
		ReasonCodes:   codes,
		Reason:        v.Reason,
		Confidence:    v.Confidence,
		Model:         v.Model,
		PromptVersion: v.PromptVersion,
		CreatedAt:     v.CreatedAt,
		Stale:         v.Stale,
	}
}

type listBookQueueResponse struct {
	Items []bookQueueItemResponse `json:"items"`
	Total int64                   `json:"total"`
	Page  int                     `json:"page"`
	Limit int                     `json:"limit"`
}

type bulkGenreStatusRequest struct {
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}

// mergeGenreRequest is the body of POST /admin/genres/:id/merge. The
// path's :id is the source genre that becomes an alias; CanonicalID is the
// target that receives its books.
type mergeGenreRequest struct {
	CanonicalID string `json:"canonical_id"`
}

// mergeGenreResponse is returned on a successful merge: the target genre
// (with a carried QID) plus how many books were reassigned.
type mergeGenreResponse struct {
	Genre           genreResponse `json:"genre"`
	BooksReassigned int64         `json:"books_reassigned"`
}

type moderateReviewRequest struct {
	IsHidden bool `json:"is_hidden"`
}

type bulkBookStatusRequest struct {
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}

// lookupResponse is the normalized record returned by GET /admin/books/lookup,
// matching docs/features/catalog-manual-import.md's field list.
type lookupResponse struct {
	Title         string   `json:"title"`
	Authors       []string `json:"authors"`
	Genres        []string `json:"genres"`
	ISBN          *string  `json:"isbn,omitempty"`
	Description   *string  `json:"description,omitempty"`
	CoverURL      *string  `json:"cover_url,omitempty"`
	PublishedYear *int     `json:"published_year,omitempty"`
	PageCount     *int     `json:"page_count,omitempty"`
	Language      string   `json:"language,omitempty"`
	SourceName    string   `json:"source_name"`
	SourceID      string   `json:"source_id"`
}

func toLookupResponse(rec bookdata.ExternalBookRecord) lookupResponse {
	// Authors/Genres must never be nil in the response: the admin
	// lookup-assistant frontend calls .length on both unconditionally, and
	// a nil slice here serializes to JSON null, not [] - a boundary
	// guarantee independent of whatever bookdata client produced rec (see
	// bookdata's own mapOLSearchDoc fix for the root cause this guards
	// against recurring).
	authors := rec.Authors
	if authors == nil {
		authors = []string{}
	}
	genres := rec.Genres
	if genres == nil {
		genres = []string{}
	}
	return lookupResponse{
		Title:         rec.Title,
		Authors:       authors,
		Genres:        genres,
		ISBN:          rec.ISBN,
		Description:   rec.Description,
		CoverURL:      rec.CoverURL,
		PublishedYear: rec.PublishedYear,
		PageCount:     rec.PageCount,
		Language:      rec.Language,
		SourceName:    rec.SourceName,
		SourceID:      rec.SourceID,
	}
}

type reviewResponse struct {
	ID         string              `json:"id"`
	BookID     string              `json:"book_id"`
	BookTitle  string              `json:"book_title"`
	UserID     string              `json:"user_id"`
	Rating     int                 `json:"rating"`
	Content    string              `json:"content"`
	LikesCount int                 `json:"likes_count"`
	IsHidden   bool                `json:"is_hidden"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
	User       *reviewUserResponse `json:"user,omitempty"`
}

type reviewUserResponse struct {
	ID        string  `json:"id"`
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

type listReviewsResponse struct {
	Items []reviewResponse `json:"items"`
	Total int64            `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

func formatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func toAuthorResponse(a catalogDomain.Author) authorResponse {
	id := ""
	if a.ID != uuid.Nil {
		id = a.ID.String()
	}
	return authorResponse{
		ID:             id,
		Name:           a.Name,
		Slug:           a.Slug,
		Biography:      a.Biography,
		PhotoURL:       a.PhotoURL,
		BirthDate:      formatDate(a.BirthDate),
		DeathDate:      formatDate(a.DeathDate),
		Status:         a.Status,
		ExternalSource: a.ExternalSource,
		ExternalID:     a.ExternalID,

		PhotoAttribution:    toPhotoAttributionResponse(a),
		AutoHarvest:         a.AutoHarvest,
		LastAutoHarvestedAt: formatTimestamp(a.LastAutoHarvestedAt),
	}
}

// formatTimestamp renders an optional timestamp as RFC 3339 (UTC), or nil.
func formatTimestamp(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func toGenreResponse(g catalogDomain.Genre) genreResponse {
	id := ""
	if g.ID != uuid.Nil {
		id = g.ID.String()
	}
	return genreResponse{
		ID:          id,
		Name:        g.Name,
		Slug:        g.Slug,
		Status:      g.Status,
		WikidataQID: g.WikidataQID,
	}
}

func toBookResponse(b catalogDomain.Book) bookResponse {
	authors := make([]authorResponse, 0, len(b.Authors))
	for _, a := range b.Authors {
		authors = append(authors, toAuthorResponse(a))
	}

	genres := make([]genreResponse, 0, len(b.Genres))
	for _, g := range b.Genres {
		genres = append(genres, toGenreResponse(g))
	}

	return bookResponse{
		ID:             b.ID.String(),
		Title:          b.Title,
		Slug:           b.Slug,
		Description:    b.Description,
		CoverURL:       b.CoverURL,
		ISBN:           b.ISBN,
		PublishedYear:  b.PublishedYear,
		PageCount:      b.PageCount,
		Language:       b.Language,
		Rating:         b.Rating,
		RatingsCount:   b.RatingsCount,
		ReviewsCount:   b.ReviewsCount,
		Status:         b.Status,
		PublishedAt:    b.PublishedAt,
		ExternalSource: b.ExternalSource,
		ExternalID:     b.ExternalID,
		Authors:        authors,
		Genres:         genres,
		CreatedAt:      b.CreatedAt,
		UpdatedAt:      b.UpdatedAt,
	}
}

func toReviewResponse(r reviewsDomain.Review) reviewResponse {
	resp := reviewResponse{
		ID:         r.ID.String(),
		BookID:     r.BookID.String(),
		BookTitle:  r.BookTitle,
		UserID:     r.UserID.String(),
		Rating:     r.Rating,
		Content:    r.Content,
		LikesCount: r.LikesCount,
		IsHidden:   r.IsHidden,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if r.Author != nil {
		resp.User = &reviewUserResponse{
			ID:        r.Author.ID.String(),
			Name:      r.Author.Name,
			AvatarURL: r.Author.AvatarURL,
		}
	}
	return resp
}

func parseID(c echo.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

func parsePagination(c echo.Context) (page, limit int) {
	page, _ = strconv.Atoi(c.QueryParam("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ = strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

// Dashboard godoc
// @Summary Сводка для админ-панели
// @Description Требуется роль admin
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dashboardResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/dashboard [get]
func (h *Handler) Dashboard(c echo.Context) error {
	ctx := c.Request().Context()

	stats, err := h.service.DashboardCounts(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load dashboard"})
	}

	return c.JSON(http.StatusOK, dashboardResponse{
		Books:          stats.Books,
		PendingBooks:   stats.PendingBooks,
		RejectedBooks:  stats.RejectedBooks,
		Authors:        stats.Authors,
		Genres:         stats.Genres,
		PendingReviews: stats.PendingReviews,
		PendingAuthors: stats.PendingAuthors,
		PendingGenres:  stats.PendingGenres,
	})
}

// ListBooks godoc
// @Summary Список книг (админ)
// @Description Требуется роль admin
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Поиск"
// @Param status query string false "Статус: pending, published, rejected"
// @Param genre_id query string false "Фильтр по жанру (UUID)"
// @Param author_id query string false "Фильтр по автору (UUID)"
// @Param source query string false "Фильтр по источнику (manual/harvested)"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} listBooksResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books [get]
func (h *Handler) ListBooks(c echo.Context) error {
	ctx := c.Request().Context()

	page, limit := parsePagination(c)
	filter := repository.BookFilter{
		Search: c.QueryParam("search"),
		Status: c.QueryParam("status"),
		Source: c.QueryParam("source"),
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
	if filter.Status != "" && !catalogDomain.IsValidBookStatus(filter.Status) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": adminService.ErrInvalidBookStatus.Error()})
	}

	if genreID := c.QueryParam("genre_id"); genreID != "" {
		if id, err := uuid.Parse(genreID); err == nil {
			filter.GenreID = &id
		}
	}

	if authorID := c.QueryParam("author_id"); authorID != "" {
		if id, err := uuid.Parse(authorID); err == nil {
			filter.AuthorID = &id
		}
	}

	books, total, err := h.service.ListBooks(ctx, filter)
	if err != nil {
		if errors.Is(err, adminService.ErrInvalidBookStatus) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list books"})
	}

	items := make([]bookResponse, 0, len(books))
	for _, b := range books {
		items = append(items, toBookResponse(b))
	}

	return c.JSON(http.StatusOK, listBooksResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// GetBook godoc
// @Summary Получить книгу по ID (админ)
// @Description Требуется роль admin
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID книги"
// @Success 200 {object} bookResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/{id} [get]
func (h *Handler) GetBook(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book id"})
	}

	book, err := h.service.GetBook(ctx, id)
	if err != nil {
		if errors.Is(err, adminService.ErrBookNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get book"})
	}

	return c.JSON(http.StatusOK, toBookResponse(*book))
}

// CreateBook godoc
// @Summary Создать книгу (админ)
// @Description Требуется роль admin и Bearer-токен (state-changing запросы не принимаются по cookie)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createUpdateBookRequest true "Данные книги"
// @Success 201 {object} bookResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books [post]
func (h *Handler) CreateBook(c echo.Context) error {
	ctx := c.Request().Context()

	var req createUpdateBookRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Status != nil && !catalogDomain.IsValidBookStatus(*req.Status) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": adminService.ErrInvalidBookStatus.Error()})
	}

	book, err := h.service.CreateBook(ctx, adminService.BookInput{
		Title:         req.Title,
		Slug:          req.Slug,
		Description:   req.Description,
		CoverURL:      req.CoverURL,
		ISBN:          req.ISBN,
		PublishedYear: req.PublishedYear,
		PageCount:     req.PageCount,
		Language:      req.Language,
		AuthorIDs:     req.AuthorIDs,
		GenreIDs:      req.GenreIDs,
		Status:        req.Status,

		AllowNonRussianTitle: req.AllowNonRussianTitle,
	})
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrInvalidBookStatus):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrInvalidTitle),
			errors.Is(err, adminService.ErrTitleNotRussian),
			errors.Is(err, adminService.ErrDescriptionNotRussian),
			errors.Is(err, adminService.ErrInvalidLanguage),
			errors.Is(err, adminService.ErrInvalidYear),
			errors.Is(err, adminService.ErrInvalidPageCount),
			errors.Is(err, adminService.ErrInvalidISBN),
			errors.Is(err, adminService.ErrInvalidSlug),
			errors.Is(err, adminService.ErrInvalidAuthorIDs),
			errors.Is(err, adminService.ErrInvalidGenreIDs):
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrDuplicateSlug):
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create book"})
		}
	}

	return c.JSON(http.StatusCreated, toBookResponse(*book))
}

// UpdateBook godoc
// @Summary Обновить книгу (админ)
// @Description Требуется роль admin и Bearer-токен (state-changing запросы не принимаются по cookie)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID книги"
// @Param body body createUpdateBookRequest true "Данные книги"
// @Success 200 {object} bookResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/{id} [patch]
func (h *Handler) UpdateBook(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book id"})
	}

	var req createUpdateBookRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Status != nil && !catalogDomain.IsValidBookStatus(*req.Status) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": adminService.ErrInvalidBookStatus.Error()})
	}

	book, err := h.service.UpdateBook(ctx, id, adminService.BookInput{
		Title:         req.Title,
		Slug:          req.Slug,
		Description:   req.Description,
		CoverURL:      req.CoverURL,
		ISBN:          req.ISBN,
		PublishedYear: req.PublishedYear,
		PageCount:     req.PageCount,
		Language:      req.Language,
		AuthorIDs:     req.AuthorIDs,
		GenreIDs:      req.GenreIDs,
		Status:        req.Status,

		AllowNonRussianTitle: req.AllowNonRussianTitle,
	})
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrBookNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not found"})
		case errors.Is(err, adminService.ErrInvalidBookStatus):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrInvalidTitle),
			errors.Is(err, adminService.ErrTitleNotRussian),
			errors.Is(err, adminService.ErrDescriptionNotRussian),
			errors.Is(err, adminService.ErrInvalidLanguage),
			errors.Is(err, adminService.ErrInvalidYear),
			errors.Is(err, adminService.ErrInvalidPageCount),
			errors.Is(err, adminService.ErrInvalidISBN),
			errors.Is(err, adminService.ErrInvalidSlug),
			errors.Is(err, adminService.ErrInvalidAuthorIDs),
			errors.Is(err, adminService.ErrInvalidGenreIDs):
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrDuplicateSlug):
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update book"})
		}
	}

	return c.JSON(http.StatusOK, toBookResponse(*book))
}

// ListAuthors godoc
// @Summary Список авторов (админ)
// @Description Требуется роль admin
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Поиск"
// @Param status query string false "Статус: pending, published, rejected"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} listAuthorsResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors [get]
func (h *Handler) ListAuthors(c echo.Context) error {
	ctx := c.Request().Context()
	search := c.QueryParam("search")
	page, limit := parsePagination(c)

	status := c.QueryParam("status")
	if status != "" && !catalogDomain.IsValidAuthorStatus(status) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": adminService.ErrInvalidAuthorStatus.Error()})
	}

	authors, total, err := h.service.ListAuthors(ctx, search, status, page, limit)
	if err != nil {
		if errors.Is(err, adminService.ErrInvalidAuthorStatus) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list authors"})
	}

	items := make([]authorResponse, 0, len(authors))
	for _, a := range authors {
		items = append(items, toAuthorResponse(a))
	}

	return c.JSON(http.StatusOK, listAuthorsResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// GetAuthor godoc
// @Summary Получить автора по ID (админ)
// @Description Требуется роль admin. Возвращает автора в любом статусе.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID автора"
// @Success 200 {object} authorResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors/{id} [get]
func (h *Handler) GetAuthor(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid author id"})
	}

	author, err := h.service.GetAuthor(ctx, id)
	if err != nil {
		if errors.Is(err, adminService.ErrAuthorNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "author not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get author"})
	}

	return c.JSON(http.StatusOK, toAuthorResponse(*author))
}

// CreateAuthor godoc
// @Summary Создать автора (админ)
// @Description Требуется роль admin и Bearer-токен. Автор создаётся опубликованным; status и auto_harvest игнорируются.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createUpdateAuthorRequest true "Данные автора"
// @Success 201 {object} authorResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors [post]
func (h *Handler) CreateAuthor(c echo.Context) error {
	ctx := c.Request().Context()

	var req createUpdateAuthorRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	author, err := h.service.CreateAuthor(ctx, adminService.AuthorInput{
		Name:      req.Name,
		Slug:      req.Slug,
		Biography: req.Biography,
		PhotoURL:  req.PhotoURL,
		BirthDate: req.BirthDate,
		DeathDate: req.DeathDate,
	})
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrInvalidAuthorName),
			errors.Is(err, adminService.ErrInvalidSlug),
			errors.Is(err, adminService.ErrInvalidDate):
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrDuplicateSlug):
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create author"})
		}
	}

	return c.JSON(http.StatusCreated, toAuthorResponse(*author))
}

// UpdateAuthor godoc
// @Summary Обновить автора (админ)
// @Description Требуется роль admin и Bearer-токен. Необязательные status и auto_harvest при отсутствии в теле не меняются. auto_harvest: true для автора без Wikidata QID - 422.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID автора"
// @Param body body createUpdateAuthorRequest true "Данные автора"
// @Success 200 {object} authorResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors/{id} [patch]
func (h *Handler) UpdateAuthor(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid author id"})
	}

	var req createUpdateAuthorRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	author, err := h.service.UpdateAuthor(ctx, id, adminService.AuthorInput{
		Name:        req.Name,
		Slug:        req.Slug,
		Biography:   req.Biography,
		PhotoURL:    req.PhotoURL,
		BirthDate:   req.BirthDate,
		DeathDate:   req.DeathDate,
		Status:      req.Status,
		AutoHarvest: req.AutoHarvest,
	})
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrAuthorNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "author not found"})
		case errors.Is(err, adminService.ErrInvalidAuthorStatus):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrInvalidAuthorName),
			errors.Is(err, adminService.ErrInvalidSlug),
			errors.Is(err, adminService.ErrInvalidDate),
			errors.Is(err, adminService.ErrAutoHarvestRequiresQID):
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrDuplicateSlug):
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update author"})
		}
	}

	return c.JSON(http.StatusOK, toAuthorResponse(*author))
}

// MergeAuthors godoc
// @Summary Слить дубликат автора в канонического (админ)
// @Description Требуется роль admin и Bearer-токен. :id — автор-дубликат, который будет удалён; canonical_id в теле запроса — автор, который останется (его собственные поля не изменяются). Необратимо.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID автора-дубликата (будет удалён)"
// @Param body body mergeAuthorsRequest true "ID канонического автора"
// @Success 200 {object} mergeAuthorsResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors/{id}/merge [post]
func (h *Handler) MergeAuthors(c echo.Context) error {
	ctx := c.Request().Context()

	duplicateID, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid author id"})
	}

	var req mergeAuthorsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	canonicalID, err := uuid.Parse(req.CanonicalID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid canonical_id"})
	}

	author, booksReassigned, err := h.service.MergeAuthors(ctx, duplicateID, canonicalID)
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrCannotMergeIntoSelf):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrAuthorNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "author not found"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to merge authors"})
		}
	}

	return c.JSON(http.StatusOK, mergeAuthorsResponse{
		Author:          toAuthorResponse(*author),
		BooksReassigned: booksReassigned,
	})
}

// ListAuthorModerationQueue godoc
// @Summary Очередь модерации авторов (админ)
// @Description Требуется роль admin. Авторы с заданным статусом (по умолчанию pending), новые сверху, с количеством связанных книг, до 3 примеров книг и подсказками «возможно, тот же человек» (только для отображения).
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "Статус: pending (по умолчанию) / published / rejected"
// @Param qid query string false "Wikidata QID: with / without"
// @Param books query string false "Книги: none — без связанных книг"
// @Param search query string false "Поиск"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 50"
// @Success 200 {object} listAuthorQueueResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors/moderation [get]
func (h *Handler) ListAuthorModerationQueue(c echo.Context) error {
	ctx := c.Request().Context()
	page, limit := parsePagination(c)
	if limit > 50 {
		limit = 50
	}

	filter := repository.AuthorModerationFilter{
		Status: c.QueryParam("status"),
		QID:    c.QueryParam("qid"),
		Books:  c.QueryParam("books"),
		Search: c.QueryParam("search"),
	}

	queue, total, err := h.service.ListAuthorModerationQueue(ctx, filter, page, limit)
	if err != nil {
		if errors.Is(err, adminService.ErrInvalidModerationFilter) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list author moderation queue"})
	}

	items := make([]authorQueueItemResponse, 0, len(queue))
	for _, q := range queue {
		samples := make([]authorSampleBookResponse, 0, len(q.SampleBooks))
		for _, b := range q.SampleBooks {
			samples = append(samples, authorSampleBookResponse{ID: b.ID.String(), Title: b.Title, Status: b.Status})
		}
		similar := make([]authorHintResponse, 0, len(q.Similar))
		for _, hint := range q.Similar {
			similar = append(similar, authorHintResponse{Author: toAuthorResponse(hint.Author), Match: hint.Match})
		}
		items = append(items, authorQueueItemResponse{
			Author:       toAuthorResponse(q.Author),
			CreatedAt:    q.Author.CreatedAt,
			BooksTotal:   q.BooksTotal,
			BooksVisible: q.BooksVisible,
			SampleBooks:  samples,
			Similar:      similar,
		})
	}

	return c.JSON(http.StatusOK, listAuthorQueueResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// BulkSetAuthorStatus godoc
// @Summary Сменить статус выбранных авторов (админ)
// @Description Требуется роль admin и Bearer-токен. Меняет только status (и updated_at) у каждого автора из ids (не более 100, дубликаты схлопываются); имя, slug, биография, QID и связи с книгами не меняются. Каждый id обрабатывается независимо: неизвестный id — построчная ошибка, остальные применяются. HTTP 200 независимо от построчных исходов.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body bulkAuthorStatusRequest true "ID авторов и новый статус (pending / published / rejected)"
// @Success 200 {object} adminService.BulkStatusReport
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors/bulk-status [post]
func (h *Handler) BulkSetAuthorStatus(c echo.Context) error {
	ctx := c.Request().Context()

	var req bulkAuthorStatusRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if !catalogDomain.IsValidAuthorStatus(req.Status) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": adminService.ErrInvalidAuthorStatus.Error()})
	}
	if len(req.IDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ids must not be empty"})
	}

	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid author id: %q", raw)})
		}
		ids = append(ids, id)
	}

	report, err := h.service.BulkSetAuthorStatus(ctx, ids, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrInvalidAuthorStatus),
			errors.Is(err, adminService.ErrBulkStatusTooLarge):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update author statuses"})
		}
	}

	return c.JSON(http.StatusOK, report)
}

// importWikidataAuthorRequest is the POST /admin/authors/import-wikidata body.
type importWikidataAuthorRequest struct {
	QID string `json:"qid" example:"Q7200"`
}

// authorImportResponse is the POST /admin/authors/import-wikidata result.
type authorImportResponse struct {
	// Outcome is created | existing | rejected.
	Outcome string `json:"outcome"`
	// Match is qid | slug for existing/rejected, null for created.
	Match         *string         `json:"match"`
	QIDBackfilled bool            `json:"qid_backfilled"`
	NamesakeOf    *authorResponse `json:"namesake_of"`
	Author        authorResponse  `json:"author"`
	// Enrichment is the Wikidata enrichment applied to a created author
	// (filled fields, notes, error); null unless outcome is created.
	Enrichment *adminService.AuthorEnrichmentSummary `json:"enrichment"`
}

func toAuthorImportResponse(r adminService.AuthorImportResult) authorImportResponse {
	resp := authorImportResponse{
		Outcome:       r.Outcome,
		QIDBackfilled: r.QIDBackfilled,
		Author:        toAuthorResponse(*r.Author),
		Enrichment:    r.Enrichment,
	}
	if r.Match != "" {
		match := r.Match
		resp.Match = &match
	}
	if r.NamesakeOf != nil {
		namesake := toAuthorResponse(*r.NamesakeOf)
		resp.NamesakeOf = &namesake
	}
	return resp
}

// ImportWikidataAuthor godoc
// @Summary Импортировать автора из Wikidata по QID (админ)
// @Description Требуется роль admin и Bearer-токен. Сначала ищет автора в БД по QID (без запроса к Wikidata), затем загружает элемент Wikidata (русское название + P31: человек / псевдоним / коллективный псевдоним) и сопоставляет по правилам харвеста: совпадение по slug без QID — QID привязывается; тёзка с другим QID — создаётся отдельный автор; иначе создаётся автор в статусе pending с QID. Отклонённые авторы повторно не создаются (outcome=rejected). Статус, имя и slug существующего автора не меняются. 201 — создан, 200 — existing / rejected. 422 содержит code: no_ru_label | not_a_person.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body importWikidataAuthorRequest true "QID элемента Wikidata"
// @Success 201 {object} authorImportResponse
// @Success 200 {object} authorImportResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/authors/import-wikidata [post]
func (h *Handler) ImportWikidataAuthor(c echo.Context) error {
	ctx := c.Request().Context()

	var req importWikidataAuthorRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	result, err := h.service.ImportWikidataAuthor(ctx, req.QID)
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrInvalidQID):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid qid"})
		case errors.Is(err, adminService.ErrWikidataEntityNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "wikidata entity not found"})
		case errors.Is(err, adminService.ErrNoRussianLabel):
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "wikidata entity has no russian label", "code": "no_ru_label"})
		case errors.Is(err, adminService.ErrNotAPerson):
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "wikidata entity is not a person or pseudonym", "code": "not_a_person"})
		case errors.Is(err, adminService.ErrDuplicateSlug):
			return c.JSON(http.StatusConflict, map[string]string{"error": "author was created concurrently; retry"})
		case errors.Is(err, bookdata.ErrRateLimited), errors.Is(err, bookdata.ErrSourceUnavailable):
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "wikidata is unavailable; retry later"})
		case errors.Is(err, adminService.ErrAuthorImportUnavailable):
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "author import is not configured"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to import author"})
		}
	}

	status := http.StatusOK
	if result.Outcome == adminService.AuthorOutcomeCreated {
		status = http.StatusCreated
	}
	return c.JSON(status, toAuthorImportResponse(*result))
}

// enrichWikidataRequest is the POST /admin/authors/enrich-wikidata body.
type enrichWikidataRequest struct {
	IDs []string `json:"ids"`
}

// EnrichAuthorsFromWikidata godoc
// @Summary Обогатить авторов данными из Wikidata (админ)
// @Description Требуется роль admin и Bearer-токен. Для 1..50 авторов (любой статус, дубликаты схлопываются) с QID Wikidata заполняет только пустые поля: дату рождения и смерти (P569/P570, точность до дня, юлианские даты переводятся в григорианский календарь) и фото (P18 → Wikimedia Commons, только свободные лицензии) с атрибуцией. Заполненные значения никогда не перезаписываются. Каждый автор обрабатывается независимо: outcome = enriched | unchanged | skipped (notes: no_qid, entity_not_found) | error. 502 — только если запрос к Wikidata не удался целиком.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body enrichWikidataRequest true "ID авторов (1..50)"
// @Success 200 {object} adminService.AuthorEnrichmentReport
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/authors/enrich-wikidata [post]
func (h *Handler) EnrichAuthorsFromWikidata(c echo.Context) error {
	ctx := c.Request().Context()

	var req enrichWikidataRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if len(req.IDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ids must not be empty"})
	}
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid author id: %q", raw)})
		}
		ids = append(ids, id)
	}

	report, err := h.service.EnrichAuthorsFromWikidata(ctx, ids)
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrAuthorEnrichmentTooLarge),
			errors.Is(err, adminService.ErrAuthorEnrichmentEmpty):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrAuthorEnrichmentUnavailable):
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "author enrichment is not configured"})
		case errors.Is(err, bookdata.ErrRateLimited), errors.Is(err, bookdata.ErrSourceUnavailable):
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "wikidata is unavailable; retry later"})
		case report != nil:
			// Every wbgetentities call failed for another reason (e.g.
			// a rejected request): the per-item report says why.
			return c.JSON(http.StatusOK, report)
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to enrich authors"})
		}
	}
	return c.JSON(http.StatusOK, report)
}

// ListGenres godoc
// @Summary Список жанров (админ)
// @Description Требуется роль admin
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Поиск"
// @Param status query string false "Статус: pending, published, rejected"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} listGenresResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres [get]
func (h *Handler) ListGenres(c echo.Context) error {
	ctx := c.Request().Context()
	search := c.QueryParam("search")
	page, limit := parsePagination(c)

	status := c.QueryParam("status")
	if status != "" && !catalogDomain.IsValidGenreStatus(status) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": adminService.ErrInvalidGenreStatus.Error()})
	}

	genres, total, err := h.service.ListGenres(ctx, search, status, page, limit)
	if err != nil {
		if errors.Is(err, adminService.ErrInvalidGenreStatus) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list genres"})
	}

	items := make([]genreResponse, 0, len(genres))
	for _, g := range genres {
		items = append(items, toGenreResponse(g))
	}

	return c.JSON(http.StatusOK, listGenresResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// GetGenre godoc
// @Summary Получить жанр по ID (админ)
// @Description Требуется роль admin. Возвращает жанр в любом статусе; ID алиаса (слитого жанра) даёт 404.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID жанра"
// @Success 200 {object} genreResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres/{id} [get]
func (h *Handler) GetGenre(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid genre id"})
	}

	genre, err := h.service.GetGenre(ctx, id)
	if err != nil {
		if errors.Is(err, adminService.ErrGenreNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "genre not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get genre"})
	}

	return c.JSON(http.StatusOK, toGenreResponse(*genre))
}

// CreateGenre godoc
// @Summary Создать жанр (админ)
// @Description Требуется роль admin и Bearer-токен
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createUpdateGenreRequest true "Данные жанра"
// @Success 201 {object} genreResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres [post]
func (h *Handler) CreateGenre(c echo.Context) error {
	ctx := c.Request().Context()

	var req createUpdateGenreRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	genre, err := h.service.CreateGenre(ctx, adminService.GenreInput{
		Name:        req.Name,
		Slug:        req.Slug,
		WikidataQID: req.WikidataQID,
	})
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrInvalidQID):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrDuplicateGenreQID):
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrInvalidGenreName),
			errors.Is(err, adminService.ErrInvalidSlug):
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrDuplicateSlug):
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create genre"})
		}
	}

	return c.JSON(http.StatusCreated, toGenreResponse(*genre))
}

// UpdateGenre godoc
// @Summary Обновить жанр (админ)
// @Description Требуется роль admin и Bearer-токен
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID жанра"
// @Param body body createUpdateGenreRequest true "Данные жанра"
// @Success 200 {object} genreResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres/{id} [patch]
func (h *Handler) UpdateGenre(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid genre id"})
	}

	var req createUpdateGenreRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	genre, err := h.service.UpdateGenre(ctx, id, adminService.GenreInput{
		Name:        req.Name,
		Slug:        req.Slug,
		Status:      req.Status,
		WikidataQID: req.WikidataQID,
	})
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrGenreNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "genre not found"})
		case errors.Is(err, adminService.ErrInvalidGenreStatus),
			errors.Is(err, adminService.ErrInvalidQID):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrDuplicateGenreQID):
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrInvalidGenreName),
			errors.Is(err, adminService.ErrInvalidSlug):
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrDuplicateSlug):
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update genre"})
		}
	}

	return c.JSON(http.StatusOK, toGenreResponse(*genre))
}

// ListGenreModerationQueue godoc
// @Summary Очередь модерации жанров (админ)
// @Description Требуется роль admin. Жанры с заданным статусом (по умолчанию pending), новые сверху, с количеством связанных книг, до 3 примеров книг и подсказками «возможно, тот же жанр» (только для отображения). Слитые жанры (алиасы) не выводятся.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "Статус: pending (по умолчанию) / published / rejected"
// @Param qid query string false "Wikidata QID: with / without"
// @Param search query string false "Поиск по названию"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 50"
// @Success 200 {object} listGenreQueueResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres/moderation [get]
func (h *Handler) ListGenreModerationQueue(c echo.Context) error {
	ctx := c.Request().Context()
	page, limit := parsePagination(c)
	if limit > 50 {
		limit = 50
	}

	filter := repository.GenreModerationFilter{
		Status: c.QueryParam("status"),
		QID:    c.QueryParam("qid"),
		Search: c.QueryParam("search"),
	}

	queue, total, err := h.service.ListGenreModerationQueue(ctx, filter, page, limit)
	if err != nil {
		if errors.Is(err, adminService.ErrInvalidModerationFilter) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list genre moderation queue"})
	}

	items := make([]genreQueueItemResponse, 0, len(queue))
	for _, q := range queue {
		samples := make([]genreSampleBookResponse, 0, len(q.SampleBooks))
		for _, b := range q.SampleBooks {
			samples = append(samples, genreSampleBookResponse{ID: b.ID.String(), Title: b.Title, Status: b.Status})
		}
		similar := make([]genreHintResponse, 0, len(q.Similar))
		for _, hint := range q.Similar {
			similar = append(similar, genreHintResponse{Genre: toGenreResponse(hint.Genre), Match: hint.Match})
		}
		items = append(items, genreQueueItemResponse{
			Genre:        toGenreResponse(q.Genre),
			CreatedAt:    q.Genre.CreatedAt,
			BooksTotal:   q.BooksTotal,
			BooksVisible: q.BooksVisible,
			SampleBooks:  samples,
			Similar:      similar,
		})
	}

	return c.JSON(http.StatusOK, listGenreQueueResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// ListBookModerationQueue godoc
// @Summary Очередь модерации книг (админ)
// @Description Требуется роль admin. Книги с заданным статусом (по умолчанию pending), новые сверху, с авторами и жанрами (с их статусами), источником, external_id, подсказками «возможно, та же книга» (same_isbn / same_title_author, только для отображения) и ИИ-вердиктом ai_verdict (null, если книга не проверялась; stale = вердикт устарел).
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "Статус: pending (по умолчанию) / published / rejected"
// @Param source query string false "Источник: manual / harvested"
// @Param ai query string false "ИИ-вердикт: publish / reject / needs_human / none (без вердикта)"
// @Param search query string false "Полнотекстовый поиск (название, ISBN, описание)"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 50"
// @Success 200 {object} listBookQueueResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/moderation [get]
func (h *Handler) ListBookModerationQueue(c echo.Context) error {
	ctx := c.Request().Context()
	page, limit := parsePagination(c)
	if limit > 50 {
		limit = 50
	}

	filter := repository.BookModerationFilter{
		Status: c.QueryParam("status"),
		Source: c.QueryParam("source"),
		AI:     c.QueryParam("ai"),
		Search: c.QueryParam("search"),
	}

	queue, total, err := h.service.ListBookModerationQueue(ctx, filter, page, limit)
	if err != nil {
		if errors.Is(err, adminService.ErrInvalidModerationFilter) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list book moderation queue"})
	}

	items := make([]bookQueueItemResponse, 0, len(queue))
	for _, q := range queue {
		similar := make([]bookHintResponse, 0, len(q.Similar))
		for _, hint := range q.Similar {
			similar = append(similar, bookHintResponse{
				Book: bookHintBookResponse{
					ID:             hint.Book.ID.String(),
					Title:          hint.Book.Title,
					Slug:           hint.Book.Slug,
					Status:         hint.Book.Status,
					ISBN:           hint.Book.ISBN,
					PublishedYear:  hint.Book.PublishedYear,
					ExternalSource: hint.Book.ExternalSource,
				},
				Match: hint.Match,
			})
		}
		items = append(items, bookQueueItemResponse{
			Book:       toBookResponse(q.Book),
			ExternalID: q.Book.ExternalID,
			CreatedAt:  q.Book.CreatedAt,
			Similar:    similar,
			AIVerdict:  toBookAIVerdictResponse(q.AIVerdict),
		})
	}

	return c.JSON(http.StatusOK, listBookQueueResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// BulkSetGenreStatus godoc
// @Summary Сменить статус выбранных жанров (админ)
// @Description Требуется роль admin и Bearer-токен. Меняет только status у каждого жанра из ids (не более 100, дубликаты схлопываются); название, slug, QID и связи с книгами не меняются. Каждый id обрабатывается независимо: неизвестный id или слитый жанр — построчная ошибка «genre not found», остальные применяются. HTTP 200 независимо от построчных исходов.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body bulkGenreStatusRequest true "ID жанров и новый статус (pending / published / rejected)"
// @Success 200 {object} adminService.GenreBulkStatusReport
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres/bulk-status [post]
func (h *Handler) BulkSetGenreStatus(c echo.Context) error {
	ctx := c.Request().Context()

	var req bulkGenreStatusRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if !catalogDomain.IsValidGenreStatus(req.Status) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": adminService.ErrInvalidGenreStatus.Error()})
	}
	if len(req.IDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ids must not be empty"})
	}

	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid genre id: %q", raw)})
		}
		ids = append(ids, id)
	}

	report, err := h.service.BulkSetGenreStatus(ctx, ids, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrInvalidGenreStatus),
			errors.Is(err, adminService.ErrBulkStatusTooLarge):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update genre statuses"})
		}
	}

	return c.JSON(http.StatusOK, report)
}

// MergeGenres godoc
// @Summary Слить жанр в другой (админ)
// @Description Требуется роль admin и Bearer-токен. Книги жанра id переносятся в canonical_id (без дублей); жанр id становится отклонённым алиасом canonical_id и сохраняет название и slug, поэтому будущие импорты его названия или QID указывают на canonical_id. QID переносится, только если у canonical_id его нет. Необратимо.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID сливаемого жанра"
// @Param body body mergeGenreRequest true "ID жанра, в который сливаем"
// @Success 200 {object} mergeGenreResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres/{id}/merge [post]
func (h *Handler) MergeGenres(c echo.Context) error {
	ctx := c.Request().Context()

	sourceID, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid genre id"})
	}

	var req mergeGenreRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	targetID, err := uuid.Parse(req.CanonicalID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid canonical_id"})
	}

	genre, booksReassigned, err := h.service.MergeGenres(ctx, sourceID, targetID)
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrCannotMergeGenreIntoSelf):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, adminService.ErrGenreNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "genre not found"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to merge genres"})
		}
	}

	return c.JSON(http.StatusOK, mergeGenreResponse{
		Genre:           toGenreResponse(*genre),
		BooksReassigned: booksReassigned,
	})
}

// ListReviews godoc
// @Summary Список рецензий (модерация)
// @Description Требуется роль admin или moderator
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param visibility query string false "Фильтр видимости (all/visible/hidden)"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} listReviewsResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/reviews [get]
func (h *Handler) ListReviews(c echo.Context) error {
	ctx := c.Request().Context()
	page, limit := parsePagination(c)

	filter := repository.ReviewFilter{
		Visibility: c.QueryParam("visibility"),
		Limit:      limit,
		Offset:     (page - 1) * limit,
	}

	reviews, total, err := h.service.ListReviews(ctx, filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list reviews"})
	}

	items := make([]reviewResponse, 0, len(reviews))
	for _, r := range reviews {
		items = append(items, toReviewResponse(r))
	}

	return c.JSON(http.StatusOK, listReviewsResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// ModerateReview godoc
// @Summary Скрыть/показать рецензию (модерация)
// @Description Требуется роль admin или moderator, и Bearer-токен
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID рецензии"
// @Param body body moderateReviewRequest true "Флаг видимости"
// @Success 200 {object} reviewResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/reviews/{id} [patch]
func (h *Handler) ModerateReview(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid review id"})
	}

	var req moderateReviewRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	review, err := h.service.ModerateReview(ctx, id, req.IsHidden)
	if err != nil {
		if errors.Is(err, adminService.ErrReviewNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "review not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to moderate review"})
	}

	return c.JSON(http.StatusOK, toReviewResponse(*review))
}

// LookupBook godoc
// @Summary Поиск книги по ISBN или названию во внешнем источнике (админ)
// @Description Требуется роль admin. Синхронный, только для чтения вызов Open Library - не создаёт и не изменяет записи; сохранение выполняется отдельным POST /admin/books.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param isbn query string false "ISBN"
// @Param q query string false "Название книги"
// @Param author query string false "Автор (уточняет поиск по названию)"
// @Success 200 {object} lookupResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/lookup [get]
func (h *Handler) LookupBook(c echo.Context) error {
	ctx := c.Request().Context()

	record, err := h.service.LookupBook(ctx, c.QueryParam("isbn"), c.QueryParam("q"), c.QueryParam("author"))
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrLookupQueryRequired):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, bookdata.ErrNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "no matching book found"})
		case errors.Is(err, bookdata.ErrRateLimited), errors.Is(err, bookdata.ErrSourceUnavailable), errors.Is(err, adminService.ErrLookupUnavailable):
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "lookup source is unavailable; retry later or enter the data manually"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to look up book"})
		}
	}

	return c.JSON(http.StatusOK, toLookupResponse(*record))
}

// ImportBooks godoc
// @Summary Массовый импорт книг из CSV/JSON файла (админ)
// @Description Требуется роль admin и Bearer-токен. multipart/form-data: file (обязательно, .csv/.json), format (опционально, csv|json — иначе определяется по расширению), status (опционально, pending|published, применяется ко всей партии; по умолчанию published, иное значение — 400). Каждая строка обрабатывается независимо; результат — отчёт со счётчиками и построчными ошибками, HTTP 200 независимо от построчных исходов.
// @Tags admin
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Success 200 {object} adminService.ImportReport
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 413 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/import [post]
func (h *Handler) ImportBooks(c echo.Context) error {
	ctx := c.Request().Context()

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "file is required"})
	}

	format := strings.ToLower(strings.TrimSpace(c.FormValue("format")))
	if format == "" {
		format = sniffImportFormat(fileHeader.Filename)
	}
	if format != "csv" && format != "json" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unable to determine file format; upload a .csv/.json file or set the format field"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read uploaded file"})
	}
	defer file.Close()

	var rows []adminService.ImportRowInput
	if format == "csv" {
		rows, err = parseImportCSV(file)
	} else {
		rows, err = parseImportJSON(file)
	}
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unable to parse file: " + err.Error()})
	}

	if len(rows) > maxImportRows {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": fmt.Sprintf("file has %d rows, which exceeds the %d row limit per import; use cmd/harvest or cmd/seed for larger loads", len(rows), maxImportRows),
		})
	}

	status := strings.TrimSpace(c.FormValue("status"))
	if status == "" {
		status = catalogDomain.BookStatusPublished
	}
	if status != catalogDomain.BookStatusPending && status != catalogDomain.BookStatusPublished {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "status must be pending or published"})
	}

	report := h.service.ImportBooks(ctx, rows, status)
	return c.JSON(http.StatusOK, report)
}

// BulkSetBookStatus godoc
// @Summary Сменить статус выбранных книг (админ)
// @Description Требуется роль admin и Bearer-токен. Меняет только status (и updated_at) у каждой книги из ids (не более 100, дубликаты схлопываются); остальные поля и связи не меняются, повторная валидация контента не выполняется. Каждый id обрабатывается независимо: неизвестный id — построчная ошибка «book not found», остальные применяются. HTTP 200 независимо от построчных исходов.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body bulkBookStatusRequest true "ID книг и новый статус (pending / published / rejected)"
// @Success 200 {object} adminService.BookBulkStatusReport
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/bulk-status [post]
func (h *Handler) BulkSetBookStatus(c echo.Context) error {
	ctx := c.Request().Context()

	var req bulkBookStatusRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if !catalogDomain.IsValidBookStatus(req.Status) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": adminService.ErrInvalidBookStatus.Error()})
	}
	if len(req.IDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ids must not be empty"})
	}

	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid book id: %q", raw)})
		}
		ids = append(ids, id)
	}

	report, err := h.service.BulkSetBookStatus(ctx, ids, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrInvalidBookStatus),
			errors.Is(err, adminService.ErrBulkStatusTooLarge):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update book statuses"})
		}
	}

	return c.JSON(http.StatusOK, report)
}
