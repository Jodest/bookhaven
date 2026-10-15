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

	"github.com/bookhaven/backend/internal/modules/aicollections/domain"
	draftservice "github.com/bookhaven/backend/internal/modules/aicollections/service"
	curateddomain "github.com/bookhaven/backend/internal/modules/curated/domain"
	curatedservice "github.com/bookhaven/backend/internal/modules/curated/service"
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
	return &asynq.TaskInfo{ID: "t1", Queue: draftservice.QueueName}, nil
}

type fakeService struct {
	active      bool
	status      *draftservice.Status
	createErr   error
	created     *domain.Draft
	createUser  uuid.UUID
	createArgs  []any
	enqueueFail []uuid.UUID
	list        []domain.Draft
	listCol     uuid.UUID
	detail      *domain.Draft
	report      *draftservice.Report
	getErr      error
	statusUser  uuid.UUID
}

func (f *fakeService) Active() bool { return f.active }

func (f *fakeService) Status(ctx context.Context, userID uuid.UUID) (*draftservice.Status, error) {
	f.statusUser = userID
	return f.status, nil
}

func (f *fakeService) CreateDraft(ctx context.Context, userID uuid.UUID, collectionID, brief string, count *int) (*domain.Draft, error) {
	f.createUser = userID
	f.createArgs = []any{collectionID, brief, count}
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.created, nil
}

func (f *fakeService) MarkEnqueueFailed(ctx context.Context, id uuid.UUID) error {
	f.enqueueFail = append(f.enqueueFail, id)
	return nil
}

func (f *fakeService) ListDrafts(ctx context.Context, collectionID uuid.UUID) ([]domain.Draft, error) {
	f.listCol = collectionID
	return f.list, nil
}

func (f *fakeService) GetDraft(ctx context.Context, id uuid.UUID) (*domain.Draft, *draftservice.Report, error) {
	if f.getErr != nil {
		return nil, nil, f.getErr
	}
	return f.detail, f.report, nil
}

var created = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

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

func queuedDraft(user uuid.UUID) *domain.Draft {
	return &domain.Draft{
		ID: uuid.New(), CollectionID: uuid.New(), Brief: "научная фантастика", Count: 120, Status: domain.StatusQueued,
		PromptVersion: 1, CreatedBy: &user, Creator: &domain.Creator{ID: user, Username: "admin"}, CreatedAt: created, UpdatedAt: created,
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
		{"disabled", &fakeService{active: false}, `{"brief":"x"}`, http.StatusServiceUnavailable, "AI collection drafts are disabled"},
		{"not json", &fakeService{active: true}, `nope`, http.StatusBadRequest, "invalid request"},
		{"bad collection id", &fakeService{active: true, createErr: draftservice.ErrInvalidCollectionID}, `{}`, http.StatusBadRequest, "collection_id"},
		{"bad brief", &fakeService{active: true, createErr: draftservice.ErrInvalidBrief}, `{"brief":""}`, http.StatusBadRequest, "brief: must be 1-500 characters"},
		{"bad count", &fakeService{active: true, createErr: draftservice.ErrInvalidCount}, `{"count":0}`, http.StatusBadRequest, "count: must be between 1 and 1000"},
		{"unknown collection", &fakeService{active: true, createErr: draftservice.ErrCollectionNotFound}, `{}`, http.StatusNotFound, "collection not found"},
		{"automatic", &fakeService{active: true, createErr: draftservice.ErrCollectionAutomatic}, `{}`, http.StatusUnprocessableEntity, "collection is automatic"},
		{"active", &fakeService{active: true, createErr: &draftservice.ActiveDraftError{DraftID: uuid.MustParse("11111111-1111-1111-1111-111111111111")}}, `{}`, http.StatusConflict, `"draft_id":"11111111-1111-1111-1111-111111111111"`},
		{"daily limit", &fakeService{active: true, createErr: draftservice.ErrDailyLimit}, `{}`, http.StatusTooManyRequests, "daily AI draft limit reached"},
		{"service disabled", &fakeService{active: true, createErr: draftservice.ErrDisabled}, `{}`, http.StatusServiceUnavailable, "disabled"},
		{"db error", &fakeService{active: true, createErr: errors.New("db")}, `{}`, http.StatusInternalServerError, "failed"},
	}
	for _, tt := range tests {
		enq := &fakeEnqueuer{}
		c, rec := newCtx(http.MethodPost, "/api/v1/admin/ai-collections/drafts", tt.body, user.String())
		if err := NewHandler(enq, tt.svc).CreateDraft(c); err != nil {
			t.Fatal(err)
		}
		if rec.Code != tt.want || !strings.Contains(rec.Body.String(), tt.contains) || len(enq.tasks) != 0 {
			t.Errorf("%s: got %d %s (tasks %d)", tt.name, rec.Code, rec.Body.String(), len(enq.tasks))
		}
	}
}

