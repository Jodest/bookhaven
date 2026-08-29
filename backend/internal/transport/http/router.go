package http

import (
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	cataloghttp "github.com/bookhaven/backend/internal/modules/catalog/transport/http"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers all public API routes.
func RegisterRoutes(e *echo.Echo, catalogService *catalogservice.Service) {
	api := e.Group("/api/v1")

	catalogHandler := cataloghttp.NewHandler(catalogService)
	api.GET("/books", catalogHandler.ListBooks)
	api.GET("/books/:slug", catalogHandler.GetBook)
	api.GET("/authors/:slug", catalogHandler.GetAuthor)
	api.GET("/genres", catalogHandler.ListGenres)
}
