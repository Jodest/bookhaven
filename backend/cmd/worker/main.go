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
// The harvest server runs with Concurrency 1 (harvestServerConfig; asynq
// defaults to NumCPU otherwise, #191): one harvest job at a time on any
// host, manual and scheduled alike. See the package doc comment on
// internal/modules/harvest/service for why (bookdata's Wikidata/OpenLibrary
// clients throttle per-instance; running more than one harvest task at a
// time would multiply the effective outbound request rate).
//
// Scheduled harvest (HARVEST_SCHEDULE_ENABLED, off by default): the
// scheduler enqueues a lightweight harvest:schedule_tick task on
// HARVEST_SCHEDULE_CRON (UTC). It runs on its own small server (queue
// harvest_schedule, Concurrency 1), so ticks never appear in the admin jobs
// list and never wait behind a harvest. The tick picks up to
// HARVEST_SCHEDULE_BATCH flagged authors and queues one ordinary harvest job
// for them (everything it creates is pending). Like the similarity
// schedule, this assumes a single worker replica; the tick's asynq.Unique
// and its "scheduled job already queued" check are a defense in depth only.
// When disabled, neither the tick schedule nor its server is set up.
package main

import (
	"fmt"
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

	redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisURL}
	srv := asynq.NewServer(redisOpt, harvestServerConfig(zapAsynqLogger{logger}))

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

	// Scheduled harvest: the tick enqueues ordinary harvest jobs through
	// an asynq Client and checks the harvest queue through an Inspector.
	var (
		harvestClient    *asynq.Client
		harvestInspector *asynq.Inspector
		scheduleSrv      *asynq.Server
	)
	shutdownServers := func() {
		if scheduleSrv != nil {
			scheduleSrv.Shutdown()
		}
		similaritySrv.Shutdown()
		emailSrv.Shutdown()
		srv.Shutdown()
		if harvestInspector != nil {
			_ = harvestInspector.Close()
		}
		if harvestClient != nil {
			_ = harvestClient.Close()
		}
	}

	if cfg.HarvestScheduleEnabled {
		harvestClient = asynq.NewClient(redisOpt)
		harvestInspector = asynq.NewInspector(redisOpt)
		tick := harvestservice.NewScheduleTick(harvestClient, harvestInspector, adminService, cfg.HarvestScheduleBatch, logger)

		scheduleMux := asynq.NewServeMux()
		scheduleMux.HandleFunc(harvestservice.TaskTypeHarvestScheduleTick, tick.HandleTask)

		scheduleSrv = asynq.NewServer(redisOpt, scheduleServerConfig(zapAsynqLogger{logger}))
		if err := scheduleSrv.Start(scheduleMux); err != nil {
			scheduleSrv = nil
			shutdownServers()
			logger.Fatal("failed to start harvest schedule asynq server", zap.Error(err))
		}
	} else {
		logger.Info("scheduled harvest disabled")
	}

	scheduler := asynq.NewScheduler(
		redisOpt,
		&asynq.SchedulerOpts{
			Location: time.UTC,
			Logger:   zapAsynqLogger{logger},
		},
	)
	if err := registerSchedules(scheduler, cfg); err != nil {
		shutdownServers()
		logger.Fatal("failed to register schedules", zap.Error(err))
	}
	if err := scheduler.Start(); err != nil {
		shutdownServers()
		logger.Fatal("failed to start asynq scheduler", zap.Error(err))
	}
	logger.Info("asynq scheduler started",
		zap.String("task", catalogservice.TaskTypeRecomputeSimilarities),
		zap.String("cron", similarityCronSpec),
		zap.String("location", "UTC"))
	if cfg.HarvestScheduleEnabled {
		logger.Info("scheduled harvest enabled",
			zap.String("task", harvestservice.TaskTypeHarvestScheduleTick),
			zap.String("cron", cfg.HarvestScheduleCron),
			zap.Int("batch", cfg.HarvestScheduleBatch),
			zap.String("location", "UTC"))
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down worker")
	scheduler.Shutdown()
	shutdownServers()
}

// harvestServerConfig is the harvest server's config: the harvest queue
// only, and Concurrency 1 so exactly one harvest job runs at a time on any
// host (asynq's default is NumCPU, #191).
func harvestServerConfig(logger asynq.Logger) asynq.Config {
	return asynq.Config{
		Queues: map[string]int{
			harvestservice.QueueName: 1,
		},
		Concurrency: 1,
		Logger:      logger,
	}
}

// scheduleServerConfig is the config of the small server that runs the
// scheduled-harvest tick (its own queue, one tick at a time).
func scheduleServerConfig(logger asynq.Logger) asynq.Config {
	return asynq.Config{
		Queues: map[string]int{
			harvestservice.ScheduleQueueName: 1,
		},
		Concurrency: 1,
		Logger:      logger,
	}
}

// similarityCronSpec runs the similar-books recompute daily at 03:30 (the
// scheduler's Location is UTC).
const similarityCronSpec = "30 3 * * *"

// scheduleRegistrar is the subset of *asynq.Scheduler used by
// registerSchedules (a seam for tests).
type scheduleRegistrar interface {
	Register(cronspec string, task *asynq.Task, opts ...asynq.Option) (string, error)
}

// registerSchedules registers all periodic tasks: the similarity recompute
// always, the scheduled-harvest tick only when cfg enables it (on
// cfg.HarvestScheduleCron; an invalid spec fails here).
func registerSchedules(s scheduleRegistrar, cfg *config.Config) error {
	if _, err := s.Register(similarityCronSpec,
		catalogservice.NewRecomputeSimilaritiesTask(),
		catalogservice.RecomputeTaskOptions()...); err != nil {
		return err
	}
	if cfg == nil || !cfg.HarvestScheduleEnabled {
		return nil
	}
	if _, err := s.Register(cfg.HarvestScheduleCron,
		harvestservice.NewScheduleTickTask(),
		harvestservice.ScheduleTickOptions()...); err != nil {
		return fmt.Errorf("register scheduled harvest (%q): %w", cfg.HarvestScheduleCron, err)
	}
	return nil
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
