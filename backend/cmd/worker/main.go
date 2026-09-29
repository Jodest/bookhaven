// Command worker runs the asynq server that processes queued background
// jobs, currently just the catalog harvest job (see
// internal/modules/harvest/service). It connects to the same Postgres
// database as cmd/api (via gorm) and to Redis (via cfg.RedisURL, the same
// config value cmd/api already loads but does not yet use).
//
// The harvest queue is configured with concurrency 1: see the package doc
// comment on internal/modules/harvest/service for why (bookdata's
// Wikidata/OpenLibrary clients throttle per-instance; running more than one
// harvest task at a time would multiply the effective outbound request
// rate).
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/bookhaven/backend/internal/config"
	adminrepository "github.com/bookhaven/backend/internal/modules/admin/repository"
	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/hibiken/asynq"
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

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}

	adminRepo := adminrepository.NewGormRepository(db)
	adminService := adminservice.NewService(adminRepo)

	wikidataClient := bookdata.NewWikidataClient()
	// WithWorkDescriptionLookup is opted in here (harvest enrichment) only -
	// backend/cmd/api/main.go's admin lookup-assistant client deliberately
	// does not, to preserve its "exactly one outbound Open Library call per
	// request" contract. See docs/features/openlibrary-works-description.md.
	openLibraryClient := bookdata.NewOpenLibraryClient(bookdata.WithWorkDescriptionLookup())

	harvestHandler := harvestservice.NewHandler(wikidataClient, openLibraryClient, adminService)

	mux := asynq.NewServeMux()
	mux.HandleFunc(harvestservice.TaskTypeHarvestWikidataQuery, harvestHandler.HandleTask)

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisURL},
		asynq.Config{
			Queues: map[string]int{
				harvestservice.QueueName: 1,
			},
			Logger: zapAsynqLogger{logger},
		},
	)

	if err := srv.Start(mux); err != nil {
		logger.Fatal("failed to start asynq server", zap.Error(err))
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down worker")
	srv.Shutdown()
}

// zapAsynqLogger adapts *zap.Logger to asynq's minimal logging interface.
type zapAsynqLogger struct {
	l *zap.Logger
}

func (z zapAsynqLogger) Debug(args ...interface{}) { z.l.Sugar().Debug(args...) }
func (z zapAsynqLogger) Info(args ...interface{})  { z.l.Sugar().Info(args...) }
func (z zapAsynqLogger) Warn(args ...interface{})  { z.l.Sugar().Warn(args...) }
func (z zapAsynqLogger) Error(args ...interface{}) { z.l.Sugar().Error(args...) }
func (z zapAsynqLogger) Fatal(args ...interface{}) { z.l.Sugar().Fatal(args...) }
