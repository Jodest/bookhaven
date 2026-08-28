package domain

import (
	"time"

	"github.com/google/uuid"
)

type Author struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name        string    `gorm:"not null"`
	Slug        string    `gorm:"uniqueIndex;not null"`
	Biography   *string
	PhotoURL    *string
	BirthDate   *string   `gorm:"type:date"`
	DeathDate   *string   `gorm:"type:date"`
	Books       []Book    `gorm:"many2many:book_authors;"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
