package domain

import (
	"errors"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// Collection kinds. The kind is chosen on creation and never changes.
const (
	// KindManual collections hold an admin-ordered list of books.
	KindManual = "manual"
	// KindAuto collections are ranked from site data by a Rule and stored as
	// a snapshot recomputed by the worker.
	KindAuto = "auto"
)

const (
	// MaxSize is the largest number of ranked places in a collection.
	MaxSize = 1000
	// MaxBooks is the most books (ranked + reserve) a collection may hold.
	MaxBooks = 1000
)

// Item states in the admin view.
const (
	StateRanked  = "ranked"
	StateReserve = "reserve"
	StateHidden  = "hidden"
)

var (
	ErrCollectionNotFound = errors.New("collection not found")
	ErrBookNotFound       = errors.New("book not found")
	ErrBookNotInList      = errors.New("book not in collection")
	ErrBookAlreadyIn      = errors.New("book already in collection")
	ErrBookRejected       = errors.New("book is rejected")
	ErrCollectionFull     = errors.New("collection is full")
	ErrSlugTaken          = errors.New("slug already exists")
	ErrInvalidGenre       = errors.New("invalid genre_id")
	ErrInvalidPosition    = errors.New("invalid position")
	ErrTooManyLines       = errors.New("too many lines (max 500)")

	ErrCollectionAutomatic = errors.New("collection is automatic")
	ErrNotAutomatic        = errors.New("collection is not automatic")
)

// Collection is a BookHaven-owned ranked collection of catalog books.
type Collection struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Slug        string     `gorm:"not null;size:100;uniqueIndex"`
	Title       string     `gorm:"not null;size:255"`
	Description string     `gorm:"type:text;not null;default:''"`
	Kind        string     `gorm:"not null;size:20;default:'manual'"`
	Size        int        `gorm:"not null"`
	Source      *string    `gorm:"size:255"`
	SourceURL   *string    `gorm:"size:500"`
	GenreID     *uuid.UUID `gorm:"type:uuid"`
	IsPublished bool       `gorm:"not null;default:false"`
	PublishedAt *time.Time

	// Rule columns, set only for kind auto.
	RuleMetric     *string
	RulePeriodDays *int
	RuleMinRatings *int
	RuleConfidence *int
	// ComputedAt is the time of the current snapshot; BaselineAt the time of
	// the snapshot that CollectionItem.PreviousPosition refers to.
	ComputedAt *time.Time
	BaselineAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time

	Genre *catalogDomain.Genre `gorm:"foreignKey:GenreID"`
}

func (Collection) TableName() string { return "curated_collections" }

// IsAuto reports whether the collection is built by a rule.
func (c Collection) IsAuto() bool { return c.Kind == KindAuto }

// CollectionItem is a book in the admin-ordered list of a collection.
type CollectionItem struct {
	CollectionID uuid.UUID `gorm:"type:uuid;primaryKey"`
	BookID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	Position     int       `gorm:"not null"`
	AddedAt      time.Time `gorm:"not null;default:now()"`
	// PreviousPosition and Score are set on snapshot items of auto collections.
	PreviousPosition *int
	Score            *float64
	// Source* name the adaptation a verdict-metric snapshot item was scored by.
	SourceScreenWorkID *uuid.UUID `gorm:"type:uuid"`
	SourceVotes        *int
	SourceShare        *float64
}

func (CollectionItem) TableName() string { return "curated_collection_items" }

// RankedItem is an item with its computed read-time standing.
type RankedItem struct {
	BookID   uuid.UUID
	Position int
	// Place is 1..size for ranked items, 0 otherwise.
	Place int
	State string
}

// BookRef is the minimum the ranking rule needs about a stored item.
type BookRef struct {
	BookID    uuid.UUID
	Published bool
}

// Rank turns the admin-ordered items (index 0 is position 1) into places:
// published books take places 1..size in order, published books beyond size
// are reserve, unpublished books are hidden and take no place.
func Rank(items []BookRef, size int) []RankedItem {
	out := make([]RankedItem, 0, len(items))
	place := 0
	for i, it := range items {
		r := RankedItem{BookID: it.BookID, Position: i + 1}
		switch {
		case !it.Published:
			r.State = StateHidden
		case place < size:
			place++
			r.Place = place
			r.State = StateRanked
		default:
			r.State = StateReserve
		}
		out = append(out, r)
	}
	return out
}

// ShownCount is the number of public places given the count of published
// books in the collection.
func ShownCount(published, size int) int {
	if published < size {
		return published
	}
	return size
}
