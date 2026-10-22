// Package repository persists the cover copy state kept on the books table
// (#489). The catalog Book maps these columns read-only ("->"); only this
// repository writes them, with raw UPDATEs guarded on the source link.
package repository

import (
	"context"
	"errors"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StateUpdate is the copy state written after a mirror attempt.
type StateUpdate struct {
	// SourceURL is the link the attempt was made for; the update applies only
	// while books.cover_url still equals it.
	SourceURL     string
	Status        string
	Error         *string
	Attempts      int
	NextAttemptAt *time.Time
	// Path is set only for the stored status.
	Path *string
	Now  time.Time
}

// StoredRef is a stored copy to verify.
type StoredRef struct {
	ID   uuid.UUID
	Path string
}

// Repository is the persistence the covers service needs.
type Repository interface {
	// GetBook returns nil, nil when the book does not exist.
	GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error)
	// SaveState writes the state when the cover link is still SourceURL and
	// returns the previously stored path (nil when none).
	SaveState(ctx context.Context, id uuid.UUID, st StateUpdate) (applied bool, prevPath *string, err error)
	// PathReferenced reports whether any book refers to the stored path.
	PathReferenced(ctx context.Context, path string) (bool, error)
	// ReferencedPaths returns every stored path any book refers to.
	ReferencedPaths(ctx context.Context) (map[string]struct{}, error)
	// DueCandidates returns ids of books that need a copy attempt.
	DueCandidates(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error)
	// VerifyBatch returns stored copies, least recently checked first.
	VerifyBatch(ctx context.Context, limit int) ([]StoredRef, error)
	// MarkMissing sets a stored copy whose file vanished back to pending.
	MarkMissing(ctx context.Context, id uuid.UUID, path string, now time.Time) error
	// TouchChecked records a successful verification.
	TouchChecked(ctx context.Context, ids []uuid.UUID, now time.Time) error
	// ResetAllStored sets every stored copy to pending (lost or new volume).
	ResetAllStored(ctx context.Context, now time.Time) (int64, error)
}

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error) {
	var b catalogDomain.Book
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *GormRepository) SaveState(ctx context.Context, id uuid.UUID, st StateUpdate) (bool, *string, error) {
	var applied bool
	var prev *string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ CoverPath *string }
		if err := tx.Table("books").Select("cover_path").Where("id = ?", id).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var storedAt *time.Time
		if st.Path != nil {
			t := st.Now
			storedAt = &t
		}
		res := tx.Exec(`UPDATE books SET cover_source_url = ?, cover_path = ?, cover_status = ?, cover_error = ?,
			cover_attempts = ?, cover_next_attempt_at = ?, cover_stored_at = ?, cover_checked_at = ?
			WHERE id = ? AND cover_url = ?`,
			st.SourceURL, st.Path, st.Status, st.Error, st.Attempts, st.NextAttemptAt, storedAt, st.Now, id, st.SourceURL)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			applied = true
			prev = row.CoverPath
		}
		return nil
	})
	return applied, prev, err
}

func (r *GormRepository) PathReferenced(ctx context.Context, path string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("books").Where("cover_path = ?", path).Count(&n).Error
	return n > 0, err
}

func (r *GormRepository) ReferencedPaths(ctx context.Context) (map[string]struct{}, error) {
	var paths []string
	if err := r.db.WithContext(ctx).Table("books").Where("cover_path IS NOT NULL").Pluck("cover_path", &paths).Error; err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		out[p] = struct{}{}
	}
	return out, nil
}

func (r *GormRepository) DueCandidates(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).Table("books").
		Where("cover_url IS NOT NULL AND cover_url <> ''").
		Where("(COALESCE(cover_source_url, '') <> cover_url OR (cover_status IN ('pending', 'failed') AND cover_next_attempt_at <= ?))", now).
		Order("cover_next_attempt_at ASC NULLS FIRST, created_at ASC").
		Limit(limit).
		Pluck("id", &ids).Error
	return ids, err
}

func (r *GormRepository) VerifyBatch(ctx context.Context, limit int) ([]StoredRef, error) {
	var rows []struct {
		ID   uuid.UUID
		Path string
	}
	err := r.db.WithContext(ctx).Table("books").Select("id, cover_path AS path").
		Where("cover_status = 'stored' AND cover_path IS NOT NULL").
		Order("cover_checked_at ASC NULLS FIRST, id ASC").
		Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]StoredRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, StoredRef{ID: r.ID, Path: r.Path})
	}
	return out, nil
}

func (r *GormRepository) MarkMissing(ctx context.Context, id uuid.UUID, path string, now time.Time) error {
	return r.db.WithContext(ctx).Exec(`UPDATE books SET cover_status = 'pending', cover_path = NULL, cover_stored_at = NULL,
		cover_error = NULL, cover_attempts = 0, cover_next_attempt_at = ?, cover_checked_at = ?
		WHERE id = ? AND cover_status = 'stored' AND cover_path = ?`, now, now, id, path).Error
}

func (r *GormRepository) TouchChecked(ctx context.Context, ids []uuid.UUID, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Exec("UPDATE books SET cover_checked_at = ? WHERE id IN ?", now, ids).Error
}

func (r *GormRepository) ResetAllStored(ctx context.Context, now time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Exec(`UPDATE books SET cover_status = 'pending', cover_path = NULL, cover_stored_at = NULL,
		cover_error = NULL, cover_attempts = 0, cover_next_attempt_at = ? WHERE cover_status = 'stored'`, now)
	return res.RowsAffected, res.Error
}
