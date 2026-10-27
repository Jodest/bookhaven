// Package repository stores screen works and their book links.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository is the persistence port of the screenworks service.
type Repository interface {
	VoteRepository

	// GetBooksForSync reads id, title, status and external ids of the books
	// that exist (read-only).
	GetBooksForSync(ctx context.Context, ids []uuid.UUID) ([]domain.BookRef, error)
	// ListCatalogSyncBookIDs returns the ids of every published book with a
	// Wikidata QID, oldest first (the catalog-wide sync job's selection).
	ListCatalogSyncBookIDs(ctx context.Context) ([]uuid.UUID, error)
	// BookStatus returns the status of a book or domain.ErrBookNotFound.
	BookStatus(ctx context.Context, id uuid.UUID) (string, error)
	// SyncBook stores the works and links them to the book in one
	// transaction. It returns the number of links created and of posters
	// filled.
	SyncBook(ctx context.Context, bookID uuid.UUID, works []domain.WorkInput) (SyncCounts, error)
	// PosterCandidates reports, for every given work QID, whether the sync
	// may fill its poster: true when the work is not stored yet or has no
	// poster state and was not edited by hand.
	PosterCandidates(ctx context.Context, qids []string) (map[string]bool, error)
	// ListPublicByBook returns the non-hidden links, ordered.
	ListPublicByBook(ctx context.Context, bookID uuid.UUID) ([]domain.LinkedWork, error)
	// ListAdminByBook returns every link including hidden ones, ordered.
	ListAdminByBook(ctx context.Context, bookID uuid.UUID) ([]domain.LinkedWork, error)
	// SetLinkHidden sets the hidden flag; domain.ErrLinkNotFound when the
	// link does not exist.
	SetLinkHidden(ctx context.Context, bookID, workID uuid.UUID, hidden bool) error

	// GetAuthorsForSync reads id, name, status and external ids of the
	// authors that exist (read-only).
	GetAuthorsForSync(ctx context.Context, ids []uuid.UUID) ([]domain.AuthorRef, error)
	// AuthorStatus returns the status of an author or domain.ErrAuthorNotFound.
	AuthorStatus(ctx context.Context, id uuid.UUID) (string, error)
	// SyncAuthor stores the films and links them to the author in one
	// transaction. It returns the number of links created.
	SyncAuthor(ctx context.Context, authorID uuid.UUID, films []domain.AuthorFilmInput) (int, error)
	// ListPublicByAuthor returns the non-hidden author links, ordered.
	ListPublicByAuthor(ctx context.Context, authorID uuid.UUID) ([]domain.LinkedWork, error)
	// ListAdminByAuthor returns every author link including hidden ones.
	ListAdminByAuthor(ctx context.Context, authorID uuid.UUID) ([]domain.LinkedWork, error)
	// SetAuthorLinkHidden sets the hidden flag; domain.ErrAuthorLinkNotFound
	// when the link does not exist.
	SetAuthorLinkHidden(ctx context.Context, authorID, workID uuid.UUID, hidden bool) error

	// Manual editing (openspec change add-screen-works-admin-editing).

	ListWorks(ctx context.Context, f domain.WorkFilter) ([]domain.WorkListItem, int64, error)
	GetWork(ctx context.Context, id uuid.UUID) (*domain.WorkDetail, error)
	// UpdateWork replaces the fields and sets manually_edited_at.
	UpdateWork(ctx context.Context, id uuid.UUID, f domain.WorkFields) error
	// DeleteWorkIfUnlinked: ErrWorkNotFound or ErrWorkHasLinks otherwise. It
	// returns the uploaded poster path of the deleted work (nil without one).
	DeleteWorkIfUnlinked(ctx context.Context, id uuid.UUID) (*string, error)
	// SetManualPoster stores an uploaded poster (source manual) and returns
	// the previous uploaded path (nil if none); ErrWorkNotFound. It never
	// touches manually_edited_at.
	SetManualPoster(ctx context.Context, id uuid.UUID, path string) (prevPath *string, err error)
	// RemovePoster turns manual into no poster and commons/tmdb into
	// removed; other states stay. It returns the removed uploaded path (nil
	// if none); ErrWorkNotFound.
	RemovePoster(ctx context.Context, id uuid.UUID) (prevPath *string, err error)
	// PosterPathReferenced reports whether any work still uses the path.
	PosterPathReferenced(ctx context.Context, path string) (bool, error)
	// AddManualBookLink resolves or creates the work and inserts a manual
	// link in one transaction; created is true for a new work.
	AddManualBookLink(ctx context.Context, bookID uuid.UUID, ref domain.WorkRef) (workID uuid.UUID, created bool, err error)
	AddManualAuthorLink(ctx context.Context, authorID uuid.UUID, ref domain.WorkRef, relation string) (workID uuid.UUID, created bool, err error)
	// DeleteManualBookLink: ErrLinkNotFound, ErrLinkNotManual.
	DeleteManualBookLink(ctx context.Context, bookID, workID uuid.UUID) error
	// DeleteManualAuthorLink: ErrAuthorLinkNotFound, ErrLinkNotManual.
	DeleteManualAuthorLink(ctx context.Context, authorID, workID uuid.UUID) error
	// UpdateAuthorLink sets hidden, relation or both (ErrAuthorLinkNotFound).
	UpdateAuthorLink(ctx context.Context, authorID, workID uuid.UUID, hidden *bool, relation *string) error
}

