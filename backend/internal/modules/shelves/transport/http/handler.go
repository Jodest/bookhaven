package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles shelves HTTP requests.
type Handler struct {
	service *service.Service
}

// NewHandler creates a new shelves Handler.
func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

type createShelfRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

type renameShelfRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

type addBookRequest struct {
	BookID string `json:"book_id" validate:"required,uuid"`
}

// authorResponse: Slug is null (never omitted) for a non-published author,
// embedded by name only (see docs/features/author-status.md).
type authorResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Slug      *string `json:"slug"`
	PhotoURL  *string `json:"photo_url,omitempty"`
	BirthDate *string `json:"birth_date,omitempty"`
}

type genreResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type bookListItemResponse struct {
	ID            string           `json:"id"`
	Title         string           `json:"title"`
	Slug          string           `json:"slug"`
	CoverURL      *string          `json:"cover_url,omitempty"`
	PublishedYear *int             `json:"published_year,omitempty"`
	Rating        float64          `json:"rating"`
	RatingsCount  int              `json:"ratings_count"`
	Authors       []authorResponse `json:"authors"`
	Genres        []genreResponse  `json:"genres"`
}

type shelfResponse struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	BooksCount int64     `json:"books_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type shelfListItemResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	BooksCount int64     `json:"books_count"`
	HasBook    *bool     `json:"has_book,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type listShelvesResponse struct {
	Items []shelfListItemResponse `json:"items"`
	Total int                     `json:"total"`
}

type shelfDetailResponse struct {
	Shelf shelfResponse          `json:"shelf"`
	Items []bookListItemResponse `json:"items"`
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
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
	if !a.IsPublished() {
		return authorResponse{ID: id, Name: a.Name}
	}
	slug := a.Slug
	return authorResponse{
		ID:        id,
		Name:      a.Name,
		Slug:      &slug,
		PhotoURL:  a.PhotoURL,
		BirthDate: formatDate(a.BirthDate),
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

func toBookListItemResponse(b catalogDomain.Book) bookListItemResponse {
	authors := make([]authorResponse, 0, len(b.Authors))
	for _, a := range b.Authors {
		authors = append(authors, toAuthorResponse(a))
	}

	// Only published genres are public (docs/features/genre-status.md).
	publishedGenres := catalogDomain.PublishedGenres(b.Genres)
	genres := make([]genreResponse, 0, len(publishedGenres))
	for _, g := range publishedGenres {
		genres = append(genres, toGenreResponse(g))
	}

	return bookListItemResponse{
		ID:            b.ID.String(),
		Title:         b.Title,
		Slug:          b.Slug,
		CoverURL:      b.CoverURL,
		PublishedYear: b.PublishedYear,
		Rating:        b.Rating,
		RatingsCount:  b.RatingsCount,
		Authors:       authors,
		Genres:        genres,
	}
}

func toShelfResponse(s domain.Shelf) shelfResponse {
	return shelfResponse{
		ID:         s.ID.String(),
		UserID:     s.UserID.String(),
		Name:       s.Name,
		Type:       string(s.Type),
		BooksCount: s.BooksCount,
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}
}

func getUserID(c echo.Context) (uuid.UUID, error) {
	userIDStr, ok := c.Get("user_id").(string)
	if !ok || userIDStr == "" {
		return uuid.Nil, errors.New("unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, errors.New("invalid user")
	}
	return userID, nil
}

func parseShelfID(c echo.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

// ListShelves godoc
// @Summary Список полок пользователя
// @Tags shelves
// @Produce json
// @Security BearerAuth
// @Param book_id query string false "Если указан, для каждой полки помечается наличие книги"
// @Success 200 {object} listShelvesResponse
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /shelves [get]
func (h *Handler) ListShelves(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var bookID *uuid.UUID
	if raw := c.QueryParam("book_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err == nil {
			bookID = &id
		}
	}

	shelves, err := h.service.ListShelves(ctx, userID, bookID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list shelves"})
	}

	items := make([]shelfListItemResponse, 0, len(shelves))
	for _, s := range shelves {
		item := shelfListItemResponse{
			ID:         s.ID.String(),
			Name:       s.Name,
			Type:       string(s.Type),
			BooksCount: s.BooksCount,
			CreatedAt:  s.CreatedAt,
		}
		if bookID != nil {
			item.HasBook = &s.HasBook
		}
		items = append(items, item)
	}

	return c.JSON(http.StatusOK, listShelvesResponse{
		Items: items,
		Total: len(items),
	})
}

// CreateShelf godoc
// @Summary Создать полку
// @Tags shelves
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createShelfRequest true "Название полки"
// @Success 201 {object} shelfResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /shelves [post]
func (h *Handler) CreateShelf(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var req createShelfRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	shelf, err := h.service.CreateShelf(ctx, userID, req.Name)
	if err != nil {
		if errors.Is(err, service.ErrInvalidName) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid shelf name"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create shelf"})
	}

	return c.JSON(http.StatusCreated, toShelfResponse(*shelf))
}

// GetShelf godoc
// @Summary Получить полку с книгами
// @Tags shelves
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID полки"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} shelfDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /shelves/{id} [get]
func (h *Handler) GetShelf(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	shelfID, err := parseShelfID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid shelf id"})
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	shelf, books, total, err := h.service.GetShelf(ctx, userID, shelfID, page, limit)
	if err != nil {
		if errors.Is(err, service.ErrShelfNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "shelf not found"})
		}
		if errors.Is(err, service.ErrForbidden) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get shelf"})
	}

	items := make([]bookListItemResponse, 0, len(books))
	for _, b := range books {
		items = append(items, toBookListItemResponse(b))
	}

	return c.JSON(http.StatusOK, shelfDetailResponse{
		Shelf: toShelfResponse(*shelf),
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// GetPublicShelf godoc
// @Summary Получить публичную полку с книгами
// @Description Доступно без авторизации; полки не приватны, проверка владельца не выполняется
// @Tags shelves
// @Produce json
// @Param id path string true "ID полки"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} shelfDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /shelves/public/{id} [get]
func (h *Handler) GetPublicShelf(c echo.Context) error {
	ctx := c.Request().Context()

	shelfID, err := parseShelfID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid shelf id"})
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	shelf, books, total, err := h.service.GetPublicShelf(ctx, shelfID, page, limit)
	if err != nil {
		if errors.Is(err, service.ErrShelfNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "shelf not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get shelf"})
	}

	items := make([]bookListItemResponse, 0, len(books))
	for _, b := range books {
		items = append(items, toBookListItemResponse(b))
	}

	return c.JSON(http.StatusOK, shelfDetailResponse{
		Shelf: toShelfResponse(*shelf),
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// RenameShelf godoc
// @Summary Переименовать полку
// @Tags shelves
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID полки"
// @Param body body renameShelfRequest true "Новое название"
// @Success 200 {object} shelfResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /shelves/{id} [patch]
func (h *Handler) RenameShelf(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	shelfID, err := parseShelfID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid shelf id"})
	}

	var req renameShelfRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	shelf, err := h.service.RenameShelf(ctx, userID, shelfID, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrShelfNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "shelf not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, service.ErrDefaultShelfImmutable), errors.Is(err, service.ErrInvalidName):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to rename shelf"})
		}
	}

	return c.JSON(http.StatusOK, toShelfResponse(*shelf))
}

