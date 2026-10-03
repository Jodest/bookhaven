package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bookhaven/backend/internal/config"
	"github.com/bookhaven/backend/internal/infrastructure/validator"
	adminrepository "github.com/bookhaven/backend/internal/modules/admin/repository"
	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	authservice "github.com/bookhaven/backend/internal/modules/auth/service"
	authhttp "github.com/bookhaven/backend/internal/modules/auth/transport/http"
	catalogrepository "github.com/bookhaven/backend/internal/modules/catalog/repository"
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	listsrepository "github.com/bookhaven/backend/internal/modules/lists/repository"
	listsservice "github.com/bookhaven/backend/internal/modules/lists/service"
	notificationsrepository "github.com/bookhaven/backend/internal/modules/notifications/repository"
	notificationsservice "github.com/bookhaven/backend/internal/modules/notifications/service"
	ratingsrepository "github.com/bookhaven/backend/internal/modules/ratings/repository"
	ratingsservice "github.com/bookhaven/backend/internal/modules/ratings/service"
	readingchallengesrepository "github.com/bookhaven/backend/internal/modules/reading_challenges/repository"
	readingchallengesservice "github.com/bookhaven/backend/internal/modules/reading_challenges/service"
	reviewsrepository "github.com/bookhaven/backend/internal/modules/reviews/repository"
	reviewsservice "github.com/bookhaven/backend/internal/modules/reviews/service"
	shelvesrepository "github.com/bookhaven/backend/internal/modules/shelves/repository"
	shelvesservice "github.com/bookhaven/backend/internal/modules/shelves/service"
	"github.com/bookhaven/backend/internal/modules/users/repository"
	usersservice "github.com/bookhaven/backend/internal/modules/users/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/pkg/mailer"
	"github.com/bookhaven/backend/internal/pkg/ratelimit"
	httptransport "github.com/bookhaven/backend/internal/transport/http"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "github.com/bookhaven/backend/docs"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// @title BookHaven API
