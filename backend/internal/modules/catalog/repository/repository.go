package repository

import (
	"context"

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
}

// BookFilter defines filtering options for books.
type BookFilter struct {
	Search   string
	GenreID  *uuid.UUID
	AuthorID *uuid.UUID
	Limit    int
	Offset   int
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
		searchQuery := "%" + filter.Search + "%"
		query = query.Where("title ILIKE ? OR isbn ILIKE ?", searchQuery, searchQuery)
	}

	if filter.GenreID != nil {
		query = query.Joins("JOIN book_genres ON book_genres.book_id = books.id").
			Where("book_genres.genre_id = ?", filter.GenreID)
	}

	if filter.AuthorID != nil {
		query = query.Joins("JOIN book_authors ON book_authors.book_id = books.id").
			Where("book_authors.author_id = ?", filter.AuthorID)
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

	if err := query.Order("rating DESC, created_at DESC").
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
		Where("slug = ?", slug).
		First(&author).Error; err != nil {
		return nil, err
	}
	return &author, nil
}

func (r *GormRepository) ListGenres(ctx context.Context) ([]domain.Genre, error) {
	var genres []domain.Genre
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&genres).Error; err != nil {
		return nil, err
	}
	return genres, nil
}
