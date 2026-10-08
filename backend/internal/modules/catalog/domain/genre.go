package domain

import (
	"time"

	"github.com/google/uuid"
)

// Genre moderation statuses (see docs/features/genre-status.md). Only
// published genres are visible on the public site; pending/rejected genres
// are omitted from every public read.
const (
	GenreStatusPending   = "pending"
	GenreStatusPublished = "published"
	GenreStatusRejected  = "rejected"
)

type Genre struct {
	ID   uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name string    `gorm:"not null"`
	Slug string    `gorm:"uniqueIndex;not null"`
	// Status is one of GenreStatus*. Go code sets it explicitly on every
	// create; the DB default (pending) only covers raw SQL inserts.
	Status string `gorm:"type:varchar(20);not null;default:'pending'"`
	// WikidataQID is the genre's bare Wikidata QID (^Q[0-9]+$), unique when
	// set. The explicit column tag is required: GORM naming would otherwise
	// produce wikidata_q_id.
	WikidataQID *string `gorm:"column:wikidata_qid"`
	// MergedIntoID, when set, makes this row an alias of the target genre
	// (docs/features/genre-moderation-queue.md): resolvers that match it by
	// slug or QID follow it to the target. An alias is always rejected (DB
	// CHECK) and hidden from every admin list.
	MergedIntoID *uuid.UUID `gorm:"type:uuid;column:merged_into_id"`
	Books        []Book     `gorm:"many2many:book_genres;"`
	CreatedAt    time.Time
}

// IsPublished reports whether the genre is publicly visible.
func (g Genre) IsPublished() bool {
	return g.Status == GenreStatusPublished
}

// IsAlias reports whether the genre was merged into another genre.
func (g Genre) IsAlias() bool {
	return g.MergedIntoID != nil
}

// IsValidGenreStatus reports whether s is one of the known statuses.
func IsValidGenreStatus(s string) bool {
	switch s {
	case GenreStatusPending, GenreStatusPublished, GenreStatusRejected:
		return true
	}
	return false
}

// PublishedGenres returns only the published genres of gs, preserving
// order. Public mappers use it so a non-published genre never appears in a
// public book payload. It always returns a non-nil slice.
func PublishedGenres(gs []Genre) []Genre {
	out := make([]Genre, 0, len(gs))
	for _, g := range gs {
		if g.IsPublished() {
			out = append(out, g)
		}
	}
	return out
}
