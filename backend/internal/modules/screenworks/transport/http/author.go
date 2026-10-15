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

// Films about authors (openspec change add-author-screen-works).

type authorFilmResponse struct {
	adaptationResponse
	Relation string `json:"relation"`
}

type authorFilmsResponse struct {
	Items []authorFilmResponse `json:"items"`
	Total int                  `json:"total"`
}

type adminAuthorFilmResponse struct {
	authorFilmResponse
	Hidden      bool    `json:"hidden"`
	Source      string  `json:"source"`
	QID         *string `json:"qid"`
	IMDbID      *string `json:"imdb_id"`
	KinopoiskID *string `json:"kinopoisk_id"`
}

type adminAuthorFilmsResponse struct {
	Items []adminAuthorFilmResponse `json:"items"`
	Total int                       `json:"total"`
}

func toAuthorFilmResponse(w domain.LinkedWork) authorFilmResponse {
	return authorFilmResponse{adaptationResponse: toResponse(w), Relation: w.Relation}
}

func toAdminAuthorFilmResponse(w domain.LinkedWork) adminAuthorFilmResponse {
	return adminAuthorFilmResponse{
		authorFilmResponse: toAuthorFilmResponse(w), Hidden: w.Hidden, Source: w.Source,
		QID: w.ExternalID, IMDbID: w.IMDbID, KinopoiskID: w.KinopoiskID,
	}
}

// ListAuthorFilms godoc
// @Summary Фильмы об авторе
// @Description Доступно без авторизации. Видимые фильмы об опубликованном авторе (художественные байопики и документальные, relation = biopic | documentary) без пагинации: сначала по году (без года - в конце), затем по названию. Скрытые администратором не возвращаются. Неизвестный или неопубликованный автор - 404. Параметр id - это ID автора, не slug
// @Tags screen-works
// @Produce json
// @Param id path string true "ID автора"
// @Success 200 {object} authorFilmsResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /authors/{id}/screen-works [get]
func (h *Handler) ListAuthorFilms(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid author id")
	}
	list, err := h.service.ListPublicByAuthor(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrAuthorNotFound) {
			return errJSON(c, http.StatusNotFound, "author not found")
		}
		return errJSON(c, http.StatusInternalServerError, "failed to get author films")
	}
	items := make([]authorFilmResponse, 0, len(list))
	for _, w := range list {
		items = append(items, toAuthorFilmResponse(w))
	}
	return c.JSON(http.StatusOK, authorFilmsResponse{Items: items, Total: len(items)})
}

// AdminListAuthorFilms godoc
// @Summary Фильмы об авторе (админ)
// @Description Требуется роль admin. Все связанные фильмы об авторе любого статуса, включая скрытые, в том же порядке, что и публичный список
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID автора"
// @Success 200 {object} adminAuthorFilmsResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors/{id}/screen-works [get]
func (h *Handler) AdminListAuthorFilms(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid author id")
	}
	list, err := h.service.ListAdminByAuthor(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrAuthorNotFound) {
			return errJSON(c, http.StatusNotFound, "author not found")
		}
		return errJSON(c, http.StatusInternalServerError, "failed to get author films")
	}
	items := make([]adminAuthorFilmResponse, 0, len(list))
	for _, w := range list {
		items = append(items, toAdminAuthorFilmResponse(w))
	}
	return c.JSON(http.StatusOK, adminAuthorFilmsResponse{Items: items, Total: len(items)})
}

type updateAuthorFilmRequest struct {
	Hidden   *bool   `json:"hidden"`
	Relation *string `json:"relation"`
}

// AdminUpdateAuthorFilm godoc
// @Summary Изменить связь фильма с автором (админ)
// @Description Требуется роль admin и Bearer-токен. Тело: hidden, relation (biopic | documentary) или оба поля; хотя бы одно обязательно. Связь любого источника. Связь и фильм не удаляются при скрытии, поэтому повторная синхронизация скрытую связь не возвращает; выставленный relation синхронизация не меняет. Видимость экранизации той же работы у книги не меняется. Неизвестный автор или связь - 404
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID автора"
// @Param screen_work_id path string true "ID фильма"
// @Param body body updateAuthorFilmRequest true "Видимость и/или тип связи"
// @Success 200 {object} adminAuthorFilmResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors/{id}/screen-works/{screen_work_id} [patch]
func (h *Handler) AdminUpdateAuthorFilm(c echo.Context) error {
	authorID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid author id")
	}
	workID, err := uuid.Parse(c.Param("screen_work_id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	var req updateAuthorFilmRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, domain.ErrLinkPatchEmpty.Error())
	}
	item, err := h.service.UpdateAuthorFilm(c.Request().Context(), authorID, workID, req.Hidden, req.Relation)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrLinkPatchEmpty), errors.Is(err, domain.ErrInvalidRelation):
			return errJSON(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, domain.ErrAuthorLinkNotFound):
			return errJSON(c, http.StatusNotFound, "author film link not found")
		}
		return errJSON(c, http.StatusInternalServerError, "failed to update author film")
	}
	return c.JSON(http.StatusOK, toAdminAuthorFilmResponse(*item))
}

// SyncAuthorFilms godoc
// @Summary Найти фильмы об авторах в Wikidata (админ)
// @Description Требуется роль admin и Bearer-токен. Для 1..50 авторов (любой статус, дубликаты схлопываются) с QID Wikidata находит видеоработы, у которых главная тема (P921) - автор: фильмы, телефильмы, сериалы, мультфильмы, мультсериалы, телеспектакли, документальные и биографические фильмы; серии фильмов, игры, статьи-списки и эпизоды отбрасываются. Документальные получают relation = documentary, остальные - biopic. Связи публикуются сразу, синхронизация только добавляет и никогда не возвращает скрытую администратором связь. Каждый автор обрабатывается независимо: outcome = added | unchanged | skipped (notes: no_qid) | error (notes: wikidata_unavailable). 502 - если все запросы к Wikidata не удались
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body syncRequest true "ID авторов (1..50)"
// @Success 200 {object} service.AuthorSyncReport
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/authors/sync-screen-works [post]
func (h *Handler) SyncAuthorFilms(c echo.Context) error {
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
			return errJSON(c, http.StatusBadRequest, fmt.Sprintf("invalid author id: %q", raw))
		}
		ids = append(ids, id)
	}
	report, err := h.service.SyncAuthors(c.Request().Context(), ids)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSyncEmpty), errors.Is(err, service.ErrAuthorSyncTooLarge):
			return errJSON(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrAuthorSyncUnavailable):
			return errJSON(c, http.StatusServiceUnavailable, service.ErrAuthorSyncUnavailable.Error())
		case report != nil:
			// Every Wikidata call failed: the report is discarded.
			return errJSON(c, http.StatusBadGateway, "wikidata is unavailable; retry later")
		default:
			return errJSON(c, http.StatusInternalServerError, "failed to sync author films")
		}
	}
	return c.JSON(http.StatusOK, report)
}
