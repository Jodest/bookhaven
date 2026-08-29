package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/catalog/repository"
	"github.com/bookhaven/backend/internal/modules/catalog/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles catalog HTTP requests.
type Handler struct {
	service *service.Service
}

// NewHandler creates a new catalog Handler.
func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

type authorResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
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
	Authors       []authorResponse `json:"authors"`
	Genres        []genreResponse  `json:"genres"`
}

type authorDetailResponse struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Slug      string                 `json:"slug"`
	Biography *string                `json:"biography,omitempty"`
	PhotoURL  *string                `json:"photo_url,omitempty"`
	BirthDate *string                `json:"birth_date,omitempty"`
	DeathDate *string                `json:"death_date,omitempty"`
	Books     []bookListItemResponse `json:"books"`
}

type listBooksResponse struct {
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

func toAuthorResponse(a domain.Author) authorResponse {
	id := ""
	if a.ID != uuid.Nil {
		id = a.ID.String()
	}
	return authorResponse{
		ID:        id,
		Name:      a.Name,
		Slug:      a.Slug,
		PhotoURL:  a.PhotoURL,
		BirthDate: formatDate(a.BirthDate),
	}
}

func toGenreResponse(g domain.Genre) genreResponse {
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

func toBookListItemResponse(b domain.Book) bookListItemResponse {
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

func toBookResponse(b domain.Book) bookResponse {
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
		Authors:       authors,
		Genres:        genres,
	}
}

func toAuthorDetailResponse(a domain.Author) authorDetailResponse {
	books := make([]bookListItemResponse, 0, len(a.Books))
	for _, b := range a.Books {
		books = append(books, toBookListItemResponse(b))
	}

	id := ""
	if a.ID != uuid.Nil {
		id = a.ID.String()
	}

	return authorDetailResponse{
		ID:        id,
		Name:      a.Name,
		Slug:      a.Slug,
		Biography: a.Biography,
		PhotoURL:  a.PhotoURL,
		BirthDate: formatDate(a.BirthDate),
		DeathDate: formatDate(a.DeathDate),
		Books:     books,
	}
}

// ListBooks handles GET /api/v1/books.
func (h *Handler) ListBooks(c echo.Context) error {
	ctx := c.Request().Context()

	filter := repository.BookFilter{
		Search: c.QueryParam("search"),
	}

	if genreID := c.QueryParam("genre_id"); genreID != "" {
		id, err := uuid.Parse(genreID)
		if err == nil {
			filter.GenreID = &id
		}
	}

	if authorID := c.QueryParam("author_id"); authorID != "" {
		id, err := uuid.Parse(authorID)
		if err == nil {
			filter.AuthorID = &id
		}
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
	filter.Limit = limit
	filter.Offset = (page - 1) * limit

	books, total, err := h.service.ListBooks(ctx, filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list books"})
	}

	items := make([]bookListItemResponse, 0, len(books))
	for _, b := range books {
		items = append(items, toBookListItemResponse(b))
	}

	return c.JSON(http.StatusOK, listBooksResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// GetBook handles GET /api/v1/books/:slug.
func (h *Handler) GetBook(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	book, err := h.service.GetBookBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, service.ErrBookNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get book"})
	}

	return c.JSON(http.StatusOK, toBookResponse(*book))
}

// GetAuthor handles GET /api/v1/authors/:slug.
func (h *Handler) GetAuthor(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	author, err := h.service.GetAuthorBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, service.ErrAuthorNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "author not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get author"})
	}

	return c.JSON(http.StatusOK, toAuthorDetailResponse(*author))
}

// ListGenres handles GET /api/v1/genres.
func (h *Handler) ListGenres(c echo.Context) error {
	ctx := c.Request().Context()

	genres, err := h.service.ListGenres(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list genres"})
	}

	items := make([]genreResponse, 0, len(genres))
	for _, g := range genres {
		items = append(items, toGenreResponse(g))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"items": items,
		"total": len(items),
	})
}
