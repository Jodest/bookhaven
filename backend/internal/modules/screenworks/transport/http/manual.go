package http

import (
	"errors"
	"net/http"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Manual links (openspec change add-screen-works-admin-editing).

type addAdaptationRequest struct {
	ScreenWorkID *string            `json:"screen_work_id"`
	Work         *domain.WorkFields `json:"work"`
}

type addAuthorFilmRequest struct {
	addAdaptationRequest
	Relation string `json:"relation"`
}

type addAdaptationResponse struct {
	adminAdaptationResponse
	WorkCreated bool `json:"work_created"`
}

type addAuthorFilmResponse struct {
	adminAuthorFilmResponse
	WorkCreated bool `json:"work_created"`
}

func parseWorkID(raw *string) (*uuid.UUID, bool) {
	if raw == nil {
		return nil, true
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, false
	}
	return &id, true
}

// AdminAddAdaptation godoc
// @Summary Добавить экранизацию книге вручную (админ)
// @Description Требуется роль admin и Bearer-токен. Тело: screen_work_id (существующая работа) либо work (новая работа: title, original_title, kind, year, director, imdb_id, kinopoisk_id, qid) - ровно одно из двух. Связь с source = manual публикуется сразу. Книга любого статуса. Если QID уже есть в каталоге, привязывается существующая работа без изменений (work_created = false). Одинаковый IMDb или Кинопоиск id у новой работы - 409 с screen_work_id существующей. Уже привязанная работа (в том числе скрытая) - 409. Ошибки полей - 422
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID книги"
// @Param body body addAdaptationRequest true "Существующая или новая работа"
// @Success 201 {object} addAdaptationResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/{id}/adaptations [post]
func (h *Handler) AdminAddAdaptation(c echo.Context) error {
	bookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	var req addAdaptationRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request")
	}
	workID, ok := parseWorkID(req.ScreenWorkID)
	if !ok {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	res, err := h.service.AddBookAdaptation(c.Request().Context(), bookID, workID, req.Work)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrLinkRefRequired):
			return errJSON(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, domain.ErrBookNotFound):
			return errJSON(c, http.StatusNotFound, "book not found")
		case errors.Is(err, domain.ErrAlreadyLinked):
			return errJSON(c, http.StatusConflict, "screen work is already linked to this book")
		}
		if handled, r := workErr(c, err); handled {
			return r
		}
		return errJSON(c, http.StatusInternalServerError, "failed to add adaptation")
	}
	return c.JSON(http.StatusCreated, addAdaptationResponse{adminAdaptationResponse: toAdminResponse(*res.Item), WorkCreated: res.WorkCreated})
}

// AdminDeleteAdaptation godoc
// @Summary Удалить ручную связь книги с экранизацией (админ)
// @Description Требуется роль admin и Bearer-токен. Удаляет только связь с source = manual, сама работа остаётся. Связь из Wikidata не удаляется (409), её нужно скрыть. Отсутствующая связь - 404
// @Tags admin
// @Security BearerAuth
// @Param id path string true "ID книги"
// @Param screen_work_id path string true "ID экранизации"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/{id}/adaptations/{screen_work_id} [delete]
func (h *Handler) AdminDeleteAdaptation(c echo.Context) error {
	bookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	workID, err := uuid.Parse(c.Param("screen_work_id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	if err := h.service.DeleteBookAdaptation(c.Request().Context(), bookID, workID); err != nil {
		switch {
		case errors.Is(err, domain.ErrLinkNotFound):
			return errJSON(c, http.StatusNotFound, "adaptation link not found")
		case errors.Is(err, domain.ErrLinkNotManual):
			return errJSON(c, http.StatusConflict, err.Error())
		}
		return errJSON(c, http.StatusInternalServerError, "failed to delete adaptation link")
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminAddAuthorFilm godoc
// @Summary Добавить фильм об авторе вручную (админ)
// @Description Требуется роль admin и Bearer-токен. Работает как добавление экранизации книге (screen_work_id либо work), автор любого статуса; дополнительно обязателен relation (biopic | documentary). Связь с source = manual публикуется сразу. Уже привязанная работа - 409
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID автора"
// @Param body body addAuthorFilmRequest true "Существующая или новая работа и тип связи"
// @Success 201 {object} addAuthorFilmResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors/{id}/screen-works [post]
func (h *Handler) AdminAddAuthorFilm(c echo.Context) error {
	authorID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid author id")
	}
	var req addAuthorFilmRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request")
	}
	workID, ok := parseWorkID(req.ScreenWorkID)
	if !ok {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	res, err := h.service.AddAuthorFilm(c.Request().Context(), authorID, workID, req.Work, req.Relation)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrLinkRefRequired), errors.Is(err, domain.ErrInvalidRelation):
			return errJSON(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, domain.ErrAuthorNotFound):
			return errJSON(c, http.StatusNotFound, "author not found")
		case errors.Is(err, domain.ErrAlreadyLinked):
			return errJSON(c, http.StatusConflict, "screen work is already linked to this author")
		}
		if handled, r := workErr(c, err); handled {
			return r
		}
		return errJSON(c, http.StatusInternalServerError, "failed to add author film")
	}
	return c.JSON(http.StatusCreated, addAuthorFilmResponse{adminAuthorFilmResponse: toAdminAuthorFilmResponse(*res.Item), WorkCreated: res.WorkCreated})
}

// AdminDeleteAuthorFilm godoc
// @Summary Удалить ручную связь автора с фильмом (админ)
// @Description Требуется роль admin и Bearer-токен. Удаляет только связь с source = manual, сама работа остаётся. Связь из Wikidata не удаляется (409), её нужно скрыть. Отсутствующая связь - 404
// @Tags admin
// @Security BearerAuth
// @Param id path string true "ID автора"
// @Param screen_work_id path string true "ID фильма"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/authors/{id}/screen-works/{screen_work_id} [delete]
func (h *Handler) AdminDeleteAuthorFilm(c echo.Context) error {
	authorID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid author id")
	}
	workID, err := uuid.Parse(c.Param("screen_work_id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	if err := h.service.DeleteAuthorFilm(c.Request().Context(), authorID, workID); err != nil {
		switch {
		case errors.Is(err, domain.ErrAuthorLinkNotFound):
			return errJSON(c, http.StatusNotFound, "author film link not found")
		case errors.Is(err, domain.ErrLinkNotManual):
			return errJSON(c, http.StatusConflict, err.Error())
		}
		return errJSON(c, http.StatusInternalServerError, "failed to delete author film link")
	}
	return c.NoContent(http.StatusNoContent)
}
