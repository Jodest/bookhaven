package http

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bookhaven/backend/internal/modules/screenworks/repository"
	"github.com/bookhaven/backend/internal/modules/screenworks/service"
	"github.com/bookhaven/backend/internal/pkg/coverstore"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func posterFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	if err := f.db.Exec(`
CREATE TABLE authors (id TEXT PRIMARY KEY, name TEXT, status TEXT);
CREATE TABLE author_screen_works (author_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, relation TEXT, hidden BOOLEAN NOT NULL DEFAULT 0, source TEXT NOT NULL DEFAULT 'wikidata', created_at DATETIME, updated_at DATETIME, PRIMARY KEY (author_id, screen_work_id));`).Error; err != nil {
		t.Fatal(err)
	}
	f.h = NewHandler(service.NewService(repository.NewGormRepository(f.db), nil, nil, service.WithPosterStore(coverstore.NewMem())))
	return f
}

func uploadReq(t *testing.T, field, filename string, data []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if field != "" {
		part, err := mw.CreateFormFile(field, filename)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(data)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPut, "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func runWithID(h echo.HandlerFunc, req *http.Request, id string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)
	_ = h(c)
	return rec
}

var (
	jpegData = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{1}, 64)...)
	pdfData  = append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte{2}, 64)...)
)

func TestAdminUploadPoster(t *testing.T) {
	f := posterFixture(t)
	id := f.work.String()

	rec := runWithID(f.h.AdminUploadPoster, uploadReq(t, "file", "p.jpg", jpegData), id)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload = %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Poster *struct {
			URL    string `json:"url"`
			Source string `json:"source"`
		} `json:"poster"`
		PosterRemoved bool `json:"poster_removed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Poster == nil || got.Poster.Source != "manual" || len(got.Poster.URL) < 12 || got.Poster.URL[:9] != "/posters/" || got.PosterRemoved {
		t.Errorf("detail = %s", rec.Body.String())
	}

	if rec := runWithID(f.h.AdminUploadPoster, uploadReq(t, "other", "p.jpg", jpegData), id); rec.Code != http.StatusBadRequest || errMsg(t, rec) != "file is required" {
		t.Errorf("no file = %d", rec.Code)
	}
	rec = runWithID(f.h.AdminUploadPoster, uploadReq(t, "file", "p.jpg", pdfData), id)
	if rec.Code != http.StatusUnprocessableEntity || errMsg(t, rec) != "poster must be a JPEG, PNG or WebP image" {
		t.Errorf("pdf named .jpg = %d %s", rec.Code, rec.Body.String())
	}
	big := append([]byte{0xFF, 0xD8, 0xFF}, make([]byte, 5<<20)...)
	rec = runWithID(f.h.AdminUploadPoster, uploadReq(t, "file", "p.jpg", big), id)
	if rec.Code != http.StatusRequestEntityTooLarge || errMsg(t, rec) != "poster is too large" {
		t.Errorf("big = %d", rec.Code)
	}
	rec = runWithID(f.h.AdminUploadPoster, uploadReq(t, "file", "p.jpg", jpegData), uuid.NewString())
	if rec.Code != http.StatusNotFound || errMsg(t, rec) != "screen work not found" {
		t.Errorf("unknown = %d", rec.Code)
	}
	if rec := runWithID(f.h.AdminUploadPoster, uploadReq(t, "file", "p.jpg", jpegData), "x"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id = %d", rec.Code)
	}
}

func TestAdminDeletePoster(t *testing.T) {
	f := posterFixture(t)
	id := f.work.String()
	if rec := runWithID(f.h.AdminUploadPoster, uploadReq(t, "file", "p.jpg", jpegData), id); rec.Code != http.StatusOK {
		t.Fatalf("upload = %d", rec.Code)
	}
	del := func(id string) *httptest.ResponseRecorder {
		return runWithID(f.h.AdminDeletePoster, httptest.NewRequest(http.MethodDelete, "/", nil), id)
	}
	for i := 0; i < 2; i++ { // idempotent
		rec := del(id)
		if rec.Code != http.StatusOK {
			t.Fatalf("delete #%d = %d", i, rec.Code)
		}
		var got map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got["poster"] != nil || got["poster_removed"] != false {
			t.Errorf("detail = %s", rec.Body.String())
		}
	}
	if rec := del(uuid.NewString()); rec.Code != http.StatusNotFound {
		t.Errorf("unknown = %d", rec.Code)
	}
}

func TestListBookAdaptations_PosterObject(t *testing.T) {
	f := posterFixture(t)
	rec := runWithID(f.h.ListBookAdaptations, httptest.NewRequest(http.MethodGet, "/", nil), f.book.String())
	var res struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || len(res.Items) != 1 {
		t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
	}
	if v, ok := res.Items[0]["poster"]; !ok || v != nil {
		t.Errorf("poster must be null: %s", rec.Body.String())
	}
	if rec := runWithID(f.h.AdminUploadPoster, uploadReq(t, "file", "p.jpg", jpegData), f.work.String()); rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	rec = runWithID(f.h.ListBookAdaptations, httptest.NewRequest(http.MethodGet, "/", nil), f.book.String())
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	p, _ := res.Items[0]["poster"].(map[string]any)
	if p == nil || p["source"] != "manual" || p["author"] != nil {
		t.Errorf("poster = %s", rec.Body.String())
	}
}
