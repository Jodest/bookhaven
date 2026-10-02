package repository

import (
	"context"
	"errors"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrBookAlreadyOnShelf = errors.New("book already on shelf")
	ErrBookNotOnShelf     = errors.New("book not on shelf")
)

// Repository defines operations for working with shelves.
type Repository interface {
	EnsureDefaultShelves(ctx context.Context, userID uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Shelf, error)
	ListByUserVisible(ctx context.Context, userID uuid.UUID) ([]domain.Shelf, error)
	GetByID(ctx context.Context, shelfID uuid.UUID) (*domain.Shelf, error)
	GetByUserAndType(ctx context.Context, userID uuid.UUID, shelfType domain.ShelfType) (*domain.Shelf, error)
	Create(ctx context.Context, shelf *domain.Shelf) error
	Update(ctx context.Context, shelf *domain.Shelf) error
	Delete(ctx context.Context, shelfID uuid.UUID) error
	AddBook(ctx context.Context, shelfID, bookID uuid.UUID) error
	RemoveBook(ctx context.Context, shelfID, bookID uuid.UUID) error
	BookOnShelf(ctx context.Context, shelfID, bookID uuid.UUID) (bool, error)
	ListBooks(ctx context.Context, shelfID uuid.UUID, limit, offset int) ([]catalogDomain.Book, int64, error)
	BookExists(ctx context.Context, bookID uuid.UUID) (bool, error)
	UserShelvesWithBook(ctx context.Context, userID, bookID uuid.UUID) (map[uuid.UUID]struct{}, error)
	Transaction(ctx context.Context, fn func(repo Repository) error) error
}

// GormRepository is a GORM-based implementation of Repository.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a new GormRepository.
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

// EnsureDefaultShelves lazily creates the three default shelves for a user.
func (r *GormRepository) EnsureDefaultShelves(ctx context.Context, userID uuid.UUID) error {
	sql := `
		INSERT INTO shelves (id, user_id, name, type)
		VALUES
			(gen_random_uuid(), ?, 'Хочу прочитать', 'want_to_read'),
			(gen_random_uuid(), ?, 'Читаю', 'reading'),
			(gen_random_uuid(), ?, 'Прочитано', 'read')
		ON CONFLICT (user_id, type) WHERE type <> 'custom' DO NOTHING
	`
	return r.db.WithContext(ctx).Exec(sql, userID, userID, userID).Error
}

type shelfListRow struct {
	ID         uuid.UUID        `gorm:"column:id"`
	UserID     uuid.UUID        `gorm:"column:user_id"`
	Name       string           `gorm:"column:name"`
	Type       domain.ShelfType `gorm:"column:type"`
	CreatedAt  time.Time        `gorm:"column:created_at"`
	UpdatedAt  time.Time        `gorm:"column:updated_at"`
	BooksCount int64            `gorm:"column:books_count"`
}

// ListByUser returns all shelves for a user with their book counts.
// Default shelves are returned first in a fixed order, followed by custom shelves.
func (r *GormRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Shelf, error) {
	var rows []shelfListRow

	sql := `
		SELECT s.*, COUNT(si.book_id) AS books_count
		FROM shelves s
		LEFT JOIN shelf_items si ON si.shelf_id = s.id
		WHERE s.user_id = ?
		GROUP BY s.id
		ORDER BY
			CASE s.type
				WHEN 'want_to_read' THEN 1
				WHEN 'reading' THEN 2
				WHEN 'read' THEN 3
				ELSE 4
			END,
			s.created_at ASC
	`

	if err := r.db.WithContext(ctx).Raw(sql, userID).Scan(&rows).Error; err != nil {
		return nil, err
	}

	shelves := make([]domain.Shelf, 0, len(rows))
	for _, row := range rows {
		shelves = append(shelves, domain.Shelf{
			ID:         row.ID,
			UserID:     row.UserID,
			Name:       row.Name,
			Type:       row.Type,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
			BooksCount: row.BooksCount,
		})
	}

	return shelves, nil
}

// ListByUserVisible returns all shelves for a user with their book counts
// computed over visible (is_hidden = false) books only. Used for the public
// profile listing, where hidden books must not count towards a shelf's size
// or its inclusion. Unlike ListByUser (which counts all shelf_items rows
// regardless of visibility - a pre-existing gap for the owner-facing count,
// out of scope to fix here), this method is visible-books-aware from the start.
func (r *GormRepository) ListByUserVisible(ctx context.Context, userID uuid.UUID) ([]domain.Shelf, error) {
	var rows []shelfListRow

	sql := `
		SELECT s.*, COUNT(b.id) AS books_count
		FROM shelves s
		LEFT JOIN shelf_items si ON si.shelf_id = s.id
		LEFT JOIN books b ON b.id = si.book_id AND b.is_hidden = false
		WHERE s.user_id = ?
		GROUP BY s.id
		ORDER BY
			CASE s.type
				WHEN 'want_to_read' THEN 1
				WHEN 'reading' THEN 2
				WHEN 'read' THEN 3
				ELSE 4
			END,
			s.created_at ASC
	`

	if err := r.db.WithContext(ctx).Raw(sql, userID).Scan(&rows).Error; err != nil {
		return nil, err
	}

	shelves := make([]domain.Shelf, 0, len(rows))
	for _, row := range rows {
		shelves = append(shelves, domain.Shelf{
			ID:         row.ID,
			UserID:     row.UserID,
			Name:       row.Name,
			Type:       row.Type,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
			BooksCount: row.BooksCount,
		})
	}

	return shelves, nil
}

