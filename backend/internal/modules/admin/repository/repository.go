package repository

import (
	"context"
	"fmt"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository defines admin-only catalog and moderation operations.
type Repository interface {
	// Dashboard counts.
	DashboardCounts(ctx context.Context) (books, hiddenBooks, authors, genres, pendingReviews int64, err error)

	// Books (includes hidden records).
	ListBooks(ctx context.Context, filter BookFilter) ([]catalogDomain.Book, int64, error)
	GetBookByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error)
	// GetBookByExternalID looks up a book by its (external_source,
	// external_id) idempotency key. Returns gorm.ErrRecordNotFound
	// (passthrough) when no matching book exists.
	GetBookByExternalID(ctx context.Context, source, id string) (*catalogDomain.Book, error)
	CreateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error
	UpdateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error
	BookSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error)

	// Authors.
	ListAuthors(ctx context.Context, search string, limit, offset int) ([]catalogDomain.Author, int64, error)
	GetAuthorByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error)
	GetAuthorBySlug(ctx context.Context, slug string) (*catalogDomain.Author, error)
	CreateAuthor(ctx context.Context, author *catalogDomain.Author) error
	UpdateAuthor(ctx context.Context, author *catalogDomain.Author) error
	AuthorSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error)

	// Genres.
	ListGenres(ctx context.Context, search string, limit, offset int) ([]catalogDomain.Genre, int64, error)
	GetGenreByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error)
	GetGenreBySlug(ctx context.Context, slug string) (*catalogDomain.Genre, error)
	CreateGenre(ctx context.Context, genre *catalogDomain.Genre) error
	UpdateGenre(ctx context.Context, genre *catalogDomain.Genre) error
	GenreSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error)

	// Reviews moderation.
	ListReviews(ctx context.Context, filter ReviewFilter) ([]reviewsDomain.Review, int64, error)
	GetReviewByID(ctx context.Context, id uuid.UUID) (*reviewsDomain.Review, error)
	// ModerateReview hides or restores a review and recalculates the book's
	// reviews_count in the same transaction. It is idempotent.
	ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error)

	// Relationship validation helpers.
	AuthorsExist(ctx context.Context, ids []uuid.UUID) (bool, error)
	GenresExist(ctx context.Context, ids []uuid.UUID) (bool, error)
}

// BookFilter defines filtering options for admin book list.
type BookFilter struct {
	Search     string
	Visibility string // "", "visible", "hidden"
	Limit      int
	Offset     int
}

// ReviewFilter defines filtering options for review moderation.
type ReviewFilter struct {
	Visibility string // "", "visible", "hidden"
	Limit      int
	Offset     int
}

// GormRepository is a GORM-based implementation of Repository.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a new GormRepository.
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

func (r *GormRepository) DashboardCounts(ctx context.Context) (books, hiddenBooks, authors, genres, pendingReviews int64, err error) {
	db := r.db.WithContext(ctx)

	if err = db.Model(&catalogDomain.Book{}).Count(&books).Error; err != nil {
		return 0, 0, 0, 0, 0, err
	}
	if err = db.Model(&catalogDomain.Book{}).Where("is_hidden = ?", true).Count(&hiddenBooks).Error; err != nil {
		return 0, 0, 0, 0, 0, err
	}
	if err = db.Model(&catalogDomain.Author{}).Count(&authors).Error; err != nil {
		return 0, 0, 0, 0, 0, err
	}
	if err = db.Model(&catalogDomain.Genre{}).Count(&genres).Error; err != nil {
		return 0, 0, 0, 0, 0, err
	}
	if err = db.Model(&reviewsDomain.Review{}).Where("is_hidden = ?", true).Count(&pendingReviews).Error; err != nil {
		return 0, 0, 0, 0, 0, err
	}

	return books, hiddenBooks, authors, genres, pendingReviews, nil
}

func (r *GormRepository) ListBooks(ctx context.Context, filter BookFilter) ([]catalogDomain.Book, int64, error) {
	var books []catalogDomain.Book
	var total int64

	query := r.db.WithContext(ctx).Model(&catalogDomain.Book{}).
		Preload("Authors").
		Preload("Genres")

	if filter.Search != "" {
		query = query.Where(
			"to_tsvector('russian', title || ' ' || COALESCE(isbn, '') || ' ' || COALESCE(description, '')) @@ plainto_tsquery('russian', ?)",
			filter.Search,
		)
	}

	switch filter.Visibility {
	case "visible":
		query = query.Where("is_hidden = ?", false)
	case "hidden":
		query = query.Where("is_hidden = ?", true)
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

	if err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&books).Error; err != nil {
		return nil, 0, err
	}

	return books, total, nil
}

