// Package http is the admin HTTP surface of the AI genre suggestions
// (/admin/genres/ai-suggestions/*): list, status, queueing an analysis, apply
// and dismiss (single and bulk). The API never calls the model; it only
// queues worker tasks, and it changes genres only by applying a suggestion
// through the existing batch genre merge or genre bulk status. It depends on
// small interfaces so it can be tested without Redis or a database.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	adminrepository "github.com/bookhaven/backend/internal/modules/admin/repository"
	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	aigenresservice "github.com/bookhaven/backend/internal/modules/aigenres/service"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

// TaskEnqueuer is the subset of *asynq.Client this handler depends on.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// SuggestionService is the subset of *aigenresservice.Service this handler
// depends on.
type SuggestionService interface {
	Active() bool
	Status(ctx context.Context) (*aigenresservice.Status, error)
	StartAnalysis(ctx context.Context, userID uuid.UUID) (*domain.Run, error)
	MarkRunEnqueueFailed(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, f aigenresservice.ListFilter) ([]aigenresservice.SuggestionView, int64, error)
	Apply(ctx context.Context, userID, id uuid.UUID, targetOverride *uuid.UUID) (*aigenresservice.ApplyResult, error)
	ApplyBulk(ctx context.Context, userID uuid.UUID, items []aigenresservice.ApplyItem) *aigenresservice.BulkApplyReport
	Dismiss(ctx context.Context, userID, id uuid.UUID) (*aigenresservice.SuggestionView, error)
	DismissBulk(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) *aigenresservice.BulkDismissReport
}

// Handler handles the AI genre suggestion endpoints.
type Handler struct {
	enqueuer TaskEnqueuer
	service  SuggestionService
}

// NewHandler builds a Handler.
func NewHandler(enqueuer TaskEnqueuer, service SuggestionService) *Handler {
	return &Handler{enqueuer: enqueuer, service: service}
}

// ---- response types ----

type genreRefResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type sampleBookResponse struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type suggestionGenreResponse struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Status      string               `json:"status" enums:"pending,published,rejected"`
	Merged      bool                 `json:"merged"`
	Exists      bool                 `json:"exists"`
	WikidataQID *string              `json:"wikidata_qid"`
	BooksTotal  int64                `json:"books_total"`
	SampleBooks []sampleBookResponse `json:"sample_books"`
}

type suggestionResponse struct {
	ID            string                    `json:"id"`
	Kind          string                    `json:"kind" enums:"duplicate,narrow,junk"`
	Status        string                    `json:"status" enums:"open,applied,dismissed,stale"`
	ReasonCodes   []string                  `json:"reason_codes"`
	Reason        string                    `json:"reason"`
	Confidence    float64                   `json:"confidence"`
	Model         string                    `json:"model"`
	PromptVersion int                       `json:"prompt_version"`
	CreatedAt     time.Time                 `json:"created_at"`
	DecidedAt     *time.Time                `json:"decided_at"`
	Stale         bool                      `json:"stale"`
	Target        *genreRefResponse         `json:"target"`
	Genres        []suggestionGenreResponse `json:"genres"`
	AppliedTarget *genreRefResponse         `json:"applied_target"`
}

type listResponse struct {
	Items []suggestionResponse `json:"items"`
	Total int64                `json:"total"`
}

type runResponse struct {
	ID                 string     `json:"id"`
	Trigger            string     `json:"trigger" enums:"manual,scheduled"`
	Status             string     `json:"status" enums:"queued,running,done,failed"`
	BatchesTotal       int        `json:"batches_total"`
	BatchesDone        int        `json:"batches_done"`
	SuggestionsCreated int        `json:"suggestions_created"`
	ErrorCode          *string    `json:"error_code"`
	CreatedAt          time.Time  `json:"created_at"`
	FinishedAt         *time.Time `json:"finished_at"`
}