// DeleteShelf godoc
// @Summary Удалить полку
// @Tags shelves
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID полки"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /shelves/{id} [delete]
func (h *Handler) DeleteShelf(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	shelfID, err := parseShelfID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid shelf id"})
	}

	if err := h.service.DeleteShelf(ctx, userID, shelfID); err != nil {
		switch {
		case errors.Is(err, service.ErrShelfNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "shelf not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, service.ErrDefaultShelfImmutable):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete shelf"})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// AddBook godoc
// @Summary Добавить книгу на полку
// @Tags shelves
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID полки"
// @Param body body addBookRequest true "ID книги"
// @Success 201 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /shelves/{id}/books [post]
func (h *Handler) AddBook(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	shelfID, err := parseShelfID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid shelf id"})
	}

	var req addBookRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	bookID, err := uuid.Parse(req.BookID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book id"})
	}

	if err := h.service.AddBook(ctx, userID, shelfID, bookID); err != nil {
		switch {
		case errors.Is(err, service.ErrShelfNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "shelf not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, service.ErrBookNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not found"})
		case errors.Is(err, service.ErrBookAlreadyOnShelf):
			return c.JSON(http.StatusConflict, map[string]string{"error": "book already on shelf"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to add book"})
		}
	}

	return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
}

// RemoveBook godoc
// @Summary Убрать книгу с полки
// @Tags shelves
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID полки"
// @Param bookId path string true "ID книги"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /shelves/{id}/books/{bookId} [delete]
func (h *Handler) RemoveBook(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	shelfID, err := parseShelfID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid shelf id"})
	}

	bookID, err := uuid.Parse(c.Param("bookId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book id"})
	}

	if err := h.service.RemoveBook(ctx, userID, shelfID, bookID); err != nil {
		switch {
		case errors.Is(err, service.ErrShelfNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "shelf not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, service.ErrBookNotOnShelf):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not on shelf"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to remove book"})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
