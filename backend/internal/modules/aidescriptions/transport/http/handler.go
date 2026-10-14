// Package http is the admin HTTP surface of AI book descriptions
// (/admin/ai-descriptions/*): status, generate, the review list, one
// book's record, approve and discard. The API never calls the model; it
// queues worker tasks. Approve and discard are plain DB updates and work
// while the feature is switched off. It depends on small interfaces so it
// can be tested without Redis or a database.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	descservice "github.com/bookhaven/backend/internal/modules/aidescriptions/service"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

// TaskEnqueuer is the subset of *asynq.Client this handler depends on.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// DescriptionService is the subset of *descservice.Service this handler
// depends on.
type DescriptionService interface {
	Active() bool
	Now() time.Time
	Status(ctx context.Context) (*descservice.Status, error)
	List(ctx context.Context, status string, page, limit int) (*descservice.ListResult, error)
	Get(ctx context.Context, bookID uuid.UUID) (*domain.Item, error)
	Approve(ctx context.Context, bookID, userID uuid.UUID, text *string) (*domain.Item, error)
	Discard(ctx context.Context, bookID, userID uuid.UUID) (*domain.Item, error)
}

// Handler handles the AI descriptions endpoints.
type Handler struct {
	enqueuer TaskEnqueuer
	service  DescriptionService
}

// NewHandler builds a Handler.
func NewHandler(enqueuer TaskEnqueuer, service DescriptionService) *Handler {
	return &Handler{enqueuer: enqueuer, service: service}
}

type aiDescriptionsStatusResponse struct {
	// Enabled: the flag and the API key are both set.
	Enabled       bool   `json:"enabled"`
	Model         string `json:"model"`
	PromptVersion int    `json:"prompt_version"`
	// Missing: books an all_missing run would pick (without the cap).
	Missing int64 `json:"missing"`
	// Drafts: drafts whose book has no own description.
	Drafts  int64 `json:"drafts"`
	MaxBulk int   `json:"max_bulk"`
}

type generateAIDescriptionsRequest struct {
	BookIDs    *[]string `json:"book_ids"`
	AllMissing bool      `json:"all_missing"`
}

type generateAIDescriptionsResponse struct {
	TaskID string `json:"task_id"`
}

type approveAIDescriptionRequest struct {
	Text *string `json:"text"`
}

type aiDescriptionBookResponse struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Authors       []string `json:"authors"`
	PublishedYear *int     `json:"published_year"`
	Status        string   `json:"status"`
}

type aiDescriptionResponse struct {
	Status        string     `json:"status" enums:"draft,approved,discarded,unknown,failed"`
	Text          string     `json:"text"`
	ErrorCode     *string    `json:"error_code" enums:"model_refusal,model_truncated,invalid_output"`
	Model         string     `json:"model"`
	PromptVersion int        `json:"prompt_version"`
	GeneratedAt   time.Time  `json:"generated_at"`
	ReviewedAt    *time.Time `json:"reviewed_at"`
	Edited        bool       `json:"edited"`
	// Stale: the prompt version or the book's model input changed since
	// generation.
	Stale bool `json:"stale"`
	// Superseded: the book now has its own description (the AI one is not
	// public).
	Superseded bool `json:"superseded"`
}

type aiDescriptionItemResponse struct {
	Book aiDescriptionBookResponse `json:"book"`
	aiDescriptionResponse
}

type aiDescriptionDetailResponse struct {
	AIDescription *aiDescriptionResponse `json:"ai_description"`
}

type listAIDescriptionsResponse struct {
	Items []aiDescriptionItemResponse `json:"items"`
	Total int64                       `json:"total"`
	Page  int                         `json:"page"`
	Limit int                         `json:"limit"`
}

func toDescription(it *domain.Item) aiDescriptionResponse {
	return aiDescriptionResponse{
		Status:        it.Status,
		Text:          it.Text,
		ErrorCode:     it.ErrorCode,
		Model:         it.Model,
		PromptVersion: it.PromptVersion,
		GeneratedAt:   it.GeneratedAt,
		ReviewedAt:    it.ReviewedAt,
		Edited:        it.Edited,
		Stale:         it.Stale,
		Superseded:    it.Superseded,
	}
}

func toItem(it *domain.Item) aiDescriptionItemResponse {
	authors := it.Book.Authors
	if authors == nil {
		authors = []string{}
	}
	return aiDescriptionItemResponse{
		Book: aiDescriptionBookResponse{
			ID:            it.Book.ID.String(),
			Title:         it.Book.Title,
			Authors:       authors,
			PublishedYear: it.Book.PublishedYear,
			Status:        it.Book.Status,
		},
		aiDescriptionResponse: toDescription(it),
	}
}

