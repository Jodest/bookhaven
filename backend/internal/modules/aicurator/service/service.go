package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	"github.com/bookhaven/backend/internal/modules/aicurator/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Repository is the storage the service and the task handler use
// (satisfied by *repository.GormRepository).
type Repository interface {
	CreateRequest(ctx context.Context, req *domain.Request) error
	GetRequest(ctx context.Context, id uuid.UUID) (*domain.Request, error)
	ListRecentRequests(ctx context.Context, limit int) ([]domain.Request, error)
	FindActiveRequest(ctx context.Context, userID uuid.UUID, staleBefore time.Time) (*domain.Request, error)
	CountRequestsSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error)
	MarkRunning(ctx context.Context, id uuid.UUID, now time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, code string, now time.Time) error
	MarkDone(ctx context.Context, id uuid.UUID, now time.Time) error
	SaveModelResult(ctx context.Context, id uuid.UUID, model string, inputTokens, outputTokens int, cands []domain.NewCandidate, now time.Time) error
	ListPendingCandidates(ctx context.Context, requestID uuid.UUID) ([]domain.Candidate, error)
	SaveCandidateResolution(ctx context.Context, requestID, candidateID uuid.UUID, status string, options []domain.Option, now time.Time) error
	PurgeOlderThan(ctx context.Context, t time.Time) (int64, error)
}

// ExistingAuthorFinder looks up catalog authors by Wikidata QID (satisfied
// by *adminservice.Service).
type ExistingAuthorFinder interface {
	FindAuthorsByWikidataQIDs(ctx context.Context, qids []string) (map[string]catalogDomain.Author, error)
}

// Errors of CreateRequest, in check order.
var (
	// ErrDisabled: AI curation is switched off (503).
	ErrDisabled = errors.New("AI curation is disabled")
	// ErrInvalidPrompt: the prompt is empty or too long after trim (400).
	ErrInvalidPrompt = errors.New("prompt must be 1-500 characters")
	// ErrInvalidMaxNames: max_names is outside 1..30 (400).
	ErrInvalidMaxNames = errors.New("max_names must be between 1 and 30")
	// ErrDailyLimit: the admin created DailyLimit requests in 24 h (429).
	ErrDailyLimit = errors.New("daily curation limit reached")
	// ErrNotFound: no request with that id (404).
	ErrNotFound = repository.ErrNotFound
)

// ActiveRequestError: the admin already has an active request (409).
type ActiveRequestError struct {
	RequestID uuid.UUID
}

func (e *ActiveRequestError) Error() string { return "curation request already in progress" }

// Status is the caller's view of the feature switch and limits.
type Status struct {
	Enabled         bool
	Model           string
	UsedToday       int64
	ActiveRequestID *uuid.UUID
}

// Service creates and reads curation requests (API side; it never calls
// the model).
type Service struct {
	repo   Repository
	finder ExistingAuthorFinder
	active bool
	model  string
	now    func() time.Time
	log    *zap.Logger
}

// Option configures a Service.
type Option func(*Service)

// WithClock overrides the clock (tests).
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// WithLogger sets the logger.
func WithLogger(log *zap.Logger) Option {
	return func(s *Service) {
		if log != nil {
			s.log = log
		}
	}
}

// NewService builds a Service. active is config.AICuratorActive() (the
// switch the worker uses too), model the configured model id.
func NewService(repo Repository, finder ExistingAuthorFinder, active bool, model string, opts ...Option) *Service {
	s := &Service{repo: repo, finder: finder, active: active, model: model, now: time.Now, log: zap.NewNop()}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) clock() time.Time { return s.now().UTC() }

// Active reports whether the feature is on.
func (s *Service) Active() bool { return s.active }

// Status returns the switch, the model and the caller's usage.
func (s *Service) Status(ctx context.Context, userID uuid.UUID) (*Status, error) {
	now := s.clock()
	used, err := s.repo.CountRequestsSince(ctx, userID, now.Add(-24*time.Hour))
	if err != nil {
		return nil, err
	}
	active, err := s.repo.FindActiveRequest(ctx, userID, now.Add(-domain.ActiveStaleAfter))
	if err != nil {
		return nil, err
	}
	st := &Status{Enabled: s.active, Model: s.model, UsedToday: used}
	if active != nil {
		id := active.ID
		st.ActiveRequestID = &id
	}
	return st, nil
}

