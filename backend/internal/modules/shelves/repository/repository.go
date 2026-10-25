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

	// Reading journal (book_reads). All methods work on the tx repository.

	// OpenRead starts an attempt unless one is already open.
	OpenRead(ctx context.Context, userID, bookID uuid.UUID, now time.Time) error
	// FinishRead closes the open attempt as finished, or inserts a finished one
	// without a start date when none is open.
	FinishRead(ctx context.Context, userID, bookID uuid.UUID, now time.Time) error
	// AbandonOpenRead closes the open attempt as abandoned (no-op without one).
	AbandonOpenRead(ctx context.Context, userID, bookID uuid.UUID, now time.Time) error
	// DeleteOpenRead deletes the open attempt (no-op without one).
	DeleteOpenRead(ctx context.Context, userID, bookID uuid.UUID) error
	// DeleteLatestFinishedRead deletes only the latest finished attempt that was
	// not dated by hand (manual_end = false) and finished at or after since.
	DeleteLatestFinishedRead(ctx context.Context, userID, bookID uuid.UUID, since time.Time) error
	// ReadSummary returns the caller's reading summary of one book.
	ReadSummary(ctx context.Context, userID, bookID uuid.UUID) (domain.ReadSummary, error)
	// ReadCounts returns finished-readings counts for many books in one query.
	ReadCounts(ctx context.Context, userID uuid.UUID, bookIDs []uuid.UUID) (map[uuid.UUID]domain.ReadCount, error)

	// Reading history (owner edits of book_reads).

	// ListReads returns the user's attempts of a book, oldest first.
	ListReads(ctx context.Context, userID, bookID uuid.UUID) ([]domain.BookRead, error)
	// GetRead returns the user's attempt; gorm.ErrRecordNotFound when missing or not owned.
	GetRead(ctx context.Context, userID, readID uuid.UUID) (*domain.BookRead, error)
	// CountReads returns how many attempts the user has for a book.
	CountReads(ctx context.Context, userID, bookID uuid.UUID) (int64, error)
	// InsertFinishedRead inserts a hand-entered finished attempt.
	InsertFinishedRead(ctx context.Context, read *domain.BookRead) error
	// UpdateReadDates updates columns only while the attempt is still in loadedState;
	// it returns the affected row count.
	UpdateReadDates(ctx context.Context, userID, readID uuid.UUID, loadedState domain.ReadState, fields map[string]any) (int64, error)
	// DeleteClosedRead deletes a finished/abandoned attempt (never the open one).
	DeleteClosedRead(ctx context.Context, userID, readID uuid.UUID) (int64, error)
	// ShelfItemAddedAt returns when the book was put on the shelf.
	ShelfItemAddedAt(ctx context.Context, shelfID, bookID uuid.UUID) (time.Time, bool, error)
}

// GormRepository is a GORM-based implementation of Repository.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a new GormRepository.
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

// EnsureDefaultShelves lazily creates the four default shelves for a user.
func (r *GormRepository) EnsureDefaultShelves(ctx context.Context, userID uuid.UUID) error {
	sql := `
		INSERT INTO shelves (id, user_id, name, type)
		VALUES
			(gen_random_uuid(), ?, 'Хочу прочитать', 'want_to_read'),
			(gen_random_uuid(), ?, 'Читаю', 'reading'),
			(gen_random_uuid(), ?, 'Прочитано', 'read'),
			(gen_random_uuid(), ?, 'Брошено', 'abandoned')
		ON CONFLICT (user_id, type) WHERE type <> 'custom' DO NOTHING
	`
	return r.db.WithContext(ctx).Exec(sql, userID, userID, userID, userID).Error
}

type shelfListRow struct {
	ID          uuid.UUID        `gorm:"column:id"`
	UserID      uuid.UUID        `gorm:"column:user_id"`
	Name        string           `gorm:"column:name"`
	Type        domain.ShelfType `gorm:"column:type"`
	Description string           `gorm:"column:description"`
	IsPublic    bool             `gorm:"column:is_public"`
	CreatedAt   time.Time        `gorm:"column:created_at"`
	UpdatedAt   time.Time        `gorm:"column:updated_at"`
	BooksCount  int64            `gorm:"column:books_count"`
}

