// Package http implements the admin-facing HTTP trigger/observability
// surface for the catalog harvest pipeline (see
// docs/features/admin-harvest-trigger.md). It is a separate transport/http
// subpackage of the harvest module - not part of the admin module - so the
// admin module itself stays free of an asynq dependency (harvest/service
// already depends on adminservice, never the other way around; this package
// preserves that direction).
//
// Handler depends on two small interfaces (TaskEnqueuer, TaskInspector)
// rather than the concrete *asynq.Client/*asynq.Inspector types directly, so
// it can be unit tested with hand-rolled fakes instead of a real
// Redis-backed asynq instance (the repo's existing no-mocking-library
// convention).
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

// jobsPageSize bounds each per-state task listing in GET /admin/harvest/jobs
// to the most recent 50 tasks. "Recent" here means up to 50 most-recent
// tasks per state (pending/active/retry/completed/archived), which given the
// accepted low-frequency usage pattern for this MVP tool (occasional,
// low-volume triggering by a small number of admins) comfortably covers
// realistic admin usage. Revisit only if it proves too small in practice.
const jobsPageSize = 50

// TaskEnqueuer is the subset of *asynq.Client this handler depends on.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// TaskInspector is the subset of *asynq.Inspector this handler depends on.
type TaskInspector interface {
	GetTaskInfo(qname, id string) (*asynq.TaskInfo, error)
	ListPendingTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListActiveTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListRetryTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListArchivedTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListCompletedTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
}

// AuthorSearcher is the subset of *bookdata.WikidataClient this handler
// depends on for GET /admin/harvest/authors/search (satisfied directly by
// *bookdata.WikidataClient, no adapter needed).
type AuthorSearcher interface {
	SearchAuthors(ctx context.Context, query string) ([]bookdata.AuthorCandidate, error)
}

// Handler handles admin harvest-trigger/job-status/author-search HTTP
// requests.
type Handler struct {
	enqueuer  TaskEnqueuer
	inspector TaskInspector
	searcher  AuthorSearcher
}

// NewHandler builds a Handler.
func NewHandler(enqueuer TaskEnqueuer, inspector TaskInspector, searcher AuthorSearcher) *Handler {
	return &Handler{enqueuer: enqueuer, inspector: inspector, searcher: searcher}
}

type triggerHarvestRequest struct {
	AuthorQIDs []string `json:"author_qids"`
	Limit      int      `json:"limit"`
}

type triggerHarvestResponse struct {
	TaskID string `json:"task_id"`
	Queue  string `json:"queue"`
	State  string `json:"state"`
}

// harvestJobResponse is the shape returned for both GET /jobs and
// GET /jobs/:id, mapped from an *asynq.TaskInfo.
type harvestJobResponse struct {
	ID        string                        `json:"id"`
	Queue     string                        `json:"queue"`
	State     string                        `json:"state"`
	Type      string                        `json:"type"`
	MaxRetry  int                           `json:"max_retry"`
	Retried   int                           `json:"retried"`
	LastError string                        `json:"last_error,omitempty"`
	Result    *harvestservice.HarvestResult `json:"result,omitempty"`
}

func toHarvestJobResponse(info *asynq.TaskInfo) harvestJobResponse {
	resp := harvestJobResponse{
		ID:        info.ID,
		Queue:     info.Queue,
		State:     info.State.String(),
		Type:      info.Type,
		MaxRetry:  info.MaxRetry,
		Retried:   info.Retried,
		LastError: info.LastErr,
	}
	// A TaskInfo.Result that fails to unmarshal is treated as absent, not a
	// server error - defensive only, not expected to trigger in practice
	// (Result is untyped []byte from asynq's perspective, but this handler
	// only ever writes it via harvestservice.HandleTask's
	// json.Marshal(HarvestResult)).
	if len(info.Result) > 0 {
		var result harvestservice.HarvestResult
		if err := json.Unmarshal(info.Result, &result); err == nil {
			resp.Result = &result
		}
	}
	return resp
}

// TriggerHarvest godoc
// @Summary Запустить харвест по списку авторов (Wikidata)
// @Description Требуется роль admin и Bearer-токен (state-changing запросы не принимаются по cookie). Ставит задачу харвеста в очередь; сам харвест выполняется асинхронно воркером.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body triggerHarvestRequest true "QID авторов Wikidata и опциональный лимит"
// @Success 201 {object} triggerHarvestResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Router /admin/harvest [post]
func (h *Handler) TriggerHarvest(c echo.Context) error {
	var req triggerHarvestRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	query, err := bookdata.BuildAuthorListQuery(req.AuthorQIDs, req.Limit)
	if err != nil {
		if errors.Is(err, bookdata.ErrInvalidQuery) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to build harvest query"})
	}

	task, err := harvestservice.NewHarvestTask(query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to build harvest task"})
	}

	info, err := h.enqueuer.Enqueue(task)
	if err != nil {
		// 502, not 500: this is an upstream-dependency (Redis/asynq) failure,
		// not a server bug - mirrors admin.LookupBook's use of 502 for
		// downstream-source failures (bookdata.ErrRateLimited/
		// ErrSourceUnavailable). See docs/features/admin-harvest-trigger.md
		// Acceptance Criteria for the explicit 502-vs-500 discussion.
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to enqueue harvest job"})
	}

	return c.JSON(http.StatusCreated, triggerHarvestResponse{
		TaskID: info.ID,
		Queue:  info.Queue,
		State:  info.State.String(),
	})
}

