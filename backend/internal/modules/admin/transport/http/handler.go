package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/admin/repository"
	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Service defines the admin operations used by the handler.
type Service interface {
	DashboardCounts(ctx context.Context) (books, hiddenBooks, authors, genres, pendingReviews int64, err error)
	ListBooks(ctx context.Context, filter repository.BookFilter) ([]catalogDomain.Book, int64, error)
	GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error)
	CreateBook(ctx context.Context, input adminService.BookInput) (*catalogDomain.Book, error)
	UpdateBook(ctx context.Context, id uuid.UUID, input adminService.BookInput) (*catalogDomain.Book, error)
	ListAuthors(ctx context.Context, search string, page, limit int) ([]catalogDomain.Author, int64, error)
	CreateAuthor(ctx context.Context, input adminService.AuthorInput) (*catalogDomain.Author, error)
	UpdateAuthor(ctx context.Context, id uuid.UUID, input adminService.AuthorInput) (*catalogDomain.Author, error)
	ListGenres(ctx context.Context, search string, page, limit int) ([]catalogDomain.Genre, int64, error)
	CreateGenre(ctx context.Context, input adminService.GenreInput) (*catalogDomain.Genre, error)
	UpdateGenre(ctx context.Context, id uuid.UUID, input adminService.GenreInput) (*catalogDomain.Genre, error)
	ListReviews(ctx context.Context, filter repository.ReviewFilter) ([]reviewsDomain.Review, int64, error)
	ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error)
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
	ID            string           `json:"id"`
	Title         string           `json:"title"`
	Slug          string           `json:"slug"`
	Description   *string          `json:"description,omitempty"`
	CoverURL      *string          `json:"cover_url,omitempty"`
	ISBN          *string          `json:"isbn,omitempty"`
	PublishedYear *int             `json:"published_year,omitempty"`
	PageCount     *int             `json:"page_count,omitempty"`
	Language      string           `json:"language"`
	Rating        float64          `json:"rating"`
	RatingsCount  int              `json:"ratings_count"`
	ReviewsCount  int              `json:"reviews_count"`
	IsHidden      bool             `json:"is_hidden"`
	Authors       []authorResponse `json:"authors"`
	Genres        []genreResponse  `json:"genres"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
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
	Books           int64 `json:"books"`
	HiddenBooks     int64 `json:"hidden_books"`
	Authors         int64 `json:"authors"`
	Genres          int64 `json:"genres"`
	PendingReviews  int64 `json:"pending_reviews"`
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

type createUpdateGenreRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type moderateReviewRequest struct {
	IsHidden bool `json:"is_hidden"`
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
		ID:            b.ID.String(),
		Title:         b.Title,
		Slug:          b.Slug,
		Description:   b.Description,
		CoverURL:      b.CoverURL,
		ISBN:          b.ISBN,
		PublishedYear: b.PublishedYear,
		PageCount:     b.PageCount,
		Language:      b.Language,
		Rating:        b.Rating,
		RatingsCount:  b.RatingsCount,
		ReviewsCount:  b.ReviewsCount,
		IsHidden:      b.IsHidden,
		Authors:       authors,
		Genres:        genres,
		CreatedAt:     b.CreatedAt,
		UpdatedAt:     b.UpdatedAt,
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

// Dashboard handles GET /api/v1/admin/dashboard.
func (h *Handler) Dashboard(c echo.Context) error {
	ctx := c.Request().Context()

	books, hiddenBooks, authors, genres, pendingReviews, err := h.service.DashboardCounts(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load dashboard"})
	}

	return c.JSON(http.StatusOK, dashboardResponse{
		Books:          books,
		HiddenBooks:    hiddenBooks,
		Authors:        authors,
		Genres:         genres,
		PendingReviews: pendingReviews,
	})
}

// ListBooks handles GET /api/v1/admin/books.
func (h *Handler) ListBooks(c echo.Context) error {
	ctx := c.Request().Context()

	page, limit := parsePagination(c)
	filter := repository.BookFilter{
		Search:     c.QueryParam("search"),
		Visibility: c.QueryParam("visibility"),
		Limit:      limit,
		Offset:     (page - 1) * limit,
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

// GetBook handles GET /api/v1/admin/books/:id.
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

// CreateBook handles POST /api/v1/admin/books.
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

// UpdateBook handles PATCH /api/v1/admin/books/:id.
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

// ListAuthors handles GET /api/v1/admin/authors.
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

// CreateAuthor handles POST /api/v1/admin/authors.
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

// UpdateAuthor handles PATCH /api/v1/admin/authors/:id.
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

// ListGenres handles GET /api/v1/admin/genres.
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

// CreateGenre handles POST /api/v1/admin/genres.
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

// UpdateGenre handles PATCH /api/v1/admin/genres/:id.
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

// ListReviews handles GET /api/v1/admin/reviews.
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

// ModerateReview handles PATCH /api/v1/admin/reviews/:id.
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