// CreateRequest validates and stores a queued request, in this order:
// feature off → ErrDisabled; prompt empty or over MaxPromptRunes after
// trim → ErrInvalidPrompt; max_names outside 1..MaxNames →
// ErrInvalidMaxNames (nil means DefaultNames); an active request →
// *ActiveRequestError; DailyLimit requests in the last 24 h →
// ErrDailyLimit. The caller enqueues the task (and calls MarkEnqueueFailed
// when that fails).
func (s *Service) CreateRequest(ctx context.Context, userID uuid.UUID, prompt string, maxNames *int) (*domain.Request, error) {
	if !s.active {
		return nil, ErrDisabled
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || utf8.RuneCountInString(prompt) > domain.MaxPromptRunes {
		return nil, ErrInvalidPrompt
	}
	n := domain.DefaultNames
	if maxNames != nil {
		n = *maxNames
	}
	if n < 1 || n > domain.MaxNames {
		return nil, ErrInvalidMaxNames
	}

	now := s.clock()
	active, err := s.repo.FindActiveRequest(ctx, userID, now.Add(-domain.ActiveStaleAfter))
	if err != nil {
		return nil, err
	}
	if active != nil {
		return nil, &ActiveRequestError{RequestID: active.ID}
	}
	used, err := s.repo.CountRequestsSince(ctx, userID, now.Add(-24*time.Hour))
	if err != nil {
		return nil, err
	}
	if used >= domain.DailyLimit {
		return nil, ErrDailyLimit
	}

	uid := userID
	req := &domain.Request{
		Prompt:        prompt,
		MaxNames:      n,
		Status:        domain.StatusQueued,
		PromptVersion: PromptVersion,
		CreatedBy:     &uid,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreateRequest(ctx, req); err != nil {
		return nil, err
	}
	s.log.Info("AI curation request created",
		zap.String("request_id", req.ID.String()),
		zap.String("user_id", userID.String()),
		zap.Int("max_names", n))
	// Re-read to get the creator; fall back to the stored values.
	if stored, err := s.repo.GetRequest(ctx, req.ID); err == nil {
		return s.project(stored, now), nil
	}
	return req, nil
}

// MarkEnqueueFailed ends a request whose task could not be queued.
func (s *Service) MarkEnqueueFailed(ctx context.Context, id uuid.UUID) error {
	return s.repo.MarkFailed(ctx, id, domain.ErrorEnqueueFailed, s.clock())
}

// ListRecent returns the newest ListLimit requests of all admins, with the
// timeout projection applied.
func (s *Service) ListRecent(ctx context.Context) ([]domain.Request, error) {
	items, err := s.repo.ListRecentRequests(ctx, domain.ListLimit)
	if err != nil {
		return nil, err
	}
	now := s.clock()
	for i := range items {
		s.project(&items[i], now)
	}
	return items, nil
}

// GetRequest returns a request with its candidates and the catalog
// authors currently carrying any option's QID (one finder call). A finder
// failure is logged and yields a nil map (every existing marker null).
func (s *Service) GetRequest(ctx context.Context, id uuid.UUID) (*domain.Request, map[string]catalogDomain.Author, error) {
	req, err := s.repo.GetRequest(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	s.project(req, s.clock())

	var qids []string
	seen := map[string]struct{}{}
	for _, c := range req.Candidates {
		for _, o := range c.Options {
			if _, dup := seen[o.QID]; dup {
				continue
			}
			seen[o.QID] = struct{}{}
			qids = append(qids, o.QID)
		}
	}
	var existing map[string]catalogDomain.Author
	if len(qids) > 0 && s.finder != nil {
		found, err := s.finder.FindAuthorsByWikidataQIDs(ctx, qids)
		if err != nil {
			s.log.Error("AI curation: existing author lookup failed",
				zap.String("request_id", id.String()), zap.Error(err))
		} else {
			existing = found
		}
	}
	return req, existing, nil
}

// project reports a stale active request as failed/timeout (read-side
// only; nothing is written).
func (s *Service) project(req *domain.Request, now time.Time) *domain.Request {
	if req.IsStale(now.Add(-domain.ActiveStaleAfter)) {
		req.Status = domain.StatusFailed
		code := domain.ErrorTimeout
		req.ErrorCode = &code
	}
	return req
}
