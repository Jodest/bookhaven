// Package http is the admin HTTP surface of AI pre-moderation: queueing a
// check (POST /admin/books/ai-review) and its status (GET
// /admin/books/ai-review/status). The API never calls the model; it only
// queues worker tasks. Like modules/harvest/transport/http it depends on
// small interfaces so it can be tested without Redis.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	aimodservice "github.com/bookhaven/backend/internal/modules/aimoderation/service"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

// TaskEnqueuer is the subset of *asynq.Client this handler depends on.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// CandidateCounter is the subset of *adminservice.Service this handler
// depends on.
type CandidateCounter interface {
	CountAIReviewCandidates(ctx context.Context, promptVersion int) (int64, error)
}

// Handler handles the AI review trigger and status endpoints.
type Handler struct {
	enqueuer TaskEnqueuer
	counter  CandidateCounter
	active   bool
	model    string
}

// NewHandler builds a Handler. active is config.AIModerationActive() (the
// same switch the worker uses), model the configured model id.
func NewHandler(enqueuer TaskEnqueuer, counter CandidateCounter, active bool, model string) *Handler {
	return &Handler{enqueuer: enqueuer, counter: counter, active: active, model: model}
}

// aiReviewRequest: exactly one of BookIDs or AllUnreviewed (true).
type aiReviewRequest struct {
	BookIDs       *[]string `json:"book_ids"`
	AllUnreviewed *bool     `json:"all_unreviewed"`
}

type aiReviewResponse struct {
	TaskID string `json:"task_id"`
}

type aiReviewStatusResponse struct {
	Enabled       bool   `json:"enabled"`
	Model         string `json:"model"`
	PromptVersion int    `json:"prompt_version"`
	// NeedsReview is the number of pending books the periodic sweep would
	// pick (no verdict, an older prompt version or updated after it).
	NeedsReview int64 `json:"needs_review"`
}

// TriggerAIReview godoc
// @Summary Поставить ИИ-проверку книг в очередь (админ)
// @Description Требуется роль admin и Bearer-токен. Ровно одно из: book_ids (1–100 UUID, дубли убираются; проверяются заново только книги на модерации, даже со свежим вердиктом) или all_unreviewed: true (одна проверка всех непроверенных книг на модерации, не более 1000). Вердикты вычисляет воркер; книги не публикуются автоматически. 503, если ИИ-модерация выключена; 409, если проверка всех непроверенных уже в очереди.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body aiReviewRequest true "book_ids или all_unreviewed"
// @Success 202 {object} aiReviewResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/books/ai-review [post]
func (h *Handler) TriggerAIReview(c echo.Context) error {
	if !h.active {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "AI moderation is disabled"})
	}

	var req aiReviewRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	all := req.AllUnreviewed != nil && *req.AllUnreviewed
	hasIDs := req.BookIDs != nil
	if all == hasIDs {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "exactly one of book_ids or all_unreviewed is required"})
	}

	var (
		task *asynq.Task
		opts []asynq.Option
		err  error
	)
	if all {
		task, err = aimodservice.NewSweepTask(aimodservice.ManualSweepMax, aimodservice.TriggerManual)
		opts = aimodservice.ManualSweepOptions()
	} else {
		ids, msg := parseBookIDs(*req.BookIDs)
		if msg != "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": msg})
		}
		task, err = aimodservice.NewReviewBooksTask(ids)
		opts = aimodservice.ReviewBooksOptions()
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to build AI review task"})
	}

	info, err := h.enqueuer.Enqueue(task, opts...)
	if err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "AI review already queued"})
		}
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to enqueue AI review"})
	}
	return c.JSON(http.StatusAccepted, aiReviewResponse{TaskID: info.ID})
}

// parseBookIDs deduplicates and validates book ids: 1..MaxReviewBookIDs
// UUIDs. A non-empty message means 400.
func parseBookIDs(raw []string) ([]uuid.UUID, string) {
	ids := make([]uuid.UUID, 0, len(raw))
	seen := make(map[uuid.UUID]struct{}, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, "book_ids must be UUIDs"
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, "book_ids must not be empty"
	}
	if len(ids) > aimodservice.MaxReviewBookIDs {
		return nil, "at most 100 book_ids are allowed"
	}
	return ids, ""
}

// AIReviewStatus godoc
// @Summary Статус ИИ-модерации книг (админ)
// @Description Требуется роль admin. enabled — включена ли ИИ-модерация (флаг и ключ API заданы), model — модель, prompt_version — текущая версия промпта, needs_review — число книг на модерации, которые возьмёт ближайшая проверка (без вердикта, со старой версией промпта или изменённые после вердикта).
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} aiReviewStatusResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/books/ai-review/status [get]
func (h *Handler) AIReviewStatus(c echo.Context) error {
	n, err := h.counter.CountAIReviewCandidates(c.Request().Context(), aimodservice.PromptVersion)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load AI review status"})
	}
	return c.JSON(http.StatusOK, aiReviewStatusResponse{
		Enabled:       h.active,
		Model:         h.model,
		PromptVersion: aimodservice.PromptVersion,
		NeedsReview:   n,
	})
}
