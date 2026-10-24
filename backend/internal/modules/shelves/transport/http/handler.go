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
	Name        string  `json:"name" validate:"required,min=2,max=100"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	IsPublic    *bool   `json:"is_public"`
}

type updateShelfRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=2,max=100"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	IsPublic    *bool   `json:"is_public"`
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

// shelfBookItemResponse is a shelf book item with the shelf owner's reading
// counter (finished reads count and the latest finish date).
type shelfBookItemResponse struct {
	bookListItemResponse
	ReadsCount int64      `json:"reads_count"`
	LastReadAt *time.Time `json:"last_read_at"`
}

// readsResponse is the caller's reading summary of one book.
type readsResponse struct {
	ReadsCount int64      `json:"reads_count"`
	LastReadAt *time.Time `json:"last_read_at"`
	Rereading  bool       `json:"rereading"`
}

type shelfResponse struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	BooksCount  int64     `json:"books_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type shelfListItemResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	BooksCount  int64     `json:"books_count"`
	HasBook     *bool     `json:"has_book,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type listShelvesResponse struct {
	Items []shelfListItemResponse `json:"items"`
	Total int                     `json:"total"`
	// Reads is the caller's reading summary of book_id; omitted without a valid book_id.
	Reads *readsResponse `json:"reads,omitempty"`
}

type shelfDetailResponse struct {
	Shelf shelfResponse           `json:"shelf"`
	Items []shelfBookItemResponse `json:"items"`
	Total int64                   `json:"total"`
	Page  int                     `json:"page"`
	Limit int                     `json:"limit"`
}

// toShelfBookItems builds shelf items with the read counters of the shelf owner.
func (h *Handler) toShelfBookItems(c echo.Context, ownerID uuid.UUID, books []catalogDomain.Book) ([]shelfBookItemResponse, error) {
	counts, err := h.service.ReadCounts(c.Request().Context(), ownerID, books)
	if err != nil {
		return nil, err
	}
	items := make([]shelfBookItemResponse, 0, len(books))
	for _, b := range books {
		rc := counts[b.ID]
		items = append(items, shelfBookItemResponse{
			bookListItemResponse: toBookListItemResponse(b),
			ReadsCount:           rc.Count,
			LastReadAt:           rc.LastReadAt,
		})
	}
	return items, nil
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
		CoverURL:      b.DisplayCoverURL(),
		PublishedYear: b.PublishedYear,
		Rating:        b.Rating,
		RatingsCount:  b.RatingsCount,
		Authors:       authors,
		Genres:        genres,
	}
}

func toShelfResponse(s domain.Shelf) shelfResponse {
	return shelfResponse{
		ID:          s.ID.String(),
		UserID:      s.UserID.String(),
		Name:        s.Name,
		Type:        string(s.Type),
		Description: s.Description,
		IsPublic:    s.IsPublic,
		BooksCount:  s.BooksCount,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
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
// @Description Дефолтные полки: «Хочу прочитать», «Читаю», «Прочитано», «Брошено» (создаются лениво). С book_id в ответ добавляется reads — сводка чтений этой книги вызывающим: reads_count (завершённые чтения), last_read_at, rereading (есть открытое чтение и хотя бы одно завершённое)
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
			ID:          s.ID.String(),
			Name:        s.Name,
			Type:        string(s.Type),
			Description: s.Description,
			IsPublic:    s.IsPublic,
			BooksCount:  s.BooksCount,
			CreatedAt:   s.CreatedAt,
		}
		if bookID != nil {
			item.HasBook = &s.HasBook
		}
		items = append(items, item)
	}

	resp := listShelvesResponse{Items: items, Total: len(items)}
	if bookID != nil {
		sum, err := h.service.ReadSummary(ctx, userID, *bookID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list shelves"})
		}
		resp.Reads = &readsResponse{ReadsCount: sum.Count, LastReadAt: sum.LastReadAt, Rereading: sum.Rereading}
	}

	return c.JSON(http.StatusOK, resp)
}

// CreateShelf godoc
// @Summary Создать полку
// @Tags shelves
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createShelfRequest true "Название, описание и приватность полки"
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

	shelf, err := h.service.CreateShelf(ctx, userID, req.Name, req.Description, req.IsPublic)
	if err != nil {
		if errors.Is(err, service.ErrInvalidName) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid shelf name"})
		}
		if errors.Is(err, service.ErrInvalidDescription) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid shelf description"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create shelf"})
	}

	return c.JSON(http.StatusCreated, toShelfResponse(*shelf))
}

// GetShelf godoc
// @Summary Получить полку с книгами
// @Description Каждая книга несёт reads_count и last_read_at — завершённые чтения владельца полки
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

	items, err := h.toShelfBookItems(c, shelf.UserID, books)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get shelf"})
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
// @Description Доступно без авторизации. Приватная полка возвращает 404 (в том числе владельцу); владелец управляет ею через GET /shelves/{id}. reads_count и last_read_at считаются для владельца полки, не для вызывающего
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

	items, err := h.toShelfBookItems(c, shelf.UserID, books)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get shelf"})
	}

	return c.JSON(http.StatusOK, shelfDetailResponse{
		Shelf: toShelfResponse(*shelf),
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// UpdateShelf godoc
// @Summary Изменить полку
// @Description Частичное обновление: название, описание, приватность. Дефолтные полки изменить нельзя (400)
// @Tags shelves
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID полки"
// @Param body body updateShelfRequest true "Изменяемые поля"
// @Success 200 {object} shelfResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /shelves/{id} [patch]
func (h *Handler) UpdateShelf(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	shelfID, err := parseShelfID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid shelf id"})
	}

	var req updateShelfRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	shelf, err := h.service.UpdateShelf(ctx, userID, shelfID, service.UpdateShelfInput{Name: req.Name, Description: req.Description, IsPublic: req.IsPublic})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrShelfNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "shelf not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, service.ErrDefaultShelfImmutable), errors.Is(err, service.ErrInvalidName),
			errors.Is(err, service.ErrInvalidDescription), errors.Is(err, service.ErrNothingToUpdate):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update shelf"})
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
// @Description Дефолтные полки эксклюзивны: книга снимается с остальных. Журнал чтений обновляется в той же транзакции: «Читаю» открывает чтение (повторное нажатие после «Прочитано» — перечитывание, прежние чтения сохраняются), «Прочитано» закрывает открытое чтение как завершённое, «Брошено» — как брошенное, «Хочу прочитать» удаляет открытое
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
// @Description Снятие с «Прочитано» — отмена: удаляется только последнее завершённое чтение, история остаётся. Снятие с «Читаю» удаляет открытое чтение
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
