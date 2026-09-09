package domain

import (
	"time"

	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
)

// Review represents a user's book review with a mandatory rating.
type Review struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	BookID     uuid.UUID `gorm:"type:uuid;not null"`
	UserID     uuid.UUID `gorm:"type:uuid;not null"`
	Rating     int       `gorm:"not null"`
	Content    string    `gorm:"not null"`
	LikesCount int       `gorm:"default:0"`
	IsHidden   bool      `gorm:"default:false"`
	CreatedAt  time.Time
	UpdatedAt  time.Time

	Author    *usersDomain.User `gorm:"-"`
	LikedByMe bool              `gorm:"-"`
	BookTitle string            `gorm:"-"`
}

// ReviewLike represents a like on a review.
type ReviewLike struct {
	ReviewID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time
}
