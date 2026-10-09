package repository

import (
	"context"
	"strings"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// authorBookStatsJoin attaches per-author linked-books counts. Portable
// SQL (SUM(CASE ...), not FILTER) so the SQLite repository tests run it.
const authorBookStatsJoin = `LEFT JOIN (
	SELECT ba.author_id,
		COUNT(*) AS books_total,
		SUM(CASE WHEN b.status = 'published' THEN 1 ELSE 0 END) AS books_visible
	FROM book_authors ba
	JOIN books b ON b.id = ba.book_id
	GROUP BY ba.author_id
) s ON s.author_id = authors.id`

// authorModerationScanRow is the flat scan target of
// ListAuthorsForModeration (authors.* + the two counts).
type authorModerationScanRow struct {
	ID              uuid.UUID
	Name            string
	Slug            string
	Biography       *string
	PhotoURL        *string
	BirthDate       *time.Time
	DeathDate       *time.Time
	Status          string
	ExternalSource  *string
	ExternalID      *string
	PhotoAuthor     *string
	PhotoLicense    *string
	PhotoLicenseURL *string `gorm:"column:photo_license_url"`
	PhotoSourceURL  *string `gorm:"column:photo_source_url"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	BooksTotal      int64
	BooksVisible    int64
}

func (r *GormRepository) ListAuthorsForModeration(ctx context.Context, filter AuthorModerationFilter) ([]AuthorModerationRow, int64, error) {
	query := r.db.WithContext(ctx).Table("authors").Joins(authorBookStatsJoin)

	if filter.Status != "" {
		query = query.Where("authors.status = ?", filter.Status)
	}
	switch filter.QID {
	case "with":
		query = query.Where("authors.external_id IS NOT NULL")
	case "without":
		query = query.Where("authors.external_id IS NULL")
	}
	if filter.Books == "none" {
		query = query.Where("s.author_id IS NULL")
	}
	if filter.Search != "" {
		query = query.Where(
			"to_tsvector('russian', authors.name || ' ' || COALESCE(authors.biography, '')) @@ plainto_tsquery('russian', ?)",
			filter.Search,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var scanned []authorModerationScanRow
	if err := query.
		Select("authors.id, authors.name, authors.slug, authors.biography, authors.photo_url, authors.birth_date, authors.death_date, " +
			"authors.status, authors.external_source, authors.external_id, " +
			"authors.photo_author, authors.photo_license, authors.photo_license_url, authors.photo_source_url, " +
			"authors.created_at, authors.updated_at, " +
			"COALESCE(s.books_total, 0) AS books_total, COALESCE(s.books_visible, 0) AS books_visible").
		Order("authors.created_at DESC, authors.name ASC").
		Limit(limit).
		Offset(offset).
		Scan(&scanned).Error; err != nil {
		return nil, 0, err
	}

	rows := make([]AuthorModerationRow, 0, len(scanned))
	for _, s := range scanned {
		rows = append(rows, AuthorModerationRow{
			Author: catalogDomain.Author{
				ID:              s.ID,
				Name:            s.Name,
				Slug:            s.Slug,
				Biography:       s.Biography,
				PhotoURL:        s.PhotoURL,
				BirthDate:       s.BirthDate,
				DeathDate:       s.DeathDate,
				Status:          s.Status,
				ExternalSource:  s.ExternalSource,
				ExternalID:      s.ExternalID,
				PhotoAuthor:     s.PhotoAuthor,
				PhotoLicense:    s.PhotoLicense,
				PhotoLicenseURL: s.PhotoLicenseURL,
				PhotoSourceURL:  s.PhotoSourceURL,
				CreatedAt:       s.CreatedAt,
				UpdatedAt:       s.UpdatedAt,
			},
			BooksTotal:   s.BooksTotal,
			BooksVisible: s.BooksVisible,
		})
	}
	return rows, total, nil
}

func (r *GormRepository) ListAuthorSampleBooks(ctx context.Context, authorIDs []uuid.UUID) (map[uuid.UUID][]SampleBook, error) {
	out := make(map[uuid.UUID][]SampleBook, len(authorIDs))
	if len(authorIDs) == 0 {
		return out, nil
	}

	var rows []struct {
		AuthorID uuid.UUID
		ID       uuid.UUID
		Title    string
		Status   string
	}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT author_id, id, title, status FROM (
			SELECT ba.author_id, b.id, b.title, b.status,
				ROW_NUMBER() OVER (PARTITION BY ba.author_id ORDER BY b.title ASC, b.id ASC) AS rn
			FROM book_authors ba
			JOIN books b ON b.id = ba.book_id
			WHERE ba.author_id IN ?
		) t
		WHERE rn <= ?
		ORDER BY author_id, title ASC, id ASC
	`, authorIDs, sampleBooksPerAuthor).Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		if len(out[row.AuthorID]) >= sampleBooksPerAuthor {
			continue
		}
		out[row.AuthorID] = append(out[row.AuthorID], SampleBook{ID: row.ID, Title: row.Title, Status: row.Status})
	}
	return out, nil
}

// likeEscaper escapes LIKE metacharacters for an ESCAPE '\' pattern.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (r *GormRepository) FindAuthorsBySurnames(ctx context.Context, surnames []string, excludeIDs []uuid.UUID) ([]catalogDomain.Author, error) {
	if len(surnames) == 0 {
		return []catalogDomain.Author{}, nil
	}

	// replace(), not translate(): both Postgres and SQLite have it.
	const normName = "replace(lower(name), 'ё', 'е')"
	conds := make([]string, 0, len(surnames))
	args := make([]interface{}, 0, len(surnames)*4)
	for _, s := range surnames {
		esc := likeEscaper.Replace(s)
		conds = append(conds, "("+normName+" = ? OR "+
			normName+` LIKE ? ESCAPE '\' OR `+
			normName+` LIKE ? ESCAPE '\' OR `+
			normName+` LIKE ? ESCAPE '\')`)
		args = append(args, s, "% "+esc, esc+",%", esc+" ,%")
	}

	query := r.db.WithContext(ctx).Model(&catalogDomain.Author{}).
		Where("status <> ?", catalogDomain.AuthorStatusRejected).
		Where("("+strings.Join(conds, " OR ")+")", args...)
	if len(excludeIDs) > 0 {
		query = query.Where("id NOT IN ?", excludeIDs)
	}

	var authors []catalogDomain.Author
	if err := query.Order("name ASC").Find(&authors).Error; err != nil {
		return nil, err
	}
	return authors, nil
}

func (r *GormRepository) SetAuthorStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	// Updates with an explicit column map (not Save): only status and
	// updated_at are written; name, slug, bio, QID and book_authors stay.
	result := r.db.WithContext(ctx).Model(&catalogDomain.Author{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "updated_at": time.Now()})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *GormRepository) SetBookStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	// Updates with an explicit column map (not Save): only status and
	// updated_at are written; every other book column and the
	// book_authors/book_genres links stay (book-status-model.md rule 7).
	result := r.db.WithContext(ctx).Model(&catalogDomain.Book{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "updated_at": time.Now()})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
