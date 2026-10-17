package service

import (
	"context"
	"time"

	adminrepository "github.com/bookhaven/backend/internal/modules/admin/repository"
	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/bookhaven/backend/internal/modules/aigenres/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// Repository is the storage the module needs; satisfied by
// *repository.GormRepository.
type Repository interface {
	CreateRun(ctx context.Context, run *domain.Run) error
	GetRun(ctx context.Context, id uuid.UUID) (*domain.Run, error)
	LatestRun(ctx context.Context) (*domain.Run, error)
	ActiveRun(ctx context.Context) (*domain.Run, error)
	InterruptStaleRuns(ctx context.Context, cutoff, now time.Time) (int64, error)
	StartRun(ctx context.Context, id uuid.UUID, model, fingerprint string, batchesTotal int, now time.Time) error
	AddBatchResult(ctx context.Context, id uuid.UUID, b repository.BatchResult) error
	FinishRun(ctx context.Context, id uuid.UUID, status string, errorCode *string, now time.Time) error
	LastCleanFingerprint(ctx context.Context) (string, error)
	PurgeRunsOlderThan(ctx context.Context, t time.Time) (int64, error)

	InsertSuggestions(ctx context.Context, items []domain.NewSuggestion, runID uuid.UUID, now time.Time) (int, error)
	ListSuggestions(ctx context.Context, f repository.ListFilter) ([]domain.Suggestion, int64, error)
	GetSuggestion(ctx context.Context, id uuid.UUID) (*domain.Suggestion, error)
	ListOpenSuggestions(ctx context.Context) ([]domain.Suggestion, error)
	CountOpen(ctx context.Context) (int64, error)
	MarkStale(ctx context.Context, ids []uuid.UUID) (int64, error)
	MarkApplied(ctx context.Context, id uuid.UUID, by uuid.UUID, appliedTarget *uuid.UUID, now time.Time) (bool, error)
	MarkDismissed(ctx context.Context, id uuid.UUID, by uuid.UUID, now time.Time) (bool, error)
}

// GenreCatalog is the read-only view of the genres; satisfied by
// *adminservice.Service. Nothing here writes.
type GenreCatalog interface {
	ListGenreAnalysisCandidates(ctx context.Context) ([]adminservice.GenreAnalysisCandidate, error)
	ListGenreSampleBooks(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]adminrepository.SampleBook, error)
	GenreHintPairs(ctx context.Context, genres []catalogDomain.Genre) []adminservice.GenreHintPair
	GenresByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]adminservice.GenreState, error)
}

// GenreApplier is the only way a suggestion changes genres: the existing
// batch genre merge and genre bulk status; satisfied by
// *adminservice.Service. Only Service.Apply uses it, never the Analyzer.
type GenreApplier interface {
	MergeGenresBatch(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID) (*adminservice.GenreBatchMergeOutcome, error)
	BulkSetGenreStatus(ctx context.Context, ids []uuid.UUID, status string) (adminservice.GenreBulkStatusReport, error)
}

// suggestionStale reports whether any genre of the suggestion (target
// included) no longer exists, was merged into another genre or was rejected.
func suggestionStale(s *domain.Suggestion, states map[uuid.UUID]adminservice.GenreState) bool {
	for _, g := range s.Genres {
		st, ok := states[g.GenreID]
		if !ok || st.MergedIntoID != nil || st.Status == catalogDomain.GenreStatusRejected {
			return true
		}
	}
	return false
}
