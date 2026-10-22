// Package service mirrors book covers into our own storage (#489): one
// copy attempt per book (MirrorOne), a periodic Sweep (backfill, retries,
// recovery after a lost volume, orphan cleanup) and the asynq tasks.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
}

// Option configures Service.
type Option func(*Service)

// WithClock overrides the clock and the pause (tests).
func WithClock(now func() time.Time, sleep func(ctx context.Context, d time.Duration) error) Option {
	return func(s *Service) { s.now, s.sleep = now, sleep }
}

func New(repo repository.Repository, store coverstore.Store, fetcher Fetcher, log *zap.Logger, opts ...Option) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	s := &Service{repo: repo, store: store, fetcher: fetcher, log: log, now: time.Now, sleep: sleepCtx}
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

// MirrorOne makes (or refreshes) the copy of one book's current cover link.
// Failures are recorded on the book and swallowed; only context errors and
// database errors are returned.
func (s *Service) MirrorOne(ctx context.Context, bookID uuid.UUID, reason string) error {
	book, err := s.repo.GetBook(ctx, bookID)
	if err != nil {
		return err
	}
	if book == nil || book.CoverURL == nil || strings.TrimSpace(*book.CoverURL) == "" {
		return nil
	}
	link := *book.CoverURL
	if book.HasStoredCover() {
		return nil
	}
	attempts := 0
	if reason != ReasonSave && book.CoverSourceURL != nil && *book.CoverSourceURL == link {
		attempts = book.CoverAttempts
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
	case ferr == nil && res != nil && res.Absent:
		st.Status = catalogDomain.CoverStatusAbsent
	case ferr == nil && res != nil:
		sum := sha256.Sum256(res.Data)
		h := hex.EncodeToString(sum[:])
		newName = h[:2] + "/" + h + "." + res.Ext
		if perr := s.store.Put(ctx, newName, res.Data); perr != nil {
			s.log.Warn("cover store put failed", zap.String("book_id", bookID.String()), zap.Error(perr))
			ferr = &coverfetch.Error{Code: coverfetch.CodeNetwork, Transient: true, Err: perr}
			newName = ""
		} else {
			st.Status = catalogDomain.CoverStatusStored
			st.Path = &newName
		}
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
			zap.String("book_id", bookID.String()), zap.String("error", code), zap.Bool("transient", fe.Transient), zap.String("status", st.Status))
	}

	applied, prev, err := s.repo.SaveState(ctx, bookID, st)
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

// collect deletes a stored file once no book refers to it.
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
}

// Sweep runs the four periodic steps: marker/reset, verify, mirror due
// candidates (at most batch), orphan cleanup.
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
			s.log.Warn("cover storage is new: stored copies reset to pending", zap.Int64("books", n))
		}
	}

	refs, err := s.repo.VerifyBatch(ctx, VerifyBatchSize)
	if err != nil {
		return stats, err
	}
	var ok []uuid.UUID
	for _, ref := range refs {
		exists, err := s.store.Exists(ctx, ref.Path)
		if err != nil && !errors.Is(err, coverstore.ErrInvalidName) {
			return stats, err
		}
		if exists {
			ok = append(ok, ref.ID)
			continue
		}
		if err := s.repo.MarkMissing(ctx, ref.ID, ref.Path, s.clock()); err != nil {
			return stats, err
		}
		stats.Missing++
	}
	if err := s.repo.TouchChecked(ctx, ok, s.clock()); err != nil {
		return stats, err
	}
	stats.Verified = len(ok)

	ids, err := s.repo.DueCandidates(ctx, s.clock(), batch)
	if err != nil {
		return stats, err
	}
	for _, id := range ids {
		if err := s.MirrorOne(ctx, id, ReasonSweep); err != nil {
			return stats, err
		}
		stats.Mirrored++
	}

	stats.Orphans, err = s.collectOrphans(ctx)
	if err != nil {
		s.log.Warn("cover orphan cleanup failed", zap.Error(err))
	}
	s.log.Info("cover sweep done", zap.Int64("reset", stats.Reset), zap.Int("missing", stats.Missing),
		zap.Int("verified", stats.Verified), zap.Int("mirrored", stats.Mirrored), zap.Int("orphans", stats.Orphans))
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
