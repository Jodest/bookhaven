// Package http is the admin HTTP surface of the AI drafts of manual curated
// collections (/admin/ai-collections/*): status, create a draft, list a
// collection drafts, get one draft with its catalog match report. The API
// never calls the model; it stores the draft and queues a worker task. It
// depends on small interfaces so it can be tested without Redis or a
// database.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicollections/domain"
	draftservice "github.com/bookhaven/backend/internal/modules/aicollections/service"
	curatedhttp "github.com/bookhaven/backend/internal/modules/curated/transport/http"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

// TaskEnqueuer is the subset of *asynq.Client this handler depends on.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// DraftService is the subset of *draftservice.Service this handler depends
// on.
type DraftService interface {
	Active() bool
	Status(ctx context.Context, userID uuid.UUID) (*draftservice.Status, error)
	CreateDraft(ctx context.Context, userID uuid.UUID, collectionID, brief string, count *int) (*domain.Draft, error)
	MarkEnqueueFailed(ctx context.Context, id uuid.UUID) error
	ListDrafts(ctx context.Context, collectionID uuid.UUID) ([]domain.Draft, error)
	GetDraft(ctx context.Context, id uuid.UUID) (*domain.Draft, *draftservice.Report, error)
}

// Handler handles the AI collection draft endpoints.
type Handler struct {
	enqueuer TaskEnqueuer
	service  DraftService
}

// NewHandler builds a Handler.
func NewHandler(enqueuer TaskEnqueuer, service DraftService) *Handler {
	return &Handler{enqueuer: enqueuer, service: service}
}

type draftStatusResponse struct {
	// Enabled: the flag and the API key are both set.
	Enabled        bool   `json:"enabled"`
	Model          string `json:"model"`
	PromptVersion  int    `json:"prompt_version"`
	MaxBriefLength int    `json:"max_brief_length"`
	MaxCount       int    `json:"max_count"`
	DailyLimit     int    `json:"daily_limit"`
	UsedToday      int64  `json:"used_today"`
}

type createDraftRequest struct {
	CollectionID string `json:"collection_id"`
	Brief        string `json:"brief"`
	Count        *int   `json:"count"`
}

type draftCreatorResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type draftSummaryResponse struct {
	ID             string                `json:"id"`
	CollectionID   string                `json:"collection_id"`
	Brief          string                `json:"brief"`
	Count          int                   `json:"count"`
	Status         string                `json:"status" enums:"queued,running,done,failed"`
	ErrorCode      *string               `json:"error_code" enums:"model_refusal,model_truncated,invalid_output,model_error,enqueue_failed,timeout"`
	ProposalsCount int                   `json:"proposals_count"`
	Model          string                `json:"model"`
	PromptVersion  int                   `json:"prompt_version"`
	InputTokens    int                   `json:"input_tokens"`
	OutputTokens   int                   `json:"output_tokens"`
	CreatedBy      *draftCreatorResponse `json:"created_by"`
	CreatedAt      time.Time             `json:"created_at"`
	FinishedAt     *time.Time            `json:"finished_at"`
}

// draftLineResponse is a bulk-paste report entry plus the proposal original
// title and note.
type draftLineResponse struct {
	curatedhttp.PasteLineResponse
	OriginalTitle *string `json:"original_title"`
	Note          string  `json:"note"`
}

type draftReportResponse struct {
	BooksCount int                             `json:"books_count"`
	FreeSlots  int                             `json:"free_slots"`
	Counts     curatedhttp.PasteCountsResponse `json:"counts"`
	Lines      []draftLineResponse             `json:"lines"`
}

type draftDetailResponse struct {
	draftSummaryResponse
	// Report is null unless the draft is done.
	Report *draftReportResponse `json:"report"`
}

type listDraftsResponse struct {
	Items []draftSummaryResponse `json:"items"`
}

func toSummary(d *domain.Draft) draftSummaryResponse {
	out := draftSummaryResponse{
		ID:             d.ID.String(),
		CollectionID:   d.CollectionID.String(),
		Brief:          d.Brief,
		Count:          d.Count,
		Status:         d.Status,
		ErrorCode:      d.ErrorCode,
		ProposalsCount: d.ProposalsCount,
		Model:          d.Model,
		PromptVersion:  d.PromptVersion,
		InputTokens:    d.InputTokens,
		OutputTokens:   d.OutputTokens,
		CreatedAt:      d.CreatedAt,
		FinishedAt:     d.FinishedAt,
	}
	if d.Creator != nil {
		out.CreatedBy = &draftCreatorResponse{ID: d.Creator.ID.String(), Username: d.Creator.Username}
	}
	return out
}

func toReport(r *draftservice.Report) *draftReportResponse {
	p := r.Preview
	lines := make([]draftLineResponse, 0, len(p.Lines))
	for _, l := range p.Lines {
		line := draftLineResponse{PasteLineResponse: curatedhttp.ToPasteLineResponse(l)}
		if prop, ok := r.Proposals[l.Line]; ok {
			line.Note = prop.Note
			if prop.OriginalTitle != "" {
				ot := prop.OriginalTitle
				line.OriginalTitle = &ot
			}
		}
		lines = append(lines, line)
	}
	return &draftReportResponse{
		BooksCount: p.BooksCount, FreeSlots: p.FreeSlots, Counts: curatedhttp.ToPasteCountsResponse(p.Counts), Lines: lines,
	}
}

// currentUserID reads the authenticated user id set by the Auth middleware.
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

func unauthorized(c echo.Context) error {
	return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
}

