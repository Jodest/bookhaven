package repository

import (
	"context"
	"errors"
	"strings"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/curated/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CoverSample is how many leading placed books feed a collection's covers.
const CoverSample = 4

// Counts are the stored and public-eligible book counts of a collection.
type Counts struct {
	Books     int
	Published int
}

// AdminItem is a stored item with its book (authors preloaded).
type AdminItem struct {
	Position int
	Book     catalogDomain.Book
}

// Repository defines persistence for curated collections.
type Repository interface {
	Create(ctx context.Context, c *domain.Collection) error
	Update(ctx context.Context, c *domain.Collection) error
	Delete(ctx context.Context, id uuid.UUID) (bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Collection, error)
	GetPublishedBySlug(ctx context.Context, slug string) (*domain.Collection, error)
	// ListAll returns every collection newest first by creation.
	ListAll(ctx context.Context) ([]domain.Collection, error)
	// ListPublished returns published collections, newest first by
	// published_at, ties by title.
	ListPublished(ctx context.Context) ([]domain.Collection, error)
	Counts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Counts, error)
	// Covers returns the non-null covers of the first CoverSample placed books.
	Covers(ctx context.Context, id uuid.UUID, size int) ([]string, error)
	// PublishedPage returns published books in order for limit/offset.
	PublishedPage(ctx context.Context, id uuid.UUID, limit, offset int) ([]catalogDomain.Book, error)
	AdminItems(ctx context.Context, id uuid.UUID) ([]AdminItem, error)
	// GetGenre and GetBook return (nil, nil) when the row does not exist.
	GetGenre(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error)
	GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error)
	// AddBook inserts at position (nil = end, clamped) in one transaction
	// holding the collection row lock.
	AddBook(ctx context.Context, collectionID, bookID uuid.UUID, position *int) error
	RemoveBook(ctx context.Context, collectionID, bookID uuid.UUID) error
	MoveBook(ctx context.Context, collectionID, bookID uuid.UUID, position int) error
}

// GormRepository is the GORM implementation of Repository.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a GormRepository.
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

// IsUniqueViolation reports a unique-constraint error from Postgres or SQLite.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "duplicate key")
}

func (r *GormRepository) Create(ctx context.Context, c *domain.Collection) error {
	err := r.db.WithContext(ctx).Omit("Genre").Create(c).Error
	if IsUniqueViolation(err) {
		return domain.ErrSlugTaken
	}
	return err
}

func (r *GormRepository) Update(ctx context.Context, c *domain.Collection) error {
	err := r.db.WithContext(ctx).Omit("Genre").Save(c).Error
	if IsUniqueViolation(err) {
		return domain.ErrSlugTaken
	}
	return err
}

func (r *GormRepository) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	// Items go with the collection; do not rely on FK cascade (SQLite tests).
	var deleted bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("collection_id = ?", id).Delete(&domain.CollectionItem{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", id).Delete(&domain.Collection{})
		deleted = res.RowsAffected > 0
		return res.Error
	})
	return deleted, err
}

func (r *GormRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Collection, error) {
	var c domain.Collection
	if err := r.db.WithContext(ctx).Preload("Genre").Where("id = ?", id).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrCollectionNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *GormRepository) GetPublishedBySlug(ctx context.Context, slug string) (*domain.Collection, error) {
	var c domain.Collection
	if err := r.db.WithContext(ctx).Preload("Genre").
		Where("slug = ? AND is_published = ?", slug, true).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrCollectionNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *GormRepository) ListAll(ctx context.Context) ([]domain.Collection, error) {
	var cs []domain.Collection
	err := r.db.WithContext(ctx).Order("created_at DESC, title ASC").Find(&cs).Error
	return cs, err
}

func (r *GormRepository) ListPublished(ctx context.Context) ([]domain.Collection, error) {
	var cs []domain.Collection
	err := r.db.WithContext(ctx).Preload("Genre").Where("is_published = ?", true).
		Order("published_at DESC, title ASC").Find(&cs).Error
	return cs, err
}

