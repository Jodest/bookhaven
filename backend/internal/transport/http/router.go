package http

import (
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	cataloghttp "github.com/bookhaven/backend/internal/modules/catalog/transport/http"
	ratingsservice "github.com/bookhaven/backend/internal/modules/ratings/service"
	ratingshttp "github.com/bookhaven/backend/internal/modules/ratings/transport/http"
	reviewsservice "github.com/bookhaven/backend/internal/modules/reviews/service"
	reviewshttp "github.com/bookhaven/backend/internal/modules/reviews/transport/http"
	shelvesservice "github.com/bookhaven/backend/internal/modules/shelves/service"
	shelveshttp "github.com/bookhaven/backend/internal/modules/shelves/transport/http"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers all public API routes.
func RegisterRoutes(
	e *echo.Echo,
	catalogService *catalogservice.Service,
	shelvesService *shelvesservice.Service,
	ratingsService *ratingsservice.Service,
	reviewsService *reviewsservice.Service,
	authMiddleware echo.MiddlewareFunc,
	optionalAuth echo.MiddlewareFunc,
) {
	api := e.Group("/api/v1")

	catalogHandler := cataloghttp.NewHandler(catalogService)
	api.GET("/books", catalogHandler.ListBooks)
	api.GET("/books/:slug", catalogHandler.GetBook)
	api.GET("/authors", catalogHandler.ListAuthors)
	api.GET("/authors/:slug", catalogHandler.GetAuthor)
	api.GET("/genres", catalogHandler.ListGenres)
	api.GET("/languages", catalogHandler.ListLanguages)

	shelvesHandler := shelveshttp.NewHandler(shelvesService)
	shelves := api.Group("/shelves", authMiddleware)
	shelves.GET("", shelvesHandler.ListShelves)
	shelves.POST("", shelvesHandler.CreateShelf)
	shelves.GET("/:id", shelvesHandler.GetShelf)
	shelves.PATCH("/:id", shelvesHandler.RenameShelf)
	shelves.DELETE("/:id", shelvesHandler.DeleteShelf)
	shelves.POST("/:id/books", shelvesHandler.AddBook)
	shelves.DELETE("/:id/books/:bookId", shelvesHandler.RemoveBook)

	ratingsHandler := ratingshttp.NewHandler(ratingsService)
	reviewsHandler := reviewshttp.NewHandler(reviewsService)

	api.POST("/books/:id/ratings", ratingsHandler.SetRating, authMiddleware)
	api.GET("/books/:id/ratings/me", ratingsHandler.GetMyRating, authMiddleware)
	api.GET("/books/:id/reviews", reviewsHandler.ListReviews, optionalAuth)
	api.POST("/books/:id/reviews", reviewsHandler.CreateReview, authMiddleware)

	reviews := api.Group("/reviews", authMiddleware)
	reviews.PATCH("/:id", reviewsHandler.UpdateReview)
	reviews.DELETE("/:id", reviewsHandler.DeleteReview)
	reviews.POST("/:id/likes", reviewsHandler.ToggleLike)
}