// GormRepository implements Repository.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a GormRepository.
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

type bookRow struct {
	ID             uuid.UUID `gorm:"column:id"`
	Title          string    `gorm:"column:title"`
	Status         string    `gorm:"column:status"`
	ExternalSource *string   `gorm:"column:external_source"`
	ExternalID     *string   `gorm:"column:external_id"`
}

func (r *GormRepository) GetBooksForSync(ctx context.Context, ids []uuid.UUID) ([]domain.BookRef, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []bookRow
	err := r.db.WithContext(ctx).Table("books").
		Select("id, title, status, external_source, external_id").
		Where("id IN ?", ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.BookRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.BookRef{
			ID: row.ID, Title: row.Title, Status: row.Status,
			ExternalSource: row.ExternalSource, ExternalID: row.ExternalID,
		})
	}
	return out, nil
}

func (r *GormRepository) ListCatalogSyncBookIDs(ctx context.Context) ([]uuid.UUID, error) {
	var rows []bookRow
	err := r.db.WithContext(ctx).Table("books").Select("id").
		Where("status = ? AND external_source = ? AND external_id IS NOT NULL", "published", "wikidata").
		Order("created_at, id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func (r *GormRepository) BookStatus(ctx context.Context, id uuid.UUID) (string, error) {
	var rows []bookRow
	err := r.db.WithContext(ctx).Table("books").Select("id, title, status").
		Where("id = ?", id).Limit(1).Scan(&rows).Error
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", domain.ErrBookNotFound
	}
	return rows[0].Status, nil
}

// SyncCounts are the counters of one book sync.
type SyncCounts struct {
	Added   int
	Posters int
}

func (r *GormRepository) SyncBook(ctx context.Context, bookID uuid.UUID, works []domain.WorkInput) (SyncCounts, error) {
	var counts SyncCounts
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, w := range works {
			workID, posters, err := upsertWikidataWork(tx, w)
			if err != nil {
				return err
			}
			counts.Posters += posters
			now := time.Now()
			res := tx.Exec(`INSERT INTO book_screen_works (book_id, screen_work_id, hidden, source, created_at, updated_at)
VALUES (?, ?, FALSE, ?, ?, ?) ON CONFLICT (book_id, screen_work_id) DO NOTHING`,
				bookID, workID, domain.SourceWikidata, now, now)
			if res.Error != nil {
				return res.Error
			}
			counts.Added += int(res.RowsAffected)
		}
		return nil
	})
	if err != nil {
		return SyncCounts{}, err
	}
	return counts, nil
}

func (r *GormRepository) PosterCandidates(ctx context.Context, qids []string) (map[string]bool, error) {
	out := make(map[string]bool, len(qids))
	for _, q := range qids {
		out[q] = true
	}
	if len(qids) == 0 {
		return out, nil
	}
	var rows []struct {
		ExternalID   string  `gorm:"column:external_id"`
		PosterSource *string `gorm:"column:poster_source"`
		Edited       bool    `gorm:"column:edited"`
	}
	err := r.db.WithContext(ctx).Table("screen_works").
		Select("external_id, poster_source, (manually_edited_at IS NOT NULL) AS edited").
		Where("external_source = ? AND external_id IN ?", domain.ExternalSourceWikidata, qids).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ExternalID] = row.PosterSource == nil && !row.Edited
	}
	return out, nil
}