// ListByUser returns all shelves for a user with their book counts, computed
// over published books only so they match what the shelf pages show.
// Default shelves are returned first in a fixed order, followed by custom shelves.
func (r *GormRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Shelf, error) {
	var rows []shelfListRow

	sql := `
		SELECT s.*, COUNT(b.id) AS books_count
		FROM shelves s
		LEFT JOIN shelf_items si ON si.shelf_id = s.id
		LEFT JOIN books b ON b.id = si.book_id AND b.status = 'published'
		WHERE s.user_id = ?
		GROUP BY s.id
		ORDER BY
			CASE s.type
				WHEN 'want_to_read' THEN 1
				WHEN 'reading' THEN 2
				WHEN 'read' THEN 3
				WHEN 'abandoned' THEN 4
				ELSE 5
			END,
			s.created_at ASC
	`

	if err := r.db.WithContext(ctx).Raw(sql, userID).Scan(&rows).Error; err != nil {
		return nil, err
	}

	shelves := make([]domain.Shelf, 0, len(rows))
	for _, row := range rows {
		shelves = append(shelves, domain.Shelf{
			ID:          row.ID,
			UserID:      row.UserID,
			Name:        row.Name,
			Type:        row.Type,
			Description: row.Description,
			IsPublic:    row.IsPublic,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
			BooksCount:  row.BooksCount,
		})
	}

	return shelves, nil
}

// ListByUserVisible returns the same shelves and published-only counts as
// ListByUser. It is kept as the public profile's entry point, which then drops
// shelves without published books.
func (r *GormRepository) ListByUserVisible(ctx context.Context, userID uuid.UUID) ([]domain.Shelf, error) {
	return r.ListByUser(ctx, userID)
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
	// Shelf.IsPublic has no gorm default tag, so is_public=false is stored as-is.
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
		Where("shelf_items.shelf_id = ? AND books.status = ?", shelfID, catalogDomain.BookStatusPublished).
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

// BookExists checks whether a published book exists.
func (r *GormRepository) BookExists(ctx context.Context, bookID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&catalogDomain.Book{}).
		Where("id = ? AND status = ?", bookID, catalogDomain.BookStatusPublished).
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

// openReadExists reports whether the user has an open attempt for the book.
func (r *GormRepository) openReadExists(ctx context.Context, userID, bookID uuid.UUID) (bool, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&domain.BookRead{}).
		Where("user_id = ? AND book_id = ? AND finished_at IS NULL AND abandoned_at IS NULL", userID, bookID).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// OpenRead starts an attempt unless one is already open. The partial unique
// index uq_book_reads_open is the backstop for concurrent opens.
func (r *GormRepository) OpenRead(ctx context.Context, userID, bookID uuid.UUID, now time.Time) error {
	open, err := r.openReadExists(ctx, userID, bookID)
	if err != nil {
		return err
	}
	if open {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&domain.BookRead{ID: uuid.New(), UserID: userID, BookID: bookID, StartedAt: &now, CreatedAt: now}).Error
}

// FinishRead closes the open attempt as finished, or inserts a finished one.
func (r *GormRepository) FinishRead(ctx context.Context, userID, bookID uuid.UUID, now time.Time) error {
	res := r.db.WithContext(ctx).Model(&domain.BookRead{}).
		Where("user_id = ? AND book_id = ? AND finished_at IS NULL AND abandoned_at IS NULL", userID, bookID).
		Update("finished_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Create(&domain.BookRead{ID: uuid.New(), UserID: userID, BookID: bookID, FinishedAt: &now, CreatedAt: now}).Error
}

// AbandonOpenRead closes the open attempt as abandoned.
func (r *GormRepository) AbandonOpenRead(ctx context.Context, userID, bookID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Model(&domain.BookRead{}).
		Where("user_id = ? AND book_id = ? AND finished_at IS NULL AND abandoned_at IS NULL", userID, bookID).
		Update("abandoned_at", now).Error
}

// DeleteOpenRead deletes the open attempt.
func (r *GormRepository) DeleteOpenRead(ctx context.Context, userID, bookID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND book_id = ? AND finished_at IS NULL AND abandoned_at IS NULL", userID, bookID).
		Delete(&domain.BookRead{}).Error
}

// DeleteLatestFinishedRead deletes the single latest finished attempt that was
// recorded by the status (manual_end = false) at or after since.
func (r *GormRepository) DeleteLatestFinishedRead(ctx context.Context, userID, bookID uuid.UUID, since time.Time) error {
	var latest domain.BookRead
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND book_id = ? AND finished_at IS NOT NULL AND manual_end = ? AND finished_at >= ?", userID, bookID, false, since).
		Order("finished_at DESC, created_at DESC").
		Limit(1).Take(&latest).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return r.db.WithContext(ctx).Delete(&domain.BookRead{}, "id = ?", latest.ID).Error
}

// ReadSummary returns the caller's reading summary of one book.
func (r *GormRepository) ReadSummary(ctx context.Context, userID, bookID uuid.UUID) (domain.ReadSummary, error) {
	var finished []domain.BookRead
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND book_id = ? AND finished_at IS NOT NULL", userID, bookID).
		Order("finished_at DESC").Find(&finished).Error; err != nil {
		return domain.ReadSummary{}, err
	}
	sum := domain.ReadSummary{Count: int64(len(finished))}
	if len(finished) > 0 {
		sum.LastReadAt = finished[0].FinishedAt
		open, err := r.openReadExists(ctx, userID, bookID)
		if err != nil {
			return domain.ReadSummary{}, err
		}
		sum.Rereading = open
	}
	return sum, nil
}

// ReadCounts returns finished-readings counts for many books in one query.
func (r *GormRepository) ReadCounts(ctx context.Context, userID uuid.UUID, bookIDs []uuid.UUID) (map[uuid.UUID]domain.ReadCount, error) {
	result := make(map[uuid.UUID]domain.ReadCount, len(bookIDs))
	if len(bookIDs) == 0 {
		return result, nil
	}
	var rows []domain.BookRead
	if err := r.db.WithContext(ctx).
		Select("book_id", "finished_at").
		Where("user_id = ? AND book_id IN ? AND finished_at IS NOT NULL", userID, bookIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		c := result[row.BookID]
		c.Count++
		if row.FinishedAt != nil && (c.LastReadAt == nil || row.FinishedAt.After(*c.LastReadAt)) {
			t := *row.FinishedAt
			c.LastReadAt = &t
		}
		result[row.BookID] = c
	}
	return result, nil
}

const readOrder = "COALESCE(started_at, finished_at, abandoned_at) ASC, created_at ASC"

// ListReads returns the user's attempts of a book, oldest first.
func (r *GormRepository) ListReads(ctx context.Context, userID, bookID uuid.UUID) ([]domain.BookRead, error) {
	var reads []domain.BookRead
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND book_id = ?", userID, bookID).
		Order(readOrder).Find(&reads).Error; err != nil {
		return nil, err
	}
	return reads, nil
}

// GetRead returns the user's attempt by id.
func (r *GormRepository) GetRead(ctx context.Context, userID, readID uuid.UUID) (*domain.BookRead, error) {
	var read domain.BookRead
	if err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", readID, userID).Take(&read).Error; err != nil {
		return nil, err
	}
	return &read, nil
}

// CountReads returns how many attempts the user has for a book.
func (r *GormRepository) CountReads(ctx context.Context, userID, bookID uuid.UUID) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&domain.BookRead{}).
		Where("user_id = ? AND book_id = ?", userID, bookID).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// InsertFinishedRead inserts a hand-entered finished attempt.
