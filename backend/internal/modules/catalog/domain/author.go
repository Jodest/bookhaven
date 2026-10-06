package domain

import (
	"time"

	"github.com/google/uuid"
)

// Author moderation statuses (see docs/features/author-status.md). Only
// published authors are first-class public entities; pending/rejected
// authors are embedded in public book payloads by name only.
const (
	AuthorStatusPending   = "pending"
	AuthorStatusPublished = "published"
	AuthorStatusRejected  = "rejected"
)

// AuthorExternalSourceWikidata is the only external_source value in use;
// external_id then holds a bare QID (^Q[0-9]+$).
const AuthorExternalSourceWikidata = "wikidata"

type Author struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name      string    `gorm:"not null"`
	Slug      string    `gorm:"uniqueIndex;not null"`
	Biography *string
	PhotoURL  *string
	BirthDate *time.Time `gorm:"type:date"`
	DeathDate *time.Time `gorm:"type:date"`
	// Status is one of AuthorStatus*. Service code sets it explicitly on
	// every create; the DB default (published) only covers seed/raw SQL.
	Status string `gorm:"type:varchar(20);not null;default:'published'"`
	// ExternalSource/ExternalID identify the author's external identity
	// (currently only Wikidata + QID). Nil for manual/seed/legacy authors.
	ExternalSource *string
	ExternalID     *string
	Books          []Book `gorm:"many2many:book_authors;"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsPublished reports whether the author is publicly visible.
func (a Author) IsPublished() bool {
	return a.Status == AuthorStatusPublished
}

// IsValidAuthorStatus reports whether s is one of the known statuses.
func IsValidAuthorStatus(s string) bool {
	switch s {
	case AuthorStatusPending, AuthorStatusPublished, AuthorStatusRejected:
		return true
	}
	return false
}
