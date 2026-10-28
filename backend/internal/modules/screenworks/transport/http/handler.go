// Package http exposes the screen works endpoints: the public adaptations of
// a book and the admin sync, list and hide/show endpoints.
package http

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/bookhaven/backend/internal/modules/screenworks/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles screen works HTTP requests.
type Handler struct {
	service *service.Service
}

// NewHandler creates a Handler.
func NewHandler(svc *service.Service) *Handler {
	return &Handler{service: svc}
}

type adaptationResponse struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle *string `json:"original_title"`
	Kind          string  `json:"kind"`
	Year          *int    `json:"year"`
	Director      *string `json:"director"`
	IMDbURL       *string `json:"imdb_url"`
	KinopoiskURL  *string `json:"kinopoisk_url"`
	// Poster is null when the work has no poster.
	Poster *posterResponse `json:"poster"`
}

type posterResponse struct {
	// URL is an own /covers/... path once a copy of a Commons or TMDB poster is stored, otherwise the source link;
	// an uploaded poster is /posters/...
	URL        string  `json:"url"`
	Source     string  `json:"source" enums:"manual,commons,tmdb"`
	Author     *string `json:"author"`
	License    *string `json:"license"`
	LicenseURL *string `json:"license_url"`
	SourceURL  *string `json:"source_url"`
}

func toPosterResponse(p *domain.Poster) *posterResponse {
	if p == nil {
		return nil
	}
	return &posterResponse{
		URL: p.URL, Source: p.Source, Author: p.Author, License: p.License,
		LicenseURL: p.LicenseURL, SourceURL: p.SourceURL,
	}
}

type adaptationsResponse struct {
	Items []adaptationResponse `json:"items"`
	Total int                  `json:"total"`
}

type adminAdaptationResponse struct {
	adaptationResponse
	Hidden      bool    `json:"hidden"`
	Source      string  `json:"source"`
	QID         *string `json:"qid"`
	IMDbID      *string `json:"imdb_id"`
	KinopoiskID *string `json:"kinopoisk_id"`
}

type adminAdaptationsResponse struct {
	Items []adminAdaptationResponse `json:"items"`
	Total int                       `json:"total"`
}

func toResponse(w domain.LinkedWork) adaptationResponse {
	return adaptationResponse{
		ID: w.ID.String(), Title: w.Title, OriginalTitle: w.OriginalTitle, Kind: w.Kind,
		Year: w.Year, Director: w.Director, IMDbURL: w.IMDbURL(), KinopoiskURL: w.KinopoiskURL(),
		Poster: toPosterResponse(w.Poster()),
	}
}

func toAdminResponse(w domain.LinkedWork) adminAdaptationResponse {
	return adminAdaptationResponse{
		adaptationResponse: toResponse(w), Hidden: w.Hidden, Source: w.Source,
		QID: w.ExternalID, IMDbID: w.IMDbID, KinopoiskID: w.KinopoiskID,
	}
}

func errJSON(c echo.Context, code int, msg string) error {
	return c.JSON(code, map[string]string{"error": msg})
}

// ListBookAdaptations godoc
// @Summary Экранизации книги
// @Description Доступно без авторизации. Видимые экранизации опубликованной книги без пагинации: сначала по году (без года - в конце), затем по названию. Скрытые администратором не возвращаются. Неизвестная или неопубликованная книга - 404
// @Tags screen-works
// @Produce json
// @Param id path string true "ID книги"
// @Success 200 {object} adaptationsResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{id}/adaptations [get]
func (h *Handler) ListBookAdaptations(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	list, err := h.service.ListPublic(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrBookNotFound) {
			return errJSON(c, http.StatusNotFound, "book not found")
		}
		return errJSON(c, http.StatusInternalServerError, "failed to get book adaptations")
	}
	items := make([]adaptationResponse, 0, len(list))
	for _, w := range list {
		items = append(items, toResponse(w))
	}
	return c.JSON(http.StatusOK, adaptationsResponse{Items: items, Total: len(items)})
}

