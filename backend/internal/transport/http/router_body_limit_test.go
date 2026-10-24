package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	curatedrepository "github.com/bookhaven/backend/internal/modules/curated/repository"
	curatedservice "github.com/bookhaven/backend/internal/modules/curated/service"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"
)

const testOrigin = "http://localhost:3000"

// newBodyLimitRouterEcho wires the middleware in the cmd/api order
// (CORS, then BodyLimit) over the real admin/curated routes plus a stub
// register route (the real one lives in cmd/api).
func newBodyLimitRouterEcho(t *testing.T) *echo.Echo {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
CREATE TABLE curated_collections (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL DEFAULT 'manual', size INTEGER NOT NULL, source TEXT, source_url TEXT, genre_id TEXT, is_published BOOLEAN NOT NULL DEFAULT 0, published_at DATETIME, rule_metric TEXT, rule_period_days INTEGER, rule_min_ratings INTEGER, rule_confidence INTEGER, computed_at DATETIME, baseline_at DATETIME, created_at DATETIME, updated_at DATETIME);
CREATE TABLE curated_collection_items (collection_id TEXT NOT NULL, book_id TEXT NOT NULL, position INTEGER NOT NULL, added_at DATETIME, previous_position INTEGER, score REAL, PRIMARY KEY (collection_id, book_id));
CREATE TABLE books (id TEXT PRIMARY KEY, status TEXT, title TEXT, deleted_at DATETIME);`).Error; err != nil {
		t.Fatal(err)
	}
	svc := curatedservice.NewService(curatedrepository.NewGormRepository(db))

	e := echo.New()
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{AllowOrigins: []string{testOrigin}, AllowCredentials: true}))
	e.Use(middleware.BodyLimit(DefaultBodyLimit, BodyLimitOverrides()))
	RegisterAdminRoutes(e, adminservice.NewService(nil), fakeAuth, &countingEnqueuer{}, nil, nil,
		AIModerationDeps{}, AICuratorDeps{}, AIDescriptionsDeps{}, AICollectionsDeps{}, AIGenresDeps{}, svc, nil)
	e.POST("/api/v1/auth/register", func(c echo.Context) error { return c.NoContent(nethttp.StatusOK) })
	return e
}

func importRequest(t *testing.T, size int) *nethttp.Request {
	t.Helper()
	// 501 rows (over the 500-row limit => 422 before the service is
	// touched), padded with descriptions to roughly size bytes.
	const rows = 501
	pad := strings.Repeat("x", size/rows)
	items := make([]map[string]string, rows)
	for i := range items {
		items[i] = map[string]string{"title": fmt.Sprintf("T%d", i), "description": pad}
	}
	file, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "books.json")
	_, _ = fw.Write(file)
	_ = mw.Close()
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/admin/books/import", &buf)
	req.Header.Set(echo.HeaderContentType, mw.FormDataContentType())
	req.Header.Set("X-Test-Role", "admin")
	req.Header.Set("Authorization", "Bearer tok")
	return req
}

func TestBodyLimit_Router_RegisterOverDefault(t *testing.T) {
	e := newBodyLimitRouterEcho(t)
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/register", strings.NewReader(strings.Repeat("a", int(DefaultBodyLimit)+1)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderOrigin, testOrigin)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != nethttp.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"error":"request body too large"}` {
		t.Errorf("body = %s", got)
	}
	if rec.Header().Get(echo.HeaderAccessControlAllowOrigin) != testOrigin {
		t.Errorf("missing CORS header on 413: %v", rec.Header())
	}
}

func TestBodyLimit_Router_ImportOverride(t *testing.T) {
	e := newBodyLimitRouterEcho(t)

	rec := httptest.NewRecorder()
	req := importRequest(t, 3<<19) // ~1.5 MiB
	if req.ContentLength <= DefaultBodyLimit {
		t.Fatalf("test body too small: %d", req.ContentLength)
	}
	e.ServeHTTP(rec, req)
	if rec.Code != nethttp.StatusUnprocessableEntity {
		t.Errorf("1.5 MiB import: status = %d (%s), want 422", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, importRequest(t, 2<<20+1024))
	if rec.Code != nethttp.StatusRequestEntityTooLarge {
		t.Errorf("over 2 MiB import: status = %d, want 413", rec.Code)
	}
}

func TestBodyLimit_Router_MaxPastePreviewFits(t *testing.T) {
	e := newBodyLimitRouterEcho(t)
	// 100 000 Cyrillic characters, ASCII-escaped as \uXXXX (6 bytes each).
	var sb strings.Builder
	sb.WriteString(`{"text":"`)
	for i := 0; i < 100000; i++ {
		sb.WriteString(`м`)
	}
	sb.WriteString(`"}`)
	if int64(sb.Len()) > DefaultBodyLimit {
		t.Fatalf("body %d exceeds default limit", sb.Len())
	}
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/admin/collections/"+uuid.NewString()+"/books/paste/preview", strings.NewReader(sb.String()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-Test-Role", "admin")
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code == nethttp.StatusRequestEntityTooLarge {
		t.Errorf("max paste preview rejected with 413")
	}
}
