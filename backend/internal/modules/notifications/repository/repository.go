package repository

import (
	"context"
	"time"

	"github.com/bookhaven/backend/internal/modules/notifications/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository defines operations for working with notifications. Creation is
// intentionally not part of this interface: a notification row is written
// as a side effect of the event that causes it (e.g. reviews/repository's
// ToggleLike upserts directly into the notifications table within its own
// transaction - see docs/features/review-like-notifications.md Backend
// Design), keeping this module independent of its producers.
type Repository interface {
	// ListByUser returns a recipient's notifications, newest first.
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Notification, int64, error)
	// UnreadCount returns the number of unread notifications for a recipient.
	UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)
	// MarkRead marks one notification read, scoped to its owner. Returns
	// gorm.ErrRecordNotFound if it doesn't exist or isn't owned by userID -
	// ownership is enforced in the query itself, not a separate check, so a
	// non-owner can't distinguish "doesn't exist" from "not yours".
	MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error
	// MarkAllRead marks all of a recipient's unread notifications read.
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
}

// GormRepository is a GORM-based implementation of Repository.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a new GormRepository.
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

type notificationRow struct {
	ID             uuid.UUID
	ActorID        uuid.UUID
	Type           string
	ReviewID       *uuid.UUID `gorm:"column:review_id"`
	IsRead         bool       `gorm:"column:is_read"`
	CreatedAt      time.Time
	ActorName      *string    `gorm:"column:actor_name"`
	ActorAvatarURL *string    `gorm:"column:actor_avatar_url"`
	BookID         *uuid.UUID `gorm:"column:book_id"`
	BookSlug       *string    `gorm:"column:book_slug"`
	BookTitle      *string    `gorm:"column:book_title"`
}

// ListByUser returns a recipient's notifications with pagination, newest
// first, joining in the acting user's display fields and the liked review's
// book (same raw-SQL join style as reviews/repository.GormRepository.ListByBook).
func (r *GormRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Notification, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("user_id = ?", userID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var rows []notificationRow
	sql := `
		SELECT
			n.id,
			n.actor_id,
			n.type,
			n.review_id,
			n.is_read,
			n.created_at,
			actor.name AS actor_name,
			actor.avatar_url AS actor_avatar_url,
			b.id AS book_id,
			b.slug AS book_slug,
			b.title AS book_title
		FROM notifications n
		JOIN users actor ON actor.id = n.actor_id
		LEFT JOIN reviews r ON r.id = n.review_id
		LEFT JOIN books b ON b.id = r.book_id
		WHERE n.user_id = ?
		ORDER BY n.created_at DESC
		LIMIT ? OFFSET ?
	`
	if err := r.db.WithContext(ctx).Raw(sql, userID, limit, offset).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	notifications := make([]domain.Notification, 0, len(rows))
	for _, row := range rows {
		notifications = append(notifications, domain.Notification{
			ID:             row.ID,
			UserID:         userID,
			ActorID:        row.ActorID,
			Type:           row.Type,
			ReviewID:       row.ReviewID,
			IsRead:         row.IsRead,
			CreatedAt:      row.CreatedAt,
			ActorName:      row.ActorName,
			ActorAvatarURL: row.ActorAvatarURL,
			BookID:         row.BookID,
			BookSlug:       row.BookSlug,
			BookTitle:      row.BookTitle,
		})
	}

	return notifications, total, nil
}

// UnreadCount returns the number of unread notifications for a recipient -
// the single cheap query polled on an interval by the frontend bell badge.
func (r *GormRepository) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// MarkRead marks a single notification read, scoped to its owner.
func (r *GormRepository) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Update("is_read", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// MarkAllRead marks all of a recipient's unread notifications read.
func (r *GormRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Update("is_read", true).Error
}