func (r *GormRepository) Counts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Counts, error) {
	out := make(map[uuid.UUID]Counts, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	type row struct {
		CollectionID uuid.UUID
		Books        int
		Published    int
	}
	var rows []row
	err := r.db.WithContext(ctx).Table("curated_collection_items AS i").
		Select("i.collection_id AS collection_id, COUNT(*) AS books, "+
			"COALESCE(SUM(CASE WHEN b.status = ? THEN 1 ELSE 0 END), 0) AS published", catalogDomain.BookStatusPublished).
		Joins("JOIN books b ON b.id = i.book_id").
		Where("i.collection_id IN ?", ids).
		Group("i.collection_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, rw := range rows {
		out[rw.CollectionID] = Counts{Books: rw.Books, Published: rw.Published}
	}
	return out, nil
}

func (r *GormRepository) Covers(ctx context.Context, id uuid.UUID, size int) ([]string, error) {
	n := CoverSample
	if size < n {
		n = size
	}
	if n <= 0 {
		return []string{}, nil
	}
	var rows []struct{ CoverURL *string }
	err := r.db.WithContext(ctx).Table("books").
		Joins("JOIN curated_collection_items i ON i.book_id = books.id").
		Where("i.collection_id = ? AND books.status = ?", id, catalogDomain.BookStatusPublished).
		Order("i.position ASC, i.book_id ASC").Limit(n).
		Select("books.cover_url AS cover_url").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, c := range rows {
		if c.CoverURL != nil && *c.CoverURL != "" {
			out = append(out, *c.CoverURL)
		}
	}
	return out, nil
}

func (r *GormRepository) PublishedPage(ctx context.Context, id uuid.UUID, limit, offset int) ([]catalogDomain.Book, error) {
	var books []catalogDomain.Book
	if limit <= 0 {
		return books, nil
	}
	err := r.db.WithContext(ctx).Model(&catalogDomain.Book{}).
		Joins("JOIN curated_collection_items i ON i.book_id = books.id").
		Where("i.collection_id = ? AND books.status = ?", id, catalogDomain.BookStatusPublished).
		Order("i.position ASC, i.book_id ASC").
		Limit(limit).Offset(offset).
		Preload("Authors").Preload("Genres").
		Find(&books).Error
	return books, err
}

func (r *GormRepository) AdminItems(ctx context.Context, id uuid.UUID) ([]AdminItem, error) {
	var items []domain.CollectionItem
	if err := r.db.WithContext(ctx).Where("collection_id = ?", id).
		Order("position ASC, book_id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []AdminItem{}, nil
	}
	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.BookID
	}
	var books []catalogDomain.Book
	if err := r.db.WithContext(ctx).Preload("Authors").Where("id IN ?", ids).Find(&books).Error; err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]catalogDomain.Book, len(books))
	for _, b := range books {
		byID[b.ID] = b
	}
	out := make([]AdminItem, 0, len(items))
	for _, it := range items {
		if b, ok := byID[it.BookID]; ok {
			out = append(out, AdminItem{Position: it.Position, Book: b})
		}
	}
	return out, nil
}

func (r *GormRepository) GetGenre(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error) {
	var g catalogDomain.Genre
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&g).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}

func (r *GormRepository) GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error) {
	var b catalogDomain.Book
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

// lock takes the collection row lock inside tx, serialising item edits.
func lock(tx *gorm.DB, collectionID uuid.UUID) error {
	var c domain.Collection
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").Where("id = ?", collectionID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrCollectionNotFound
	}
	return err
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (r *GormRepository) AddBook(ctx context.Context, collectionID, bookID uuid.UUID, position *int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lock(tx, collectionID); err != nil {
			return err
		}
		var dup int64
		if err := tx.Model(&domain.CollectionItem{}).
			Where("collection_id = ? AND book_id = ?", collectionID, bookID).Count(&dup).Error; err != nil {
			return err
		}
		if dup > 0 {
			return domain.ErrBookAlreadyIn
		}
		var k int64
		if err := tx.Model(&domain.CollectionItem{}).
			Where("collection_id = ?", collectionID).Count(&k).Error; err != nil {
			return err
		}
		if k >= domain.MaxBooks {
			return domain.ErrCollectionFull
		}
		pos := int(k) + 1
		if position != nil {
			pos = clamp(*position, 1, int(k)+1)
		}
		if err := tx.Model(&domain.CollectionItem{}).
			Where("collection_id = ? AND position >= ?", collectionID, pos).
			UpdateColumn("position", gorm.Expr("position + 1")).Error; err != nil {
			return err
		}
		err := tx.Create(&domain.CollectionItem{CollectionID: collectionID, BookID: bookID, Position: pos}).Error
		if IsUniqueViolation(err) {
			return domain.ErrBookAlreadyIn
		}
		return err
	})
}

func (r *GormRepository) RemoveBook(ctx context.Context, collectionID, bookID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lock(tx, collectionID); err != nil {
			return err
		}
		var it domain.CollectionItem
		if err := tx.Where("collection_id = ? AND book_id = ?", collectionID, bookID).First(&it).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrBookNotInList
			}
			return err
		}
		if err := tx.Where("collection_id = ? AND book_id = ?", collectionID, bookID).
			Delete(&domain.CollectionItem{}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.CollectionItem{}).
			Where("collection_id = ? AND position > ?", collectionID, it.Position).
			UpdateColumn("position", gorm.Expr("position - 1")).Error
	})
}

func (r *GormRepository) MoveBook(ctx context.Context, collectionID, bookID uuid.UUID, position int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lock(tx, collectionID); err != nil {
			return err
		}
		var it domain.CollectionItem
		if err := tx.Where("collection_id = ? AND book_id = ?", collectionID, bookID).First(&it).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrBookNotInList
			}
			return err
		}
		var k int64
		if err := tx.Model(&domain.CollectionItem{}).Where("collection_id = ?", collectionID).Count(&k).Error; err != nil {
			return err
		}
		target := clamp(position, 1, int(k))
		if target == it.Position {
			return nil
		}
		shift := tx.Model(&domain.CollectionItem{}).Where("collection_id = ?", collectionID)
		if target > it.Position {
			shift = shift.Where("position > ? AND position <= ?", it.Position, target).
				UpdateColumn("position", gorm.Expr("position - 1"))
		} else {
			shift = shift.Where("position >= ? AND position < ?", target, it.Position).
				UpdateColumn("position", gorm.Expr("position + 1"))
		}
		if shift.Error != nil {
			return shift.Error
		}
		return tx.Model(&domain.CollectionItem{}).
			Where("collection_id = ? AND book_id = ?", collectionID, bookID).
			UpdateColumn("position", target).Error
	})
}
