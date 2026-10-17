package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	aigenresservice "github.com/bookhaven/backend/internal/modules/aigenres/service"
	usersdomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// stubSuggestions answers every call with empty data; the router test only
// checks the middleware chain.
type stubSuggestions struct{ calls int }

func (s *stubSuggestions) Active() bool { return true }
func (s *stubSuggestions) Status(ctx context.Context) (*aigenresservice.Status, error) {
	s.calls++
	return &aigenresservice.Status{}, nil
}
func (s *stubSuggestions) StartAnalysis(ctx context.Context, userID uuid.UUID) (*domain.Run, error) {
	s.calls++
	return &domain.Run{ID: uuid.New()}, nil
}
func (s *stubSuggestions) MarkRunEnqueueFailed(ctx context.Context, id uuid.UUID) error { return nil }
func (s *stubSuggestions) List(ctx context.Context, f aigenresservice.ListFilter) ([]aigenresservice.SuggestionView, int64, error) {
	s.calls++
	return nil, 0, nil
}
func (s *stubSuggestions) Apply(ctx context.Context, userID, id uuid.UUID, target *uuid.UUID) (*aigenresservice.ApplyResult, error) {
	s.calls++
	return &aigenresservice.ApplyResult{View: &aigenresservice.SuggestionView{Suggestion: domain.Suggestion{ID: id}}}, nil
}
func (s *stubSuggestions) ApplyBulk(ctx context.Context, userID uuid.UUID, items []aigenresservice.ApplyItem) *aigenresservice.BulkApplyReport {
	s.calls++
	return &aigenresservice.BulkApplyReport{}
}
func (s *stubSuggestions) Dismiss(ctx context.Context, userID, id uuid.UUID) (*aigenresservice.SuggestionView, error) {
	s.calls++
	return &aigenresservice.SuggestionView{Suggestion: domain.Suggestion{ID: id}}, nil
}
func (s *stubSuggestions) DismissBulk(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) *aigenresservice.BulkDismissReport {
	s.calls++
	return &aigenresservice.BulkDismissReport{}
}

func TestAIGenresRoutes_RoleAndBearer(t *testing.T) {
	e := echo.New()
	stub := &stubSuggestions{}
	RegisterAdminRoutes(e, adminservice.NewService(nil), fakeAuth, stubEnqueuer{}, nil, nil,
		AIModerationDeps{}, AICuratorDeps{}, AIDescriptionsDeps{}, AICollectionsDeps{}, AIGenresDeps{Service: stub}, nil, nil)

	do := func(method, path, body, role, bearer string) int {
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
	base := "/api/v1/admin/genres/ai-suggestions"
	routes := []struct {
		method, path, body string
		bearerOnly         bool
		ok                 int
	}{
		{http.MethodGet, base, "", false, http.StatusOK},
		{http.MethodGet, base + "/status", "", false, http.StatusOK},
		{http.MethodPost, base + "/analyze", "", true, http.StatusAccepted},
		{http.MethodPost, base + "/apply", `{"items":[{"id":"` + id + `"}]}`, true, http.StatusOK},
		{http.MethodPost, base + "/dismiss", `{"ids":["` + id + `"]}`, true, http.StatusOK},
		{http.MethodPost, base + "/" + id + "/apply", "", true, http.StatusOK},
		{http.MethodPost, base + "/" + id + "/dismiss", "", true, http.StatusOK},
	}
	for _, r := range routes {
		for _, role := range []usersdomain.Role{usersdomain.RoleModerator, usersdomain.RoleUser} {
			if code := do(r.method, r.path, r.body, string(role), "tok"); code != http.StatusForbidden {
				t.Errorf("%s %s %s: %d, want 403", role, r.method, r.path, code)
			}
		}
		if r.bearerOnly {
			if code := do(r.method, r.path, r.body, string(usersdomain.RoleAdmin), ""); code != http.StatusUnauthorized {
				t.Errorf("%s %s without bearer: %d, want 401", r.method, r.path, code)
			}
		}
	}
	if stub.calls != 0 {
		t.Fatalf("rejected requests must not reach the service, calls=%d", stub.calls)
	}

	// An admin passes everywhere; the static routes are not swallowed by /:id.
	for _, r := range routes {
		if code := do(r.method, r.path, r.body, string(usersdomain.RoleAdmin), "tok"); code != r.ok {
			t.Errorf("admin %s %s: %d, want %d", r.method, r.path, code, r.ok)
		}
	}
	if stub.calls != len(routes) {
		t.Errorf("calls = %d, want %d", stub.calls, len(routes))
	}
}