// currentUserID reads the authenticated user's id set by the Auth
// middleware.
func currentUserID(c echo.Context) (uuid.UUID, bool) {
	raw, ok := c.Get("user_id").(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func errJSON(c echo.Context, code int, msg string) error {
	return c.JSON(code, map[string]string{"error": msg})
}

// AIDescriptionsStatus godoc
// @Summary Статус описаний ИИ (админ)
// @Description Требуется роль admin. enabled — включена ли генерация (флаг AI_DESCRIPTIONS_ENABLED и ключ API заданы), model — модель, prompt_version — версия промпта, missing — сколько книг выберет генерация «для книг без описания» (опубликованные, без собственного описания и без какой-либо записи описания ИИ; без учёта ограничения), drafts — черновики, у книг которых до сих пор нет собственного описания, max_bulk — ограничение одного запуска (200).
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} aiDescriptionsStatusResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/ai-descriptions/status [get]
func (h *Handler) AIDescriptionsStatus(c echo.Context) error {
	st, err := h.service.Status(c.Request().Context())
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to load AI descriptions status")
	}
	return c.JSON(http.StatusOK, aiDescriptionsStatusResponse{
		Enabled:       st.Enabled,
		Model:         st.Model,
		PromptVersion: descservice.PromptVersion,
		Missing:       st.Missing,
		Drafts:        st.Drafts,
		MaxBulk:       domain.MaxBulk,
	})
}

// GenerateAIDescriptions godoc
// @Summary Поставить генерацию описаний ИИ в очередь (админ)
// @Description Требуется роль admin и Bearer-токен. Ровно одна форма: {"book_ids": [...]} — 1–100 UUID (повторы убираются), каждая подходящая книга (без собственного описания, статус pending или published) генерируется заново, какой бы ни была её текущая запись; или {"all_missing": true} — до 200 опубликованных книг без собственного описания и без какой-либо записи описания ИИ, старые первыми. Генерация идёт только в фоновом воркере; результат — черновик, который одобряет администратор. В описание книги ничего не записывается. 503 — генерация выключена (проверяется первым); 400 — неверный запрос; 409 — генерация для книг без описания уже в очереди; 502 — не удалось поставить задачу в очередь.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body generateAIDescriptionsRequest true "book_ids или all_missing"
// @Success 202 {object} generateAIDescriptionsResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/ai-descriptions/generate [post]
func (h *Handler) GenerateAIDescriptions(c echo.Context) error {
	if !h.service.Active() {
		return errJSON(c, http.StatusServiceUnavailable, "AI descriptions are disabled")
	}
	var body generateAIDescriptionsRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&body); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid request")
	}
	if (body.BookIDs != nil) == body.AllMissing {
		return errJSON(c, http.StatusBadRequest, "exactly one of book_ids or all_missing is required")
	}

	var (
		task *asynq.Task
		opts []asynq.Option
		err  error
	)
	if body.AllMissing {
		task, err = descservice.NewSweepTask(domain.MaxBulk)
		opts = descservice.SweepOptions()
	} else {
		ids, msg := parseBookIDs(*body.BookIDs)
		if msg != "" {
			return errJSON(c, http.StatusBadRequest, msg)
		}
		task, err = descservice.NewGenerateTask(ids, h.service.Now())
		opts = descservice.GenerateOptions()
	}
	if err != nil {
		return errJSON(c, http.StatusBadGateway, "failed to queue generation")
	}
	info, err := h.enqueuer.Enqueue(task, opts...)
	if err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			return errJSON(c, http.StatusConflict, "AI description generation already queued")
		}
		return errJSON(c, http.StatusBadGateway, "failed to queue generation")
	}
	return c.JSON(http.StatusAccepted, generateAIDescriptionsResponse{TaskID: info.ID})
}

// parseBookIDs deduplicates raw ids (first occurrence order) and checks
// them; msg is non-empty for a 400.
func parseBookIDs(raw []string) ([]uuid.UUID, string) {
	if len(raw) == 0 {
		return nil, "book_ids must not be empty"
	}
	ids := make([]uuid.UUID, 0, len(raw))
	seen := make(map[uuid.UUID]struct{}, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, "invalid book id"
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) > domain.MaxIDs {
		return nil, "at most 100 book ids"
	}
	return ids, ""
}