type listHarvestJobsResponse struct {
	Items []harvestJobResponse `json:"items"`
}

// ListHarvestJobs godoc
// @Summary Список задач харвеста (недавние)
// @Description Требуется роль admin. Источник данных - состояние очереди asynq/Redis, а не отдельная таблица в БД; список ограничен 50 задачами на состояние и сгруппирован по состоянию (active, retry, pending, completed, archived), не единой хронологией.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} listHarvestJobsResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/harvest/jobs [get]
func (h *Handler) ListHarvestJobs(c echo.Context) error {
	opts := []asynq.ListOption{asynq.PageSize(jobsPageSize)}

	// Fixed order: active -> retry -> pending -> completed -> archived. This
	// is not a global chronological sort - asynq.TaskInfo doesn't expose a
	// single comparable timestamp valid across all states - see
	// docs/features/admin-harvest-trigger.md Acceptance Criteria / Risks.
	listers := []func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error){
		h.inspector.ListActiveTasks,
		h.inspector.ListRetryTasks,
		h.inspector.ListPendingTasks,
		h.inspector.ListCompletedTasks,
		h.inspector.ListArchivedTasks,
	}

	items := make([]harvestJobResponse, 0)
	for _, list := range listers {
		infos, err := list(harvestservice.QueueName, opts...)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list harvest jobs"})
		}
		for _, info := range infos {
			items = append(items, toHarvestJobResponse(info))
		}
	}

	return c.JSON(http.StatusOK, listHarvestJobsResponse{Items: items})
}

// GetHarvestJob godoc
// @Summary Статус задачи харвеста по ID
// @Description Требуется роль admin.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID задачи asynq"
// @Success 200 {object} harvestJobResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/harvest/jobs/{id} [get]
func (h *Handler) GetHarvestJob(c echo.Context) error {
	id := c.Param("id")

	info, err := h.inspector.GetTaskInfo(harvestservice.QueueName, id)
	if err != nil {
		if errors.Is(err, asynq.ErrTaskNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "harvest job not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get harvest job"})
	}

	return c.JSON(http.StatusOK, toHarvestJobResponse(info))
}

// authorCandidateResponse is one item of listAuthorCandidatesResponse,
// mapped from bookdata.AuthorCandidate.
type authorCandidateResponse struct {
	QID         string `json:"qid"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// listAuthorCandidatesResponse is the 200 response of
// GET /admin/harvest/authors/search.
type listAuthorCandidatesResponse struct {
	Items []authorCandidateResponse `json:"items"`
}

// SearchAuthors godoc
// @Summary Поиск авторов в Wikidata по имени (для подбора QID перед харвестом)
// @Description Требуется роль admin. Синхронный, только для чтения вызов Wikidata (wbsearchentities) - не создаёт и не изменяет записи; результат - кандидаты для ручного подтверждения администратором перед запуском POST /admin/harvest. Отсутствие совпадений - не ошибка (200, items: []).
// @Tags admin
// @Produce json
// @Param q query string true "Искомое имя автора"
// @Success 200 {object} listAuthorCandidatesResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/harvest/authors/search [get]
func (h *Handler) SearchAuthors(c echo.Context) error {
	q := strings.TrimSpace(c.QueryParam("q"))
	if q == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "q is required"})
	}

	candidates, err := h.searcher.SearchAuthors(c.Request().Context(), q)
	if err != nil {
		switch {
		case errors.Is(err, bookdata.ErrInvalidQuery):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, bookdata.ErrRateLimited), errors.Is(err, bookdata.ErrSourceUnavailable):
			// 502, not 500: an upstream-dependency (Wikidata) failure, not a
			// server bug - mirrors admin.LookupBook's convention. Unlike book
			// lookup, this endpoint always has a manual-QID escape hatch in
			// the UI, so the message points the admin at it.
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "search source is unavailable; retry later or enter the QID manually"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to search authors"})
		}
	}

	items := make([]authorCandidateResponse, 0, len(candidates))
	for _, cand := range candidates {
		items = append(items, authorCandidateResponse{
			QID:         cand.QID,
			Label:       cand.Label,
			Description: cand.Description,
		})
	}

	return c.JSON(http.StatusOK, listAuthorCandidatesResponse{Items: items})
}
