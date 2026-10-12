package domain

import (
	"time"

	"github.com/google/uuid"
)

// Book moderation statuses (see docs/features/book-status-model.md). Only
// published books are visible on the public site; pending/rejected books
// are omitted from every public read.
const (
	BookStatusPending   = "pending"
	BookStatusPublished = "published"
	BookStatusRejected  = "rejected"
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
	Language      string  `gorm:"default:'ru'"`
	Rating        float64 `gorm:"type:decimal(3,2);default:0"`
	RatingsCount  int     `gorm:"default:0"`
	ReviewsCount  int     `gorm:"default:0"`
	// Status is one of BookStatus*. Go code sets it explicitly on every
	// create; the DB default (pending) only covers raw SQL inserts.
	Status string `gorm:"type:varchar(20);not null;default:'pending'"`
	// PublishedAt is the first time the book became published (#245); nil
	// until then. Set once via MarkPublished or a COALESCE update and never
	// cleared: unpublishing and republishing keep the original time.
	PublishedAt *time.Time
	Authors     []Author `gorm:"many2many:book_authors;"`
	Genres      []Genre  `gorm:"many2many:book_genres;"`
	// ExternalSource/ExternalID identify the origin record for books
	// created via an automated harvest (e.g. "wikidata" + a QID), used as
	// an idempotency key to avoid re-importing the same record. Nil for
	// books created through the regular admin UI/API.
	ExternalSource *string
	ExternalID     *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsPublished reports whether the book is publicly visible.
func (b Book) IsPublished() bool {
	return b.Status == BookStatusPublished
}

// MarkPublished records now as the first-publication time when the book is
// published and has none yet. An existing time is kept.
func (b *Book) MarkPublished(now time.Time) {
	if b.Status == BookStatusPublished && b.PublishedAt == nil {
		t := now
		b.PublishedAt = &t
	}
}

// IsValidBookStatus reports whether s is one of the known statuses.
func IsValidBookStatus(s string) bool {
	switch s {
	case BookStatusPending, BookStatusPublished, BookStatusRejected:
		return true
	}
	return false
}
