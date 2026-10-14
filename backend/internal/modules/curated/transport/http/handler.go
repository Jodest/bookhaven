package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/curated/domain"
	"github.com/bookhaven/backend/internal/modules/curated/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles curated collection HTTP requests (public and admin).
type Handler struct {
	service *service.Service
}

// NewHandler creates a Handler.
func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

// ---- response shapes ----

// authorResponse: Slug is null (never omitted) for a non-published author,
// embedded by name only (same rule as the lists module).
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

type ruleResponse struct {
	Metric     string `json:"metric"`
	PeriodDays *int   `json:"period_days"`
	MinRatings *int   `json:"min_ratings"`
	Confidence *int   `json:"confidence"`
}

type movementResponse struct {
	Direction string `json:"direction"`
	By        int    `json:"by"`
}

type collectionSummaryResponse struct {
	ID          string         `json:"id"`
	Slug        string         `json:"slug"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Kind        string         `json:"kind"`
	Size        int            `json:"size"`
	ShownCount  int            `json:"shown_count"`
	Source      *string        `json:"source"`
	SourceURL   *string        `json:"source_url"`
	Genre       *genreResponse `json:"genre"`
	CoverURLs   []string       `json:"cover_urls"`
	Rule        *ruleResponse  `json:"rule"`
	ComputedAt  *time.Time     `json:"computed_at"`
}

type collectionIndexResponse struct {
	Items []collectionSummaryResponse `json:"items"`
	Total int                         `json:"total"`
}

type placedBookResponse struct {
	Place    int                  `json:"place"`
	Movement *movementResponse    `json:"movement"`
	Book     bookListItemResponse `json:"book"`
}

type collectionDetailResponse struct {
	Collection collectionSummaryResponse `json:"collection"`
	Items      []placedBookResponse      `json:"items"`
	Total      int                       `json:"total"`
	Page       int                       `json:"page"`
	Limit      int                       `json:"limit"`
}

type adminGenreResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Status string `json:"status"`
}

type adminCollectionResponse struct {
	ID          string              `json:"id"`
	Slug        string              `json:"slug"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Kind        string              `json:"kind"`
	Size        int                 `json:"size"`
	Source      *string             `json:"source"`
	SourceURL   *string             `json:"source_url"`
	Genre       *adminGenreResponse `json:"genre"`
	IsPublished bool                `json:"is_published"`
	PublishedAt *time.Time          `json:"published_at"`
	Rule        *ruleResponse       `json:"rule"`
	ComputedAt  *time.Time          `json:"computed_at"`
	BaselineAt  *time.Time          `json:"baseline_at"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

type adminListItemResponse struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Kind        string     `json:"kind"`
	Size        int        `json:"size"`
	IsPublished bool       `json:"is_published"`
	BooksCount  int        `json:"books_count"`
	ShownCount  int        `json:"shown_count"`
	ComputedAt  *time.Time `json:"computed_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type adminListResponse struct {
	Items []adminListItemResponse `json:"items"`
	Total int                     `json:"total"`
}

type adminAuthorResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type adminBookResponse struct {
	ID       string                `json:"id"`
	Title    string                `json:"title"`
	Slug     string                `json:"slug"`
	Status   string                `json:"status"`
	CoverURL *string               `json:"cover_url"`
	Authors  []adminAuthorResponse `json:"authors"`
}

type adminItemResponse struct {
	Position int               `json:"position"`
	Place    *int              `json:"place"`
	State    string            `json:"state"`
	Score    *float64          `json:"score"`
	Movement *movementResponse `json:"movement"`
	Book     adminBookResponse `json:"book"`
}

type adminDetailResponse struct {
	adminCollectionResponse
	BooksCount int                 `json:"books_count"`
	ShownCount int                 `json:"shown_count"`
	Items      []adminItemResponse `json:"items"`
}

// ---- request shapes ----

type ruleRequest struct {
	Metric     string `json:"metric"`
	PeriodDays *int   `json:"period_days"`
	MinRatings *int   `json:"min_ratings"`
	Confidence *int   `json:"confidence"`
}

