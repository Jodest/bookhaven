// Package http is the admin HTTP surface of the AI curator of author
// import lists (/admin/ai-curator/*): status, create a request, list recent
// requests, get one request with its candidates. The API never calls the
// model; it stores the request and queues a worker task. It depends on
// small interfaces so it can be tested without Redis or a database.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	curatorservice "github.com/bookhaven/backend/internal/modules/aicurator/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

// TaskEnqueuer is the subset of *asynq.Client this handler depends on.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// CurationService is the subset of *curatorservice.Service this handler
// depends on.
type CurationService interface {
	Active() bool
	Status(ctx context.Context, userID uuid.UUID) (*curatorservice.Status, error)
	CreateRequest(ctx context.Context, userID uuid.UUID, prompt string, maxNames *int) (*domain.Request, error)
	MarkEnqueueFailed(ctx context.Context, id uuid.UUID) error
	ListRecent(ctx context.Context) ([]domain.Request, error)
	GetRequest(ctx context.Context, id uuid.UUID) (*domain.Request, map[string]catalogDomain.Author, error)
}

// Handler handles the AI curator endpoints.
type Handler struct {
	enqueuer TaskEnqueuer
	service  CurationService
}

// NewHandler builds a Handler.
func NewHandler(enqueuer TaskEnqueuer, service CurationService) *Handler {
	return &Handler{enqueuer: enqueuer, service: service}
}

type curationStatusResponse struct {
	// Enabled: the flag and the API key are both set.
	Enabled         bool    `json:"enabled"`
	Model           string  `json:"model"`
	PromptVersion   int     `json:"prompt_version"`
	MaxPromptLength int     `json:"max_prompt_length"`
	MaxNames        int     `json:"max_names"`
	DefaultNames    int     `json:"default_names"`
	DailyLimit      int     `json:"daily_limit"`
	UsedToday       int64   `json:"used_today"`
	ActiveRequestID *string `json:"active_request_id"`
}

type createCurationRequest struct {
	Prompt   string `json:"prompt"`
	MaxNames *int   `json:"max_names"`
}

type curationCreatorResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type curationSummaryResponse struct {
	ID            string                   `json:"id"`
	Prompt        string                   `json:"prompt"`
	MaxNames      int                      `json:"max_names"`
	Status        string                   `json:"status" enums:"queued,running,done,failed"`
	ErrorCode     *string                  `json:"error_code" enums:"model_refusal,model_truncated,invalid_output,model_error,enqueue_failed,timeout"`
	Model         string                   `json:"model"`
	PromptVersion int                      `json:"prompt_version"`
	CreatedBy     *curationCreatorResponse `json:"created_by"`
	CreatedAt     time.Time                `json:"created_at"`
	FinishedAt    *time.Time               `json:"finished_at"`
	// CandidatesTotal / CandidatesResolved drive the progress «Проверено в
	// Wikidata: X из Y».
	CandidatesTotal    int `json:"candidates_total"`
	CandidatesResolved int `json:"candidates_resolved"`
}

type curationExistingAuthorResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Status string `json:"status"`
}

type curationOptionResponse struct {
	QID         string `json:"qid"`
	Label       string `json:"label"`
	Description string `json:"description"`
	// Existing is the catalog author now carrying this QID, or null (also
	// null when the lookup failed).
	Existing *curationExistingAuthorResponse `json:"existing"`
}

type curationCandidateResponse struct {
	ID          string                   `json:"id"`
	Position    int                      `json:"position"`
	Name        string                   `json:"name"`
	Note        string                   `json:"note"`
	MatchStatus string                   `json:"match_status" enums:"pending,matched,ambiguous,not_found,error"`
	Options     []curationOptionResponse `json:"options"`
}

type curationDetailResponse struct {
	curationSummaryResponse
	InputTokens  int                         `json:"input_tokens"`
	OutputTokens int                         `json:"output_tokens"`
	Candidates   []curationCandidateResponse `json:"candidates"`
}

type listCurationRequestsResponse struct {
	Items []curationSummaryResponse `json:"items"`
}

func toSummary(r *domain.Request) curationSummaryResponse {
	out := curationSummaryResponse{
		ID:                 r.ID.String(),
		Prompt:             r.Prompt,
		MaxNames:           r.MaxNames,
		Status:             r.Status,
		ErrorCode:          r.ErrorCode,
		Model:              r.Model,
		PromptVersion:      r.PromptVersion,
		CreatedAt:          r.CreatedAt,
		FinishedAt:         r.FinishedAt,
		CandidatesTotal:    r.CandidatesTotal,
		CandidatesResolved: r.CandidatesResolved,
	}
	if r.Creator != nil {
		out.CreatedBy = &curationCreatorResponse{ID: r.Creator.ID.String(), Username: r.Creator.Username}
	}
	return out
}

