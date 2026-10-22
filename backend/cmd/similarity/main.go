// Command similarity recomputes the precomputed similar-books table
// (book_similarities, stage 2, see docs/features/similar-books-stage2.md)
// synchronously, in-process, and prints the run stats. It runs the same
// SimilarityJob as the daily worker task but needs no Redis or worker.
//
//	dev:  docker compose exec backend go run ./cmd/similarity
//	prod: docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/similarity backend
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/bookhaven/backend/internal/config"
	catalogrepository "github.com/bookhaven/backend/internal/modules/catalog/repository"
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	"github.com/bookhaven/backend/internal/pkg/database"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}

	db, err := database.Open(cfg.DatabaseURL, database.PoolConfigFrom(cfg))
	if err != nil {
		fmt.Printf("failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	repo := catalogrepository.NewGormSimilarityRepository(db)
	stats, err := catalogservice.NewSimilarityJob(repo, repo, logger).Run(ctx)
	if err != nil {
		fmt.Printf("similarity recompute failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("similarity recompute done: interactions=%d users_used=%d books_with_co=%d co_pairs=%d books_with_rows=%d rows=%d skipped=%d duration=%s\n",
		stats.Interactions, stats.UsersUsed, stats.BooksWithCo, stats.CoPairs,
		stats.BooksWithRows, stats.Rows, stats.Skipped, stats.Duration)
}
