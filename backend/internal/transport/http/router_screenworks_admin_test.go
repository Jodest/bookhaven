package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	screenworksrepository "github.com/bookhaven/backend/internal/modules/screenworks/repository"
	screenworksservice "github.com/bookhaven/backend/internal/modules/screenworks/service"
	usersdomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Manual editing routes (openspec change add-screen-works-admin-editing).
func TestScreenWorksAdminEditingRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec(`
CREATE TABLE books (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT);
CREATE TABLE authors (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT);
CREATE TABLE screen_works (id TEXT PRIMARY KEY, title TEXT NOT NULL, original_title TEXT, kind TEXT NOT NULL, year INTEGER, director TEXT, imdb_id TEXT, kinopoisk_id TEXT, external_source TEXT, external_id TEXT, manually_edited_at DATETIME, poster_source TEXT, poster_url TEXT, poster_path TEXT, poster_author TEXT, poster_license TEXT, poster_license_url TEXT, poster_source_url TEXT, poster_updated_at DATETIME, created_at DATETIME, updated_at DATETIME);
CREATE UNIQUE INDEX uq_screen_works_external ON screen_works (external_source, external_id) WHERE external_id IS NOT NULL;
CREATE TABLE book_screen_works (book_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, hidden BOOLEAN NOT NULL DEFAULT 0, source TEXT NOT NULL DEFAULT 'wikidata', created_at DATETIME, updated_at DATETIME, PRIMARY KEY (book_id, screen_work_id));
CREATE TABLE author_screen_works (author_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, relation TEXT NOT NULL, hidden BOOLEAN NOT NULL DEFAULT 0, source TEXT NOT NULL DEFAULT 'wikidata', created_at DATETIME, updated_at DATETIME, PRIMARY KEY (author_id, screen_work_id));
`).Error; err != nil {
		t.Fatal(err)
	}
	book, author := uuid.NewString(), uuid.NewString()
	if err := db.Exec(`INSERT INTO books (id, title, status) VALUES (?, 'Книга', 'published')`, book).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO authors (id, name, status) VALUES (?, 'Автор', 'published')`, author).Error; err != nil {
		t.Fatal(err)
	}
	svc := screenworksservice.NewService(screenworksrepository.NewGormRepository(db), nil, nil)

	e := echo.New()
	RegisterAdminRoutes(e, adminservice.NewService(nil), fakeAuth, &countingEnqueuer{}, nil, nil,
		AIModerationDeps{}, AICuratorDeps{}, AIDescriptionsDeps{}, AICollectionsDeps{}, AIGenresDeps{}, nil, svc)
	RegisterRoutes(e, nil, nil, nil, nil, nil, nil, nil, nil, nil, svc, fakeAuth, fakeAuth, nil)

	do := func(method, path, role, bearer, body string) (int, string) {
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
		return rec.Code, rec.Body.String()
	}
	admin := string(usersdomain.RoleAdmin)
	bookBase := "/api/v1/admin/books/" + book + "/adaptations"
	authorBase := "/api/v1/admin/authors/" + author + "/screen-works"
	someID := uuid.NewString()
	newWork := `{"work":{"title":"Собачье сердце","kind":"tv_film","year":1988}}`

	// Role and bearer gates on every new route.
	routes := []struct{ method, path, body string }{
		{http.MethodPost, bookBase, newWork},
		{http.MethodDelete, bookBase + "/" + someID, ""},
		{http.MethodPost, authorBase, newWork},
		{http.MethodDelete, authorBase + "/" + someID, ""},
		{http.MethodGet, "/api/v1/admin/screen-works", ""},
		{http.MethodGet, "/api/v1/admin/screen-works/" + someID, ""},
		{http.MethodPatch, "/api/v1/admin/screen-works/" + someID, `{"title":"x","kind":"film"}`},
		{http.MethodDelete, "/api/v1/admin/screen-works/" + someID, ""},
	}
	for _, r := range routes {
		for _, role := range []usersdomain.Role{usersdomain.RoleModerator, usersdomain.RoleUser} {
			if code, _ := do(r.method, r.path, string(role), "tok", r.body); code != http.StatusForbidden {
				t.Errorf("%s %s %s: %d", role, r.method, r.path, code)
			}
		}
		if r.method != http.MethodGet {
			if code, _ := do(r.method, r.path, admin, "", r.body); code != http.StatusUnauthorized {
				t.Errorf("cookie-only %s %s: %d", r.method, r.path, code)
			}
		}
	}

	// Book: validation and 404s.
	for name, c := range map[string]struct {
		path, body string
		want       int
	}{
		"neither":      {bookBase, `{}`, http.StatusBadRequest},
		"bad json":     {bookBase, `{`, http.StatusBadRequest},
		"bad work id":  {bookBase, `{"screen_work_id":"nope"}`, http.StatusBadRequest},
		"bad book id":  {"/api/v1/admin/books/nope/adaptations", newWork, http.StatusBadRequest},
		"unknown book": {"/api/v1/admin/books/" + uuid.NewString() + "/adaptations", newWork, http.StatusNotFound},
		"unknown work": {bookBase, `{"screen_work_id":"` + someID + `"}`, http.StatusNotFound},
		"422":          {bookBase, `{"work":{"title":"x","kind":"film","year":1850}}`, http.StatusUnprocessableEntity},
	} {
		if code, body := do(http.MethodPost, c.path, admin, "tok", c.body); code != c.want {
			t.Errorf("book %s: %d %s", name, code, body)
		}
	}

	// Book: create, public, duplicate, delete.
	code, body := do(http.MethodPost, bookBase, admin, "tok", `{"work":{"title":"Собачье сердце","kind":"tv_film","year":1988,"imdb_id":"tt0095826","qid":"q1"}}`)
	var created struct {
		ID          string `json:"id"`
		Source      string `json:"source"`
		Hidden      bool   `json:"hidden"`
		WorkCreated bool   `json:"work_created"`
		QID         string `json:"qid"`
	}
	_ = json.Unmarshal([]byte(body), &created)
	if code != http.StatusCreated || created.Source != "manual" || created.Hidden || !created.WorkCreated || created.QID != "Q1" {
		t.Fatalf("create: %d %s", code, body)
	}
	if code, _ := do(http.MethodPost, bookBase, admin, "tok", `{"screen_work_id":"`+created.ID+`"}`); code != http.StatusConflict {
		t.Errorf("already linked: %d", code)
	}
	code, body = do(http.MethodPost, bookBase, admin, "tok", `{"work":{"title":"Дубль","kind":"film","imdb_id":"tt0095826"}}`)
	if code != http.StatusConflict || !strings.Contains(body, `"screen_work_id":"`+created.ID+`"`) || !strings.Contains(body, "imdb id") {
		t.Errorf("dup imdb: %d %s", code, body)
	}
	if _, body = do(http.MethodGet, "/api/v1/books/"+book+"/adaptations", "", "", ""); !strings.Contains(body, "Собачье сердце") {
		t.Errorf("public adaptations: %s", body)
	}
	if code, _ := do(http.MethodDelete, bookBase+"/"+created.ID, admin, "tok", ""); code != http.StatusNoContent {
		t.Errorf("delete link: %d", code)
	}
	if code, _ := do(http.MethodDelete, bookBase+"/"+created.ID, admin, "tok", ""); code != http.StatusNotFound {
		t.Errorf("delete missing link: %d", code)
	}
	if code, _ := do(http.MethodDelete, bookBase+"/nope", admin, "tok", ""); code != http.StatusBadRequest {
		t.Errorf("delete bad id: %d", code)
	}

	// Author: relation, create, patch relation, empty patch.
	if code, body := do(http.MethodPost, authorBase, admin, "tok", `{"screen_work_id":"`+created.ID+`"}`); code != http.StatusBadRequest || !strings.Contains(body, "invalid relation") {
		t.Errorf("missing relation: %d %s", code, body)
	}
	code, body = do(http.MethodPost, authorBase, admin, "tok", `{"screen_work_id":"`+created.ID+`","relation":"documentary"}`)
	if code != http.StatusCreated || !strings.Contains(body, `"relation":"documentary"`) || !strings.Contains(body, `"work_created":false`) {
		t.Fatalf("author create: %d %s", code, body)
	}
	if code, _ := do(http.MethodPost, authorBase, admin, "tok", `{"screen_work_id":"`+created.ID+`","relation":"biopic"}`); code != http.StatusConflict {
		t.Errorf("author already linked: %d", code)
	}
	if code, body := do(http.MethodPatch, authorBase+"/"+created.ID, admin, "tok", `{"relation":"biopic"}`); code != http.StatusOK || !strings.Contains(body, `"relation":"biopic"`) {
		t.Errorf("patch relation: %d %s", code, body)
	}
	if code, body := do(http.MethodPatch, authorBase+"/"+created.ID, admin, "tok", `{}`); code != http.StatusBadRequest || !strings.Contains(body, "hidden or relation is required") {
		t.Errorf("empty patch: %d %s", code, body)
	}
	if code, body := do(http.MethodPatch, authorBase+"/"+created.ID, admin, "tok", `{"relation":"x"}`); code != http.StatusBadRequest || !strings.Contains(body, "invalid relation") {
		t.Errorf("bad relation: %d %s", code, body)
	}

	// Work endpoints.
	workPath := "/api/v1/admin/screen-works/" + created.ID
	if code, body := do(http.MethodGet, workPath, admin, "", ""); code != http.StatusOK || !strings.Contains(body, `"authors":[{`) || !strings.Contains(body, `"books":[]`) {
		t.Errorf("get: %d %s", code, body)
	}
	if code, _ := do(http.MethodGet, "/api/v1/admin/screen-works/nope", admin, "", ""); code != http.StatusBadRequest {
		t.Errorf("get bad id: %d", code)
	}
	if code, _ := do(http.MethodGet, "/api/v1/admin/screen-works/"+someID, admin, "", ""); code != http.StatusNotFound {
		t.Errorf("get unknown: %d", code)
	}
	if code, body := do(http.MethodPatch, workPath, admin, "tok", `{"title":"Новое","kind":"film","qid":"Q9"}`); code != http.StatusUnprocessableEntity || !strings.Contains(body, "qid cannot be changed") {
		t.Errorf("qid change: %d %s", code, body)
	}
	if code, body := do(http.MethodPatch, workPath, admin, "tok", `{"title":"Новое","kind":"film"}`); code != http.StatusOK || !strings.Contains(body, `"title":"Новое"`) || strings.Contains(body, `"manually_edited_at":null`) {
		t.Errorf("patch: %d %s", code, body)
	}
	if code, body := do(http.MethodDelete, workPath, admin, "tok", ""); code != http.StatusConflict || !strings.Contains(body, "screen work has links") {
		t.Errorf("delete linked: %d %s", code, body)
	}
	if code, _ := do(http.MethodDelete, authorBase+"/"+created.ID, admin, "tok", ""); code != http.StatusNoContent {
		t.Errorf("delete author link: %d", code)
	}
	if code, body := do(http.MethodGet, "/api/v1/admin/screen-works?search=Нов&unlinked=true", admin, "", ""); code != http.StatusOK || !strings.Contains(body, `"total":1`) {
		t.Errorf("list: %d %s", code, body)
	}
	if code, _ := do(http.MethodDelete, workPath, admin, "tok", ""); code != http.StatusNoContent {
		t.Errorf("delete work: %d", code)
	}
	if code, _ := do(http.MethodDelete, workPath, admin, "tok", ""); code != http.StatusNotFound {
		t.Errorf("delete work again: %d", code)
	}

	// The static sync route still wins over /:id/screen-works.
	if code, body := do(http.MethodPost, "/api/v1/admin/authors/sync-screen-works", admin, "tok", `{"ids":[]}`); code != http.StatusBadRequest || strings.Contains(body, "invalid author id") {
		t.Errorf("sync route: %d %s", code, body)
	}
}
