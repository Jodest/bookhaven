package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	// ErrGenreMergeNoSources is returned by MergeGenresBatch and
	// PreviewGenreMerge, without changing anything, for an empty source list.
	ErrGenreMergeNoSources = errors.New("source_ids must not be empty")
	// ErrGenreMergeDuplicateSource is returned for a source id listed twice;
	// a merge is irreversible, so a duplicate is a client bug, not collapsed.
	ErrGenreMergeDuplicateSource = errors.New("duplicate source id")
	// ErrGenreMergeTooLarge is returned for more sources than the cap.
	ErrGenreMergeTooLarge = fmt.Errorf("too many source ids: at most %d per request", bulkGenreStatusCap)
)

// CollectionRecomputer recomputes automatic curated collections. Declared
// here so the admin module does not import curated; satisfied by
// *curated/service.Service.
type CollectionRecomputer interface {
	RecomputeCollections(ctx context.Context, ids []uuid.UUID)
}

// WithCollectionRecomputer enables the recomputation of automatic curated
// collections switched by a genre merge. Without it they keep their previous
// snapshot until the next scheduled recomputation.
func WithCollectionRecomputer(r CollectionRecomputer) Option {
	return func(s *Service) {
		s.collectionRecomputer = r
	}
}

// recomputeSwitchedCollections refreshes the automatic collections a merge
// switched to another genre. Failures are only logged by the recomputer: the
// merge never fails because of it.
func (s *Service) recomputeSwitchedCollections(ctx context.Context, ids []uuid.UUID) {
	if s.collectionRecomputer == nil || len(ids) == 0 {
		return
	}
	s.collectionRecomputer.RecomputeCollections(ctx, ids)
}

// GenreBatchMergeOutcome is the result of MergeGenresBatch.
type GenreBatchMergeOutcome struct {
	Genre               *catalogDomain.Genre
	SourcesMerged       int
	BooksReassigned     int64
	CollectionsSwitched int
	MergeID             uuid.UUID
}

// validateGenreMergeInput checks the shared batch/preview input: at least one
// source, no duplicates, at most bulkGenreStatusCap, target not a source.
func validateGenreMergeInput(sourceIDs []uuid.UUID, targetID uuid.UUID) error {
	if len(sourceIDs) == 0 {
		return ErrGenreMergeNoSources
	}
	if len(sourceIDs) > bulkGenreStatusCap {
		return ErrGenreMergeTooLarge
	}
	seen := make(map[uuid.UUID]struct{}, len(sourceIDs))
	for _, id := range sourceIDs {
		if id == targetID {
			return ErrCannotMergeGenreIntoSelf
		}
		if _, dup := seen[id]; dup {
			return ErrGenreMergeDuplicateSource
		}
		seen[id] = struct{}{}
	}
	return nil
}

// MergeGenresBatch merges every source genre into the target in one
// transaction with the per-source semantics of MergeGenres, in sourceIDs
// order (the first source with a QID wins the carry-over). Returns the
// validation errors of validateGenreMergeInput and ErrGenreNotFound when any
// genre is missing or already an alias - in every error case nothing changes.
func (s *Service) MergeGenresBatch(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID) (*GenreBatchMergeOutcome, error) {
	if err := validateGenreMergeInput(sourceIDs, targetID); err != nil {
		return nil, err
	}

	merged, err := s.repo.MergeGenresBatch(ctx, sourceIDs, targetID, actorFromContext(ctx))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGenreNotFound
		}
		return nil, fmt.Errorf("merge genres batch: %w", err)
	}
	s.invalidateSimilar(ctx)
	s.recomputeSwitchedCollections(ctx, merged.AutoCollectionIDs)

	for sourceID, qid := range merged.KeptQIDs {
		s.log.Info("genre merge kept the source qid on the alias; target keeps its own",
			zap.String("target_id", targetID.String()),
			zap.String("source_id", sourceID.String()),
			zap.String("source_qid", qid))
	}

	// Re-read the target: the QID carry-over may have changed it.
	target, err := s.repo.GetGenreByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGenreNotFound
		}
		return nil, fmt.Errorf("get target genre: %w", err)
	}
	return &GenreBatchMergeOutcome{
		Genre:               target,
		SourcesMerged:       merged.SourcesMerged,
		BooksReassigned:     merged.BooksReassigned,
		CollectionsSwitched: merged.CollectionsSwitched,
		MergeID:             merged.MergeID,
	}, nil
}

// PreviewGenreMerge is the read-only summary of a merge (one source previews
// a single merge): books that would move and collections that would be
// switched. Same validation and not-found rules as MergeGenresBatch.
func (s *Service) PreviewGenreMerge(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID) (*repository.GenreMergePreview, error) {
	if err := validateGenreMergeInput(sourceIDs, targetID); err != nil {
		return nil, err
	}
	preview, err := s.repo.PreviewGenreMerge(ctx, sourceIDs, targetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGenreNotFound
		}
		return nil, fmt.Errorf("preview genre merge: %w", err)
	}
	return &preview, nil
}
