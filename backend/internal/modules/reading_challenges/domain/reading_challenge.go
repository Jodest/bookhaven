package domain

import (
	"time"

	"github.com/google/uuid"
)

// ReadingChallenge represents a user's annual reading goal.
type ReadingChallenge struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	Year      int       `gorm:"not null"`
	Goal      int       `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
