// Command worker runs the asynq servers that process queued background
// jobs: the catalog harvest job (see internal/modules/harvest/service),
// transactional email delivery (see internal/pkg/mailer) and the daily
// similar-books recompute (see internal/modules/catalog/service,
// docs/features/similar-books-stage2.md). Email and similarity each run on
// their own asynq.Server so neither is stuck behind a long-running harvest
// task.
//
// The worker also runs an asynq.Scheduler that enqueues the similarity
// recompute daily at 03:30 UTC. Prod runs exactly one worker; do not scale
// it beyond one replica without moving the scheduler (asynq.Unique on the
// task is only a defense in depth against duplicate enqueues). There is no
// recompute on startup (air restarts the dev worker on every save); run
// cmd/similarity by hand when needed. It connects to the same Postgres
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
	"time"

	"github.com/bookhaven/backend/internal/config"
	adminrepository "github.com/bookhaven/backend/internal/modules/admin/repository"
	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogrepository "github.com/bookhaven/backend/internal/modules/catalog/repository"
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/pkg/mailer"
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
	// One WikidataClient instance for the harvest query and the author
	// enrichment (Wikidata + Commons), so both share one rate limiter
	// (docs/features/author-wikidata-enrichment.md rule 8).
	wikidataClient := bookdata.NewWikidataClient()
	adminService := adminservice.NewService(adminRepo,
		adminservice.WithAuthorEnrichmentSource(wikidataClient),
		adminservice.WithLogger(logger))
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

	emailHandler := mailer.NewHandler(mailer.NewSender(cfg, logger))

	emailMux := asynq.NewServeMux()
	emailMux.HandleFunc(mailer.TaskTypeSendEmail, emailHandler.HandleTask)

	emailSrv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisURL},
		asynq.Config{
			Queues: map[string]int{
				mailer.QueueName: 1,
			},
			Concurrency: 4,
			Logger:      zapAsynqLogger{logger},
		},
	)

	if err := emailSrv.Start(emailMux); err != nil {
		srv.Shutdown()
		logger.Fatal("failed to start email asynq server", zap.Error(err))
	}

	catalogRepo := catalogrepository.NewGormSimilarityRepository(db)
	similarityJob := catalogservice.NewSimilarityJob(catalogRepo, catalogRepo, logger)

	similarityMux := asynq.NewServeMux()
	similarityMux.HandleFunc(catalogservice.TaskTypeRecomputeSimilarities, similarityJob.HandleTask)

	similaritySrv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisURL},
		asynq.Config{
			Queues: map[string]int{
				catalogservice.SimilarityQueue: 1,
			},
			Concurrency: 1,
			Logger:      zapAsynqLogger{logger},
		},
	)

	if err := similaritySrv.Start(similarityMux); err != nil {
		emailSrv.Shutdown()
		srv.Shutdown()
		logger.Fatal("failed to start similarity asynq server", zap.Error(err))
	}

	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: cfg.RedisURL},
		&asynq.SchedulerOpts{
			Location: time.UTC,
			Logger:   zapAsynqLogger{logger},
		},
	)
	if err := registerSchedules(scheduler); err != nil {
		similaritySrv.Shutdown()
		emailSrv.Shutdown()
		srv.Shutdown()
		logger.Fatal("failed to register schedules", zap.Error(err))
	}
	if err := scheduler.Start(); err != nil {
		similaritySrv.Shutdown()
		emailSrv.Shutdown()
		srv.Shutdown()
		logger.Fatal("failed to start asynq scheduler", zap.Error(err))
	}
	logger.Info("asynq scheduler started",
		zap.String("task", catalogservice.TaskTypeRecomputeSimilarities),
		zap.String("cron", similarityCronSpec),
		zap.String("location", "UTC"))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down worker")
	scheduler.Shutdown()
	similaritySrv.Shutdown()
	emailSrv.Shutdown()
	srv.Shutdown()
}

// similarityCronSpec runs the similar-books recompute daily at 03:30 (the
// scheduler's Location is UTC).
const similarityCronSpec = "30 3 * * *"

// scheduleRegistrar is the subset of *asynq.Scheduler used by
// registerSchedules (a seam for tests).
type scheduleRegistrar interface {
	Register(cronspec string, task *asynq.Task, opts ...asynq.Option) (string, error)
}

// registerSchedules registers all periodic tasks.
func registerSchedules(s scheduleRegistrar) error {
	_, err := s.Register(similarityCronSpec,
		catalogservice.NewRecomputeSimilaritiesTask(),
		catalogservice.RecomputeTaskOptions()...)
	return err
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
