package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bookhaven/backend/internal/pkg/errtrack"
	"github.com/labstack/echo/v4"
)

// ErrorReporting reports requests that end with status >= 500 and panics to
// the error tracker (docs/features/error-tracking.md). 4xx and /health* never
// produce events. A nil reporter makes it a pass-through.
//
// Register it directly after echomiddleware.Recover(): a panic is captured
// here and re-panicked so Recover still answers the usual 500.
func ErrorReporting(r errtrack.Reporter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		if r == nil {
			return next
		}
		return func(c echo.Context) (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					if rec != http.ErrAbortHandler && !isHealthRoute(c) {
						r.CapturePanic(c.Request().Context(), rec, c.Request().Method, c.Path())
					}
					panic(rec)
				}
			}()

			err = next(c)

			status := c.Response().Status
			if err != nil {
				var he *echo.HTTPError
				if errors.As(err, &he) {
					status = he.Code
				} else {
					status = http.StatusInternalServerError
				}
			}
			if status >= http.StatusInternalServerError && !isHealthRoute(c) {
				r.CaptureHTTP(c.Request().Context(), errtrack.HTTPFailure{
					Method: c.Request().Method,
					Route:  c.Path(),
					Status: status,
					Err:    err,
				})
			}
			return err
		}
	}
}

func isHealthRoute(c echo.Context) bool {
	return strings.HasPrefix(c.Path(), "/health") || strings.HasPrefix(c.Request().URL.Path, "/health")
}
