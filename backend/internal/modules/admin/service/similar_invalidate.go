package service

import (
	"context"

	"go.uber.org/zap"
)

// SimilarInvalidator drops the cached "similar books" lists. Declared here
// (not imported from catalog) to keep the admin module decoupled; the Redis
// similar-books cache satisfies it. Any book visibility or composition change
// (status, authors, genres) can alter the lists of many other books, so the
// whole cache is invalidated at once.
type SimilarInvalidator interface {
	Invalidate(ctx context.Context) error
}

// WithSimilarInvalidator enables similar-books cache invalidation after book,
// author and genre writes. Without it (tests, deployments without Redis) the
// writes simply skip invalidation.
func WithSimilarInvalidator(i SimilarInvalidator) Option {
	return func(s *Service) {
		s.similarInvalidator = i
	}
}

// invalidateSimilar is called after the write has committed. A failure is
// logged and never fails the admin operation: the cached lists then expire
// by their TTL.
func (s *Service) invalidateSimilar(ctx context.Context) {
	if s.similarInvalidator == nil {
		return
	}
	if err := s.similarInvalidator.Invalidate(ctx); err != nil {
		s.log.Warn("similar books cache invalidation failed", zap.Error(err))
	}
}
