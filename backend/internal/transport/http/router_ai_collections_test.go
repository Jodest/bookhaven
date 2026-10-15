package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aicollections/domain"
	draftservice "github.com/bookhaven/backend/internal/modules/aicollections/service"
	usersdomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// stubDrafts answers every call with empty data; the router test only checks
// the middleware chain.
type stubDrafts struct{ creates int }

func (s *stubDrafts) Active() bool { return true }
func (s *stubDrafts) Status(ctx context.Context, userID uuid.UUID) (*draftservice.Status, error) {
	return &draftservice.Status{}, nil
}
func (s *stubDrafts) CreateDraft(ctx context.Context, userID uuid.UUID, collectionID, brief string, count *int) (*domain.Draft, error) {
	s.creates++
	return &domain.Draft{ID: uuid.New(), Status: domain.StatusQueued}, nil
}
func (s *stubDrafts) MarkEnqueueFailed(ctx context.Context, id uuid.UUID) error { return nil }
func (s *stubDrafts) ListDrafts(ctx context.Context, collectionID uuid.UUID) ([]domain.Draft, error) {
	return nil, nil
}
func (s *stubDrafts) GetDraft(ctx context.Context, id uuid.UUID) (*domain.Draft, *draftservice.Report, error) {
	return &domain.Draft{ID: id}, nil, nil
}

func TestAICollectionsRoutes_RoleAndBearer(t *testing.T) {
	e := echo.New()
	stub := &stubDrafts{}
	RegisterAdminRoutes(e, adminservice.NewService(nil), fakeAuth, stubEnqueuer{}, nil, nil,
		AIModerationDeps{}, AICuratorDeps{}, AIDescriptionsDeps{}, AICollectionsDeps{Service: stub}, nil, nil)

	do := func(method, path, role, bearer string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(`{"brief":"x"}`))
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

	id, col := uuid.NewString(), uuid.NewString()
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/ai-collections/status"},
		{http.MethodGet, "/api/v1/admin/ai-collections/drafts?collection_id=" + col},
		{http.MethodGet, "/api/v1/admin/ai-collections/drafts/" + id},
		{http.MethodPost, "/api/v1/admin/ai-collections/drafts"},
	} {
		for _, role := range []usersdomain.Role{usersdomain.RoleModerator, usersdomain.RoleUser} {
			if code := do(r.method, r.path, string(role), "tok"); code != http.StatusForbidden {
				t.Errorf("%s %s %s: %d", role, r.method, r.path, code)
			}
		}
	}

	if code := do(http.MethodPost, "/api/v1/admin/ai-collections/drafts", string(usersdomain.RoleAdmin), ""); code != http.StatusUnauthorized {
		t.Errorf("POST without bearer: %d", code)
	}
	if stub.creates != 0 {
		t.Errorf("rejected requests must not reach the service")
	}

	// /status is not swallowed by /drafts/:id, and an admin passes.
	if code := do(http.MethodGet, "/api/v1/admin/ai-collections/status", string(usersdomain.RoleAdmin), ""); code != http.StatusOK {
		t.Errorf("admin status: %d", code)
	}
	if code := do(http.MethodGet, "/api/v1/admin/ai-collections/drafts/"+id, string(usersdomain.RoleAdmin), ""); code != http.StatusOK {
		t.Errorf("admin get: %d", code)
	}
	if code := do(http.MethodPost, "/api/v1/admin/ai-collections/drafts", string(usersdomain.RoleAdmin), "tok"); code != http.StatusAccepted || stub.creates != 1 {
		t.Errorf("admin create: %d (creates %d)", code, stub.creates)
	}
}
