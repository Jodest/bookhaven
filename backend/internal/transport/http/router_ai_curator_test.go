package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	curatorservice "github.com/bookhaven/backend/internal/modules/aicurator/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	usersdomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

// stubCurator answers every call with empty data; the router test only
// checks the middleware chain.
type stubCurator struct{ creates int }

func (s *stubCurator) Active() bool { return true }
func (s *stubCurator) Status(ctx context.Context, userID uuid.UUID) (*curatorservice.Status, error) {
	return &curatorservice.Status{}, nil
}
func (s *stubCurator) CreateRequest(ctx context.Context, userID uuid.UUID, prompt string, maxNames *int) (*domain.Request, error) {
	s.creates++
	return &domain.Request{ID: uuid.New(), Status: domain.StatusQueued}, nil
}
func (s *stubCurator) MarkEnqueueFailed(ctx context.Context, id uuid.UUID) error { return nil }
func (s *stubCurator) ListRecent(ctx context.Context) ([]domain.Request, error) {
	return nil, nil
}
func (s *stubCurator) GetRequest(ctx context.Context, id uuid.UUID) (*domain.Request, map[string]catalogDomain.Author, error) {
	return &domain.Request{ID: id}, nil, nil
}

type stubEnqueuer struct{}

func (stubEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return &asynq.TaskInfo{ID: "t"}, nil
}

// fakeAuth authenticates from the X-Test-Role header (no tokens involved).
func fakeAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		role := c.Request().Header.Get("X-Test-Role")
		if role == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}
		id := uuid.New()
		c.Set("user_id", id.String())
		c.Set("user", &usersdomain.User{ID: id, Role: usersdomain.Role(role)})
		return next(c)
	}
}

func TestAICuratorRoutes_RoleAndBearer(t *testing.T) {
	e := echo.New()
	stub := &stubCurator{}
	RegisterAdminRoutes(e, adminservice.NewService(nil), fakeAuth, stubEnqueuer{}, nil, nil,
		AIModerationDeps{}, AICuratorDeps{Service: stub})

	do := func(method, path, role, bearer string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(`{"prompt":"x"}`))
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
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/ai-curator/status"},
		{http.MethodGet, "/api/v1/admin/ai-curator/requests"},
		{http.MethodGet, "/api/v1/admin/ai-curator/requests/" + id},
		{http.MethodPost, "/api/v1/admin/ai-curator/requests"},
	} {
		if code := do(r.method, r.path, string(usersdomain.RoleModerator), "tok"); code != http.StatusForbidden {
			t.Errorf("moderator %s %s: %d", r.method, r.path, code)
		}
		if code := do(r.method, r.path, string(usersdomain.RoleUser), "tok"); code != http.StatusForbidden {
			t.Errorf("user %s %s: %d", r.method, r.path, code)
		}
	}

	if code := do(http.MethodPost, "/api/v1/admin/ai-curator/requests", string(usersdomain.RoleAdmin), ""); code != http.StatusUnauthorized {
		t.Errorf("POST without bearer: %d", code)
	}
	if stub.creates != 0 {
		t.Errorf("rejected requests must not reach the service")
	}
	if code := do(http.MethodPost, "/api/v1/admin/ai-curator/requests", string(usersdomain.RoleAdmin), "tok"); code != http.StatusAccepted {
		t.Errorf("admin POST: %d", code)
	}
	if code := do(http.MethodGet, "/api/v1/admin/ai-curator/status", string(usersdomain.RoleAdmin), ""); code != http.StatusOK {
		t.Errorf("admin status (cookie-style, no bearer): %d", code)
	}
}
