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

// AuthorResponse is the author shape embedded in book list items. Slug is
// null (never omitted) for a non-published author, which is then embedded
// by name only - the signal for clients to render plain text, no link.
type AuthorResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Slug      *string `json:"slug"`
	PhotoURL  *string `json:"photo_url,omitempty"`
	BirthDate *string `json:"birth_date,omitempty"`
}

// GenreResponse is the genre shape embedded in book list items.
type GenreResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// BookListItemResponse is the shared book card shape. It is exported so
// other modules (home) render books with the exact same JSON.
type BookListItemResponse struct {
	ID            string           `json:"id"`
	Title         string           `json:"title"`
	Slug          string           `json:"slug"`
	CoverURL      *string          `json:"cover_url,omitempty"`
	PublishedYear *int             `json:"published_year,omitempty"`
	Rating        float64          `json:"rating"`
	RatingsCount  int              `json:"ratings_count"`
	Authors       []AuthorResponse `json:"authors"`
	Genres        []GenreResponse  `json:"genres"`
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
	Authors       []AuthorResponse `json:"authors"`
	Genres        []GenreResponse  `json:"genres"`
}

type authorDetailResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	Biography *string `json:"biography,omitempty"`
	PhotoURL  *string `json:"photo_url,omitempty"`
	BirthDate *string `json:"birth_date,omitempty"`
	DeathDate *string `json:"death_date,omitempty"`
	// PhotoAttribution is the Wikimedia Commons attribution of the photo
	// (always present; null unless photo_url and the Commons source page
	// are both set).
	PhotoAttribution *PhotoAttributionResponse `json:"photo_attribution"`
	Books            []BookListItemResponse    `json:"books"`
}

// PhotoAttributionResponse is the Commons attribution of an author photo
// (docs/features/author-wikidata-enrichment.md).
type PhotoAttributionResponse struct {
	Author     *string `json:"author"`
	License    string  `json:"license"`
	LicenseURL *string `json:"license_url"`
	SourceURL  string  `json:"source_url"`
}

func toPhotoAttributionResponse(a domain.Author) *PhotoAttributionResponse {
	if a.PhotoURL == nil || *a.PhotoURL == "" || a.PhotoSourceURL == nil || *a.PhotoSourceURL == "" {
		return nil
	}
	resp := &PhotoAttributionResponse{
		Author:     a.PhotoAuthor,
		LicenseURL: a.PhotoLicenseURL,
		SourceURL:  *a.PhotoSourceURL,
	}
	if a.PhotoLicense != nil {
		resp.License = *a.PhotoLicense
	}
	return resp
}

type listBooksResponse struct {
	Items []BookListItemResponse `json:"items"`
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
}

type similarBooksResponse struct {
	Items []BookListItemResponse `json:"items"`
}

type listAuthorsResponse struct {
	Items []AuthorResponse `json:"items"`
	Total int64            `json:"total"`
}

type languageResponse struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type listLanguagesResponse struct {
	Items []languageResponse `json:"items"`
}

func formatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func toAuthorResponse(a domain.Author) AuthorResponse {
	id := ""
	if a.ID != uuid.Nil {
		id = a.ID.String()
	}
	if !a.IsPublished() {
		return AuthorResponse{ID: id, Name: a.Name}
	}
	slug := a.Slug
	return AuthorResponse{
		ID:        id,
		Name:      a.Name,
		Slug:      &slug,
		PhotoURL:  a.PhotoURL,
		BirthDate: formatDate(a.BirthDate),
	}
}

func toGenreResponse(g domain.Genre) GenreResponse {
	id := ""
	if g.ID != uuid.Nil {
		id = g.ID.String()
	}
	return GenreResponse{
		ID:   id,
		Name: g.Name,
		Slug: g.Slug,
	}
}

