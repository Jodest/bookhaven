package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

const testLimit = 100

type bodyLimitRig struct {
	e      *echo.Echo
	called int
	got    []byte
}

func newBodyLimitRig() *bodyLimitRig {
	r := &bodyLimitRig{e: echo.New()}
	r.e.Use(BodyLimit(testLimit, map[string]int64{"POST /big": 2 * testLimit}))
	h := func(c echo.Context) error {
		r.called++
		b, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		r.got = b
		return c.NoContent(http.StatusOK)
	}
	r.e.POST("/small", h)
	r.e.POST("/big", h)
	return r
}

func (r *bodyLimitRig) do(path string, n int, chunked bool) *httptest.ResponseRecorder {
	body := strings.Repeat("a", n)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if chunked {
		req.ContentLength = -1
	}
	rec := httptest.NewRecorder()
	r.e.ServeHTTP(rec, req)
	return rec
}

func assertTooLarge(t *testing.T, rec *httptest.ResponseRecorder, r *bodyLimitRig) {
	t.Helper()
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"error":"request body too large"}` {
		t.Errorf("body = %s", got)
	}
	if r.called != 0 {
		t.Errorf("handler called %d times", r.called)
	}
}

func TestBodyLimit_DeclaredOverLimit(t *testing.T) {
	r := newBodyLimitRig()
	assertTooLarge(t, r.do("/small", testLimit+1, false), r)
}

func TestBodyLimit_AtLimit(t *testing.T) {
	r := newBodyLimitRig()
	if rec := r.do("/small", testLimit, false); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(r.got) != testLimit {
		t.Errorf("handler read %d bytes", len(r.got))
	}
}

func TestBodyLimit_ChunkedOverLimit(t *testing.T) {
	r := newBodyLimitRig()
	assertTooLarge(t, r.do("/small", testLimit+1, true), r)
}

func TestBodyLimit_ChunkedUnderAndAtLimit(t *testing.T) {
	for _, n := range []int{10, testLimit} {
		r := newBodyLimitRig()
		if rec := r.do("/small", n, true); rec.Code != http.StatusOK {
			t.Fatalf("n=%d status = %d", n, rec.Code)
		}
		if !bytes.Equal(r.got, []byte(strings.Repeat("a", n))) {
			t.Errorf("n=%d handler got %d bytes", n, len(r.got))
		}
	}
}

func TestBodyLimit_Override(t *testing.T) {
	r := newBodyLimitRig()
	if rec := r.do("/big", testLimit+50, false); rec.Code != http.StatusOK {
		t.Errorf("override route: status = %d", rec.Code)
	}
	r2 := newBodyLimitRig()
	assertTooLarge(t, r2.do("/small", testLimit+50, false), r2)
	r3 := newBodyLimitRig()
	assertTooLarge(t, r3.do("/big", 2*testLimit+1, false), r3)
	r4 := newBodyLimitRig()
	assertTooLarge(t, r4.do("/big", 2*testLimit+1, true), r4)
}

func TestBodyLimit_UnknownRouteUsesDefault(t *testing.T) {
	r := newBodyLimitRig()
	if rec := r.do("/nope", testLimit+1, false); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
	if rec := r.do("/nope", 10, false); rec.Code != http.StatusNotFound {
		t.Errorf("small body on unknown route: status = %d, want 404", rec.Code)
	}
}
