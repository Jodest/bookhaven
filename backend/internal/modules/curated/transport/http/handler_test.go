package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/curated/repository"
	"github.com/bookhaven/backend/internal/modules/curated/service"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const ddl = `
CREATE TABLE books (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', slug TEXT NOT NULL DEFAULT '', description TEXT, cover_url TEXT, isbn TEXT, published_year INTEGER, page_count INTEGER, language TEXT DEFAULT 'ru', rating REAL DEFAULT 0, ratings_count INTEGER DEFAULT 0, reviews_count INTEGER DEFAULT 0, status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT, created_at DATETIME, updated_at DATETIME, published_at DATETIME);
CREATE TABLE authors (id TEXT PRIMARY KEY, name TEXT, slug TEXT, status TEXT);
CREATE TABLE genres (id TEXT PRIMARY KEY, name TEXT, slug TEXT, status TEXT, merged_into_id TEXT);
CREATE TABLE book_authors (book_id TEXT NOT NULL, author_id TEXT NOT NULL);
CREATE TABLE book_genres (book_id TEXT NOT NULL, genre_id TEXT NOT NULL);
CREATE TABLE curated_collections (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL DEFAULT 'manual', size INTEGER NOT NULL, source TEXT, source_url TEXT, genre_id TEXT, is_published BOOLEAN NOT NULL DEFAULT 0, published_at DATETIME, created_at DATETIME, updated_at DATETIME);
CREATE TABLE curated_collection_items (collection_id TEXT NOT NULL, book_id TEXT NOT NULL, position INTEGER NOT NULL, added_at DATETIME, PRIMARY KEY (collection_id, book_id));
`

type env struct {
	e  *echo.Echo
	db *gorm.DB
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatal(err)
	}
	h := NewHandler(service.NewService(repository.NewGormRepository(db)))
	e := echo.New()
	e.GET("/collections", h.ListPublic)
	e.GET("/collections/:slug", h.GetPublic)
	a := e.Group("/admin/collections")
	a.GET("", h.AdminList)
	a.POST("", h.AdminCreate)
	a.GET("/:id", h.AdminGet)
	a.PATCH("/:id", h.AdminUpdate)
	a.DELETE("/:id", h.AdminDelete)
	a.POST("/:id/books", h.AdminAddBook)
	a.DELETE("/:id/books/:bookId", h.AdminRemoveBook)
	a.PUT("/:id/books/:bookId/position", h.AdminMoveBook)
	return &env{e: e, db: db}
}

func (v *env) do(method, path, body string) (int, map[string]any) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	v.e.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func (v *env) book(t *testing.T, status string) string {
	t.Helper()
	id := uuid.NewString()
	now := time.Now()
	if err := v.db.Exec("INSERT INTO books (id, title, slug, status, created_at, updated_at) VALUES (?, 'К', ?, ?, ?, ?)", id, id, status, now, now).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func (v *env) collection(t *testing.T, body string) string {
	t.Helper()
	code, out := v.do(http.MethodPost, "/admin/collections", body)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %v", code, out)
	}
	return out["id"].(string)
}

func TestPublic_DraftAndUnknown404(t *testing.T) {
	v := newEnv(t)
	v.collection(t, `{"title":"Черновик","size":3,"slug":"draft"}`)
	for _, slug := range []string{"draft", "nope"} {
		code, out := v.do(http.MethodGet, "/collections/"+slug, "")
		if code != 404 || out["error"] != "collection not found" {
			t.Errorf("%s: %d %v", slug, code, out)
		}
	}
	code, out := v.do(http.MethodGet, "/collections", "")
	if code != 200 || out["total"].(float64) != 0 || len(out["items"].([]any)) != 0 {
		t.Fatalf("index: %d %v", code, out)
	}
}

func TestPublic_PagesAndPlaces(t *testing.T) {
	v := newEnv(t)
	id := v.collection(t, `{"title":"Топ","size":3,"slug":"top","is_published":true}`)
	for i := 0; i < 4; i++ {
		b := v.book(t, "published")
		if code, out := v.do(http.MethodPost, "/admin/collections/"+id+"/books", `{"book_id":"`+b+`"}`); code != 201 {
			t.Fatalf("add: %d %v", code, out)
		}
	}
	code, out := v.do(http.MethodGet, "/collections/top?page=2&limit=2", "")
	items := out["items"].([]any)
	if code != 200 || len(items) != 1 || items[0].(map[string]any)["place"].(float64) != 3 || out["total"].(float64) != 3 {
		t.Fatalf("page 2: %d %v", code, out)
	}
	_, out = v.do(http.MethodGet, "/collections/top?limit=500", "")
	if out["limit"].(float64) != 100 {
		t.Fatal("limit clamp")
	}
	coll := out["collection"].(map[string]any)
	if coll["shown_count"].(float64) != 3 || coll["kind"] != "manual" || coll["genre"] != nil {
		t.Fatalf("collection: %v", coll)
	}
	_, out = v.do(http.MethodGet, "/collections", "")
	if len(out["items"].([]any)) != 1 {
		t.Fatal("index must list the published collection")
	}
}

