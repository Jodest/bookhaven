package domain

import (
	"time"

	"github.com/google/uuid"
)

type Genre struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name      string    `gorm:"not null"`
	Slug      string    `gorm:"uniqueIndex;not null"`
	Books     []Book    `gorm:"many2many:book_genres;"`
	CreatedAt time.Time
}
