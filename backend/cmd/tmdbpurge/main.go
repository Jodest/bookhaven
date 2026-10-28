// Command tmdbpurge removes every TMDB-sourced screen work poster and its
// stored copy. Run it when the TMDB licence ends. It has no HTTP endpoint on
// purpose: it needs the read-write cover storage, which only the worker mounts.
//
//	prod: docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/tmdbpurge worker [-dry-run]
//	dev:  go run ./cmd/tmdbpurge -dry-run   (inside the worker container, COVERS_DIR set)
//
// Remove TMDB_API_KEY from the api and the worker first, otherwise the next
// adaptation sync fills TMDB posters again. See docs/DEPLOYMENT.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/bookhaven/backend/internal/config"
	coversrepository "github.com/bookhaven/backend/internal/modules/covers/repository"
	coversservice "github.com/bookhaven/backend/internal/modules/covers/service"
	"github.com/bookhaven/backend/internal/pkg/coverstore"
	"github.com/bookhaven/backend/internal/pkg/database"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "print the counts without changing anything")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.TMDBAPIKey != "" {
		fmt.Println("warning: TMDB_API_KEY is still set; the next adaptation sync can fill TMDB posters again. Remove the key from the api and worker first.")
	}

	db, err := database.Open(cfg.DatabaseURL, database.PoolConfigFrom(cfg))
	if err != nil {
		fmt.Printf("failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	svc := coversservice.New(coversrepository.NewGormRepository(db), coverstore.NewFS(cfg.CoversDir), nil, logger)
	stats, err := svc.PurgeTMDB(ctx, *dryRun)
	if err != nil {
		fmt.Printf("tmdb purge failed: %v\n", err)
		os.Exit(1)
	}
	if *dryRun {
		fmt.Print("dry run: ")
	}
	fmt.Printf("posters_removed=%d files_deleted=%d\n", stats.PostersRemoved, stats.FilesDeleted)
}
