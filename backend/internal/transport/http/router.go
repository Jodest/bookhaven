package http

import (
	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	adminhttp "github.com/bookhaven/backend/internal/modules/admin/transport/http"
	aicuratorhttp "github.com/bookhaven/backend/internal/modules/aicurator/transport/http"
	aicollectionshttp "github.com/bookhaven/backend/internal/modules/aicollections/transport/http"
	aidescriptionshttp "github.com/bookhaven/backend/internal/modules/aidescriptions/transport/http"
	aimodhttp "github.com/bookhaven/backend/internal/modules/aimoderation/transport/http"
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	cataloghttp "github.com/bookhaven/backend/internal/modules/catalog/transport/http"
	curatedservice "github.com/bookhaven/backend/internal/modules/curated/service"
	curatedhttp "github.com/bookhaven/backend/internal/modules/curated/transport/http"
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
	screenworksservice "github.com/bookhaven/backend/internal/modules/screenworks/service"
	screenworkshttp "github.com/bookhaven/backend/internal/modules/screenworks/transport/http"
	shelvesservice "github.com/bookhaven/backend/internal/modules/shelves/service"
	shelveshttp "github.com/bookhaven/backend/internal/modules/shelves/transport/http"
	usersdomain "github.com/bookhaven/backend/internal/modules/users/domain"
	usersservice "github.com/bookhaven/backend/internal/modules/users/service"
	usershttp "github.com/bookhaven/backend/internal/modules/users/transport/http"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

// RegisterRoutes registers all public API routes. aiDescriptions (may be
// nil) is the approved AI description reader behind the book detail's
// ai_description fallback.
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
	curatedService *curatedservice.Service,
	screenWorksService *screenworksservice.Service,
	authMiddleware echo.MiddlewareFunc,
	optionalAuth echo.MiddlewareFunc,
	aiDescriptions cataloghttp.AIDescriptionReader,
) {
	api := e.Group("/api/v1")

	var catalogOpts []cataloghttp.HandlerOption
	if aiDescriptions != nil {
		catalogOpts = append(catalogOpts, cataloghttp.WithAIDescriptionReader(aiDescriptions))
	}
	catalogHandler := cataloghttp.NewHandler(catalogService, catalogOpts...)
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

	// Curated collections (openspec collections/curated-collections): public.
	curatedHandler := curatedhttp.NewHandler(curatedService)
	api.GET("/collections", curatedHandler.ListPublic)
	api.GET("/collections/:slug", curatedHandler.GetPublic)
	api.GET("/books/:id/collections", curatedHandler.ListBookPlaces)

	// Screen adaptations of a book (openspec screen-works/book-adaptations): public.
	screenWorksHandler := screenworkshttp.NewHandler(screenWorksService)
	api.GET("/books/:id/adaptations", screenWorksHandler.ListBookAdaptations)
	// Films about an author (openspec screen-works/author-films): public.
	api.GET("/authors/:id/screen-works", screenWorksHandler.ListAuthorFilms)
}

// AIModerationDeps configures the AI moderation trigger/status endpoints:
// Active is config.AIModerationActive() (the switch the worker uses too),
// Model the configured model id. The API never calls the model.
type AIModerationDeps struct {
	Active bool
	Model  string
}

// AICuratorDeps configures the AI curator endpoints (/admin/ai-curator):
// Service is the aicurator service (it carries config.AICuratorActive()
// and the model id; the API never calls the model). Its task is queued on
// the shared asynq client.
type AICuratorDeps struct {
	Service aicuratorhttp.CurationService
}

// AIDescriptionsDeps configures the AI descriptions endpoints
// (/admin/ai-descriptions): Service is the aidescriptions service (it
// carries config.AIDescriptionsActive() and the model id; the API never
// calls the model). Its tasks are queued on the shared asynq client.
type AIDescriptionsDeps struct {
	Service aidescriptionshttp.DescriptionService
}

// AICollectionsDeps configures the AI collection draft endpoints
// (/admin/ai-collections): Service is the aicollections service (it carries
// config.AICollectionsActive() and the model id; the API never calls the
// model). Its task is queued on the shared asynq client.
type AICollectionsDeps struct {
	Service aicollectionshttp.DraftService
}

