// Package service mirrors book covers and screen work posters into our own storage
// (#489): one copy attempt per row (MirrorOne), a periodic Sweep (backfill, retries,
// recovery after a lost volume, orphan cleanup) and the asynq tasks.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/rand"
	"strings"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/covers/domain"
	"github.com/bookhaven/backend/internal/modules/covers/repository"
	"github.com/bookhaven/backend/internal/pkg/coverfetch"
	"github.com/bookhaven/backend/internal/pkg/coverstore"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// ReasonSave marks a copy queued by a save: it resets the attempts.
	ReasonSave = "save"
	// ReasonSweep marks a copy picked by the sweep.
	ReasonSweep = "sweep"

	// MinDownloadInterval is the pause between two downloads.
	MinDownloadInterval = time.Second
	// VerifyBatchSize is the number of stored copies checked per sweep.
	VerifyBatchSize = 100
	// OrphanMinAge is how old an unreferenced file must be to be deleted.
	OrphanMinAge = time.Hour
)

// Fetcher downloads a cover.
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) (*coverfetch.Result, error)
}

// Service mirrors covers. It is used by one worker goroutine at a time (the
// covers queue has Concurrency 1), so pacing state needs no lock.
type Service struct {
	repo    repository.Repository
	store   coverstore.Store
	fetcher Fetcher
	log     *zap.Logger

	now      func() time.Time
	sleep    func(ctx context.Context, d time.Duration) error
	lastCall time.Time

	// tmdb re-checks TMDB posters; nil means no API key is configured.
	tmdb PosterLookup
	// rnd returns a value in [0, 1) for the re-check jitter.
	rnd func() float64
}

// Option configures Service.
type Option func(*Service)

// WithClock overrides the clock and the pause (tests).
func WithClock(now func() time.Time, sleep func(ctx context.Context, d time.Duration) error) Option {
	return func(s *Service) { s.now, s.sleep = now, sleep }
}

// WithTMDB enables the TMDB poster re-check with the given client.
func WithTMDB(l PosterLookup) Option {
	return func(s *Service) { s.tmdb = l }
}

// WithRand overrides the random source of the re-check jitter (tests).
func WithRand(rnd func() float64) Option {
	return func(s *Service) { s.rnd = rnd }
}

func New(repo repository.Repository, store coverstore.Store, fetcher Fetcher, log *zap.Logger, opts ...Option) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	s := &Service{repo: repo, store: store, fetcher: fetcher, log: log, now: time.Now, sleep: sleepCtx, rnd: rand.Float64}
	for _, o := range opts {
		o(s)
	}
	return s
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (s *Service) clock() time.Time { return s.now().UTC() }

func (s *Service) pace(ctx context.Context) error {
	if s.lastCall.IsZero() {
		return nil
	}
	if wait := MinDownloadInterval - s.now().Sub(s.lastCall); wait > 0 {
		return s.sleep(ctx, wait)
	}
	return nil
}

// MirrorOne makes (or refreshes) the copy of one row's current source link
// (a book cover or a screen work poster). Failures are recorded on the row and
// swallowed; only context errors and database errors are returned.
func (s *Service) MirrorOne(ctx context.Context, owner domain.Owner, id uuid.UUID, reason string) error {
	sub, err := s.repo.GetSubject(ctx, owner, id)
	if err != nil {
		return err
	}
	if sub == nil || strings.TrimSpace(sub.Link) == "" {
		return nil
	}
	link := sub.Link
	if sub.HasStoredCopy() {
		return nil
	}
	attempts := 0
	if reason != ReasonSave && sub.CopySourceURL != nil && *sub.CopySourceURL == link {
		attempts = sub.Attempts
	}

	if err := s.pace(ctx); err != nil {
		return err
	}
	res, ferr := s.fetcher.Fetch(ctx, link)
	s.lastCall = s.now()
	if ctx.Err() != nil {
		return ctx.Err()
	}

	now := s.clock()
	st := repository.StateUpdate{SourceURL: link, Now: now}
	var newName string
	switch {
	case ferr == nil && res != nil && res.Absent && owner == domain.OwnerBook:
		st.Status = catalogDomain.CoverStatusAbsent
	case ferr == nil && res != nil && !res.Absent:
		sum := sha256.Sum256(res.Data)
		h := hex.EncodeToString(sum[:])
		newName = h[:2] + "/" + h + "." + res.Ext
		if perr := s.store.Put(ctx, newName, res.Data); perr != nil {
			s.log.Warn("cover store put failed", zap.String("owner", string(owner)), zap.String("id", id.String()), zap.Error(perr))
			ferr = &coverfetch.Error{Code: coverfetch.CodeNetwork, Transient: true, Err: perr}
			newName = ""
		} else {
			st.Status = catalogDomain.CoverStatusStored
			st.Path = &newName
		}
	case ferr == nil:
		// "Absent" is a book-only answer (Open Library); for other owners it
		// is a permanent failure.
		ferr = &coverfetch.Error{Code: coverfetch.CodeHTTPError}
	}
	if st.Status == "" {
		var fe *coverfetch.Error
		if !errors.As(ferr, &fe) {
			fe = &coverfetch.Error{Code: coverfetch.CodeNetwork, Transient: true, Err: ferr}
		}
		code := fe.Code
		st.Error = &code
		if fe.Transient {
			status, n, next := domain.AfterTransientFailure(attempts, now)
			st.Status, st.Attempts, st.NextAttemptAt = status, n, &next
		} else {
			st.Status = catalogDomain.CoverStatusFailed
		}
		s.log.Info("cover copy failed",
			zap.String("owner", string(owner)), zap.String("id", id.String()),
			zap.String("error", code), zap.Bool("transient", fe.Transient), zap.String("status", st.Status))
	}

	applied, prev, err := s.repo.SaveState(ctx, owner, id, st)
	if err != nil {
		return err
	}
	if !applied {
		// The link changed while we downloaded: the new file stays for GC.
		return nil
	}
	if prev != nil && *prev != newName {
		s.collect(ctx, *prev)
	}
	return nil
}

