package repository

import (
	"context"
	"errors"
	"strings"
	"time"

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

// SourceAdaptation is the adaptation a verdict-metric item was scored by, with
// the vote figures frozen by the snapshot. It is read only while the book's
// link to the work is visible.
type SourceAdaptation struct {
	ID         uuid.UUID
	Title      string
	Year       *int
	Kind       string
	VotesCount int
	Share      float64
}

// AdminItem is a stored item with its book (authors preloaded).
type AdminItem struct {
	Position         int
	PreviousPosition *int
	Score            *float64
	Book             catalogDomain.Book
	Adaptation       *SourceAdaptation
}

// PublishedBook is a published book of a collection with its stored position
// and the position in the previous snapshot (auto collections).
type PublishedBook struct {
	Book             catalogDomain.Book
	Position         int
	PreviousPosition *int
	Adaptation       *SourceAdaptation
}

// BookPlace is the public place of a book in one published collection.
type BookPlace struct {
	Collection domain.Collection
	Place      int
}

// AutoSource gives a ranking callback the per-book aggregates of a
// collection's scope (published books, its genre, signals since the cut-off).
type AutoSource interface {
	AutoRows(ctx context.Context, c domain.Collection, since *time.Time) ([]domain.AutoRow, error)
	// VerdictRows returns one vote aggregate per visible adaptation of the
	// in-scope books (verdict metrics).
	VerdictRows(ctx context.Context, c domain.Collection, since *time.Time) ([]domain.VerdictRow, error)
}

// RankFunc computes the ordered snapshot of an auto collection from the
// source. It runs inside the Recompute transaction.
type RankFunc func(ctx context.Context, c domain.Collection, src AutoSource) ([]domain.ScoredBook, error)

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
	PublishedPage(ctx context.Context, id uuid.UUID, limit, offset int) ([]PublishedBook, error)
	// BookPlaces returns the public places of a book in published
	// collections, best place first. The numbers equal PublishedPage's order.
	BookPlaces(ctx context.Context, bookID uuid.UUID) ([]BookPlace, error)
	AdminItems(ctx context.Context, id uuid.UUID) ([]AdminItem, error)
	// GetGenre and GetBook return (nil, nil) when the row does not exist.
	GetGenre(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error)
	GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error)
	// AddBook inserts at position (nil = end, clamped) in one transaction
	// holding the collection row lock.
	AddBook(ctx context.Context, collectionID, bookID uuid.UUID, position *int) error
	RemoveBook(ctx context.Context, collectionID, bookID uuid.UUID) error
	MoveBook(ctx context.Context, collectionID, bookID uuid.UUID, position int) error
	// TitleCandidates returns up to limit books whose title contains every
	// word of key (a textnorm.Key), shortest titles first, authors
	// preloaded. The caller still compares the full normalized title.
	TitleCandidates(ctx context.Context, key string, limit int) ([]catalogDomain.Book, error)
	// CollectionBookIDs returns the ids of the books held by the collection.
	CollectionBookIDs(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error)
	// BooksByIDs returns the existing books among ids (authors not loaded).
	BooksByIDs(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Book, error)
	// AddBooks inserts the books as one contiguous block (position nil =
	// end, clamped) in one transaction holding the collection row lock.
	// Books already present are skipped and returned in AlreadyIn. When the
	// remaining books exceed the free places nothing is written and
	// ErrCollectionFull is returned.
	AddBooks(ctx context.Context, collectionID uuid.UUID, bookIDs []uuid.UUID, position *int) (BulkAdded, error)
	// AutoCollectionIDs returns the ids of every auto collection.
	AutoCollectionIDs(ctx context.Context) ([]uuid.UUID, error)
	// Recompute replaces the snapshot of an auto collection in one
	// transaction holding the row lock: it re-reads the collection (so the
	// rule is the one saved at that moment), ranks it through rank, keeps the
	// current positions as previous_position unless reset, writes the items
	// and sets computed_at/baseline_at. A failure leaves the old snapshot.
	// A manual collection returns ErrNotAutomatic.
	Recompute(ctx context.Context, id uuid.UUID, reset bool, now time.Time, rank RankFunc) error
}

// AddedBook is a book inserted by AddBooks with its final position.
type AddedBook struct {
	BookID   uuid.UUID
	Position int
}

