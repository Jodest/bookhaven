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
	Language      string    `gorm:"default:'ru'"`
	Rating        float64   `gorm:"type:decimal(3,2);default:0"`
	RatingsCount  int       `gorm:"default:0"`
	ReviewsCount  int       `gorm:"default:0"`
	IsHidden      bool      `gorm:"default:false"`
	Authors       []Author  `gorm:"many2many:book_authors;"`
	Genres        []Genre   `gorm:"many2many:book_genres;"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
