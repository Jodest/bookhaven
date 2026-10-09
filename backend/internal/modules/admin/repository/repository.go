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
	DashboardCounts(ctx context.Context) (DashboardStats, error)

	// Books (every status).
	ListBooks(ctx context.Context, filter BookFilter) ([]catalogDomain.Book, int64, error)
	GetBookByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error)
	// GetBookByExternalID looks up a book by its (external_source,
	// external_id) idempotency key. Returns gorm.ErrRecordNotFound
	// (passthrough) when no matching book exists.
	GetBookByExternalID(ctx context.Context, source, id string) (*catalogDomain.Book, error)
	CreateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error
	UpdateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error
	BookSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error)
	// SetBookStatus updates only status (and updated_at) of one book.
	// found is false when no book has that id.
	SetBookStatus(ctx context.Context, id uuid.UUID, status string) (found bool, err error)

	// Authors.
	// ListAuthors lists authors of every status; status, when non-empty,
	// restricts the result to that status.
	ListAuthors(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Author, int64, error)
	GetAuthorByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error)
	GetAuthorBySlug(ctx context.Context, slug string) (*catalogDomain.Author, error)
	// GetAuthorByExternalID returns the author with the given
	// (external_source, external_id) identity, any status. Returns
	// gorm.ErrRecordNotFound when none exists.
	GetAuthorByExternalID(ctx context.Context, source, id string) (*catalogDomain.Author, error)
	// SetAuthorExternalID sets an author's external identity without
	// touching any other column (status included).
	SetAuthorExternalID(ctx context.Context, id uuid.UUID, source, externalID string) error
	// ListAuthorsByExternalIDs returns the authors (any status) whose
	// external identity is source plus one of ids, in a single query. An
	// empty ids slice returns an empty result without querying.
	ListAuthorsByExternalIDs(ctx context.Context, source string, ids []string) ([]catalogDomain.Author, error)
	CreateAuthor(ctx context.Context, author *catalogDomain.Author) error
	UpdateAuthor(ctx context.Context, author *catalogDomain.Author) error
	AuthorSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error)
	// MergeAuthors reassigns every book_authors row referencing duplicateID
	// to canonicalID - except rows for a book already linked to
	// canonicalID, which are dropped instead of reassigned, so no
	// book_authors row ever violates the (book_id, author_id) composite
	// primary key - then deletes the duplicate author row itself.
	// booksReassigned counts only the reassigned rows, not the dropped
	// duplicates. Callers (Service.MergeAuthors) must have already verified
	// both authors exist and are distinct; MergeAuthors itself does not
	// check either. In the same transaction, when the canonical author has
	// no external identity and the duplicate has one, the identity moves
	// to the canonical (after the duplicate row is deleted, so the partial
	// unique index is never violated).
	MergeAuthors(ctx context.Context, duplicateID, canonicalID uuid.UUID) (booksReassigned int64, err error)

	// Author moderation queue (docs/features/author-moderation-queue.md).
	// ListAuthorsForModeration lists authors matching filter, newest first,
	// each with its linked-books counts.
	ListAuthorsForModeration(ctx context.Context, filter AuthorModerationFilter) ([]AuthorModerationRow, int64, error)
	// ListAuthorSampleBooks returns up to sampleBooksPerAuthor linked books
	// (ordered by title) per author, for all authorIDs in one query.
	ListAuthorSampleBooks(ctx context.Context, authorIDs []uuid.UUID) (map[uuid.UUID][]SampleBook, error)
	// FindAuthorsBySurnames is a coarse SQL prefilter for similar-name
	// hints: non-rejected authors (excluding excludeIDs) whose lowercased
	// name equals a surname, ends with " <surname>" or starts with
	// "<surname>," / "<surname> ,". Callers re-check every result.
	FindAuthorsBySurnames(ctx context.Context, surnames []string, excludeIDs []uuid.UUID) ([]catalogDomain.Author, error)
	// SetAuthorStatus updates only status (and updated_at) of one author.
	// found is false when no author has that id.
	SetAuthorStatus(ctx context.Context, id uuid.UUID, status string) (found bool, err error)

	// Author enrichment (docs/features/author-wikidata-enrichment.md).
	// GetAuthorsByIDs returns the authors (any status) with the given ids
	// in one query; unknown ids are absent. Empty ids → empty result.
	GetAuthorsByIDs(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Author, error)
	// FillAuthorEnrichment writes only the non-nil parts of fill, each
	// only where the column is still empty (atomic per column in one
	// UPDATE); the photo attribution columns follow the photo_url
	// condition. A fill with no parts is a no-op.
	FillAuthorEnrichment(ctx context.Context, id uuid.UUID, fill AuthorEnrichmentFill) error

	// Genres. Alias rows (merged_into_id set, see
	// docs/features/genre-moderation-queue.md) are hidden from ListGenres,
	// GetGenreByID, GenresExist and the dashboard count, but GetGenreBySlug,
	// GetGenreByQID and GenreSlugExists still return them, so resolvers can
	// follow an alias and its slug/QID stay reserved.
	// ListGenres lists genres of every status; status, when non-empty,
	// restricts the result to that status.
	ListGenres(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Genre, int64, error)
	// GetGenreByID returns a non-alias genre; an alias id gives
	// gorm.ErrRecordNotFound.
	GetGenreByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error)
	// GetGenreBySlug returns the genre with the slug, alias rows included.
	GetGenreBySlug(ctx context.Context, slug string) (*catalogDomain.Genre, error)
	// GetGenreByQID returns the genre with the given Wikidata QID, any
	// status, alias rows included. Returns gorm.ErrRecordNotFound when none
	// exists.
	GetGenreByQID(ctx context.Context, qid string) (*catalogDomain.Genre, error)
	// ListGenresByQIDs returns the genres (any status, alias rows
	// included) whose wikidata_qid is one of qids, in one query. Empty
	// input returns an empty slice without a query.
	ListGenresByQIDs(ctx context.Context, qids []string) ([]catalogDomain.Genre, error)
	// SetGenreQID sets a genre's wikidata_qid without touching any other
	// column (status included).
	SetGenreQID(ctx context.Context, id uuid.UUID, qid string) error
	CreateGenre(ctx context.Context, genre *catalogDomain.Genre) error
	// UpdateGenre persists only the PATCH-editable columns (name, slug,
	// status, wikidata_qid) of a non-alias genre. It never writes
	// merged_into_id, so a PATCH racing a merge cannot resurrect the alias.
	// Returns gorm.ErrRecordNotFound when no non-alias row has genre.ID.
	UpdateGenre(ctx context.Context, genre *catalogDomain.Genre) error
	GenreSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error)

	// Genre moderation queue (docs/features/genre-moderation-queue.md).
	// ListGenresForModeration lists non-alias genres matching filter,
	// newest first, each with its linked-books counts.
	ListGenresForModeration(ctx context.Context, filter GenreModerationFilter) ([]GenreModerationRow, int64, error)
	// ListGenreSampleBooks returns up to sampleBooksPerAuthor linked books
	// (ordered by title) per genre, for all genreIDs in one query.
	ListGenreSampleBooks(ctx context.Context, genreIDs []uuid.UUID) (map[uuid.UUID][]SampleBook, error)
	// ListGenreHintCandidates returns every non-rejected, non-alias genre
	// (the similar-name hint pool) in one query.
	ListGenreHintCandidates(ctx context.Context) ([]catalogDomain.Genre, error)
	// SetGenreStatus updates only the status of one non-alias genre. found
	// is false when no such genre exists (unknown id or alias).
	SetGenreStatus(ctx context.Context, id uuid.UUID, status string) (found bool, err error)
	// MergeGenres turns sourceID into an alias of targetID in one
	// transaction (see genre_moderation.go). Returns gorm.ErrRecordNotFound,
	// changing nothing, when either genre is missing or already an alias.
	MergeGenres(ctx context.Context, sourceID, targetID uuid.UUID) (booksReassigned int64, err error)

	// Book moderation queue (docs/features/book-moderation-queue.md).
	// ListBooksForModeration lists books matching filter, newest first,
	// with authors and genres preloaded.
	ListBooksForModeration(ctx context.Context, filter BookModerationFilter) ([]catalogDomain.Book, int64, error)
	// FindBooksByNormalizedISBNs returns books whose ISBN, with hyphens and
	// spaces stripped and uppercased, is in isbns, excluding excludeIDs.
	// Empty isbns returns an empty slice without a query.
	FindBooksByNormalizedISBNs(ctx context.Context, isbns []string, excludeIDs []uuid.UUID) ([]BookHintCandidate, error)
	// FindBooksByAuthorIDs returns books linked to any of authorIDs
	// (excluding excludeIDs), each with the matched author ids. Empty
	// authorIDs returns an empty slice without a query.
	FindBooksByAuthorIDs(ctx context.Context, authorIDs, excludeIDs []uuid.UUID) ([]BookHintCandidate, error)

	// Reviews moderation.
	ListReviews(ctx context.Context, filter ReviewFilter) ([]reviewsDomain.Review, int64, error)
	GetReviewByID(ctx context.Context, id uuid.UUID) (*reviewsDomain.Review, error)
	// ModerateReview hides or restores a review and recalculates the book's
	// reviews_count in the same transaction. It is idempotent.
	ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error)

	// Users (docs/features/admin-users.md).
	// ListUsers lists users matching filter, newest first.
	ListUsers(ctx context.Context, filter UserFilter) ([]usersDomain.User, int64, error)
	// ChangeUserRole runs the locked role-change transaction (see users.go);
	// decide owns the business rules. Returns the user after the change and
	// its previous role.
	ChangeUserRole(ctx context.Context, targetID uuid.UUID, decide RoleDecider) (*usersDomain.User, usersDomain.Role, error)

	// Relationship validation helpers.
	AuthorsExist(ctx context.Context, ids []uuid.UUID) (bool, error)
	GenresExist(ctx context.Context, ids []uuid.UUID) (bool, error)
}

