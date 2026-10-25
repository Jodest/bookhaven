package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// readResponse is one reading attempt of the reading history.
type readResponse struct {
	ID          string     `json:"id"`
	BookID      string     `json:"book_id"`
	State       string     `json:"state" enums:"open,finished,abandoned"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	AbandonedAt *time.Time `json:"abandoned_at"`
}

type readListResponse struct {
	Items []readResponse `json:"items"`
}

type createReadRequest struct {
	BookID     string  `json:"book_id"`
	StartedOn  *string `json:"started_on"`
	FinishedOn string  `json:"finished_on"`
}

// optionalDate tells an absent key from an explicit null: encoding/json calls
// UnmarshalJSON for null too, but not for a missing key.
type optionalDate struct {
	Set   bool
	Value *string
}

func (o *optionalDate) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	o.Value = &s
	return nil
}

func (o optionalDate) toService() service.OptionalDate {
	return service.OptionalDate{Set: o.Set, Value: o.Value}
}

// updateReadRequest is the PATCH body; swagger sees plain nullable strings.
type updateReadRequest struct {
	StartedOn   optionalDate `json:"started_on" swaggertype:"string" example:"2020-05-03"`
	FinishedOn  optionalDate `json:"finished_on" swaggertype:"string" example:"2020-06-01"`
	AbandonedOn optionalDate `json:"abandoned_on" swaggertype:"string" example:"2020-06-01"`
}

func toReadResponse(r domain.BookRead) readResponse {
	return readResponse{
		ID: r.ID.String(), BookID: r.BookID.String(), State: string(r.State()),
		StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, AbandonedAt: r.AbandonedAt,
	}
}

func jsonErr(c echo.Context, status int, msg string) error {
	return c.JSON(status, map[string]string{"error": msg})
}

// mapReadError turns service errors of the reading history into responses.
func mapReadError(c echo.Context, err error, fallback string) error {
	switch {
	case errors.Is(err, service.ErrReadNotFound):
		return jsonErr(c, http.StatusNotFound, "reading not found")
	case errors.Is(err, service.ErrBookNotFound):
		return jsonErr(c, http.StatusNotFound, "book not found")
	case errors.Is(err, service.ErrInvalidDate),
		errors.Is(err, service.ErrDateInFuture),
		errors.Is(err, service.ErrFinishBeforeStart),
		errors.Is(err, service.ErrFinishRequired),
		errors.Is(err, service.ErrStartRequired),
		errors.Is(err, service.ErrFieldNotEditable),
		errors.Is(err, service.ErrNothingToUpdate):
		return jsonErr(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrOpenReadNotDeletable),
		errors.Is(err, service.ErrReadChanged),
		errors.Is(err, service.ErrTooManyReads):
		return jsonErr(c, http.StatusConflict, err.Error())
	}
	return jsonErr(c, http.StatusInternalServerError, fallback)
}

// ListReads godoc
// @Summary История чтений книги
// @Description Собственные чтения вызывающего по книге, от старых к новым. Чужую историю получить нельзя
// @Tags reads
// @Produce json
// @Security BearerAuth
// @Param book_id query string true "ID книги"
// @Success 200 {object} readListResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reads [get]
func (h *Handler) ListReads(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return jsonErr(c, http.StatusUnauthorized, "unauthorized")
	}
	bookID, err := uuid.Parse(c.QueryParam("book_id"))
	if err != nil {
		return jsonErr(c, http.StatusBadRequest, "invalid book_id")
	}
	reads, err := h.service.ListReads(c.Request().Context(), userID, bookID)
	if err != nil {
		return mapReadError(c, err, "failed to list readings")
	}
	items := make([]readResponse, 0, len(reads))
	for _, r := range reads {
		items = append(items, toReadResponse(r))
	}
	return c.JSON(http.StatusOK, readListResponse{Items: items})
}

// CreateRead godoc
// @Summary Добавить прошлое прочтение
// @Description Записывает завершённое чтение с датами (YYYY-MM-DD, хранится как 12:00 UTC). Полки не меняются. Не больше 100 чтений на книгу
// @Tags reads
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createReadRequest true "Книга и даты"
// @Success 201 {object} readResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reads [post]
func (h *Handler) CreateRead(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return jsonErr(c, http.StatusUnauthorized, "unauthorized")
	}
	var req createReadRequest
	if err := c.Bind(&req); err != nil {
		return jsonErr(c, http.StatusBadRequest, "invalid request")
	}
	bookID, err := uuid.Parse(req.BookID)
	if err != nil {
		return jsonErr(c, http.StatusBadRequest, "invalid book_id")
	}
	read, err := h.service.AddPastRead(c.Request().Context(), userID, bookID, req.StartedOn, req.FinishedOn)
	if err != nil {
		return mapReadError(c, err, "failed to add reading")
	}
	return c.JSON(http.StatusCreated, toReadResponse(*read))
}

// UpdateRead godoc
// @Summary Изменить даты чтения
// @Description Открытое чтение меняет только started_on (не null). Завершённое: started_on (null очищает) и finished_on; брошенное: started_on и abandoned_on. Исход чтения не меняется
// @Tags reads
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID чтения"
// @Param body body updateReadRequest true "Изменяемые даты"
// @Success 200 {object} readResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reads/{id} [patch]
func (h *Handler) UpdateRead(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return jsonErr(c, http.StatusUnauthorized, "unauthorized")
	}
	readID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return jsonErr(c, http.StatusNotFound, "reading not found")
	}
	var req updateReadRequest
	if err := c.Bind(&req); err != nil {
		return jsonErr(c, http.StatusBadRequest, "invalid request")
	}
	read, err := h.service.UpdateRead(c.Request().Context(), userID, readID, service.ReadPatch{
		StartedOn: req.StartedOn.toService(), FinishedOn: req.FinishedOn.toService(), AbandonedOn: req.AbandonedOn.toService(),
	})
	if err != nil {
		return mapReadError(c, err, "failed to update reading")
	}
	return c.JSON(http.StatusOK, toReadResponse(*read))
}

// DeleteRead godoc
// @Summary Удалить чтение из истории
// @Description Удаляет завершённое или брошенное чтение. Открытое чтение удалить нельзя (409): оно следует статусу «Читаю». Полки не меняются
// @Tags reads
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID чтения"
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reads/{id} [delete]
func (h *Handler) DeleteRead(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return jsonErr(c, http.StatusUnauthorized, "unauthorized")
	}
	readID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return jsonErr(c, http.StatusNotFound, "reading not found")
	}
	if err := h.service.DeleteRead(c.Request().Context(), userID, readID); err != nil {
		return mapReadError(c, err, "failed to delete reading")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
