package repository

import (
	"context"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Genre moderation queue and merge (docs/features/genre-moderation-queue.md).

// GenreModerationFilter defines filtering options for the genre moderation
// queue. Status is a valid genre status or "" (any); QID is "" (any),
// "with" or "without". Search uses the same name ILIKE condition as
// ListGenres. Alias rows are always excluded.
type GenreModerationFilter struct {
	Status string
	QID    string
	Search string
	Limit  int
	Offset int
}

// GenreModerationRow is one genre of the moderation queue with its
// linked-books counts (books_visible = linked books with is_hidden = false).
type GenreModerationRow struct {
	Genre        catalogDomain.Genre
	BooksTotal   int64
	BooksVisible int64
}

// genreBookStatsJoin attaches per-genre linked-books counts. Portable SQL
// (SUM(CASE ...), not FILTER) so the SQLite repository tests run it.
const genreBookStatsJoin = `LEFT JOIN (
	SELECT bg.genre_id,
		COUNT(*) AS books_total,
		SUM(CASE WHEN b.is_hidden THEN 0 ELSE 1 END) AS books_visible
	FROM book_genres bg
	JOIN books b ON b.id = bg.book_id
	GROUP BY bg.genre_id
) s ON s.genre_id = genres.id`

// genreModerationScanRow is the flat scan target of ListGenresForModeration.
type genreModerationScanRow struct {
	ID           uuid.UUID
	Name         string
	Slug         string
	Status       string
	WikidataQID  *string `gorm:"column:wikidata_qid"`
	CreatedAt    time.Time
	BooksTotal   int64
	BooksVisible int64
}

func (r *GormRepository) ListGenresForModeration(ctx context.Context, filter GenreModerationFilter) ([]GenreModerationRow, int64, error) {
	query := r.db.WithContext(ctx).Table("genres").Joins(genreBookStatsJoin).
		Where("genres.merged_into_id IS NULL")

	if filter.Status != "" {
		query = query.Where("genres.status = ?", filter.Status)
	}
	switch filter.QID {
	case "with":
		query = query.Where("genres.wikidata_qid IS NOT NULL")
	case "without":
		query = query.Where("genres.wikidata_qid IS NULL")
	}
	if filter.Search != "" {
		query = query.Where("genres.name ILIKE ?", "%"+filter.Search+"%")
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

	var scanned []genreModerationScanRow
	if err := query.
		Select("genres.id, genres.name, genres.slug, genres.status, genres.wikidata_qid, genres.created_at, " +
			"COALESCE(s.books_total, 0) AS books_total, COALESCE(s.books_visible, 0) AS books_visible").
		Order("genres.created_at DESC, genres.name ASC, genres.id ASC").
		Limit(limit).
		Offset(offset).
		Scan(&scanned).Error; err != nil {
		return nil, 0, err
	}

	rows := make([]GenreModerationRow, 0, len(scanned))
	for _, s := range scanned {
		rows = append(rows, GenreModerationRow{
			Genre: catalogDomain.Genre{
				ID:          s.ID,
				Name:        s.Name,
				Slug:        s.Slug,
				Status:      s.Status,
				WikidataQID: s.WikidataQID,
				CreatedAt:   s.CreatedAt,
			},
			BooksTotal:   s.BooksTotal,
			BooksVisible: s.BooksVisible,
		})
	}
	return rows, total, nil
}

func (r *GormRepository) ListGenreSampleBooks(ctx context.Context, genreIDs []uuid.UUID) (map[uuid.UUID][]SampleBook, error) {
	out := make(map[uuid.UUID][]SampleBook, len(genreIDs))
	if len(genreIDs) == 0 {
		return out, nil
	}

	var rows []struct {
		GenreID  uuid.UUID
		ID       uuid.UUID
		Title    string
		IsHidden bool
	}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT genre_id, id, title, is_hidden FROM (
			SELECT bg.genre_id, b.id, b.title, b.is_hidden,
				ROW_NUMBER() OVER (PARTITION BY bg.genre_id ORDER BY b.title ASC, b.id ASC) AS rn
			FROM book_genres bg
			JOIN books b ON b.id = bg.book_id
			WHERE bg.genre_id IN ?
		) t
		WHERE rn <= ?
		ORDER BY genre_id, title ASC, id ASC
	`, genreIDs, sampleBooksPerAuthor).Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		if len(out[row.GenreID]) >= sampleBooksPerAuthor {
			continue
		}
		out[row.GenreID] = append(out[row.GenreID], SampleBook{ID: row.ID, Title: row.Title, IsHidden: row.IsHidden})
	}
	return out, nil
}

func (r *GormRepository) ListGenreHintCandidates(ctx context.Context) ([]catalogDomain.Genre, error) {
	var genres []catalogDomain.Genre
	if err := r.db.WithContext(ctx).
		Select("id, name, slug, status, wikidata_qid").
		Where("status <> ? AND merged_into_id IS NULL", catalogDomain.GenreStatusRejected).
		Order("name ASC, id ASC").
		Find(&genres).Error; err != nil {
		return nil, err
	}
	return genres, nil
}

func (r *GormRepository) SetGenreStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	// Updates with an explicit column map (not Save): only status is
	// written (genres has no updated_at); name, slug, QID and book_genres
	// stay. Alias rows are not found (their status is pinned to rejected).
	result := r.db.WithContext(ctx).Model(&catalogDomain.Genre{}).
		Where("id = ? AND merged_into_id IS NULL", id).
		Updates(map[string]interface{}{"status": status})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// MergeGenres implements Repository.MergeGenres as a single transaction
// (genre-moderation-queue.md rule 4):
//  1. Move book_genres rows the target does not already have to the
//     target (their count is booksReassigned).
//  2. Delete the remaining source rows (books already linked to both).
//  3. Re-point the source's existing aliases to the target, so alias
//     chains stay one hop deep.
//  4. When the source has a QID and the target has none, clear it on the
//     source, then set it on the target (this order keeps the unique
//     wikidata_qid index happy). Otherwise the source keeps its QID and
//     the alias resolves it.
//  5. Turn the source into an alias: merged_into_id = target, status =
//     rejected. Name, slug and any remaining QID stay.
//
// Both rows are re-read inside the transaction (locked FOR UPDATE on
// Postgres); a missing or alias row returns gorm.ErrRecordNotFound with
// nothing changed.
func (r *GormRepository) MergeGenres(ctx context.Context, sourceID, targetID uuid.UUID) (int64, error) {
	var booksReassigned int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lock := ""
		if tx.Dialector.Name() == "postgres" {
			lock = " FOR UPDATE"
		}
		var rows []struct {
			ID          uuid.UUID
			WikidataQID *string `gorm:"column:wikidata_qid"`
		}
		if err := tx.Raw("SELECT id, wikidata_qid FROM genres WHERE id IN ? AND merged_into_id IS NULL ORDER BY id"+lock,
			[]uuid.UUID{sourceID, targetID}).Scan(&rows).Error; err != nil {
			return err
		}
		var source, target *string
		found := 0
		for i := range rows {
			switch rows[i].ID {
			case sourceID:
				source = rows[i].WikidataQID
				found++
			case targetID:
				target = rows[i].WikidataQID
				found++
			}
		}
		if found != 2 || sourceID == targetID {
			return gorm.ErrRecordNotFound
		}

		result := tx.Exec(`
			UPDATE book_genres
			SET genre_id = ?
			WHERE genre_id = ?
			AND book_id NOT IN (SELECT book_id FROM book_genres WHERE genre_id = ?)
		`, targetID, sourceID, targetID)
		if result.Error != nil {
			return result.Error
		}
		booksReassigned = result.RowsAffected

		if err := tx.Exec("DELETE FROM book_genres WHERE genre_id = ?", sourceID).Error; err != nil {
			return err
		}

		if err := tx.Exec("UPDATE genres SET merged_into_id = ? WHERE merged_into_id = ?", targetID, sourceID).Error; err != nil {
			return err
		}

		if source != nil && target == nil {
			if err := tx.Exec("UPDATE genres SET wikidata_qid = NULL WHERE id = ?", sourceID).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE genres SET wikidata_qid = ? WHERE id = ?", *source, targetID).Error; err != nil {
				return err
			}
		}

		return tx.Exec("UPDATE genres SET merged_into_id = ?, status = ? WHERE id = ?",
			targetID, catalogDomain.GenreStatusRejected, sourceID).Error
	})
	if err != nil {
		return 0, err
	}
	return booksReassigned, nil
}
