package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/lists/domain"
	"github.com/bookhaven/backend/internal/modules/lists/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles lists HTTP requests.
type Handler struct {
	service *service.Service
}

// NewHandler creates a new lists Handler.
func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

type createListRequest struct {
	Title       string  `json:"title" validate:"required,min=2,max=255"`
	Description *string `json:"description,omitempty"`
	IsPublic    *bool   `json:"is_public,omitempty"`
}

type updateListRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	IsPublic    *bool   `json:"is_public,omitempty"`
}

type addBookRequest struct {
	BookID string `json:"book_id" validate:"required,uuid"`
}

type reorderBooksRequest struct {
	BookIDs []string `json:"book_ids" validate:"required"`
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

type listResponse struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	BooksCount  int64     `json:"books_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type listListItemResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	BooksCount  int64     `json:"books_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type listListsResponse struct {
	Items       []listListItemResponse `json:"items"`
	Total       int                    `json:"total"`
	BookInLists []string               `json:"book_in_lists,omitempty"`
}

type listDetailResponse struct {
	List  listResponse           `json:"list"`
	Items []bookListItemResponse `json:"items"`
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
}

type publicListResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	BooksCount  int64     `json:"books_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type publicListDetailResponse struct {
	List  publicListResponse     `json:"list"`
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

	genres := make([]genreResponse, 0, len(b.Genres))
	for _, g := range b.Genres {
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

func toListResponse(l domain.List) listResponse {
	return listResponse{
		ID:          l.ID.String(),
		UserID:      l.UserID.String(),
		Title:       l.Title,
		Description: l.Description,
		IsPublic:    l.IsPublic,
		BooksCount:  l.BooksCount,
		CreatedAt:   l.CreatedAt,
		UpdatedAt:   l.UpdatedAt,
	}
}

func toPublicListResponse(l domain.List) publicListResponse {
	return publicListResponse{
		ID:          l.ID.String(),
		Title:       l.Title,
		Description: l.Description,
		IsPublic:    l.IsPublic,
		BooksCount:  l.BooksCount,
		CreatedAt:   l.CreatedAt,
		UpdatedAt:   l.UpdatedAt,
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

func parseListID(c echo.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

// ListLists godoc
// @Summary Список списков пользователя
// @Tags lists
// @Produce json
// @Security BearerAuth
// @Param book_id query string false "Если указан, возвращается book_in_lists с ID списков, содержащих книгу"
// @Success 200 {object} listListsResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /lists [get]
func (h *Handler) ListLists(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var bookID *uuid.UUID
	if raw := c.QueryParam("book_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book_id"})
		}
		bookID = &parsed
	}

	lists, inLists, err := h.service.ListLists(ctx, userID, bookID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list lists"})
	}

	items := make([]listListItemResponse, 0, len(lists))
	for _, l := range lists {
		items = append(items, listListItemResponse{
			ID:          l.ID.String(),
			Title:       l.Title,
			Description: l.Description,
			IsPublic:    l.IsPublic,
			BooksCount:  l.BooksCount,
			CreatedAt:   l.CreatedAt,
		})
	}

	resp := listListsResponse{
		Items: items,
		Total: len(items),
	}
	if inLists != nil {
		resp.BookInLists = make([]string, 0, len(inLists))
		for id := range inLists {
			resp.BookInLists = append(resp.BookInLists, id.String())
		}
	}

	return c.JSON(http.StatusOK, resp)
}

// CreateList godoc
// @Summary Создать список
// @Tags lists
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createListRequest true "Данные списка"
// @Success 201 {object} listResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /lists [post]
func (h *Handler) CreateList(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var req createListRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	isPublic := true
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	list, err := h.service.CreateList(ctx, userID, req.Title, req.Description, isPublic)
	if err != nil {
		if errors.Is(err, service.ErrInvalidTitle) || errors.Is(err, service.ErrInvalidDescription) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create list"})
	}

	return c.JSON(http.StatusCreated, toListResponse(*list))
}

// GetList godoc
// @Summary Получить список с книгами
// @Tags lists
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID списка"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} listDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /lists/{id} [get]
func (h *Handler) GetList(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	listID, err := parseListID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid list id"})
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

	list, books, total, err := h.service.GetList(ctx, userID, listID, page, limit)
	if err != nil {
		if errors.Is(err, service.ErrListNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "list not found"})
		}
		if errors.Is(err, service.ErrForbidden) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get list"})
	}

	items := make([]bookListItemResponse, 0, len(books))
	for _, b := range books {
		items = append(items, toBookListItemResponse(b))
	}

	return c.JSON(http.StatusOK, listDetailResponse{
		List:  toListResponse(*list),
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// UpdateList godoc
// @Summary Обновить список
// @Tags lists
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID списка"
// @Param body body updateListRequest true "Поля для обновления"
// @Success 200 {object} listResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /lists/{id} [patch]
func (h *Handler) UpdateList(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	listID, err := parseListID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid list id"})
	}

	var req updateListRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	list, err := h.service.UpdateList(ctx, userID, listID, req.Title, req.Description, req.IsPublic)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrListNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "list not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, service.ErrInvalidTitle), errors.Is(err, service.ErrInvalidDescription):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update list"})
		}
	}

	return c.JSON(http.StatusOK, toListResponse(*list))
}

// DeleteList godoc
// @Summary Удалить список
// @Tags lists
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID списка"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /lists/{id} [delete]
func (h *Handler) DeleteList(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	listID, err := parseListID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid list id"})
	}

	if err := h.service.DeleteList(ctx, userID, listID); err != nil {
		switch {
		case errors.Is(err, service.ErrListNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "list not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete list"})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// AddBook godoc
// @Summary Добавить книгу в список
// @Tags lists
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID списка"
// @Param body body addBookRequest true "ID книги"
// @Success 201 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /lists/{id}/books [post]
func (h *Handler) AddBook(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	listID, err := parseListID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid list id"})
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

	if err := h.service.AddBook(ctx, userID, listID, bookID); err != nil {
		switch {
		case errors.Is(err, service.ErrListNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "list not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, service.ErrBookNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not found"})
		case errors.Is(err, service.ErrBookAlreadyOnList):
			return c.JSON(http.StatusConflict, map[string]string{"error": "book already on list"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to add book"})
		}
	}

	return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
}

// RemoveBook godoc
// @Summary Убрать книгу из списка
// @Tags lists
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID списка"
// @Param bookId path string true "ID книги"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /lists/{id}/books/{bookId} [delete]
func (h *Handler) RemoveBook(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	listID, err := parseListID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid list id"})
	}

	bookID, err := uuid.Parse(c.Param("bookId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book id"})
	}

	if err := h.service.RemoveBook(ctx, userID, listID, bookID); err != nil {
		switch {
		case errors.Is(err, service.ErrListNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "list not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, service.ErrBookNotOnList):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not on list"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to remove book"})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ReorderBooks godoc
// @Summary Изменить порядок книг в списке
// @Tags lists
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID списка"
// @Param body body reorderBooksRequest true "Новый порядок ID книг"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /lists/{id}/books/reorder [patch]
func (h *Handler) ReorderBooks(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	listID, err := parseListID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid list id"})
	}

	var req reorderBooksRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	bookIDs := make([]uuid.UUID, 0, len(req.BookIDs))
	for _, raw := range req.BookIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book id"})
		}
		bookIDs = append(bookIDs, id)
	}

	if err := h.service.ReorderBooks(ctx, userID, listID, bookIDs); err != nil {
		switch {
		case errors.Is(err, service.ErrListNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "list not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to reorder books"})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// GetPublicList godoc
// @Summary Получить публичный список с книгами
// @Description Доступно без авторизации, если список публичный
// @Tags lists
// @Produce json
// @Param id path string true "ID списка"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} publicListDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /lists/public/{id} [get]
func (h *Handler) GetPublicList(c echo.Context) error {
	ctx := c.Request().Context()

	listID, err := parseListID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid list id"})
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

	list, books, total, err := h.service.GetPublicList(ctx, listID, page, limit)
	if err != nil {
		if errors.Is(err, service.ErrListNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "list not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get list"})
	}

	items := make([]bookListItemResponse, 0, len(books))
	for _, b := range books {
		items = append(items, toBookListItemResponse(b))
	}

	return c.JSON(http.StatusOK, publicListDetailResponse{
		List:  toPublicListResponse(*list),
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}