// @version 1.0
// @description Социальный каталог книг для рынка РФ и СНГ. REST API для веб- и админ-приложений BookHaven.
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите access-токен в формате: Bearer {token}
func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// Database connection
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}

	// Repositories
	usersRepo := repository.NewGormRepository(db)
	usersService := usersservice.NewService(usersRepo)

	catalogRepo := catalogrepository.NewGormRepository(db)
	adminRepo := adminrepository.NewGormRepository(db)
	shelvesRepo := shelvesrepository.NewGormRepository(db)
	listsRepo := listsrepository.NewGormRepository(db)
	ratingsRepo := ratingsrepository.NewGormRepository(db)
	reviewsRepo := reviewsrepository.NewGormRepository(db)
	readingChallengesRepo := readingchallengesrepository.NewGormRepository(db)
	notificationsRepo := notificationsrepository.NewGormRepository(db)

	// Services
	catalogService := catalogservice.NewService(catalogRepo)
	// bookdata.NewOpenLibraryClient() is constructed with no options, mirroring
	// backend/cmd/worker/main.go's own zero-config construction (see
	// docs/features/bookdata-clients.md for the accepted-risk placeholder
	// User-Agent contact).
	adminService := adminservice.NewService(adminRepo, adminservice.WithLookupClient(bookdata.NewOpenLibraryClient()))
	// wikidataClient is a second, independent bookdata client instance (same
	// zero-config, placeholder-User-Agent caveat as above) used only for the
	// admin harvest author-name-search endpoint (see
	// docs/features/harvest-author-search.md) - it does not share the
	// OpenLibraryClient's rate-limit budget, nor does it touch the same host.
	wikidataClient := bookdata.NewWikidataClient()
	shelvesService := shelvesservice.NewService(shelvesRepo)
	listsService := listsservice.NewService(listsRepo)
	ratingsService := ratingsservice.NewService(ratingsRepo)
	reviewsService := reviewsservice.NewService(reviewsRepo)
	readingChallengesService := readingchallengesservice.NewService(readingChallengesRepo)
	notificationsService := notificationsservice.NewService(notificationsRepo)

	// Shared asynq.Client for every task the api enqueues: admin
	// harvest-trigger jobs (see docs/features/admin-harvest-trigger.md) and
	// transactional emails (internal/pkg/mailer). The Inspector serves the
	// admin harvest status endpoints.
	taskClient := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisURL})
	defer taskClient.Close()
	harvestInspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: cfg.RedisURL})
	defer harvestInspector.Close()

	mailQueue := mailer.NewQueue(taskClient)

	// Plain Redis client for rate limiting (see
	// docs/features/email-verification-backend.md).
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
	defer redisClient.Close()
	// Public resend-verification limits (see
	// docs/features/email-verification-login-gate.md): per client IP, then
	// per sha256 of the normalised email.
	verifyResendIPLimiter := ratelimit.NewRedisLimiter(redisClient, ratelimit.Options{
		Prefix:      "rl:verify-resend-ip",
		MinInterval: 0,
		WindowLimit: 20,
		Window:      time.Hour,
	})
	verifyResendEmailLimiter := ratelimit.NewRedisLimiter(redisClient, ratelimit.Options{
		Prefix:      "rl:verify-resend-email",
		MinInterval: time.Minute,
		WindowLimit: 5,
		Window:      24 * time.Hour,
	})
	// Public forgot-password limits (see docs/features/password-reset.md):
	// same shape as resend-verification, separate prefixes so the two flows
	// do not share quota.
	pwResetIPLimiter := ratelimit.NewRedisLimiter(redisClient, ratelimit.Options{
		Prefix:      "rl:pwreset-ip",
		MinInterval: 0,
		WindowLimit: 20,
		Window:      time.Hour,
	})
	pwResetEmailLimiter := ratelimit.NewRedisLimiter(redisClient, ratelimit.Options{
		Prefix:      "rl:pwreset-email",
		MinInterval: time.Minute,
		WindowLimit: 5,
		Window:      24 * time.Hour,
	})

	authService := authservice.NewService(authservice.Config{
		JWTSecret:            cfg.JWTSecret,
		AccessTokenTTL:       15 * time.Minute,
		RefreshTokenTTL:      30 * 24 * time.Hour,
		Issuer:               "bookhaven",
		WebBaseURL:           cfg.WebBaseURL,
		VerificationTokenTTL: 24 * time.Hour,
	}, db, usersRepo, mailQueue, authservice.ResendLimiters{
		IP:    verifyResendIPLimiter,
		Email: verifyResendEmailLimiter,
	}, authservice.ResendLimiters{
		IP:    pwResetIPLimiter,
		Email: pwResetEmailLimiter,
	}, logger)

	// Validator
	e := echo.New()
	e.HideBanner = true
	e.Validator = validator.New()
	// c.RealIP() = client IP from X-Forwarded-For, honouring only trusted
	// proxy hops (default trust: loopback, link-local, private networks).
	// In prod Caddy reaches the api over the private Docker network and does
	// not trust client-supplied X-Forwarded-For, so the header carries the
	// real client IP; direct callers cannot spoof it. Used by the per-IP
	// resend-verification and forgot-password rate limits - see
	// docs/DEPLOYMENT.md before putting another proxy in front.
	e.IPExtractor = echo.ExtractIPFromXFFHeader()

	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
	}))
	e.Use(echomiddleware.RequestLoggerWithConfig(echomiddleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c echo.Context, v echomiddleware.RequestLoggerValues) error {
			logger.Info("request",
				zap.String("method", v.Method),
				zap.String("uri", v.URI),
				zap.Int("status", v.Status),
			)
			return nil
		},
	}))

	// Auth middleware
	authMiddleware := middleware.Auth(authService, usersRepo)
	optionalAuth := middleware.OptionalAuth(authService, usersRepo)

	// Public + protected routes
	httptransport.RegisterRoutes(e, catalogService, shelvesService, listsService, ratingsService, reviewsService, usersService, readingChallengesService, notificationsService, authMiddleware, optionalAuth)

	// Admin routes
	httptransport.RegisterAdminRoutes(e, adminService, authMiddleware, taskClient, harvestInspector, wikidataClient)

	// Auth routes
	authHandler := authhttp.NewHandler(authService)
	api := e.Group("/api/v1")
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/refresh", authHandler.Refresh)
	api.GET("/auth/me", authHandler.Me, authMiddleware)
	api.POST("/auth/logout", authHandler.Logout, authMiddleware)
	api.POST("/auth/verify-email", authHandler.VerifyEmail)
	api.POST("/auth/resend-verification", authHandler.ResendVerification)
	api.POST("/auth/forgot-password", authHandler.ForgotPassword)
	api.POST("/auth/reset-password", authHandler.ResetPassword)

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	e.GET("/swagger/*", echoSwagger.WrapHandler)

	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		logger.Fatal("failed to shutdown server", zap.Error(err))
	}
}
