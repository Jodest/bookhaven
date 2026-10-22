// Package http serves self-hosted cover files (#489) at GET|HEAD
// /covers/{shard}/{file}. The route is public: no auth, no CSRF.
package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bookhaven/backend/internal/pkg/coverstore"
	"github.com/labstack/echo/v4"
)

var contentTypes = map[string]string{
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

type Handler struct {
	store coverstore.Store
}

func NewHandler(store coverstore.Store) *Handler { return &Handler{store: store} }

// Register adds GET and HEAD /covers/:shard/:file.
func (h *Handler) Register(e *echo.Echo) {
	e.GET("/covers/:shard/:file", h.Serve)
	e.HEAD("/covers/:shard/:file", h.Serve)
}

// Serve streams one cover file: 200 with the image and immutable cache
// headers, 304 on a matching If-None-Match, 404 for a malformed or unknown
// name. HEAD gets the headers only. It is deliberately not a swagger route:
// it lives at the root (/covers), outside the documented /api/v1 base path.
func (h *Handler) Serve(c echo.Context) error {
	name := c.Param("shard") + "/" + c.Param("file")
	if !coverstore.ValidName(name) {
		return c.NoContent(http.StatusNotFound)
	}
	f, info, err := h.store.Open(c.Request().Context(), name)
	if err != nil {
		if errors.Is(err, coverstore.ErrNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		return c.NoContent(http.StatusInternalServerError)
	}
	defer f.Close()

	ext := name[strings.LastIndex(name, "."):]
	hash := name[3 : len(name)-len(ext)]
	hdr := c.Response().Header()
	hdr.Set("Content-Type", contentTypes[ext])
	hdr.Set("Cache-Control", "public, max-age=31536000, immutable")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("ETag", strconv.Quote(hash))
	http.ServeContent(c.Response(), c.Request(), name, info.ModTime, f)
	return nil
}
