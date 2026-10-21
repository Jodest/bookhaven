package service

import (
	"context"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/google/uuid"
)

// publishedBook returns domain.ErrBookNotFound for an unknown or not
// published book.
func (s *Service) publishedBook(ctx context.Context, bookID uuid.UUID) error {
	status, err := s.repo.BookStatus(ctx, bookID)
	if err != nil {
		return err
	}
	if status != catalogDomain.BookStatusPublished {
		return domain.ErrBookNotFound
	}
	return nil
}

// ListVerdicts returns the verdicts of the visible adaptations of a published
// book. userID is nil for a guest. CanVote is true for a user who has the book
// on «Прочитано».
func (s *Service) ListVerdicts(ctx context.Context, bookID uuid.UUID, userID *uuid.UUID) (*domain.Verdicts, error) {
	if err := s.publishedBook(ctx, bookID); err != nil {
		return nil, err
	}
	counts, err := s.repo.ListVerdicts(ctx, bookID, userID)
	if err != nil {
		return nil, err
	}
	out := &domain.Verdicts{Items: make([]domain.Verdict, 0, len(counts))}
	for _, c := range counts {
		out.Items = append(out.Items, domain.NewVerdict(c))
	}
	if userID != nil {
		if out.CanVote, err = s.repo.IsReader(ctx, *userID, bookID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Vote creates or replaces the user's vote and returns the adaptation's
// verdict. The shelf is checked at write time only.
func (s *Service) Vote(ctx context.Context, userID, bookID, workID uuid.UUID, value string) (*domain.Verdict, error) {
	value, err := domain.ParseVoteValue(value)
	if err != nil {
		return nil, err
	}
	if err := s.publishedBook(ctx, bookID); err != nil {
		return nil, err
	}
	visible, err := s.repo.LinkVisible(ctx, bookID, workID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, domain.ErrAdaptationNotFound
	}
	reader, err := s.repo.IsReader(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	if !reader {
		return nil, domain.ErrNotReader
	}
	if err := s.repo.UpsertVote(ctx, userID, bookID, workID, value); err != nil {
		return nil, err
	}
	counts, err := s.repo.ListVerdicts(ctx, bookID, &userID)
	if err != nil {
		return nil, err
	}
	for _, c := range counts {
		if c.ScreenWorkID == workID {
			v := domain.NewVerdict(c)
			return &v, nil
		}
	}
	return nil, domain.ErrAdaptationNotFound
}

// Unvote removes the user's vote. It is idempotent and checks nothing else, so
// a reader can always retract.
func (s *Service) Unvote(ctx context.Context, userID, bookID, workID uuid.UUID) error {
	return s.repo.DeleteVote(ctx, userID, bookID, workID)
}
