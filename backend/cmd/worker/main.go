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
//
// Catalog-wide adaptation sync (screenworks:sync_catalog, see
// internal/modules/screenworks/service): an admin action queues it on the
// harvest queue, so it runs in the same single slot as harvest jobs. An
// optional weekly run (ADAPTATION_SYNC_SCHEDULE_ENABLED, off by default,
// ADAPTATION_SYNC_SCHEDULE_CRON, UTC) enqueues a screenworks:sync_catalog_tick
// on the same harvest_schedule server, which is started when either schedule
// is enabled and registers only the enabled ticks.
//
// Automatic curated collections (AUTO_COLLECTIONS_SCHEDULE_ENABLED, on by
// default; see internal/modules/curated): the scheduler enqueues a
// curated:recompute_auto task on AUTO_COLLECTIONS_SCHEDULE_CRON (UTC, 01:00
// by default) that recomputes the stored snapshot of every automatic
// collection from ratings and reading-status shelves. It runs on its own
// small server (queue curated, Concurrency 1) and makes no external calls.
// A failing collection keeps its old snapshot and does not stop the others.
// Like the other schedules this assumes a single worker replica. When
// disabled, neither the schedule nor its server is set up and automatic
// collections change only through admin actions.
//
// AI pre-moderation (AI_MODERATION_ENABLED and ANTHROPIC_API_KEY, off by
// default; see internal/modules/aimoderation): the scheduler enqueues an
// ai_moderation:sweep on AI_MODERATION_CRON (UTC, every 30 minutes by
// default) that gives up to AI_MODERATION_BATCH pending books an AI
// verdict; admins can also queue ai_moderation:review_books and a manual
// sweep from the API. Both run on their own server (queue ai_moderation,
// Concurrency 1), so model calls are serialized and paced. Verdicts only
// classify: nothing is published or edited. Like the other schedules this
// assumes a single worker replica (the sweep's asynq.Unique is a defense in
// depth only). When not active, neither the sweep schedule nor its server
// is set up and the model is never called.
//
// AI curator of author import lists (AI_CURATOR_ENABLED and the shared
// ANTHROPIC_API_KEY, off by default; see internal/modules/aicurator): admins
// queue ai_curator:curate from the API; it runs on its own server (queue
// ai_curator, Concurrency 1), makes one model call per request for author
// names and resolves each name through the same WikidataClient (and so the
// same rate limiter) as the harvest. Nothing is imported or published.
// When not active, its server is not set up and the model is never called.
//
// AI book descriptions (AI_DESCRIPTIONS_ENABLED and the shared
// ANTHROPIC_API_KEY, off by default; see internal/modules/aidescriptions):
// admins queue ai_descriptions:generate (selected books) or
// ai_descriptions:sweep (up to 200 published books without any description)
// from the API; there is no schedule. Both run on their own server (queue
// ai_descriptions, Concurrency 1), one paced model call per book. Results
// are drafts in book_ai_descriptions: no book field is written and nothing
// is published without an admin. When not active, its server is not set up
// and the model is never called.
//
// AI collection drafts (AI_COLLECTIONS_ENABLED and the shared
// ANTHROPIC_API_KEY, off by default; see internal/modules/aicollections):
// admins queue ai_collections:draft from the API; it runs on its own server
// (queue ai_collections, Concurrency 1) and asks the model for ranked books
// in chunks of 100. Nothing is added to a collection or the catalog: the
// match against the catalog is computed when an admin reads the draft. When
// not active, its server is not set up and the model is never called.
//
// AI genre suggestions (AI_GENRES_ENABLED and the shared ANTHROPIC_API_KEY,
// off by default; see internal/modules/aigenres): admins queue
// ai_genres:analyze from the API, and a weekly cron (AI_GENRES_CRON, empty =
// none) enqueues ai_genres:scheduled, which analyzes only when the genres
// changed since the last clean run. Both run on their own server (queue
// ai_genres, Concurrency 1), one paced model call per batch. The analysis
// only stores suggestions: no genre, book or collection is ever changed by
// the worker. When not active, its server and schedule are not set up and
// the model is never called.
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
	aicollrepository "github.com/bookhaven/backend/internal/modules/aicollections/repository"
	aicollservice "github.com/bookhaven/backend/internal/modules/aicollections/service"
	curatorrepository "github.com/bookhaven/backend/internal/modules/aicurator/repository"
	curatorservice "github.com/bookhaven/backend/internal/modules/aicurator/service"
	aidescrepository "github.com/bookhaven/backend/internal/modules/aidescriptions/repository"
	aidescservice "github.com/bookhaven/backend/internal/modules/aidescriptions/service"
	aigenresrepository "github.com/bookhaven/backend/internal/modules/aigenres/repository"
	aigenresservice "github.com/bookhaven/backend/internal/modules/aigenres/service"
	aimodservice "github.com/bookhaven/backend/internal/modules/aimoderation/service"
	catalogrepository "github.com/bookhaven/backend/internal/modules/catalog/repository"
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	coversrepository "github.com/bookhaven/backend/internal/modules/covers/repository"
	coversservice "github.com/bookhaven/backend/internal/modules/covers/service"
	curatedrepository "github.com/bookhaven/backend/internal/modules/curated/repository"
	curatedservice "github.com/bookhaven/backend/internal/modules/curated/service"
	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
	screenworksrepository "github.com/bookhaven/backend/internal/modules/screenworks/repository"
	screenworksservice "github.com/bookhaven/backend/internal/modules/screenworks/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/pkg/coverfetch"
	"github.com/bookhaven/backend/internal/pkg/coverstore"
	"github.com/bookhaven/backend/internal/pkg/database"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/bookhaven/backend/internal/pkg/mailer"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	db, err := database.Open(cfg.DatabaseURL, database.PoolConfigFrom(cfg))
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	logger.Info("database pool configured",
		zap.Int("max_open_conns", cfg.DBMaxOpenConns),
		zap.Int("max_idle_conns", cfg.DBMaxIdleConns),
		zap.Duration("conn_max_lifetime", cfg.DBConnMaxLifetime),
		zap.Duration("conn_max_idle_time", cfg.DBConnMaxIdleTime))

	adminRepo := adminrepository.NewGormRepository(db)
	// One WikidataClient instance for the harvest query and the author
	// enrichment (Wikidata + Commons), so both share one rate limiter
	// (docs/features/author-wikidata-enrichment.md rule 8).
	wikidataClient := bookdata.NewWikidataClient()
	// Harvested books that are created already published change the cached
	// similar-books lists, so the worker invalidates them like the api does.
	similarRedis := redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
	defer similarRedis.Close()
	adminOpts := []adminservice.Option{
		adminservice.WithSimilarInvalidator(catalogrepository.NewRedisSimilarCache(similarRedis)),
		adminservice.WithAuthorEnrichmentSource(wikidataClient),
		adminservice.WithLogger(logger),
	}
	// Self-hosted covers (#489): a book created by a harvest (with a cover
	// link, also one filled in from Open Library) queues covers:mirror.
	var coversClient *asynq.Client
	if cfg.CoversMirrorEnabled {
		coversClient = asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisURL})
		adminOpts = append(adminOpts, adminservice.WithCoverEnqueuer(coversservice.NewEnqueuer(coversClient)))
	}
	adminService := adminservice.NewService(adminRepo, adminOpts...)
	// WithWorkDescriptionLookup is opted in here (harvest enrichment) only -
	// backend/cmd/api/main.go's admin lookup-assistant client deliberately
	// does not, to preserve its "exactly one outbound Open Library call per
	// request" contract. See docs/features/openlibrary-works-description.md.
	openLibraryClient := bookdata.NewOpenLibraryClient(bookdata.WithWorkDescriptionLookup())

	// Adaptation sync of the books a harvest created shares the same
	// WikidataClient (one rate limiter).
	screenWorksService := screenworksservice.NewService(screenworksrepository.NewGormRepository(db), wikidataClient, logger)
	harvestHandler := harvestservice.NewHandler(wikidataClient, openLibraryClient, adminService,
		harvestservice.WithAdaptationSyncer(screenWorksService),
		harvestservice.WithAuthorFilmSyncer(screenWorksService))

	mux := asynq.NewServeMux()
	mux.HandleFunc(harvestservice.TaskTypeHarvestWikidataQuery, harvestHandler.HandleTask)
	mux.HandleFunc(screenworksservice.TaskTypeCatalogSync, screenworksservice.NewCatalogSyncJob(screenWorksService, logger).HandleTask)

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
		curatedSrv       *asynq.Server
		aiSrv            *asynq.Server
		curatorSrv       *asynq.Server
		descSrv          *asynq.Server
		collSrv          *asynq.Server
		genresSrv        *asynq.Server
		coversSrv        *asynq.Server
	)
	shutdownServers := func() {
		if coversSrv != nil {
			coversSrv.Shutdown()
		}
		if coversClient != nil {
			_ = coversClient.Close()
		}
		if genresSrv != nil {
			genresSrv.Shutdown()
		}
		if collSrv != nil {
			collSrv.Shutdown()
		}
		if descSrv != nil {
			descSrv.Shutdown()
		}
		if curatorSrv != nil {
			curatorSrv.Shutdown()
		}
		if curatedSrv != nil {
			curatedSrv.Shutdown()
		}
		if aiSrv != nil {
			aiSrv.Shutdown()
		}
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

	if cfg.HarvestScheduleEnabled || cfg.AdaptationSyncScheduleEnabled {
		harvestClient = asynq.NewClient(redisOpt)
		harvestInspector = asynq.NewInspector(redisOpt)

		scheduleMux := asynq.NewServeMux()
		if cfg.HarvestScheduleEnabled {
			tick := harvestservice.NewScheduleTick(harvestClient, harvestInspector, adminService, cfg.HarvestScheduleBatch, logger)
			scheduleMux.HandleFunc(harvestservice.TaskTypeHarvestScheduleTick, tick.HandleTask)
		}
		if cfg.AdaptationSyncScheduleEnabled {
			syncTick := screenworksservice.NewCatalogSyncTick(harvestClient, harvestInspector, harvestservice.QueueName, logger)
			scheduleMux.HandleFunc(screenworksservice.TaskTypeCatalogSyncTick, syncTick.HandleTask)
		}

		scheduleSrv = asynq.NewServer(redisOpt, scheduleServerConfig(zapAsynqLogger{logger}))
		if err := scheduleSrv.Start(scheduleMux); err != nil {
			scheduleSrv = nil
			shutdownServers()
			logger.Fatal("failed to start harvest schedule asynq server", zap.Error(err))
		}
	}
	if !cfg.HarvestScheduleEnabled {
		logger.Info("scheduled harvest disabled")
	}
	if !cfg.AdaptationSyncScheduleEnabled {
		logger.Info("scheduled adaptation sync disabled")
	}

	// Automatic collections: the nightly snapshot recompute.
	if cfg.AutoCollectionsScheduleEnabled {
		recompute := curatedservice.NewRecomputeAutoJob(
			curatedservice.NewService(curatedrepository.NewGormRepository(db)).WithLogger(logger))

		curatedMux := asynq.NewServeMux()
		curatedMux.HandleFunc(curatedservice.TaskTypeRecomputeAuto, recompute.HandleTask)

		curatedSrv = asynq.NewServer(redisOpt, curatedServerConfig(zapAsynqLogger{logger}))
		if err := curatedSrv.Start(curatedMux); err != nil {
			curatedSrv = nil
			shutdownServers()
			logger.Fatal("failed to start curated collections asynq server", zap.Error(err))
		}
	} else {
		logger.Info("scheduled auto collections recompute disabled")
	}

	// AI pre-moderation: only when switched on and a key is set.
	if cfg.AIModerationActive() {
		client := llm.NewAnthropic(cfg.AnthropicAPIKey, cfg.AIModerationModel)
		reviewer := aimodservice.NewReviewer(client, adminService, cfg.AIModerationModel, logger)
		aiHandlers := aimodservice.NewHandlers(reviewer, adminService, logger)

		aiMux := asynq.NewServeMux()
		aiMux.HandleFunc(aimodservice.TaskTypeSweep, aiHandlers.HandleSweep)
		aiMux.HandleFunc(aimodservice.TaskTypeReviewBooks, aiHandlers.HandleReviewBooks)

		aiSrv = asynq.NewServer(redisOpt, aiModerationServerConfig(zapAsynqLogger{logger}))
		if err := aiSrv.Start(aiMux); err != nil {
			aiSrv = nil
			shutdownServers()
			logger.Fatal("failed to start AI moderation asynq server", zap.Error(err))
		}
	} else {
		if cfg.AIModerationEnabled {
			logger.Warn("AI_MODERATION_ENABLED is set but ANTHROPIC_API_KEY is empty; AI moderation disabled")
		}
		logger.Info("AI moderation disabled")
	}

	// AI curator: only when switched on and a key is set. The resolver
	// shares the harvest's wikidataClient so all Wikidata calls go through
	// one rate limiter.
	if cfg.AICuratorActive() {
		client := llm.NewAnthropic(cfg.AnthropicAPIKey, cfg.AICuratorModel)
		resolver := curatorservice.NewResolver(wikidataClient, wikidataClient, adminservice.IsImportableAuthorRecord)
		curatorHandlers := curatorservice.NewHandlers(curatorrepository.NewGormRepository(db), client, cfg.AICuratorModel, resolver, logger)

		curatorMux := asynq.NewServeMux()
		curatorMux.HandleFunc(curatorservice.TaskTypeCurate, curatorHandlers.HandleCurate)

		curatorSrv = asynq.NewServer(redisOpt, aiCuratorServerConfig(zapAsynqLogger{logger}))
		if err := curatorSrv.Start(curatorMux); err != nil {
			curatorSrv = nil
			shutdownServers()
			logger.Fatal("failed to start AI curator asynq server", zap.Error(err))
		}
		// Never log the API key.
		logger.Info("AI curation enabled",
			zap.String("task", curatorservice.TaskTypeCurate),
			zap.String("model", cfg.AICuratorModel))
	} else {
		if cfg.AICuratorEnabled {
			logger.Warn("AI_CURATOR_ENABLED is set but ANTHROPIC_API_KEY is empty; AI curation disabled")
		}
		logger.Info("AI curation disabled")
	}

	// AI descriptions: only when switched on and a key is set.
	if cfg.AIDescriptionsActive() {
		client := llm.NewAnthropic(cfg.AnthropicAPIKey, cfg.AIDescriptionsModel)
		descHandlers := aidescservice.NewHandlers(aidescrepository.NewGormRepository(db), adminService, client, cfg.AIDescriptionsModel, logger)

		descMux := asynq.NewServeMux()
		descMux.HandleFunc(aidescservice.TaskTypeGenerate, descHandlers.HandleGenerate)
		descMux.HandleFunc(aidescservice.TaskTypeSweep, descHandlers.HandleSweep)

		descSrv = asynq.NewServer(redisOpt, aiDescriptionsServerConfig(zapAsynqLogger{logger}))
		if err := descSrv.Start(descMux); err != nil {
			descSrv = nil
			shutdownServers()
			logger.Fatal("failed to start AI descriptions asynq server", zap.Error(err))
		}
		// Never log the API key.
		logger.Info("AI descriptions enabled",
			zap.String("queue", aidescservice.QueueName),
			zap.String("model", cfg.AIDescriptionsModel))
	} else {
		if cfg.AIDescriptionsEnabled {
			logger.Warn("AI_DESCRIPTIONS_ENABLED is set but ANTHROPIC_API_KEY is empty; AI descriptions disabled")
		}
		logger.Info("AI descriptions disabled")
	}

	// AI collection drafts: only when switched on and a key is set.
	if cfg.AICollectionsActive() {
		client := llm.NewAnthropic(cfg.AnthropicAPIKey, cfg.AICollectionsModel)
		collLookup := curatedservice.NewService(curatedrepository.NewGormRepository(db)).WithLogger(logger)
		collHandlers := aicollservice.NewHandlers(aicollrepository.NewGormRepository(db), collLookup, client, cfg.AICollectionsModel, logger)

		collMux := asynq.NewServeMux()
		collMux.HandleFunc(aicollservice.TaskTypeDraft, collHandlers.HandleDraft)

		collSrv = asynq.NewServer(redisOpt, aiCollectionsServerConfig(zapAsynqLogger{logger}))
		if err := collSrv.Start(collMux); err != nil {
			collSrv = nil
			shutdownServers()
			logger.Fatal("failed to start AI collection drafts asynq server", zap.Error(err))
		}
		// Never log the API key.
		logger.Info("AI collection drafts enabled",
			zap.String("queue", aicollservice.QueueName),
			zap.String("model", cfg.AICollectionsModel))
	} else {
		if cfg.AICollectionsEnabled {
			logger.Warn("AI_COLLECTIONS_ENABLED is set but ANTHROPIC_API_KEY is empty; AI collection drafts disabled")
		}
		logger.Info("AI collection drafts disabled")
	}

	// AI genre suggestions: only when switched on and a key is set.
	if cfg.AIGenresActive() {
		client := llm.NewAnthropic(cfg.AnthropicAPIKey, cfg.AIGenresModel)
		genresRepo := aigenresrepository.NewGormRepository(db)
		analyzer := aigenresservice.NewAnalyzer(client, genresRepo, adminService, cfg.AIGenresModel, cfg.AIGenresBatch, logger)
		genresHandlers := aigenresservice.NewHandlers(analyzer, genresRepo, adminService, cfg.AIGenresModel, logger)

		genresMux := asynq.NewServeMux()
		genresMux.HandleFunc(aigenresservice.TaskTypeAnalyze, genresHandlers.HandleAnalyze)
		genresMux.HandleFunc(aigenresservice.TaskTypeScheduled, genresHandlers.HandleScheduled)

		genresSrv = asynq.NewServer(redisOpt, aiGenresServerConfig(zapAsynqLogger{logger}))
		if err := genresSrv.Start(genresMux); err != nil {
			genresSrv = nil
			shutdownServers()
			logger.Fatal("failed to start AI genres asynq server", zap.Error(err))
		}
		// Never log the API key.
		logger.Info("AI genre suggestions enabled",
			zap.String("queue", aigenresservice.QueueName),
			zap.String("model", cfg.AIGenresModel),
			zap.Int("batch", cfg.AIGenresBatch))
	} else {
		if cfg.AIGenresEnabled {
			logger.Warn("AI_GENRES_ENABLED is set but ANTHROPIC_API_KEY is empty; AI genre suggestions disabled")
		}
		logger.Info("AI genre suggestions disabled")
	}

	// Self-hosted covers (#489): the worker is the only writer of the storage.
	// Own server with Concurrency 1 so downloads are serialized and paced.
	if cfg.CoversMirrorEnabled {
		coversSvc := coversservice.New(coversrepository.NewGormRepository(db), coverstore.NewFS(cfg.CoversDir),
			coverfetch.New(cfg.CoversAllowedHosts), logger)
		coversHandlers := coversservice.NewHandlers(coversSvc, cfg.CoversSweepBatch, logger)

		coversMux := asynq.NewServeMux()
		coversMux.HandleFunc(coversservice.TaskTypeMirror, coversHandlers.HandleMirror)
		coversMux.HandleFunc(coversservice.TaskTypeSweep, coversHandlers.HandleSweep)

		coversSrv = asynq.NewServer(redisOpt, coversServerConfig(zapAsynqLogger{logger}))
		if err := coversSrv.Start(coversMux); err != nil {
			coversSrv = nil
			shutdownServers()
			logger.Fatal("failed to start covers asynq server", zap.Error(err))
		}
		logger.Info("self-hosted covers enabled",
			zap.String("queue", coversservice.QueueName),
			zap.String("dir", cfg.CoversDir),
			zap.Strings("allowed_hosts", cfg.CoversAllowedHosts))
	} else {
		logger.Info("self-hosted covers mirroring disabled")
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
	if cfg.AdaptationSyncScheduleEnabled {
		logger.Info("scheduled adaptation sync enabled",
			zap.String("task", screenworksservice.TaskTypeCatalogSyncTick),
			zap.String("cron", cfg.AdaptationSyncScheduleCron),
			zap.String("location", "UTC"))
	}
	if cfg.AutoCollectionsScheduleEnabled {
		logger.Info("scheduled auto collections recompute enabled",
			zap.String("task", curatedservice.TaskTypeRecomputeAuto),
			zap.String("cron", cfg.AutoCollectionsScheduleCron),
			zap.String("location", "UTC"))
	}
	if cfg.AIModerationActive() {
		// Never log the API key.
		logger.Info("AI moderation enabled",
			zap.String("task", aimodservice.TaskTypeSweep),
			zap.String("cron", cfg.AIModerationCron),
			zap.Int("batch", cfg.AIModerationBatch),
			zap.String("model", cfg.AIModerationModel),
			zap.String("location", "UTC"))
	}

	if cfg.CoversMirrorEnabled && cfg.CoversSweepCron != "" {
		logger.Info("covers sweep schedule enabled",
			zap.String("task", coversservice.TaskTypeSweep),
			zap.String("cron", cfg.CoversSweepCron),
			zap.Int("batch", cfg.CoversSweepBatch),
			zap.String("location", "UTC"))
	}

	if cfg.AIGenresActive() && cfg.AIGenresCron != "" {
		logger.Info("AI genre analysis schedule enabled",
			zap.String("task", aigenresservice.TaskTypeScheduled),
			zap.String("cron", cfg.AIGenresCron),
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

// curatedServerConfig is the config of the automatic collections server (its
// own queue, Concurrency 1: one recompute at a time).
func curatedServerConfig(logger asynq.Logger) asynq.Config {
	return asynq.Config{
		Queues: map[string]int{
			curatedservice.QueueName: 1,
		},
		Concurrency: 1,
		Logger:      logger,
	}
}

// aiModerationServerConfig is the config of the AI moderation server (its
// own queue, Concurrency 1: one model call at a time).
func aiModerationServerConfig(logger asynq.Logger) asynq.Config {
	return asynq.Config{
		Queues: map[string]int{
			aimodservice.QueueName: 1,
		},
		Concurrency: 1,
		Logger:      logger,
	}
}

// aiCuratorServerConfig is the config of the AI curator server (its own
// queue, Concurrency 1: one curation request at a time).
func aiCuratorServerConfig(logger asynq.Logger) asynq.Config {
	return asynq.Config{
		Queues: map[string]int{
			curatorservice.QueueName: 1,
		},
		Concurrency: 1,
		Logger:      logger,
	}
}

// coversServerConfig is the config of the covers server (its own queue,
// Concurrency 1: one download at a time).
func coversServerConfig(logger asynq.Logger) asynq.Config {
	return asynq.Config{
		Queues: map[string]int{
			coversservice.QueueName: 1,
		},
		Concurrency: 1,
		Logger:      logger,
	}
}

// aiDescriptionsServerConfig is the config of the AI descriptions server
// (its own queue, Concurrency 1: one model call at a time).
func aiDescriptionsServerConfig(logger asynq.Logger) asynq.Config {
	return asynq.Config{
		Queues: map[string]int{
			aidescservice.QueueName: 1,
		},
		Concurrency: 1,
		Logger:      logger,
	}
}

// aiCollectionsServerConfig is the config of the AI collection drafts server
// (its own queue, Concurrency 1: one draft is generated at a time).
func aiCollectionsServerConfig(logger asynq.Logger) asynq.Config {
	return asynq.Config{
		Queues: map[string]int{
			aicollservice.QueueName: 1,
		},
		Concurrency: 1,
		Logger:      logger,
	}
}

// aiGenresServerConfig is the config of the AI genre suggestions server (its
// own queue, Concurrency 1: one analysis at a time, one model call at a time).
func aiGenresServerConfig(logger asynq.Logger) asynq.Config {
	return asynq.Config{
		Queues: map[string]int{
			aigenresservice.QueueName: 1,
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
// cfg.HarvestScheduleCron), the catalog adaptation sync tick only when
// cfg.AdaptationSyncScheduleEnabled (on cfg.AdaptationSyncScheduleCron), the automatic collections recompute only when
// cfg.AutoCollectionsScheduleEnabled (on cfg.AutoCollectionsScheduleCron) and
// the AI moderation sweep only when
// cfg.AIModerationActive() (on cfg.AIModerationCron); an invalid spec
// fails here.
func registerSchedules(s scheduleRegistrar, cfg *config.Config) error {
	if _, err := s.Register(similarityCronSpec,
		catalogservice.NewRecomputeSimilaritiesTask(),
		catalogservice.RecomputeTaskOptions()...); err != nil {
		return err
	}
	if cfg == nil {
		return nil
	}
	if cfg.HarvestScheduleEnabled {
		if _, err := s.Register(cfg.HarvestScheduleCron,
			harvestservice.NewScheduleTickTask(),
			harvestservice.ScheduleTickOptions()...); err != nil {
			return fmt.Errorf("register scheduled harvest (%q): %w", cfg.HarvestScheduleCron, err)
		}
	}
	if cfg.AdaptationSyncScheduleEnabled {
		if _, err := s.Register(cfg.AdaptationSyncScheduleCron,
			screenworksservice.NewCatalogSyncTickTask(),
			screenworksservice.CatalogSyncTickOptions()...); err != nil {
			return fmt.Errorf("register scheduled adaptation sync (%q): %w", cfg.AdaptationSyncScheduleCron, err)
		}
	}
	if cfg.AutoCollectionsScheduleEnabled {
		if _, err := s.Register(cfg.AutoCollectionsScheduleCron,
			curatedservice.NewRecomputeAutoTask(),
			curatedservice.RecomputeAutoOptions()...); err != nil {
			return fmt.Errorf("register auto collections recompute (%q): %w", cfg.AutoCollectionsScheduleCron, err)
		}
	}
	if cfg.AIModerationActive() {
		task, err := aimodservice.NewSweepTask(cfg.AIModerationBatch, aimodservice.TriggerScheduled)
		if err != nil {
			return err
		}
		if _, err := s.Register(cfg.AIModerationCron, task, aimodservice.ScheduledSweepOptions()...); err != nil {
			return fmt.Errorf("register AI moderation sweep (%q): %w", cfg.AIModerationCron, err)
		}
	}
	if cfg.CoversMirrorEnabled && cfg.CoversSweepCron != "" {
		if _, err := s.Register(cfg.CoversSweepCron, coversservice.NewSweepTask(), coversservice.SweepOptions()...); err != nil {
			return fmt.Errorf("register covers sweep (%q): %w", cfg.CoversSweepCron, err)
		}
	}
	if cfg.AIGenresActive() && cfg.AIGenresCron != "" {
		if _, err := s.Register(cfg.AIGenresCron, aigenresservice.NewScheduledTask(), aigenresservice.ScheduledOptions()...); err != nil {
			return fmt.Errorf("register AI genre analysis (%q): %w", cfg.AIGenresCron, err)
		}
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
