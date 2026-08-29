package http

import (
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	cataloghttp "github.com/bookhaven/backend/internal/modules/catalog/transport/http"
	shelvesservice "github.com/bookhaven/backend/internal/modules/shelves/service"
	shelveshttp "github.com/bookhaven/backend/internal/modules/shelves/transport/http"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers all public API routes.
func RegisterRoutes(e *echo.Echo, catalogService *catalogservice.Service, shelvesService *shelvesservice.Service, authMiddleware echo.MiddlewareFunc) {
	api := e.Group("/api/v1")

	catalogHandler := cataloghttp.NewHandler(catalogService)
	api.GET("/books", catalogHandler.ListBooks)
	api.GET("/books/:slug", catalogHandler.GetBook)
	api.GET("/authors/:slug", catalogHandler.GetAuthor)
	api.GET("/genres", catalogHandler.ListGenres)

	shelvesHandler := shelveshttp.NewHandler(shelvesService)
	shelves := api.Group("/shelves", authMiddleware)
	shelves.GET("", shelvesHandler.ListShelves)
	shelves.POST("", shelvesHandler.CreateShelf)
	shelves.GET("/:id", shelvesHandler.GetShelf)
	shelves.PATCH("/:id", shelvesHandler.RenameShelf)
	shelves.DELETE("/:id", shelvesHandler.DeleteShelf)
	shelves.POST("/:id/books", shelvesHandler.AddBook)
	shelves.DELETE("/:id/books/:bookId", shelvesHandler.RemoveBook)
}