func (r *ruleRequest) toDomain() *domain.Rule {
	if r == nil {
		return nil
	}
	return &domain.Rule{Metric: r.Metric, PeriodDays: r.PeriodDays, MinRatings: r.MinRatings, Confidence: r.Confidence}
}

type createRequest struct {
	Kind        string       `json:"kind"`
	Rule        *ruleRequest `json:"rule"`
	Title       string       `json:"title"`
	Size        int          `json:"size"`
	Slug        string       `json:"slug"`
	Description string       `json:"description"`
	Source      string       `json:"source"`
	SourceURL   string       `json:"source_url"`
	GenreID     *string      `json:"genre_id"`
	IsPublished bool         `json:"is_published"`
}

// updateRequest: GenreID is raw so an explicit null (clear) differs from an
// absent field (leave as is).
type updateRequest struct {
	Kind        *string         `json:"kind"`
	Rule        *ruleRequest    `json:"rule"`
	Title       *string         `json:"title"`
	Size        *int            `json:"size"`
	Slug        *string         `json:"slug"`
	Description *string         `json:"description"`
	Source      *string         `json:"source"`
	SourceURL   *string         `json:"source_url"`
	GenreID     json.RawMessage `json:"genre_id"`
	IsPublished *bool           `json:"is_published"`
}

type addBookRequest struct {
	BookID   string `json:"book_id"`
	Position *int   `json:"position"`
}

type moveBookRequest struct {
	Position int `json:"position"`
}

// ---- mappers ----

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
	return authorResponse{ID: id, Name: a.Name, Slug: &slug, PhotoURL: a.PhotoURL, BirthDate: formatDate(a.BirthDate)}
}

func toGenreResponse(g catalogDomain.Genre) genreResponse {
	id := ""
	if g.ID != uuid.Nil {
		id = g.ID.String()
	}
	return genreResponse{ID: id, Name: g.Name, Slug: g.Slug}
}

func toBookListItemResponse(b catalogDomain.Book) bookListItemResponse {
	authors := make([]authorResponse, 0, len(b.Authors))
	for _, a := range b.Authors {
		authors = append(authors, toAuthorResponse(a))
	}
	published := catalogDomain.PublishedGenres(b.Genres)
	genres := make([]genreResponse, 0, len(published))
	for _, g := range published {
		genres = append(genres, toGenreResponse(g))
	}
	return bookListItemResponse{
		ID: b.ID.String(), Title: b.Title, Slug: b.Slug, CoverURL: b.CoverURL,
		PublishedYear: b.PublishedYear, Rating: b.Rating, RatingsCount: b.RatingsCount,
		Authors: authors, Genres: genres,
	}
}

func toRuleResponse(c *domain.Collection) *ruleResponse {
	r := domain.RuleOf(c)
	if r == nil {
		return nil
	}
	return &ruleResponse{Metric: r.Metric, PeriodDays: r.PeriodDays, MinRatings: r.MinRatings, Confidence: r.Confidence}
}

func toMovementResponse(m *domain.MovementInfo) *movementResponse {
	if m == nil {
		return nil
	}
	return &movementResponse{Direction: m.Direction, By: m.By}
}

func toSummaryResponse(s service.Summary) collectionSummaryResponse {
	c := s.Collection
	var genre *genreResponse
	// Public output shows the genre only when it is published.
	if c.Genre != nil && c.Genre.IsPublished() {
		g := toGenreResponse(*c.Genre)
		genre = &g
	}
	covers := s.CoverURLs
	if covers == nil {
		covers = []string{}
	}
	return collectionSummaryResponse{
		ID: c.ID.String(), Slug: c.Slug, Title: c.Title, Description: c.Description,
		Kind: c.Kind, Size: c.Size, ShownCount: s.ShownCount, Source: c.Source,
		SourceURL: c.SourceURL, Genre: genre, CoverURLs: covers,
		Rule: toRuleResponse(&c), ComputedAt: computedAt(c),
	}
}

// computedAt is the snapshot time of an automatic collection, else null.
func computedAt(c domain.Collection) *time.Time {
	if !c.IsAuto() {
		return nil
	}
	return c.ComputedAt
}

func baselineAt(c domain.Collection) *time.Time {
	if !c.IsAuto() {
		return nil
	}
	return c.BaselineAt
}

