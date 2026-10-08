package repository

import (
	"context"
	"strings"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository defines operations for working with catalog.
type Repository interface {
	ListBooks(ctx context.Context, filter BookFilter) ([]domain.Book, int64, error)
	GetBookBySlug(ctx context.Context, slug string) (*domain.Book, error)
	GetAuthorBySlug(ctx context.Context, slug string) (*domain.Author, error)
	ListGenres(ctx context.Context) ([]domain.Genre, error)
	ListAuthors(ctx context.Context, search string, limit int) ([]domain.Author, int64, error)
	ListLanguages(ctx context.Context) ([]string, error)
	// GetSimilarSource returns the author/genre IDs of a visible book.
	// Returns gorm.ErrRecordNotFound when the book is missing or hidden.
	GetSimilarSource(ctx context.Context, bookID uuid.UUID) (*SimilarSource, error)
	// ListSimilarCandidates returns raw (unscored) candidate rows for the
	// similar-books ranking: visible books sharing at least one author or
	// genre with the source, bounded to authorPool author-matched and
	// genrePool genre-only rows.
	ListSimilarCandidates(ctx context.Context, bookID uuid.UUID, src SimilarSource, authorPool, genrePool int) ([]SimilarCandidate, error)
	// GetBooksByIDs returns visible books with Authors and Genres preloaded.
	// The order of the result is not guaranteed.
	GetBooksByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Book, error)
}

// SimilarSource is the visible source book's author/genre IDs.
type SimilarSource struct {
	AuthorIDs []uuid.UUID
	GenreIDs  []uuid.UUID
}

// SimilarCandidate is one raw candidate row (unscored).
type SimilarCandidate struct {
	BookID        uuid.UUID
	SharedAuthors int
	SharedGenres  int
	GenreCount    int
	Rating        float64
	RatingsCount  int
	AuthorIDs     []uuid.UUID // filled by a second query, for the per-author cap
}

// BookFilter defines filtering options for books.
type BookFilter struct {
	Search    string
	GenreID   []uuid.UUID
	AuthorID  *uuid.UUID
	RatingMin *int
	Language  *string
	YearFrom  *int
	YearTo    *int
	Sort      string // "rating" (default), "new", "title"
	Limit     int
	Offset    int
}

// GormRepository is a GORM-based implementation of Repository.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a new GormRepository.
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

func (r *GormRepository) ListBooks(ctx context.Context, filter BookFilter) ([]domain.Book, int64, error) {
	var books []domain.Book
	var total int64

	query := r.db.WithContext(ctx).Model(&domain.Book{}).
		Preload("Authors").
		Preload("Genres").
		Where("is_hidden = ?", false)

	if filter.Search != "" {
		query = query.Where(
			"to_tsvector('russian', title || ' ' || COALESCE(isbn, '') || ' ' || COALESCE(description, '')) @@ plainto_tsquery('russian', ?)",
			filter.Search,
		)
	}

	if len(filter.GenreID) > 0 {
		// Only published genres are public filter targets; non-published
		// genre IDs match nothing (empty result when none is published).
		query = query.Joins("JOIN book_genres ON book_genres.book_id = books.id").
			Where("book_genres.genre_id IN ?", filter.GenreID).
			Where("EXISTS (SELECT 1 FROM genres g WHERE g.id = book_genres.genre_id AND g.status = ?)", domain.GenreStatusPublished)
	}

	if filter.AuthorID != nil {
		// Only a published author is a public filter target; a
		// non-published author ID yields an empty result.
		query = query.Joins("JOIN book_authors ON book_authors.book_id = books.id").
			Where("book_authors.author_id = ?", filter.AuthorID).
			Where("EXISTS (SELECT 1 FROM authors a WHERE a.id = book_authors.author_id AND a.status = ?)", domain.AuthorStatusPublished)
	}

	if filter.RatingMin != nil {
		query = query.Where("rating >= ?", *filter.RatingMin)
	}

	if filter.Language != nil {
		query = query.Where("language = ?", *filter.Language)
	}

	if filter.YearFrom != nil {
		query = query.Where("published_year >= ?", *filter.YearFrom)
	}

	if filter.YearTo != nil {
		query = query.Where("published_year <= ?", *filter.YearTo)
	}

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

	order := "rating DESC, created_at DESC"
	switch filter.Sort {
	case "new":
		order = "published_year DESC NULLS LAST, created_at DESC"
	case "title":
		order = "title ASC"
	}

	if err := query.Order(order).
		Limit(limit).
		Offset(offset).
		Find(&books).Error; err != nil {
		return nil, 0, err
	}

	return books, total, nil
}