// DraftsStatus godoc
// @Summary Статус черновиков подборок от ИИ (админ)
// @Description Требуется роль admin. enabled — включены ли черновики (флаг AI_COLLECTIONS_ENABLED и ключ API заданы), model — модель, prompt_version — версия промпта, лимиты (max_brief_length, max_count, daily_limit), used_today — число черновиков текущего администратора за последние 24 часа. Работает и при выключенной функции.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} draftStatusResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/ai-collections/status [get]
func (h *Handler) DraftsStatus(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return unauthorized(c)
	}
	st, err := h.service.Status(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load AI draft status"})
	}
	return c.JSON(http.StatusOK, draftStatusResponse{
		Enabled:        st.Enabled,
		Model:          st.Model,
		PromptVersion:  draftservice.PromptVersion,
		MaxBriefLength: domain.MaxBriefRunes,
		MaxCount:       domain.MaxCount,
		DailyLimit:     domain.DailyLimit,
		UsedToday:      st.UsedToday,
	})
}

// CreateDraft godoc
// @Summary Создать черновик подборки от ИИ (админ)
// @Description Требуется роль admin и Bearer-токен. collection_id — ручная подборка, brief — описание подборки (1–500 символов после обрезки пробелов), count — сколько книг предложить (1–1000; по умолчанию размер подборки плюс пятая часть, не больше 1000). Черновик сохраняется в статусе queued и составляется воркером: модель предлагает книги, сопоставление с каталогом выполняется при чтении черновика. Подборка и каталог не меняются. 503 — функция выключена; 400 — неверные collection_id, brief или count; 404 — подборка не найдена; 422 — подборка автоматическая; 409 — у подборки уже составляется черновик (draft_id); 429 — исчерпан дневной лимит (20 черновиков за 24 часа); 502 — не удалось поставить задачу в очередь (черновик помечается failed / enqueue_failed).
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createDraftRequest true "collection_id, brief и необязательный count"
// @Success 202 {object} draftSummaryResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 429 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/ai-collections/drafts [post]
func (h *Handler) CreateDraft(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return unauthorized(c)
	}
	if !h.service.Active() {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": draftservice.ErrDisabled.Error()})
	}
	var body createDraftRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := c.Request().Context()
	d, err := h.service.CreateDraft(ctx, userID, body.CollectionID, body.Brief, body.Count)
	if err != nil {
		var active *draftservice.ActiveDraftError
		switch {
		case errors.Is(err, draftservice.ErrDisabled):
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		case errors.Is(err, draftservice.ErrInvalidCollectionID), errors.Is(err, draftservice.ErrInvalidBrief), errors.Is(err, draftservice.ErrInvalidCount):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, draftservice.ErrCollectionNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		case errors.Is(err, draftservice.ErrCollectionAutomatic):
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		case errors.As(err, &active):
			return c.JSON(http.StatusConflict, map[string]string{"error": active.Error(), "draft_id": active.DraftID.String()})
		case errors.Is(err, draftservice.ErrDailyLimit):
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create AI draft"})
		}
	}

	task, err := draftservice.NewDraftTask(d.ID)
	if err == nil {
		_, err = h.enqueuer.Enqueue(task, draftservice.DraftOptions()...)
	}
	if err != nil {
		if mErr := h.service.MarkEnqueueFailed(ctx, d.ID); mErr != nil {
			c.Logger().Errorf("ai collections: mark enqueue_failed for %s: %v", d.ID, mErr)
		}
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to queue AI draft"})
	}
	return c.JSON(http.StatusAccepted, toSummary(d))
}

// ListDrafts godoc
// @Summary Недавние черновики подборки от ИИ (админ)
// @Description Требуется роль admin. 20 последних черновиков подборки collection_id, новые первыми. Черновик в статусе queued/running без прогресса больше часа показывается как failed с error_code timeout. Работает и при выключенной функции.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param collection_id query string true "ID подборки (UUID)"
// @Success 200 {object} listDraftsResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/ai-collections/drafts [get]
func (h *Handler) ListDrafts(c echo.Context) error {
	colID, err := uuid.Parse(c.QueryParam("collection_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "collection_id: must be a valid UUID"})
	}
	items, err := h.service.ListDrafts(c.Request().Context(), colID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list AI drafts"})
	}
	out := make([]draftSummaryResponse, 0, len(items))
	for i := range items {
		out = append(out, toSummary(&items[i]))
	}
	return c.JSON(http.StatusOK, listDraftsResponse{Items: out})
}

// GetDraft godoc
// @Summary Черновик подборки от ИИ с отчётом по каталогу (админ)
// @Description Требуется роль admin. Для готового черновика (done) поле report вычисляется при чтении по текущему каталогу и составу подборки: те же статусы, что у предпросмотра вставленного списка (found, ambiguous, not_found, rejected, in_collection, duplicate), по записи на каждое предложение в порядке рангов; line — ранг, original_title и note — мнение модели, только для показа как обычный текст. Для остальных статусов report равен null, а proposals_count показывает прогресс. Книга, которой нет в каталоге (not_found), не создаётся и не добавляется.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID черновика (UUID)"
// @Success 200 {object} draftDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/ai-collections/drafts/{id} [get]
func (h *Handler) GetDraft(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid draft id"})
	}
	d, report, err := h.service.GetDraft(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, draftservice.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "AI draft not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get AI draft"})
	}
	resp := draftDetailResponse{draftSummaryResponse: toSummary(d)}
	if report != nil {
		resp.Report = toReport(report)
	}
	return c.JSON(http.StatusOK, resp)
}
