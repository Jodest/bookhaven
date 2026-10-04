package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/bookhaven/backend/internal/modules/catalog/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SimilarityStats summarizes one recompute run.
type SimilarityStats struct {
	Interactions  int
	UsersUsed     int
	BooksWithCo   int // books with at least one co-reading partner
	CoPairs       int // distinct pairs with co >= MinCoReaders
	BooksWithRows int
	Rows          int
	Skipped       int // sources hidden/deleted mid-run
	Duration      time.Duration
}

// SimilarityJob recomputes the whole book_similarities table (stage 2, see
// docs/features/similar-books-stage2.md). It runs from the asynq worker
// (daily) and from cmd/similarity (manual).
type SimilarityJob struct {
	repo  repository.Repository
	store repository.SimilarityStore
	now   func() time.Time
	log   *zap.Logger
}

// NewSimilarityJob builds a SimilarityJob. A nil logger is replaced by a
// no-op logger.
func NewSimilarityJob(repo repository.Repository, store repository.SimilarityStore, log *zap.Logger) *SimilarityJob {
	if log == nil {
		log = zap.NewNop()
	}
	return &SimilarityJob{repo: repo, store: store, now: time.Now, log: log}
}

// Run loads the interactions, computes the co-reading partners and, for
// each book with partners, blends them with the stage-1 candidates. All
// rows are written in one ReplaceSimilarities call (one transaction); any
// error or a cancelled ctx returns before the write, leaving the previous
// data intact.
func (j *SimilarityJob) Run(ctx context.Context) (SimilarityStats, error) {
	start := j.now()
	var stats SimilarityStats

	inter, err := j.store.ListInteractions(ctx)
	if err != nil {
		return stats, fmt.Errorf("list interactions: %w", err)
	}
	partners, readers, usersUsed := coReading(inter, MinCoReaders, MaxUserInteractions)
	stats.Interactions = len(inter)
	stats.UsersUsed = usersUsed
	stats.BooksWithCo = len(partners)
	for _, list := range partners {
		stats.CoPairs += len(list)
	}
	stats.CoPairs /= 2

	var rows []repository.SimilarityRow
	if len(partners) > 0 {
		bookStats, err := j.store.ListVisibleBookStats(ctx)
		if err != nil {
			return stats, fmt.Errorf("list book stats: %w", err)
		}
		ratingsCount := make(map[uuid.UUID]int, len(bookStats))
		for _, b := range bookStats {
			ratingsCount[b.ID] = b.RatingsCount
		}
		authorIDs, err := j.store.ListBookAuthorIDs(ctx)
		if err != nil {
			return stats, fmt.Errorf("list book authors: %w", err)
		}

		sources := make([]uuid.UUID, 0, len(partners))
		for id := range partners {
			sources = append(sources, id)
		}
		sort.Slice(sources, func(i, k int) bool { return sources[i].String() < sources[k].String() })

		computedAt := j.now()
		for _, id := range sources {
			if err := ctx.Err(); err != nil {
				return stats, err
			}
			src, err := j.repo.GetSimilarSource(ctx, id)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					stats.Skipped++
					continue
				}
				return stats, fmt.Errorf("get similar source %s: %w", id, err)
			}
			cands, err := j.repo.ListSimilarCandidates(ctx, id, *src, authorPool, genrePool)
			if err != nil {
				return stats, fmt.Errorf("list similar candidates %s: %w", id, err)
			}
			bookRows := blendSimilar(id, *src, cands, partners[id], readers[id], ratingsCount, authorIDs, MaxSimilarLimit)
			for i := range bookRows {
				bookRows[i].ComputedAt = computedAt
			}
			if len(bookRows) > 0 {
				stats.BooksWithRows++
			}
			rows = append(rows, bookRows...)
		}
	}

	if err := ctx.Err(); err != nil {
		return stats, err
	}
	// With no partners rows is nil: the write clears stale rows and every
	// book falls back to stage 1.
	if err := j.store.ReplaceSimilarities(ctx, rows); err != nil {
		return stats, fmt.Errorf("replace similarities: %w", err)
	}
	stats.Rows = len(rows)
	stats.Duration = j.now().Sub(start)

	j.log.Info("similar books recomputed",
		zap.Int("interactions", stats.Interactions),
		zap.Int("users_used", stats.UsersUsed),
		zap.Int("books_with_co", stats.BooksWithCo),
		zap.Int("co_pairs", stats.CoPairs),
		zap.Int("books_with_rows", stats.BooksWithRows),
		zap.Int("rows", stats.Rows),
		zap.Int("skipped", stats.Skipped),
		zap.Duration("duration", stats.Duration),
	)
	return stats, nil
}
