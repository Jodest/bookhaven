// Package domain holds the screen works model (films, series, cartoons,
// teleplays) and their links to books (openspec screen-works domain).
package domain

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Kinds of a screen work (CHECK constraint of migration 000029).
const (
	KindFilm           = "film"
	KindTVFilm         = "tv_film"
	KindSeries         = "series"
	KindAnimatedFilm   = "animated_film"
	KindAnimatedSeries = "animated_series"
	KindTeleplay       = "teleplay"
)

// Link sources.
const (
	SourceWikidata = "wikidata"
	SourceManual   = "manual"
)

// ExternalSourceWikidata is the external_source of works synced from Wikidata.
const ExternalSourceWikidata = "wikidata"

// Sync outcomes of one book.
const (
	OutcomeAdded     = "added"
	OutcomeUnchanged = "unchanged"
	OutcomeSkipped   = "skipped"
	OutcomeError     = "error"
)

// Sync notes.
const (
	NoteNoQID               = "no_qid"
	NoteWikidataUnavailable = "wikidata_unavailable"
)

var (
	ErrBookNotFound = errors.New("book not found")
	ErrLinkNotFound = errors.New("adaptation link not found")
)

// WorkInput is an adaptation to store: a work found in Wikidata. Optional
// values are nil when unknown.
type WorkInput struct {
	QID           string
	Title         string
	OriginalTitle *string
	Kind          string
	Year          *int
	Director      *string
	IMDbID        *string
	KinopoiskID   *string
}

// BookRef is the read-only part of a book the sync needs.
type BookRef struct {
	ID             uuid.UUID
	Title          string
	Status         string
	ExternalSource *string
	ExternalID     *string
}

// WikidataQID returns the book's Wikidata QID or "".
func (b BookRef) WikidataQID() string {
	if b.ExternalSource == nil || b.ExternalID == nil || *b.ExternalSource != ExternalSourceWikidata {
		return ""
	}
	return strings.TrimSpace(*b.ExternalID)
}

// LinkedWork is a screen work as linked to one book.
type LinkedWork struct {
	ID            uuid.UUID `gorm:"column:id"`
	Title         string    `gorm:"column:title"`
	OriginalTitle *string   `gorm:"column:original_title"`
	Kind          string    `gorm:"column:kind"`
	Year          *int      `gorm:"column:year"`
	Director      *string   `gorm:"column:director"`
	IMDbID        *string   `gorm:"column:imdb_id"`
	KinopoiskID   *string   `gorm:"column:kinopoisk_id"`
	ExternalID    *string   `gorm:"column:external_id"`
	Hidden        bool      `gorm:"column:hidden"`
	Source        string    `gorm:"column:source"`
}

// IMDbURL is the public IMDb link or nil.
func (w LinkedWork) IMDbURL() *string {
	if w.IMDbID == nil || *w.IMDbID == "" {
		return nil
	}
	u := "https://www.imdb.com/title/" + *w.IMDbID + "/"
	return &u
}

// KinopoiskURL is the public Kinopoisk link or nil.
func (w LinkedWork) KinopoiskURL() *string {
	if w.KinopoiskID == nil || *w.KinopoiskID == "" {
		return nil
	}
	u := "https://www.kinopoisk.ru/film/" + *w.KinopoiskID + "/"
	return &u
}
