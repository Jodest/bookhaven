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
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Service defines the admin operations used by the handler.
type Service interface {
	DashboardCounts(ctx context.Context) (books, hiddenBooks, authors, genres, pendingReviews, pendingImportReview int64, err error)
	ListBooks(ctx context.Context, filter repository.BookFilter) ([]catalogDomain.Book, int64, error)
	GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error)
	CreateBook(ctx context.Context, input adminService.BookInput) (*catalogDomain.Book, error)
	UpdateBook(ctx context.Context, id uuid.UUID, input adminService.BookInput) (*catalogDomain.Book, error)
	ListAuthors(ctx context.Context, search string, page, limit int) ([]catalogDomain.Author, int64, error)
	CreateAuthor(ctx context.Context, input adminService.AuthorInput) (*catalogDomain.Author, error)
	UpdateAuthor(ctx context.Context, id uuid.UUID, input adminService.AuthorInput) (*catalogDomain.Author, error)
	MergeAuthors(ctx context.Context, duplicateID, canonicalID uuid.UUID) (*catalogDomain.Author, int64, error)
	ListGenres(ctx context.Context, search string, page, limit int) ([]catalogDomain.Genre, int64, error)
	CreateGenre(ctx context.Context, input adminService.GenreInput) (*catalogDomain.Genre, error)
	UpdateGenre(ctx context.Context, id uuid.UUID, input adminService.GenreInput) (*catalogDomain.Genre, error)
	ListReviews(ctx context.Context, filter repository.ReviewFilter) ([]reviewsDomain.Review, int64, error)
	ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error)
	LookupBook(ctx context.Context, isbn, query, author string) (*bookdata.ExternalBookRecord, error)
	ImportBooks(ctx context.Context, rows []adminService.ImportRowInput, defaultHidden bool) adminService.ImportReport
	BulkPublishBooks(ctx context.Context, search string) (adminService.BulkPublishReport, error)
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
}

type genreResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

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
	IsHidden       bool             `json:"is_hidden"`
	ExternalSource *string          `json:"external_source,omitempty"`
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
	Books               int64 `json:"books"`
	HiddenBooks         int64 `json:"hidden_books"`
	Authors             int64 `json:"authors"`
	Genres              int64 `json:"genres"`
	PendingReviews      int64 `json:"pending_reviews"`
	PendingImportReview int64 `json:"pending_import_review"`
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
	IsHidden      bool     `json:"is_hidden"`
}

type createUpdateAuthorRequest struct {
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	Biography *string `json:"biography,omitempty"`
	PhotoURL  *string `json:"photo_url,omitempty"`
	BirthDate *string `json:"birth_date,omitempty"`
	DeathDate *string `json:"death_date,omitempty"`
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
}

type moderateReviewRequest struct {
	IsHidden bool `json:"is_hidden"`
}

// bulkPublishBooksRequest is the optional body of
// POST /admin/books/bulk-publish. Search mirrors the existing search query
// param already accepted by GET /admin/books; an absent/empty value means
// "all pending-review books". There is deliberately no visibility/filter
// field here - the handler/service always and only target the
// pending_review predicate server-side, regardless of request body content.
type bulkPublishBooksRequest struct {
	Search string `json:"search,omitempty"`
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
		ID:        id,
		Name:      a.Name,
		Slug:      a.Slug,
		Biography: a.Biography,
		PhotoURL:  a.PhotoURL,
		BirthDate: formatDate(a.BirthDate),
		DeathDate: formatDate(a.DeathDate),
	}
}

