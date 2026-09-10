package repository

import (
	"context"
	"time"

	"github.com/bookhaven/backend/internal/modules/reading_challenges/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository defines operations for working with reading challenges.
type Repository interface {
	GetByUserAndYear(ctx context.Context, userID uuid.UUID, year int) (*domain.ReadingChallenge, error)
	Upsert(ctx context.Context, challenge *domain.ReadingChallenge) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.ReadingChallenge, error)
	CountReadBooks(ctx context.Context, userID uuid.UUID, year int) (int64, error)
}

// GormRepository is a GORM-based implementation of Repository.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a new GormRepository.
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

// GetByUserAndYear returns the caller's stored challenge for a year, if any.
func (r *GormRepository) GetByUserAndYear(ctx context.Context, userID uuid.UUID, year int) (*domain.ReadingChallenge, error) {
	var challenge domain.ReadingChallenge
	if err := r.db.WithContext(ctx).First(&challenge, "user_id = ? AND year = ?", userID, year).Error; err != nil {
		return nil, err
	}
	return &challenge, nil
}

// Upsert creates or updates the (user_id, year) goal row.
func (r *GormRepository) Upsert(ctx context.Context, challenge *domain.ReadingChallenge) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "year"}},
			DoUpdates: clause.AssignmentColumns([]string{"goal", "updated_at"}),
		}).
		Create(challenge).Error
}

// ListByUser returns all stored challenges for a user, newest year first.
func (r *GormRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.ReadingChallenge, error) {
	var challenges []domain.ReadingChallenge
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("year DESC").
		Find(&challenges).Error; err != nil {
		return nil, err
	}
	return challenges, nil
}

// CountReadBooks returns the number of distinct books on the user's "read"
// shelf whose shelf_items.added_at falls within the given calendar year
// (UTC). This is a completion-date proxy (see reading-challenge feature
// docs); if BookHaven later adds an explicit "finished reading" date, this
// query should prefer that field instead.
func (r *GormRepository) CountReadBooks(ctx context.Context, userID uuid.UUID, year int) (int64, error) {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)

	sql := `
		SELECT COUNT(DISTINCT si.book_id)
		FROM shelf_items si
		JOIN shelves s ON s.id = si.shelf_id
		WHERE s.user_id = ? AND s.type = 'read' AND si.added_at >= ? AND si.added_at < ?
	`

	var count int64
	if err := r.db.WithContext(ctx).Raw(sql, userID, start, end).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
