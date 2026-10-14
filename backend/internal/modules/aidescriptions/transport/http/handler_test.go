package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	descservice "github.com/bookhaven/backend/internal/modules/aidescriptions/service"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

var now = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

type fakeEnqueuer struct {
	tasks []*asynq.Task
	opts  [][]asynq.Option
	err   error
}

func (f *fakeEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.tasks = append(f.tasks, task)
	f.opts = append(f.opts, opts)
	return &asynq.TaskInfo{ID: "t1", Queue: descservice.QueueName}, nil
}

type fakeService struct {
	active     bool
	item       *domain.Item
	list       *descservice.ListResult
	listErr    error
	reviewErr  error
	approveArg *string
	userID     uuid.UUID
}

func (f *fakeService) Active() bool   { return f.active }
func (f *fakeService) Now() time.Time { return now }
func (f *fakeService) Status(ctx context.Context) (*descservice.Status, error) {
	return &descservice.Status{Enabled: f.active, Model: "claude-sonnet-5-5", Missing: 134, Drafts: 6}, nil
}
func (f *fakeService) List(ctx context.Context, status string, page, limit int) (*descservice.ListResult, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.list, nil
}
func (f *fakeService) Get(ctx context.Context, bookID uuid.UUID) (*domain.Item, error) {
	return f.item, nil
}
func (f *fakeService) Approve(ctx context.Context, bookID, userID uuid.UUID, text *string) (*domain.Item, error) {
	f.approveArg, f.userID = text, userID
	if f.reviewErr != nil {
		return nil, f.reviewErr
	}
	return f.item, nil
}
func (f *fakeService) Discard(ctx context.Context, bookID, userID uuid.UUID) (*domain.Item, error) {
	if f.reviewErr != nil {
		return nil, f.reviewErr
	}
	return f.item, nil
}

func newCtx(method, target, body string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("user_id", uuid.NewString())
	return c, rec
}

func sampleItem() *domain.Item {
	year := 1925
	return &domain.Item{
		Description: domain.Description{
			BookID: uuid.New(), Status: domain.StatusApproved, Text: "Повесть о враче.", Edited: true,
			Model: "claude-sonnet-5-5", PromptVersion: 1, GeneratedAt: now,
		},
		Book:  domain.BookCard{Title: "Записки юного врача", Authors: []string{"Михаил Булгаков"}, PublishedYear: &year, Status: "published"},
		Stale: true, Superseded: true,
	}
}

func TestGenerate_StatusCodes(t *testing.T) {
	ids := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = `"` + uuid.NewString() + `"`
		}
		return `{"book_ids":[` + strings.Join(parts, ",") + `]}`
	}
	dupID := uuid.NewString()
	tests := []struct {
		name     string
		active   bool
		enqErr   error
		body     string
		want     int
		contains string
	}{
		{"disabled first", false, nil, `nope`, http.StatusServiceUnavailable, "AI descriptions are disabled"},
		{"not json", true, nil, `nope`, http.StatusBadRequest, "invalid request"},
		{"neither", true, nil, `{}`, http.StatusBadRequest, "exactly one"},
		{"all_missing false", true, nil, `{"all_missing":false}`, http.StatusBadRequest, "exactly one"},
		{"both", true, nil, `{"book_ids":["` + dupID + `"],"all_missing":true}`, http.StatusBadRequest, "exactly one"},
		{"empty list", true, nil, `{"book_ids":[]}`, http.StatusBadRequest, "must not be empty"},
		{"bad uuid", true, nil, `{"book_ids":["nope"]}`, http.StatusBadRequest, "invalid book id"},
		{"too many", true, nil, ids(101), http.StatusBadRequest, "at most 100"},
		{"100 distinct + duplicates ok", true, nil, strings.Replace(ids(99), "]}", `,"`+dupID+`","`+dupID+`"]}`, 1), http.StatusAccepted, "t1"},
		{"duplicate sweep", true, fmt.Errorf("wrap: %w", asynq.ErrDuplicateTask), `{"all_missing":true}`, http.StatusConflict, "AI description generation already queued"},
		{"enqueue failure", true, errors.New("redis down"), `{"all_missing":true}`, http.StatusBadGateway, "failed to queue generation"},
		{"sweep ok", true, nil, `{"all_missing":true}`, http.StatusAccepted, `"task_id":"t1"`},
	}
	for _, tt := range tests {
		enq := &fakeEnqueuer{err: tt.enqErr}
		c, rec := newCtx(http.MethodPost, "/api/v1/admin/ai-descriptions/generate", tt.body)
		if err := NewHandler(enq, &fakeService{active: tt.active}).GenerateAIDescriptions(c); err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if rec.Code != tt.want || !strings.Contains(rec.Body.String(), tt.contains) {
			t.Errorf("%s: %d %s", tt.name, rec.Code, rec.Body.String())
		}
		if rec.Code != http.StatusAccepted && len(enq.tasks) != 0 {
			t.Errorf("%s: nothing must be queued", tt.name)
		}
	}
}