func (r *GormRepository) GetBookBySlug(ctx context.Context, slug string) (*domain.Book, error) {
	var book domain.Book
	if err := r.db.WithContext(ctx).
		Preload("Authors").
		Preload("Genres").
		Where("slug = ? AND is_hidden = ?", slug, false).
		First(&book).Error; err != nil {
		return nil, err
	}
	return &book, nil
}

func (r *GormRepository) GetAuthorBySlug(ctx context.Context, slug string) (*domain.Author, error) {
	var author domain.Author
	if err := r.db.WithContext(ctx).
		Preload("Books", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_hidden = ?", false).Order("rating DESC, created_at DESC")
		}).
		Preload("Books.Authors").
		Preload("Books.Genres").
		Where("slug = ? AND status = ?", slug, domain.AuthorStatusPublished).
		First(&author).Error; err != nil {
		return nil, err
	}
	return &author, nil
}

// ListGenres returns the published genres only (public read).
func (r *GormRepository) ListGenres(ctx context.Context) ([]domain.Genre, error) {
	var genres []domain.Genre
	if err := r.db.WithContext(ctx).
		Where("status = ?", domain.GenreStatusPublished).
		Order("name ASC").Find(&genres).Error; err != nil {
		return nil, err
	}
	return genres, nil
}