// BookFilter defines filtering options for admin book list.
type BookFilter struct {
	Search string
	// Status is "" (all) or a valid book status (pending, published,
	// rejected); see docs/features/book-status-model.md rule 6.
	Status   string
	GenreID  *uuid.UUID
	AuthorID *uuid.UUID
	// Source is one of "" (all), "manual" (external_source IS NULL), or
	// "harvested" (external_source IS NOT NULL) - a coarse manual/harvested
	// split, independent of and ANDed with Status (see
	// docs/features/admin-book-list-filters.md).
	Source string
	Limit  int
	Offset int
}

// DashboardStats holds the admin dashboard counters.
type DashboardStats struct {
	Books          int64
	PendingBooks   int64
	RejectedBooks  int64
	Authors        int64
	Genres         int64
	PendingReviews int64
	PendingAuthors int64
	PendingGenres  int64
}

// AuthorModerationFilter defines filtering options for the author
// moderation queue. Status is a valid author status or "" (any); QID is ""
// (any), "with" or "without"; Books is "" (any) or "none". Search uses the
// same full-text condition as ListAuthors.
type AuthorModerationFilter struct {
	Status string
	QID    string
	Books  string
	Search string
	Limit  int
	Offset int
}

// AuthorModerationRow is one author of the moderation queue with its
// linked-books counts (books_visible = linked books with status = 'published').
type AuthorModerationRow struct {
	Author       catalogDomain.Author
	BooksTotal   int64
	BooksVisible int64
}

