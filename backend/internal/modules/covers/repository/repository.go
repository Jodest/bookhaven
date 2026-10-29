// Package repository persists the copy state of the covers module: the
// books.cover_* columns (#489) and the screen_works.poster_copy_* columns.
// The domain models map these columns read-only ("->"); only this repository
// writes them, with raw UPDATEs guarded on the source link.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bookhaven/backend/internal/modules/covers/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// descriptor lists the table and columns that hold one owner's copy state.
// Adding an owner (for example author photos) is one more descriptor.
type descriptor struct {
	table    string
	link     string
	source   string
	path     string
	status   string
	errCol   string
	attempts string
	next     string
	stored   string
	checked  string
	// eligible is extra SQL a row must satisfy to be mirrored ("" for none).
	eligible string
}

var descriptors = map[domain.Owner]descriptor{
	domain.OwnerBook: {
		table: "books", link: "cover_url", source: "cover_source_url", path: "cover_path",
		status: "cover_status", errCol: "cover_error", attempts: "cover_attempts",
		next: "cover_next_attempt_at", stored: "cover_stored_at", checked: "cover_checked_at",
	},
	domain.OwnerScreenWorkPoster: {
		table: "screen_works", link: "poster_url", source: "poster_copy_source_url", path: "poster_copy_path",
		status: "poster_copy_status", errCol: "poster_copy_error", attempts: "poster_copy_attempts",
		next: "poster_copy_next_attempt_at", stored: "poster_copy_fetched_at", checked: "poster_copy_checked_at",
		eligible: "poster_source IN ('commons', 'tmdb')",
	},
}

// owners is the fixed iteration order (UNION branches, transactions).
var owners = []domain.Owner{domain.OwnerBook, domain.OwnerScreenWorkPoster}

func (d descriptor) and() string {
	if d.eligible == "" {
		return ""
	}
	return " AND " + d.eligible
}

// StateUpdate is the copy state written after a mirror attempt.
type StateUpdate struct {
	// SourceURL is the link the attempt was made for; the update applies only
	// while the owner's link still equals it.
	SourceURL     string
	Status        string
	Error         *string
	Attempts      int
	NextAttemptAt *time.Time
	// Path is set only for the stored status.
	Path *string
	Now  time.Time
}

// Subject is the mirror-relevant part of one owner row.
type Subject struct {
	// Link is the current source link ("" when none).
	Link          string
	CopySourceURL *string
	CopyPath      *string
	CopyStatus    *string
	Attempts      int
}

// HasStoredCopy reports whether a copy of the current link is stored.
func (s Subject) HasStoredCopy() bool {
	return s.Link != "" && s.CopySourceURL != nil && *s.CopySourceURL == s.Link &&
		s.CopyStatus != nil && *s.CopyStatus == "stored" && s.CopyPath != nil && *s.CopyPath != ""
}

// Ref identifies one owner row.
type Ref struct {
	Owner domain.Owner
	ID    uuid.UUID
}

// StoredRef is a stored copy to verify.
type StoredRef struct {
	Owner domain.Owner
	ID    uuid.UUID
	Path  string
}

// TMDBPoster is a TMDB poster row for the re-check.
type TMDBPoster struct {
	ID   uuid.UUID
	Link string
	// Ref is "movie/<id>" or "tv/<id>"; nil when none was kept.
	Ref *string
}

