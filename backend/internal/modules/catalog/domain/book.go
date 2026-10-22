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

	// Cover copy state (#489). CoverURL stays the source link; these columns
	// describe the local copy made from CoverSourceURL. They are read-only for
	// GORM ("->"): a full-row Save of a Book never writes them, so an admin
	// save cannot overwrite a copy the worker stored meanwhile. Only the
	// covers repository writes them with raw guarded UPDATEs.
	CoverSourceURL     *string    `gorm:"->"`
	CoverPath          *string    `gorm:"->"`
	CoverStatus        *string    `gorm:"->"`
	CoverError         *string    `gorm:"->"`
	CoverAttempts      int        `gorm:"->"`
	CoverNextAttemptAt *time.Time `gorm:"->"`
	CoverStoredAt      *time.Time `gorm:"->"`
	CoverCheckedAt     *time.Time `gorm:"->"`
}

// Cover copy statuses stored in books.cover_status.
const (
	CoverStatusPending = "pending"
	CoverStatusStored  = "stored"
	CoverStatusFailed  = "failed"
	CoverStatusAbsent  = "absent"
)

// CoverPublicPrefix is the public path prefix of self-hosted covers.
const CoverPublicPrefix = "/covers/"

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// DisplayCoverURL resolves the cover link to show publicly from the source
// link and the copy state. A copy is valid only for the exact link it was
// made from: an empty link gives nil, a stored copy of the same link gives
// its /covers path, an absent copy of the same link gives nil, and every
// other case falls back to the source link.
func DisplayCoverURL(coverURL, sourceURL, path *string, status *string) *string {
	cu := strVal(coverURL)
	if cu == "" {
		return nil
	}
	if strVal(sourceURL) == cu {
		switch strVal(status) {
		case CoverStatusStored:
			if p := strVal(path); p != "" {
				v := CoverPublicPrefix + p
				return &v
			}
		case CoverStatusAbsent:
			return nil
		}
	}
	v := cu
	return &v
}

// DisplayCoverURL is the link the public API shows for this book.
func (b Book) DisplayCoverURL() *string {
	return DisplayCoverURL(b.CoverURL, b.CoverSourceURL, b.CoverPath, b.CoverStatus)
}

// HasStoredCover reports whether a copy of the current cover link is stored.
func (b Book) HasStoredCover() bool {
	cu := strVal(b.CoverURL)
	return cu != "" && strVal(b.CoverSourceURL) == cu &&
		strVal(b.CoverStatus) == CoverStatusStored && strVal(b.CoverPath) != ""
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
