package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	screenworksrepository "github.com/bookhaven/backend/internal/modules/screenworks/repository"
	screenworksservice "github.com/bookhaven/backend/internal/modules/screenworks/service"
	usersdomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type stubAuthorFilmSource struct {
	films map[string][]bookdata.ExternalAuthorFilm
	err   error
}

func (s *stubAuthorFilmSource) FetchAuthorFilms(_ context.Context, _ []string) (map[string][]bookdata.ExternalAuthorFilm, error) {
	return s.films, s.err
}

func authorFilm(qid, title, relation string, year int) bookdata.ExternalAuthorFilm {
	return bookdata.ExternalAuthorFilm{
		ExternalScreenWork: bookdata.ExternalScreenWork{QID: qid, Title: title, Kind: bookdata.ScreenKindFilm, Year: intp(year)},
		Relation:           relation,
	}
}

func TestAuthorScreenWorksRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec(`
CREATE TABLE authors (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT);
CREATE TABLE screen_works (id TEXT PRIMARY KEY, title TEXT NOT NULL, original_title TEXT, kind TEXT NOT NULL, year INTEGER, director TEXT, imdb_id TEXT, kinopoisk_id TEXT, external_source TEXT, external_id TEXT, manually_edited_at DATETIME, poster_source TEXT, poster_url TEXT, poster_path TEXT, poster_author TEXT, poster_license TEXT, poster_license_url TEXT, poster_source_url TEXT, poster_updated_at DATETIME, poster_copy_source_url TEXT, poster_copy_path TEXT, poster_copy_status TEXT, poster_copy_error TEXT, poster_copy_attempts INTEGER NOT NULL DEFAULT 0, poster_copy_next_attempt_at DATETIME, poster_copy_fetched_at DATETIME, poster_copy_checked_at DATETIME, poster_tmdb_ref TEXT, poster_verified_at DATETIME, poster_refresh_at DATETIME, created_at DATETIME, updated_at DATETIME);
CREATE UNIQUE INDEX uq_screen_works_external ON screen_works (external_source, external_id) WHERE external_id IS NOT NULL;
CREATE TABLE author_screen_works (author_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, relation TEXT NOT NULL, hidden BOOLEAN NOT NULL DEFAULT 0, source TEXT NOT NULL DEFAULT 'wikidata', created_at DATETIME, updated_at DATETIME, PRIMARY KEY (author_id, screen_work_id));
`).Error; err != nil {
		t.Fatal(err)
	}
	pubAuthor, pendingAuthor := uuid.NewString(), uuid.NewString()
	if err := db.Exec(`INSERT INTO authors (id, name, status, external_source, external_id) VALUES
(?, 'Толкин', 'published', 'wikidata', 'Q892'), (?, 'Ожидает', 'pending', NULL, NULL)`, pubAuthor, pendingAuthor).Error; err != nil {
		t.Fatal(err)
	}
	src := &stubAuthorFilmSource{films: map[string][]bookdata.ExternalAuthorFilm{"Q892": {
		authorFilm("Q1", "Толкин", "biopic", 2019),
		authorFilm("Q2", "Документальный", "documentary", 2011),
		authorFilm("Q3", "Скрытый", "biopic", 2000),
	}}}
	svc := screenworksservice.NewService(screenworksrepository.NewGormRepository(db), nil, nil).WithAuthorSource(src)

	e := echo.New()
	RegisterAdminRoutes(e, adminservice.NewService(nil), fakeAuth, &countingEnqueuer{}, nil, nil,
		AIModerationDeps{}, AICuratorDeps{}, AIDescriptionsDeps{}, AICollectionsDeps{}, AIGenresDeps{}, nil, svc)
	RegisterRoutes(e, nil, nil, nil, nil, nil, nil, nil, nil, nil, svc, fakeAuth, fakeAuth, nil)

	do := func(method, path, role, bearer, body string) (int, []byte) {
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
		return rec.Code, rec.Body.Bytes()
	}
	admin := string(usersdomain.RoleAdmin)
	syncPath := "/api/v1/admin/authors/sync-screen-works"
	syncBody := `{"ids":["` + pubAuthor + `"]}`
	adminBase := "/api/v1/admin/authors/" + pubAuthor + "/screen-works"

	// The slug route still exists next to the new one.
	var hasSlug bool
	for _, r := range e.Routes() {
		if r.Method == http.MethodGet && r.Path == "/api/v1/authors/:slug" {
			hasSlug = true
		}
	}
	if !hasSlug {
		t.Error("GET /authors/:slug is not registered")
	}

	// Role and bearer gates.
	for _, r := range []struct{ method, path, body string }{
		{http.MethodPost, syncPath, syncBody},
		{http.MethodGet, adminBase, ""},
		{http.MethodPatch, adminBase + "/" + uuid.NewString(), `{"hidden":true}`},
	} {
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

	// Sync validation.
	for name, body := range map[string]string{
		"empty": `{"ids":[]}`, "malformed id": `{"ids":["nope"]}`, "bad json": `{`, "51 ids": manyIDs(51),
	} {
		if code, _ := do(http.MethodPost, syncPath, admin, "tok", body); code != http.StatusBadRequest {
			t.Errorf("sync %s: %d", name, code)
		}
	}

	// Public: anonymous, empty list; pending/unknown -> 404; slug -> 400.
	code, body := do(http.MethodGet, "/api/v1/authors/"+pubAuthor+"/screen-works", "", "", "")
	if code != http.StatusOK || !strings.Contains(string(body), `"items":[]`) || !strings.Contains(string(body), `"total":0`) {
		t.Errorf("empty public: %d %s", code, body)
	}
	if code, _ := do(http.MethodGet, "/api/v1/authors/"+pendingAuthor+"/screen-works", "", "", ""); code != http.StatusNotFound {
		t.Errorf("pending: %d", code)
	}
	if code, _ := do(http.MethodGet, "/api/v1/authors/"+uuid.NewString()+"/screen-works", "", "", ""); code != http.StatusNotFound {
		t.Errorf("unknown: %d", code)
	}
	if code, body := do(http.MethodGet, "/api/v1/authors/tolkien/screen-works", "", "", ""); code != http.StatusBadRequest || !strings.Contains(string(body), "invalid author id") {
		t.Errorf("slug: %d %s", code, body)
	}

	// Sync adds three links.
	code, body = do(http.MethodPost, syncPath, admin, "tok", syncBody)
	var report struct {
		Processed, Updated, Added int
		Items                     []struct{ Outcome string }
	}
	_ = json.Unmarshal(body, &report)
	if code != http.StatusOK || report.Added != 3 || report.Updated != 1 || report.Items[0].Outcome != "added" {
		t.Fatalf("sync: %d %s", code, body)
	}

	code, body = do(http.MethodGet, adminBase, admin, "", "")
	var adminList struct {
		Items []struct {
			ID, Title, Source, Relation string
			Hidden                      bool
			QID                         *string `json:"qid"`
		}
		Total int
	}
	_ = json.Unmarshal(body, &adminList)
	if code != http.StatusOK || adminList.Total != 3 || adminList.Items[0].Title != "Скрытый" || adminList.Items[0].Source != "wikidata" || adminList.Items[0].QID == nil {
		t.Fatalf("admin list: %d %s", code, body)
	}
	var hiddenID string
	for _, it := range adminList.Items {
		if it.Title == "Скрытый" {
			hiddenID = it.ID
		}
	}
	if code, _ := do(http.MethodPatch, adminBase+"/"+hiddenID, admin, "tok", `{}`); code != http.StatusBadRequest {
		t.Errorf("patch without hidden: %d", code)
	}
	if code, _ := do(http.MethodPatch, adminBase+"/"+uuid.NewString(), admin, "tok", `{"hidden":true}`); code != http.StatusNotFound {
		t.Errorf("patch unknown link: %d", code)
	}
	if code, _ := do(http.MethodPatch, adminBase+"/nope", admin, "tok", `{"hidden":true}`); code != http.StatusBadRequest {
		t.Errorf("patch bad work id: %d", code)
	}
	code, body = do(http.MethodPatch, adminBase+"/"+hiddenID, admin, "tok", `{"hidden":true}`)
	if code != http.StatusOK || !strings.Contains(string(body), `"hidden":true`) {
		t.Fatalf("patch: %d %s", code, body)
	}

	code, body = do(http.MethodGet, "/api/v1/authors/"+pubAuthor+"/screen-works", "", "", "")
	var pub struct {
		Items []struct {
			Title    string `json:"title"`
			Kind     string `json:"kind"`
			Relation string `json:"relation"`
			Year     *int   `json:"year"`
		}
		Total int
	}
	_ = json.Unmarshal(body, &pub)
	if code != http.StatusOK || pub.Total != 2 || pub.Items[0].Title != "Документальный" || pub.Items[0].Relation != "documentary" ||
		pub.Items[1].Title != "Толкин" || pub.Items[1].Relation != "biopic" || pub.Items[1].Kind != "film" {
		t.Fatalf("public: %d %s", code, body)
	}

	// Re-sync keeps the link hidden; show again.
	do(http.MethodPost, syncPath, admin, "tok", syncBody)
	if _, body = do(http.MethodGet, "/api/v1/authors/"+pubAuthor+"/screen-works", "", "", ""); !strings.Contains(string(body), `"total":2`) {
		t.Errorf("hidden link came back: %s", body)
	}
	do(http.MethodPatch, adminBase+"/"+hiddenID, admin, "tok", `{"hidden":false}`)
	if _, body = do(http.MethodGet, "/api/v1/authors/"+pubAuthor+"/screen-works", "", "", ""); !strings.Contains(string(body), `"total":3`) {
		t.Errorf("show again: %s", body)
	}

	// Wikidata down -> 502.
	src.err = bookdata.ErrSourceUnavailable
	if code, _ := do(http.MethodPost, syncPath, admin, "tok", syncBody); code != http.StatusBadGateway {
		t.Errorf("wikidata down: %d", code)
	}
}

func TestAuthorScreenWorksSyncNotConfigured(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	svc := screenworksservice.NewService(screenworksrepository.NewGormRepository(db), nil, nil)
	e := echo.New()
	RegisterAdminRoutes(e, adminservice.NewService(nil), fakeAuth, &countingEnqueuer{}, nil, nil,
		AIModerationDeps{}, AICuratorDeps{}, AIDescriptionsDeps{}, AICollectionsDeps{}, AIGenresDeps{}, nil, svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/authors/sync-screen-works", strings.NewReader(`{"ids":["`+uuid.NewString()+`"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-Test-Role", string(usersdomain.RoleAdmin))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d", rec.Code)
	}
}
