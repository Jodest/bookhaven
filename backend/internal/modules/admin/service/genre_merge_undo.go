package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	genreMergeHistoryDefaultLimit = 20
	genreMergeHistoryMaxLimit     = 100
)

// ErrGenreMergeNotFound is returned when a merge journal entry does not exist.
var ErrGenreMergeNotFound = errors.New("genre merge not found")

// GenreMergeUndoBlockedError is returned, with nothing changed, when a merge
// cannot be undone safely; Code is one of the repository.GenreMergeUndo*
// codes (already_undone, target_missing, target_merged, source_missing,
// source_not_alias, qid_conflict).
type GenreMergeUndoBlockedError = repository.GenreMergeUndoBlockedError

type actorContextKey struct{}

// WithActor returns a context carrying the id of the admin performing a
// genre merge, so the merge journal can record who did it without changing
// the service signatures. Without it the entry records no performer.
func WithActor(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, actorContextKey{}, userID)
}

// actorFromContext returns the admin set by WithActor, or nil.
func actorFromContext(ctx context.Context) *uuid.UUID {
	id, ok := ctx.Value(actorContextKey{}).(uuid.UUID)
	if !ok {
		return nil
	}
	return &id
}

// GenreMergeUndoOutcome is the result of UndoGenreMerge.
type GenreMergeUndoOutcome struct {
	Genre                   *catalogDomain.Genre
	SourcesRestored         int
	BooksReturned           int64
	BooksRelinked           int64
	CollectionsSwitchedBack int
}

// GenreMergePage is one page of the merge journal.
type GenreMergePage struct {
	Items []repository.GenreMergeEntry
	Total int64
	Page  int
	Limit int
}

// ListGenreMerges returns the merge journal, newest first. page below 1 is
// 1; limit below 1 is 20 and is capped at 100.
func (s *Service) ListGenreMerges(ctx context.Context, page, limit int) (*GenreMergePage, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = genreMergeHistoryDefaultLimit
	}
	if limit > genreMergeHistoryMaxLimit {
		limit = genreMergeHistoryMaxLimit
	}
	entries, total, err := s.repo.ListGenreMerges(ctx, page, limit)
	if err != nil {
		return nil, fmt.Errorf("list genre merges: %w", err)
	}
	return &GenreMergePage{Items: entries, Total: total, Page: page, Limit: limit}, nil
}

// PreviewGenreMergeUndo is the read-only summary of an undo. Returns
// ErrGenreMergeNotFound or *GenreMergeUndoBlockedError.
func (s *Service) PreviewGenreMergeUndo(ctx context.Context, mergeID uuid.UUID) (*repository.GenreMergeUndoPreview, error) {
	preview, err := s.repo.PreviewGenreMergeUndo(ctx, mergeID)
	if err != nil {
		return nil, mapGenreMergeUndoError(err, "preview genre merge undo")
	}
	return &preview, nil
}

// UndoGenreMerge restores the state recorded by a merge journal entry in one
// transaction (see repository.UndoGenreMerge) and recomputes the switched
// back automatic collections afterwards. Returns ErrGenreMergeNotFound or
// *GenreMergeUndoBlockedError, changing nothing.
func (s *Service) UndoGenreMerge(ctx context.Context, mergeID, actorID uuid.UUID) (*GenreMergeUndoOutcome, error) {
	res, err := s.repo.UndoGenreMerge(ctx, mergeID, actorID)
	if err != nil {
		return nil, mapGenreMergeUndoError(err, "undo genre merge")
	}
	s.recomputeSwitchedCollections(ctx, res.AutoCollectionIDs)

	// Re-read the target: the QID may have been taken back from it.
	target, err := s.repo.GetGenreByID(ctx, res.TargetID)
	if err != nil {
		return nil, fmt.Errorf("get target genre: %w", err)
	}
	return &GenreMergeUndoOutcome{
		Genre:                   target,
		SourcesRestored:         res.SourcesRestored,
		BooksReturned:           res.BooksReturned,
		BooksRelinked:           res.BooksRelinked,
		CollectionsSwitchedBack: res.CollectionsSwitchedBack,
	}, nil
}

func mapGenreMergeUndoError(err error, what string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrGenreMergeNotFound
	}
	var blocked *repository.GenreMergeUndoBlockedError
	if errors.As(err, &blocked) {
		return blocked
	}
	return fmt.Errorf("%s: %w", what, err)
}
