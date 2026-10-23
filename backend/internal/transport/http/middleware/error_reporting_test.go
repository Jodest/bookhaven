package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bookhaven/backend/internal/pkg/errtrack"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

type fakeReporter struct {
	http   []errtrack.HTTPFailure
	panics []string
}

func (f *fakeReporter) CaptureHTTP(_ context.Context, h errtrack.HTTPFailure) {
	f.http = append(f.http, h)
}
func (f *fakeReporter) CapturePanic(_ context.Context, _ any, method, route string) {
	f.panics = append(f.panics, method+" "+route)
}
func (f *fakeReporter) CaptureTaskFailure(context.Context, errtrack.TaskFailure) {}

func runErrorReporting(t *testing.T, rep errtrack.Reporter, path, target string, h echo.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.Use(echomiddleware.Recover())
	e.Use(ErrorReporting(rep))
	e.GET(path, h)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestErrorReporting_ServerErrors(t *testing.T) {
	tests := []struct {
		name       string
		handler    echo.HandlerFunc
		wantStatus int
	}{
		{"written 500", func(c echo.Context) error { return c.JSON(500, map[string]string{"error": "x"}) }, 500},
		{"returned 503", func(c echo.Context) error { return echo.NewHTTPError(503) }, 503},
		{"plain error", func(c echo.Context) error { return errors.New("boom") }, 500},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rep := &fakeReporter{}
			runErrorReporting(t, rep, "/x/:id", "/x/42?email=a@b.com", tc.handler)
			if len(rep.http) != 1 {
				t.Fatalf("want 1 capture, got %d", len(rep.http))
			}
			got := rep.http[0]
			if got.Route != "/x/:id" || got.Status != tc.wantStatus || got.Method != "GET" {
				t.Errorf("capture = %+v", got)
			}
		})
	}
}

func TestErrorReporting_NoEvents(t *testing.T) {
	for _, code := range []int{200, 400, 401, 403, 404, 409, 422, 429} {
		code := code
		t.Run("written", func(t *testing.T) {
			rep := &fakeReporter{}
			runErrorReporting(t, rep, "/x", "/x", func(c echo.Context) error { return c.NoContent(code) })
			if len(rep.http) != 0 || len(rep.panics) != 0 {
				t.Errorf("status %d produced events", code)
			}
		})
		t.Run("returned", func(t *testing.T) {
			if code < 400 {
				return
			}
			rep := &fakeReporter{}
			runErrorReporting(t, rep, "/x", "/x", func(c echo.Context) error { return echo.NewHTTPError(code) })
			if len(rep.http) != 0 {
				t.Errorf("returned %d produced events", code)
			}
		})
	}
}

func TestErrorReporting_HealthExcluded(t *testing.T) {
	rep := &fakeReporter{}
	rec := runErrorReporting(t, rep, "/health/ready", "/health/ready", func(c echo.Context) error {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "down"})
	})
	if rec.Code != 503 || len(rep.http) != 0 {
		t.Errorf("code=%d captures=%d", rec.Code, len(rep.http))
	}
}

func TestErrorReporting_PanicCapturedAndRecovered(t *testing.T) {
	rep := &fakeReporter{}
	rec := runErrorReporting(t, rep, "/p/:id", "/p/1", func(c echo.Context) error { panic("kaboom") })
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500 from Recover", rec.Code)
	}
	if len(rep.panics) != 1 || rep.panics[0] != "GET /p/:id" {
		t.Errorf("panics = %v", rep.panics)
	}
	if len(rep.http) != 0 {
		t.Errorf("panic must not also produce an HTTP capture: %v", rep.http)
	}
}

func TestErrorReporting_NilReporterPassThrough(t *testing.T) {
	rec := runErrorReporting(t, nil, "/x", "/x", func(c echo.Context) error { return c.JSON(500, "x") })
	if rec.Code != 500 {
		t.Errorf("code = %d", rec.Code)
	}
}
