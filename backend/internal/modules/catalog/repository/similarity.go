package repository

import (
	"context"
	"time"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Stage-2 similar books (co-reading + daily precompute), see
// docs/features/similar-books-stage2.md. These interfaces are kept separate
// from Repository so the existing hand-written Repository fakes stay
// unchanged; GormRepository implements all of them.

// Interaction is one filtered (user, book) positive signal.
type Interaction struct {
	UserID uuid.UUID
	BookID uuid.UUID
}

// BookStat is a visible book's ratings count (tie-break for co-only
// partners).
type BookStat struct {
	ID           uuid.UUID
	RatingsCount int
}

// SimilarityRow is one precomputed book_similarities row.
type SimilarityRow struct {
	BookID        uuid.UUID
	SimilarBookID uuid.UUID
	Position      int
	Score         float64
	ContentScore  float64
	CoScore       float64
	CoReaders     int
	ComputedAt    time.Time
}

// PrecomputedSimilarReader is used by the similar-books endpoint.
type PrecomputedSimilarReader interface {
	// ListPrecomputedSimilar returns the stored similar-book IDs of bookID
	// ordered by position (empty when the book has no rows).
	ListPrecomputedSimilar(ctx context.Context, bookID uuid.UUID, limit int) ([]uuid.UUID, error)
}

// SimilarityStore is used by the batch recompute job only.
type SimilarityStore interface {
	// ListInteractions returns the filtered, deduplicated (user, book)
	// positive signals.
	ListInteractions(ctx context.Context) ([]Interaction, error)
	// ListVisibleBookStats returns ratings_count of every visible book.
	ListVisibleBookStats(ctx context.Context) ([]BookStat, error)
	// ListBookAuthorIDs returns all book_authors rows of published authors
	// grouped by book (non-published authors are not a similarity signal).
	ListBookAuthorIDs(ctx context.Context) (map[uuid.UUID][]uuid.UUID, error)
	// ReplaceSimilarities atomically replaces the whole table with rows.
	ReplaceSimilarities(ctx context.Context, rows []SimilarityRow) error
}

var (
	_ PrecomputedSimilarReader = (*GormRepository)(nil)
	_ SimilarityStore          = (*GormRepository)(nil)
)

// NewGormSimilarityRepository returns the concrete GormRepository, which
// implements Repository, SimilarityStore and PrecomputedSimilarReader.
// NewGormRepository returns only the Repository interface.
func NewGormSimilarityRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// bookSimilarityRecord is the gorm model of book_similarities.
type bookSimilarityRecord struct {
	BookID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	SimilarBookID uuid.UUID `gorm:"type:uuid;primaryKey"`
	Position      int
	Score         float64
	ContentScore  float64
	CoScore       float64
	CoReaders     int
	ComputedAt    time.Time
}

func (bookSimilarityRecord) TableName() string { return "book_similarities" }

// interactionsQuery loads the positive (user, book) signals. It reads the
// shelves, shelf_items, ratings and users tables owned by other modules as a
// read-only aggregate (no Go import dependency). If those schemas change
// (shelf types, the 1..5 rating scale, email verification), update this
// query. UNION deduplicates (user, book); the heavy-user cap and the
// "< 2 interactions" drop are applied in Go (catalog service coReading).
const interactionsQuery = `WITH pos AS (
    SELECT s.user_id, si.book_id
    FROM shelf_items si
    JOIN shelves s ON s.id = si.shelf_id
    WHERE s.type IN ('read', 'reading')
  UNION
    SELECT r.user_id, r.book_id FROM ratings r WHERE r.value >= 4
)
SELECT pos.user_id, pos.book_id
FROM pos
JOIN users u ON u.id = pos.user_id AND u.email_verified_at IS NOT NULL
JOIN books b ON b.id = pos.book_id AND b.is_hidden = FALSE
WHERE NOT EXISTS (
    SELECT 1 FROM ratings r2
    WHERE r2.user_id = pos.user_id AND r2.book_id = pos.book_id AND r2.value <= 2
)`

func (r *GormRepository) ListInteractions(ctx context.Context) ([]Interaction, error) {
	var rows []Interaction
	if err := r.db.WithContext(ctx).Raw(interactionsQuery).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormRepository) ListVisibleBookStats(ctx context.Context) ([]BookStat, error) {
	var rows []BookStat
	if err := r.db.WithContext(ctx).
		Raw("SELECT id, ratings_count FROM books WHERE is_hidden = FALSE").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormRepository) ListBookAuthorIDs(ctx context.Context) (map[uuid.UUID][]uuid.UUID, error) {
	var links []struct {
		BookID   uuid.UUID
		AuthorID uuid.UUID
	}
	if err := r.db.WithContext(ctx).
		Raw(`SELECT ba.book_id, ba.author_id
        FROM book_authors ba
        JOIN authors a ON a.id = ba.author_id
        WHERE a.status = ?`, domain.AuthorStatusPublished).
		Scan(&links).Error; err != nil {
		return nil, err
	}
	result := make(map[uuid.UUID][]uuid.UUID)
	for _, l := range links {
		result[l.BookID] = append(result[l.BookID], l.AuthorID)
	}
	return result, nil
}

// ReplaceSimilarities deletes all rows and inserts rows in one transaction.
// Concurrent readers keep seeing the previous set until commit; a failure
// rolls back and leaves the previous set intact.
func (r *GormRepository) ReplaceSimilarities(ctx context.Context, rows []SimilarityRow) error {
	records := make([]bookSimilarityRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, bookSimilarityRecord{
			BookID:        row.BookID,
			SimilarBookID: row.SimilarBookID,
			Position:      row.Position,
			Score:         row.Score,
			ContentScore:  row.ContentScore,
			CoScore:       row.CoScore,
			CoReaders:     row.CoReaders,
			ComputedAt:    row.ComputedAt,
		})
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM book_similarities").Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		return tx.CreateInBatches(records, 1000).Error
	})
}

func (r *GormRepository) ListPrecomputedSimilar(ctx context.Context, bookID uuid.UUID, limit int) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	if err := r.db.WithContext(ctx).
		Table("book_similarities").
		Where("book_id = ?", bookID).
		Order("position").
		Limit(limit).
		Pluck("similar_book_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