// Repository is the persistence the covers service needs.
type Repository interface {
	// GetSubject returns nil, nil when the row does not exist or is not
	// eligible for mirroring.
	GetSubject(ctx context.Context, owner domain.Owner, id uuid.UUID) (*Subject, error)
	// SaveState writes the state when the link is still SourceURL and returns
	// the previously stored path (nil when none).
	SaveState(ctx context.Context, owner domain.Owner, id uuid.UUID, st StateUpdate) (applied bool, prevPath *string, err error)
	// PathReferenced reports whether any owner refers to the stored path.
	PathReferenced(ctx context.Context, path string) (bool, error)
	// ReferencedPaths returns every stored path any owner refers to.
	ReferencedPaths(ctx context.Context) (map[string]struct{}, error)
	// DueCandidates returns rows that need a copy attempt, longest waiting first.
	DueCandidates(ctx context.Context, now time.Time, limit int) ([]Ref, error)
	// VerifyBatch returns stored copies, least recently checked first.
	VerifyBatch(ctx context.Context, limit int) ([]StoredRef, error)
	// MarkMissing sets a stored copy whose file vanished back to pending.
	MarkMissing(ctx context.Context, owner domain.Owner, id uuid.UUID, path string, now time.Time) error
	// TouchChecked records a successful verification.
	TouchChecked(ctx context.Context, owner domain.Owner, ids []uuid.UUID, now time.Time) error
	// ResetAllStored sets every stored copy of every owner to pending (lost or
	// new volume).
	ResetAllStored(ctx context.Context, now time.Time) (int64, error)

	// DropExpiredTMDB drops every TMDB poster last confirmed before cutoff.
	DropExpiredTMDB(ctx context.Context, cutoff, now time.Time) (int64, error)
	// DueTMDB returns TMDB posters whose re-check is due, earliest first.
	DueTMDB(ctx context.Context, now time.Time, limit int) ([]TMDBPoster, error)
	// DropTMDB drops one TMDB poster while its link is still link.
	DropTMDB(ctx context.Context, id uuid.UUID, link string, now time.Time) (bool, error)
	// ConfirmTMDB records a successful re-check of an unchanged poster.
	ConfirmTMDB(ctx context.Context, id uuid.UUID, link string, now, refreshAt time.Time) error
	// ReplaceTMDB swaps the poster link and resets its copy.
	ReplaceTMDB(ctx context.Context, id uuid.UUID, oldLink, newLink string, now, refreshAt time.Time) (bool, error)
	// DeferTMDB postpones a re-check, keeping the poster.
	DeferTMDB(ctx context.Context, id uuid.UUID, link string, at time.Time) error
	// PurgeTMDB drops every TMDB poster and returns the number of posters and
	// the distinct copy paths they held. With dryRun nothing changes.
	PurgeTMDB(ctx context.Context, dryRun bool, now time.Time) (posters int64, paths []string, err error)
	// PathReferencedOutsideTMDB reports whether a book or a non-TMDB poster
	// refers to the stored path (dry-run file count).
	PathReferencedOutsideTMDB(ctx context.Context, path string) (bool, error)
}

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) desc(owner domain.Owner) (descriptor, error) {
	d, ok := descriptors[owner]
	if !ok {
		return descriptor{}, fmt.Errorf("covers: unknown owner %q", owner)
	}
	return d, nil
}

func (r *GormRepository) GetSubject(ctx context.Context, owner domain.Owner, id uuid.UUID) (*Subject, error) {
	d, err := r.desc(owner)
	if err != nil {
		return nil, err
	}
	var row struct {
		Link          *string
		CopySourceURL *string
		CopyPath      *string
		CopyStatus    *string
		Attempts      int
	}
	err = r.db.WithContext(ctx).Table(d.table).
		Select(fmt.Sprintf("%s AS link, %s AS copy_source_url, %s AS copy_path, %s AS copy_status, %s AS attempts",
			d.link, d.source, d.path, d.status, d.attempts)).
		Where("id = ?"+d.and(), id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := &Subject{CopySourceURL: row.CopySourceURL, CopyPath: row.CopyPath, CopyStatus: row.CopyStatus, Attempts: row.Attempts}
	if row.Link != nil {
		s.Link = *row.Link
	}
	return s, nil
}

func (r *GormRepository) SaveState(ctx context.Context, owner domain.Owner, id uuid.UUID, st StateUpdate) (bool, *string, error) {
	d, err := r.desc(owner)
	if err != nil {
		return false, nil, err
	}
	var applied bool
	var prev *string
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ Path *string }
		if err := tx.Table(d.table).Select(d.path+" AS path").Where("id = ?", id).Take(&row).Error; err != nil {
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
		res := tx.Exec(fmt.Sprintf(`UPDATE %s SET %s = ?, %s = ?, %s = ?, %s = ?,
			%s = ?, %s = ?, %s = ?, %s = ?
			WHERE id = ? AND %s = ?%s`,
			d.table, d.source, d.path, d.status, d.errCol, d.attempts, d.next, d.stored, d.checked, d.link, d.and()),
			st.SourceURL, st.Path, st.Status, st.Error, st.Attempts, st.NextAttemptAt, storedAt, st.Now, id, st.SourceURL)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			applied = true
			prev = row.Path
		}
		return nil
	})
	return applied, prev, err
}

