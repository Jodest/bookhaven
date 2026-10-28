package repository

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// dropSet clears a TMDB poster with its copy and re-check state. poster_source
// goes back to NULL (never 'removed'), so a later sync may fill the work
// again. The two placeholders are poster_updated_at and updated_at.
const dropSet = `poster_source = NULL, poster_url = NULL, poster_author = NULL, poster_license = NULL,
	poster_license_url = NULL, poster_source_url = NULL,
	poster_copy_source_url = NULL, poster_copy_path = NULL, poster_copy_status = NULL, poster_copy_error = NULL,
	poster_copy_attempts = 0, poster_copy_next_attempt_at = NULL, poster_copy_fetched_at = NULL,
	poster_copy_checked_at = NULL, poster_tmdb_ref = NULL, poster_verified_at = NULL, poster_refresh_at = NULL,
	poster_updated_at = ?, updated_at = ?`

func (r *GormRepository) DropExpiredTMDB(ctx context.Context, cutoff, now time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Exec(`UPDATE screen_works SET `+dropSet+`
		WHERE poster_source = 'tmdb' AND COALESCE(poster_verified_at, poster_updated_at) < ?`, now, now, cutoff)
	return res.RowsAffected, res.Error
}

func (r *GormRepository) DueTMDB(ctx context.Context, now time.Time, limit int) ([]TMDBPoster, error) {
	var rows []struct {
		ID   uuid.UUID
		Link *string
		Ref  *string
	}
	err := r.db.WithContext(ctx).Table("screen_works").
		Select("id, poster_url AS link, poster_tmdb_ref AS ref").
		Where("poster_source = 'tmdb' AND (poster_refresh_at IS NULL OR poster_refresh_at <= ?)", now).
		Order("poster_refresh_at ASC NULLS FIRST, id ASC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]TMDBPoster, 0, len(rows))
	for _, row := range rows {
		p := TMDBPoster{ID: row.ID, Ref: row.Ref}
		if row.Link != nil {
			p.Link = *row.Link
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *GormRepository) DropTMDB(ctx context.Context, id uuid.UUID, link string, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Exec(`UPDATE screen_works SET `+dropSet+`
		WHERE id = ? AND poster_source = 'tmdb' AND poster_url = ?`, now, now, id, link)
	return res.RowsAffected > 0, res.Error
}

func (r *GormRepository) ConfirmTMDB(ctx context.Context, id uuid.UUID, link string, now, refreshAt time.Time) error {
	return r.db.WithContext(ctx).Exec(`UPDATE screen_works SET poster_verified_at = ?, poster_refresh_at = ?
		WHERE id = ? AND poster_source = 'tmdb' AND poster_url = ?`, now, refreshAt, id, link).Error
}

func (r *GormRepository) ReplaceTMDB(ctx context.Context, id uuid.UUID, oldLink, newLink string, now, refreshAt time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Exec(`UPDATE screen_works SET poster_url = ?,
		poster_copy_source_url = NULL, poster_copy_path = NULL, poster_copy_status = NULL, poster_copy_error = NULL,
		poster_copy_attempts = 0, poster_copy_next_attempt_at = NULL, poster_copy_fetched_at = NULL,
		poster_copy_checked_at = NULL, poster_verified_at = ?, poster_refresh_at = ?, poster_updated_at = ?, updated_at = ?
		WHERE id = ? AND poster_source = 'tmdb' AND poster_url = ?`,
		newLink, now, refreshAt, now, now, id, oldLink)
	return res.RowsAffected > 0, res.Error
}

func (r *GormRepository) DeferTMDB(ctx context.Context, id uuid.UUID, link string, at time.Time) error {
	return r.db.WithContext(ctx).Exec(`UPDATE screen_works SET poster_refresh_at = ?
		WHERE id = ? AND poster_source = 'tmdb' AND poster_url = ?`, at, id, link).Error
}

func (r *GormRepository) PurgeTMDB(ctx context.Context, dryRun bool, now time.Time) (int64, []string, error) {
	var posters int64
	var paths []string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("screen_works").Where("poster_source = 'tmdb'").Count(&posters).Error; err != nil {
			return err
		}
		var raw []string
		if err := tx.Table("screen_works").Where("poster_source = 'tmdb' AND poster_copy_path IS NOT NULL").
			Pluck("poster_copy_path", &raw).Error; err != nil {
			return err
		}
		seen := map[string]struct{}{}
		for _, p := range raw {
			if _, ok := seen[p]; !ok {
				seen[p] = struct{}{}
				paths = append(paths, p)
			}
		}
		sort.Strings(paths)
		if dryRun {
			return nil
		}
		return tx.Exec(`UPDATE screen_works SET `+dropSet+` WHERE poster_source = 'tmdb'`, now, now).Error
	})
	return posters, paths, err
}

func (r *GormRepository) PathReferencedOutsideTMDB(ctx context.Context, path string) (bool, error) {
	var books, works int64
	if err := r.db.WithContext(ctx).Table("books").Where("cover_path = ?", path).Count(&books).Error; err != nil {
		return false, err
	}
	if err := r.db.WithContext(ctx).Table("screen_works").
		Where("poster_copy_path = ? AND COALESCE(poster_source, '') <> 'tmdb'", path).Count(&works).Error; err != nil {
		return false, err
	}
	return books+works > 0, nil
}
