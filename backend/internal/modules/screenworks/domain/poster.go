package domain

import "errors"

// Poster sources (CHECK constraint of migration 000043). NULL means no poster
// and the sync may fill it; PosterSourceRemoved means an admin removed an
// automatic poster and the sync never fills it again.
const (
	PosterSourceManual  = "manual"
	PosterSourceCommons = "commons"
	PosterSourceTMDB    = "tmdb"
	PosterSourceRemoved = "removed"
)

// PostersURLPrefix is the public route of uploaded posters.
const PostersURLPrefix = "/posters/"

// Poster upload errors.
var (
	// ErrPosterTooLarge: the uploaded file exceeds the size limit (413).
	ErrPosterTooLarge = errors.New("poster is too large")
	// ErrPosterType: the uploaded file is not a JPEG, PNG or WebP image (422).
	ErrPosterType = errors.New("poster must be a JPEG, PNG or WebP image")
	// ErrPostersUnavailable: no poster store configured (503).
	ErrPostersUnavailable = errors.New("poster storage is not configured")
)

// PosterInput is a poster found by the sync for a work (Commons or TMDB).
type PosterInput struct {
	Source     string
	URL        string
	Author     *string
	License    *string
	LicenseURL *string
	SourceURL  *string
}

// PosterColumns are the stored poster columns of a screen work. It is
// embedded into the read models.
type PosterColumns struct {
	PosterSource     *string `gorm:"column:poster_source"`
	PosterURL        *string `gorm:"column:poster_url"`
	PosterPath       *string `gorm:"column:poster_path"`
	PosterAuthor     *string `gorm:"column:poster_author"`
	PosterLicense    *string `gorm:"column:poster_license"`
	PosterLicenseURL *string `gorm:"column:poster_license_url"`
	PosterSourceURL  *string `gorm:"column:poster_source_url"`
}

// Poster is the API poster object.
type Poster struct {
	URL        string
	Source     string
	Author     *string
	License    *string
	LicenseURL *string
	SourceURL  *string
}

// Poster returns the poster to show or nil (no poster or removed).
func (c PosterColumns) Poster() *Poster {
	if c.PosterSource == nil {
		return nil
	}
	p := &Poster{
		Source: *c.PosterSource, Author: c.PosterAuthor, License: c.PosterLicense,
		LicenseURL: c.PosterLicenseURL, SourceURL: c.PosterSourceURL,
	}
	switch *c.PosterSource {
	case PosterSourceManual:
		if c.PosterPath == nil || *c.PosterPath == "" {
			return nil
		}
		p.URL = PostersURLPrefix + *c.PosterPath
	case PosterSourceCommons, PosterSourceTMDB:
		if c.PosterURL == nil || *c.PosterURL == "" {
			return nil
		}
		p.URL = *c.PosterURL
	default:
		return nil
	}
	return p
}

// PosterRemoved reports whether an admin removed an automatic poster.
func (c PosterColumns) PosterRemoved() bool {
	return c.PosterSource != nil && *c.PosterSource == PosterSourceRemoved
}