// AdminListBookAdaptations godoc
// @Summary Экранизации книги (админ)
// @Description Требуется роль admin. Все связанные экранизации книги любого статуса, включая скрытые, в том же порядке, что и публичный список
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID книги"
// @Success 200 {object} adminAdaptationsResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/{id}/adaptations [get]
func (h *Handler) AdminListBookAdaptations(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	list, err := h.service.ListAdmin(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrBookNotFound) {
			return errJSON(c, http.StatusNotFound, "book not found")
		}
		return errJSON(c, http.StatusInternalServerError, "failed to get book adaptations")
	}
	items := make([]adminAdaptationResponse, 0, len(list))
	for _, w := range list {
		items = append(items, toAdminResponse(w))
	}
	return c.JSON(http.StatusOK, adminAdaptationsResponse{Items: items, Total: len(items)})
}

type setHiddenRequest struct {
	Hidden *bool `json:"hidden"`
}

// AdminSetAdaptationHidden godoc
// @Summary Скрыть или показать экранизацию книги (админ)
// @Description Требуется роль admin и Bearer-токен. Поле hidden обязательно. Связь и экранизация не удаляются, поэтому повторная синхронизация скрытую связь не возвращает. Неизвестная книга или связь - 404
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID книги"
// @Param screen_work_id path string true "ID экранизации"
// @Param body body setHiddenRequest true "Видимость"
// @Success 200 {object} adminAdaptationResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/{id}/adaptations/{screen_work_id} [patch]
func (h *Handler) AdminSetAdaptationHidden(c echo.Context) error {
	bookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	workID, err := uuid.Parse(c.Param("screen_work_id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	var req setHiddenRequest
	if err := c.Bind(&req); err != nil || req.Hidden == nil {
		return errJSON(c, http.StatusBadRequest, "hidden is required")
	}
	item, err := h.service.SetHidden(c.Request().Context(), bookID, workID, *req.Hidden)
	if err != nil {
		if errors.Is(err, domain.ErrLinkNotFound) {
			return errJSON(c, http.StatusNotFound, "adaptation link not found")
		}
		return errJSON(c, http.StatusInternalServerError, "failed to update adaptation")
	}
	return c.JSON(http.StatusOK, toAdminResponse(*item))
}

type syncRequest struct {
	IDs []string `json:"ids"`
}

// SyncAdaptations godoc
// @Summary Найти экранизации книг в Wikidata (админ)
// @Description Требуется роль admin и Bearer-токен. Для 1..50 книг (любой статус, дубликаты схлопываются) с QID Wikidata находит экранизации по свойству «основано на» (P144): только фильмы, телефильмы, сериалы, мультфильмы, мультсериалы и телеспектакли; серии фильмов, игры, статьи-списки и эпизоды отбрасываются. Связи публикуются сразу, синхронизация только добавляет и никогда не возвращает скрытую администратором связь. Каждая книга обрабатывается независимо: outcome = added | unchanged | skipped (notes: no_qid) | error (notes: wikidata_unavailable). 502 - если все запросы к Wikidata не удались
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body syncRequest true "ID книг (1..50)"
// @Success 200 {object} service.SyncReport
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/books/sync-adaptations [post]
func (h *Handler) SyncAdaptations(c echo.Context) error {
	var req syncRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request")
	}
	if len(req.IDs) == 0 {
		return errJSON(c, http.StatusBadRequest, service.ErrSyncEmpty.Error())
	}
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return errJSON(c, http.StatusBadRequest, fmt.Sprintf("invalid book id: %q", raw))
		}
		ids = append(ids, id)
	}
	report, err := h.service.SyncBooks(c.Request().Context(), ids)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSyncEmpty), errors.Is(err, service.ErrSyncTooLarge):
			return errJSON(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrSyncUnavailable):
			return errJSON(c, http.StatusServiceUnavailable, service.ErrSyncUnavailable.Error())
		case report != nil:
			// Every Wikidata call failed: the report is discarded.
			return errJSON(c, http.StatusBadGateway, "wikidata is unavailable; retry later")
		default:
			return errJSON(c, http.StatusInternalServerError, "failed to sync adaptations")
		}
	}
	return c.JSON(http.StatusOK, report)
}