func toGenreResponse(g catalogDomain.Genre) genreResponse {
	id := ""
	if g.ID != uuid.Nil {
		id = g.ID.String()
	}
	return genreResponse{
		ID:   id,
		Name: g.Name,
		Slug: g.Slug,
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
		IsHidden:       b.IsHidden,
		ExternalSource: b.ExternalSource,
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

	books, hiddenBooks, authors, genres, pendingReviews, pendingImportReview, err := h.service.DashboardCounts(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load dashboard"})
	}

	return c.JSON(http.StatusOK, dashboardResponse{
		Books:               books,
		HiddenBooks:         hiddenBooks,
		Authors:             authors,
		Genres:              genres,
		PendingReviews:      pendingReviews,
		PendingImportReview: pendingImportReview,
	})
}

// ListBooks godoc
// @Summary Список книг (админ)
// @Description Требуется роль admin
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Поиск"
// @Param visibility query string false "Фильтр видимости (all/visible/hidden/pending_review)"
// @Param genre_id query string false "Фильтр по жанру (UUID)"
// @Param author_id query string false "Фильтр по автору (UUID)"
// @Param source query string false "Фильтр по источнику (manual/harvested)"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} listBooksResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books [get]
func (h *Handler) ListBooks(c echo.Context) error {
	ctx := c.Request().Context()

	page, limit := parsePagination(c)
	filter := repository.BookFilter{
		Search:     c.QueryParam("search"),
		Visibility: c.QueryParam("visibility"),
		Source:     c.QueryParam("source"),
		Limit:      limit,
		Offset:     (page - 1) * limit,
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
		IsHidden:      req.IsHidden,
	})
	if err != nil {
		switch {
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
		IsHidden:      req.IsHidden,
	})
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrBookNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not found"})
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
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} listAuthorsResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors [get]
func (h *Handler) ListAuthors(c echo.Context) error {
	ctx := c.Request().Context()
	search := c.QueryParam("search")
	page, limit := parsePagination(c)

	authors, total, err := h.service.ListAuthors(ctx, search, page, limit)
	if err != nil {
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

// CreateAuthor godoc
// @Summary Создать автора (админ)
// @Description Требуется роль admin и Bearer-токен
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
// @Description Требуется роль admin и Bearer-токен
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID автора"
// @Param body body createUpdateAuthorRequest true "Данные автора"
// @Success 200 {object} authorResponse
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
		Name:      req.Name,
		Slug:      req.Slug,
		Biography: req.Biography,
		PhotoURL:  req.PhotoURL,
		BirthDate: req.BirthDate,
		DeathDate: req.DeathDate,
	})
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrAuthorNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "author not found"})
		case errors.Is(err, adminService.ErrInvalidAuthorName),
			errors.Is(err, adminService.ErrInvalidSlug),
			errors.Is(err, adminService.ErrInvalidDate):
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

// ListGenres godoc
// @Summary Список жанров (админ)
// @Description Требуется роль admin
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Поиск"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} listGenresResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres [get]
func (h *Handler) ListGenres(c echo.Context) error {
	ctx := c.Request().Context()
	search := c.QueryParam("search")
	page, limit := parsePagination(c)

	genres, total, err := h.service.ListGenres(ctx, search, page, limit)
	if err != nil {
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

// CreateGenre godoc
// @Summary Создать жанр (админ)
// @Description Требуется роль admin и Bearer-токен
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createUpdateGenreRequest true "Данные жанра"
// @Success 201 {object} genreResponse
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
		Name: req.Name,
		Slug: req.Slug,
	})
	if err != nil {
		switch {
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
		Name: req.Name,
		Slug: req.Slug,
	})
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrGenreNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "genre not found"})
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
// @Description Требуется роль admin и Bearer-токен. multipart/form-data: file (обязательно, .csv/.json), format (опционально, csv|json — иначе определяется по расширению), is_hidden (опционально, true/false, применяется ко всей партии). Каждая строка обрабатывается независимо; результат — отчёт со счётчиками и построчными ошибками, HTTP 200 независимо от построчных исходов.
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

	defaultHidden := strings.EqualFold(strings.TrimSpace(c.FormValue("is_hidden")), "true")

	report := h.service.ImportBooks(ctx, rows, defaultHidden)
	return c.JSON(http.StatusOK, report)
}

// BulkPublishBooks godoc
// @Summary Опубликовать все книги на проверке (админ)
// @Description Требуется роль admin и Bearer-токен. Публикует (is_hidden=false) каждую книгу, подходящую под предикат pending_review (is_hidden=true AND external_source IS NOT NULL), опционально уточнённый тем же search, что и GET /admin/books. Поле visibility в теле запроса игнорируется — предикат всегда фиксирован на сервере. Каждая строка обрабатывается независимо; результат — отчёт со счётчиками и построчными ошибками, HTTP 200 независимо от построчных исходов (кроме 422 при превышении лимита партии).
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body bulkPublishBooksRequest false "Опциональный поиск"
// @Success 200 {object} adminService.BulkPublishReport
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/bulk-publish [post]
func (h *Handler) BulkPublishBooks(c echo.Context) error {
	ctx := c.Request().Context()

	var req bulkPublishBooksRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	report, err := h.service.BulkPublishBooks(ctx, req.Search)
	if err != nil {
		if errors.Is(err, adminService.ErrBulkPublishTooLarge) {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to bulk-publish books"})
	}

	return c.JSON(http.StatusOK, report)
}
