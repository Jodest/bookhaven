package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	adminrepository "github.com/bookhaven/backend/internal/modules/admin/repository"
	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/bookhaven/backend/internal/modules/aigenres/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Errors with the exact texts of the API (openspec
// catalog-moderation/ai-genre-suggestions).
var (
	ErrDisabled       = errors.New("AI genre analysis is disabled")
	ErrAlreadyRunning = errors.New("AI genre analysis already running")
	ErrNotFound       = errors.New("suggestion not found")
	ErrNotOpen        = errors.New("suggestion is not open")
	ErrStale          = errors.New("suggestion is stale")
	// ErrGenreNotFound: the chosen merge target (or a genre) was not found.
	ErrGenreNotFound = adminservice.ErrGenreNotFound
	// ErrInvalidFilter: an unknown status or kind in a list request.
	ErrInvalidFilter = errors.New("invalid filter")
)

// InputError is a request the apply rules reject (HTTP 400).
type InputError struct{ Msg string }

func (e *InputError) Error() string { return e.Msg }

// Service is the logic behind the admin endpoints. It never calls the model.
type Service struct {
	repo    Repository
	catalog GenreCatalog
	applier GenreApplier
	active  bool
	model   string
	log     *zap.Logger
	now     func() time.Time
}

// NewService builds a Service. active is config.AIGenresActive(), model the
// configured model id. A nil logger is replaced by a no-op logger.
func NewService(repo Repository, catalog GenreCatalog, applier GenreApplier, active bool, model string, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, catalog: catalog, applier: applier, active: active, model: model, log: log, now: time.Now}
}

// Active reports whether AI genre analysis is switched on.
func (s *Service) Active() bool { return s.active }

// Status is the data of GET /admin/genres/ai-suggestions/status.
type Status struct {
	Enabled       bool
	Model         string
	PromptVersion int
	OpenCount     int64
	LastRun       *domain.Run
}

// Status returns the switch, the open count (stale ones included) and the
// latest run. A queued or running run older than domain.ActiveStaleAfter is
// reported as failed / interrupted.
func (s *Service) Status(ctx context.Context) (*Status, error) {
	open, err := s.repo.CountOpen(ctx)
	if err != nil {
		return nil, err
	}
	last, err := s.repo.LatestRun(ctx)
	if err != nil {
		return nil, err
	}
	if last != nil {
		last = s.projectRun(last)
	}
	return &Status{Enabled: s.active, Model: s.model, PromptVersion: PromptVersion, OpenCount: open, LastRun: last}, nil
}

func (s *Service) projectRun(r *domain.Run) *domain.Run {
	if domain.IsActiveRunStatus(r.Status) && s.now().UTC().Sub(r.CreatedAt) > domain.ActiveStaleAfter {
		cp := *r
		code := domain.ErrorInterrupted
		cp.Status = domain.RunFailed
		cp.ErrorCode = &code
		return &cp
	}
	return r
}

