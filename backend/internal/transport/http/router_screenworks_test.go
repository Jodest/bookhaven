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

type stubAdaptationSource struct {
	works map[string][]bookdata.ExternalScreenWork
	err   error
}

func (s *stubAdaptationSource) FetchAdaptations(_ context.Context, _ []string) (map[string][]bookdata.ExternalScreenWork, error) {
	return s.works, s.err
}

func intp(v int) *int { return &v }

func TestScreenWorksRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec(`
CREATE TABLE books (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', slug TEXT, status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT);
CREATE TABLE screen_works (id TEXT PRIMARY KEY, title TEXT NOT NULL, original_title TEXT, kind TEXT NOT NULL, year INTEGER, director TEXT, imdb_id TEXT, kinopoisk_id TEXT, external_source TEXT, external_id TEXT, manually_edited_at DATETIME, poster_source TEXT, poster_url TEXT, poster_path TEXT, poster_author TEXT, poster_license TEXT, poster_license_url TEXT, poster_source_url TEXT, poster_updated_at DATETIME, created_at DATETIME, updated_at DATETIME);
CREATE UNIQUE INDEX uq_screen_works_external ON screen_works (external_source, external_id) WHERE external_id IS NOT NULL;
CREATE TABLE book_screen_works (book_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, hidden BOOLEAN NOT NULL DEFAULT 0, source TEXT NOT NULL DEFAULT 'wikidata', created_at DATETIME, updated_at DATETIME, PRIMARY KEY (book_id, screen_work_id));
`).Error; err != nil {
		t.Fatal(err)
	}
	pubBook, pendingBook := uuid.NewString(), uuid.NewString()
	if err := db.Exec(`INSERT INTO books (id, title, slug, status, external_source, external_id) VALUES
(?, 'Опубликованная', 'pub', 'published', 'wikidata', 'Q1'), (?, 'Ожидает', 'pend', 'pending', NULL, NULL)`, pubBook, pendingBook).Error; err != nil {
		t.Fatal(err)
	}
	src := &stubAdaptationSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {
		{QID: "Q10", Title: "Фильм 1994", Kind: bookdata.ScreenKindFilm, Year: intp(1994), IMDbID: "tt0110413", KinopoiskID: "448"},
		{QID: "Q11", Title: "Сериал 1972", Kind: bookdata.ScreenKindSeries, Year: intp(1972), Director: "Режиссёр"},
		{QID: "Q12", Title: "Скрытый", Kind: bookdata.ScreenKindFilm, Year: intp(1980)},
	}}}
	svc := screenworksservice.NewService(screenworksrepository.NewGormRepository(db), src, nil)

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
	syncBody := `{"ids":["` + pubBook + `"]}`
	adminBase := "/api/v1/admin/books/" + pubBook + "/adaptations"

	// Role and bearer gates on every admin route.
	for _, r := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/admin/books/sync-adaptations", syncBody},
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
		"empty": `{"ids":[]}`, "malformed id": `{"ids":["nope"]}`, "bad json": `{`,
		"51 ids": manyIDs(51),
	} {
		if code, _ := do(http.MethodPost, "/api/v1/admin/books/sync-adaptations", admin, "tok", body); code != http.StatusBadRequest {
			t.Errorf("sync %s: %d", name, code)
		}
	}

	// Public: nothing yet -> 200 with empty list; pending and unknown -> 404.
	code, body := do(http.MethodGet, "/api/v1/books/"+pubBook+"/adaptations", "", "", "")
	if code != http.StatusOK || !strings.Contains(string(body), `"items":[]`) || !strings.Contains(string(body), `"total":0`) {
		t.Errorf("empty public: %d %s", code, body)
	}
	if code, _ := do(http.MethodGet, "/api/v1/books/"+pendingBook+"/adaptations", "", "", ""); code != http.StatusNotFound {
		t.Errorf("pending: %d", code)
	}
	if code, _ := do(http.MethodGet, "/api/v1/books/"+uuid.NewString()+"/adaptations", "", "", ""); code != http.StatusNotFound {
		t.Errorf("unknown: %d", code)
	}
	if code, _ := do(http.MethodGet, "/api/v1/books/not-a-uuid/adaptations", "", "", ""); code != http.StatusBadRequest {
		t.Errorf("bad id: %d", code)
	}

	// Sync adds three links.
	code, body = do(http.MethodPost, "/api/v1/admin/books/sync-adaptations", admin, "tok", syncBody)
	var report struct {
		Processed, Updated, Added int
		Items                     []struct{ Outcome string }
	}
	_ = json.Unmarshal(body, &report)
	if code != http.StatusOK || report.Added != 3 || report.Updated != 1 || report.Items[0].Outcome != "added" {
		t.Fatalf("sync: %d %s", code, body)
	}

	// Hide one; the public list is year-ordered without it.
	code, body = do(http.MethodGet, adminBase, admin, "", "")
	var adminList struct {
		Items []struct {
			ID, Title, Source string
			Hidden            bool
			QID               *string `json:"qid"`
		}
		Total int
	}
	_ = json.Unmarshal(body, &adminList)
	if code != http.StatusOK || adminList.Total != 3 || adminList.Items[0].Title != "Сериал 1972" || adminList.Items[0].Source != "wikidata" || adminList.Items[0].QID == nil {
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
	if code, _ := do(http.MethodPatch, "/api/v1/admin/books/"+uuid.NewString()+"/adaptations/"+hiddenID, admin, "tok", `{"hidden":true}`); code != http.StatusNotFound {
		t.Errorf("patch unknown book: %d", code)
	}
	code, body = do(http.MethodPatch, adminBase+"/"+hiddenID, admin, "tok", `{"hidden":true}`)
	if code != http.StatusOK || !strings.Contains(string(body), `"hidden":true`) {
		t.Fatalf("patch: %d %s", code, body)
	}

	code, body = do(http.MethodGet, "/api/v1/books/"+pubBook+"/adaptations", "", "", "")
	var pub struct {
		Items []struct {
			Title        string  `json:"title"`
			Kind         string  `json:"kind"`
			Year         *int    `json:"year"`
			Director     *string `json:"director"`
			IMDbURL      *string `json:"imdb_url"`
			KinopoiskURL *string `json:"kinopoisk_url"`
		}
		Total int
	}
	_ = json.Unmarshal(body, &pub)
	if code != http.StatusOK || pub.Total != 2 || pub.Items[0].Title != "Сериал 1972" || pub.Items[1].Title != "Фильм 1994" {
		t.Fatalf("public: %d %s", code, body)
	}
	if pub.Items[0].IMDbURL != nil || pub.Items[0].KinopoiskURL != nil || pub.Items[0].Director == nil {
		t.Errorf("series item = %+v", pub.Items[0])
	}
	if u := pub.Items[1].IMDbURL; u == nil || *u != "https://www.imdb.com/title/tt0110413/" {
		t.Errorf("imdb url = %v", u)
	}
	if u := pub.Items[1].KinopoiskURL; u == nil || *u != "https://www.kinopoisk.ru/film/448/" {
		t.Errorf("kinopoisk url = %v", u)
	}

	// Re-sync keeps the link hidden.
	do(http.MethodPost, "/api/v1/admin/books/sync-adaptations", admin, "tok", syncBody)
	if _, body = do(http.MethodGet, "/api/v1/books/"+pubBook+"/adaptations", "", "", ""); !strings.Contains(string(body), `"total":2`) {
		t.Errorf("hidden link came back: %s", body)
	}
	// Show again.
	do(http.MethodPatch, adminBase+"/"+hiddenID, admin, "tok", `{"hidden":false}`)
	if _, body = do(http.MethodGet, "/api/v1/books/"+pubBook+"/adaptations", "", "", ""); !strings.Contains(string(body), `"total":3`) {
		t.Errorf("show again: %s", body)
	}

	// Wikidata down -> 502.
	src.err = bookdata.ErrSourceUnavailable
	if code, _ := do(http.MethodPost, "/api/v1/admin/books/sync-adaptations", admin, "tok", syncBody); code != http.StatusBadGateway {
		t.Errorf("wikidata down: %d", code)
	}

	// The book slug route is untouched.
	if code, _ := do(http.MethodGet, "/api/v1/books/some-slug/adaptations", "", "", ""); code != http.StatusBadRequest {
		t.Errorf("slug-like id on adaptations: %d", code)
	}
}

func TestScreenWorksSyncNotConfigured(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	svc := screenworksservice.NewService(screenworksrepository.NewGormRepository(db), nil, nil)
	e := echo.New()
	RegisterAdminRoutes(e, adminservice.NewService(nil), fakeAuth, &countingEnqueuer{}, nil, nil,
		AIModerationDeps{}, AICuratorDeps{}, AIDescriptionsDeps{}, AICollectionsDeps{}, AIGenresDeps{}, nil, svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/books/sync-adaptations", strings.NewReader(`{"ids":["`+uuid.NewString()+`"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-Test-Role", string(usersdomain.RoleAdmin))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d", rec.Code)
	}
}

func manyIDs(n int) string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = `"` + uuid.NewString() + `"`
	}
	return `{"ids":[` + strings.Join(ids, ",") + `]}`
}
