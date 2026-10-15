package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	curatedrepository "github.com/bookhaven/backend/internal/modules/curated/repository"
	curatedservice "github.com/bookhaven/backend/internal/modules/curated/service"
	usersdomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

func TestCuratedRoutes_RoleBearerAndPublic(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
CREATE TABLE genres (id TEXT PRIMARY KEY, name TEXT, slug TEXT, status TEXT, merged_into_id TEXT);
CREATE TABLE curated_collections (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL DEFAULT 'manual', size INTEGER NOT NULL, source TEXT, source_url TEXT, genre_id TEXT, is_published BOOLEAN NOT NULL DEFAULT 0, published_at DATETIME, rule_metric TEXT, rule_period_days INTEGER, rule_min_ratings INTEGER, rule_confidence INTEGER, computed_at DATETIME, baseline_at DATETIME, created_at DATETIME, updated_at DATETIME);
CREATE TABLE curated_collection_items (collection_id TEXT NOT NULL, book_id TEXT NOT NULL, position INTEGER NOT NULL, added_at DATETIME, previous_position INTEGER, score REAL, PRIMARY KEY (collection_id, book_id));
CREATE TABLE ratings (id TEXT PRIMARY KEY, book_id TEXT NOT NULL, user_id TEXT NOT NULL, value INTEGER NOT NULL, created_at DATETIME, updated_at DATETIME);
CREATE TABLE shelves (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, name TEXT, type TEXT NOT NULL DEFAULT 'custom');
CREATE TABLE shelf_items (shelf_id TEXT NOT NULL, book_id TEXT NOT NULL, added_at DATETIME, PRIMARY KEY (shelf_id, book_id));
CREATE TABLE books (id TEXT PRIMARY KEY, status TEXT);`).Error; err != nil {
		t.Fatal(err)
	}
	svc := curatedservice.NewService(curatedrepository.NewGormRepository(db))

	e := echo.New()
	RegisterAdminRoutes(e, adminservice.NewService(nil), fakeAuth, &countingEnqueuer{}, nil, nil,
		AIModerationDeps{}, AICuratorDeps{}, AIDescriptionsDeps{}, AICollectionsDeps{}, svc, nil)
	RegisterRoutes(e, nil, nil, nil, nil, nil, nil, nil, nil, nil, svc, nil, fakeAuth, fakeAuth, nil)

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

	id, book := uuid.NewString(), uuid.NewString()
	base := "/api/v1/admin/collections"
	routes := []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodPost, base, `{"title":"Ok title","size":2}`},
		{http.MethodGet, base + "/" + id, ""},
		{http.MethodPatch, base + "/" + id, `{}`},
		{http.MethodDelete, base + "/" + id, ""},
		{http.MethodPost, base + "/" + id + "/recompute", ""},
		{http.MethodPost, base + "/" + id + "/books", `{"book_id":"` + book + `"}`},
		{http.MethodPost, base + "/" + id + "/books/paste/preview", `{"text":"Мать"}`},
		{http.MethodPost, base + "/" + id + "/books/bulk", `{"book_ids":["` + book + `"]}`},
		{http.MethodDelete, base + "/" + id + "/books/" + book, ""},
		{http.MethodPut, base + "/" + id + "/books/" + book + "/position", `{"position":1}`},
	}
	for _, r := range routes {
		for _, role := range []usersdomain.Role{usersdomain.RoleModerator, usersdomain.RoleUser} {
			if code := do(r.method, r.path, string(role), "tok", r.body); code != http.StatusForbidden {
				t.Errorf("%s %s %s: %d", role, r.method, r.path, code)
			}
		}
		if r.method != http.MethodGet {
			if code := do(r.method, r.path, string(usersdomain.RoleAdmin), "", r.body); code != http.StatusUnauthorized {
				t.Errorf("cookie-only %s %s: %d", r.method, r.path, code)
			}
		}
	}
	if code := do(http.MethodGet, base, string(usersdomain.RoleAdmin), "", ""); code != http.StatusOK {
		t.Errorf("admin list: %d", code)
	}
	if code := do(http.MethodPost, base, string(usersdomain.RoleAdmin), "tok", `{"title":"Ok title","size":2}`); code != http.StatusCreated {
		t.Errorf("admin create: %d", code)
	}
	// Public routes need no token.
	if code := do(http.MethodGet, "/api/v1/collections", "", "", ""); code != http.StatusOK {
		t.Errorf("public index: %d", code)
	}
	if code := do(http.MethodGet, "/api/v1/collections/ok-title", "", "", ""); code != http.StatusNotFound {
		t.Errorf("public draft: %d", code)
	}
	// Per-book places are public: unknown book 404, bad id 400, never 401/403.
	if code := do(http.MethodGet, "/api/v1/books/"+uuid.NewString()+"/collections", "", "", ""); code != http.StatusNotFound {
		t.Errorf("public book places unknown: %d", code)
	}
	if code := do(http.MethodGet, "/api/v1/books/not-a-uuid/collections", "", "", ""); code != http.StatusBadRequest {
		t.Errorf("public book places bad id: %d", code)
	}
}