// BulkAdded is the outcome of AddBooks.
type BulkAdded struct {
	Added     []AddedBook
	AlreadyIn []uuid.UUID
	// Total is the number of books in the collection after the call.
	Total int
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
	// computed_at/baseline_at belong to Recompute; a stale copy must not
	// overwrite a snapshot written meanwhile.
	err := r.db.WithContext(ctx).Omit("Genre", "ComputedAt", "BaselineAt").Save(c).Error
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

// NOTE: BookPlaces must keep the same order as this function
// ((position, book_id) over published books), otherwise badges and /top differ.
func (r *GormRepository) PublishedPage(ctx context.Context, id uuid.UUID, limit, offset int) ([]PublishedBook, error) {
	if limit <= 0 {
		return nil, nil
	}
	var books []catalogDomain.Book
	err := r.db.WithContext(ctx).Model(&catalogDomain.Book{}).
		Joins("JOIN curated_collection_items i ON i.book_id = books.id").
		Where("i.collection_id = ? AND books.status = ?", id, catalogDomain.BookStatusPublished).
		Order("i.position ASC, i.book_id ASC").
		Limit(limit).Offset(offset).
		Preload("Authors").Preload("Genres").
		Find(&books).Error
	if err != nil || len(books) == 0 {
		return nil, err
	}
	ids := make([]uuid.UUID, len(books))
	for i, b := range books {
		ids[i] = b.ID
	}
	var items []domain.CollectionItem
	if err := r.db.WithContext(ctx).Select("book_id", "position", "previous_position").
		Where("collection_id = ? AND book_id IN ?", id, ids).Find(&items).Error; err != nil {
		return nil, err
	}
	sources, err := r.sourceAdaptations(ctx, id, ids)
	if err != nil {
		return nil, err
	}
	byBook := make(map[uuid.UUID]domain.CollectionItem, len(items))
	for _, it := range items {
		byBook[it.BookID] = it
	}
	out := make([]PublishedBook, len(books))
	for i, b := range books {
		it := byBook[b.ID]
		out[i] = PublishedBook{Book: b, Position: it.Position, PreviousPosition: it.PreviousPosition, Adaptation: sources[b.ID]}
	}
	return out, nil
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
	sources, err := r.sourceAdaptations(ctx, id, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]catalogDomain.Book, len(books))
	for _, b := range books {
		byID[b.ID] = b
	}
	out := make([]AdminItem, 0, len(items))
	for _, it := range items {
		if b, ok := byID[it.BookID]; ok {
			out = append(out, AdminItem{Position: it.Position, PreviousPosition: it.PreviousPosition, Score: it.Score, Book: b, Adaptation: sources[it.BookID]})
		}
	}
	return out, nil
}

// sourceAdaptations reads the stored source adaptation of the given books of a
// collection. A hidden or deleted link gives no entry.
func (r *GormRepository) sourceAdaptations(ctx context.Context, id uuid.UUID, bookIDs []uuid.UUID) (map[uuid.UUID]*SourceAdaptation, error) {
	type row struct {
		BookID     uuid.UUID
		WorkID     uuid.UUID
		Title      string
		Year       *int
		Kind       string
		VotesCount int
		Share      float64
	}
	var rows []row
	err := r.db.WithContext(ctx).Table("curated_collection_items AS i").
		Select("i.book_id AS book_id, w.id AS work_id, w.title AS title, w.year AS year, w.kind AS kind, "+
			"i.source_votes AS votes_count, i.source_share AS share").
		Joins("JOIN book_screen_works l ON l.book_id = i.book_id AND l.screen_work_id = i.source_screen_work_id AND NOT l.hidden").
		Joins("JOIN screen_works w ON w.id = l.screen_work_id").
		Where("i.collection_id = ? AND i.book_id IN ? AND i.source_votes IS NOT NULL AND i.source_share IS NOT NULL", id, bookIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]*SourceAdaptation, len(rows))
	for _, x := range rows {
		out[x.BookID] = &SourceAdaptation{ID: x.WorkID, Title: x.Title, Year: x.Year, Kind: x.Kind, VotesCount: x.VotesCount, Share: x.Share}
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

// lock takes the collection row lock inside tx, serialising item edits and
// snapshot writes. It returns the collection kind.
func lock(tx *gorm.DB, collectionID uuid.UUID) (string, error) {
	var c domain.Collection
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "kind").Where("id = ?", collectionID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", domain.ErrCollectionNotFound
	}
	return c.Kind, err
}