func toAdminCollectionResponse(c domain.Collection) adminCollectionResponse {
	var genre *adminGenreResponse
	if c.Genre != nil {
		genre = &adminGenreResponse{ID: c.Genre.ID.String(), Name: c.Genre.Name, Slug: c.Genre.Slug, Status: c.Genre.Status}
	}
	return adminCollectionResponse{
		ID: c.ID.String(), Slug: c.Slug, Title: c.Title, Description: c.Description,
		Kind: c.Kind, Size: c.Size, Source: c.Source, SourceURL: c.SourceURL, Genre: genre,
		IsPublished: c.IsPublished, PublishedAt: c.PublishedAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		Rule: toRuleResponse(&c), ComputedAt: computedAt(c), BaselineAt: baselineAt(c),
	}
}

type bookPlaceCollection struct {
	ID    uuid.UUID `json:"id"`
	Slug  string    `json:"slug"`
	Title string    `json:"title"`
	Kind  string    `json:"kind"`
	Size  int       `json:"size"`
}

type bookPlaceResponse struct {
	Place      int                 `json:"place"`
	Collection bookPlaceCollection `json:"collection"`
}

type bookPlacesResponse struct {
	Items []bookPlaceResponse `json:"items"`
	Total int                 `json:"total"`
}

func errJSON(c echo.Context, code int, msg string) error {
	return c.JSON(code, map[string]string{"error": msg})
}

// mapError turns service/domain errors into HTTP responses.
func mapError(c echo.Context, err error, fallback string) error {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		return errJSON(c, http.StatusBadRequest, ve.Error())
	case errors.Is(err, domain.ErrInvalidGenre):
		return errJSON(c, http.StatusBadRequest, "invalid genre_id")
	case errors.Is(err, domain.ErrInvalidPosition):
		return errJSON(c, http.StatusBadRequest, "invalid position")
	case errors.Is(err, domain.ErrCollectionNotFound):
		return errJSON(c, http.StatusNotFound, "collection not found")
	case errors.Is(err, domain.ErrBookNotFound):
		return errJSON(c, http.StatusNotFound, "book not found")
	case errors.Is(err, domain.ErrBookNotInList):
		return errJSON(c, http.StatusNotFound, "book not in collection")
	case errors.Is(err, domain.ErrSlugTaken):
		return errJSON(c, http.StatusConflict, "slug already exists")
	case errors.Is(err, domain.ErrBookAlreadyIn):
		return errJSON(c, http.StatusConflict, "book already in collection")
	case errors.Is(err, domain.ErrBookRejected):
		return errJSON(c, http.StatusUnprocessableEntity, "book is rejected")
	case errors.Is(err, domain.ErrTooManyLines):
		return errJSON(c, http.StatusUnprocessableEntity, "too many lines (max 500)")
	case errors.Is(err, domain.ErrCollectionAutomatic):
		return errJSON(c, http.StatusUnprocessableEntity, "collection is automatic")
	case errors.Is(err, domain.ErrNotAutomatic):
		return errJSON(c, http.StatusUnprocessableEntity, "collection is not automatic")
	case errors.Is(err, domain.ErrCollectionFull):
		return errJSON(c, http.StatusUnprocessableEntity, "collection is full")
	}
	return errJSON(c, http.StatusInternalServerError, fallback)
}

// ---- public ----

// ListPublic godoc
// @Summary Список опубликованных подборок
// @Description Доступно без авторизации. Сначала новые по дате первой публикации
// @Tags collections
// @Produce json
// @Param limit query int false "Сколько подборок вернуть (1-50), по умолчанию все; total - число всех опубликованных"
// @Success 200 {object} collectionIndexResponse
// @Failure 500 {object} map[string]string
// @Router /collections [get]
func (h *Handler) ListPublic(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	list, total, err := h.service.ListPublic(c.Request().Context(), limit)
	if err != nil {
		return mapError(c, err, "failed to list collections")
	}
	items := make([]collectionSummaryResponse, 0, len(list))
	for _, s := range list {
		items = append(items, toSummaryResponse(s))
	}
	return c.JSON(http.StatusOK, collectionIndexResponse{Items: items, Total: total})
}

