package domain

import (
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// ShelfType represents the type of a shelf.
type ShelfType string

const (
	ShelfTypeWantToRead ShelfType = "want_to_read"
	ShelfTypeReading    ShelfType = "reading"
	ShelfTypeRead       ShelfType = "read"
	ShelfTypeAbandoned  ShelfType = "abandoned"
	ShelfTypeFavorite   ShelfType = "favorite"
	ShelfTypeCustom     ShelfType = "custom"
)

// Shelf represents a user's bookshelf.
type Shelf struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID     uuid.UUID `gorm:"type:uuid;not null"`
	Name       string    `gorm:"not null"`
	Type       ShelfType `gorm:"type:shelf_type;not null;default:'custom'"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	BooksCount int64                `gorm:"-"`
	HasBook    bool                 `gorm:"-"`
	Books      []catalogDomain.Book `gorm:"many2many:shelf_items;"`
}

// ShelfItem represents a book placed on a shelf.
type ShelfItem struct {
	ShelfID uuid.UUID `gorm:"type:uuid;primaryKey"`
	BookID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	AddedAt time.Time `gorm:"not null;default:now()"`
}
