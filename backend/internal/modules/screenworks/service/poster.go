package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/bookhaven/backend/internal/pkg/coverfetch"
	"github.com/bookhaven/backend/internal/pkg/coverstore"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Option configures optional parts of the Service.
type Option func(*Service)

// WithPosterStore enables admin poster uploads, stored in store.
func WithPosterStore(store coverstore.Store) Option {
	return func(s *Service) { s.posters = store }
}

// WithTMDB enables the TMDB poster lookup of the adaptation sync.
func WithTMDB(l PosterLookup) Option {
	return func(s *Service) { s.tmdb = l }
}

// UploadPoster stores an uploaded poster as the work's manual poster and
// returns the updated detail. The file type is sniffed from the bytes (the
// client's content type and file name are not trusted).
func (s *Service) UploadPoster(ctx context.Context, id uuid.UUID, data []byte) (*domain.WorkDetail, error) {
	if s.posters == nil {
		return nil, domain.ErrPostersUnavailable
	}
	if len(data) > coverfetch.MaxBytes {
		return nil, domain.ErrPosterTooLarge
	}
	ext := coverfetch.SniffExt(data)
	if ext == "" {
		return nil, domain.ErrPosterType
	}
	if _, err := s.repo.GetWork(ctx, id); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	hexSum := hex.EncodeToString(sum[:])
	name := hexSum[:2] + "/" + hexSum + "." + ext
	if err := s.posters.Put(ctx, name, data); err != nil {
		return nil, err
	}
	prev, err := s.repo.SetManualPoster(ctx, id, name)
	if err != nil {
		s.collectPoster(ctx, name)
		return nil, err
	}
	if prev != nil && *prev != name {
		s.collectPoster(ctx, *prev)
	}
	return s.repo.GetWork(ctx, id)
}

// RemovePoster removes the work's poster (idempotent) and returns the detail.
func (s *Service) RemovePoster(ctx context.Context, id uuid.UUID) (*domain.WorkDetail, error) {
	prev, err := s.repo.RemovePoster(ctx, id)
	if err != nil {
		return nil, err
	}
	if prev != nil {
		s.collectPoster(ctx, *prev)
	}
	return s.repo.GetWork(ctx, id)
}

// collectPoster deletes an uploaded poster file once no work references it.
// Failures are logged only: an orphan file is harmless.
func (s *Service) collectPoster(ctx context.Context, path string) {
	if s.posters == nil || path == "" {
		return
	}
	used, err := s.repo.PosterPathReferenced(ctx, path)
	if err != nil {
		s.log.Warn("poster cleanup: reference check failed", zap.String("path", path), zap.Error(err))
		return
	}
	if used {
		return
	}
	if err := s.posters.Delete(ctx, path); err != nil {
		s.log.Warn("poster cleanup: delete failed", zap.String("path", path), zap.Error(err))
	}
}