func (r *GormRepository) ListAuthors(ctx context.Context, search string, limit int) ([]domain.Author, int64, error) {
	var authors []domain.Author
	var total int64

	query := r.db.WithContext(ctx).Model(&domain.Author{}).
		Where("status = ?", domain.AuthorStatusPublished)

	if search != "" {
		query = query.Where(
			"to_tsvector('russian', name || ' ' || COALESCE(biography, '')) @@ plainto_tsquery('russian', ?)",
			search,
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 20
	}

	if err := query.Order("name ASC").
		Limit(limit).
		Find(&authors).Error; err != nil {
		return nil, 0, err
	}

	return authors, total, nil
}

func (r *GormRepository) ListLanguages(ctx context.Context) ([]string, error) {
	var languages []string
	if err := r.db.WithContext(ctx).
		Model(&domain.Book{}).
		Where("is_hidden = ?", false).
		Distinct("language").
		Order("language ASC").
		Pluck("language", &languages).Error; err != nil {
		return nil, err
	}
	return languages, nil
}

func (r *GormRepository) GetSimilarSource(ctx context.Context, bookID uuid.UUID) (*SimilarSource, error) {
	db := r.db.WithContext(ctx)

	var ids []uuid.UUID
	if err := db.Model(&domain.Book{}).
		Where("id = ? AND is_hidden = ?", bookID, false).
		Limit(1).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	// Only published authors count as a shared-author signal.
	src := &SimilarSource{}
	if err := db.Table("book_authors").
		Joins("JOIN authors ON authors.id = book_authors.author_id").
		Where("book_authors.book_id = ? AND authors.status = ?", bookID, domain.AuthorStatusPublished).
		Pluck("book_authors.author_id", &src.AuthorIDs).Error; err != nil {
		return nil, err
	}
	// Likewise only published genres count as a shared-genre signal.
	if err := db.Table("book_genres").
		Joins("JOIN genres ON genres.id = book_genres.genre_id").
		Where("book_genres.book_id = ? AND genres.status = ?", bookID, domain.GenreStatusPublished).
		Pluck("book_genres.genre_id", &src.GenreIDs).Error; err != nil {
		return nil, err
	}
	return src, nil
}

// similarCandidateRow is the scan target of the candidate query.
type similarCandidateRow struct {
	BookID        uuid.UUID
	SharedAuthors int
	SharedGenres  int
	GenreCount    int
	Rating        float64
	RatingsCount  int
}

// ListSimilarCandidates collects a bounded set of candidate rows with raw
// features. The SQL is kept free of Postgres-only syntax (plain CTEs,
// UNION ALL, GROUP BY, ROW_NUMBER) so it stays portable; scoring happens in
// Go (catalog service).
func (r *GormRepository) ListSimilarCandidates(ctx context.Context, bookID uuid.UUID, src SimilarSource, authorPool, genrePool int) ([]SimilarCandidate, error) {
	if len(src.AuthorIDs) == 0 && len(src.GenreIDs) == 0 {
		return nil, nil
	}

	params := map[string]interface{}{
		"book_id":     bookID,
		"author_pool": authorPool,
		"genre_pool":  genrePool,
		// Only published genres count toward a candidate's genre_count
		// (docs/features/genre-status.md rule 8).
		"published_genre": domain.GenreStatusPublished,
	}
	// Build the matches CTE only from non-empty branches so no empty IN ()
	// is ever sent.
	var branches []string
	if len(src.AuthorIDs) > 0 {
		branches = append(branches, `SELECT ba.book_id, 1 AS is_author, 0 AS is_genre
        FROM book_authors ba
        WHERE ba.author_id IN @author_ids`)
		params["author_ids"] = src.AuthorIDs
	}
	if len(src.GenreIDs) > 0 {
		branches = append(branches, `SELECT bg.book_id, 0 AS is_author, 1 AS is_genre
        FROM book_genres bg
        WHERE bg.genre_id IN @genre_ids`)
		params["genre_ids"] = src.GenreIDs
	}

	query := `WITH matches AS (
    ` + strings.Join(branches, "\n    UNION ALL\n    ") + `
),
agg AS (
    SELECT book_id,
           SUM(is_author) AS shared_authors,
           SUM(is_genre)  AS shared_genres
    FROM matches
    WHERE book_id <> @book_id
    GROUP BY book_id
),
ranked AS (
    SELECT b.id AS book_id, agg.shared_authors, agg.shared_genres,
           b.rating, b.ratings_count,
           ROW_NUMBER() OVER (
               PARTITION BY CASE WHEN agg.shared_authors > 0 THEN 1 ELSE 0 END
               ORDER BY agg.shared_genres DESC, b.ratings_count DESC, b.rating DESC, b.id
           ) AS rn
    FROM agg
    JOIN books b ON b.id = agg.book_id
    WHERE b.is_hidden = FALSE
)
SELECT r.book_id, r.shared_authors, r.shared_genres, r.rating, r.ratings_count,
       (SELECT COUNT(*) FROM book_genres g
        JOIN genres gn ON gn.id = g.genre_id
        WHERE g.book_id = r.book_id AND gn.status = @published_genre) AS genre_count
FROM ranked r
WHERE (r.shared_authors > 0 AND r.rn <= @author_pool)
   OR (r.shared_authors = 0 AND r.rn <= @genre_pool)`

	db := r.db.WithContext(ctx)
	var rows []similarCandidateRow
	if err := db.Raw(query, params).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.BookID)
	}
	var links []struct {
		BookID   uuid.UUID
		AuthorID uuid.UUID
	}
	if err := db.Raw(`SELECT ba.book_id, ba.author_id
        FROM book_authors ba
        JOIN authors a ON a.id = ba.author_id
        WHERE ba.book_id IN ? AND a.status = ?`, ids, domain.AuthorStatusPublished).
		Scan(&links).Error; err != nil {
		return nil, err
	}
	authorsByBook := make(map[uuid.UUID][]uuid.UUID, len(rows))
	for _, l := range links {
		authorsByBook[l.BookID] = append(authorsByBook[l.BookID], l.AuthorID)
	}

	cands := make([]SimilarCandidate, 0, len(rows))
	for _, row := range rows {
		cands = append(cands, SimilarCandidate{
			BookID:        row.BookID,
			SharedAuthors: row.SharedAuthors,
			SharedGenres:  row.SharedGenres,
			GenreCount:    row.GenreCount,
			Rating:        row.Rating,
			RatingsCount:  row.RatingsCount,
			AuthorIDs:     authorsByBook[row.BookID],
		})
	}
	return cands, nil
}

func (r *GormRepository) GetBooksByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Book, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var books []domain.Book
	if err := r.db.WithContext(ctx).
		Preload("Authors").
		Preload("Genres").
		Where("id IN ? AND is_hidden = ?", ids, false).
		Find(&books).Error; err != nil {
		return nil, err
	}
	return books, nil
}
