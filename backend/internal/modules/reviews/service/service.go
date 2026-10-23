package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	"github.com/bookhaven/backend/internal/modules/reviews/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	// ErrBookNotFound is returned when the requested book does not exist or is hidden.
	ErrBookNotFound = errors.New("book not found")
	// ErrReviewNotFound is returned when a review does not exist or is hidden.
	ErrReviewNotFound = errors.New("review not found")
	// ErrReviewHidden is returned when the author tries to edit a review hidden by a moderator.
	ErrReviewHidden = errors.New("review is hidden")
	// ErrForbidden is returned when a user tries to modify someone else's review.
	ErrForbidden = errors.New("forbidden")
	// ErrAlreadyReviewed is returned when a user attempts to create a second review for a book.
	ErrAlreadyReviewed = errors.New("already reviewed")
	// ErrInvalidRating is returned when a rating is outside 1-5.
	ErrInvalidRating = errors.New("invalid rating")
	// ErrInvalidContent is returned when review content is empty or too long.
	ErrInvalidContent = errors.New("invalid content")
	// ErrNothingToUpdate is returned when a PATCH request has no fields to update.
	ErrNothingToUpdate = errors.New("nothing to update")
)

// Service provides reviews logic.
type Service struct {
	repo repository.Repository
}

// NewService creates a new reviews Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// ListReviewsResult is the result of listing reviews.
type ListReviewsResult struct {
	Items    []reviewsDomain.Review
	Total    int64
	Page     int
	Limit    int
	MyReview *reviewsDomain.Review
}

// ListReviews returns visible reviews for a book with pagination.
func (s *Service) ListReviews(ctx context.Context, bookID uuid.UUID, userID *uuid.UUID, page, limit int) (*ListReviewsResult, error) {
	exists, err := s.repo.BookExists(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("check book exists: %w", err)
	}
	if !exists {
		return nil, ErrBookNotFound
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	items, total, err := s.repo.ListByBook(ctx, bookID, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}

	var myReview *reviewsDomain.Review
	if userID != nil {
		mr, err := s.repo.GetByUserAndBook(ctx, *userID, bookID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("get my review: %w", err)
		}
		if err == nil {
			myReview = mr
		}
	}

	return &ListReviewsResult{
		Items:    items,
		Total:    total,
		Page:     page,
		Limit:    limit,
		MyReview: myReview,
	}, nil
}

// CreateReview creates a new review for a book.
func (s *Service) CreateReview(ctx context.Context, userID, bookID uuid.UUID, rating int, content string) (*reviewsDomain.Review, error) {
	content = strings.TrimSpace(content)
	if rating < 1 || rating > 5 {
		return nil, ErrInvalidRating
	}
	if len(content) < 1 || len(content) > 10000 {
		return nil, ErrInvalidContent
	}

	exists, err := s.repo.BookExists(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("check book exists: %w", err)
	}
	if !exists {
		return nil, ErrBookNotFound
	}

	existing, err := s.repo.GetByUserAndBook(ctx, userID, bookID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("check existing review: %w", err)
	}
	if existing != nil {
		return nil, ErrAlreadyReviewed
	}

	review := &reviewsDomain.Review{
		ID:      uuid.New(),
		BookID:  bookID,
		UserID:  userID,
		Rating:  rating,
		Content: content,
	}
	if err := s.repo.Create(ctx, review); err != nil {
		return nil, fmt.Errorf("create review: %w", err)
	}
	return review, nil
}

// UpdateReview updates the current user's review.
func (s *Service) UpdateReview(ctx context.Context, userID, reviewID uuid.UUID, rating *int, content *string) (*reviewsDomain.Review, error) {
	review, err := s.repo.GetByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("get review: %w", err)
	}

	if review.UserID != userID {
		return nil, ErrForbidden
	}
	if review.IsHidden {
		return nil, ErrReviewHidden
	}
	if rating == nil && content == nil {
		return nil, ErrNothingToUpdate
	}

	if rating != nil {
		if *rating < 1 || *rating > 5 {
			return nil, ErrInvalidRating
		}
		review.Rating = *rating
	}
	if content != nil {
		c := strings.TrimSpace(*content)
		if len(c) < 1 || len(c) > 10000 {
			return nil, ErrInvalidContent
		}
		review.Content = c
	}

	if err := s.repo.Update(ctx, review); err != nil {
		return nil, fmt.Errorf("update review: %w", err)
	}
	return review, nil
}

// DeleteReview deletes the current user's review.
func (s *Service) DeleteReview(ctx context.Context, userID, reviewID uuid.UUID) error {
	review, err := s.repo.GetByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReviewNotFound
		}
		return fmt.Errorf("get review: %w", err)
	}

	if review.UserID != userID {
		return ErrForbidden
	}

	if err := s.repo.Delete(ctx, reviewID); err != nil {
		return fmt.Errorf("delete review: %w", err)
	}
	return nil
}

// ToggleLike toggles a like on a review.
func (s *Service) ToggleLike(ctx context.Context, userID, reviewID uuid.UUID) (likesCount int, liked bool, err error) {
	review, err := s.repo.GetByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, ErrReviewNotFound
		}
		return 0, false, fmt.Errorf("get review: %w", err)
	}
	if review.IsHidden {
		return 0, false, ErrReviewNotFound
	}

	liked, likesCount, err = s.repo.ToggleLike(ctx, reviewID, userID, review.UserID)
	if err != nil {
		return 0, false, fmt.Errorf("toggle like: %w", err)
	}
	return likesCount, liked, nil
}
