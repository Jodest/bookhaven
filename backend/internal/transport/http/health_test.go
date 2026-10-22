package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type fakeChecker struct {
	name string
	fn   func(ctx context.Context) error
}

func (f fakeChecker) Name() string                    { return f.name }
func (f fakeChecker) Check(ctx context.Context) error { return f.fn(ctx) }

func okCheck(name string) fakeChecker {
	return fakeChecker{name, func(context.Context) error { return nil }}
}

func failCheck(name, msg string) fakeChecker {
	return fakeChecker{name, func(context.Context) error { return errors.New(msg) }}
}

func hangCheck(name string) fakeChecker {
	return fakeChecker{name, func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }}
}

func doHealth(t *testing.T, path string, timeout time.Duration, checks ...HealthChecker) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	RegisterHealthRoutes(e, zap.NewNop(), timeout, checks...)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthLivenessIgnoresChecks(t *testing.T) {
	rec := doHealth(t, "/health", time.Second, failCheck("database", "boom"))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestHealthReadyAllOK(t *testing.T) {
	rec := doHealth(t, "/health/ready", time.Second, okCheck("database"), okCheck("redis"))
	want := `{"status":"ok","checks":{"database":"ok","redis":"ok"}}`
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("missing no-store")
	}
}

func TestHealthReadyDependencyDown(t *testing.T) {
	cases := []struct{ name, want string }{
		{"database", `{"status":"unavailable","checks":{"database":"unavailable","redis":"ok"}}`},
		{"redis", `{"status":"unavailable","checks":{"database":"ok","redis":"unavailable"}}`},
	}
	for _, tc := range cases {
		db, rd := HealthChecker(okCheck("database")), HealthChecker(okCheck("redis"))
		if tc.name == "database" {
			db = failCheck("database", "dial tcp 10.0.0.5:5432: secret-host refused")
		} else {
			rd = failCheck("redis", "dial tcp 10.0.0.6:6379: secret-host refused")
		}
		rec := doHealth(t, "/health/ready", time.Second, db, rd)
		if rec.Code != http.StatusServiceUnavailable || strings.TrimSpace(rec.Body.String()) != tc.want {
			t.Fatalf("%s: got %d %s", tc.name, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "secret-host") || strings.Contains(rec.Body.String(), "dial") {
			t.Fatalf("%s: error text leaked: %s", tc.name, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("missing no-store")
		}
	}
}

func TestHealthReadyHangingChecksTimeOutConcurrently(t *testing.T) {
	start := time.Now()
	rec := doHealth(t, "/health/ready", 50*time.Millisecond, hangCheck("database"), hangCheck("redis"))
	if el := time.Since(start); el > 500*time.Millisecond {
		t.Fatalf("took too long: %s", el)
	}
	want := `{"status":"unavailable","checks":{"database":"unavailable","redis":"unavailable"}}`
	if rec.Code != http.StatusServiceUnavailable || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}