func TestAdmin_ErrorMapping(t *testing.T) {
	v := newEnv(t)
	id := v.collection(t, `{"title":"Подборка","size":2,"slug":"one"}`)

	check := func(name, method, path, body string, want int, msg string) {
		t.Helper()
		code, out := v.do(method, path, body)
		got, _ := out["error"].(string)
		if code != want || (msg != "" && !strings.HasPrefix(got, msg)) {
			t.Errorf("%s: got %d %v, want %d %q", name, code, out, want, msg)
		}
	}
	check("short title", "POST", "/admin/collections", `{"title":"a","size":2}`, 400, "title")
	check("bad size", "POST", "/admin/collections", `{"title":"Ok title","size":0}`, 400, "size")
	check("bad json", "POST", "/admin/collections", `{`, 400, "invalid request")
	check("slug taken", "POST", "/admin/collections", `{"title":"Ok title","size":2,"slug":"one"}`, 409, "slug already exists")
	check("genre missing", "POST", "/admin/collections", `{"title":"Ok title","size":2,"genre_id":"`+uuid.NewString()+`"}`, 400, "invalid genre_id")
	check("genre not uuid", "POST", "/admin/collections", `{"title":"Ok title","size":2,"genre_id":"x"}`, 400, "invalid genre_id")
	check("get bad id", "GET", "/admin/collections/zzz", "", 400, "")
	check("get unknown", "GET", "/admin/collections/"+uuid.NewString(), "", 404, "collection not found")
	check("patch unknown", "PATCH", "/admin/collections/"+uuid.NewString(), `{}`, 404, "collection not found")
	check("delete unknown", "DELETE", "/admin/collections/"+uuid.NewString(), "", 404, "collection not found")

	pub, rej := v.book(t, "published"), v.book(t, "rejected")
	base := "/admin/collections/" + id + "/books"
	check("add", "POST", base, `{"book_id":"`+pub+`"}`, 201, "")
	check("add dup", "POST", base, `{"book_id":"`+pub+`"}`, 409, "book already in collection")
	check("add rejected", "POST", base, `{"book_id":"`+rej+`"}`, 422, "book is rejected")
	check("add unknown", "POST", base, `{"book_id":"`+uuid.NewString()+`"}`, 404, "book not found")
	check("add bad id", "POST", base, `{"book_id":"x"}`, 400, "")
	check("add to unknown collection", "POST", "/admin/collections/"+uuid.NewString()+"/books", `{"book_id":"`+pub+`"}`, 404, "collection not found")
	check("move invalid", "PUT", base+"/"+pub+"/position", `{"position":0}`, 400, "invalid position")
	check("move not integer", "PUT", base+"/"+pub+"/position", `{"position":"a"}`, 400, "invalid position")
	check("move fractional", "PUT", base+"/"+pub+"/position", `{"position":1.5}`, 400, "invalid position")
	check("move missing", "PUT", base+"/"+rej+"/position", `{"position":1}`, 404, "book not in collection")
	check("remove missing", "DELETE", base+"/"+rej, "", 404, "book not in collection")
	check("remove", "DELETE", base+"/"+pub, "", 200, "")
}

func TestAdmin_PatchClearsFieldsAndDetail(t *testing.T) {
	v := newEnv(t)
	g := uuid.NewString()
	v.db.Exec("INSERT INTO genres (id, name, slug, status) VALUES (?, 'Фантастика', 'fantastika', 'published')", g)
	id := v.collection(t, `{"title":"Подборка","size":2,"genre_id":"`+g+`","source":"BBC","source_url":"https://bbc.co.uk"}`)

	code, out := v.do("GET", "/admin/collections/"+id, "")
	if code != 200 || out["genre"].(map[string]any)["status"] != "published" || out["source"] != "BBC" {
		t.Fatalf("detail: %d %v", code, out)
	}
	code, out = v.do("PATCH", "/admin/collections/"+id, `{"genre_id":null,"source":"","source_url":""}`)
	if code != 200 || out["genre"] != nil || out["source"] != nil || out["source_url"] != nil {
		t.Fatalf("clear: %d %v", code, out)
	}
	// absent genre_id leaves it alone
	v.do("PATCH", "/admin/collections/"+id, `{"genre_id":"`+g+`"}`)
	code, out = v.do("PATCH", "/admin/collections/"+id, `{"title":"Новое имя"}`)
	if code != 200 || out["genre"] == nil || out["title"] != "Новое имя" {
		t.Fatalf("keep genre: %d %v", code, out)
	}

	// hidden book in the middle, move, list and delete
	a, b, c := v.book(t, "published"), v.book(t, "pending"), v.book(t, "published")
	for _, x := range []string{a, b, c} {
		v.do("POST", "/admin/collections/"+id+"/books", `{"book_id":"`+x+`"}`)
	}
	_, out = v.do("GET", "/admin/collections/"+id, "")
	items := out["items"].([]any)
	st := func(i int) string { return items[i].(map[string]any)["state"].(string) }
	if st(0) != "ranked" || st(1) != "hidden" || st(2) != "ranked" || items[1].(map[string]any)["place"] != nil {
		t.Fatalf("states: %v", items)
	}
	if code, _ := v.do("PUT", "/admin/collections/"+id+"/books/"+c+"/position", `{"position":1}`); code != 200 {
		t.Fatal("move")
	}
	_, out = v.do("GET", "/admin/collections/"+id, "")
	if out["items"].([]any)[0].(map[string]any)["book"].(map[string]any)["id"] != c {
		t.Fatal("moved to top")
	}
	_, out = v.do("GET", "/admin/collections", "")
	row := out["items"].([]any)[0].(map[string]any)
	if row["books_count"].(float64) != 3 || row["shown_count"].(float64) != 2 {
		t.Fatalf("list row: %v", row)
	}
	if code, _ := v.do("DELETE", "/admin/collections/"+id, ""); code != 200 {
		t.Fatal("delete")
	}
}
