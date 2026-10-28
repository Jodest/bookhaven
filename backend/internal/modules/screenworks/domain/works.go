package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Manual editing of screen works (openspec change add-screen-works-admin-editing).

// Field limits (match the screen_works columns).
const (
	MaxTitleLen    = 500
	MaxDirectorLen = 500
	MaxExternalLen = 20
	MinYear        = 1880
	MaxYear        = 2100
)

var (
	// ErrWorkNotFound: no such screen work.
	ErrWorkNotFound = errors.New("screen work not found")
	// ErrAlreadyLinked: the work is already linked to the owner (hidden or not).
	ErrAlreadyLinked = errors.New("screen work is already linked")
	// ErrLinkNotManual: only manual links can be deleted.
	ErrLinkNotManual = errors.New("only manual links can be deleted; hide the link instead")
	// ErrWorkHasLinks: a work with any link cannot be deleted.
	ErrWorkHasLinks = errors.New("screen work has links")
	// ErrQIDTaken: another work already has this QID.
	ErrQIDTaken = errors.New("screen work with this qid already exists")
	// ErrQIDImmutable: the QID of a work cannot be changed or cleared.
	ErrQIDImmutable = errors.New("qid cannot be changed")
	// ErrLinkRefRequired: neither or both of screen_work_id and work were given.
	ErrLinkRefRequired = errors.New("either screen_work_id or work is required")
	// ErrInvalidRelation: relation is not biopic or documentary.
	ErrInvalidRelation = errors.New("invalid relation")
	// ErrLinkPatchEmpty: a link patch with neither hidden nor relation.
	ErrLinkPatchEmpty = errors.New("hidden or relation is required")
)

// ErrDuplicateExternalID: a stored work already has this IMDb or Kinopoisk id.
type ErrDuplicateExternalID struct {
	Field  string // "imdb id" | "kinopoisk id"
	WorkID uuid.UUID
}

func (e *ErrDuplicateExternalID) Error() string {
	return "screen work with this " + e.Field + " already exists"
}

// ValidationError is a rejected work field (422).
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// IsValidKind reports whether k is a known work kind.
func IsValidKind(k string) bool {
	switch k {
	case KindFilm, KindTVFilm, KindSeries, KindAnimatedFilm, KindAnimatedSeries, KindTeleplay:
		return true
	}
	return false
}

// IsValidRelation reports whether r is biopic or documentary.
func IsValidRelation(r string) bool {
	return r == RelationBiopic || r == RelationDocumentary
}

// WorkFields is the editable data of a screen work. Optional values are nil
// when empty.
type WorkFields struct {
	Title         string  `json:"title"`
	OriginalTitle *string `json:"original_title"`
	Kind          string  `json:"kind"`
	Year          *int    `json:"year"`
	Director      *string `json:"director"`
	IMDbID        *string `json:"imdb_id"`
	KinopoiskID   *string `json:"kinopoisk_id"`
	QID           *string `json:"qid"`
}

var (
	imdbRe      = regexp.MustCompile(`^tt[0-9]+$`)
	kinopoiskRe = regexp.MustCompile(`^[0-9]+$`)
	qidRe       = regexp.MustCompile(`^Q[0-9]+$`)
)

func trimOpt(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	if t == "" {
		return nil
	}
	return &t
}

// NormalizeWorkFields trims and validates admin-entered work data. It returns
// a *ValidationError with the exact message on the first violation.
func NormalizeWorkFields(f WorkFields) (WorkFields, error) {
	bad := func(msg string) (WorkFields, error) { return WorkFields{}, &ValidationError{Msg: msg} }

	f.Title = strings.TrimSpace(f.Title)
	f.Kind = strings.TrimSpace(f.Kind)
	f.OriginalTitle = trimOpt(f.OriginalTitle)
	f.Director = trimOpt(f.Director)
	f.IMDbID = trimOpt(f.IMDbID)
	f.KinopoiskID = trimOpt(f.KinopoiskID)
	f.QID = trimOpt(f.QID)

	if f.Title == "" {
		return bad("title is required")
	}
	if utf8.RuneCountInString(f.Title) > MaxTitleLen {
		return bad("title is too long")
	}
	if f.OriginalTitle != nil {
		if utf8.RuneCountInString(*f.OriginalTitle) > MaxTitleLen {
			return bad("original title is too long")
		}
		if *f.OriginalTitle == f.Title {
			f.OriginalTitle = nil
		}
	}
	if !IsValidKind(f.Kind) {
		return bad("invalid kind")
	}
	if f.Year != nil && (*f.Year < MinYear || *f.Year > MaxYear) {
		return bad("year must be between 1880 and 2100")
	}
	if f.Director != nil && utf8.RuneCountInString(*f.Director) > MaxDirectorLen {
		return bad("director is too long")
	}
	if f.IMDbID != nil && (len(*f.IMDbID) > MaxExternalLen || !imdbRe.MatchString(*f.IMDbID)) {
		return bad("invalid imdb id")
	}
	if f.KinopoiskID != nil && (len(*f.KinopoiskID) > MaxExternalLen || !kinopoiskRe.MatchString(*f.KinopoiskID)) {
		return bad("invalid kinopoisk id")
	}
	if f.QID != nil {
		q := strings.ToUpper(*f.QID)
		if !qidRe.MatchString(q) {
			return bad("invalid qid")
		}
		f.QID = &q
	}
	return f, nil
}

// WorkRef is either an existing work (WorkID) or a new one (Fields).
type WorkRef struct {
	WorkID *uuid.UUID
	Fields *WorkFields
}

// WorkFilter is the admin work list query.
type WorkFilter struct {
	Search   string
	Unlinked bool
	Page     int
	Limit    int
}

// WorkListItem is one row of the admin work list.
type WorkListItem struct {
	ID               uuid.UUID  `gorm:"column:id"`
	Title            string     `gorm:"column:title"`
	OriginalTitle    *string    `gorm:"column:original_title"`
	Kind             string     `gorm:"column:kind"`
	Year             *int       `gorm:"column:year"`
	Director         *string    `gorm:"column:director"`
	IMDbID           *string    `gorm:"column:imdb_id"`
	KinopoiskID      *string    `gorm:"column:kinopoisk_id"`
	QID              *string    `gorm:"column:qid"`
	ManuallyEditedAt *time.Time `gorm:"column:manually_edited_at"`
	BookCount        int        `gorm:"column:book_count"`
	AuthorCount      int        `gorm:"column:author_count"`
	PosterColumns
}

// WorkBookLink is a book linked to a work.
type WorkBookLink struct {
	ID     uuid.UUID `gorm:"column:id"`
	Title  string    `gorm:"column:title"`
	Status string    `gorm:"column:status"`
	Hidden bool      `gorm:"column:hidden"`
	Source string    `gorm:"column:source"`
}

// WorkAuthorLink is an author linked to a work.
type WorkAuthorLink struct {
	ID       uuid.UUID `gorm:"column:id"`
	Name     string    `gorm:"column:name"`
	Status   string    `gorm:"column:status"`
	Hidden   bool      `gorm:"column:hidden"`
	Source   string    `gorm:"column:source"`
	Relation string    `gorm:"column:relation"`
}

// WorkDetail is a work with its links.
type WorkDetail struct {
	WorkListItem
	Books   []WorkBookLink
	Authors []WorkAuthorLink
}