// ListBookPlaces godoc
// @Summary Места книги в подборках
// @Description Доступно без авторизации. Места совпадают с /collections/{slug}; резерв не учитывается. Неизвестная или неопубликованная книга - 404
// @Tags collections
// @Produce json
// @Param id path string true "ID книги"
// @Success 200 {object} bookPlacesResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{id}/collections [get]
func (h *Handler) ListBookPlaces(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	places, err := h.service.ListBookPlaces(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err, "failed to get book collections")
	}
	items := make([]bookPlaceResponse, 0, len(places))
	for _, p := range places {
		items = append(items, bookPlaceResponse{
			Place: p.Place,
			Collection: bookPlaceCollection{
				ID: p.Collection.ID, Slug: p.Collection.Slug, Title: p.Collection.Title,
				Kind: p.Collection.Kind, Size: p.Collection.Size,
			},
		})
	}
	return c.JSON(http.StatusOK, bookPlacesResponse{Items: items, Total: len(items)})
}

// GetPublic godoc
// @Summary Опубликованная подборка с местами
// @Description Доступно без авторизации. Черновик или неизвестный адрес - 404
// @Tags collections
// @Produce json
// @Param slug path string true "Адрес подборки"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, по умолчанию 50, максимум 100"
// @Success 200 {object} collectionDetailResponse
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /collections/{slug} [get]
func (h *Handler) GetPublic(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	res, err := h.service.GetPublic(c.Request().Context(), c.Param("slug"), page, limit)
	if err != nil {
		return mapError(c, err, "failed to get collection")
	}
	items := make([]placedBookResponse, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, placedBookResponse{Place: it.Place, Movement: toMovementResponse(it.Movement), Book: toBookListItemResponse(it.Book)})
	}
	return c.JSON(http.StatusOK, collectionDetailResponse{
		Collection: toSummaryResponse(res.Summary), Items: items, Total: res.Total, Page: res.Page, Limit: res.Limit,
	})
}

// ---- admin ----

func parseID(c echo.Context, name string) (uuid.UUID, error) {
	return uuid.Parse(c.Param(name))
}

// AdminList godoc
// @Summary Список подборок (админ)
// @Tags admin-collections
// @Produce json
// @Security BearerAuth
// @Success 200 {object} adminListResponse
// @Failure 500 {object} map[string]string
// @Router /admin/collections [get]
func (h *Handler) AdminList(c echo.Context) error {
	list, err := h.service.ListAdmin(c.Request().Context())
	if err != nil {
		return mapError(c, err, "failed to list collections")
	}
	items := make([]adminListItemResponse, 0, len(list))
	for _, it := range list {
		col := it.Collection
		items = append(items, adminListItemResponse{
			ID: col.ID.String(), Slug: col.Slug, Title: col.Title, Kind: col.Kind, Size: col.Size,
			IsPublished: col.IsPublished, BooksCount: it.BooksCount, ShownCount: it.ShownCount, ComputedAt: computedAt(col), UpdatedAt: col.UpdatedAt,
		})
	}
	return c.JSON(http.StatusOK, adminListResponse{Items: items, Total: len(items)})
}

// AdminCreate godoc
// @Summary Создать подборку
// @Tags admin-collections
// @Accept json
// @Produce json
// @Security BearerAuth
// @Description kind: manual (по умолчанию) или auto; для auto обязателен rule, для manual rule запрещён. Автоматическая подборка сразу получает первый снимок
// @Param body body createRequest true "Данные подборки"
// @Success 201 {object} adminCollectionResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/collections [post]
func (h *Handler) AdminCreate(c echo.Context) error {
	var req createRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request")
	}
	in := service.CreateInput{
		Title: req.Title, Size: req.Size, Slug: req.Slug, Description: req.Description,
		Source: req.Source, SourceURL: req.SourceURL, IsPublished: req.IsPublished,
		Kind: req.Kind, Rule: req.Rule.toDomain(),
	}
	if req.GenreID != nil && *req.GenreID != "" {
		gid, err := uuid.Parse(*req.GenreID)
		if err != nil {
			return errJSON(c, http.StatusBadRequest, "invalid genre_id")
		}
		in.GenreID = &gid
	}
	col, err := h.service.Create(c.Request().Context(), in)
	if err != nil {
		return mapError(c, err, "failed to create collection")
	}
	return c.JSON(http.StatusCreated, toAdminCollectionResponse(*col))
}