func (r *GormRepository) InsertFinishedRead(ctx context.Context, read *domain.BookRead) error {
	return r.db.WithContext(ctx).Create(read).Error
}

func stateCondition(state domain.ReadState) string {
	switch state {
	case domain.ReadStateFinished:
		return "finished_at IS NOT NULL"
	case domain.ReadStateAbandoned:
		return "abandoned_at IS NOT NULL"
	}
	return "finished_at IS NULL AND abandoned_at IS NULL"
}

// UpdateReadDates updates columns while the attempt is still in loadedState.
func (r *GormRepository) UpdateReadDates(ctx context.Context, userID, readID uuid.UUID, loadedState domain.ReadState, fields map[string]any) (int64, error) {
	res := r.db.WithContext(ctx).Model(&domain.BookRead{}).
		Where("id = ? AND user_id = ? AND "+stateCondition(loadedState), readID, userID).
		Updates(fields)
	return res.RowsAffected, res.Error
}

// DeleteClosedRead deletes a finished/abandoned attempt, never the open one.
func (r *GormRepository) DeleteClosedRead(ctx context.Context, userID, readID uuid.UUID) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND NOT (finished_at IS NULL AND abandoned_at IS NULL)", readID, userID).
		Delete(&domain.BookRead{})
	return res.RowsAffected, res.Error
}

// ShelfItemAddedAt returns when the book was put on the shelf.
func (r *GormRepository) ShelfItemAddedAt(ctx context.Context, shelfID, bookID uuid.UUID) (time.Time, bool, error) {
	var items []domain.ShelfItem
	if err := r.db.WithContext(ctx).
		Where("shelf_id = ? AND book_id = ?", shelfID, bookID).Limit(1).Find(&items).Error; err != nil {
		return time.Time{}, false, err
	}
	if len(items) == 0 {
		return time.Time{}, false, nil
	}
	return items[0].AddedAt, true, nil
}