func (r *GormRepository) PathReferenced(ctx context.Context, path string) (bool, error) {
	for _, o := range owners {
		d := descriptors[o]
		var n int64
		if err := r.db.WithContext(ctx).Table(d.table).Where(d.path+" = ?", path).Count(&n).Error; err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (r *GormRepository) ReferencedPaths(ctx context.Context) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	for _, o := range owners {
		d := descriptors[o]
		var paths []string
		if err := r.db.WithContext(ctx).Table(d.table).Where(d.path+" IS NOT NULL").Pluck(d.path, &paths).Error; err != nil {
			return nil, err
		}
		for _, p := range paths {
			out[p] = struct{}{}
		}
	}
	return out, nil
}

func (r *GormRepository) DueCandidates(ctx context.Context, now time.Time, limit int) ([]Ref, error) {
	var parts []string
	var args []any
	for _, o := range owners {
		d := descriptors[o]
		parts = append(parts, fmt.Sprintf(`SELECT '%s' AS owner, id, %s AS next_at, created_at AS created FROM %s
			WHERE %s IS NOT NULL AND %s <> ''%s
			AND (COALESCE(%s, '') <> %s OR (%s IN ('pending', 'failed', 'linked') AND %s <= ?))`,
			o, d.next, d.table, d.link, d.link, d.and(), d.source, d.link, d.status, d.next))
		args = append(args, now)
	}
	args = append(args, limit)
	var rows []struct {
		Owner string
		ID    uuid.UUID
	}
	q := strings.Join(parts, " UNION ALL ") + " ORDER BY next_at ASC NULLS FIRST, created ASC LIMIT ?"
	if err := r.db.WithContext(ctx).Raw(q, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Ref, 0, len(rows))
	for _, row := range rows {
		out = append(out, Ref{Owner: domain.Owner(row.Owner), ID: row.ID})
	}
	return out, nil
}

func (r *GormRepository) VerifyBatch(ctx context.Context, limit int) ([]StoredRef, error) {
	var parts []string
	for _, o := range owners {
		d := descriptors[o]
		parts = append(parts, fmt.Sprintf(`SELECT '%s' AS owner, id, %s AS path, %s AS checked FROM %s
			WHERE %s = 'stored' AND %s IS NOT NULL`, o, d.path, d.checked, d.table, d.status, d.path))
	}
	var rows []struct {
		Owner string
		ID    uuid.UUID
		Path  string
	}
	q := strings.Join(parts, " UNION ALL ") + " ORDER BY checked ASC NULLS FIRST, id ASC LIMIT ?"
	if err := r.db.WithContext(ctx).Raw(q, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]StoredRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, StoredRef{Owner: domain.Owner(row.Owner), ID: row.ID, Path: row.Path})
	}
	return out, nil
}

func (r *GormRepository) MarkMissing(ctx context.Context, owner domain.Owner, id uuid.UUID, path string, now time.Time) error {
	d, err := r.desc(owner)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Exec(fmt.Sprintf(`UPDATE %s SET %s = 'pending', %s = NULL, %s = NULL,
		%s = NULL, %s = 0, %s = ?, %s = ?
		WHERE id = ? AND %s = 'stored' AND %s = ?`,
		d.table, d.status, d.path, d.stored, d.errCol, d.attempts, d.next, d.checked, d.status, d.path),
		now, now, id, path).Error
}

func (r *GormRepository) TouchChecked(ctx context.Context, owner domain.Owner, ids []uuid.UUID, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	d, err := r.desc(owner)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Exec(fmt.Sprintf("UPDATE %s SET %s = ? WHERE id IN ?", d.table, d.checked), now, ids).Error
}

func (r *GormRepository) ResetAllStored(ctx context.Context, now time.Time) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, o := range owners {
			d := descriptors[o]
			res := tx.Exec(fmt.Sprintf(`UPDATE %s SET %s = 'pending', %s = NULL, %s = NULL,
				%s = NULL, %s = 0, %s = ? WHERE %s = 'stored'`,
				d.table, d.status, d.path, d.stored, d.errCol, d.attempts, d.next, d.status), now)
			if res.Error != nil {
				return res.Error
			}
			total += res.RowsAffected
		}
		return nil
	})
	return total, err
}
