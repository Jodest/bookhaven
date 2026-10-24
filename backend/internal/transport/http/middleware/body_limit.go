package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// BodyLimit caps the request body of every route. The limit is
// overrides["METHOD routePattern"] when present, otherwise defaultLimit.
//
// Register it with e.Use (not e.Pre) so routing has run and c.Path() holds the
// matched route pattern. Over the limit the request is answered with 413 and
// {"error": "request body too large"} before the handler runs. A body without
// a declared length (chunked) is buffered up to the limit so the outcome is a
// deterministic 413 instead of a handler-level bind error.
func BodyLimit(defaultLimit int64, overrides map[string]int64) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			limit := defaultLimit
			if l, ok := overrides[req.Method+" "+c.Path()]; ok {
				limit = l
			}

			if req.ContentLength > limit {
				return bodyTooLarge(c)
			}
			if req.ContentLength == -1 && req.Body != nil && req.Body != http.NoBody {
				buf, err := io.ReadAll(io.LimitReader(req.Body, limit+1))
				if err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
				}
				if int64(len(buf)) > limit {
					return bodyTooLarge(c)
				}
				_ = req.Body.Close()
				req.Body = io.NopCloser(bytes.NewReader(buf))
				req.ContentLength = int64(len(buf))
			}
			return next(c)
		}
	}
}

func bodyTooLarge(c echo.Context) error {
	return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"})
}
