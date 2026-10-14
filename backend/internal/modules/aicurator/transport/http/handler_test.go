package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	curatorservice "github.com/bookhaven/backend/internal/modules/aicurator/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

type fakeEnqueuer struct {
	tasks []*asynq.Task
	err   error
}

func (f *fakeEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.tasks = append(f.tasks, task)
	return &asynq.TaskInfo{ID: "t1", Queue: curatorservice.QueueName}, nil
}

type fakeService struct {
	active      bool
	status      *curatorservice.Status
	createErr   error
	created     *domain.Request
	createUser  uuid.UUID
	createArgs  []any
	enqueueFail []uuid.UUID
	list        []domain.Request
	detail      *domain.Request
	existing    map[string]catalogDomain.Author
	getErr      error
	statusUser  uuid.UUID
}

func (f *fakeService) Active() bool { return f.active }

func (f *fakeService) Status(ctx context.Context, userID uuid.UUID) (*curatorservice.Status, error) {
	f.statusUser = userID
	return f.status, nil
}

func (f *fakeService) CreateRequest(ctx context.Context, userID uuid.UUID, prompt string, maxNames *int) (*domain.Request, error) {
	f.createUser = userID
	f.createArgs = []any{prompt, maxNames}
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.created, nil
}

func (f *fakeService) MarkEnqueueFailed(ctx context.Context, id uuid.UUID) error {
	f.enqueueFail = append(f.enqueueFail, id)
	return nil
}

func (f *fakeService) ListRecent(ctx context.Context) ([]domain.Request, error) { return f.list, nil }

func (f *fakeService) GetRequest(ctx context.Context, id uuid.UUID) (*domain.Request, map[string]catalogDomain.Author, error) {
	if f.getErr != nil {
		return nil, nil, f.getErr
	}
	return f.detail, f.existing, nil
}

var created = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func newCtx(method, target, body string, userID string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	if userID != "" {
		c.Set("user_id", userID)
	}
	return c, rec
}

func queuedRequest(user uuid.UUID) *domain.Request {
	return &domain.Request{
		ID: uuid.New(), Prompt: "русская классика", MaxNames: 20, Status: domain.StatusQueued, PromptVersion: 1,
		CreatedBy: &user, Creator: &domain.Creator{ID: user, Username: "admin"}, CreatedAt: created, UpdatedAt: created,
	}
}

func TestCreate_StatusCodes(t *testing.T) {
	user := uuid.New()
	tests := []struct {
		name     string
		svc      *fakeService
		body     string
		want     int
		contains string
	}{
		{"disabled", &fakeService{active: false}, `{"prompt":"x"}`, http.StatusServiceUnavailable, "AI curation is disabled"},
		{"not json", &fakeService{active: true}, `nope`, http.StatusBadRequest, "invalid request"},
		{"bad prompt", &fakeService{active: true, createErr: curatorservice.ErrInvalidPrompt}, `{"prompt":""}`, http.StatusBadRequest, "prompt"},
		{"bad max_names", &fakeService{active: true, createErr: curatorservice.ErrInvalidMaxNames}, `{"prompt":"x","max_names":31}`, http.StatusBadRequest, "max_names"},
		{"active", &fakeService{active: true, createErr: &curatorservice.ActiveRequestError{RequestID: uuid.MustParse("11111111-1111-1111-1111-111111111111")}}, `{"prompt":"x"}`, http.StatusConflict, `"request_id":"11111111-1111-1111-1111-111111111111"`},
		{"daily limit", &fakeService{active: true, createErr: curatorservice.ErrDailyLimit}, `{"prompt":"x"}`, http.StatusTooManyRequests, "daily curation limit reached"},
		{"service disabled", &fakeService{active: true, createErr: curatorservice.ErrDisabled}, `{"prompt":"x"}`, http.StatusServiceUnavailable, "disabled"},
		{"db error", &fakeService{active: true, createErr: errors.New("db")}, `{"prompt":"x"}`, http.StatusInternalServerError, "failed"},
	}
	for _, tt := range tests {
		enq := &fakeEnqueuer{}
		c, rec := newCtx(http.MethodPost, "/api/v1/admin/ai-curator/requests", tt.body, user.String())
		if err := NewHandler(enq, tt.svc).CreateCurationRequest(c); err != nil {
			t.Fatal(err)
		}
		if rec.Code != tt.want || !strings.Contains(rec.Body.String(), tt.contains) || len(enq.tasks) != 0 {
			t.Errorf("%s: got %d %s (tasks %d)", tt.name, rec.Code, rec.Body.String(), len(enq.tasks))
		}
	}
}

func TestCreate_Accepted(t *testing.T) {
	user := uuid.New()
	req := queuedRequest(user)
	svc := &fakeService{active: true, created: req}
	enq := &fakeEnqueuer{}
	c, rec := newCtx(http.MethodPost, "/api/v1/admin/ai-curator/requests", `{"prompt":"русская классика","max_names":12}`, user.String())
	if err := NewHandler(enq, svc).CreateCurationRequest(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if svc.createUser != user || svc.createArgs[0] != "русская классика" || *(svc.createArgs[1].(*int)) != 12 {
		t.Errorf("create args = %v user=%s", svc.createArgs, svc.createUser)
	}
	if len(enq.tasks) != 1 || enq.tasks[0].Type() != curatorservice.TaskTypeCurate || !strings.Contains(string(enq.tasks[0].Payload()), req.ID.String()) {
		t.Errorf("tasks = %+v", enq.tasks)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["id"] != req.ID.String() || body["status"] != "queued" || body["error_code"] != nil || body["finished_at"] != nil {
		t.Errorf("body = %v", body)
	}
	if cb, _ := body["created_by"].(map[string]any); cb["username"] != "admin" || cb["id"] != user.String() {
		t.Errorf("created_by = %v", body["created_by"])
	}
}

func TestCreate_EnqueueFailure(t *testing.T) {
	user := uuid.New()
	req := queuedRequest(user)
	svc := &fakeService{active: true, created: req}
	c, rec := newCtx(http.MethodPost, "/x", `{"prompt":"x"}`, user.String())
	if err := NewHandler(&fakeEnqueuer{err: errors.New("redis down")}, svc).CreateCurationRequest(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "failed to queue curation request") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if len(svc.enqueueFail) != 1 || svc.enqueueFail[0] != req.ID {
		t.Errorf("enqueue_failed marks = %v", svc.enqueueFail)
	}
}

func TestUserIDRequired(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeService{active: true})
	for name, fn := range map[string]func(echo.Context) error{"status": h.CurationStatus, "create": h.CreateCurationRequest} {
		c, rec := newCtx(http.MethodPost, "/x", `{"prompt":"x"}`, "")
		if err := fn(c); err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d", name, rec.Code)
		}
	}
}

