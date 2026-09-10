package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/bookhaven/backend/internal/modules/reading_challenges/domain"
	"github.com/bookhaven/backend/internal/modules/reading_challenges/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidGoal = errors.New("invalid goal")
	ErrInvalidYear = errors.New("invalid year")
)

const (
	// MinGoal and MaxGoal bound the number of books a user may set as a goal.
	MinGoal = 1
	MaxGoal = 10000
	// MinYear is the earliest year a challenge may be recorded for.
	MinYear = 2000
)

// Progress represents a computed reading-challenge snapshot for a year: the
// stored goal (nil if unset) and the live-computed progress toward it.
type Progress struct {
	Year            int
	Goal            *int
	BooksRead       int64
	ProgressPercent *int
	Completed       bool
}

// Service provides reading-challenge logic.
type Service struct {
	repo repository.Repository
}

// NewService creates a new reading-challenges Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

func maxYear() int {
	return time.Now().UTC().Year() + 1
}

func validateYear(year int) error {
	if year < MinYear || year > maxYear() {
		return ErrInvalidYear
	}
	return nil
}

func validateGoal(goal int) error {
	if goal < MinGoal || goal > MaxGoal {
		return ErrInvalidGoal
	}
	return nil
}

func computeProgress(year int, goal *int, booksRead int64) Progress {
	progress := Progress{Year: year, Goal: goal, BooksRead: booksRead}
	if goal == nil {
		return progress
	}

	percent := int(math.Round(float64(booksRead) / float64(*goal) * 100))
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}
	progress.ProgressPercent = &percent
	progress.Completed = booksRead >= int64(*goal)
	return progress
}

// UpsertGoal creates or updates the caller's goal for a year and returns the
// resulting progress. It is an upsert: calling it twice for the same
// (userID, year) updates the existing row rather than creating a duplicate.
func (s *Service) UpsertGoal(ctx context.Context, userID uuid.UUID, year, goal int) (*Progress, error) {
	if err := validateYear(year); err != nil {
		return nil, err
	}
	if err := validateGoal(goal); err != nil {
		return nil, err
	}

	challenge := &domain.ReadingChallenge{
		ID:     uuid.New(),
		UserID: userID,
		Year:   year,
		Goal:   goal,
	}
	if err := s.repo.Upsert(ctx, challenge); err != nil {
		return nil, fmt.Errorf("upsert reading challenge: %w", err)
	}

	return s.GetProgress(ctx, userID, year)
}

// GetProgress returns the caller's goal (nullable) and live-computed
// progress for a year.
func (s *Service) GetProgress(ctx context.Context, userID uuid.UUID, year int) (*Progress, error) {
	if err := validateYear(year); err != nil {
		return nil, err
	}

	booksRead, err := s.repo.CountReadBooks(ctx, userID, year)
	if err != nil {
		return nil, fmt.Errorf("count read books: %w", err)
	}

	challenge, err := s.repo.GetByUserAndYear(ctx, userID, year)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			progress := computeProgress(year, nil, booksRead)
			return &progress, nil
		}
		return nil, fmt.Errorf("get reading challenge: %w", err)
	}

	goal := challenge.Goal
	progress := computeProgress(year, &goal, booksRead)
	return &progress, nil
}

// ListHistory returns the caller's challenge history (every year with a
// stored goal), newest first, each with live-computed progress.
func (s *Service) ListHistory(ctx context.Context, userID uuid.UUID) ([]Progress, error) {
	challenges, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list reading challenges: %w", err)
	}

	result := make([]Progress, 0, len(challenges))
	for _, c := range challenges {
		booksRead, err := s.repo.CountReadBooks(ctx, userID, c.Year)
		if err != nil {
			return nil, fmt.Errorf("count read books: %w", err)
		}
		goal := c.Goal
		result = append(result, computeProgress(c.Year, &goal, booksRead))
	}
	return result, nil
}