// StartAnalysis creates a queued manual run. Runs that stayed queued or
// running for more than two hours are first marked failed / interrupted;
// ErrAlreadyRunning when a younger one exists. The caller queues the worker
// task and calls MarkRunEnqueueFailed when that fails.
func (s *Service) StartAnalysis(ctx context.Context, userID uuid.UUID) (*domain.Run, error) {
	if !s.active {
		return nil, ErrDisabled
	}
	now := s.now().UTC()
	if _, err := s.repo.InterruptStaleRuns(ctx, now.Add(-domain.ActiveStaleAfter), now); err != nil {
		return nil, err
	}
	active, err := s.repo.ActiveRun(ctx)
	if err != nil {
		return nil, err
	}
	if active != nil {
		return nil, ErrAlreadyRunning
	}
	run := &domain.Run{
		Trigger: domain.TriggerManual, Status: domain.RunQueued, Model: s.model,
		PromptVersion: PromptVersion, CreatedBy: &userID, CreatedAt: now,
	}
	if err := s.repo.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

// MarkRunEnqueueFailed ends a run whose worker task could not be queued.
func (s *Service) MarkRunEnqueueFailed(ctx context.Context, id uuid.UUID) error {
	code := domain.ErrorEnqueueFailed
	return s.repo.FinishRun(ctx, id, domain.RunFailed, &code, s.now().UTC())
}

// ---- reading ----

// GenreView is the current state of one genre of a suggestion. For a genre
// that no longer exists only the stored name is known.
type GenreView struct {
	ID          uuid.UUID
	Name        string
	Status      string
	Merged      bool
	Exists      bool
	WikidataQID *string
	BooksTotal  int64
	SampleBooks []adminrepository.SampleBook
}

// GenreRef is a genre by id and name.
type GenreRef struct {
	ID   uuid.UUID
	Name string
}

// SuggestionView is a suggestion with the current state of its genres.
type SuggestionView struct {
	Suggestion domain.Suggestion
	// Stale: the suggestion is open and one of its genres changed.
	Stale bool
	// Target is nil for junk.
	Target        *GenreRef
	Genres        []GenreView
	AppliedTarget *GenreRef
}

// ListFilter selects suggestions; empty Status means open.
type ListFilter struct {
	Status string
	Kind   string
	Page   int
	Limit  int
}

// List returns one page of suggestions and the total.
func (s *Service) List(ctx context.Context, f ListFilter) ([]SuggestionView, int64, error) {
	if f.Status == "" {
		f.Status = domain.StatusOpen
	}
	if !domain.IsStatus(f.Status) || (f.Kind != "" && !domain.IsKind(f.Kind)) {
		return nil, 0, ErrInvalidFilter
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = domain.ListDefaultLimit
	}
	if f.Limit > domain.ListMaxLimit {
		f.Limit = domain.ListMaxLimit
	}
	items, total, err := s.repo.ListSuggestions(ctx, repository.ListFilter{
		Status: f.Status, Kind: f.Kind, Limit: f.Limit, Offset: (f.Page - 1) * f.Limit,
	})
	if err != nil {
		return nil, 0, err
	}
	views, err := s.views(ctx, items)
	return views, total, err
}

// views loads the current state of every genre of the suggestions with two
// queries (states and sample books).
func (s *Service) views(ctx context.Context, items []domain.Suggestion) ([]SuggestionView, error) {
	views := make([]SuggestionView, 0, len(items))
	if len(items) == 0 {
		return views, nil
	}
	seen := map[uuid.UUID]struct{}{}
	var ids []uuid.UUID
	addID := func(id uuid.UUID) {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	for _, it := range items {
		for _, g := range it.Genres {
			addID(g.GenreID)
		}
		if it.AppliedTargetID != nil {
			addID(*it.AppliedTargetID)
		}
	}
	states, err := s.catalog.GenresByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	samples, err := s.catalog.ListGenreSampleBooks(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		it := items[i]
		v := SuggestionView{Suggestion: it, Genres: make([]GenreView, 0, len(it.Genres))}
		v.Stale = it.Status == domain.StatusOpen && suggestionStale(&it, states)
		names := map[uuid.UUID]string{}
		for _, g := range it.Genres {
			names[g.GenreID] = g.NameSnapshot
			gv := GenreView{ID: g.GenreID, Name: g.NameSnapshot, SampleBooks: []adminrepository.SampleBook{}}
			if st, ok := states[g.GenreID]; ok {
				gv.Name = st.Name
				gv.Status = st.Status
				gv.Merged = st.MergedIntoID != nil
				gv.Exists = true
				gv.WikidataQID = st.WikidataQID
				gv.BooksTotal = st.BooksTotal
				if sb := samples[g.GenreID]; sb != nil {
					gv.SampleBooks = sb
				}
			}
			v.Genres = append(v.Genres, gv)
			if g.Role == domain.RoleTarget {
				v.Target = &GenreRef{ID: g.GenreID, Name: gv.Name}
			}
		}
		if it.AppliedTargetID != nil {
			ref := &GenreRef{ID: *it.AppliedTargetID, Name: names[*it.AppliedTargetID]}
			if st, ok := states[*it.AppliedTargetID]; ok {
				ref.Name = st.Name
			}
			v.AppliedTarget = ref
		}
		views = append(views, v)
	}
	return views, nil
}

// view loads one suggestion as a view.
func (s *Service) view(ctx context.Context, id uuid.UUID) (*SuggestionView, error) {
	sug, err := s.repo.GetSuggestion(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	views, err := s.views(ctx, []domain.Suggestion{*sug})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// ---- apply ----

// ApplyResult is the outcome of applying one suggestion.
type ApplyResult struct {
	View *SuggestionView
	// Merge is set for a merge suggestion, StatusReport for a junk one.
	Merge        *adminservice.GenreBatchMergeOutcome
	StatusReport *adminservice.GenreBulkStatusReport
}

func (s *Service) isStale(ctx context.Context, sug *domain.Suggestion) (bool, error) {
	states, err := s.catalog.GenresByIDs(ctx, sug.GenreIDs())
	if err != nil {
		return false, err
	}
	return suggestionStale(sug, states), nil
}

func (s *Service) markStale(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.MarkStale(ctx, []uuid.UUID{id})
	return err
}

func isMergeInputError(err error) bool {
	return errors.Is(err, adminservice.ErrGenreMergeNoSources) ||
		errors.Is(err, adminservice.ErrGenreMergeDuplicateSource) ||
		errors.Is(err, adminservice.ErrGenreMergeTooLarge) ||
		errors.Is(err, adminservice.ErrCannotMergeGenreIntoSelf)
}

// Apply applies one open suggestion: a merge through the batch genre merge
// (the chosen target is targetOverride when given, otherwise the suggested
// one), a junk suggestion through the genre bulk status (rejected). userID is
// the deciding admin. Errors: ErrNotFound, ErrNotOpen, ErrStale (the
// suggestion is marked stale and nothing changes), *InputError,
// ErrGenreNotFound (the chosen target is missing or merged).
func (s *Service) Apply(ctx context.Context, userID, id uuid.UUID, targetOverride *uuid.UUID) (*ApplyResult, error) {
	sug, err := s.repo.GetSuggestion(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if sug.Kind == domain.KindJunk && targetOverride != nil {
		return nil, &InputError{Msg: "target_id is not allowed for a junk suggestion"}
	}
	if sug.Status != domain.StatusOpen {
		return nil, ErrNotOpen
	}
	stale, err := s.isStale(ctx, sug)
	if err != nil {
		return nil, err
	}
	if stale {
		if err := s.markStale(ctx, sug.ID); err != nil {
			return nil, err
		}
		return nil, ErrStale
	}

	result := &ApplyResult{}
	var appliedTarget *uuid.UUID
	if sug.Kind == domain.KindJunk {
		report, err := s.applier.BulkSetGenreStatus(ctx, sug.GenreIDs(), "rejected")
		if err != nil {
			return nil, fmt.Errorf("reject junk genre: %w", err)
		}
		if len(report.Errors) > 0 || report.Updated != len(sug.Genres) {
			// The genre vanished or became an alias between the check and the
			// update: the suggestion no longer describes the catalog.
			if err := s.markStale(ctx, sug.ID); err != nil {
				return nil, err
			}
			return nil, ErrStale
		}
		result.StatusReport = &report
	} else {
		suggested := sug.Target()
		if suggested == nil {
			return nil, &InputError{Msg: "suggestion has no target"}
		}
		chosen := suggested.GenreID
		if targetOverride != nil {
			chosen = *targetOverride
		}
		sources := make([]uuid.UUID, 0, len(sug.Genres))
		for _, g := range sug.Genres {
			if g.GenreID != chosen {
				sources = append(sources, g.GenreID)
			}
		}
		outcome, err := s.applier.MergeGenresBatch(ctx, sources, chosen)
		if err != nil {
			switch {
			case isMergeInputError(err):
				return nil, &InputError{Msg: err.Error()}
			case errors.Is(err, adminservice.ErrGenreNotFound):
				stale, sErr := s.isStale(ctx, sug)
				if sErr != nil {
					return nil, sErr
				}
				if stale {
					if err := s.markStale(ctx, sug.ID); err != nil {
						return nil, err
					}
					return nil, ErrStale
				}
				return nil, ErrGenreNotFound
			default:
				return nil, fmt.Errorf("merge genres: %w", err)
			}
		}
		result.Merge = outcome
		appliedTarget = &chosen
	}

	changed, err := s.repo.MarkApplied(ctx, sug.ID, userID, appliedTarget, s.now().UTC())
	if err != nil {
		// The merge stands; the suggestion will read as stale (its sources
		// are aliases now).
		s.log.Error("AI genres: mark suggestion applied failed after the change",
			zap.String("suggestion_id", sug.ID.String()), zap.Error(err))
	} else if !changed {
		s.log.Warn("AI genres: suggestion was decided concurrently", zap.String("suggestion_id", sug.ID.String()))
	}
	view, err := s.view(ctx, sug.ID)
	if err != nil {
		return nil, err
	}
	result.View = view
	return result, nil
}

// ApplyItem is one item of a bulk apply.
type ApplyItem struct {
	ID       uuid.UUID
	TargetID *uuid.UUID
}

// ItemError is a per-suggestion failure of a bulk request.
type ItemError struct {
	SuggestionID string
	Message      string
}

// BulkApplyReport is the report of ApplyBulk.
type BulkApplyReport struct {
	Processed int
	Applied   int
	Stale     int
	Errors    []ItemError
}

// ApplyBulk applies the items one after another in request order, each merge
// in its own transaction: a failed item does not undo earlier ones, and an
// item made stale by an earlier one is counted as stale.
func (s *Service) ApplyBulk(ctx context.Context, userID uuid.UUID, items []ApplyItem) *BulkApplyReport {
	report := &BulkApplyReport{Processed: len(items), Errors: []ItemError{}}
	for _, it := range items {
		_, err := s.Apply(ctx, userID, it.ID, it.TargetID)
		switch {
		case err == nil:
			report.Applied++
		case errors.Is(err, ErrStale):
			report.Stale++
		default:
			msg := err.Error()
			var input *InputError
			if !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrNotOpen) && !errors.Is(err, ErrGenreNotFound) && !errors.As(err, &input) {
				s.log.Error("AI genres: bulk apply item failed", zap.String("suggestion_id", it.ID.String()), zap.Error(err))
				msg = "failed to apply suggestion"
			}
			report.Errors = append(report.Errors, ItemError{SuggestionID: it.ID.String(), Message: msg})
		}
	}
	return report
}

// ---- dismiss ----

// Dismiss sets an open suggestion (stale or not) to dismissed; nothing in the
// genres changes. Errors: ErrNotFound, ErrNotOpen.
func (s *Service) Dismiss(ctx context.Context, userID, id uuid.UUID) (*SuggestionView, error) {
	sug, err := s.repo.GetSuggestion(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if sug.Status != domain.StatusOpen {
		return nil, ErrNotOpen
	}
	changed, err := s.repo.MarkDismissed(ctx, id, userID, s.now().UTC())
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, ErrNotOpen
	}
	return s.view(ctx, id)
}

// BulkDismissReport is the report of DismissBulk.
type BulkDismissReport struct {
	Processed int
	Dismissed int
	Errors    []ItemError
}

// DismissBulk dismisses the ids one after another.
func (s *Service) DismissBulk(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) *BulkDismissReport {
	report := &BulkDismissReport{Processed: len(ids), Errors: []ItemError{}}
	for _, id := range ids {
		_, err := s.Dismiss(ctx, userID, id)
		switch {
		case err == nil:
			report.Dismissed++
		case errors.Is(err, ErrNotFound), errors.Is(err, ErrNotOpen):
			report.Errors = append(report.Errors, ItemError{SuggestionID: id.String(), Message: err.Error()})
		default:
			s.log.Error("AI genres: bulk dismiss item failed", zap.String("suggestion_id", id.String()), zap.Error(err))
			report.Errors = append(report.Errors, ItemError{SuggestionID: id.String(), Message: "failed to dismiss suggestion"})
		}
	}
	return report
}
