package service

import (
	"context"
	"fmt"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// LoadBooksWithRelations loads the books with ids (any status) with their
// authors and genres preloaded, in no particular order; unknown ids are
// dropped. It is the narrow book loader of the AI descriptions module
// (modules/aidescriptions), a thin wrapper over the repository's
// LoadBooksForReview.
func (s *Service) LoadBooksWithRelations(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Book, error) {
	if len(ids) == 0 {
		return []catalogDomain.Book{}, nil
	}
	books, err := s.repo.LoadBooksForReview(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load books with relations: %w", err)
	}
	return books, nil
}
