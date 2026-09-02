package domain

import (
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// List represents a user's book list.
type List struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID      uuid.UUID `gorm:"type:uuid;not null"`
	Title       string    `gorm:"not null;size:255"`
	Description string    `gorm:"type:text"`
	IsPublic    bool      `gorm:"not null;default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	BooksCount  int64                `gorm:"-"`
	Books       []catalogDomain.Book `gorm:"many2many:list_items;"`
}

// ListItem represents a book placed on a list.
type ListItem struct {
	ListID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	BookID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Position int       `gorm:"not null;default:0"`
	AddedAt  time.Time `gorm:"not null;default:now()"`
}
