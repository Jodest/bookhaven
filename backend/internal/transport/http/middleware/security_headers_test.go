package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

func newSecurityEcho() *echo.Echo {
	e := echo.New()
	e.Use(SecurityHeaders())
	e.GET("/api/v1/ok", func(c echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"a": "b"}) })
	e.GET("/api/v1/fail", func(c echo.Context) error { return echo.NewHTTPError(http.StatusUnauthorized, "no") })
	e.GET("/api/v1/boom", func(c echo.Context) error { return errors.New("boom") })
	e.GET("/swagger/index.html", func(c echo.Context) error { return c.String(http.StatusOK, "x") })
	return e
}

func TestSecurityHeaders(t *testing.T) {
	cases := []struct {
		name, path string
		status     int
		csp        string
	}{
		{"json route", "/api/v1/ok", 200, apiCSP},
		{"404", "/nope", 404, apiCSP},
		{"handler http error", "/api/v1/fail", 401, apiCSP},
		{"handler plain error", "/api/v1/boom", 500, apiCSP},
		{"swagger", "/swagger/index.html", 200, swaggerCSP},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newSecurityEcho().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			want := map[string]string{
				"Content-Security-Policy": tc.csp,
				"X-Frame-Options":         "DENY",
				"X-Content-Type-Options":  "nosniff",
				"Referrer-Policy":         "no-referrer",
			}
			for k, v := range want {
				if got := rec.Header().Values(k); len(got) != 1 || got[0] != v {
					t.Errorf("%s = %v, want [%s]", k, got, v)
				}
			}
			if v := rec.Header().Get("Strict-Transport-Security"); v != "" {
				t.Errorf("unexpected HSTS %q", v)
			}
		})
	}
}

func TestSecurityHeaders_CORSPreflightStillWorks(t *testing.T) {
	e := echo.New()
	e.Use(SecurityHeaders())
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000"},
		AllowMethods: []string{http.MethodGet},
	}))
	e.GET("/api/v1/ok", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/ok", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("ACAO = %q", got)
	}
	if rec.Header().Get("Content-Security-Policy") != apiCSP {
		t.Error("CSP missing on preflight")
	}
}
