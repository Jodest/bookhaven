package http

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/bookhaven/backend/internal/pkg/coverfetch"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Admin screen works (openspec change add-screen-works-admin-editing).

type workItemResponse struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	OriginalTitle    *string    `json:"original_title"`
	Kind             string     `json:"kind"`
	Year             *int       `json:"year"`
	Director         *string    `json:"director"`
	IMDbID           *string    `json:"imdb_id"`
	KinopoiskID      *string    `json:"kinopoisk_id"`
	QID              *string    `json:"qid"`
	ManuallyEditedAt *time.Time `json:"manually_edited_at"`
	BookCount        int        `json:"book_count"`
	AuthorCount      int        `json:"author_count"`
}

type workBookLinkResponse struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Hidden bool   `json:"hidden"`
	Source string `json:"source"`
}

type workAuthorLinkResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Hidden   bool   `json:"hidden"`
	Source   string `json:"source"`
	Relation string `json:"relation"`
}

type workDetailResponse struct {
	workItemResponse
	// Poster is null without a poster; PosterRemoved is true when an admin
	// removed an automatic poster (the sync will not refill it).
	Poster        *posterResponse          `json:"poster"`
	PosterRemoved bool                     `json:"poster_removed"`
	Books         []workBookLinkResponse   `json:"books"`
	Authors       []workAuthorLinkResponse `json:"authors"`
}

type workListResponse struct {
	Items []workItemResponse `json:"items"`
	Total int64              `json:"total"`
	Page  int                `json:"page"`
	Limit int                `json:"limit"`
}

func toWorkItemResponse(w domain.WorkListItem) workItemResponse {
	return workItemResponse{
		ID: w.ID.String(), Title: w.Title, OriginalTitle: w.OriginalTitle, Kind: w.Kind, Year: w.Year,
		Director: w.Director, IMDbID: w.IMDbID, KinopoiskID: w.KinopoiskID, QID: w.QID,
		ManuallyEditedAt: w.ManuallyEditedAt, BookCount: w.BookCount, AuthorCount: w.AuthorCount,
	}
}

func toWorkDetailResponse(d domain.WorkDetail) workDetailResponse {
	out := workDetailResponse{
		workItemResponse: toWorkItemResponse(d.WorkListItem),
		Poster:           toPosterResponse(d.Poster()),
		PosterRemoved:    d.PosterRemoved(),
		Books:            make([]workBookLinkResponse, 0, len(d.Books)),
		Authors:          make([]workAuthorLinkResponse, 0, len(d.Authors)),
	}
	for _, b := range d.Books {
		out.Books = append(out.Books, workBookLinkResponse{ID: b.ID.String(), Title: b.Title, Status: b.Status, Hidden: b.Hidden, Source: b.Source})
	}
	for _, a := range d.Authors {
		out.Authors = append(out.Authors, workAuthorLinkResponse{ID: a.ID.String(), Name: a.Name, Status: a.Status, Hidden: a.Hidden, Source: a.Source, Relation: a.Relation})
	}
	return out
}

// workErr maps the work errors shared by the manual endpoints; ok is false
// for an unknown error.
func workErr(c echo.Context, err error) (handled bool, result error) {
	var ve *domain.ValidationError
	var dup *domain.ErrDuplicateExternalID
	switch {
	case errors.As(err, &ve):
		return true, errJSON(c, http.StatusUnprocessableEntity, ve.Msg)
	case errors.As(err, &dup):
		return true, c.JSON(http.StatusConflict, map[string]string{"error": dup.Error(), "screen_work_id": dup.WorkID.String()})
	case errors.Is(err, domain.ErrWorkNotFound):
		return true, errJSON(c, http.StatusNotFound, domain.ErrWorkNotFound.Error())
	case errors.Is(err, domain.ErrQIDTaken):
		return true, errJSON(c, http.StatusConflict, domain.ErrQIDTaken.Error())
	case errors.Is(err, domain.ErrQIDImmutable):
		return true, errJSON(c, http.StatusUnprocessableEntity, domain.ErrQIDImmutable.Error())
	case errors.Is(err, domain.ErrWorkHasLinks):
		return true, errJSON(c, http.StatusConflict, domain.ErrWorkHasLinks.Error())
	case errors.Is(err, domain.ErrPosterTooLarge):
		return true, errJSON(c, http.StatusRequestEntityTooLarge, domain.ErrPosterTooLarge.Error())
	case errors.Is(err, domain.ErrPosterType):
		return true, errJSON(c, http.StatusUnprocessableEntity, domain.ErrPosterType.Error())
	case errors.Is(err, domain.ErrPostersUnavailable):
		return true, errJSON(c, http.StatusServiceUnavailable, domain.ErrPostersUnavailable.Error())
	}
	return false, nil
}

// AdminListWorks godoc
// @Summary Список фильмов и сериалов (админ)
// @Description Требуется роль admin. Все работы с пагинацией (page по умолчанию 1, limit по умолчанию 20, максимум 100), по названию, затем году. search - подстрока названия или оригинального названия без учёта регистра либо точный QID, IMDb id или Кинопоиск id. unlinked=true оставляет работы без связей с книгами и авторами. Счётчики включают скрытые связи
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Поиск"
// @Param unlinked query bool false "Только без связей"
// @Param page query int false "Страница"
// @Param limit query int false "Размер страницы (1..100)"
// @Success 200 {object} workListResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/screen-works [get]
func (h *Handler) AdminListWorks(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	res, err := h.service.ListWorks(c.Request().Context(), domain.WorkFilter{
		Search: c.QueryParam("search"), Unlinked: c.QueryParam("unlinked") == "true", Page: page, Limit: limit,
	})
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to list screen works")
	}
	items := make([]workItemResponse, 0, len(res.Items))
	for _, w := range res.Items {
		items = append(items, toWorkItemResponse(w))
	}
	return c.JSON(http.StatusOK, workListResponse{Items: items, Total: res.Total, Page: res.Page, Limit: res.Limit})
}