// AdminGet godoc
// @Summary Подборка со всеми книгами (админ)
// @Tags admin-collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID подборки"
// @Success 200 {object} adminDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /admin/collections/{id} [get]
func (h *Handler) AdminGet(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid collection id")
	}
	d, err := h.service.GetAdmin(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err, "failed to get collection")
	}
	return c.JSON(http.StatusOK, toAdminDetailResponse(d))
}

// AdminRecompute godoc
// @Summary Пересчитать автоматическую подборку сейчас
// @Description Текущий снимок становится базой движения. Для ручной подборки - 422, для неизвестной - 404
// @Tags admin-collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID подборки"
// @Success 200 {object} adminDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Router /admin/collections/{id}/recompute [post]
func (h *Handler) AdminRecompute(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid collection id")
	}
	d, err := h.service.Recompute(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err, "failed to recompute collection")
	}
	return c.JSON(http.StatusOK, toAdminDetailResponse(d))
}

func toAdminDetailResponse(d *service.AdminDetail) adminDetailResponse {
	items := make([]adminItemResponse, 0, len(d.Items))
	for _, it := range d.Items {
		authors := make([]adminAuthorResponse, 0, len(it.Book.Authors))
		for _, a := range it.Book.Authors {
			authors = append(authors, adminAuthorResponse{ID: a.ID.String(), Name: a.Name})
		}
		var place *int
		if it.State == domain.StateRanked {
			p := it.Place
			place = &p
		}
		items = append(items, adminItemResponse{
			Position: it.Position, Place: place, State: it.State, Score: it.Score,
			Movement: toMovementResponse(it.Movement),
			Book: adminBookResponse{
				ID: it.Book.ID.String(), Title: it.Book.Title, Slug: it.Book.Slug, Status: it.Book.Status,
				CoverURL: it.Book.CoverURL, Authors: authors,
			},
		})
	}
	return adminDetailResponse{
		adminCollectionResponse: toAdminCollectionResponse(d.Collection),
		BooksCount:              d.BooksCount, ShownCount: d.ShownCount, Items: items,
	}
}

// AdminUpdate godoc
// @Summary Изменить подборку
// @Description Все поля необязательны; genre_id null сбрасывает жанр, пустые source/source_url очищаются; kind менять нельзя (400); rule заменяется целиком; смена rule, жанра или размера автоматической подборки сразу пересчитывает её без базы движения
// @Tags admin-collections
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID подборки"
// @Param body body updateRequest true "Поля для изменения"
// @Success 200 {object} adminCollectionResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/collections/{id} [patch]
func (h *Handler) AdminUpdate(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid collection id")
	}
	var req updateRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request")
	}
	in := service.UpdateInput{
		Title: req.Title, Size: req.Size, Slug: req.Slug, Description: req.Description,
		Source: req.Source, SourceURL: req.SourceURL, IsPublished: req.IsPublished,
		Kind: req.Kind, Rule: req.Rule.toDomain(),
	}
	if len(req.GenreID) > 0 {
		in.GenreIDSet = true
		if string(req.GenreID) != "null" {
			var raw string
			if err := json.Unmarshal(req.GenreID, &raw); err != nil {
				return errJSON(c, http.StatusBadRequest, "invalid genre_id")
			}
			if raw != "" {
				gid, err := uuid.Parse(raw)
				if err != nil {
					return errJSON(c, http.StatusBadRequest, "invalid genre_id")
				}
				in.GenreID = &gid
			}
		}
	}
	col, err := h.service.Update(c.Request().Context(), id, in)
	if err != nil {
		return mapError(c, err, "failed to update collection")
	}
	return c.JSON(http.StatusOK, toAdminCollectionResponse(*col))
}