// SampleBook is a short linked-book reference shown in the moderation queue.
type SampleBook struct {
	ID     uuid.UUID
	Title  string
	Status string
}

// sampleBooksPerAuthor bounds ListAuthorSampleBooks per author.
const sampleBooksPerAuthor = 3

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

func (r *GormRepository) DashboardCounts(ctx context.Context) (DashboardStats, error) {
	db := r.db.WithContext(ctx)
	var st DashboardStats

	if err := db.Model(&catalogDomain.Book{}).Count(&st.Books).Error; err != nil {
		return DashboardStats{}, err
	}
	if err := db.Model(&catalogDomain.Book{}).Where("status = ?", catalogDomain.BookStatusPending).Count(&st.PendingBooks).Error; err != nil {
		return DashboardStats{}, err
	}
	if err := db.Model(&catalogDomain.Book{}).Where("status = ?", catalogDomain.BookStatusRejected).Count(&st.RejectedBooks).Error; err != nil {
		return DashboardStats{}, err
	}
	if err := db.Model(&catalogDomain.Author{}).Count(&st.Authors).Error; err != nil {
		return DashboardStats{}, err
	}
	// Alias rows (merged genres) are not genres of their own.
	if err := db.Model(&catalogDomain.Genre{}).Where("merged_into_id IS NULL").Count(&st.Genres).Error; err != nil {
		return DashboardStats{}, err
	}
	if err := db.Model(&reviewsDomain.Review{}).Where("is_hidden = ?", true).Count(&st.PendingReviews).Error; err != nil {
		return DashboardStats{}, err
	}
	if err := db.Model(&catalogDomain.Author{}).Where("status = ?", catalogDomain.AuthorStatusPending).Count(&st.PendingAuthors).Error; err != nil {
		return DashboardStats{}, err
	}
	if err := db.Model(&catalogDomain.Genre{}).Where("status = ?", catalogDomain.GenreStatusPending).Count(&st.PendingGenres).Error; err != nil {
		return DashboardStats{}, err
	}

	return st, nil
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

	if filter.Status != "" {
		query = query.Where("books.status = ?", filter.Status)
	}

	if filter.GenreID != nil {
		query = query.Joins("JOIN book_genres ON book_genres.book_id = books.id").
			Where("book_genres.genre_id = ?", filter.GenreID)
	}
	if filter.AuthorID != nil {
		query = query.Joins("JOIN book_authors ON book_authors.book_id = books.id").
			Where("book_authors.author_id = ?", filter.AuthorID)
	}
	switch filter.Source {
	case "manual":
		query = query.Where("external_source IS NULL")
	case "harvested":
		query = query.Where("external_source IS NOT NULL")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	// maxBookListLimit bounds Limit from above, matching the admin books
	// list page-size cap (parsePagination caps requested limits at 100
	// before building BookFilter).
	const maxBookListLimit = 100
	if limit > maxBookListLimit {
		limit = maxBookListLimit
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

func (r *GormRepository) ListAuthors(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Author, int64, error) {
	var authors []catalogDomain.Author
	var total int64

	query := r.db.WithContext(ctx).Model(&catalogDomain.Author{})

	if status != "" {
		query = query.Where("status = ?", status)
	}

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

func (r *GormRepository) GetAuthorByExternalID(ctx context.Context, source, id string) (*catalogDomain.Author, error) {
	var author catalogDomain.Author
	if err := r.db.WithContext(ctx).
		Where("external_source = ? AND external_id = ?", source, id).
		First(&author).Error; err != nil {
		return nil, err
	}
	return &author, nil
}

func (r *GormRepository) SetAuthorExternalID(ctx context.Context, id uuid.UUID, source, externalID string) error {
	return r.db.WithContext(ctx).Model(&catalogDomain.Author{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"external_source": source, "external_id": externalID}).Error
}

func (r *GormRepository) ListAuthorsByExternalIDs(ctx context.Context, source string, ids []string) ([]catalogDomain.Author, error) {
	if len(ids) == 0 {
		return []catalogDomain.Author{}, nil
	}
	var authors []catalogDomain.Author
	if err := r.db.WithContext(ctx).
		Where("external_source = ? AND external_id IN ?", source, ids).
		Find(&authors).Error; err != nil {
		return nil, err
	}
	return authors, nil
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

// MergeAuthors implements Repository.MergeAuthors as a single transaction,
// matching replaceBookAuthors' existing raw-SQL-inside-tx.Transaction
// style:
//  1. Reassign book_authors rows the canonical author doesn't already
//     have, from duplicateID to canonicalID.
//  2. Delete any remaining book_authors rows still referencing
//     duplicateID (books already linked to both authors before the merge
//     - now a redundant duplicate-author row).
//  3. Delete the duplicate author row.
//  4. If the canonical author has no external identity, copy the
//     duplicate's (read before step 3) onto it. Done after step 3 so the
//     (external_source, external_id) partial unique index never sees two
//     rows with the same identity. A canonical that already has an
//     identity keeps it; the duplicate's is dropped.
//
// Step 3's ON DELETE CASCADE on book_authors.author_id is not relied upon
// for correctness - step 2 already leaves nothing to cascade - so the
// explicit order never depends on cascade timing.
func (r *GormRepository) MergeAuthors(ctx context.Context, duplicateID, canonicalID uuid.UUID) (int64, error) {
	var booksReassigned int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var dup struct {
			ExternalSource *string
			ExternalID     *string
		}
		if err := tx.Raw("SELECT external_source, external_id FROM authors WHERE id = ?", duplicateID).
			Scan(&dup).Error; err != nil {
			return err
		}

		result := tx.Exec(`
			UPDATE book_authors
			SET author_id = ?
			WHERE author_id = ?
			AND book_id NOT IN (SELECT book_id FROM book_authors WHERE author_id = ?)
		`, canonicalID, duplicateID, canonicalID)
		if result.Error != nil {
			return result.Error
		}
		booksReassigned = result.RowsAffected

		if err := tx.Exec("DELETE FROM book_authors WHERE author_id = ?", duplicateID).Error; err != nil {
			return err
		}

		if err := tx.Exec("DELETE FROM authors WHERE id = ?", duplicateID).Error; err != nil {
			return err
		}

		if dup.ExternalSource == nil || dup.ExternalID == nil {
			return nil
		}
		return tx.Exec(
			"UPDATE authors SET external_source = ?, external_id = ? WHERE id = ? AND external_source IS NULL",
			*dup.ExternalSource, *dup.ExternalID, canonicalID,
		).Error
	})
	if err != nil {
		return 0, err
	}
	return booksReassigned, nil
}

func (r *GormRepository) ListGenres(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Genre, int64, error) {
	var genres []catalogDomain.Genre
	var total int64

	// Alias rows (merged genres) never appear in admin lists.
	query := r.db.WithContext(ctx).Model(&catalogDomain.Genre{}).Where("merged_into_id IS NULL")

	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
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
	if err := r.db.WithContext(ctx).First(&genre, "id = ? AND merged_into_id IS NULL", id).Error; err != nil {
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

func (r *GormRepository) GetGenreByQID(ctx context.Context, qid string) (*catalogDomain.Genre, error) {
	var genre catalogDomain.Genre
	if err := r.db.WithContext(ctx).First(&genre, "wikidata_qid = ?", qid).Error; err != nil {
		return nil, err
	}
	return &genre, nil
}

func (r *GormRepository) ListGenresByQIDs(ctx context.Context, qids []string) ([]catalogDomain.Genre, error) {
	if len(qids) == 0 {
		return []catalogDomain.Genre{}, nil
	}
	var genres []catalogDomain.Genre
	if err := r.db.WithContext(ctx).Where("wikidata_qid IN ?", qids).Find(&genres).Error; err != nil {
		return nil, err
	}
	return genres, nil
}

func (r *GormRepository) SetGenreQID(ctx context.Context, id uuid.UUID, qid string) error {
	return r.db.WithContext(ctx).Model(&catalogDomain.Genre{}).
		Where("id = ?", id).
		Update("wikidata_qid", qid).Error
}

func (r *GormRepository) CreateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	return r.db.WithContext(ctx).Create(genre).Error
}

func (r *GormRepository) UpdateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	res := r.db.WithContext(ctx).Model(&catalogDomain.Genre{}).
		Where("id = ? AND merged_into_id IS NULL", genre.ID).
		Updates(map[string]any{
			"name":         genre.Name,
			"slug":         genre.Slug,
			"status":       genre.Status,
			"wikidata_qid": genre.WikidataQID,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
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
		BookTitle     string `gorm:"column:book_title"`
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
	// An alias id counts as unknown: books never link alias rows.
	if err := r.db.WithContext(ctx).Model(&catalogDomain.Genre{}).Where("id IN ? AND merged_into_id IS NULL", ids).Count(&count).Error; err != nil {
		return false, err
	}
	return count == int64(len(ids)), nil
}