func (r *GormRepository) GetBookByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error) {
	var book catalogDomain.Book
	if err := r.db.WithContext(ctx).
		Preload("Authors").
		Preload("Genres").
		First(&book, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &book, nil
}

func (r *GormRepository) GetBookByExternalID(ctx context.Context, source, id string) (*catalogDomain.Book, error) {
	var book catalogDomain.Book
	if err := r.db.WithContext(ctx).
		First(&book, "external_source = ? AND external_id = ?", source, id).Error; err != nil {
		return nil, err
	}
	return &book, nil
}

func (r *GormRepository) CreateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(book).Error; err != nil {
			return err
		}
		if err := r.replaceBookAuthors(tx, book.ID, authorIDs); err != nil {
			return err
		}
		return r.replaceBookGenres(tx, book.ID, genreIDs)
	})
}

func (r *GormRepository) UpdateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Authors", "Genres").Save(book).Error; err != nil {
			return err
		}
		if err := r.replaceBookAuthors(tx, book.ID, authorIDs); err != nil {
			return err
		}
		return r.replaceBookGenres(tx, book.ID, genreIDs)
	})
}

func (r *GormRepository) replaceBookAuthors(tx *gorm.DB, bookID uuid.UUID, authorIDs []uuid.UUID) error {
	if err := tx.Exec("DELETE FROM book_authors WHERE book_id = ?", bookID).Error; err != nil {
		return err
	}
	if len(authorIDs) == 0 {
		return nil
	}
	values := make([]map[string]interface{}, 0, len(authorIDs))
	for _, id := range authorIDs {
		values = append(values, map[string]interface{}{"book_id": bookID, "author_id": id})
	}
	return tx.Table("book_authors").Create(values).Error
}

func (r *GormRepository) replaceBookGenres(tx *gorm.DB, bookID uuid.UUID, genreIDs []uuid.UUID) error {
	if err := tx.Exec("DELETE FROM book_genres WHERE book_id = ?", bookID).Error; err != nil {
		return err
	}
	if len(genreIDs) == 0 {
		return nil
	}
	values := make([]map[string]interface{}, 0, len(genreIDs))
	for _, id := range genreIDs {
		values = append(values, map[string]interface{}{"book_id": bookID, "genre_id": id})
	}
	return tx.Table("book_genres").Create(values).Error
}

func (r *GormRepository) BookSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&catalogDomain.Book{}).Where("slug = ?", slug)
	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *GormRepository) ListAuthors(ctx context.Context, search string, limit, offset int) ([]catalogDomain.Author, int64, error) {
	var authors []catalogDomain.Author
	var total int64

	query := r.db.WithContext(ctx).Model(&catalogDomain.Author{})

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
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	if err := query.Order("name ASC").
		Limit(limit).
		Offset(offset).
		Find(&authors).Error; err != nil {
		return nil, 0, err
	}

	return authors, total, nil
}

func (r *GormRepository) GetAuthorByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error) {
	var author catalogDomain.Author
	if err := r.db.WithContext(ctx).First(&author, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &author, nil
}

func (r *GormRepository) GetAuthorBySlug(ctx context.Context, slug string) (*catalogDomain.Author, error) {
	var author catalogDomain.Author
	if err := r.db.WithContext(ctx).First(&author, "slug = ?", slug).Error; err != nil {
		return nil, err
	}
	return &author, nil
}

func (r *GormRepository) CreateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	return r.db.WithContext(ctx).Create(author).Error
}

func (r *GormRepository) UpdateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	return r.db.WithContext(ctx).Save(author).Error
}

func (r *GormRepository) AuthorSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&catalogDomain.Author{}).Where("slug = ?", slug)
	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *GormRepository) ListGenres(ctx context.Context, search string, limit, offset int) ([]catalogDomain.Genre, int64, error) {
	var genres []catalogDomain.Genre
	var total int64

	query := r.db.WithContext(ctx).Model(&catalogDomain.Genre{})

	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

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

	if err := query.Order("name ASC").
		Limit(limit).
		Offset(offset).
		Find(&genres).Error; err != nil {
		return nil, 0, err
	}

	return genres, total, nil
}