func toDetail(r *domain.Request, existing map[string]catalogDomain.Author) curationDetailResponse {
	out := curationDetailResponse{
		curationSummaryResponse: toSummary(r),
		InputTokens:             r.InputTokens,
		OutputTokens:            r.OutputTokens,
		Candidates:              make([]curationCandidateResponse, 0, len(r.Candidates)),
	}
	for _, c := range r.Candidates {
		cand := curationCandidateResponse{
			ID:          c.ID.String(),
			Position:    c.Position,
			Name:        c.Name,
			Note:        c.Note,
			MatchStatus: c.MatchStatus,
			Options:     make([]curationOptionResponse, 0, len(c.Options)),
		}
		for _, o := range c.Options {
			opt := curationOptionResponse{QID: o.QID, Label: o.Label, Description: o.Description}
			if a, ok := existing[o.QID]; ok {
				opt.Existing = &curationExistingAuthorResponse{ID: a.ID.String(), Name: a.Name, Slug: a.Slug, Status: a.Status}
			}
			cand.Options = append(cand.Options, opt)
		}
		out.Candidates = append(out.Candidates, cand)
	}
	return out
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

func unauthorized(c echo.Context) error {
	return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
}

// CurationStatus godoc
// @Summary Статус подбора авторов ИИ (админ)
// @Description Требуется роль admin. enabled — включён ли подбор (флаг AI_CURATOR_ENABLED и ключ API заданы), model — модель, prompt_version — версия промпта, лимиты (max_prompt_length, max_names, default_names, daily_limit), used_today — число запросов текущего администратора за последние 24 часа, active_request_id — его активный (в очереди или выполняется) запрос или null.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} curationStatusResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/ai-curator/status [get]
func (h *Handler) CurationStatus(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return unauthorized(c)
	}
	st, err := h.service.Status(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load curation status"})
	}
	resp := curationStatusResponse{
		Enabled:         st.Enabled,
		Model:           st.Model,
		PromptVersion:   curatorservice.PromptVersion,
		MaxPromptLength: domain.MaxPromptRunes,
		MaxNames:        domain.MaxNames,
		DefaultNames:    domain.DefaultNames,
		DailyLimit:      domain.DailyLimit,
		UsedToday:       st.UsedToday,
	}
	if st.ActiveRequestID != nil {
		s := st.ActiveRequestID.String()
		resp.ActiveRequestID = &s
	}
	return c.JSON(http.StatusOK, resp)
}

// CreateCurationRequest godoc
// @Summary Создать запрос на подбор авторов ИИ (админ)
// @Description Требуется роль admin и Bearer-токен. prompt — описание, каких авторов не хватает (1–500 символов после обрезки пробелов), max_names — сколько имён предложить (1–30, по умолчанию 20). Запрос сохраняется в статусе queued и обрабатывается воркером: модель предлагает только имена, каждое проверяется по Wikidata. Ничего не импортируется и не публикуется автоматически. 503 — подбор выключен; 400 — неверный prompt или max_names; 409 — у администратора уже есть активный запрос (request_id); 429 — исчерпан дневной лимит (20 запросов за 24 часа); 502 — не удалось поставить задачу в очередь (запрос помечается failed / enqueue_failed).
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createCurationRequest true "prompt и необязательный max_names"
// @Success 202 {object} curationSummaryResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 429 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/ai-curator/requests [post]
func (h *Handler) CreateCurationRequest(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return unauthorized(c)
	}
	if !h.service.Active() {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "AI curation is disabled"})
	}
	var body createCurationRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := c.Request().Context()
	req, err := h.service.CreateRequest(ctx, userID, body.Prompt, body.MaxNames)
	if err != nil {
		var active *curatorservice.ActiveRequestError
		switch {
		case errors.Is(err, curatorservice.ErrDisabled):
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "AI curation is disabled"})
		case errors.Is(err, curatorservice.ErrInvalidPrompt), errors.Is(err, curatorservice.ErrInvalidMaxNames):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.As(err, &active):
			return c.JSON(http.StatusConflict, map[string]string{"error": "curation request already in progress", "request_id": active.RequestID.String()})
		case errors.Is(err, curatorservice.ErrDailyLimit):
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "daily curation limit reached"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create curation request"})
		}
	}

	task, err := curatorservice.NewCurateTask(req.ID)
	if err == nil {
		_, err = h.enqueuer.Enqueue(task, curatorservice.CurateOptions()...)
	}
	if err != nil {
		if mErr := h.service.MarkEnqueueFailed(ctx, req.ID); mErr != nil {
			c.Logger().Errorf("ai curator: mark enqueue_failed for %s: %v", req.ID, mErr)
		}
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to queue curation request"})
	}
	return c.JSON(http.StatusAccepted, toSummary(req))
}

// ListCurationRequests godoc
// @Summary Недавние запросы подбора авторов ИИ (админ)
// @Description Требуется роль admin. 20 последних запросов всех администраторов, новые первыми. Запрос в статусе queued/running без прогресса больше часа показывается как failed с error_code timeout. Работает и при выключенном подборе.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} listCurationRequestsResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/ai-curator/requests [get]
func (h *Handler) ListCurationRequests(c echo.Context) error {
	items, err := h.service.ListRecent(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list curation requests"})
	}
	out := make([]curationSummaryResponse, 0, len(items))
	for i := range items {
		out = append(out, toSummary(&items[i]))
	}
	return c.JSON(http.StatusOK, listCurationRequestsResponse{Items: out})
}

// GetCurationRequest godoc
// @Summary Запрос подбора авторов ИИ с кандидатами (админ)
// @Description Требуется роль admin. Кандидаты в порядке ответа модели; у каждого match_status (pending, matched, ambiguous, not_found, error) и options — люди из Wikidata с русской меткой (qid, label, description). Поле existing вычисляется при чтении: автор BookHaven с этим QID (id, name, slug, status) или null. Имя и note — мнение модели, только для показа как обычный текст. Ничего не выбирается автоматически.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID запроса (UUID)"
// @Success 200 {object} curationDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/ai-curator/requests/{id} [get]
func (h *Handler) GetCurationRequest(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request id"})
	}
	req, existing, err := h.service.GetRequest(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, curatorservice.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "curation request not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get curation request"})
	}
	return c.JSON(http.StatusOK, toDetail(req, existing))
}
