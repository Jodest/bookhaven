package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	aimodservice "github.com/bookhaven/backend/internal/modules/aimoderation/service"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

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
	return &asynq.TaskInfo{ID: "task-1", Queue: aimodservice.QueueName}, nil
}

type fakeCounter struct {
	n          int64
	err        error
	lastPrompt int
}

func (f *fakeCounter) CountAIReviewCandidates(ctx context.Context, promptVersion int) (int64, error) {
	f.lastPrompt = promptVersion
	return f.n, f.err
}

func newCtx(method, target, body string, role usersDomain.Role) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	if role != "" {
		c.Set("user", &usersDomain.User{ID: uuid.New(), Role: role})
	}
	return c, rec
}

func post(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	c, rec := newCtx(http.MethodPost, "/api/v1/admin/books/ai-review", body, usersDomain.RoleAdmin)
	if err := h.TriggerAIReview(c); err != nil {
		t.Fatalf("TriggerAIReview: %v", err)
	}
	return rec
}

func TestTriggerAIReview_Disabled(t *testing.T) {
	enq := &fakeEnqueuer{}
	rec := post(t, NewHandler(enq, &fakeCounter{}, false, "claude-haiku-5-5"), `{"all_unreviewed":true}`)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "AI moderation is disabled") || len(enq.tasks) != 0 {
		t.Fatalf("expected 503 and nothing queued, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestTriggerAIReview_BadRequests(t *testing.T) {
	ids := make([]string, 0, 101)
	for i := 0; i < 101; i++ {
		ids = append(ids, `"`+uuid.NewString()+`"`)
	}
	for name, body := range map[string]string{
		"empty object":         `{}`,
		"both forms":           `{"book_ids":["` + uuid.NewString() + `"],"all_unreviewed":true}`,
		"empty ids":            `{"book_ids":[]}`,
		"bad uuid":             `{"book_ids":["not-a-uuid"]}`,
		"too many ids":         `{"book_ids":[` + strings.Join(ids, ",") + `]}`,
		"all_unreviewed false": `{"all_unreviewed":false}`,
		"not json":             `nope`,
	} {
		enq := &fakeEnqueuer{}
		rec := post(t, NewHandler(enq, &fakeCounter{}, true, "m"), body)
		if rec.Code != http.StatusBadRequest || len(enq.tasks) != 0 {
			t.Errorf("%s: expected 400 and nothing queued, got %d %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestTriggerAIReview_BookIDsDeduplicated(t *testing.T) {
	a, b := uuid.NewString(), uuid.NewString()
	enq := &fakeEnqueuer{}
	rec := post(t, NewHandler(enq, &fakeCounter{}, true, "m"), `{"book_ids":["`+a+`","`+b+`","`+a+`"]}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"task_id":"task-1"`) {
		t.Fatalf("expected 202, got %d %s", rec.Code, rec.Body.String())
	}
	if len(enq.tasks) != 1 || enq.tasks[0].Type() != aimodservice.TaskTypeReviewBooks {
		t.Fatalf("tasks = %+v", enq.tasks)
	}
	var p aimodservice.ReviewBooksPayload
	if err := json.Unmarshal(enq.tasks[0].Payload(), &p); err != nil || len(p.BookIDs) != 2 || p.BookIDs[0] != a || p.BookIDs[1] != b {
		t.Fatalf("payload = %s (%v)", enq.tasks[0].Payload(), err)
	}
	if len(enq.opts[0]) != len(aimodservice.ReviewBooksOptions()) {
		t.Errorf("unexpected options: %v", enq.opts[0])
	}

	// Exactly 100 distinct ids is allowed.
	hundred := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		hundred = append(hundred, `"`+uuid.NewString()+`"`)
	}
	if rec := post(t, NewHandler(&fakeEnqueuer{}, &fakeCounter{}, true, "m"), `{"book_ids":[`+strings.Join(hundred, ",")+`]}`); rec.Code != http.StatusAccepted {
		t.Fatalf("100 ids: expected 202, got %d", rec.Code)
	}
}

func TestTriggerAIReview_AllUnreviewed(t *testing.T) {
	enq := &fakeEnqueuer{}
	rec := post(t, NewHandler(enq, &fakeCounter{}, true, "m"), `{"all_unreviewed":true}`)
	if rec.Code != http.StatusAccepted || len(enq.tasks) != 1 || enq.tasks[0].Type() != aimodservice.TaskTypeSweep {
		t.Fatalf("expected a queued sweep, got %d %+v", rec.Code, enq.tasks)
	}
	var p aimodservice.SweepPayload
	if err := json.Unmarshal(enq.tasks[0].Payload(), &p); err != nil || p.Max != aimodservice.ManualSweepMax || p.Trigger != aimodservice.TriggerManual {
		t.Fatalf("payload = %s", enq.tasks[0].Payload())
	}

	dup := post(t, NewHandler(&fakeEnqueuer{err: asynq.ErrDuplicateTask}, &fakeCounter{}, true, "m"), `{"all_unreviewed":true}`)
	if dup.Code != http.StatusConflict || !strings.Contains(dup.Body.String(), "AI review already queued") {
		t.Fatalf("expected 409, got %d %s", dup.Code, dup.Body.String())
	}
	down := post(t, NewHandler(&fakeEnqueuer{err: errors.New("redis down")}, &fakeCounter{}, true, "m"), `{"all_unreviewed":true}`)
	if down.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", down.Code)
	}
}

func TestAIReviewStatus(t *testing.T) {
	counter := &fakeCounter{n: 7}
	c, rec := newCtx(http.MethodGet, "/api/v1/admin/books/ai-review/status", "", usersDomain.RoleAdmin)
	if err := NewHandler(&fakeEnqueuer{}, counter, true, "claude-haiku-5-5").AIReviewStatus(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["enabled"] != true || body["model"] != "claude-haiku-5-5" || body["prompt_version"] != float64(aimodservice.PromptVersion) || body["needs_review"] != float64(7) || len(body) != 4 {
		t.Fatalf("status = %v", body)
	}
	if counter.lastPrompt != aimodservice.PromptVersion {
		t.Errorf("counted with prompt version %d", counter.lastPrompt)
	}

	c, rec = newCtx(http.MethodGet, "/api/v1/admin/books/ai-review/status", "", usersDomain.RoleAdmin)
	if err := NewHandler(&fakeEnqueuer{}, &fakeCounter{}, false, "m").AIReviewStatus(c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("disabled status = %s", rec.Body.String())
	}

	c, rec = newCtx(http.MethodGet, "/api/v1/admin/books/ai-review/status", "", usersDomain.RoleAdmin)
	if err := NewHandler(&fakeEnqueuer{}, &fakeCounter{err: errors.New("db down")}, true, "m").AIReviewStatus(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

// The routes are registered with RequireRole(admin), and POST also with
// RequireBearerAuth (internal/transport/http/router.go).
func TestAIReviewRoutes_Auth(t *testing.T) {
	adminRole := middleware.RequireRole(usersDomain.RoleAdmin)
	for _, tc := range []struct {
		name string
		role usersDomain.Role
		want int
	}{
		{"unauthenticated", "", http.StatusUnauthorized},
		{"moderator", usersDomain.RoleModerator, http.StatusForbidden},
		{"user", usersDomain.RoleUser, http.StatusForbidden},
		{"admin", usersDomain.RoleAdmin, http.StatusAccepted},
	} {
		enq := &fakeEnqueuer{}
		h := NewHandler(enq, &fakeCounter{}, true, "m")
		c, rec := newCtx(http.MethodPost, "/api/v1/admin/books/ai-review", `{"all_unreviewed":true}`, tc.role)
		if err := adminRole(h.TriggerAIReview)(c); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if rec.Code != tc.want {
			t.Fatalf("%s: expected %d, got %d", tc.name, tc.want, rec.Code)
		}
		if tc.want != http.StatusAccepted && len(enq.tasks) != 0 {
			t.Fatalf("%s: nothing must be queued", tc.name)
		}

		c, rec = newCtx(http.MethodGet, "/api/v1/admin/books/ai-review/status", "", tc.role)
		if err := adminRole(h.AIReviewStatus)(c); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		wantStatus := tc.want
		if wantStatus == http.StatusAccepted {
			wantStatus = http.StatusOK
		}
		if rec.Code != wantStatus {
			t.Fatalf("%s status: expected %d, got %d", tc.name, wantStatus, rec.Code)
		}
	}
}
