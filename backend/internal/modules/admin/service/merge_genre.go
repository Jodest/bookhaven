package service

import (
	"context"
	"errors"
	"fmt"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ErrCannotMergeGenreIntoSelf is returned by MergeGenres, without changing
// anything, when sourceID and targetID are the same genre.
var ErrCannotMergeGenreIntoSelf = errors.New("cannot merge a genre into itself")

// MergeGenres merges the source genre into the target genre
// (docs/features/genre-moderation-queue.md rule 4). Unlike MergeAuthors the
// source is not deleted: its book_genres links move to the target (books
// already linked to both keep one link), its existing aliases are
// re-pointed to the target, and the source itself becomes a rejected alias
// row (merged_into_id = target) that keeps its name and slug, so future
// harvests and file imports of its slug or QID resolve to the target. The
// source QID moves to the target only when the target has none; otherwise
// the source keeps it (logged) and the alias resolves it.
//
// The target's name, slug and status are never changed. Returns the target
// (with a carried QID applied in memory) and the number of reassigned
// books. Returns ErrCannotMergeGenreIntoSelf when sourceID == targetID and
// ErrGenreNotFound when either genre is missing or already an alias - in
// both cases nothing is changed.
func (s *Service) MergeGenres(ctx context.Context, sourceID, targetID uuid.UUID) (*catalogDomain.Genre, int64, error) {
	if sourceID == targetID {
		return nil, 0, ErrCannotMergeGenreIntoSelf
	}

	// GetGenreByID hides alias rows, so an alias source or target is a 404.
	source, err := s.repo.GetGenreByID(ctx, sourceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, ErrGenreNotFound
		}
		return nil, 0, fmt.Errorf("get source genre: %w", err)
	}
	target, err := s.repo.GetGenreByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, ErrGenreNotFound
		}
		return nil, 0, fmt.Errorf("get target genre: %w", err)
	}

	// Snapshot both rows before the merge rewrites the source.
	src, result := *source, *target

	booksReassigned, err := s.repo.MergeGenres(ctx, sourceID, targetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Concurrently merged away between the reads and the merge.
			return nil, 0, ErrGenreNotFound
		}
		return nil, 0, fmt.Errorf("merge genres: %w", err)
	}

	// Mirror the repository's in-transaction QID carry-over.
	if src.WikidataQID != nil {
		if result.WikidataQID == nil {
			qid := *src.WikidataQID
			result.WikidataQID = &qid
		} else {
			s.log.Info("genre merge kept the source qid on the alias; target keeps its own",
				zap.String("target_id", targetID.String()),
				zap.String("source_id", sourceID.String()),
				zap.String("source_qid", *src.WikidataQID),
				zap.String("target_qid", *result.WikidataQID))
		}
	}

	return &result, booksReassigned, nil
}