// AdminDelete godoc
// @Summary Удалить подборку
// @Description Удаляется подборка и её список книг, сами книги остаются
// @Tags admin-collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID подборки"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /admin/collections/{id} [delete]
func (h *Handler) AdminDelete(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid collection id")
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return mapError(c, err, "failed to delete collection")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// AdminAddBook godoc
// @Summary Добавить книгу каталога в подборку
// @Description Принимаются книги pending и published; отклонённая - 422
// @Tags admin-collections
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID подборки"
// @Param body body addBookRequest true "Книга и необязательная позиция"
// @Success 201 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Router /admin/collections/{id}/books [post]
func (h *Handler) AdminAddBook(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid collection id")
	}
	var req addBookRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request")
	}
	bookID, err := uuid.Parse(req.BookID)
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book_id")
	}
	if err := h.service.AddBook(c.Request().Context(), id, bookID, req.Position); err != nil {
		return mapError(c, err, "failed to add book")
	}
	return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
}

// AdminRemoveBook godoc
// @Summary Убрать книгу из подборки
// @Tags admin-collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID подборки"
// @Param bookId path string true "ID книги"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /admin/collections/{id}/books/{bookId} [delete]
func (h *Handler) AdminRemoveBook(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid collection id")
	}
	bookID, err := parseID(c, "bookId")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	if err := h.service.RemoveBook(c.Request().Context(), id, bookID); err != nil {
		return mapError(c, err, "failed to remove book")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// AdminMoveBook godoc
// @Summary Переместить книгу на позицию
// @Tags admin-collections
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID подборки"
// @Param bookId path string true "ID книги"
// @Param body body moveBookRequest true "Позиция, с 1"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /admin/collections/{id}/books/{bookId}/position [put]
func (h *Handler) AdminMoveBook(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid collection id")
	}
	bookID, err := parseID(c, "bookId")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	var req moveBookRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid position")
	}
	if err := h.service.MoveBook(c.Request().Context(), id, bookID, req.Position); err != nil {
		return mapError(c, err, "failed to move book")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ---- bulk paste ----

type pastePreviewRequest struct {
	Text string `json:"text"`
}

type bulkAddRequest struct {
	BookIDs  []string `json:"book_ids"`
	Position *int     `json:"position"`
}

type pasteBookResponse struct {
	ID            string                `json:"id"`
	Title         string                `json:"title"`
	Slug          string                `json:"slug"`
	Status        string                `json:"status"`
	CoverURL      *string               `json:"cover_url"`
	PublishedYear *int                  `json:"published_year"`
	Authors       []adminAuthorResponse `json:"authors"`
	InCollection  bool                  `json:"in_collection"`
}

type pasteLineResponse struct {
	Line        int                 `json:"line"`
	Text        string              `json:"text"`
	Title       string              `json:"title"`
	Author      *string             `json:"author"`
	Status      string              `json:"status"`
	Book        *pasteBookResponse  `json:"book"`
	Candidates  []pasteBookResponse `json:"candidates"`
	DuplicateOf *int                `json:"duplicate_of"`
}

type pasteCountsResponse struct {
	Found        int `json:"found"`
	Ambiguous    int `json:"ambiguous"`
	NotFound     int `json:"not_found"`
	Rejected     int `json:"rejected"`
	InCollection int `json:"in_collection"`
	Duplicate    int `json:"duplicate"`
	Invalid      int `json:"invalid"`
}

type pastePreviewResponse struct {
	BooksCount int                 `json:"books_count"`
	FreeSlots  int                 `json:"free_slots"`
	Counts     pasteCountsResponse `json:"counts"`
	Lines      []pasteLineResponse `json:"lines"`
}

type bulkAddedResponse struct {
	BookID   string `json:"book_id"`
	Position int    `json:"position"`
}

type bulkSkippedResponse struct {
	BookID string `json:"book_id"`
	Reason string `json:"reason"`
}

type bulkAddResponse struct {
	Added      []bulkAddedResponse   `json:"added"`
	Skipped    []bulkSkippedResponse `json:"skipped"`
	BooksCount int                   `json:"books_count"`
}

func toPasteBookResponse(b service.PasteBook) pasteBookResponse {
	authors := make([]adminAuthorResponse, 0, len(b.Book.Authors))
	for _, a := range b.Book.Authors {
		authors = append(authors, adminAuthorResponse{ID: a.ID.String(), Name: a.Name})
	}
	return pasteBookResponse{
		ID: b.Book.ID.String(), Title: b.Book.Title, Slug: b.Book.Slug, Status: b.Book.Status,
		CoverURL: b.Book.CoverURL, PublishedYear: b.Book.PublishedYear, Authors: authors, InCollection: b.InCollection,
	}
}

// AdminPastePreview godoc
// @Summary Разобрать вставленный список книг (предпросмотр)
// @Description Только чтение: каждая непустая строка сопоставляется с каталогом, ничего не создаётся и не добавляется. Не более 500 строк и 100000 символов
// @Tags admin-collections
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID подборки"
// @Param body body pastePreviewRequest true "Текст со списком книг"
// @Success 200 {object} pastePreviewResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Router /admin/collections/{id}/books/paste/preview [post]
func (h *Handler) AdminPastePreview(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid collection id")
	}
	var req pastePreviewRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request")
	}
	p, err := h.service.PreviewPaste(c.Request().Context(), id, req.Text)
	if err != nil {
		return mapError(c, err, "failed to preview paste")
	}
	lines := make([]pasteLineResponse, 0, len(p.Lines))
	for _, l := range p.Lines {
		r := pasteLineResponse{
			Line: l.Line, Text: l.Text, Title: l.Title, Author: l.Author, Status: string(l.Status),
			Candidates: make([]pasteBookResponse, 0, len(l.Candidates)), DuplicateOf: l.DuplicateOf,
		}
		if l.Book != nil {
			b := toPasteBookResponse(*l.Book)
			r.Book = &b
		}
		for _, cand := range l.Candidates {
			r.Candidates = append(r.Candidates, toPasteBookResponse(cand))
		}
		lines = append(lines, r)
	}
	n := p.Counts
	return c.JSON(http.StatusOK, pastePreviewResponse{
		BooksCount: p.BooksCount, FreeSlots: p.FreeSlots, Lines: lines,
		Counts: pasteCountsResponse{
			Found: n[domain.PasteFound], Ambiguous: n[domain.PasteAmbiguous], NotFound: n[domain.PasteNotFound],
			Rejected: n[domain.PasteRejected], InCollection: n[domain.PasteInCollection],
			Duplicate: n[domain.PasteDuplicate], Invalid: n[domain.PasteInvalid],
		},
	})
}