func TestCreate_Accepted(t *testing.T) {
	user := uuid.New()
	d := queuedDraft(user)
	svc := &fakeService{active: true, created: d}
	enq := &fakeEnqueuer{}
	body := `{"collection_id":"` + d.CollectionID.String() + `","brief":"научная фантастика","count":120}`
	c, rec := newCtx(http.MethodPost, "/x", body, user.String())
	if err := NewHandler(enq, svc).CreateDraft(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if svc.createUser != user || svc.createArgs[0] != d.CollectionID.String() || svc.createArgs[1] != "научная фантастика" || *(svc.createArgs[2].(*int)) != 120 {
		t.Errorf("create args = %v user=%s", svc.createArgs, svc.createUser)
	}
	if len(enq.tasks) != 1 || enq.tasks[0].Type() != draftservice.TaskTypeDraft || !strings.Contains(string(enq.tasks[0].Payload()), d.ID.String()) {
		t.Errorf("tasks = %+v", enq.tasks)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["id"] != d.ID.String() || out["collection_id"] != d.CollectionID.String() || out["status"] != "queued" || out["count"] != float64(120) ||
		out["error_code"] != nil || out["finished_at"] != nil || out["proposals_count"] != float64(0) || out["input_tokens"] != float64(0) {
		t.Errorf("body = %v", out)
	}
	if cb, _ := out["created_by"].(map[string]any); cb["username"] != "admin" || cb["id"] != user.String() {
		t.Errorf("created_by = %v", out["created_by"])
	}
}

func TestCreate_EnqueueFailure(t *testing.T) {
	user := uuid.New()
	d := queuedDraft(user)
	svc := &fakeService{active: true, created: d}
	c, rec := newCtx(http.MethodPost, "/x", `{"brief":"x"}`, user.String())
	if err := NewHandler(&fakeEnqueuer{err: errors.New("redis down")}, svc).CreateDraft(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "failed to queue AI draft") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if len(svc.enqueueFail) != 1 || svc.enqueueFail[0] != d.ID {
		t.Errorf("enqueue_failed marks = %v", svc.enqueueFail)
	}
}

func TestUserIDRequired(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeService{active: true})
	for name, fn := range map[string]func(echo.Context) error{"status": h.DraftsStatus, "create": h.CreateDraft} {
		c, rec := newCtx(http.MethodPost, "/x", `{}`, "")
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
	svc := &fakeService{status: &draftservice.Status{Enabled: true, Model: "claude-sonnet-5-5", UsedToday: 2}}
	c, rec := newCtx(http.MethodGet, "/x", "", user.String())
	if err := NewHandler(&fakeEnqueuer{}, svc).DraftsStatus(c); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusOK || body["enabled"] != true || body["model"] != "claude-sonnet-5-5" || body["prompt_version"] != float64(2) ||
		body["max_brief_length"] != float64(500) || body["max_count"] != float64(1000) ||
		body["daily_limit"] != float64(20) || body["used_today"] != float64(2) {
		t.Errorf("got %d %v", rec.Code, body)
	}
	if svc.statusUser != user {
		t.Errorf("status user = %s", svc.statusUser)
	}

	svc.status = &draftservice.Status{}
	c, rec = newCtx(http.MethodGet, "/x", "", user.String())
	_ = NewHandler(&fakeEnqueuer{}, svc).DraftsStatus(c)
	if !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Errorf("off: %s", rec.Body.String())
	}
}

func TestList(t *testing.T) {
	user := uuid.New()
	col := uuid.New()
	orphan := domain.Draft{ID: uuid.New(), CollectionID: col, Status: domain.StatusDone, CreatedAt: created}
	svc := &fakeService{list: []domain.Draft{*queuedDraft(user), orphan}}
	c, rec := newCtx(http.MethodGet, "/x?collection_id="+col.String(), "", user.String())
	if err := NewHandler(&fakeEnqueuer{}, svc).ListDrafts(c); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusOK || len(body.Items) != 2 || body.Items[1]["created_by"] != nil || svc.listCol != col {
		t.Errorf("got %d %s", rec.Code, rec.Body.String())
	}

	svc.list = nil
	c, rec = newCtx(http.MethodGet, "/x?collection_id="+col.String(), "", user.String())
	_ = NewHandler(&fakeEnqueuer{}, svc).ListDrafts(c)
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("empty: %s", rec.Body.String())
	}

	for _, target := range []string{"/x", "/x?collection_id=nope"} {
		c, rec = newCtx(http.MethodGet, target, "", user.String())
		_ = NewHandler(&fakeEnqueuer{}, svc).ListDrafts(c)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d", target, rec.Code)
		}
	}
}