// upsertWikidataWork inserts the work once per QID. For an existing work it
// fills only empty fields, skipping works edited by hand; title and kind
// never change. A poster is stored for a new work and filled into an existing
// one only while it has no poster state and was not edited by hand; the
// second result is the number of posters stored (0 or 1). A nil poster leaves
// the poster columns untouched.
func upsertWikidataWork(tx *gorm.DB, w domain.WorkInput) (uuid.UUID, int, error) {
	now := time.Now()
	var pSource, pURL, pAuthor, pLicense, pLicenseURL, pSourceURL, pUpdated any
	if p := w.Poster; p != nil {
		pSource, pURL, pAuthor, pLicense, pLicenseURL, pSourceURL = p.Source, p.URL, p.Author, p.License, p.LicenseURL, p.SourceURL
		pUpdated = now
	}
	res := tx.Exec(`INSERT INTO screen_works (id, title, original_title, kind, year, director, imdb_id, kinopoisk_id, external_source, external_id,
  poster_source, poster_url, poster_author, poster_license, poster_license_url, poster_source_url, poster_updated_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (external_source, external_id) WHERE external_id IS NOT NULL DO NOTHING`,
		uuid.New(), w.Title, w.OriginalTitle, w.Kind, w.Year, w.Director, w.IMDbID, w.KinopoiskID,
		domain.ExternalSourceWikidata, w.QID,
		pSource, pURL, pAuthor, pLicense, pLicenseURL, pSourceURL, pUpdated, now, now)
	if res.Error != nil {
		return uuid.Nil, 0, res.Error
	}
	created := res.RowsAffected > 0

	var rows []struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	if err := tx.Table("screen_works").Select("id").
		Where("external_source = ? AND external_id = ?", domain.ExternalSourceWikidata, w.QID).
		Limit(1).Scan(&rows).Error; err != nil {
		return uuid.Nil, 0, err
	}
	if len(rows) == 0 {
		return uuid.Nil, 0, errors.New("screen work vanished after upsert")
	}
	id := rows[0].ID

	hasFill := w.Year != nil || w.OriginalTitle != nil || w.Director != nil || w.IMDbID != nil || w.KinopoiskID != nil
	if !created && hasFill {
		err := tx.Exec(`UPDATE screen_works SET
  year = COALESCE(year, ?), original_title = COALESCE(original_title, ?), director = COALESCE(director, ?),
  imdb_id = COALESCE(imdb_id, ?), kinopoisk_id = COALESCE(kinopoisk_id, ?), updated_at = ?
WHERE id = ? AND manually_edited_at IS NULL`,
			w.Year, w.OriginalTitle, w.Director, w.IMDbID, w.KinopoiskID, now, id).Error
		if err != nil {
			return uuid.Nil, 0, err
		}
	}
	posters := 0
	if created && w.Poster != nil {
		posters = 1
	} else if !created && w.Poster != nil {
		res := tx.Exec(`UPDATE screen_works SET poster_source = ?, poster_url = ?, poster_author = ?, poster_license = ?,
  poster_license_url = ?, poster_source_url = ?, poster_updated_at = ?
WHERE id = ? AND poster_source IS NULL AND manually_edited_at IS NULL`,
			pSource, pURL, pAuthor, pLicense, pLicenseURL, pSourceURL, now, id)
		if res.Error != nil {
			return uuid.Nil, 0, res.Error
		}
		posters = int(res.RowsAffected)
	}
	return id, posters, nil
}

// posterSelect lists the poster columns of a screen work aliased w.
const posterSelect = "w.poster_source, w.poster_url, w.poster_path, w.poster_author, w.poster_license, w.poster_license_url, w.poster_source_url"

const linkedSelect = "w.id, w.title, w.original_title, w.kind, w.year, w.director, w.imdb_id, w.kinopoisk_id, w.external_id, l.hidden, l.source, " + posterSelect

// Year ascending with undated works last, then title; id keeps it stable.
const linkedOrder = "CASE WHEN w.year IS NULL THEN 1 ELSE 0 END, w.year, w.title, w.id"

func (r *GormRepository) list(ctx context.Context, bookID uuid.UUID, onlyVisible bool) ([]domain.LinkedWork, error) {
	q := r.db.WithContext(ctx).Table("book_screen_works AS l").
		Joins("JOIN screen_works w ON w.id = l.screen_work_id").
		Select(linkedSelect).Where("l.book_id = ?", bookID)
	if onlyVisible {
		q = q.Where("l.hidden = ?", false)
	}
	var out []domain.LinkedWork
	if err := q.Order(linkedOrder).Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *GormRepository) ListPublicByBook(ctx context.Context, bookID uuid.UUID) ([]domain.LinkedWork, error) {
	return r.list(ctx, bookID, true)
}

func (r *GormRepository) ListAdminByBook(ctx context.Context, bookID uuid.UUID) ([]domain.LinkedWork, error) {
	return r.list(ctx, bookID, false)
}

func (r *GormRepository) SetLinkHidden(ctx context.Context, bookID, workID uuid.UUID, hidden bool) error {
	res := r.db.WithContext(ctx).Exec(
		"UPDATE book_screen_works SET hidden = ?, updated_at = ? WHERE book_id = ? AND screen_work_id = ?",
		hidden, time.Now(), bookID, workID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrLinkNotFound
	}
	return nil
}