// ListAIDescriptions godoc
// @Summary Список описаний ИИ (админ)
// @Description Требуется роль admin. status — draft (по умолчанию), approved, discarded, unknown или failed; новые первыми; limit 1–50 (по умолчанию 20). У каждой записи — книга (id, title, authors, published_year, status), статус, текст (показывать только как обычный текст), код ошибки, модель, время генерации, stale (с генерации изменились версия промпта или данные книги) и superseded (у книги появилось собственное описание). Работает и при выключенной генерации.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "Статус (draft, approved, discarded, unknown, failed)"
// @Param page query int false "Страница (с 1)"
// @Param limit query int false "Размер страницы (1–50)"
// @Success 200 {object} listAIDescriptionsResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/ai-descriptions [get]
func (h *Handler) ListAIDescriptions(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	res, err := h.service.List(c.Request().Context(), c.QueryParam("status"), page, limit)
	if err != nil {
		if errors.Is(err, descservice.ErrInvalidStatus) {
			return errJSON(c, http.StatusBadRequest, "invalid status")
		}
		return errJSON(c, http.StatusInternalServerError, "failed to list AI descriptions")
	}
	out := listAIDescriptionsResponse{Items: make([]aiDescriptionItemResponse, 0, len(res.Items)), Total: res.Total, Page: res.Page, Limit: res.Limit}
	for i := range res.Items {
		out.Items = append(out.Items, toItem(&res.Items[i]))
	}
	return c.JSON(http.StatusOK, out)
}

// GetAIDescription godoc
// @Summary Описание ИИ книги (админ)
// @Description Требуется роль admin. Возвращает запись описания ИИ книги или null. Работает и при выключенной генерации.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param book_id path string true "ID книги (UUID)"
// @Success 200 {object} aiDescriptionDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/ai-descriptions/{book_id} [get]
func (h *Handler) GetAIDescription(c echo.Context) error {
	bookID, err := uuid.Parse(c.Param("book_id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	it, err := h.service.Get(c.Request().Context(), bookID)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to get AI description")
	}
	resp := aiDescriptionDetailResponse{}
	if it != nil {
		d := toDescription(it)
		resp.AIDescription = &d
	}
	return c.JSON(http.StatusOK, resp)
}

// ApproveAIDescription godoc
// @Summary Одобрить описание ИИ (админ)
// @Description Требуется роль admin и Bearer-токен. Одобряет черновик или уже одобренное описание. Необязательный text заменяет сгенерированный текст (1–2000 символов после обрезки пробелов, в основном кириллица) и помечает описание изменённым. Описание книги не меняется: одобренное описание ИИ показывается на сайте с пометкой, пока у книги нет собственного описания. 404 — записи нет; 409 — запись не черновик и не одобрена, или у книги уже есть собственное описание; 422 — неверный текст. Работает и при выключенной генерации.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param book_id path string true "ID книги (UUID)"
// @Param body body approveAIDescriptionRequest false "Необязательный исправленный текст"
// @Success 200 {object} aiDescriptionItemResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/ai-descriptions/{book_id}/approve [post]
func (h *Handler) ApproveAIDescription(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return errJSON(c, http.StatusUnauthorized, "unauthorized")
	}
	bookID, err := uuid.Parse(c.Param("book_id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	var body approveAIDescriptionRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		return errJSON(c, http.StatusBadRequest, "invalid request")
	}
	it, err := h.service.Approve(c.Request().Context(), bookID, userID, body.Text)
	if err != nil {
		return reviewError(c, err)
	}
	if it == nil {
		return reviewError(c, descservice.ErrNotFound)
	}
	return c.JSON(http.StatusOK, toItem(it))
}

// DiscardAIDescription godoc
// @Summary Отклонить описание ИИ (админ)
// @Description Требуется роль admin и Bearer-токен. Переводит черновик или одобренное описание в отклонённые (одобренное снимается с сайта). 404 — записи нет; 409 — запись не черновик и не одобрена. Работает и при выключенной генерации.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param book_id path string true "ID книги (UUID)"
// @Success 200 {object} aiDescriptionItemResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/ai-descriptions/{book_id}/discard [post]
func (h *Handler) DiscardAIDescription(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return errJSON(c, http.StatusUnauthorized, "unauthorized")
	}
	bookID, err := uuid.Parse(c.Param("book_id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	it, err := h.service.Discard(c.Request().Context(), bookID, userID)
	if err != nil {
		return reviewError(c, err)
	}
	if it == nil {
		return reviewError(c, descservice.ErrNotFound)
	}
	return c.JSON(http.StatusOK, toItem(it))
}

func reviewError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, descservice.ErrInvalidText):
		return errJSON(c, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, descservice.ErrNotFound):
		return errJSON(c, http.StatusNotFound, "AI description not found")
	case errors.Is(err, descservice.ErrNotReviewable):
		return errJSON(c, http.StatusConflict, "AI description is not reviewable")
	case errors.Is(err, descservice.ErrHasOwnDescription):
		return errJSON(c, http.StatusConflict, "book already has a description")
	default:
		return errJSON(c, http.StatusInternalServerError, "failed to update AI description")
	}
}
