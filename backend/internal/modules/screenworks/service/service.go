// Package service holds the screen works business logic: Wikidata adaptation
// sync, public and admin lists, hide/show of a link.
package service

import (
	"context"
	"errors"
	"fmt"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/bookhaven/backend/internal/modules/screenworks/repository"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SyncCap bounds one admin sync request and is the per-call QID chunk.
const SyncCap = bookdata.MaxAdaptationBooks

var (
	// ErrSyncUnavailable: no Wikidata source configured (503).
	ErrSyncUnavailable = errors.New("adaptation sync source is not configured")
	// ErrSyncEmpty: no ids (400).
	ErrSyncEmpty = errors.New("ids must not be empty")
	// ErrSyncTooLarge: more than SyncCap distinct ids (400).
	ErrSyncTooLarge = fmt.Errorf("too many book ids: at most %d per request", SyncCap)
)

// Source fetches adaptations from Wikidata. Satisfied by
// *bookdata.WikidataClient.
type Source interface {
	FetchAdaptations(ctx context.Context, bookQIDs []string) (map[string][]bookdata.ExternalScreenWork, error)
}

// SyncReport is the JSON report of POST /admin/books/sync-adaptations.
type SyncReport struct {
	Processed int        `json:"processed"`
	Updated   int        `json:"updated"`
	Added     int        `json:"added"`
	Items     []SyncItem `json:"items"`
}

// SyncItem is the outcome for one requested book.
type SyncItem struct {
	BookID  string   `json:"book_id"`
	Title   string   `json:"title"`
	Outcome string   `json:"outcome"`
	Found   int      `json:"found"`
	Added   int      `json:"added"`
	Notes   []string `json:"notes"`
	Message string   `json:"message"`
}

// Service provides the screen works logic.
type Service struct {
	repo   repository.Repository
	source Source
	log    *zap.Logger
}

// NewService creates a Service. source may be nil: the sync is then
// unavailable (the lists and hide/show still work). log may be nil.
func NewService(repo repository.Repository, source Source, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, source: source, log: log}
}

// HasSyncSource reports whether the sync is configured.
func (s *Service) HasSyncSource() bool { return s.source != nil }

// SyncBooks is the admin action: 1..SyncCap distinct books of any status.
// When every Wikidata call failed, the report is returned together with the
// last source error.
func (s *Service) SyncBooks(ctx context.Context, ids []uuid.UUID) (*SyncReport, error) {
	if s.source == nil {
		return nil, ErrSyncUnavailable
	}
	ids = dedupe(ids)
	if len(ids) == 0 {
		return nil, ErrSyncEmpty
	}
	if len(ids) > SyncCap {
		return nil, ErrSyncTooLarge
	}
	return s.syncBooks(ctx, ids)
}

// SyncCreatedBooks syncs the books a harvest job created (no cap, chunks of
// SyncCap). Best-effort: it returns the number of links created and the
// number of books whose sync ended in an error.
func (s *Service) SyncCreatedBooks(ctx context.Context, ids []uuid.UUID) (added, failed int) {
	ids = dedupe(ids)
	if s.source == nil || len(ids) == 0 {
		return 0, 0
	}
	report, err := s.syncBooks(ctx, ids)
	if report == nil {
		if err != nil {
			return 0, len(ids)
		}
		return 0, 0
	}
	for _, item := range report.Items {
		if item.Outcome == domain.OutcomeError {
			failed++
		}
	}
	return report.Added, failed
}

func (s *Service) syncBooks(ctx context.Context, ids []uuid.UUID) (*SyncReport, error) {
	books := make(map[uuid.UUID]domain.BookRef, len(ids))
	for start := 0; start < len(ids); start += SyncCap {
		loaded, err := s.repo.GetBooksForSync(ctx, ids[start:min(start+SyncCap, len(ids))])
		if err != nil {
			return nil, fmt.Errorf("load books: %w", err)
		}
		for _, b := range loaded {
			books[b.ID] = b
		}
	}

	items := make([]SyncItem, len(ids))
	var withQID []int
	for i, id := range ids {
		items[i] = SyncItem{BookID: id.String(), Notes: []string{}}
		book, ok := books[id]
		if !ok {
			items[i].Outcome = domain.OutcomeError
			items[i].Message = domain.ErrBookNotFound.Error()
			continue
		}
		items[i].Title = book.Title
		if !bookdata.IsWikidataQID(book.WikidataQID()) {
			items[i].Outcome = domain.OutcomeSkipped
			items[i].Notes = append(items[i].Notes, domain.NoteNoQID)
			continue
		}
		withQID = append(withQID, i)
	}

	attempted, failed := 0, 0
	var lastErr error
	for start := 0; start < len(withQID); start += SyncCap {
		chunk := withQID[start:min(start+SyncCap, len(withQID))]
		attempted++
		if err := s.syncChunk(ctx, chunk, ids, books, items); err != nil {
			failed++
			lastErr = err
		}
	}

	report := &SyncReport{Processed: len(items), Items: items}
	for _, item := range items {
		report.Added += item.Added
		if item.Outcome == domain.OutcomeAdded {
			report.Updated++
		}
	}
	if attempted > 0 && failed == attempted {
		return report, lastErr
	}
	return report, nil
}

