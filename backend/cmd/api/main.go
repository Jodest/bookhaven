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
	authservice "github.com/bookhaven/backend/internal/modules/auth/service"
	authhttp "github.com/bookhaven/backend/internal/modules/auth/transport/http"
	catalogrepository "github.com/bookhaven/backend/internal/modules/catalog/repository"
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	ratingsrepository "github.com/bookhaven/backend/internal/modules/ratings/repository"
	ratingsservice "github.com/bookhaven/backend/internal/modules/ratings/service"
	reviewsrepository "github.com/bookhaven/backend/internal/modules/reviews/repository"
	reviewsservice "github.com/bookhaven/backend/internal/modules/reviews/service"
	shelvesrepository "github.com/bookhaven/backend/internal/modules/shelves/repository"
	shelvesservice "github.com/bookhaven/backend/internal/modules/shelves/service"
	"github.com/bookhaven/backend/internal/modules/users/repository"
	httptransport "github.com/bookhaven/backend/internal/transport/http"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

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
	catalogRepo := catalogrepository.NewGormRepository(db)
	shelvesRepo := shelvesrepository.NewGormRepository(db)
	ratingsRepo := ratingsrepository.NewGormRepository(db)
	reviewsRepo := reviewsrepository.NewGormRepository(db)

	// Services
	catalogService := catalogservice.NewService(catalogRepo)
	shelvesService := shelvesservice.NewService(shelvesRepo)
	ratingsService := ratingsservice.NewService(ratingsRepo)
	reviewsService := reviewsservice.NewService(reviewsRepo)

	authService := authservice.NewService(authservice.Config{
		JWTSecret:       cfg.JWTSecret,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		Issuer:          "bookhaven",
	}, db, usersRepo)

	// Validator
	e := echo.New()
	e.HideBanner = true
	e.Validator = validator.New()

	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:3001"},
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
	httptransport.RegisterRoutes(e, catalogService, shelvesService, ratingsService, reviewsService, authMiddleware, optionalAuth)

	// Auth routes
	authHandler := authhttp.NewHandler(authService)
	api := e.Group("/api/v1")
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/refresh", authHandler.Refresh)
	api.GET("/auth/me", authHandler.Me, authMiddleware)
	api.POST("/auth/logout", authHandler.Logout, authMiddleware)

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

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