// ToBookListItemResponse maps a catalog book to BookListItemResponse.
func ToBookListItemResponse(b domain.Book) BookListItemResponse {
	authors := make([]AuthorResponse, 0, len(b.Authors))
	for _, a := range b.Authors {
		authors = append(authors, toAuthorResponse(a))
	}

	// Only published genres are public (docs/features/genre-status.md).
	publishedGenres := domain.PublishedGenres(b.Genres)
	genres := make([]GenreResponse, 0, len(publishedGenres))
	for _, g := range publishedGenres {
		genres = append(genres, toGenreResponse(g))
	}

	return BookListItemResponse{
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
	authors := make([]AuthorResponse, 0, len(b.Authors))
	for _, a := range b.Authors {
		authors = append(authors, toAuthorResponse(a))
	}

	// Only published genres are public (docs/features/genre-status.md).
	publishedGenres := domain.PublishedGenres(b.Genres)
	genres := make([]GenreResponse, 0, len(publishedGenres))
	for _, g := range publishedGenres {
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
	books := make([]BookListItemResponse, 0, len(a.Books))
	for _, b := range a.Books {
		books = append(books, ToBookListItemResponse(b))
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

		PhotoAttribution: toPhotoAttributionResponse(a),
	}
}

// ListBooks godoc
// @Summary Список книг
// @Description Возвращает постраничный список книг с фильтрацией и сортировкой
// @Tags catalog
// @Produce json
// @Param search query string false "Поиск по названию/описанию"
// @Param genre_id query []string false "Фильтр по жанру (можно указать несколько)"
// @Param author_id query string false "Фильтр по автору (UUID)"
// @Param rating_min query int false "Минимальный рейтинг (1-5)"
// @Param language query string false "Фильтр по языку"
// @Param year_from query int false "Год издания от"
// @Param year_to query int false "Год издания до"
// @Param sort query string false "Сортировка"
// @Param page query int false "Номер страницы (по умолчанию 1)"
// @Param limit query int false "Размер страницы, максимум 100 (по умолчанию 20)"
// @Success 200 {object} listBooksResponse
// @Failure 500 {object} map[string]string
// @Router /books [get]
func (h *Handler) ListBooks(c echo.Context) error {
	ctx := c.Request().Context()

	filter := repository.BookFilter{
		Search: c.QueryParam("search"),
		Sort:   c.QueryParam("sort"),
	}

	// Parse multi-value genre_id params
	genreIDs := c.QueryParams()["genre_id"]
	for _, gid := range genreIDs {
		id, err := uuid.Parse(gid)
		if err == nil {
			filter.GenreID = append(filter.GenreID, id)
		}
	}

	if authorID := c.QueryParam("author_id"); authorID != "" {
		id, err := uuid.Parse(authorID)
		if err == nil {
			filter.AuthorID = &id
		}
	}

	if ratingMin := c.QueryParam("rating_min"); ratingMin != "" {
		val, err := strconv.Atoi(ratingMin)
		if err == nil && val >= 1 && val <= 5 {
			filter.RatingMin = &val
		}
	}

	if language := c.QueryParam("language"); language != "" {
		filter.Language = &language
	}

	if yearFrom := c.QueryParam("year_from"); yearFrom != "" {
		val, err := strconv.Atoi(yearFrom)
		if err == nil {
			filter.YearFrom = &val
		}
	}

	if yearTo := c.QueryParam("year_to"); yearTo != "" {
		val, err := strconv.Atoi(yearTo)
		if err == nil {
			filter.YearTo = &val
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

	items := make([]BookListItemResponse, 0, len(books))
	for _, b := range books {
		items = append(items, ToBookListItemResponse(b))
	}

	return c.JSON(http.StatusOK, listBooksResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// GetBook godoc
// @Summary Получить книгу по slug
// @Tags catalog
// @Produce json
// @Param slug path string true "Slug книги"
// @Success 200 {object} bookResponse
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{slug} [get]
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

// ListSimilarBooks godoc
// @Summary Похожие книги
// @Description Возвращает книги, похожие на указанную (общие авторы и жанры). Не более 2 книг одного автора.
// @Tags catalog
// @Produce json
// @Param id path string true "ID книги (UUID)"
// @Param limit query int false "Количество, максимум 20 (по умолчанию 8)"
// @Success 200 {object} similarBooksResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{id}/similar [get]
func (h *Handler) ListSimilarBooks(c echo.Context) error {
	ctx := c.Request().Context()

	bookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book id"})
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = service.DefaultSimilarLimit
	}
	if limit > service.MaxSimilarLimit {
		limit = service.MaxSimilarLimit
	}

	books, err := h.service.ListSimilarBooks(ctx, bookID, limit)
	if err != nil {
		if errors.Is(err, service.ErrBookNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get similar books"})
	}

	items := make([]BookListItemResponse, 0, len(books))
	for _, b := range books {
		items = append(items, ToBookListItemResponse(b))
	}

	return c.JSON(http.StatusOK, similarBooksResponse{Items: items})
}

// GetAuthor godoc
// @Summary Получить автора по slug
// @Tags catalog
// @Produce json
// @Param slug path string true "Slug автора"
// @Success 200 {object} authorDetailResponse
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /authors/{slug} [get]
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

// ListAuthors godoc
// @Summary Список авторов
// @Tags catalog
// @Produce json
// @Param search query string false "Поиск по имени"
// @Param limit query int false "Размер страницы (по умолчанию 20)"
// @Success 200 {object} listAuthorsResponse
// @Failure 500 {object} map[string]string
// @Router /authors [get]
func (h *Handler) ListAuthors(c echo.Context) error {
	ctx := c.Request().Context()
	search := c.QueryParam("search")

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 20
	}

	authors, total, err := h.service.ListAuthors(ctx, search, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list authors"})
	}

	items := make([]AuthorResponse, 0, len(authors))
	for _, a := range authors {
		items = append(items, toAuthorResponse(a))
	}

	return c.JSON(http.StatusOK, listAuthorsResponse{
		Items: items,
		Total: total,
	})
}

// ListGenres godoc
// @Summary Список жанров
// @Tags catalog
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /genres [get]
func (h *Handler) ListGenres(c echo.Context) error {
	ctx := c.Request().Context()

	genres, err := h.service.ListGenres(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list genres"})
	}

	items := make([]GenreResponse, 0, len(genres))
	for _, g := range genres {
		items = append(items, toGenreResponse(g))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"items": items,
		"total": len(items),
	})
}

// ListLanguages godoc
// @Summary Список языков, представленных в каталоге
// @Tags catalog
// @Produce json
// @Success 200 {object} listLanguagesResponse
// @Failure 500 {object} map[string]string
// @Router /languages [get]
func (h *Handler) ListLanguages(c echo.Context) error {
	ctx := c.Request().Context()

	languages, err := h.service.ListLanguages(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list languages"})
	}

	items := make([]languageResponse, 0, len(languages))
	for _, code := range languages {
		name := code
		switch code {
		case "ru":
			name = "Русский"
		case "en":
			name = "English"
		}
		items = append(items, languageResponse{
			Code: code,
			Name: name,
		})
	}

	return c.JSON(http.StatusOK, listLanguagesResponse{
		Items: items,
	})
}