func TestStatus(t *testing.T) {
	user := uuid.New()
	active := uuid.New()
	svc := &fakeService{status: &curatorservice.Status{Enabled: true, Model: "claude-haiku-5-5", UsedToday: 3, ActiveRequestID: &active}}
	c, rec := newCtx(http.MethodGet, "/x", "", user.String())
	if err := NewHandler(&fakeEnqueuer{}, svc).CurationStatus(c); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusOK || body["enabled"] != true || body["model"] != "claude-haiku-5-5" || body["prompt_version"] != float64(1) ||
		body["max_prompt_length"] != float64(500) || body["max_names"] != float64(30) || body["default_names"] != float64(20) ||
		body["daily_limit"] != float64(20) || body["used_today"] != float64(3) || body["active_request_id"] != active.String() {
		t.Errorf("got %d %v", rec.Code, body)
	}
	if svc.statusUser != user {
		t.Errorf("status user = %s", svc.statusUser)
	}

	svc.status = &curatorservice.Status{}
	c, rec = newCtx(http.MethodGet, "/x", "", user.String())
	_ = NewHandler(&fakeEnqueuer{}, svc).CurationStatus(c)
	if !strings.Contains(rec.Body.String(), `"active_request_id":null`) || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Errorf("off: %s", rec.Body.String())
	}
}

func TestList(t *testing.T) {
	user := uuid.New()
	orphan := domain.Request{ID: uuid.New(), Status: domain.StatusDone, CreatedAt: created}
	svc := &fakeService{list: []domain.Request{*queuedRequest(user), orphan}}
	c, rec := newCtx(http.MethodGet, "/x", "", user.String())
	if err := NewHandler(&fakeEnqueuer{}, svc).ListCurationRequests(c); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusOK || len(body.Items) != 2 || body.Items[1]["created_by"] != nil {
		t.Errorf("got %d %s", rec.Code, rec.Body.String())
	}

	svc.list = nil
	c, rec = newCtx(http.MethodGet, "/x", "", user.String())
	_ = NewHandler(&fakeEnqueuer{}, svc).ListCurationRequests(c)
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("empty: %s", rec.Body.String())
	}
}

func TestGet(t *testing.T) {
	user := uuid.New()
	req := queuedRequest(user)
	req.Status = domain.StatusDone
	req.InputTokens, req.OutputTokens = 812, 1630
	authorID := uuid.New()
	req.Candidates = []domain.Candidate{
		{ID: uuid.New(), Position: 0, Name: "Иван Гончаров", Note: "<b>Обломов</b>", MatchStatus: domain.MatchMatched,
			Options: []domain.Option{{QID: "Q192279", Label: "Иван Александрович Гончаров", Description: "русский писатель"}}},
		{ID: uuid.New(), Position: 1, Name: "Никто", MatchStatus: domain.MatchNotFound, Options: []domain.Option{}},
	}
	svc := &fakeService{detail: req, existing: map[string]catalogDomain.Author{
		"Q192279": {ID: authorID, Name: "Иван Гончаров", Slug: "ivan-goncharov", Status: "pending"},
	}}
	h := NewHandler(&fakeEnqueuer{}, svc)

	c, rec := newCtx(http.MethodGet, "/x", "", user.String())
	c.SetParamNames("id")
	c.SetParamValues(req.ID.String())
	if err := h.GetCurationRequest(c); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	cands, _ := body["candidates"].([]any)
	if rec.Code != http.StatusOK || body["input_tokens"] != float64(812) || len(cands) != 2 {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	c0 := cands[0].(map[string]any)
	opt := c0["options"].([]any)[0].(map[string]any)
	ex, _ := opt["existing"].(map[string]any)
	if c0["note"] != "<b>Обломов</b>" || opt["qid"] != "Q192279" || ex["id"] != authorID.String() || ex["status"] != "pending" {
		t.Errorf("candidate 0 = %v", c0)
	}
	if opts, ok := cands[1].(map[string]any)["options"].([]any); !ok || len(opts) != 0 {
		t.Errorf("options must be [] not null: %v", cands[1])
	}

	c, rec = newCtx(http.MethodGet, "/x", "", user.String())
	c.SetParamNames("id")
	c.SetParamValues("not-a-uuid")
	_ = h.GetCurationRequest(c)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad uuid: %d", rec.Code)
	}

	svc.getErr = curatorservice.ErrNotFound
	c, rec = newCtx(http.MethodGet, "/x", "", user.String())
	c.SetParamNames("id")
	c.SetParamValues(uuid.NewString())
	_ = h.GetCurationRequest(c)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown: %d", rec.Code)
	}
}
