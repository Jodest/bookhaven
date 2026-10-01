package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bookhaven/backend/internal/modules/notifications/domain"
	"github.com/bookhaven/backend/internal/modules/notifications/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNotFound is returned when a notification doesn't exist or doesn't
// belong to the requesting user - deliberately the same error for both
// cases (see docs/features/review-like-notifications.md: "even forbidden
// should read as 404, per the point above", to avoid leaking the existence
// of other users' notifications).
var ErrNotFound = errors.New("notification not found")

// Service provides notifications logic. Every method takes the
// authenticated userID explicitly and scopes its query by it - ownership is
// enforced in the query itself, not a separate authorization check.
type Service struct {
	repo repository.Repository
}

// NewService creates a new notifications Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// ListResult is the result of listing a user's notifications.
type ListResult struct {
	Items []domain.Notification
	Total int64
	Page  int
	Limit int
}

// List returns the caller's notifications with pagination, newest first.
// Default limit 20, max 100 (matches reviews pagination convention).
func (s *Service) List(ctx context.Context, userID uuid.UUID, page, limit int) (*ListResult, error) {
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

	items, total, err := s.repo.ListByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	return &ListResult{Items: items, Total: total, Page: page, Limit: limit}, nil
}

// UnreadCount returns the number of unread notifications for the caller.
func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	count, err := s.repo.UnreadCount(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

// MarkRead marks one of the caller's notifications read. Returns
// ErrNotFound if the notification doesn't exist or belongs to someone else.
func (s *Service) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	if err := s.repo.MarkRead(ctx, userID, notificationID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("mark notification read: %w", err)
	}
	return nil
}

// MarkAllRead marks all of the caller's unread notifications read.
func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	if err := s.repo.MarkAllRead(ctx, userID); err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}
