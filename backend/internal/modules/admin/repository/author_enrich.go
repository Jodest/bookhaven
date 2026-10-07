package repository

import (
	"context"
	"strings"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// AuthorEnrichmentFill is the set of values Wikidata enrichment wants to
// write (docs/features/author-wikidata-enrichment.md rule 1). Nil parts are
// not touched.
type AuthorEnrichmentFill struct {
	BirthDate *time.Time
	DeathDate *time.Time
	Photo     *PhotoFill
}

// PhotoFill is a Commons photo with its attribution. Author and LicenseURL
// may be nil.
type PhotoFill struct {
	URL        string
	Author     *string
	License    string
	LicenseURL *string
	SourceURL  string
}

// IsEmpty reports a fill with nothing to write.
func (f AuthorEnrichmentFill) IsEmpty() bool {
	return f.BirthDate == nil && f.DeathDate == nil && f.Photo == nil
}

func (r *GormRepository) GetAuthorsByIDs(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Author, error) {
	if len(ids) == 0 {
		return []catalogDomain.Author{}, nil
	}
	var authors []catalogDomain.Author
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&authors).Error; err != nil {
		return nil, err
	}
	return authors, nil
}

// photoEmptyCondition is the fill-empty condition shared by photo_url and
// its attribution columns, so attribution is only ever written together
// with the photo it describes.
const photoEmptyCondition = "photo_url IS NULL OR photo_url = ''"

func (r *GormRepository) FillAuthorEnrichment(ctx context.Context, id uuid.UUID, fill AuthorEnrichmentFill) error {
	if fill.IsEmpty() {
		return nil
	}
	var sets []string
	var args []interface{}
	if fill.BirthDate != nil {
		sets = append(sets, "birth_date = COALESCE(birth_date, ?)")
		args = append(args, *fill.BirthDate)
	}
	if fill.DeathDate != nil {
		sets = append(sets, "death_date = COALESCE(death_date, ?)")
		args = append(args, *fill.DeathDate)
	}
	if p := fill.Photo; p != nil {
		// SET expressions see the pre-update row (Postgres and SQLite), so
		// every CASE tests the same, original photo_url.
		for _, col := range []struct {
			name  string
			value interface{}
		}{
			{"photo_author", p.Author},
			{"photo_license", p.License},
			{"photo_license_url", p.LicenseURL},
			{"photo_source_url", p.SourceURL},
			{"photo_url", p.URL},
		} {
			sets = append(sets, col.name+" = CASE WHEN "+photoEmptyCondition+" THEN ? ELSE "+col.name+" END")
			args = append(args, col.value)
		}
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now(), id)

	return r.db.WithContext(ctx).
		Exec("UPDATE authors SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...).Error
}