// GetByID returns a shelf by ID.
func (r *GormRepository) GetByID(ctx context.Context, shelfID uuid.UUID) (*domain.Shelf, error) {
	var shelf domain.Shelf
	if err := r.db.WithContext(ctx).First(&shelf, "id = ?", shelfID).Error; err != nil {
		return nil, err
	}
	return &shelf, nil
}

// GetByUserAndType returns a shelf by user and type.
func (r *GormRepository) GetByUserAndType(ctx context.Context, userID uuid.UUID, shelfType domain.ShelfType) (*domain.Shelf, error) {
	var shelf domain.Shelf
	if err := r.db.WithContext(ctx).First(&shelf, "user_id = ? AND type = ?", userID, shelfType).Error; err != nil {
		return nil, err
	}
	return &shelf, nil
}

// Create creates a new shelf.
func (r *GormRepository) Create(ctx context.Context, shelf *domain.Shelf) error {
	return r.db.WithContext(ctx).Create(shelf).Error
}

// Update updates a shelf.
func (r *GormRepository) Update(ctx context.Context, shelf *domain.Shelf) error {
	return r.db.WithContext(ctx).Save(shelf).Error
}

// Delete deletes a shelf. Shelf items are removed by ON DELETE CASCADE.
func (r *GormRepository) Delete(ctx context.Context, shelfID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&domain.Shelf{ID: shelfID}).Error
}

// AddBook adds a book to a shelf.
func (r *GormRepository) AddBook(ctx context.Context, shelfID, bookID uuid.UUID) error {
	item := domain.ShelfItem{ShelfID: shelfID, BookID: bookID}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&item)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrBookAlreadyOnShelf
	}
	return nil
}

// RemoveBook removes a book from a shelf.
func (r *GormRepository) RemoveBook(ctx context.Context, shelfID, bookID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("shelf_id = ? AND book_id = ?", shelfID, bookID).
		Delete(&domain.ShelfItem{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrBookNotOnShelf
	}
	return nil
}

// BookOnShelf checks whether a book is on a shelf.
func (r *GormRepository) BookOnShelf(ctx context.Context, shelfID, bookID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.ShelfItem{}).
		Where("shelf_id = ? AND book_id = ?", shelfID, bookID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListBooks returns books on a shelf with pagination.
func (r *GormRepository) ListBooks(ctx context.Context, shelfID uuid.UUID, limit, offset int) ([]catalogDomain.Book, int64, error) {
	var books []catalogDomain.Book
	var total int64

	query := r.db.WithContext(ctx).Model(&catalogDomain.Book{}).
		Joins("JOIN shelf_items ON shelf_items.book_id = books.id").
		Where("shelf_items.shelf_id = ? AND books.is_hidden = ?", shelfID, false).
		Order("shelf_items.added_at DESC")

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	if err := query.Limit(limit).Offset(offset).
		Preload("Authors").
		Preload("Genres").
		Find(&books).Error; err != nil {
		return nil, 0, err
	}

	return books, total, nil
}

// BookExists checks whether a book exists and is not hidden.
func (r *GormRepository) BookExists(ctx context.Context, bookID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&catalogDomain.Book{}).
		Where("id = ? AND is_hidden = ?", bookID, false).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// UserShelvesWithBook returns the set of shelf IDs that contain the book for a user.
func (r *GormRepository) UserShelvesWithBook(ctx context.Context, userID, bookID uuid.UUID) (map[uuid.UUID]struct{}, error) {
	var ids []uuid.UUID
	if err := r.db.WithContext(ctx).
		Model(&domain.Shelf{}).
		Select("shelves.id").
		Joins("JOIN shelf_items ON shelf_items.shelf_id = shelves.id").
		Where("shelves.user_id = ? AND shelf_items.book_id = ?", userID, bookID).
		Pluck("shelves.id", &ids).Error; err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result, nil
}

// Transaction runs a function inside a database transaction.
func (r *GormRepository) Transaction(ctx context.Context, fn func(repo Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewGormRepository(tx))
	})
}
