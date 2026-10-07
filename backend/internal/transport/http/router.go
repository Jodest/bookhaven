package http

import (
	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	adminhttp "github.com/bookhaven/backend/internal/modules/admin/transport/http"
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	cataloghttp "github.com/bookhaven/backend/internal/modules/catalog/transport/http"
	harvesthttp "github.com/bookhaven/backend/internal/modules/harvest/transport/http"
	homeservice "github.com/bookhaven/backend/internal/modules/home/service"
	homehttp "github.com/bookhaven/backend/internal/modules/home/transport/http"
	listsservice "github.com/bookhaven/backend/internal/modules/lists/service"
	listshttp "github.com/bookhaven/backend/internal/modules/lists/transport/http"
	notificationsservice "github.com/bookhaven/backend/internal/modules/notifications/service"
	notificationshttp "github.com/bookhaven/backend/internal/modules/notifications/transport/http"
	ratingsservice "github.com/bookhaven/backend/internal/modules/ratings/service"
	ratingshttp "github.com/bookhaven/backend/internal/modules/ratings/transport/http"
	readingchallengesservice "github.com/bookhaven/backend/internal/modules/reading_challenges/service"
	readingchallengeshttp "github.com/bookhaven/backend/internal/modules/reading_challenges/transport/http"
	reviewsservice "github.com/bookhaven/backend/internal/modules/reviews/service"
	reviewshttp "github.com/bookhaven/backend/internal/modules/reviews/transport/http"
	shelvesservice "github.com/bookhaven/backend/internal/modules/shelves/service"
	shelveshttp "github.com/bookhaven/backend/internal/modules/shelves/transport/http"
	usersdomain "github.com/bookhaven/backend/internal/modules/users/domain"
	usersservice "github.com/bookhaven/backend/internal/modules/users/service"
	usershttp "github.com/bookhaven/backend/internal/modules/users/transport/http"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

// RegisterRoutes registers all public API routes.
func RegisterRoutes(
	e *echo.Echo,
	catalogService *catalogservice.Service,
	shelvesService *shelvesservice.Service,
	listsService *listsservice.Service,
	ratingsService *ratingsservice.Service,
	reviewsService *reviewsservice.Service,
	usersService *usersservice.Service,
	readingChallengesService *readingchallengesservice.Service,
	notificationsService *notificationsservice.Service,
	homeService *homeservice.Service,
	authMiddleware echo.MiddlewareFunc,
	optionalAuth echo.MiddlewareFunc,
) {
	api := e.Group("/api/v1")

	catalogHandler := cataloghttp.NewHandler(catalogService)
	api.GET("/books", catalogHandler.ListBooks)
	api.GET("/books/:slug", catalogHandler.GetBook)
	api.GET("/books/:id/similar", catalogHandler.ListSimilarBooks)
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
	api.GET("/shelves/public/:id", shelvesHandler.GetPublicShelf, optionalAuth)

	listsHandler := listshttp.NewHandler(listsService)
	lists := api.Group("/lists", authMiddleware)
	lists.GET("", listsHandler.ListLists)
	lists.POST("", listsHandler.CreateList)
	lists.GET("/:id", listsHandler.GetList)
	lists.PATCH("/:id", listsHandler.UpdateList)
	lists.DELETE("/:id", listsHandler.DeleteList)
	lists.POST("/:id/books", listsHandler.AddBook)
	lists.DELETE("/:id/books/:bookId", listsHandler.RemoveBook)
	lists.PATCH("/:id/books/reorder", listsHandler.ReorderBooks)
	api.GET("/lists/public/:id", listsHandler.GetPublicList, optionalAuth)

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

	usersHandler := usershttp.NewHandler(usersService, listsService, shelvesService)
	api.GET("/users/:id", usersHandler.GetUser, optionalAuth)
	api.PATCH("/users/me", usersHandler.UpdateMe, authMiddleware)

	readingChallengesHandler := readingchallengeshttp.NewHandler(readingChallengesService)
	readingChallenges := api.Group("/reading-challenges", authMiddleware)
	readingChallenges.GET("", readingChallengesHandler.ListChallenges)
	readingChallenges.GET("/:year", readingChallengesHandler.GetChallenge)
	readingChallenges.PUT("/:year", readingChallengesHandler.UpsertGoal)

	notificationsHandler := notificationshttp.NewHandler(notificationsService)
	notifications := api.Group("/notifications", authMiddleware)
	notifications.GET("", notificationsHandler.ListNotifications)
	notifications.GET("/unread-count", notificationsHandler.UnreadCount)
	notifications.PATCH("/:id/read", notificationsHandler.MarkRead)
	notifications.POST("/read-all", notificationsHandler.MarkAllRead)

	homeHandler := homehttp.NewHandler(homeService)
	api.GET("/home", homeHandler.GetHome)
	api.GET("/home/random", homeHandler.RandomBook)
	api.GET("/home/me", homeHandler.GetMyHome, authMiddleware)
}

// RegisterAdminRoutes registers admin-only API routes under /api/v1/admin.
// harvestEnqueuer/harvestInspector are the small interfaces the new
// harvest/transport/http.Handler depends on (satisfied by a real
// *asynq.Client/*asynq.Inspector in production, see cmd/api/main.go) - kept
// as separate parameters rather than folding harvest into adminService so
// the admin module itself stays free of an asynq dependency.
func RegisterAdminRoutes(
	e *echo.Echo,
	adminService *adminservice.Service,
	authMiddleware echo.MiddlewareFunc,
	harvestEnqueuer harvesthttp.TaskEnqueuer,
	harvestInspector harvesthttp.TaskInspector,
	harvestAuthorSearcher harvesthttp.AuthorSearcher,
) {
	admin := e.Group("/api/v1/admin", authMiddleware)

	adminHandler := adminhttp.NewHandler(adminService)
	// adminService doubles as the search's ExistingAuthorFinder ("already in
	// BookHaven" markers, docs/features/author-direct-import.md).
	harvestHandler := harvesthttp.NewHandler(harvestEnqueuer, harvestInspector, harvestAuthorSearcher, adminService)

	adminRole := middleware.RequireRole(usersdomain.RoleAdmin)
	adminOrModerator := middleware.RequireRole(usersdomain.RoleAdmin, usersdomain.RoleModerator)

	admin.GET("/dashboard", adminHandler.Dashboard, adminRole)

	requireBearer := middleware.RequireBearerAuth()

	books := admin.Group("/books", adminRole)
	books.GET("", adminHandler.ListBooks)
	books.POST("", adminHandler.CreateBook, requireBearer)
	books.GET("/lookup", adminHandler.LookupBook)
	books.POST("/import", adminHandler.ImportBooks, requireBearer, echomiddleware.BodyLimit(adminhttp.ImportMaxRequestBody))
	books.POST("/bulk-publish", adminHandler.BulkPublishBooks, requireBearer)
	books.GET("/:id", adminHandler.GetBook)
	books.PATCH("/:id", adminHandler.UpdateBook, requireBearer)

	authors := admin.Group("/authors", adminRole)
	authors.GET("", adminHandler.ListAuthors)
	authors.GET("/moderation", adminHandler.ListAuthorModerationQueue)
	authors.POST("/bulk-status", adminHandler.BulkSetAuthorStatus, requireBearer)
	authors.POST("/import-wikidata", adminHandler.ImportWikidataAuthor, requireBearer)
	authors.POST("/enrich-wikidata", adminHandler.EnrichAuthorsFromWikidata, requireBearer)
	authors.POST("", adminHandler.CreateAuthor, requireBearer)
	authors.PATCH("/:id", adminHandler.UpdateAuthor, requireBearer)
	authors.POST("/:id/merge", adminHandler.MergeAuthors, requireBearer)

	genres := admin.Group("/genres", adminRole)
	genres.GET("", adminHandler.ListGenres)
	genres.POST("", adminHandler.CreateGenre, requireBearer)
	genres.PATCH("/:id", adminHandler.UpdateGenre, requireBearer)

	reviews := admin.Group("/reviews", adminOrModerator)
	reviews.GET("", adminHandler.ListReviews)
	reviews.PATCH("/:id", adminHandler.ModerateReview, requireBearer)

	harvest := admin.Group("/harvest", adminRole)
	harvest.POST("", harvestHandler.TriggerHarvest, requireBearer)
	harvest.GET("/jobs", harvestHandler.ListHarvestJobs)
	harvest.GET("/jobs/:id", harvestHandler.GetHarvestJob)
	harvest.GET("/authors/search", harvestHandler.SearchAuthors)
}