// RegisterAdminRoutes registers admin-only API routes under /api/v1/admin.
// harvestEnqueuer/harvestInspector are the small interfaces the new
// harvest/transport/http.Handler depends on (satisfied by a real
// *asynq.Client/*asynq.Inspector in production, see cmd/api/main.go) - kept
// as separate parameters rather than folding harvest into adminService so
// the admin module itself stays free of an asynq dependency. The AI
// moderation trigger reuses harvestEnqueuer (the shared asynq client).
func RegisterAdminRoutes(
	e *echo.Echo,
	adminService *adminservice.Service,
	authMiddleware echo.MiddlewareFunc,
	harvestEnqueuer harvesthttp.TaskEnqueuer,
	harvestInspector harvesthttp.TaskInspector,
	harvestAuthorSearcher harvesthttp.AuthorSearcher,
	aiModeration AIModerationDeps,
	aiCurator AICuratorDeps,
	aiDescriptions AIDescriptionsDeps,
	aiCollections AICollectionsDeps,
	curatedService *curatedservice.Service,
	screenWorksService *screenworksservice.Service,
) {
	admin := e.Group("/api/v1/admin", authMiddleware)

	adminHandler := adminhttp.NewHandler(adminService)
	// adminService doubles as the search's ExistingAuthorFinder ("already in
	// BookHaven" markers, docs/features/author-direct-import.md).
	harvestHandler := harvesthttp.NewHandler(harvestEnqueuer, harvestInspector, harvestAuthorSearcher, adminService)
	aiReviewHandler := aimodhttp.NewHandler(harvestEnqueuer, adminService, aiModeration.Active, aiModeration.Model)

	screenWorksHandler := screenworkshttp.NewHandler(screenWorksService)

	adminRole := middleware.RequireRole(usersdomain.RoleAdmin)
	adminOrModerator := middleware.RequireRole(usersdomain.RoleAdmin, usersdomain.RoleModerator)

	admin.GET("/dashboard", adminHandler.Dashboard, adminRole)

	requireBearer := middleware.RequireBearerAuth()

	books := admin.Group("/books", adminRole)
	books.GET("", adminHandler.ListBooks)
	books.POST("", adminHandler.CreateBook, requireBearer)
	books.GET("/lookup", adminHandler.LookupBook)
	books.POST("/import", adminHandler.ImportBooks, requireBearer, echomiddleware.BodyLimit(adminhttp.ImportMaxRequestBody))
	books.POST("/bulk-status", adminHandler.BulkSetBookStatus, requireBearer)
	books.GET("/moderation", adminHandler.ListBookModerationQueue)
	books.POST("/ai-review", aiReviewHandler.TriggerAIReview, requireBearer)
	books.GET("/ai-review/status", aiReviewHandler.AIReviewStatus)
	books.POST("/sync-adaptations", screenWorksHandler.SyncAdaptations, requireBearer)
	books.GET("/:id", adminHandler.GetBook)
	books.PATCH("/:id", adminHandler.UpdateBook, requireBearer)
	books.GET("/:id/adaptations", screenWorksHandler.AdminListBookAdaptations)
	books.POST("/:id/adaptations", screenWorksHandler.AdminAddAdaptation, requireBearer)
	books.PATCH("/:id/adaptations/:screen_work_id", screenWorksHandler.AdminSetAdaptationHidden, requireBearer)
	books.DELETE("/:id/adaptations/:screen_work_id", screenWorksHandler.AdminDeleteAdaptation, requireBearer)

	authors := admin.Group("/authors", adminRole)
	authors.GET("", adminHandler.ListAuthors)
	authors.GET("/moderation", adminHandler.ListAuthorModerationQueue)
	authors.POST("/bulk-status", adminHandler.BulkSetAuthorStatus, requireBearer)
	authors.POST("/import-wikidata", adminHandler.ImportWikidataAuthor, requireBearer)
	authors.POST("/enrich-wikidata", adminHandler.EnrichAuthorsFromWikidata, requireBearer)
	authors.POST("/sync-screen-works", screenWorksHandler.SyncAuthorFilms, requireBearer)
	authors.POST("", adminHandler.CreateAuthor, requireBearer)
	authors.GET("/:id", adminHandler.GetAuthor)
	authors.PATCH("/:id", adminHandler.UpdateAuthor, requireBearer)
	authors.POST("/:id/merge", adminHandler.MergeAuthors, requireBearer)
	authors.GET("/:id/screen-works", screenWorksHandler.AdminListAuthorFilms)
	authors.POST("/:id/screen-works", screenWorksHandler.AdminAddAuthorFilm, requireBearer)
	authors.PATCH("/:id/screen-works/:screen_work_id", screenWorksHandler.AdminUpdateAuthorFilm, requireBearer)
	authors.DELETE("/:id/screen-works/:screen_work_id", screenWorksHandler.AdminDeleteAuthorFilm, requireBearer)

	screenWorks := admin.Group("/screen-works", adminRole)
	screenWorks.GET("", screenWorksHandler.AdminListWorks)
	screenWorks.GET("/:id", screenWorksHandler.AdminGetWork)
	screenWorks.PATCH("/:id", screenWorksHandler.AdminUpdateWork, requireBearer)
	screenWorks.DELETE("/:id", screenWorksHandler.AdminDeleteWork, requireBearer)

	genres := admin.Group("/genres", adminRole)
	genres.GET("", adminHandler.ListGenres)
	genres.GET("/moderation", adminHandler.ListGenreModerationQueue)
	genres.GET("/wikidata-search", adminHandler.SearchWikidataGenres)
	genres.POST("/import-wikidata", adminHandler.ImportWikidataGenre, requireBearer)
	genres.POST("/bulk-status", adminHandler.BulkSetGenreStatus, requireBearer)
	genres.POST("/merge-batch", adminHandler.MergeGenresBatch, requireBearer)
	genres.POST("/merge-preview", adminHandler.PreviewGenreMerge, requireBearer)
	genres.GET("/merges", adminHandler.ListGenreMerges, requireBearer)
	genres.GET("/merges/:id/undo-preview", adminHandler.PreviewGenreMergeUndo, requireBearer)
	genres.POST("/merges/:id/undo", adminHandler.UndoGenreMerge, requireBearer)
	genres.POST("", adminHandler.CreateGenre, requireBearer)
	genres.GET("/:id", adminHandler.GetGenre)
	genres.PATCH("/:id", adminHandler.UpdateGenre, requireBearer)
	genres.POST("/:id/merge", adminHandler.MergeGenres, requireBearer)

	users := admin.Group("/users", adminRole)
	users.GET("", adminHandler.ListUsers)
	users.PATCH("/:id/role", adminHandler.ChangeUserRole, requireBearer)

	reviews := admin.Group("/reviews", adminOrModerator)
	reviews.GET("", adminHandler.ListReviews)
	reviews.PATCH("/:id", adminHandler.ModerateReview, requireBearer)

	harvest := admin.Group("/harvest", adminRole)
	harvest.POST("", harvestHandler.TriggerHarvest, requireBearer)
	harvest.GET("/jobs", harvestHandler.ListHarvestJobs)
	harvest.GET("/jobs/:id", harvestHandler.GetHarvestJob)
	harvest.GET("/authors/search", harvestHandler.SearchAuthors)

	curatedHandler := curatedhttp.NewHandler(curatedService)
	collections := admin.Group("/collections", adminRole)
	collections.GET("", curatedHandler.AdminList)
	collections.POST("", curatedHandler.AdminCreate, requireBearer)
	collections.GET("/:id", curatedHandler.AdminGet)
	collections.PATCH("/:id", curatedHandler.AdminUpdate, requireBearer)
	collections.DELETE("/:id", curatedHandler.AdminDelete, requireBearer)
	collections.POST("/:id/recompute", curatedHandler.AdminRecompute, requireBearer)
	collections.POST("/:id/books", curatedHandler.AdminAddBook, requireBearer)
	collections.POST("/:id/books/paste/preview", curatedHandler.AdminPastePreview, requireBearer)
	collections.POST("/:id/books/bulk", curatedHandler.AdminBulkAdd, requireBearer)
	collections.DELETE("/:id/books/:bookId", curatedHandler.AdminRemoveBook, requireBearer)
	collections.PUT("/:id/books/:bookId/position", curatedHandler.AdminMoveBook, requireBearer)

	if aiCurator.Service != nil {
		curatorHandler := aicuratorhttp.NewHandler(harvestEnqueuer, aiCurator.Service)
		curator := admin.Group("/ai-curator", adminRole)
		curator.GET("/status", curatorHandler.CurationStatus)
		curator.POST("/requests", curatorHandler.CreateCurationRequest, requireBearer)
		curator.GET("/requests", curatorHandler.ListCurationRequests)
		curator.GET("/requests/:id", curatorHandler.GetCurationRequest)
	}

	if aiDescriptions.Service != nil {
		descHandler := aidescriptionshttp.NewHandler(harvestEnqueuer, aiDescriptions.Service)
		desc := admin.Group("/ai-descriptions", adminRole)
		// Static routes before /:book_id.
		desc.GET("/status", descHandler.AIDescriptionsStatus)
		desc.POST("/generate", descHandler.GenerateAIDescriptions, requireBearer)
		desc.GET("", descHandler.ListAIDescriptions)
		desc.GET("/:book_id", descHandler.GetAIDescription)
		desc.POST("/:book_id/approve", descHandler.ApproveAIDescription, requireBearer)
		desc.POST("/:book_id/discard", descHandler.DiscardAIDescription, requireBearer)
	}

	if aiCollections.Service != nil {
		draftsHandler := aicollectionshttp.NewHandler(harvestEnqueuer, aiCollections.Service)
		drafts := admin.Group("/ai-collections", adminRole)
		// Static routes before /drafts/:id.
		drafts.GET("/status", draftsHandler.DraftsStatus)
		drafts.POST("/drafts", draftsHandler.CreateDraft, requireBearer)
		drafts.GET("/drafts", draftsHandler.ListDrafts)
		drafts.GET("/drafts/:id", draftsHandler.GetDraft)
	}
}