// AdminGetWork godoc
// @Summary Фильм или сериал (админ)
// @Description Требуется роль admin. Работа со связанными книгами и авторами любого статуса, включая скрытые связи
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID фильма"
// @Success 200 {object} workDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/screen-works/{id} [get]
func (h *Handler) AdminGetWork(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	d, err := h.service.GetWork(c.Request().Context(), id)
	if err != nil {
		if handled, res := workErr(c, err); handled {
			return res
		}
		return errJSON(c, http.StatusInternalServerError, "failed to get screen work")
	}
	return c.JSON(http.StatusOK, toWorkDetailResponse(*d))
}

// AdminUpdateWork godoc
// @Summary Изменить фильм или сериал (админ)
// @Description Требуется роль admin и Bearer-токен. Полная замена полей (title, original_title, kind, year, director, imdb_id, kinopoisk_id); qid необязателен: у работы без QID его можно добавить, у работы с QID изменить или убрать нельзя (422). Работа помечается как изменённая вручную, и синхронизация Wikidata её больше не меняет. Занятый QID - 409, ошибки полей - 422
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID фильма"
// @Param body body domain.WorkFields true "Данные фильма"
// @Success 200 {object} workDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/screen-works/{id} [patch]
func (h *Handler) AdminUpdateWork(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	var req domain.WorkFields
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request")
	}
	d, err := h.service.UpdateWork(c.Request().Context(), id, req)
	if err != nil {
		if handled, res := workErr(c, err); handled {
			return res
		}
		return errJSON(c, http.StatusInternalServerError, "failed to update screen work")
	}
	return c.JSON(http.StatusOK, toWorkDetailResponse(*d))
}

// AdminDeleteWork godoc
// @Summary Удалить фильм или сериал (админ)
// @Description Требуется роль admin и Bearer-токен. Удаляет работу только без связей с книгами и авторами (скрытые связи тоже считаются): иначе 409. Строка фильма блокируется на время проверки связей и удаления, поэтому параллельное добавление связи не теряется: итог - 409 либо связь сохраняется
// @Tags admin
// @Security BearerAuth
// @Param id path string true "ID фильма"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/screen-works/{id} [delete]
func (h *Handler) AdminDeleteWork(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	if err := h.service.DeleteWork(c.Request().Context(), id); err != nil {
		if handled, res := workErr(c, err); handled {
			return res
		}
		return errJSON(c, http.StatusInternalServerError, "failed to delete screen work")
	}
	return c.NoContent(http.StatusNoContent)
}

// MaxPosterRequestBytes is the body limit of the poster upload route: the
// image limit plus the multipart envelope.
const MaxPosterRequestBytes int64 = 6 << 20

// AdminUploadPoster godoc
// @Summary Загрузить постер фильма или сериала (админ)
// @Description Требуется роль admin и Bearer-токен. multipart/form-data: file (обязательно) - JPEG, PNG или WebP до 5 МиБ; тип определяется по содержимому, а не по имени файла или Content-Type. Постер сохраняется на сервере и заменяет любой прежний (источник manual); пометка «изменено вручную» у работы не ставится. Возвращает работу. Ошибки: 400 - нет файла, 413 - файл больше 5 МиБ, 422 - не JPEG/PNG/WebP, 404 - работа не найдена
// @Tags admin
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID фильма"
// @Param file formData file true "Постер (JPEG, PNG или WebP, до 5 МиБ)"
// @Success 200 {object} workDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 413 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/screen-works/{id}/poster [put]
func (h *Handler) AdminUploadPoster(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		var httpErr *echo.HTTPError
		if errors.As(err, &maxErr) || (errors.As(err, &httpErr) && httpErr.Code == http.StatusRequestEntityTooLarge) {
			return errJSON(c, http.StatusRequestEntityTooLarge, domain.ErrPosterTooLarge.Error())
		}
		return errJSON(c, http.StatusBadRequest, "file is required")
	}
	if fileHeader.Size > coverfetch.MaxBytes {
		return errJSON(c, http.StatusRequestEntityTooLarge, domain.ErrPosterTooLarge.Error())
	}
	file, err := fileHeader.Open()
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "failed to read uploaded file")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, coverfetch.MaxBytes+1))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "failed to read uploaded file")
	}
	d, err := h.service.UploadPoster(c.Request().Context(), id, data)
	if err != nil {
		if handled, res := workErr(c, err); handled {
			return res
		}
		return errJSON(c, http.StatusInternalServerError, "failed to upload poster")
	}
	return c.JSON(http.StatusOK, toWorkDetailResponse(*d))
}

// AdminDeletePoster godoc
// @Summary Убрать постер фильма или сериала (админ)
// @Description Требуется роль admin и Bearer-токен. Загруженный вручную постер удаляется (синхронизация может найти новый), автоматический (Commons, TMDB) помечается как убранный и синхронизацией больше не возвращается. Идемпотентно: без постера ничего не меняется. Возвращает работу. Работа не найдена - 404
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID фильма"
// @Success 200 {object} workDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/screen-works/{id}/poster [delete]
func (h *Handler) AdminDeletePoster(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	d, err := h.service.RemovePoster(c.Request().Context(), id)
	if err != nil {
		if handled, res := workErr(c, err); handled {
			return res
		}
		return errJSON(c, http.StatusInternalServerError, "failed to remove poster")
	}
	return c.JSON(http.StatusOK, toWorkDetailResponse(*d))
}
