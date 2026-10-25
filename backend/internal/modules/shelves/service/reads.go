package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrReadNotFound         = errors.New("reading not found")
	ErrInvalidDate          = errors.New("invalid date")
	ErrDateInFuture         = errors.New("date in the future")
	ErrFinishBeforeStart    = errors.New("finish date before start date")
	ErrFinishRequired       = errors.New("finished_on is required")
	ErrStartRequired        = errors.New("start date required for an open reading")
	ErrFieldNotEditable     = errors.New("field not editable")
	ErrOpenReadNotDeletable = errors.New("open reading cannot be deleted")
	ErrReadChanged          = errors.New("reading changed")
	ErrTooManyReads         = errors.New("too many readings")
)

// MaxReadsPerBook bounds the journal rows of one (user, book).
const MaxReadsPerBook = 100

const dateLayout = "2006-01-02"

var minReadDate = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)

// OptionalDate is a PATCH field that distinguishes absent from null.
type OptionalDate struct {
	Set   bool
	Value *string // nil with Set == true means an explicit null
}

// ReadPatch is the set of fields of a reading-history edit.
type ReadPatch struct {
	StartedOn   OptionalDate
	FinishedOn  OptionalDate
	AbandonedOn OptionalDate
}

// parseDay parses YYYY-MM-DD, applies the 1900 and "not in the future" bounds
// and returns the day stored at 12:00 UTC.
func (s *Service) parseDay(raw string) (time.Time, error) {
	d, err := time.Parse(dateLayout, raw)
	if err != nil || d.Before(minReadDate) {
		return time.Time{}, ErrInvalidDate
	}
	// UTC+14 is the earliest time zone: the reader's own today is always valid.
	limit := s.now().UTC().Add(14 * time.Hour)
	limitDay := time.Date(limit.Year(), limit.Month(), limit.Day(), 0, 0, 0, 0, time.UTC)
	if d.After(limitDay) {
		return time.Time{}, ErrDateInFuture
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, time.UTC), nil
}

func utcDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ListReads returns the caller's attempts of a book.
func (s *Service) ListReads(ctx context.Context, userID, bookID uuid.UUID) ([]domain.BookRead, error) {
	reads, err := s.repo.ListReads(ctx, userID, bookID)
	if err != nil {
		return nil, fmt.Errorf("list reads: %w", err)
	}
	return reads, nil
}

// AddPastRead records a hand-entered finished attempt. Shelves and other
// attempts are never touched.
func (s *Service) AddPastRead(ctx context.Context, userID, bookID uuid.UUID, startedOn *string, finishedOn string) (*domain.BookRead, error) {
	if finishedOn == "" {
		return nil, ErrFinishRequired
	}
	finished, err := s.parseDay(finishedOn)
	if err != nil {
		return nil, err
	}
	var started *time.Time
	if startedOn != nil {
		st, err := s.parseDay(*startedOn)
		if err != nil {
			return nil, err
		}
		if utcDay(finished).Before(utcDay(st)) {
			return nil, ErrFinishBeforeStart
		}
		started = &st
	}

	exists, err := s.repo.BookExists(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("check book exists: %w", err)
	}
	if !exists {
		return nil, ErrBookNotFound
	}

	read := &domain.BookRead{
		ID: uuid.New(), UserID: userID, BookID: bookID,
		StartedAt: started, FinishedAt: &finished, CreatedAt: s.now(), ManualEnd: true,
	}
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		n, err := repo.CountReads(ctx, userID, bookID)
		if err != nil {
			return err
		}
		if n >= MaxReadsPerBook {
			return ErrTooManyReads
		}
		return repo.InsertFinishedRead(ctx, read)
	})
	if err != nil {
		if errors.Is(err, ErrTooManyReads) {
			return nil, err
		}
		return nil, fmt.Errorf("add past read: %w", err)
	}
	return read, nil
}

// UpdateRead changes the dates of one of the caller's attempts. The state of an
// attempt never changes.
func (s *Service) UpdateRead(ctx context.Context, userID, readID uuid.UUID, patch ReadPatch) (*domain.BookRead, error) {
	if !patch.StartedOn.Set && !patch.FinishedOn.Set && !patch.AbandonedOn.Set {
		return nil, ErrNothingToUpdate
	}
	read, err := s.repo.GetRead(ctx, userID, readID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReadNotFound
		}
		return nil, fmt.Errorf("get read: %w", err)
	}
	state := read.State()

	var endField OptionalDate
	var endColumn string
	var currentEnd *time.Time
	switch state {
	case domain.ReadStateOpen:
		if patch.FinishedOn.Set || patch.AbandonedOn.Set {
			return nil, ErrFieldNotEditable
		}
		if patch.StartedOn.Value == nil {
			return nil, ErrStartRequired
		}
	case domain.ReadStateFinished:
		if patch.AbandonedOn.Set {
			return nil, ErrFieldNotEditable
		}
		endField, endColumn, currentEnd = patch.FinishedOn, "finished_at", read.FinishedAt
	case domain.ReadStateAbandoned:
		if patch.FinishedOn.Set {
			return nil, ErrFieldNotEditable
		}
		endField, endColumn, currentEnd = patch.AbandonedOn, "abandoned_at", read.AbandonedAt
	}

	fields := map[string]any{}
	start := read.StartedAt
	if patch.StartedOn.Set {
		if patch.StartedOn.Value == nil {
			start = nil
			fields["started_at"] = nil
		} else {
			st, err := s.parseDay(*patch.StartedOn.Value)
			if err != nil {
				return nil, err
			}
			start = &st
			fields["started_at"] = st
		}
	}
	end := currentEnd
	if endField.Set {
		if endField.Value == nil {
			return nil, ErrFinishRequired
		}
		e, err := s.parseDay(*endField.Value)
		if err != nil {
			return nil, err
		}
		end = &e
		fields[endColumn] = e
		fields["manual_end"] = true
	}
	if start != nil && end != nil && utcDay(*end).Before(utcDay(*start)) {
		return nil, ErrFinishBeforeStart
	}

	n, err := s.repo.UpdateReadDates(ctx, userID, readID, state, fields)
	if err != nil {
		return nil, fmt.Errorf("update read: %w", err)
	}
	if n == 0 {
		if _, err := s.repo.GetRead(ctx, userID, readID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrReadNotFound
			}
			return nil, fmt.Errorf("get read: %w", err)
		}
		return nil, ErrReadChanged
	}
	updated, err := s.repo.GetRead(ctx, userID, readID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReadNotFound
		}
		return nil, fmt.Errorf("get read: %w", err)
	}
	return updated, nil
}

// DeleteRead deletes a finished or abandoned attempt of the caller. The open
// attempt follows the «Читаю» status and cannot be deleted here.
func (s *Service) DeleteRead(ctx context.Context, userID, readID uuid.UUID) error {
	n, err := s.repo.DeleteClosedRead(ctx, userID, readID)
	if err != nil {
		return fmt.Errorf("delete read: %w", err)
	}
	if n > 0 {
		return nil
	}
	if _, err := s.repo.GetRead(ctx, userID, readID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReadNotFound
		}
		return fmt.Errorf("get read: %w", err)
	}
	return ErrOpenReadNotDeletable
}
