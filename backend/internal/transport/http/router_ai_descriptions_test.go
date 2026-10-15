package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	descservice "github.com/bookhaven/backend/internal/modules/aidescriptions/service"
	usersdomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

// stubDescriptions answers every call with empty data and counts the
// state-changing calls; the router test only checks the middleware chain.
type stubDescriptions struct{ writes int }

func (s *stubDescriptions) Active() bool   { return true }
func (s *stubDescriptions) Now() time.Time { return time.Now() }
func (s *stubDescriptions) Status(ctx context.Context) (*descservice.Status, error) {
	return &descservice.Status{}, nil
}
func (s *stubDescriptions) List(ctx context.Context, status string, page, limit int) (*descservice.ListResult, error) {
	return &descservice.ListResult{Page: 1, Limit: 20}, nil
}
func (s *stubDescriptions) Get(ctx context.Context, bookID uuid.UUID) (*domain.Item, error) {
	return nil, nil
}
func (s *stubDescriptions) Approve(ctx context.Context, bookID, userID uuid.UUID, text *string) (*domain.Item, error) {
	s.writes++
	return &domain.Item{Description: domain.Description{BookID: bookID}}, nil
}
func (s *stubDescriptions) Discard(ctx context.Context, bookID, userID uuid.UUID) (*domain.Item, error) {
	s.writes++
	return &domain.Item{Description: domain.Description{BookID: bookID}}, nil
}

func TestAIDescriptionsRoutes_RoleAndBearer(t *testing.T) {
	e := echo.New()
	stub := &stubDescriptions{}
	enq := &countingEnqueuer{}
	RegisterAdminRoutes(e, adminservice.NewService(nil), fakeAuth, enq, nil, nil,
		AIModerationDeps{}, AICuratorDeps{}, AIDescriptionsDeps{Service: stub}, AICollectionsDeps{}, nil)

	do := func(method, path, role, bearer, body string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		if role != "" {
			req.Header.Set("X-Test-Role", role)
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	id := uuid.NewString()
	base := "/api/v1/admin/ai-descriptions"
	genBody := `{"book_ids":["` + id + `"]}`
	routes := []struct{ method, path, body string }{
		{http.MethodGet, base + "/status", ""},
		{http.MethodGet, base, ""},
		{http.MethodGet, base + "/" + id, ""},
		{http.MethodPost, base + "/generate", genBody},
		{http.MethodPost, base + "/" + id + "/approve", ""},
		{http.MethodPost, base + "/" + id + "/discard", ""},
	}
	for _, r := range routes {
		if code := do(r.method, r.path, string(usersdomain.RoleModerator), "tok", r.body); code != http.StatusForbidden {
			t.Errorf("moderator %s %s: %d", r.method, r.path, code)
		}
		if code := do(r.method, r.path, string(usersdomain.RoleUser), "tok", r.body); code != http.StatusForbidden {
			t.Errorf("user %s %s: %d", r.method, r.path, code)
		}
	}
	for _, r := range routes {
		if r.method != http.MethodPost {
			continue
		}
		if code := do(r.method, r.path, string(usersdomain.RoleAdmin), "", r.body); code != http.StatusUnauthorized {
			t.Errorf("POST %s without bearer: %d", r.path, code)
		}
	}
	if stub.writes != 0 || enq.n != 0 {
		t.Fatalf("rejected requests must not reach the service or the queue")
	}

	// /status and /generate are static routes, not book ids.
	if code := do(http.MethodGet, base+"/status", string(usersdomain.RoleAdmin), "", ""); code != http.StatusOK {
		t.Errorf("admin status: %d", code)
	}
	if code := do(http.MethodPost, base+"/generate", string(usersdomain.RoleAdmin), "tok", genBody); code != http.StatusAccepted || enq.n != 1 {
		t.Errorf("admin generate: %d (queued %d)", code, enq.n)
	}
	if code := do(http.MethodPost, base+"/"+id+"/approve", string(usersdomain.RoleAdmin), "tok", ""); code != http.StatusOK {
		t.Errorf("admin approve: %d", code)
	}
	if code := do(http.MethodPost, base+"/"+id+"/discard", string(usersdomain.RoleAdmin), "tok", ""); code != http.StatusOK {
		t.Errorf("admin discard: %d", code)
	}
}

type countingEnqueuer struct{ n int }

func (c *countingEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	c.n++
	return &asynq.TaskInfo{ID: "t"}, nil
}
