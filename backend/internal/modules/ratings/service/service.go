package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bookhaven/backend/internal/modules/ratings/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	// ErrInvalidValue is returned when a rating value is outside 1-5.
	ErrInvalidValue = errors.New("invalid rating value")
	// ErrBookNotFound is returned when the requested book does not exist or is hidden.
	ErrBookNotFound = errors.New("book not found")
	// ErrRatingNotFound is returned when a user has no rating for a book.
	ErrRatingNotFound = repository.ErrRatingNotFound
)

// Service provides ratings logic.
type Service struct {
	repo repository.Repository
}

// NewService creates a new ratings Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// SetRatingResult contains the outcome of setting a rating.
type SetRatingResult struct {
	Value        int
	Rating       float64
	RatingsCount int
	Created      bool
}

// SetRating creates or updates a user's rating for a book.
func (s *Service) SetRating(ctx context.Context, userID, bookID uuid.UUID, value int) (*SetRatingResult, error) {
	if value < 1 || value > 5 {
		return nil, ErrInvalidValue
	}

	exists, err := s.repo.BookExists(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("check book exists: %w", err)
	}
	if !exists {
		return nil, ErrBookNotFound
	}

	created, rating, count, err := s.repo.UpsertAndRecalc(ctx, bookID, userID, value)
	if err != nil {
		return nil, fmt.Errorf("upsert rating: %w", err)
	}

	return &SetRatingResult{
		Value:        value,
		Rating:       rating,
		RatingsCount: count,
		Created:      created,
	}, nil
}

// GetMyRating returns the current user's rating for a book.
func (s *Service) GetMyRating(ctx context.Context, userID, bookID uuid.UUID) (int, error) {
	rating, err := s.repo.GetByUserAndBook(ctx, userID, bookID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrRatingNotFound
		}
		return 0, fmt.Errorf("get rating: %w", err)
	}
	return rating.Value, nil
}