func TestGet_ReportAndErrors(t *testing.T) {
	user := uuid.New()
	d := queuedDraft(user)
	d.Status = domain.StatusDone
	h := func(svc *fakeService) *Handler { return NewHandler(&fakeEnqueuer{}, svc) }
	get := func(h *Handler, id string) *httptest.ResponseRecorder {
		c, rec := newCtx(http.MethodGet, "/x", "", user.String())
		c.SetParamNames("id")
		c.SetParamValues(id)
		_ = h.GetDraft(c)
		return rec
	}

	// A running draft has a null report.
	running := queuedDraft(user)
	running.Status = domain.StatusRunning
	running.ProposalsCount = 40
	rec := get(h(&fakeService{detail: running}), running.ID.String())
	if !strings.Contains(rec.Body.String(), `"report":null`) || !strings.Contains(rec.Body.String(), `"proposals_count":40`) {
		t.Errorf("running: %s", rec.Body.String())
	}

	author := "Дэн Симмонс"
	report := &draftservice.Report{
		Preview: &curatedservice.PastePreview{
			BooksCount: 3, FreeSlots: 997,
			Counts: map[curateddomain.PasteStatus]int{curateddomain.PasteFound: 1, curateddomain.PasteNotFound: 1},
			Lines: []curatedservice.PasteLine{
				{Line: 1, Text: "Гиперион — Дэн Симмонс", Title: "Гиперион", Author: &author, Status: curateddomain.PasteNotFound, Candidates: []curatedservice.PasteBook{}},
				{Line: 2, Text: "Дюна", Title: "Дюна", Status: curateddomain.PasteNotFound, Candidates: []curatedservice.PasteBook{}},
			},
		},
		Proposals: map[int]domain.Proposal{
			1: {Rank: 1, Title: "Гиперион", Author: "Дэн Симмонс", OriginalTitle: "Hyperion", Note: "<b>эпос</b>"},
			2: {Rank: 2, Title: "Дюна"},
		},
	}
	rec = get(h(&fakeService{detail: d, report: report}), d.ID.String())
	var out struct {
		Report struct {
			BooksCount int            `json:"books_count"`
			FreeSlots  int            `json:"free_slots"`
			Counts     map[string]int `json:"counts"`
			Lines      []map[string]any
		} `json:"report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	r := out.Report
	if r.BooksCount != 3 || r.FreeSlots != 997 || r.Counts["found"] != 1 || r.Counts["not_found"] != 1 || r.Counts["invalid"] != 0 || len(r.Lines) != 2 {
		t.Fatalf("report = %+v", r)
	}
	l0, l1 := r.Lines[0], r.Lines[1]
	if l0["line"] != float64(1) || l0["status"] != "not_found" || l0["original_title"] != "Hyperion" || l0["note"] != "<b>эпос</b>" ||
		l0["text"] != "Гиперион — Дэн Симмонс" || l0["author"] != "Дэн Симмонс" || l0["book"] != nil {
		t.Errorf("line 1 = %v", l0)
	}
	if l1["original_title"] != nil || l1["note"] != "" || l1["author"] != nil {
		t.Errorf("line 2 = %v", l1)
	}

	if rec := get(h(&fakeService{}), "not-a-uuid"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad uuid: %d", rec.Code)
	}
	if rec := get(h(&fakeService{getErr: draftservice.ErrNotFound}), uuid.NewString()); rec.Code != http.StatusNotFound {
		t.Errorf("unknown: %d", rec.Code)
	}
	if rec := get(h(&fakeService{getErr: errors.New("db")}), uuid.NewString()); rec.Code != http.StatusInternalServerError {
		t.Errorf("db error: %d", rec.Code)
	}
}
