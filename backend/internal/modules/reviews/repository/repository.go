package repository

import (
	"context"
	"fmt"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository defines operations for working with reviews.
type Repository interface {
	// ListByBook returns paginated visible reviews for a book.
	// When userID is provided, the LikedByMe flag is populated.
	ListByBook(ctx context.Context, bookID uuid.UUID, userID *uuid.UUID, limit, offset int) ([]reviewsDomain.Review, int64, error)
	// GetByID returns a review by ID regardless of visibility.
	GetByID(ctx context.Context, reviewID uuid.UUID) (*reviewsDomain.Review, error)
	// GetByUserAndBook returns a user's review for a specific book.
	GetByUserAndBook(ctx context.Context, userID, bookID uuid.UUID) (*reviewsDomain.Review, error)
	// Create inserts a new review, upserts the user's rating and recalculates aggregates.
	Create(ctx context.Context, review *reviewsDomain.Review) error
	// Update updates a review, upserts the user's rating and recalculates aggregates.
	Update(ctx context.Context, review *reviewsDomain.Review) error
	// Delete removes a review and recalculates aggregates.
	Delete(ctx context.Context, reviewID uuid.UUID) error
	// ToggleLike toggles a like for a review and returns the new state and
	// likes count. revieweeID is the review author's user ID (the caller
	// already has it from a prior GetByID call) - when a new like is created
	// (liked == true) and revieweeID != userID, a review_like notification
	// for the author is upserted in the same transaction.
	ToggleLike(ctx context.Context, reviewID, userID, revieweeID uuid.UUID) (liked bool, likesCount int, err error)
	// BookExists checks whether a published book exists.
	BookExists(ctx context.Context, bookID uuid.UUID) (bool, error)
}

// GormRepository is a GORM-based implementation of Repository.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a new GormRepository.
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

type reviewListRow struct {
	ID            uuid.UUID
	BookID        uuid.UUID
	UserID        uuid.UUID
	Rating        int
	Content       string
	LikesCount    int       `gorm:"column:likes_count"`
	IsHidden      bool      `gorm:"column:is_hidden"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	UserName      *string   `gorm:"column:user_name"`
	UserAvatarURL *string   `gorm:"column:user_avatar_url"`
	LikedByMe     bool      `gorm:"column:liked_by_me"`
}

// ListByBook returns visible reviews for a book with pagination.
func (r *GormRepository) ListByBook(ctx context.Context, bookID uuid.UUID, userID *uuid.UUID, limit, offset int) ([]reviewsDomain.Review, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&reviewsDomain.Review{}).
		Where("book_id = ? AND is_hidden = ?", bookID, false).
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

	likedByMeExpr := "false"
	args := []interface{}{bookID, limit, offset}
	if userID != nil {
		likedByMeExpr = "EXISTS (SELECT 1 FROM review_likes rl WHERE rl.review_id = r.id AND rl.user_id = ?)"
		args = append([]interface{}{*userID}, args...)
	}

	var rows []reviewListRow
	sql := fmt.Sprintf(`
		SELECT
			r.id,
			r.book_id,
			r.user_id,
			r.rating,
			r.content,
			r.likes_count,
			r.is_hidden,
			r.created_at,
			r.updated_at,
			u.name AS user_name,
			u.avatar_url AS user_avatar_url,
			%s AS liked_by_me
		FROM reviews r
		JOIN users u ON u.id = r.user_id
		WHERE r.book_id = ? AND r.is_hidden = false
		ORDER BY r.created_at DESC
		LIMIT ? OFFSET ?
	`, likedByMeExpr)
	err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	reviews := make([]reviewsDomain.Review, 0, len(rows))
	for _, row := range rows {
		reviews = append(reviews, reviewsDomain.Review{
			ID:         row.ID,
			BookID:     row.BookID,
			UserID:     row.UserID,
			Rating:     row.Rating,
			Content:    row.Content,
			LikesCount: row.LikesCount,
			IsHidden:   row.IsHidden,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
			Author: &usersDomain.User{
				ID:        row.UserID,
				Name:      row.UserName,
				AvatarURL: row.UserAvatarURL,
			},
			LikedByMe: row.LikedByMe,
		})
	}

	return reviews, total, nil
}

// GetByID returns a review by ID.
func (r *GormRepository) GetByID(ctx context.Context, reviewID uuid.UUID) (*reviewsDomain.Review, error) {
	var review reviewsDomain.Review
	if err := r.db.WithContext(ctx).First(&review, "id = ?", reviewID).Error; err != nil {
		return nil, err
	}
	return &review, nil
}

// GetByUserAndBook returns a review by user and book.
func (r *GormRepository) GetByUserAndBook(ctx context.Context, userID, bookID uuid.UUID) (*reviewsDomain.Review, error) {
	var review reviewsDomain.Review
	if err := r.db.WithContext(ctx).First(&review, "book_id = ? AND user_id = ?", bookID, userID).Error; err != nil {
		return nil, err
	}
	return &review, nil
}

// Create inserts a review, upserts the user's rating and recalculates book aggregates.
func (r *GormRepository) Create(ctx context.Context, review *reviewsDomain.Review) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(review).Error; err != nil {
			return err
		}
		if err := r.upsertRatingInTx(tx, review.BookID, review.UserID, review.Rating); err != nil {
			return err
		}
		return r.recalcInTx(tx, review.BookID)
	})
}

// Update updates a review and recalculates book aggregates.
func (r *GormRepository) Update(ctx context.Context, review *reviewsDomain.Review) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("rating", "content").Save(review).Error; err != nil {
			return err
		}
		if err := r.upsertRatingInTx(tx, review.BookID, review.UserID, review.Rating); err != nil {
			return err
		}
		return r.recalcInTx(tx, review.BookID)
	})
}

// Delete removes a review and recalculates book aggregates.
func (r *GormRepository) Delete(ctx context.Context, reviewID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var review reviewsDomain.Review
		if err := tx.First(&review, "id = ?", reviewID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&reviewsDomain.Review{ID: reviewID}).Error; err != nil {
			return err
		}
		return r.recalcInTx(tx, review.BookID)
	})
}

// ToggleLike toggles a like on a review. When a new like is created for
// someone else's review, a review_like notification for the review's author
// is upserted in the same transaction (see upsertNotificationInTx) - this
// keeps the notification write atomic with the like it describes.
func (r *GormRepository) ToggleLike(ctx context.Context, reviewID, userID, revieweeID uuid.UUID) (bool, int, error) {
	var liked bool
	var likesCount int

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exists bool
		err := tx.Raw(`
			SELECT EXISTS(
				SELECT 1 FROM review_likes WHERE review_id = ? AND user_id = ?
			)
		`, reviewID, userID).Scan(&exists).Error
		if err != nil {
			return err
		}

		if exists {
			if err := tx.Where("review_id = ? AND user_id = ?", reviewID, userID).Delete(&reviewsDomain.ReviewLike{}).Error; err != nil {
				return err
			}
			liked = false
		} else {
			if err := tx.Create(&reviewsDomain.ReviewLike{ReviewID: reviewID, UserID: userID}).Error; err != nil {
				return err
			}
			liked = true

			// No self-notify: a user liking their own review never creates
			// a notification.
			if revieweeID != userID {
				if err := r.upsertNotificationInTx(tx, revieweeID, userID, reviewID); err != nil {
					return err
				}
			}
		}

		var agg struct {
			LikesCount int `gorm:"column:likes_count"`
		}
		err = tx.Raw(`
			UPDATE reviews
			SET likes_count = (SELECT COUNT(*) FROM review_likes WHERE review_id = ?)
			WHERE id = ?
			RETURNING likes_count
		`, reviewID, reviewID).Scan(&agg).Error
		if err != nil {
			return err
		}
		likesCount = agg.LikesCount
		return nil
	})

	return liked, likesCount, err
}

// BookExists checks whether a published book exists (status = published).
func (r *GormRepository) BookExists(ctx context.Context, bookID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("books").
		Where("id = ? AND status = ?", bookID, catalogDomain.BookStatusPublished).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *GormRepository) upsertRatingInTx(tx *gorm.DB, bookID, userID uuid.UUID, value int) error {
	return tx.Exec(`
		INSERT INTO ratings (id, book_id, user_id, value, created_at, updated_at)
		VALUES (gen_random_uuid(), ?, ?, ?, NOW(), NOW())
		ON CONFLICT (book_id, user_id) DO UPDATE SET
			value = EXCLUDED.value,
			updated_at = NOW()
	`, bookID, userID, value).Error
}

// upsertNotificationInTx upserts a review_like notification for the review's
// author. The notifications table's UNIQUE (actor_id, review_id, type)
// constraint makes this idempotent: a repeated like/unlike/like cycle by the
// same actor on the same review reuses (refreshes to unread with a fresh
// timestamp) the existing row instead of creating a duplicate. This module
// deliberately writes to the notifications table via raw SQL rather than
// importing the notifications package, mirroring this function's siblings
// (upsertRatingInTx/recalcInTx) that already write directly to ratings/books.
func (r *GormRepository) upsertNotificationInTx(tx *gorm.DB, recipientID, actorID, reviewID uuid.UUID) error {
	return tx.Exec(`
		INSERT INTO notifications (id, user_id, actor_id, type, review_id, is_read, created_at)
		VALUES (gen_random_uuid(), ?, ?, 'review_like', ?, false, NOW())
		ON CONFLICT (actor_id, review_id, type) DO UPDATE SET
			is_read = false,
			created_at = NOW()
	`, recipientID, actorID, reviewID).Error
}

func (r *GormRepository) recalcInTx(tx *gorm.DB, bookID uuid.UUID) error {
	return tx.Exec(`
		UPDATE books
		SET rating = COALESCE(ROUND((SELECT AVG(value) FROM ratings WHERE book_id = ?)::numeric, 2), 0),
		    ratings_count = (SELECT COUNT(*) FROM ratings WHERE book_id = ?),
		    reviews_count = (SELECT COUNT(*) FROM reviews WHERE book_id = ? AND is_hidden = false)
		WHERE id = ?
	`, bookID, bookID, bookID, bookID).Error
}
