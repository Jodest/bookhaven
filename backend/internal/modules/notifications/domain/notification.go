package domain

import (
	"time"

	"github.com/google/uuid"
)

// TypeReviewLike is the only notification type in this increment: another
// user liked the recipient's review. Future event types (e.g. a new
// follower) should get their own TypeXxx constant and their own nullable FK
// column on this table rather than a generic/JSON payload - see
// docs/features/review-like-notifications.md's extensibility note.
const TypeReviewLike = "review_like"

// Notification represents an in-app notification for a recipient user.
type Notification struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null"` // recipient
	ActorID   uuid.UUID  `gorm:"type:uuid;not null"` // who triggered it
	Type      string     `gorm:"type:varchar(32);not null"`
	ReviewID  *uuid.UUID `gorm:"type:uuid"`
	IsRead    bool       `gorm:"not null;default:false"`
	CreatedAt time.Time

	// Joined display fields populated by the repository's raw-SQL join
	// (same gorm:"-" pattern as reviewsDomain.Review's Author/BookTitle) -
	// not real columns on this table.
	ActorName      *string    `gorm:"-"`
	ActorAvatarURL *string    `gorm:"-"`
	BookID         *uuid.UUID `gorm:"-"`
	BookSlug       *string    `gorm:"-"`
	BookTitle      *string    `gorm:"-"`
}
