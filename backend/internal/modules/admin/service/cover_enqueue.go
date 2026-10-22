package service

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CoverEnqueuer queues the copy of a book's cover link into our own storage
// (#489). Declared here to keep the admin module decoupled from the covers
// module; covers/service.Enqueuer satisfies it.
type CoverEnqueuer interface {
	EnqueueMirror(ctx context.Context, bookID uuid.UUID, reason string) error
}

// coverReasonSave marks a copy queued by a save (resets the retry count).
const coverReasonSave = "save"

// WithCoverEnqueuer enables queuing cover copies after book writes. Without
// it (tests, deployments with mirroring off) saves queue nothing and the
// periodic sweep, if any, makes the copies.
func WithCoverEnqueuer(e CoverEnqueuer) Option {
	return func(s *Service) {
		s.coverEnqueuer = e
	}
}

// enqueueCover is called after the write has committed. A failure is logged
// and never fails the save: the sweep makes the copy later.
func (s *Service) enqueueCover(ctx context.Context, bookID uuid.UUID) {
	if s.coverEnqueuer == nil || s.suppressCoverEnqueue {
		return
	}
	if err := s.coverEnqueuer.EnqueueMirror(ctx, bookID, coverReasonSave); err != nil {
		s.log.Warn("cover mirror enqueue failed", zap.String("book_id", bookID.String()), zap.Error(err))
	}
}