// syncChunk makes one Wikidata call for up to SyncCap books and saves each
// book in its own transaction. It returns the Wikidata error (the chunk's
// items are then marked as errors).
func (s *Service) syncChunk(ctx context.Context, chunk []int, ids []uuid.UUID, books map[uuid.UUID]domain.BookRef, items []SyncItem) error {
	qids := make([]string, 0, len(chunk))
	seen := map[string]struct{}{}
	for _, i := range chunk {
		qid := books[ids[i]].WikidataQID()
		if _, dup := seen[qid]; !dup {
			seen[qid] = struct{}{}
			qids = append(qids, qid)
		}
	}

	found, err := s.source.FetchAdaptations(ctx, qids)
	if err != nil {
		s.log.Warn("wikidata adaptation sync: fetch failed", zap.Strings("qids", qids), zap.Error(err))
		for _, i := range chunk {
			items[i].Outcome = domain.OutcomeError
			items[i].Notes = append(items[i].Notes, domain.NoteWikidataUnavailable)
			items[i].Message = "wikidata unavailable"
		}
		return err
	}

	for _, i := range chunk {
		book := books[ids[i]]
		works := found[book.WikidataQID()]
		items[i].Found = len(works)
		added, err := s.repo.SyncBook(ctx, book.ID, toInputs(works))
		if err != nil {
			s.log.Warn("wikidata adaptation sync: save failed", zap.String("book_id", book.ID.String()), zap.Error(err))
			items[i].Outcome = domain.OutcomeError
			items[i].Message = "failed to save adaptations"
			continue
		}
		items[i].Added = added
		if added > 0 {
			items[i].Outcome = domain.OutcomeAdded
		} else {
			items[i].Outcome = domain.OutcomeUnchanged
		}
		s.log.Info("wikidata adaptation sync",
			zap.String("book_id", book.ID.String()), zap.Int("found", len(works)), zap.Int("added", added))
	}
	return nil
}

func toInputs(works []bookdata.ExternalScreenWork) []domain.WorkInput {
	out := make([]domain.WorkInput, 0, len(works))
	for _, w := range works {
		out = append(out, domain.WorkInput{
			QID:           w.QID,
			Title:         w.Title,
			OriginalTitle: strPtr(w.OriginalTitle),
			Kind:          w.Kind,
			Year:          w.Year,
			Director:      strPtr(w.Director),
			IMDbID:        strPtr(w.IMDbID),
			KinopoiskID:   strPtr(w.KinopoiskID),
		})
	}
	return out
}

func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func dedupe(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// ListPublic returns the visible adaptations of a published book;
// domain.ErrBookNotFound for an unknown or not published book.
func (s *Service) ListPublic(ctx context.Context, bookID uuid.UUID) ([]domain.LinkedWork, error) {
	status, err := s.repo.BookStatus(ctx, bookID)
	if err != nil {
		return nil, err
	}
	if status != catalogDomain.BookStatusPublished {
		return nil, domain.ErrBookNotFound
	}
	return s.repo.ListPublicByBook(ctx, bookID)
}

// ListAdmin returns every adaptation link of a book of any status.
func (s *Service) ListAdmin(ctx context.Context, bookID uuid.UUID) ([]domain.LinkedWork, error) {
	if _, err := s.repo.BookStatus(ctx, bookID); err != nil {
		return nil, err
	}
	return s.repo.ListAdminByBook(ctx, bookID)
}

// SetHidden hides or shows a link and returns the updated item. An unknown
// book or link gives domain.ErrLinkNotFound.
func (s *Service) SetHidden(ctx context.Context, bookID, workID uuid.UUID, hidden bool) (*domain.LinkedWork, error) {
	if _, err := s.repo.BookStatus(ctx, bookID); err != nil {
		if errors.Is(err, domain.ErrBookNotFound) {
			return nil, domain.ErrLinkNotFound
		}
		return nil, err
	}
	if err := s.repo.SetLinkHidden(ctx, bookID, workID, hidden); err != nil {
		return nil, err
	}
	links, err := s.repo.ListAdminByBook(ctx, bookID)
	if err != nil {
		return nil, err
	}
	for i := range links {
		if links[i].ID == workID {
			return &links[i], nil
		}
	}
	return nil, domain.ErrLinkNotFound
}
