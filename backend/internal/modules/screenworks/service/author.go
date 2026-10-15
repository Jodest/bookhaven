package service

import (
	"context"
	"errors"
	"fmt"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Films about authors (openspec change add-author-screen-works).

var (
	// ErrAuthorSyncUnavailable: no Wikidata source configured (503).
	ErrAuthorSyncUnavailable = errors.New("author film sync source is not configured")
	// ErrAuthorSyncTooLarge: more than SyncCap distinct ids (400).
	ErrAuthorSyncTooLarge = fmt.Errorf("too many author ids: at most %d per request", SyncCap)
)

// AuthorSource fetches films about authors from Wikidata. Satisfied by
// *bookdata.WikidataClient.
type AuthorSource interface {
	FetchAuthorFilms(ctx context.Context, authorQIDs []string) (map[string][]bookdata.ExternalAuthorFilm, error)
}

// WithAuthorSource sets the author film source explicitly (nil disables the
// author film sync) and returns the service.
func (s *Service) WithAuthorSource(src AuthorSource) *Service {
	s.authorSource = src
	return s
}

// AuthorSyncReport is the JSON report of POST /admin/authors/sync-screen-works.
type AuthorSyncReport struct {
	Processed int              `json:"processed"`
	Updated   int              `json:"updated"`
	Added     int              `json:"added"`
	Items     []AuthorSyncItem `json:"items"`
}

// AuthorSyncItem is the outcome for one requested author.
type AuthorSyncItem struct {
	AuthorID string   `json:"author_id"`
	Name     string   `json:"name"`
	Outcome  string   `json:"outcome"`
	Found    int      `json:"found"`
	Added    int      `json:"added"`
	Notes    []string `json:"notes"`
	Message  string   `json:"message"`
}

// HasAuthorSyncSource reports whether the author film sync is configured.
func (s *Service) HasAuthorSyncSource() bool { return s.authorSource != nil }

// SyncAuthors is the admin action: 1..SyncCap distinct authors of any status.
// When every Wikidata call failed, the report is returned together with the
// last source error.
func (s *Service) SyncAuthors(ctx context.Context, ids []uuid.UUID) (*AuthorSyncReport, error) {
	if s.authorSource == nil {
		return nil, ErrAuthorSyncUnavailable
	}
	ids = dedupe(ids)
	if len(ids) == 0 {
		return nil, ErrSyncEmpty
	}
	if len(ids) > SyncCap {
		return nil, ErrAuthorSyncTooLarge
	}
	return s.syncAuthors(ctx, ids)
}

// SyncCreatedAuthors syncs the authors a harvest job or an import created (no
// cap, chunks of SyncCap). Best-effort: it returns the number of links
// created and the number of authors whose sync ended in an error.
func (s *Service) SyncCreatedAuthors(ctx context.Context, ids []uuid.UUID) (added, failed int) {
	ids = dedupe(ids)
	if s.authorSource == nil || len(ids) == 0 {
		return 0, 0
	}
	report, err := s.syncAuthors(ctx, ids)
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

func (s *Service) syncAuthors(ctx context.Context, ids []uuid.UUID) (*AuthorSyncReport, error) {
	authors := make(map[uuid.UUID]domain.AuthorRef, len(ids))
	for start := 0; start < len(ids); start += SyncCap {
		loaded, err := s.repo.GetAuthorsForSync(ctx, ids[start:min(start+SyncCap, len(ids))])
		if err != nil {
			return nil, fmt.Errorf("load authors: %w", err)
		}
		for _, a := range loaded {
			authors[a.ID] = a
		}
	}

	items := make([]AuthorSyncItem, len(ids))
	var withQID []int
	for i, id := range ids {
		items[i] = AuthorSyncItem{AuthorID: id.String(), Notes: []string{}}
		author, ok := authors[id]
		if !ok {
			items[i].Outcome = domain.OutcomeError
			items[i].Message = domain.ErrAuthorNotFound.Error()
			continue
		}
		items[i].Name = author.Name
		if !bookdata.IsWikidataQID(author.WikidataQID()) {
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
		if err := s.syncAuthorChunk(ctx, chunk, ids, authors, items); err != nil {
			failed++
			lastErr = err
		}
	}

	report := &AuthorSyncReport{Processed: len(items), Items: items}
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

// syncAuthorChunk makes one Wikidata call for up to SyncCap authors and saves
// each author in its own transaction. It returns the Wikidata error (the
// chunk's items are then marked as errors).
func (s *Service) syncAuthorChunk(ctx context.Context, chunk []int, ids []uuid.UUID, authors map[uuid.UUID]domain.AuthorRef, items []AuthorSyncItem) error {
	qids := make([]string, 0, len(chunk))
	seen := map[string]struct{}{}
	for _, i := range chunk {
		qid := authors[ids[i]].WikidataQID()
		if _, dup := seen[qid]; !dup {
			seen[qid] = struct{}{}
			qids = append(qids, qid)
		}
	}

	found, err := s.authorSource.FetchAuthorFilms(ctx, qids)
	if err != nil {
		s.log.Warn("wikidata author film sync: fetch failed", zap.Strings("qids", qids), zap.Error(err))
		for _, i := range chunk {
			items[i].Outcome = domain.OutcomeError
			items[i].Notes = append(items[i].Notes, domain.NoteWikidataUnavailable)
			items[i].Message = "wikidata unavailable"
		}
		return err
	}

	for _, i := range chunk {
		author := authors[ids[i]]
		films := found[author.WikidataQID()]
		items[i].Found = len(films)
		added, err := s.repo.SyncAuthor(ctx, author.ID, toAuthorInputs(films))
		if err != nil {
			s.log.Warn("wikidata author film sync: save failed", zap.String("author_id", author.ID.String()), zap.Error(err))
			items[i].Outcome = domain.OutcomeError
			items[i].Message = "failed to save author films"
			continue
		}
		items[i].Added = added
		if added > 0 {
			items[i].Outcome = domain.OutcomeAdded
		} else {
			items[i].Outcome = domain.OutcomeUnchanged
		}
		s.log.Info("wikidata author film sync",
			zap.String("author_id", author.ID.String()), zap.Int("found", len(films)), zap.Int("added", added))
	}
	return nil
}

func toAuthorInputs(films []bookdata.ExternalAuthorFilm) []domain.AuthorFilmInput {
	out := make([]domain.AuthorFilmInput, 0, len(films))
	for _, f := range films {
		out = append(out, domain.AuthorFilmInput{
			WorkInput: domain.WorkInput{
				QID:           f.QID,
				Title:         f.Title,
				OriginalTitle: strPtr(f.OriginalTitle),
				Kind:          f.Kind,
				Year:          f.Year,
				Director:      strPtr(f.Director),
				IMDbID:        strPtr(f.IMDbID),
				KinopoiskID:   strPtr(f.KinopoiskID),
			},
			Relation: f.Relation,
		})
	}
	return out
}

// ListPublicByAuthor returns the visible films of a published author;
// domain.ErrAuthorNotFound for an unknown or not published author.
func (s *Service) ListPublicByAuthor(ctx context.Context, authorID uuid.UUID) ([]domain.LinkedWork, error) {
	status, err := s.repo.AuthorStatus(ctx, authorID)
	if err != nil {
		return nil, err
	}
	if status != catalogDomain.AuthorStatusPublished {
		return nil, domain.ErrAuthorNotFound
	}
	return s.repo.ListPublicByAuthor(ctx, authorID)
}

// ListAdminByAuthor returns every film link of an author of any status.
func (s *Service) ListAdminByAuthor(ctx context.Context, authorID uuid.UUID) ([]domain.LinkedWork, error) {
	if _, err := s.repo.AuthorStatus(ctx, authorID); err != nil {
		return nil, err
	}
	return s.repo.ListAdminByAuthor(ctx, authorID)
}

// SetAuthorFilmHidden hides or shows a link and returns the updated item. An
// unknown author or link gives domain.ErrAuthorLinkNotFound.
func (s *Service) SetAuthorFilmHidden(ctx context.Context, authorID, workID uuid.UUID, hidden bool) (*domain.LinkedWork, error) {
	if _, err := s.repo.AuthorStatus(ctx, authorID); err != nil {
		if errors.Is(err, domain.ErrAuthorNotFound) {
			return nil, domain.ErrAuthorLinkNotFound
		}
		return nil, err
	}
	if err := s.repo.SetAuthorLinkHidden(ctx, authorID, workID, hidden); err != nil {
		return nil, err
	}
	links, err := s.repo.ListAdminByAuthor(ctx, authorID)
	if err != nil {
		return nil, err
	}
	for i := range links {
		if links[i].ID == workID {
			return &links[i], nil
		}
	}
	return nil, domain.ErrAuthorLinkNotFound
}
