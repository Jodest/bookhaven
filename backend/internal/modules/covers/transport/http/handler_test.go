package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bookhaven/backend/internal/pkg/coverstore"
	"github.com/labstack/echo/v4"
)

func setup(t *testing.T) (*echo.Echo, string) {
	t.Helper()
	store := coverstore.NewMem()
	hash := strings.Repeat("ab", 32)
	name := "ab/" + hash + ".jpg"
	if err := store.Put(context.Background(), name, []byte("JPEGDATA")); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	NewHandler(store).Register(e)
	return e, name
}

func do(e *echo.Echo, method, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestServe_OKHeaders(t *testing.T) {
	e, name := setup(t)
	rec := do(e, http.MethodGet, "/covers/"+name, nil)
	if rec.Code != 200 || rec.Body.String() != "JPEGDATA" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	h := rec.Header()
	if h.Get("Content-Type") != "image/jpeg" ||
		h.Get("Cache-Control") != "public, max-age=31536000, immutable" ||
		h.Get("X-Content-Type-Options") != "nosniff" ||
		h.Get("ETag") != `"`+strings.Repeat("ab", 32)+`"` ||
		h.Get("Content-Length") != "8" {
		t.Fatalf("headers: %v", h)
	}
}

func TestServe_NotModified(t *testing.T) {
	e, name := setup(t)
	etag := `"` + strings.Repeat("ab", 32) + `"`
	rec := do(e, http.MethodGet, "/covers/"+name, map[string]string{"If-None-Match": etag})
	if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Fatalf("%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") == "" {
		t.Fatal("304 must keep Cache-Control")
	}
}

func TestServe_Head(t *testing.T) {
	e, name := setup(t)
	rec := do(e, http.MethodHead, "/covers/"+name, nil)
	if rec.Code != 200 || rec.Body.Len() != 0 || rec.Header().Get("Content-Length") != "8" {
		t.Fatalf("%d body=%d cl=%s", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Length"))
	}
}

func TestServe_NotFoundCases(t *testing.T) {
	e, _ := setup(t)
	hash := strings.Repeat("cd", 32)
	for _, p := range []string{
		"/covers/cd/" + hash + ".jpg", // unknown
		"/covers/",
		"/covers",
		"/covers/../../etc/passwd",
		"/covers/ab/..%2f..%2fetc%2fpasswd",
		"/covers/ab/" + strings.Repeat("ab", 32) + ".gif",
		"/covers/AB/" + strings.ToUpper(strings.Repeat("ab", 32)) + ".jpg",
		"/covers/zz/zz.jpg",
	} {
		if rec := do(e, http.MethodGet, p, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
}

func TestServe_OtherMethods(t *testing.T) {
	e, name := setup(t)
	if rec := do(e, http.MethodPost, "/covers/"+name, nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", rec.Code)
	}
}

func TestRegisterAt_Posters(t *testing.T) {
	store := coverstore.NewMem()
	hash := strings.Repeat("ab", 32)
	name := "ab/" + hash + ".png"
	if err := store.Put(context.Background(), name, []byte("PNGDATA")); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	NewHandler(store).RegisterAt(e, "/posters")

	rec := do(e, http.MethodGet, "/posters/"+name, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("get = %d %v", rec.Code, rec.Header())
	}
	if rec := do(e, http.MethodGet, "/posters/"+name, map[string]string{"If-None-Match": rec.Header().Get("ETag")}); rec.Code != http.StatusNotModified {
		t.Errorf("etag = %d", rec.Code)
	}
	if rec := do(e, http.MethodHead, "/posters/"+name, nil); rec.Code != http.StatusOK {
		t.Errorf("head = %d", rec.Code)
	}
	for _, p := range []string{"/posters/cd/" + hash + ".png", "/posters/ab/..%2f..%2fetc%2fpasswd", "/posters/../../etc/passwd", "/posters/zz/zz.png"} {
		if rec := do(e, http.MethodGet, p, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d", p, rec.Code)
		}
	}
	// The covers route is not registered by RegisterAt.
	if rec := do(e, http.MethodGet, "/covers/"+name, nil); rec.Code != http.StatusNotFound {
		t.Errorf("covers = %d", rec.Code)
	}
}
