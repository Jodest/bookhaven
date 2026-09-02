package repository

import (
	"context"
	"errors"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/lists/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrBookAlreadyOnList = errors.New("book already on list")
	ErrBookNotOnList     = errors.New("book not on list")
)

// Repository defines operations for working with lists.
type Repository interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.List, error)
	GetByID(ctx context.Context, listID uuid.UUID) (*domain.List, error)
	GetPublicByID(ctx context.Context, listID uuid.UUID) (*domain.List, error)
	Create(ctx context.Context, list *domain.List) error
	Update(ctx context.Context, list *domain.List) error
	Delete(ctx context.Context, listID uuid.UUID) error
	AddBook(ctx context.Context, listID, bookID uuid.UUID, position int) error
	RemoveBook(ctx context.Context, listID, bookID uuid.UUID) error
	BookOnList(ctx context.Context, listID, bookID uuid.UUID) (bool, error)
	ListBooks(ctx context.Context, listID uuid.UUID, limit, offset int) ([]catalogDomain.Book, int64, error)
	BookExists(ctx context.Context, bookID uuid.UUID) (bool, error)
	MaxPosition(ctx context.Context, listID uuid.UUID) (int, error)
	ReorderBooks(ctx context.Context, listID uuid.UUID, bookIDs []uuid.UUID) error
	UserListsWithBook(ctx context.Context, userID, bookID uuid.UUID) (map[uuid.UUID]struct{}, error)
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

type listListRow struct {
	ID          uuid.UUID `gorm:"column:id"`
	UserID      uuid.UUID `gorm:"column:user_id"`
	Title       string    `gorm:"column:title"`
	Description string    `gorm:"column:description"`
	IsPublic    bool      `gorm:"column:is_public"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
	BooksCount  int64     `gorm:"column:books_count"`
}

// ListByUser returns all lists for a user with their book counts.
func (r *GormRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.List, error) {
	var rows []listListRow

	sql := `
		SELECT l.*, COUNT(li.book_id) AS books_count
		FROM lists l
		LEFT JOIN list_items li ON li.list_id = l.id
		WHERE l.user_id = ?
		GROUP BY l.id
		ORDER BY l.created_at DESC
	`

	if err := r.db.WithContext(ctx).Raw(sql, userID).Scan(&rows).Error; err != nil {
		return nil, err
	}

	lists := make([]domain.List, 0, len(rows))
	for _, row := range rows {
		description := row.Description
		lists = append(lists, domain.List{
			ID:          row.ID,
			UserID:      row.UserID,
			Title:       row.Title,
			Description: description,
			IsPublic:    row.IsPublic,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
			BooksCount:  row.BooksCount,
		})
	}

	return lists, nil
}

// GetByID returns a list by ID.
func (r *GormRepository) GetByID(ctx context.Context, listID uuid.UUID) (*domain.List, error) {
	var list domain.List
	if err := r.db.WithContext(ctx).First(&list, "id = ?", listID).Error; err != nil {
		return nil, err
	}
	return &list, nil
}

// GetPublicByID returns a public list by ID.
func (r *GormRepository) GetPublicByID(ctx context.Context, listID uuid.UUID) (*domain.List, error) {
	var list domain.List
	if err := r.db.WithContext(ctx).First(&list, "id = ? AND is_public = ?", listID, true).Error; err != nil {
		return nil, err
	}
	return &list, nil
}

// Create creates a new list.
func (r *GormRepository) Create(ctx context.Context, list *domain.List) error {
	return r.db.WithContext(ctx).Create(list).Error
}

// Update updates a list.
func (r *GormRepository) Update(ctx context.Context, list *domain.List) error {
	return r.db.WithContext(ctx).Save(list).Error
}

// Delete deletes a list. List items are removed by ON DELETE CASCADE.
func (r *GormRepository) Delete(ctx context.Context, listID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&domain.List{ID: listID}).Error
}

// AddBook adds a book to a list at the specified position.
func (r *GormRepository) AddBook(ctx context.Context, listID, bookID uuid.UUID, position int) error {
	item := domain.ListItem{ListID: listID, BookID: bookID, Position: position}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&item)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrBookAlreadyOnList
	}
	return nil
}

// RemoveBook removes a book from a list.
func (r *GormRepository) RemoveBook(ctx context.Context, listID, bookID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("list_id = ? AND book_id = ?", listID, bookID).
		Delete(&domain.ListItem{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrBookNotOnList
	}
	return nil
}

// BookOnList checks whether a book is on a list.
func (r *GormRepository) BookOnList(ctx context.Context, listID, bookID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.ListItem{}).
		Where("list_id = ? AND book_id = ?", listID, bookID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListBooks returns books on a list with pagination, ordered by position.
func (r *GormRepository) ListBooks(ctx context.Context, listID uuid.UUID, limit, offset int) ([]catalogDomain.Book, int64, error) {
	var books []catalogDomain.Book
	var total int64

	query := r.db.WithContext(ctx).Model(&catalogDomain.Book{}).
		Joins("JOIN list_items ON list_items.book_id = books.id").
		Where("list_items.list_id = ? AND books.is_hidden = ?", listID, false).
		Order("list_items.position ASC")

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

// MaxPosition returns the maximum position value for a list.
func (r *GormRepository) MaxPosition(ctx context.Context, listID uuid.UUID) (int, error) {
	var maxPosition *int
	if err := r.db.WithContext(ctx).
		Model(&domain.ListItem{}).
		Where("list_id = ?", listID).
		Select("MAX(position)").
		Scan(&maxPosition).Error; err != nil {
		return 0, err
	}
	if maxPosition == nil {
		return 0, nil
	}
	return *maxPosition, nil
}

// ReorderBooks updates the position of books in a list based on the order of bookIDs.
func (r *GormRepository) ReorderBooks(ctx context.Context, listID uuid.UUID, bookIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, bookID := range bookIDs {
			if err := tx.Model(&domain.ListItem{}).
				Where("list_id = ? AND book_id = ?", listID, bookID).
				Update("position", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UserListsWithBook returns the set of list IDs that contain the book for a user.
func (r *GormRepository) UserListsWithBook(ctx context.Context, userID, bookID uuid.UUID) (map[uuid.UUID]struct{}, error) {
	var ids []uuid.UUID
	if err := r.db.WithContext(ctx).
		Model(&domain.List{}).
		Select("lists.id").
		Joins("JOIN list_items ON list_items.list_id = lists.id").
		Where("lists.user_id = ? AND list_items.book_id = ?", userID, bookID).
		Pluck("lists.id", &ids).Error; err != nil {
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
