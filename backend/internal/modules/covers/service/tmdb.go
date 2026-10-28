package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bookhaven/backend/internal/modules/covers/domain"
	"github.com/bookhaven/backend/internal/pkg/tmdb"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// TMDBRefreshBatch is the number of TMDB posters re-checked per sweep.
	TMDBRefreshBatch = 20
	// TMDBRefreshMin and TMDBRefreshJitter spread the next re-check over
	// [150, 164) days after a poster was set or confirmed.
	TMDBRefreshMin    = 150 * 24 * time.Hour
	TMDBRefreshJitter = 14 * 24 * time.Hour
	// TMDBMaxAge is the longest a TMDB poster is kept without a successful
	// confirmation (TMDB API terms: no cached content older than 6 months).
	TMDBMaxAge = 175 * 24 * time.Hour
	// TMDBRetryAfter is the pause after a TMDB outage.
	TMDBRetryAfter = 24 * time.Hour
)

// PosterLookup resolves a TMDB poster URL; kind is "movie" or "tv". It
// returns "" when TMDB has no poster. Satisfied by *tmdb.Client.
type PosterLookup interface {
	Poster(ctx context.Context, kind, id string) (string, error)
}

func (s *Service) nextRefresh(now time.Time) time.Time {
	return now.Add(TMDBRefreshMin + time.Duration(s.rnd()*float64(TMDBRefreshJitter)))
}

// refreshTMDB enforces the TMDB terms: the 175-day hard limit first, then the
// re-check of up to TMDBRefreshBatch due posters. Freed copy files are left to
// the orphan cleanup that follows in the sweep.
func (s *Service) refreshTMDB(ctx context.Context, stats *SweepStats) error {
	now := s.clock()
	n, err := s.repo.DropExpiredTMDB(ctx, now.Add(-TMDBMaxAge), now)
	if err != nil {
		return err
	}
	if n > 0 {
		stats.TMDBDropped += int(n)
		s.log.Warn("tmdb posters dropped: not confirmed within the limit", zap.Int64("posters", n))
	}

	due, err := s.repo.DueTMDB(ctx, now, TMDBRefreshBatch)
	if err != nil {
		return err
	}
	unauthorized := false
	for _, p := range due {
		if err := ctx.Err(); err != nil {
			return err
		}
		now = s.clock()
		kind, id, ok := splitTMDBRef(p.Ref)
		if s.tmdb == nil || !ok {
			if err := s.dropTMDB(ctx, p.ID, p.Link, stats); err != nil {
				return err
			}
			continue
		}
		link, lerr := s.tmdb.Poster(ctx, kind, id)
		switch {
		case errors.Is(lerr, tmdb.ErrUnauthorized) || errors.Is(lerr, tmdb.ErrUnavailable):
			if errors.Is(lerr, tmdb.ErrUnauthorized) && !unauthorized {
				unauthorized = true
				s.log.Error("tmdb re-check: the api key was rejected")
			}
			if err := s.repo.DeferTMDB(ctx, p.ID, p.Link, now.Add(TMDBRetryAfter)); err != nil {
				return err
			}
		case lerr != nil:
			// Invalid query or an unknown error: retry later like an outage.
			s.log.Warn("tmdb re-check failed", zap.String("id", p.ID.String()), zap.Error(lerr))
			if err := s.repo.DeferTMDB(ctx, p.ID, p.Link, now.Add(TMDBRetryAfter)); err != nil {
				return err
			}
		case link == "":
			if err := s.dropTMDB(ctx, p.ID, p.Link, stats); err != nil {
				return err
			}
			stats.TMDBChecked++
		case link == p.Link:
			if err := s.repo.ConfirmTMDB(ctx, p.ID, p.Link, now, s.nextRefresh(now)); err != nil {
				return err
			}
			stats.TMDBChecked++
		default:
			applied, err := s.repo.ReplaceTMDB(ctx, p.ID, p.Link, link, now, s.nextRefresh(now))
			if err != nil {
				return err
			}
			stats.TMDBChecked++
			if applied {
				if err := s.MirrorOne(ctx, domain.OwnerScreenWorkPoster, p.ID, ReasonSave); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Service) dropTMDB(ctx context.Context, id uuid.UUID, link string, stats *SweepStats) error {
	dropped, err := s.repo.DropTMDB(ctx, id, link, s.clock())
	if err != nil {
		return err
	}
	if dropped {
		stats.TMDBDropped++
	}
	return nil
}

func splitTMDBRef(ref *string) (kind, id string, ok bool) {
	if ref == nil {
		return "", "", false
	}
	k, i, found := strings.Cut(*ref, "/")
	if !found || (k != tmdb.KindMovie && k != tmdb.KindTV) || i == "" {
		return "", "", false
	}
	return k, i, true
}

// PurgeStats is the outcome of PurgeTMDB.
type PurgeStats struct {
	PostersRemoved int64
	FilesDeleted   int
}

// PurgeTMDB removes every TMDB poster with its copy and deletes the copy files
// that nothing refers to any more. With dryRun it only counts. Files that
// stay referenced by another row are neither deleted nor counted.
func (s *Service) PurgeTMDB(ctx context.Context, dryRun bool) (PurgeStats, error) {
	posters, paths, err := s.repo.PurgeTMDB(ctx, dryRun, s.clock())
	stats := PurgeStats{PostersRemoved: posters}
	if err != nil {
		return stats, err
	}
	for _, name := range paths {
		if dryRun {
			// Nothing was removed: a file is freed only if no non-TMDB row
			// shares it. Count it when the TMDB rows are its only references.
			shared, err := s.repo.PathReferencedOutsideTMDB(ctx, name)
			if err != nil {
				return stats, err
			}
			if !shared {
				stats.FilesDeleted++
			}
			continue
		}
		used, err := s.repo.PathReferenced(ctx, name)
		if err != nil {
			return stats, err
		}
		if used {
			continue
		}
		if err := s.store.Delete(ctx, name); err != nil {
			return stats, err
		}
		stats.FilesDeleted++
	}
	return stats, nil
}