// AdminBulkAdd godoc
// @Summary Добавить выбранные книги каталога в подборку списком
// @Description Книги добавляются одним блоком в заданном порядке, в конец или на позицию. Неизвестные, отклонённые, повторы и уже добавленные пропускаются; если оставшихся больше свободных мест (максимум 1000 книг) - 422 и ничего не добавляется
// @Tags admin-collections
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID подборки"
// @Param body body bulkAddRequest true "Список книг и необязательная позиция"
// @Success 200 {object} bulkAddResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Router /admin/collections/{id}/books/bulk [post]
func (h *Handler) AdminBulkAdd(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid collection id")
	}
	var req bulkAddRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request")
	}
	if len(req.BookIDs) == 0 || len(req.BookIDs) > service.MaxBulkIDs {
		return errJSON(c, http.StatusBadRequest, "book_ids: must contain 1 to 500 ids")
	}
	ids := make([]uuid.UUID, len(req.BookIDs))
	for i, raw := range req.BookIDs {
		bid, err := uuid.Parse(raw)
		if err != nil {
			return errJSON(c, http.StatusBadRequest, "book_ids: invalid id")
		}
		ids[i] = bid
	}
	res, err := h.service.BulkAdd(c.Request().Context(), id, ids, req.Position)
	if err != nil {
		return mapError(c, err, "failed to add books")
	}
	out := bulkAddResponse{
		Added: make([]bulkAddedResponse, 0, len(res.Added)), Skipped: make([]bulkSkippedResponse, 0, len(res.Skipped)),
		BooksCount: res.BooksCount,
	}
	for _, a := range res.Added {
		out.Added = append(out.Added, bulkAddedResponse{BookID: a.BookID.String(), Position: a.Position})
	}
	for _, s := range res.Skipped {
		out.Skipped = append(out.Skipped, bulkSkippedResponse{BookID: s.BookID.String(), Reason: s.Reason})
	}
	return c.JSON(http.StatusOK, out)
}
