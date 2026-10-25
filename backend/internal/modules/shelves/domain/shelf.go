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
	ID     uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID uuid.UUID `gorm:"type:uuid;not null"`
	Name   string    `gorm:"not null"`
	Type   ShelfType `gorm:"type:shelf_type;not null;default:'custom'"`
	// Description is an optional free-text description (custom shelves only).
	Description string `gorm:"type:text;not null"`
	// IsPublic is false for private custom shelves. Default shelves are always public.
	// No gorm default tag on purpose: GORM would replace a false zero value with the
	// default on create. The DB default (true) covers raw inserts.
	IsPublic   bool `gorm:"not null"`
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

// BookRead is one reading attempt in the reading journal (table book_reads).
// An attempt is open (no finished_at/abandoned_at), finished or abandoned.
type BookRead struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID      uuid.UUID `gorm:"type:uuid;not null"`
	BookID      uuid.UUID `gorm:"type:uuid;not null"`
	StartedAt   *time.Time
	FinishedAt  *time.Time
	AbandonedAt *time.Time
	CreatedAt   time.Time
}

// TableName binds BookRead to book_reads.
func (BookRead) TableName() string { return "book_reads" }

// ReadSummary is the caller's reading summary of one book.
type ReadSummary struct {
	Count      int64
	LastReadAt *time.Time
	// Rereading is true while the book is being read again, i.e. there is an
	// open attempt and at least one finished one.
	Rereading bool
}

// ReadCount is the finished-readings count of one book with the latest date.
type ReadCount struct {
	Count      int64
	LastReadAt *time.Time
}