// collect deletes a stored file once nothing refers to it.
func (s *Service) collect(ctx context.Context, name string) {
	used, err := s.repo.PathReferenced(ctx, name)
	if err != nil {
		s.log.Warn("cover gc: reference check failed", zap.String("path", name), zap.Error(err))
		return
	}
	if used {
		return
	}
	if err := s.store.Delete(ctx, name); err != nil {
		s.log.Warn("cover gc: delete failed", zap.String("path", name), zap.Error(err))
	}
}

// SweepStats summarises one sweep run.
type SweepStats struct {
	Reset    int64
	Missing  int
	Verified int
	Mirrored int
	Orphans  int
	// TMDBDropped counts TMDB posters dropped (hard limit and re-check),
	// TMDBChecked the re-checks that reached a verdict.
	TMDBDropped int
	TMDBChecked int
}

// Sweep runs the periodic steps: marker/reset, verify, mirror due candidates
// of every owner (at most batch), TMDB poster re-check, orphan cleanup.
func (s *Service) Sweep(ctx context.Context, batch int) (SweepStats, error) {
	var stats SweepStats
	if batch <= 0 {
		batch = 50
	}

	created, err := s.store.EnsureMarker(ctx)
	if err != nil {
		return stats, err
	}
	if created {
		n, err := s.repo.ResetAllStored(ctx, s.clock())
		if err != nil {
			return stats, err
		}
		stats.Reset = n
		if n > 0 {
			s.log.Warn("cover storage is new: stored copies reset to pending", zap.Int64("copies", n))
		}
	}

	refs, err := s.repo.VerifyBatch(ctx, VerifyBatchSize)
	if err != nil {
		return stats, err
	}
	ok := map[domain.Owner][]uuid.UUID{}
	verified := 0
	for _, ref := range refs {
		exists, err := s.store.Exists(ctx, ref.Path)
		if err != nil && !errors.Is(err, coverstore.ErrInvalidName) {
			return stats, err
		}
		if exists {
			ok[ref.Owner] = append(ok[ref.Owner], ref.ID)
			verified++
			continue
		}
		if err := s.repo.MarkMissing(ctx, ref.Owner, ref.ID, ref.Path, s.clock()); err != nil {
			return stats, err
		}
		stats.Missing++
	}
	for owner, ids := range ok {
		if err := s.repo.TouchChecked(ctx, owner, ids, s.clock()); err != nil {
			return stats, err
		}
	}
	stats.Verified = verified

	due, err := s.repo.DueCandidates(ctx, s.clock(), batch)
	if err != nil {
		return stats, err
	}
	for _, ref := range due {
		if err := s.MirrorOne(ctx, ref.Owner, ref.ID, ReasonSweep); err != nil {
			return stats, err
		}
		stats.Mirrored++
	}

	if err := s.refreshTMDB(ctx, &stats); err != nil {
		return stats, err
	}

	stats.Orphans, err = s.collectOrphans(ctx)
	if err != nil {
		s.log.Warn("cover orphan cleanup failed", zap.Error(err))
	}
	s.log.Info("cover sweep done", zap.Int64("reset", stats.Reset), zap.Int("missing", stats.Missing),
		zap.Int("verified", stats.Verified), zap.Int("mirrored", stats.Mirrored), zap.Int("orphans", stats.Orphans),
		zap.Int("tmdb_checked", stats.TMDBChecked), zap.Int("tmdb_dropped", stats.TMDBDropped))
	return stats, nil
}

func (s *Service) collectOrphans(ctx context.Context) (int, error) {
	refs, err := s.repo.ReferencedPaths(ctx)
	if err != nil {
		return 0, err
	}
	cutoff := s.clock().Add(-OrphanMinAge)
	var orphans []string
	err = s.store.List(ctx, func(name string, mod time.Time) error {
		if _, used := refs[name]; !used && mod.Before(cutoff) {
			orphans = append(orphans, name)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, name := range orphans {
		if err := s.store.Delete(ctx, name); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
