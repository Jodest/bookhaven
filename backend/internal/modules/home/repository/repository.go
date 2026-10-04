// Package repository holds the read-only queries of the home module (see
// docs/features/home-page-content.md).
//
// The home module owns no tables. GormRepository reads these tables owned
// by other modules in raw SQL and returns IDs and scalars only:
// books, authors, book_authors, book_genres, reviews, users.
// If those schemas change (visibility flags, rating columns, review fields),
// update the queries below.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MonthDay is a (month, day) pair matched against author dates.
type MonthDay struct {
	Month int
	Day   int
}

// PoolFilter selects visible books for the book of the day and the random
// pick. Zero values disable a condition.
type PoolFilter struct {
	RequireCover    bool
	MinRating       float64
	MinRatingsCount int
	GenreID         *uuid.UUID
	ExcludeID       *uuid.UUID
}

// CalendarEventRow is one author whose birth or death date matched.
type CalendarEventRow struct {
	AuthorID     uuid.UUID  `json:"author_id"`
	Name         string     `json:"name"`
	Slug         string     `json:"slug"`
	PhotoURL     *string    `json:"photo_url,omitempty"`
	BirthDate    *time.Time `json:"birth_date,omitempty"`
	DeathDate    *time.Time `json:"death_date,omitempty"`
	Type         string     `json:"type"` // "birth" or "death"
	EventDate    time.Time  `json:"event_date"`
	VisibleBooks int        `json:"visible_books"`
}

// AnniversaryRow is a visible book whose age is a multiple of the step.
type AnniversaryRow struct {
	BookID       uuid.UUID
	Years        int
	Rating       float64
	RatingsCount int
	Title        string
}

// ReviewRow is a live, visible review with its author's public fields.
// The book card is hydrated separately through the catalog BookLoader.
type ReviewRow struct {
	ID            uuid.UUID
	BookID        uuid.UUID
	Rating        int
	Content       string
	LikesCount    int
	CreatedAt     time.Time
	UserID        uuid.UUID
	UserName      *string
	UserAvatarURL *string
}

// Repository is the home module's read-only data access. All methods apply
// the visibility filter (books.is_hidden = false, reviews.is_hidden = false)
// in SQL.
type Repository interface {
	// ListPoolIDs returns all visible book IDs matching f, ordered by id.
	ListPoolIDs(ctx context.Context, f PoolFilter) ([]uuid.UUID, error)
	// PickRandomBookID returns one random visible book ID matching f, or nil.
	PickRandomBookID(ctx context.Context, f PoolFilter) (*uuid.UUID, error)
	// ListCalendarEvents returns birth/death events of authors with at
	// least one visible book whose date matches one of days.
	ListCalendarEvents(ctx context.Context, days []MonthDay) ([]CalendarEventRow, error)
	// ListTopBooksByAuthors returns up to perAuthor visible book IDs per
	// author ordered by rating DESC, ratings_count DESC, title ASC.
	ListTopBooksByAuthors(ctx context.Context, authorIDs []uuid.UUID, perAuthor int) (map[uuid.UUID][]uuid.UUID, error)
	// ListAnniversaryBooks returns visible books whose age (year -
	// published_year) is a positive multiple of step, best first.
	ListAnniversaryBooks(ctx context.Context, year, step, limit int) ([]AnniversaryRow, error)
	// ListNewBookIDs returns the newest visible books (created_at DESC).
	ListNewBookIDs(ctx context.Context, limit int) ([]uuid.UUID, error)
	// ListTopRatedBookIDs returns the best-rated visible books.
	ListTopRatedBookIDs(ctx context.Context, minRating float64, minCount, limit int) ([]uuid.UUID, error)
	// ListRecentReviewIDs returns the newest visible reviews, at most one
	// per book, with content of at least minLen characters.
	ListRecentReviewIDs(ctx context.Context, minLen, limit int) ([]uuid.UUID, error)
	// GetReviewsByIDs reads reviews live; hidden reviews or reviews of
	// hidden books are not returned. Order is unspecified.
	GetReviewsByIDs(ctx context.Context, ids []uuid.UUID) ([]ReviewRow, error)
}

