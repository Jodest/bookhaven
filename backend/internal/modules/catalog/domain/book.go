package domain

import (
	"time"

	"github.com/google/uuid"
)

type Book struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Title         string    `gorm:"not null"`
	Slug          string    `gorm:"uniqueIndex;not null"`
	Description   *string
	CoverURL      *string
	ISBN          *string
	PublishedYear *int
	PageCount     *int
	Language      string   `gorm:"default:'ru'"`
	Rating        float64  `gorm:"type:decimal(3,2);default:0"`
	RatingsCount  int      `gorm:"default:0"`
	ReviewsCount  int      `gorm:"default:0"`
	IsHidden      bool     `gorm:"default:false"`
	Authors       []Author `gorm:"many2many:book_authors;"`
	Genres        []Genre  `gorm:"many2many:book_genres;"`
	// ExternalSource/ExternalID identify the origin record for books
	// created via an automated harvest (e.g. "wikidata" + a QID), used as
	// an idempotency key to avoid re-importing the same record. Nil for
	// books created through the regular admin UI/API.
	ExternalSource *string
	ExternalID     *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
