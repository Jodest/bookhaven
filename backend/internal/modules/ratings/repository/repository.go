package repository

import (
	"context"
	"errors"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	ratingsDomain "github.com/bookhaven/backend/internal/modules/ratings/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrRatingNotFound is returned when a rating does not exist.
var ErrRatingNotFound = errors.New("rating not found")

// Repository defines operations for working with book ratings.
type Repository interface {
	// UpsertAndRecalc inserts or updates a user's rating for a book and
	// recalculates the book's aggregate rating and ratings_count.
	// It returns whether a new row was created and the updated aggregates.
	UpsertAndRecalc(ctx context.Context, bookID, userID uuid.UUID, value int) (created bool, rating float64, ratingsCount int, err error)
	// GetByUserAndBook returns a user's rating for a specific book.
	GetByUserAndBook(ctx context.Context, userID, bookID uuid.UUID) (*ratingsDomain.Rating, error)
	// BookExists checks whether a published book exists.
	BookExists(ctx context.Context, bookID uuid.UUID) (bool, error)
}

// GormRepository is a GORM-based implementation of Repository.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a new GormRepository.
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

// UpsertAndRecalc inserts or updates a rating and refreshes book aggregates.
func (r *GormRepository) UpsertAndRecalc(ctx context.Context, bookID, userID uuid.UUID, value int) (bool, float64, int, error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return false, 0, 0, tx.Error
	}
	defer tx.Rollback()

	var result struct {
		ID      uuid.UUID `gorm:"column:id"`
		Created bool      `gorm:"column:created"`
	}

	err := tx.Raw(`
		INSERT INTO ratings (id, book_id, user_id, value, created_at, updated_at)
		VALUES (gen_random_uuid(), ?, ?, ?, NOW(), NOW())
		ON CONFLICT (book_id, user_id) DO UPDATE SET
			value = EXCLUDED.value,
			updated_at = NOW()
		RETURNING id, (xmax = 0) AS created
	`, bookID, userID, value).Scan(&result).Error
	if err != nil {
		return false, 0, 0, err
	}

	err = tx.Exec(`
		UPDATE books
		SET rating = COALESCE(ROUND((SELECT AVG(value) FROM ratings WHERE book_id = ?)::numeric, 2), 0),
		    ratings_count = (SELECT COUNT(*) FROM ratings WHERE book_id = ?)
		WHERE id = ?
	`, bookID, bookID, bookID).Error
	if err != nil {
		return false, 0, 0, err
	}

	var agg struct {
		Rating       float64 `gorm:"column:rating"`
		RatingsCount int     `gorm:"column:ratings_count"`
	}
	err = tx.Raw(`SELECT rating, ratings_count FROM books WHERE id = ?`, bookID).Scan(&agg).Error
	if err != nil {
		return false, 0, 0, err
	}

	if err := tx.Commit().Error; err != nil {
		return false, 0, 0, err
	}

	return result.Created, agg.Rating, agg.RatingsCount, nil
}

// GetByUserAndBook returns a user's rating for a book.
func (r *GormRepository) GetByUserAndBook(ctx context.Context, userID, bookID uuid.UUID) (*ratingsDomain.Rating, error) {
	var rating ratingsDomain.Rating
	if err := r.db.WithContext(ctx).First(&rating, "book_id = ? AND user_id = ?", bookID, userID).Error; err != nil {
		return nil, err
	}
	return &rating, nil
}

// BookExists checks whether a published book exists.
func (r *GormRepository) BookExists(ctx context.Context, bookID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("books").
		Where("id = ? AND status = ?", bookID, catalogDomain.BookStatusPublished).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