func TestGenerate_DeduplicatesAndStampsQueuedAt(t *testing.T) {
	id := uuid.NewString()
	enq := &fakeEnqueuer{}
	c, rec := newCtx(http.MethodPost, "/x", `{"book_ids":["`+id+`","`+id+`"]}`)
	if err := NewHandler(enq, &fakeService{active: true}).GenerateAIDescriptions(c); err != nil || rec.Code != http.StatusAccepted {
		t.Fatalf("%v %d", err, rec.Code)
	}
	var p descservice.GeneratePayload
	if err := json.Unmarshal(enq.tasks[0].Payload(), &p); err != nil {
		t.Fatal(err)
	}
	if enq.tasks[0].Type() != descservice.TaskTypeGenerate || len(p.BookIDs) != 1 || p.BookIDs[0] != id || !p.QueuedAt.Equal(now) {
		t.Errorf("payload = %+v", p)
	}
}

func TestApproveDiscard_StatusCodes(t *testing.T) {
	bookID := uuid.NewString()
	tests := []struct {
		name     string
		err      error
		body     string
		want     int
		contains string
	}{
		{"not found", descservice.ErrNotFound, ``, http.StatusNotFound, "AI description not found"},
		{"wrong status", descservice.ErrNotReviewable, ``, http.StatusConflict, "AI description is not reviewable"},
		{"own description", descservice.ErrHasOwnDescription, ``, http.StatusConflict, "book already has a description"},
		{"bad text", descservice.ErrInvalidText, `{"text":"x"}`, http.StatusUnprocessableEntity, "text must be"},
		{"ok edited", nil, `{"text":"Повесть о враче."}`, http.StatusOK, `"edited":true`},
	}
	for _, tt := range tests {
		svc := &fakeService{item: sampleItem(), reviewErr: tt.err}
		c, rec := newCtx(http.MethodPost, "/x", tt.body)
		c.SetParamNames("book_id")
		c.SetParamValues(bookID)
		if err := NewHandler(&fakeEnqueuer{}, svc).ApproveAIDescription(c); err != nil {
			t.Fatal(err)
		}
		if rec.Code != tt.want || !strings.Contains(rec.Body.String(), tt.contains) {
			t.Errorf("approve %s: %d %s", tt.name, rec.Code, rec.Body.String())
		}
	}

	svc := &fakeService{item: sampleItem()}
	c, rec := newCtx(http.MethodPost, "/x", ``)
	c.SetParamNames("book_id")
	c.SetParamValues(bookID)
	_ = NewHandler(&fakeEnqueuer{}, svc).ApproveAIDescription(c)
	if rec.Code != http.StatusOK || svc.approveArg != nil {
		t.Errorf("approve without body: %d text=%v", rec.Code, svc.approveArg)
	}

	c, rec = newCtx(http.MethodPost, "/x", ``)
	c.SetParamNames("book_id")
	c.SetParamValues("nope")
	_ = NewHandler(&fakeEnqueuer{}, svc).ApproveAIDescription(c)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("approve bad id: %d", rec.Code)
	}

	for _, tt := range []struct {
		err  error
		want int
	}{{descservice.ErrNotFound, 404}, {descservice.ErrNotReviewable, 409}, {nil, 200}} {
		c, rec := newCtx(http.MethodPost, "/x", ``)
		c.SetParamNames("book_id")
		c.SetParamValues(bookID)
		_ = NewHandler(&fakeEnqueuer{}, &fakeService{item: sampleItem(), reviewErr: tt.err}).DiscardAIDescription(c)
		if rec.Code != tt.want {
			t.Errorf("discard %v: %d", tt.err, rec.Code)
		}
	}
}

func TestList_FlagsAndBadStatus(t *testing.T) {
	svc := &fakeService{list: &descservice.ListResult{Items: []domain.Item{*sampleItem()}, Total: 1, Page: 1, Limit: 20}}
	c, rec := newCtx(http.MethodGet, "/x?status=approved", ``)
	if err := NewHandler(&fakeEnqueuer{}, svc).ListAIDescriptions(c); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || out.Total != 1 || out.Items[0]["stale"] != true || out.Items[0]["superseded"] != true {
		t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
	}
	book := out.Items[0]["book"].(map[string]any)
	if book["title"] != "Записки юного врача" || out.Items[0]["error_code"] != nil {
		t.Errorf("item = %v", out.Items[0])
	}

	svc.listErr = descservice.ErrInvalidStatus
	c, rec = newCtx(http.MethodGet, "/x?status=bogus", ``)
	_ = NewHandler(&fakeEnqueuer{}, svc).ListAIDescriptions(c)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad status: %d", rec.Code)
	}
}

func TestGet_NullAndBadID(t *testing.T) {
	c, rec := newCtx(http.MethodGet, "/x", ``)
	c.SetParamNames("book_id")
	c.SetParamValues(uuid.NewString())
	_ = NewHandler(&fakeEnqueuer{}, &fakeService{}).GetAIDescription(c)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"ai_description":null}` {
		t.Errorf("get none: %d %s", rec.Code, rec.Body.String())
	}
	c, rec = newCtx(http.MethodGet, "/x", ``)
	c.SetParamNames("book_id")
	c.SetParamValues("nope")
	_ = NewHandler(&fakeEnqueuer{}, &fakeService{}).GetAIDescription(c)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("get bad id: %d", rec.Code)
	}
}

func TestStatus(t *testing.T) {
	c, rec := newCtx(http.MethodGet, "/x", ``)
	_ = NewHandler(&fakeEnqueuer{}, &fakeService{active: true}).AIDescriptionsStatus(c)
	want := `{"enabled":true,"model":"claude-sonnet-5-5","prompt_version":1,"missing":134,"drafts":6,"max_bulk":200}`
	if strings.TrimSpace(rec.Body.String()) != want {
		t.Errorf("status = %s", rec.Body.String())
	}
}