// GormRepository implements Repository with raw SQL.
type GormRepository struct {
	db *gorm.DB
}

var _ Repository = (*GormRepository)(nil)

// NewGormRepository creates a GormRepository.
func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

type idRow struct {
	ID uuid.UUID
}

func ids(rows []idRow) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		result = append(result, r.ID)
	}
	return result
}

// poolQuery builds the shared visible-books filter.
func (r *GormRepository) poolQuery(ctx context.Context, f PoolFilter) *gorm.DB {
	q := r.db.WithContext(ctx).Table("books").Select("books.id").Where("books.is_hidden = FALSE")
	if f.RequireCover {
		q = q.Where("books.cover_url IS NOT NULL AND books.cover_url <> ''")
	}
	if f.MinRating > 0 {
		q = q.Where("books.rating >= ?", f.MinRating)
	}
	if f.MinRatingsCount > 0 {
		q = q.Where("books.ratings_count >= ?", f.MinRatingsCount)
	}
	if f.GenreID != nil {
		q = q.Where("EXISTS (SELECT 1 FROM book_genres bg WHERE bg.book_id = books.id AND bg.genre_id = ?)", *f.GenreID)
	}
	if f.ExcludeID != nil {
		q = q.Where("books.id <> ?", *f.ExcludeID)
	}
	return q
}

func (r *GormRepository) ListPoolIDs(ctx context.Context, f PoolFilter) ([]uuid.UUID, error) {
	var rows []idRow
	if err := r.poolQuery(ctx, f).Order("books.id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return ids(rows), nil
}

// PickRandomBookID uses ORDER BY random(); fine at the current catalog size
// (scaling path: COUNT + random OFFSET).
func (r *GormRepository) PickRandomBookID(ctx context.Context, f PoolFilter) (*uuid.UUID, error) {
	var rows []idRow
	if err := r.poolQuery(ctx, f).Order("random()").Limit(1).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	id := rows[0].ID
	return &id, nil
}

// calendarEventsQuery matches birth and death dates by month*100+day. An
// author whose birth and death both match yields two rows (UNION ALL).
const calendarEventsQuery = `WITH vis AS (
    SELECT ba.author_id, COUNT(*) AS visible_books
    FROM book_authors ba
    JOIN books b ON b.id = ba.book_id AND b.is_hidden = FALSE
    GROUP BY ba.author_id
)
SELECT a.id AS author_id, a.name, a.slug, a.photo_url, a.birth_date, a.death_date,
       'birth' AS type, a.birth_date AS event_date, vis.visible_books
FROM authors a
JOIN vis ON vis.author_id = a.id
WHERE a.birth_date IS NOT NULL
  AND (EXTRACT(MONTH FROM a.birth_date)::int * 100 + EXTRACT(DAY FROM a.birth_date)::int) IN ?
UNION ALL
SELECT a.id AS author_id, a.name, a.slug, a.photo_url, a.birth_date, a.death_date,
       'death' AS type, a.death_date AS event_date, vis.visible_books
FROM authors a
JOIN vis ON vis.author_id = a.id
WHERE a.death_date IS NOT NULL
  AND (EXTRACT(MONTH FROM a.death_date)::int * 100 + EXTRACT(DAY FROM a.death_date)::int) IN ?`

func (r *GormRepository) ListCalendarEvents(ctx context.Context, days []MonthDay) ([]CalendarEventRow, error) {
	if len(days) == 0 {
		return []CalendarEventRow{}, nil
	}
	keys := make([]int, 0, len(days))
	for _, d := range days {
		keys = append(keys, d.Month*100+d.Day)
	}
	var rows []CalendarEventRow
	if err := r.db.WithContext(ctx).Raw(calendarEventsQuery, keys, keys).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

const topBooksByAuthorsQuery = `SELECT author_id, book_id FROM (
    SELECT ba.author_id, b.id AS book_id,
           ROW_NUMBER() OVER (PARTITION BY ba.author_id
                              ORDER BY b.rating DESC, b.ratings_count DESC, b.title ASC, b.id ASC) AS rn
    FROM book_authors ba
    JOIN books b ON b.id = ba.book_id AND b.is_hidden = FALSE
    WHERE ba.author_id IN ?
) t
WHERE rn <= ?
ORDER BY author_id, rn`

func (r *GormRepository) ListTopBooksByAuthors(ctx context.Context, authorIDs []uuid.UUID, perAuthor int) (map[uuid.UUID][]uuid.UUID, error) {
	result := make(map[uuid.UUID][]uuid.UUID)
	if len(authorIDs) == 0 || perAuthor <= 0 {
		return result, nil
	}
	var rows []struct {
		AuthorID uuid.UUID
		BookID   uuid.UUID
	}
	if err := r.db.WithContext(ctx).Raw(topBooksByAuthorsQuery, authorIDs, perAuthor).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.AuthorID] = append(result[row.AuthorID], row.BookID)
	}
	return result, nil
}

