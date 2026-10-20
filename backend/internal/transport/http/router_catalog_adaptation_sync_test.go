package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	screenworksservice "github.com/bookhaven/backend/internal/modules/screenworks/service"
	usersdomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

type activeSyncInspector struct{ active []*asynq.TaskInfo }

func (activeSyncInspector) GetTaskInfo(string, string) (*asynq.TaskInfo, error) {
	return nil, asynq.ErrTaskNotFound
}
func (activeSyncInspector) ListPendingTasks(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return nil, nil
}
func (a activeSyncInspector) ListActiveTasks(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return a.active, nil
}
func (activeSyncInspector) ListRetryTasks(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return nil, nil
}
func (activeSyncInspector) ListArchivedTasks(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return nil, nil
}
func (activeSyncInspector) ListCompletedTasks(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return nil, nil
}

func TestCatalogAdaptationSyncRoute(t *testing.T) {
	enq := &stateEnqueuer{}
	insp := &activeSyncInspector{}
	e := echo.New()
	RegisterAdminRoutes(e, adminservice.NewService(nil), fakeAuth, enq, insp, nil,
		AIModerationDeps{}, AICuratorDeps{}, AIDescriptionsDeps{}, AICollectionsDeps{}, AIGenresDeps{}, nil, nil)

	do := func(role, bearer string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/books/sync-adaptations/catalog", nil)
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
	admin := string(usersdomain.RoleAdmin)

	if code := do("", "tok"); code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d", code)
	}
	if code := do(admin, ""); code != http.StatusUnauthorized {
		t.Errorf("no bearer: %d", code)
	}
	if code := do(string(usersdomain.RoleModerator), "tok"); code != http.StatusForbidden {
		t.Errorf("moderator: %d", code)
	}
	if enq.n != 0 {
		t.Fatalf("queued before an authorized call: %d", enq.n)
	}
	if code := do(admin, "tok"); code != http.StatusCreated || enq.n != 1 {
		t.Errorf("admin: %d queued=%d", code, enq.n)
	}
	insp.active = []*asynq.TaskInfo{{Type: screenworksservice.TaskTypeCatalogSync}}
	if code := do(admin, "tok"); code != http.StatusConflict || enq.n != 1 {
		t.Errorf("running: %d queued=%d", code, enq.n)
	}
}

type stateEnqueuer struct{ countingEnqueuer }

func (s *stateEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	s.n++
	return &asynq.TaskInfo{ID: "t", Queue: "harvest", State: asynq.TaskStatePending}, nil
}
