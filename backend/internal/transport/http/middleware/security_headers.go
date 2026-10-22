package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	apiCSP = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
	// Swagger UI needs its own scripts/styles/inline bootstrap.
	swaggerCSP = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; " +
		"base-uri 'self'; form-action 'self'"
)

// SecurityHeaders sets baseline hardening headers on every API response.
// Headers are set before the handler runs, so they also land on errors, 404s
// and CORS preflights. HSTS is deliberately not set here: only the TLS
// terminating proxy (Caddy) knows the connection is HTTPS.
func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Response().Header()
			if strings.HasPrefix(c.Request().URL.Path, "/swagger/") {
				h.Set("Content-Security-Policy", swaggerCSP)
			} else {
				h.Set("Content-Security-Policy", apiCSP)
			}
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "no-referrer")
			return next(c)
		}
	}
}
