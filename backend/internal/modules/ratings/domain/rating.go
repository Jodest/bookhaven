package domain

import (
	"time"

	"github.com/google/uuid"
)

// Rating represents a user's numeric rating (1-5) for a book.
type Rating struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	BookID    uuid.UUID `gorm:"type:uuid;not null"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	Value     int       `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