type statusResponse struct {
	// Enabled: the flag and the API key are both set.
	Enabled       bool         `json:"enabled"`
	Model         string       `json:"model"`
	PromptVersion int          `json:"prompt_version"`
	OpenCount     int64        `json:"open_count"`
	LastRun       *runResponse `json:"last_run"`
}

type analyzeResponse struct {
	RunID string `json:"run_id"`
}

// mergeGenreResponse is the genre of a merge result (the shape of the admin
// genre payload).
type mergeGenreResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Status      string  `json:"status"`
	WikidataQID *string `json:"wikidata_qid"`
}

// mergeResponse has the JSON shape of POST /admin/genres/merge-batch.
type mergeResponse struct {
	Genre               mergeGenreResponse `json:"genre"`
	SourcesMerged       int                `json:"sources_merged"`
	BooksReassigned     int64              `json:"books_reassigned"`
	CollectionsSwitched int                `json:"collections_switched"`
}

type statusReportRowError struct {
	GenreID string `json:"genre_id"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

// statusReportResponse has the JSON shape of POST /admin/genres/bulk-status.
type statusReportResponse struct {
	Processed int                    `json:"processed"`
	Updated   int                    `json:"updated"`
	Errors    []statusReportRowError `json:"errors"`
}

type applyResponse struct {
	Suggestion   suggestionResponse    `json:"suggestion"`
	Merge        *mergeResponse        `json:"merge"`
	StatusReport *statusReportResponse `json:"status_report"`
}

type applyRequest struct {
	TargetID *string `json:"target_id"`
}

type bulkApplyItem struct {
	ID       string  `json:"id"`
	TargetID *string `json:"target_id"`
}

type bulkApplyRequest struct {
	Items []bulkApplyItem `json:"items"`
}

type itemErrorResponse struct {
	SuggestionID string `json:"suggestion_id"`
	Message      string `json:"message"`
}

type bulkApplyResponse struct {
	Processed int                 `json:"processed"`
	Applied   int                 `json:"applied"`
	Stale     int                 `json:"stale"`
	Errors    []itemErrorResponse `json:"errors"`
}

type bulkDismissRequest struct {
	IDs []string `json:"ids"`
}

type bulkDismissResponse struct {
	Processed int                 `json:"processed"`
	Dismissed int                 `json:"dismissed"`
	Errors    []itemErrorResponse `json:"errors"`
}

// ---- mapping ----

func toRef(r *aigenresservice.GenreRef) *genreRefResponse {
	if r == nil {
		return nil
	}
	return &genreRefResponse{ID: r.ID.String(), Name: r.Name}
}

func toSuggestion(v aigenresservice.SuggestionView) suggestionResponse {
	s := v.Suggestion
	genres := make([]suggestionGenreResponse, 0, len(v.Genres))
	for _, g := range v.Genres {
		samples := make([]sampleBookResponse, 0, len(g.SampleBooks))
		for _, b := range g.SampleBooks {
			samples = append(samples, toSample(b))
		}
		genres = append(genres, suggestionGenreResponse{
			ID: g.ID.String(), Name: g.Name, Status: g.Status, Merged: g.Merged, Exists: g.Exists,
			WikidataQID: g.WikidataQID, BooksTotal: g.BooksTotal, SampleBooks: samples,
		})
	}
	codes := s.ReasonCodes
	if codes == nil {
		codes = []string{}
	}
	return suggestionResponse{
		ID: s.ID.String(), Kind: s.Kind, Status: s.Status, ReasonCodes: codes, Reason: s.Reason,
		Confidence: s.Confidence, Model: s.Model, PromptVersion: s.PromptVersion, CreatedAt: s.CreatedAt,
		DecidedAt: s.DecidedAt, Stale: v.Stale, Target: toRef(v.Target), Genres: genres,
		AppliedTarget: toRef(v.AppliedTarget),
	}
}

func toSample(b adminrepository.SampleBook) sampleBookResponse {
	return sampleBookResponse{ID: b.ID.String(), Title: b.Title, Status: b.Status}
}

func toRun(r *domain.Run) *runResponse {
	if r == nil {
		return nil
	}
	return &runResponse{
		ID: r.ID.String(), Trigger: r.Trigger, Status: r.Status, BatchesTotal: r.BatchesTotal,
		BatchesDone: r.BatchesDone, SuggestionsCreated: r.SuggestionsCreated, ErrorCode: r.ErrorCode,
		CreatedAt: r.CreatedAt, FinishedAt: r.FinishedAt,
	}
}

func toItemErrors(errs []aigenresservice.ItemError) []itemErrorResponse {
	out := make([]itemErrorResponse, 0, len(errs))
	for _, e := range errs {
		out = append(out, itemErrorResponse{SuggestionID: e.SuggestionID, Message: e.Message})
	}
	return out
}

func errBody(msg string) map[string]string { return map[string]string{"error": msg} }

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
	return c.JSON(http.StatusUnauthorized, errBody("unauthorized"))
}

// ---- handlers ----

// ListSuggestions godoc
// @Summary Предложения ИИ по жанрам (админ)
// @Description Требуется роль admin. status — open (по умолчанию), applied, dismissed или stale; kind — duplicate, narrow или junk (необязательно); page и limit (по умолчанию 20, не более 50). Открытые предложения идут по убыванию уверенности, затем новые первыми; остальные — по времени решения, новые первыми. У каждого предложения genres перечисляет все его жанры (целевой первым) с текущими значениями; жанр, которого больше нет, показывается с сохранённым названием и exists = false. stale вычисляется при чтении для открытых предложений: жанр предложения удалён, слит или отклонён после анализа. Работает и при выключенной функции.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "Статус" Enums(open,applied,dismissed,stale) default(open)
// @Param kind query string false "Вид предложения" Enums(duplicate,narrow,junk)
// @Param page query int false "Страница" default(1)
// @Param limit query int false "Размер страницы" default(20)
// @Success 200 {object} listResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres/ai-suggestions [get]
func (h *Handler) ListSuggestions(c echo.Context) error {
	page, limit := 1, 0
	if raw := c.QueryParam("page"); raw != "" {
		n, err := parsePositive(raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, errBody("invalid page"))
		}
		page = n
	}
	if raw := c.QueryParam("limit"); raw != "" {
		n, err := parsePositive(raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, errBody("invalid limit"))
		}
		limit = n
	}
	views, total, err := h.service.List(c.Request().Context(), aigenresservice.ListFilter{
		Status: c.QueryParam("status"), Kind: c.QueryParam("kind"), Page: page, Limit: limit,
	})
	if err != nil {
		if errors.Is(err, aigenresservice.ErrInvalidFilter) {
			return c.JSON(http.StatusBadRequest, errBody("status must be open, applied, dismissed or stale; kind must be duplicate, narrow or junk"))
		}
		return c.JSON(http.StatusInternalServerError, errBody("failed to list AI genre suggestions"))
	}
	items := make([]suggestionResponse, 0, len(views))
	for _, v := range views {
		items = append(items, toSuggestion(v))
	}
	return c.JSON(http.StatusOK, listResponse{Items: items, Total: total})
}

func parsePositive(raw string) (int, error) {
	n := 0
	if raw == "" {
		return 0, errors.New("empty")
	}
	for _, r := range raw {
		if r < '0' || r > '9' || n > 1_000_000 {
			return 0, errors.New("not a number")
		}
		n = n*10 + int(r-'0')
	}
	if n < 1 {
		return 0, errors.New("not positive")
	}
	return n, nil
}

// SuggestionsStatus godoc
// @Summary Статус ИИ-анализа жанров (админ)
// @Description Требуется роль admin. enabled — включён ли анализ (флаг AI_GENRES_ENABLED и ключ API заданы), model — модель, prompt_version — версия промпта, open_count — число открытых предложений (устаревшие включены), last_run — последний запуск анализа или null. Работает и при выключенной функции.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} statusResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres/ai-suggestions/status [get]
func (h *Handler) SuggestionsStatus(c echo.Context) error {
	st, err := h.service.Status(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errBody("failed to load AI genre status"))
	}
	return c.JSON(http.StatusOK, statusResponse{
		Enabled: st.Enabled, Model: st.Model, PromptVersion: st.PromptVersion,
		OpenCount: st.OpenCount, LastRun: toRun(st.LastRun),
	})
}

// Analyze godoc
// @Summary Запустить ИИ-анализ жанров (админ)
// @Description Требуется роль admin и Bearer-токен. Тело пустое или {}. Ставит в очередь один ручной запуск анализа всех жанров на модерации и опубликованных (не слитых), независимо от того, менялись ли они. Предложения составляет воркер; жанры не меняются. 202 — run_id запуска; 409 — анализ уже в очереди или идёт; 502 — не удалось поставить задачу в очередь (запуск помечается failed); 503 — анализ выключен.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 202 {object} analyzeResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/genres/ai-suggestions/analyze [post]
func (h *Handler) Analyze(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return unauthorized(c)
	}
	if !h.service.Active() {
		return c.JSON(http.StatusServiceUnavailable, errBody(aigenresservice.ErrDisabled.Error()))
	}
	ctx := c.Request().Context()
	run, err := h.service.StartAnalysis(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, aigenresservice.ErrDisabled):
			return c.JSON(http.StatusServiceUnavailable, errBody(err.Error()))
		case errors.Is(err, aigenresservice.ErrAlreadyRunning):
			return c.JSON(http.StatusConflict, errBody(err.Error()))
		default:
			return c.JSON(http.StatusInternalServerError, errBody("failed to start AI genre analysis"))
		}
	}
	task, err := aigenresservice.NewAnalyzeTask(run.ID)
	if err == nil {
		_, err = h.enqueuer.Enqueue(task, aigenresservice.AnalyzeOptions()...)
	}
	if err != nil {
		if mErr := h.service.MarkRunEnqueueFailed(ctx, run.ID); mErr != nil {
			c.Logger().Errorf("ai genres: mark enqueue_failed for %s: %v", run.ID, mErr)
		}
		if errors.Is(err, asynq.ErrDuplicateTask) {
			return c.JSON(http.StatusConflict, errBody(aigenresservice.ErrAlreadyRunning.Error()))
		}
		return c.JSON(http.StatusBadGateway, errBody("failed to queue AI genre analysis"))
	}
	return c.JSON(http.StatusAccepted, analyzeResponse{RunID: run.ID.String()})
}

// parseOptionalTarget reads the optional {"target_id"} body. ok is false after
// the 400 response has been written.
func parseOptionalTarget(c echo.Context) (*uuid.UUID, bool) {
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<16))
	if err != nil {
		_ = c.JSON(http.StatusBadRequest, errBody("invalid request"))
		return nil, false
	}
	if len(trimSpace(body)) == 0 {
		return nil, true
	}
	var req applyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		_ = c.JSON(http.StatusBadRequest, errBody("invalid request"))
		return nil, false
	}
	if req.TargetID == nil {
		return nil, true
	}
	id, err := uuid.Parse(*req.TargetID)
	if err != nil {
		_ = c.JSON(http.StatusBadRequest, errBody("invalid target_id"))
		return nil, false
	}
	return &id, true
}

func trimSpace(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && (b[start] == ' ' || b[start] == '\n' || b[start] == '\r' || b[start] == '\t') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\n' || b[end-1] == '\r' || b[end-1] == '\t') {
		end--
	}
	return b[start:end]
}

func toApplyResponse(res *aigenresservice.ApplyResult) applyResponse {
	out := applyResponse{Suggestion: toSuggestion(*res.View)}
	if m := res.Merge; m != nil {
		mr := &mergeResponse{
			SourcesMerged: m.SourcesMerged, BooksReassigned: m.BooksReassigned, CollectionsSwitched: m.CollectionsSwitched,
		}
		if m.Genre != nil {
			mr.Genre = mergeGenreResponse{
				ID: m.Genre.ID.String(), Name: m.Genre.Name, Slug: m.Genre.Slug, Status: m.Genre.Status, WikidataQID: m.Genre.WikidataQID,
			}
		}
		out.Merge = mr
	}
	if r := res.StatusReport; r != nil {
		sr := &statusReportResponse{Processed: r.Processed, Updated: r.Updated, Errors: make([]statusReportRowError, 0, len(r.Errors))}
		for _, e := range r.Errors {
			sr.Errors = append(sr.Errors, statusReportRowError{GenreID: e.GenreID, Name: e.Name, Message: e.Message})
		}
		out.StatusReport = sr
	}
	return out
}

// ApplySuggestion godoc
// @Summary Применить предложение ИИ по жанрам (админ)
// @Description Требуется роль admin и Bearer-токен. Необязательное тело {"target_id"}. Для duplicate/narrow жанры сливаются через пакетное слияние в одной транзакции: целевой жанр — target_id или предложенный; если он среди жанров предложения, источники — остальные жанры, иначе источниками становятся все жанры предложения. Для junk жанр получает статус rejected через массовую смену статуса, target_id для junk недопустим (400). Устаревшее предложение (жанр удалён, слит или отклонён) не применяется: оно помечается stale, жанры не меняются, 409. 404 — предложение не найдено, либо выбранный target_id не найден или уже слит (ничего не меняется); 409 — предложение не открыто или устарело. Слияние необратимо. Ответ: suggestion, merge (результат слияния или null), status_report (отчёт о смене статуса для junk или null).
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID предложения (UUID)"
// @Param body body applyRequest false "target_id — другой целевой жанр"
// @Success 200 {object} applyResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres/ai-suggestions/{id}/apply [post]
func (h *Handler) ApplySuggestion(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return unauthorized(c)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errBody("invalid suggestion id"))
	}
	target, ok := parseOptionalTarget(c)
	if !ok {
		return nil
	}
	res, err := h.service.Apply(c.Request().Context(), userID, id, target)
	if err != nil {
		return h.writeApplyError(c, err)
	}
	return c.JSON(http.StatusOK, toApplyResponse(res))
}

func (h *Handler) writeApplyError(c echo.Context, err error) error {
	var input *aigenresservice.InputError
	switch {
	case errors.Is(err, aigenresservice.ErrNotFound):
		return c.JSON(http.StatusNotFound, errBody(err.Error()))
	case errors.Is(err, aigenresservice.ErrGenreNotFound):
		return c.JSON(http.StatusNotFound, errBody("genre not found"))
	case errors.Is(err, aigenresservice.ErrNotOpen), errors.Is(err, aigenresservice.ErrStale):
		return c.JSON(http.StatusConflict, errBody(err.Error()))
	case errors.As(err, &input):
		return c.JSON(http.StatusBadRequest, errBody(input.Error()))
	default:
		c.Logger().Errorf("ai genres: apply failed: %v", err)
		return c.JSON(http.StatusInternalServerError, errBody("failed to apply suggestion"))
	}
}

// ApplySuggestions godoc
// @Summary Применить несколько предложений ИИ по жанрам (админ)
// @Description Требуется роль admin и Bearer-токен. items — от 1 до 50 элементов {id, target_id?} без повторов id; иначе 400 и ничего не меняется. Каждый элемент применяется так же, как одиночным запросом, по очереди в порядке запроса; каждое слияние — своя транзакция, сбой элемента не отменяет предыдущие. Предложение, ставшее устаревшим из-за предыдущего элемента, помечается stale и считается в stale. errors — элементы, которые не найдены, не открыты или не удались. Слияние необратимо.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body bulkApplyRequest true "Предложения для применения"
// @Success 200 {object} bulkApplyResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /admin/genres/ai-suggestions/apply [post]
func (h *Handler) ApplySuggestions(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return unauthorized(c)
	}
	var req bulkApplyRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errBody("invalid request"))
	}
	if len(req.Items) == 0 {
		return c.JSON(http.StatusBadRequest, errBody("items must not be empty"))
	}
	if len(req.Items) > domain.BulkMax {
		return c.JSON(http.StatusBadRequest, errBody("at most 50 items are allowed"))
	}
	items := make([]aigenresservice.ApplyItem, 0, len(req.Items))
	seen := make(map[uuid.UUID]struct{}, len(req.Items))
	for _, raw := range req.Items {
		id, err := uuid.Parse(raw.ID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, errBody("items must contain valid suggestion ids"))
		}
		if _, dup := seen[id]; dup {
			return c.JSON(http.StatusBadRequest, errBody("duplicate suggestion id"))
		}
		seen[id] = struct{}{}
		item := aigenresservice.ApplyItem{ID: id}
		if raw.TargetID != nil {
			t, err := uuid.Parse(*raw.TargetID)
			if err != nil {
				return c.JSON(http.StatusBadRequest, errBody("invalid target_id"))
			}
			item.TargetID = &t
		}
		items = append(items, item)
	}
	rep := h.service.ApplyBulk(c.Request().Context(), userID, items)
	return c.JSON(http.StatusOK, bulkApplyResponse{
		Processed: rep.Processed, Applied: rep.Applied, Stale: rep.Stale, Errors: toItemErrors(rep.Errors),
	})
}

// DismissSuggestion godoc
// @Summary Отклонить предложение ИИ по жанрам (админ)
// @Description Требуется роль admin и Bearer-токен. Жанры не меняются. Открытое предложение (в том числе устаревшее при чтении) становится dismissed с администратором и временем решения; ИИ больше не предложит тот же набор жанров. 404 — предложение не найдено; 409 — предложение не открыто.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID предложения (UUID)"
// @Success 200 {object} suggestionResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres/ai-suggestions/{id}/dismiss [post]
func (h *Handler) DismissSuggestion(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return unauthorized(c)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errBody("invalid suggestion id"))
	}
	view, err := h.service.Dismiss(c.Request().Context(), userID, id)
	if err != nil {
		switch {
		case errors.Is(err, aigenresservice.ErrNotFound):
			return c.JSON(http.StatusNotFound, errBody(err.Error()))
		case errors.Is(err, aigenresservice.ErrNotOpen):
			return c.JSON(http.StatusConflict, errBody(err.Error()))
		default:
			return c.JSON(http.StatusInternalServerError, errBody("failed to dismiss suggestion"))
		}
	}
	return c.JSON(http.StatusOK, toSuggestion(*view))
}

// DismissSuggestions godoc
// @Summary Отклонить несколько предложений ИИ по жанрам (админ)
// @Description Требуется роль admin и Bearer-токен. ids — от 1 до 50 UUID (повторы убираются), иначе 400. Жанры не меняются. errors — предложения, которые не найдены или не открыты.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body bulkDismissRequest true "ID предложений"
// @Success 200 {object} bulkDismissResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /admin/genres/ai-suggestions/dismiss [post]
func (h *Handler) DismissSuggestions(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return unauthorized(c)
	}
	var req bulkDismissRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errBody("invalid request"))
	}
	ids := make([]uuid.UUID, 0, len(req.IDs))
	seen := make(map[uuid.UUID]struct{}, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, errBody("ids must be UUIDs"))
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return c.JSON(http.StatusBadRequest, errBody("ids must not be empty"))
	}
	if len(ids) > domain.BulkMax {
		return c.JSON(http.StatusBadRequest, errBody("at most 50 ids are allowed"))
	}
	rep := h.service.DismissBulk(c.Request().Context(), userID, ids)
	return c.JSON(http.StatusOK, bulkDismissResponse{
		Processed: rep.Processed, Dismissed: rep.Dismissed, Errors: toItemErrors(rep.Errors),
	})
}