const anniversaryQuery = `SELECT id AS book_id, (@year - published_year) AS years, rating, ratings_count, title
FROM books
WHERE is_hidden = FALSE
  AND published_year IS NOT NULL
  AND published_year < @year
  AND (@year - published_year) % @step = 0
ORDER BY ((@year - published_year) % 100 = 0) DESC,
         ((@year - published_year) % 50 = 0) DESC,
         rating DESC, ratings_count DESC, title ASC, id ASC
LIMIT @lim`

func (r *GormRepository) ListAnniversaryBooks(ctx context.Context, year, step, limit int) ([]AnniversaryRow, error) {
	if step <= 0 || limit <= 0 {
		return []AnniversaryRow{}, nil
	}
	var rows []AnniversaryRow
	if err := r.db.WithContext(ctx).Raw(anniversaryQuery, map[string]interface{}{
		"year": year, "step": step, "lim": limit,
	}).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormRepository) ListNewBookIDs(ctx context.Context, limit int) ([]uuid.UUID, error) {
	var rows []idRow
	if err := r.db.WithContext(ctx).
		Raw("SELECT id FROM books WHERE is_hidden = FALSE ORDER BY created_at DESC, id DESC LIMIT ?", limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return ids(rows), nil
}

func (r *GormRepository) ListTopRatedBookIDs(ctx context.Context, minRating float64, minCount, limit int) ([]uuid.UUID, error) {
	var rows []idRow
	if err := r.db.WithContext(ctx).
		Raw(`SELECT id FROM books
WHERE is_hidden = FALSE AND rating >= ? AND ratings_count >= ?
ORDER BY rating DESC, ratings_count DESC, id ASC
LIMIT ?`, minRating, minCount, limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return ids(rows), nil
}

// recentReviewsQuery keeps the newest review per book, then the newest of
// those.
const recentReviewsQuery = `SELECT id FROM (
    SELECT DISTINCT ON (r.book_id) r.id, r.created_at
    FROM reviews r
    JOIN books b ON b.id = r.book_id AND b.is_hidden = FALSE
    WHERE r.is_hidden = FALSE AND char_length(r.content) >= ?
    ORDER BY r.book_id, r.created_at DESC, r.id DESC
) t
ORDER BY created_at DESC, id DESC
LIMIT ?`

func (r *GormRepository) ListRecentReviewIDs(ctx context.Context, minLen, limit int) ([]uuid.UUID, error) {
	var rows []idRow
	if err := r.db.WithContext(ctx).Raw(recentReviewsQuery, minLen, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return ids(rows), nil
}

const reviewsByIDsQuery = `SELECT r.id, r.book_id, r.rating, r.content, r.likes_count, r.created_at,
       u.id AS user_id, u.name AS user_name, u.avatar_url AS user_avatar_url
FROM reviews r
JOIN users u ON u.id = r.user_id
JOIN books b ON b.id = r.book_id AND b.is_hidden = FALSE
WHERE r.id IN ? AND r.is_hidden = FALSE`

func (r *GormRepository) GetReviewsByIDs(ctx context.Context, reviewIDs []uuid.UUID) ([]ReviewRow, error) {
	if len(reviewIDs) == 0 {
		return []ReviewRow{}, nil
	}
	var rows []ReviewRow
	if err := r.db.WithContext(ctx).Raw(reviewsByIDsQuery, reviewIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