func (r *GormRepository) GetGenreByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error) {
	var genre catalogDomain.Genre
	if err := r.db.WithContext(ctx).First(&genre, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &genre, nil
}

func (r *GormRepository) GetGenreBySlug(ctx context.Context, slug string) (*catalogDomain.Genre, error) {
	var genre catalogDomain.Genre
	if err := r.db.WithContext(ctx).First(&genre, "slug = ?", slug).Error; err != nil {
		return nil, err
	}
	return &genre, nil
}

func (r *GormRepository) CreateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	return r.db.WithContext(ctx).Create(genre).Error
}

func (r *GormRepository) UpdateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	return r.db.WithContext(ctx).Save(genre).Error
}

func (r *GormRepository) GenreSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&catalogDomain.Genre{}).Where("slug = ?", slug)
	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *GormRepository) ListReviews(ctx context.Context, filter ReviewFilter) ([]reviewsDomain.Review, int64, error) {
	var total int64

	countQuery := r.db.WithContext(ctx).Model(&reviewsDomain.Review{})
	switch filter.Visibility {
	case "visible":
		countQuery = countQuery.Where("is_hidden = ?", false)
	case "hidden":
		countQuery = countQuery.Where("is_hidden = ?", true)
	}
	if err := countQuery.Count(&total).Error; err != nil {
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

	visibilityWhere := ""
	args := []interface{}{limit, offset}
	switch filter.Visibility {
	case "visible":
		visibilityWhere = "AND r.is_hidden = false"
	case "hidden":
		visibilityWhere = "AND r.is_hidden = true"
	}

	var rows []struct {
		ID            uuid.UUID
		BookID        uuid.UUID
		BookTitle     string  `gorm:"column:book_title"`
		UserID        uuid.UUID
		Rating        int
		Content       string
		LikesCount    int `gorm:"column:likes_count"`
		IsHidden      bool
		CreatedAt     time.Time
		UpdatedAt     time.Time
		UserName      *string `gorm:"column:user_name"`
		UserAvatarURL *string `gorm:"column:user_avatar_url"`
	}

	sql := fmt.Sprintf(`
		SELECT
			r.id,
			r.book_id,
			b.title AS book_title,
			r.user_id,
			r.rating,
			r.content,
			r.likes_count,
			r.is_hidden,
			r.created_at,
			r.updated_at,
			u.name AS user_name,
			u.avatar_url AS user_avatar_url
		FROM reviews r
		JOIN users u ON u.id = r.user_id
		JOIN books b ON b.id = r.book_id
		WHERE 1=1 %s
		ORDER BY r.created_at DESC
		LIMIT ? OFFSET ?
	`, visibilityWhere)

	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	reviews := make([]reviewsDomain.Review, 0, len(rows))
	for _, row := range rows {
		reviews = append(reviews, reviewsDomain.Review{
			ID:         row.ID,
			BookID:     row.BookID,
			BookTitle:  row.BookTitle,
			UserID:     row.UserID,
			Rating:     row.Rating,
			Content:    row.Content,
			LikesCount: row.LikesCount,
			IsHidden:   row.IsHidden,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
			Author: &usersDomain.User{
				ID:        row.UserID,
				Name:      row.UserName,
				AvatarURL: row.UserAvatarURL,
			},
		})
	}

	return reviews, total, nil
}

func (r *GormRepository) GetReviewByID(ctx context.Context, id uuid.UUID) (*reviewsDomain.Review, error) {
	var review reviewsDomain.Review
	if err := r.db.WithContext(ctx).First(&review, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &review, nil
}

func (r *GormRepository) ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error) {
	var review reviewsDomain.Review
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&review, "id = ?", id).Error; err != nil {
			return err
		}
		if review.IsHidden == isHidden {
			return nil
		}
		if err := tx.Model(&reviewsDomain.Review{}).
			Where("id = ?", id).
			Update("is_hidden", isHidden).Error; err != nil {
			return err
		}
		review.IsHidden = isHidden
		return tx.Exec(`
			UPDATE books
			SET reviews_count = (SELECT COUNT(*) FROM reviews WHERE book_id = ? AND is_hidden = false)
			WHERE id = ?
		`, review.BookID, review.BookID).Error
	})
	if err != nil {
		return nil, err
	}
	return &review, nil
}

func (r *GormRepository) AuthorsExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
	if len(ids) == 0 {
		return true, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&catalogDomain.Author{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
		return false, err
	}
	return count == int64(len(ids)), nil
}

func (r *GormRepository) GenresExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
	if len(ids) == 0 {
		return true, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&catalogDomain.Genre{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
		return false, err
	}
	return count == int64(len(ids)), nil
}