// lockManual is lock for the manual book-list writes: an automatic
// collection's items are a snapshot and must not be edited by hand.
func lockManual(tx *gorm.DB, collectionID uuid.UUID) error {
	kind, err := lock(tx, collectionID)
	if err != nil {
		return err
	}
	if kind == domain.KindAuto {
		return domain.ErrCollectionAutomatic
	}
	return nil
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
		if err := lockManual(tx, collectionID); err != nil {
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
		if err := lockManual(tx, collectionID); err != nil {
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
		if err := lockManual(tx, collectionID); err != nil {
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

func (r *GormRepository) TitleCandidates(ctx context.Context, key string, limit int) ([]catalogDomain.Book, error) {
	words := strings.Fields(key)
	if len(words) == 0 || limit <= 0 {
		return nil, nil
	}
	q := r.db.WithContext(ctx).Model(&catalogDomain.Book{})
	if r.db.Dialector.Name() == "postgres" {
		// The expression must match idx_books_title_words (migration 000026).
		q = q.Where("to_tsvector('simple', replace(lower(title), 'ё', 'е')) @@ plainto_tsquery('simple', ?)", key)
	} else {
		// Fallback for engines without the word index (SQLite tests).
		for _, w := range words {
			q = q.Where("instr(replace(lower(title), 'ё', 'е'), ?) > 0", w)
		}
	}
	var books []catalogDomain.Book
	err := q.Order("length(title) ASC, id ASC").Limit(limit).Preload("Authors").Find(&books).Error
	return books, err
}

func (r *GormRepository) CollectionBookIDs(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).Model(&domain.CollectionItem{}).
		Where("collection_id = ?", id).Order("position ASC").Pluck("book_id", &ids).Error
	return ids, err
}

func (r *GormRepository) BooksByIDs(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Book, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var books []catalogDomain.Book
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&books).Error
	return books, err
}

func (r *GormRepository) AddBooks(ctx context.Context, collectionID uuid.UUID, bookIDs []uuid.UUID, position *int) (BulkAdded, error) {
	var res BulkAdded
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res = BulkAdded{}
		if err := lockManual(tx, collectionID); err != nil {
			return err
		}
		var present []uuid.UUID
		if err := tx.Model(&domain.CollectionItem{}).
			Where("collection_id = ?", collectionID).Pluck("book_id", &present).Error; err != nil {
			return err
		}
		have := make(map[uuid.UUID]bool, len(present))
		for _, id := range present {
			have[id] = true
		}
		var addable []uuid.UUID
		for _, id := range bookIDs {
			if have[id] {
				res.AlreadyIn = append(res.AlreadyIn, id)
				continue
			}
			have[id] = true
			addable = append(addable, id)
		}
		k, m := len(present), len(addable)
		res.Total = k
		if m > domain.MaxBooks-k {
			return domain.ErrCollectionFull
		}
		if m == 0 {
			return nil
		}
		pos := k + 1
		if position != nil {
			pos = clamp(*position, 1, k+1)
		}
		if err := tx.Model(&domain.CollectionItem{}).
			Where("collection_id = ? AND position >= ?", collectionID, pos).
			UpdateColumn("position", gorm.Expr("position + ?", m)).Error; err != nil {
			return err
		}
		rows := make([]domain.CollectionItem, m)
		for i, id := range addable {
			rows[i] = domain.CollectionItem{CollectionID: collectionID, BookID: id, Position: pos + i}
			res.Added = append(res.Added, AddedBook{BookID: id, Position: pos + i})
		}
		if err := tx.Create(&rows).Error; err != nil {
			if IsUniqueViolation(err) {
				return domain.ErrBookAlreadyIn
			}
			return err
		}
		res.Total = k + m
		return nil
	})
	if err != nil {
		return BulkAdded{}, err
	}
	return res, nil
}

// BookPlaces returns the places of a book in published collections: 1 plus the
// published books ahead of it by (position, book_id) - the same order as
// PublishedPage, which it must mirror. Places beyond the collection size
// (the reserve) are dropped. Best place first, then smaller size, newest, title.
func (r *GormRepository) BookPlaces(ctx context.Context, bookID uuid.UUID) ([]BookPlace, error) {
	type row struct {
		ID          uuid.UUID
		Slug        string
		Title       string
		Kind        string
		Size        int
		PublishedAt *time.Time
		Place       int
	}
	var rows []row
	err := r.db.WithContext(ctx).Raw(`
SELECT t.id, t.slug, t.title, t.kind, t.size, t.published_at, t.place FROM (
  SELECT c.id, c.slug, c.title, c.kind, c.size, c.published_at,
         1 + (SELECT COUNT(*)
              FROM curated_collection_items j
              JOIN books b ON b.id = j.book_id
              WHERE j.collection_id = i.collection_id
                AND b.status = ?
                AND (j.position < i.position
                     OR (j.position = i.position AND j.book_id < i.book_id))) AS place
  FROM curated_collection_items i
  JOIN curated_collections c ON c.id = i.collection_id
  WHERE i.book_id = ? AND c.is_published = ?
) t
WHERE t.place <= t.size
ORDER BY t.place ASC, t.size ASC, t.published_at DESC, t.title ASC`,
		catalogDomain.BookStatusPublished, bookID, true).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]BookPlace, len(rows))
	for i, rw := range rows {
		out[i] = BookPlace{
			Collection: domain.Collection{
				ID: rw.ID, Slug: rw.Slug, Title: rw.Title, Kind: rw.Kind, Size: rw.Size, PublishedAt: rw.PublishedAt,
			},
			Place: rw.Place,
		}
	}
	return out, nil
}
