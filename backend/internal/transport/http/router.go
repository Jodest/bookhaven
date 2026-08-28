package http

import (
	"github.com/bookhaven/backend/internal/transport/http/handlers"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers all API routes.
func RegisterRoutes(e *echo.Echo) {
	api := e.Group("/api/v1")

	bookHandler := handlers.NewBookHandler()
	api.GET("/books", bookHandler.List)
	api.GET("/books/:slug", bookHandler.Get)

}
