package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bookhaven/backend/internal/modules/aicollections/domain"
	"github.com/bookhaven/backend/internal/modules/aicollections/repository"
	curateddomain "github.com/bookhaven/backend/internal/modules/curated/domain"
	curatedservice "github.com/bookhaven/backend/internal/modules/curated/service"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Repository is the storage the service and the task handler use (satisfied
// by *repository.GormRepository).
type Repository interface {
	CreateDraft(ctx context.Context, d *domain.Draft) error
	GetDraft(ctx context.Context, id uuid.UUID) (*domain.Draft, error)
	ListDraftsByCollection(ctx context.Context, collectionID uuid.UUID, limit int) ([]domain.Draft, error)
	FindActiveDraft(ctx context.Context, collectionID uuid.UUID, staleBefore time.Time) (*domain.Draft, error)
	CountDraftsSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error)
	MarkRunning(ctx context.Context, id uuid.UUID, now time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, code string, now time.Time) error
	MarkDone(ctx context.Context, id uuid.UUID, errorCode *string, now time.Time) error
	ListProposals(ctx context.Context, draftID uuid.UUID) ([]domain.Proposal, error)
	AppendProposals(ctx context.Context, draftID uuid.UUID, startRank int, ps []domain.NewProposal, model string, inputTokens, outputTokens int, now time.Time) error
	PurgeOlderThan(ctx context.Context, t time.Time) (int64, error)
}

// CollectionLookup loads a manual collection (satisfied by
// *curatedservice.Service). It returns curated ErrCollectionNotFound for an
// unknown collection and ErrCollectionAutomatic for an automatic one.
type CollectionLookup interface {
	ManualInfo(ctx context.Context, id uuid.UUID) (*curatedservice.ManualInfo, error)
}

// ProposalMatcher matches proposals against the catalog and the current
// collection contents (satisfied by *curatedservice.Service). It never
// writes.
type ProposalMatcher interface {
	MatchProposals(ctx context.Context, collectionID uuid.UUID, ps []curatedservice.Proposal) (*curatedservice.PastePreview, error)
}

// Errors of CreateDraft, in check order.
var (
	// ErrDisabled: AI collection drafts are switched off (503).
	ErrDisabled = errors.New("AI collection drafts are disabled")
	// ErrInvalidCollectionID: collection_id is missing or not a UUID (400).
	ErrInvalidCollectionID = errors.New("collection_id: must be a valid UUID")
	// ErrInvalidBrief: the brief is empty or too long after trim (400).
	ErrInvalidBrief = errors.New("brief: must be 1-500 characters")
	// ErrInvalidCount: count is outside 1..1000 (400).
	ErrInvalidCount = errors.New("count: must be between 1 and 1000")
	// ErrCollectionNotFound: no such collection (404).
	ErrCollectionNotFound = errors.New("collection not found")
	// ErrCollectionAutomatic: the collection is automatic (422).
	ErrCollectionAutomatic = errors.New("collection is automatic")
	// ErrDailyLimit: the admin created DailyLimit drafts in 24 h (429).
	ErrDailyLimit = errors.New("daily AI draft limit reached")
	// ErrNotFound: no draft with that id (404).
	ErrNotFound = repository.ErrNotFound
)

// ActiveDraftError: the collection already has an active draft (409).
type ActiveDraftError struct {
	DraftID uuid.UUID
}

func (e *ActiveDraftError) Error() string { return "AI draft already in progress" }

// Status is the caller view of the feature switch and limits.
type Status struct {
	Enabled   bool
	Model     string
	UsedToday int64
}

// Report is the match report of a done draft.
type Report struct {
	Preview *curatedservice.PastePreview
	// Proposals are the stored proposals by rank (original title and note
	// for the report lines).
	Proposals map[int]domain.Proposal
}

// Service creates and reads drafts (API side; it never calls the model).
type Service struct {
	repo    Repository
	lookup  CollectionLookup
	matcher ProposalMatcher
	active  bool
	model   string
	now     func() time.Time
	log     *zap.Logger
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

// NewService builds a Service. active is config.AICollectionsActive() (the
// switch the worker uses too), model the configured model id.
func NewService(repo Repository, lookup CollectionLookup, matcher ProposalMatcher, active bool, model string, opts ...Option) *Service {
	s := &Service{repo: repo, lookup: lookup, matcher: matcher, active: active, model: model, now: time.Now, log: zap.NewNop()}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) clock() time.Time { return s.now().UTC() }

// Active reports whether the feature is on.
func (s *Service) Active() bool { return s.active }

// Status returns the switch, the model and the caller usage.
func (s *Service) Status(ctx context.Context, userID uuid.UUID) (*Status, error) {
	used, err := s.repo.CountDraftsSince(ctx, userID, s.clock().Add(-24*time.Hour))
	if err != nil {
		return nil, err
	}
	return &Status{Enabled: s.active, Model: s.model, UsedToday: used}, nil
}

// CreateDraft validates and stores a queued draft, in this order: feature
// off → ErrDisabled; collection_id → ErrInvalidCollectionID; brief empty or
// over MaxBriefRunes after trim → ErrInvalidBrief; count outside 1..MaxCount
// → ErrInvalidCount (nil means the default count); unknown collection →
// ErrCollectionNotFound; automatic → ErrCollectionAutomatic; an active draft
// of the collection → *ActiveDraftError; DailyLimit drafts in the last 24 h →
// ErrDailyLimit. The caller enqueues the task (and calls MarkEnqueueFailed
// when that fails). Nothing about the collection changes.
func (s *Service) CreateDraft(ctx context.Context, userID uuid.UUID, collectionID, brief string, count *int) (*domain.Draft, error) {
	if !s.active {
		return nil, ErrDisabled
	}
	colID, err := uuid.Parse(strings.TrimSpace(collectionID))
	if err != nil {
		return nil, ErrInvalidCollectionID
	}
	brief = strings.TrimSpace(brief)
	if brief == "" || utf8.RuneCountInString(brief) > domain.MaxBriefRunes {
		return nil, ErrInvalidBrief
	}
	if count != nil && (*count < 1 || *count > domain.MaxCount) {
		return nil, ErrInvalidCount
	}

	info, err := s.lookup.ManualInfo(ctx, colID)
	if err != nil {
		switch {
		case errors.Is(err, curateddomain.ErrCollectionNotFound):
			return nil, ErrCollectionNotFound
		case errors.Is(err, curateddomain.ErrCollectionAutomatic):
			return nil, ErrCollectionAutomatic
		}
		return nil, err
	}
	n := domain.DefaultCount(info.Size)
	if count != nil {
		n = *count
	}

	now := s.clock()
	active, err := s.repo.FindActiveDraft(ctx, colID, now.Add(-domain.ActiveStaleAfter))
	if err != nil {
		return nil, err
	}
	if active != nil {
		return nil, &ActiveDraftError{DraftID: active.ID}
	}
	used, err := s.repo.CountDraftsSince(ctx, userID, now.Add(-24*time.Hour))
	if err != nil {
		return nil, err
	}
	if used >= domain.DailyLimit {
		return nil, ErrDailyLimit
	}

	uid := userID
	d := &domain.Draft{
		CollectionID:  colID,
		Brief:         brief,
		Count:         n,
		Status:        domain.StatusQueued,
		PromptVersion: PromptVersion,
		CreatedBy:     &uid,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreateDraft(ctx, d); err != nil {
		return nil, err
	}
	s.log.Info("AI collection draft created",
		zap.String("draft_id", d.ID.String()),
		zap.String("collection_id", colID.String()),
		zap.String("user_id", userID.String()),
		zap.Int("count", n))
	// Re-read to get the creator; fall back to the stored values.
	if stored, err := s.repo.GetDraft(ctx, d.ID); err == nil {
		return s.project(stored, now), nil
	}
	return d, nil
}

// MarkEnqueueFailed ends a draft whose task could not be queued.
func (s *Service) MarkEnqueueFailed(ctx context.Context, id uuid.UUID) error {
	return s.repo.MarkFailed(ctx, id, domain.ErrorEnqueueFailed, s.clock())
}

// ListDrafts returns the newest ListLimit drafts of the collection, with the
// timeout projection applied.
func (s *Service) ListDrafts(ctx context.Context, collectionID uuid.UUID) ([]domain.Draft, error) {
	items, err := s.repo.ListDraftsByCollection(ctx, collectionID, domain.ListLimit)
	if err != nil {
		return nil, err
	}
	now := s.clock()
	for i := range items {
		s.project(&items[i], now)
	}
	return items, nil
}

// GetDraft returns a draft and, only when it is done, its match report. The
// report is computed now from the current catalog and collection contents.
// ErrNotFound for an unknown draft (or one whose collection is gone).
func (s *Service) GetDraft(ctx context.Context, id uuid.UUID) (*domain.Draft, *Report, error) {
	d, err := s.repo.GetDraft(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	s.project(d, s.clock())
	if d.Status != domain.StatusDone {
		return d, nil, nil
	}
	proposals, err := s.repo.ListProposals(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	byRank := make(map[int]domain.Proposal, len(proposals))
	in := make([]curatedservice.Proposal, 0, len(proposals))
	for _, p := range proposals {
		byRank[p.Rank] = p
		in = append(in, curatedservice.Proposal{Rank: p.Rank, Title: p.Title, Author: p.Author, OriginalTitle: p.OriginalTitle})
	}
	preview, err := s.matcher.MatchProposals(ctx, d.CollectionID, in)
	if err != nil {
		if errors.Is(err, curateddomain.ErrCollectionNotFound) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	return d, &Report{Preview: preview, Proposals: byRank}, nil
}

// project reports a stale active draft as failed/timeout (read-side only;
// nothing is written).
func (s *Service) project(d *domain.Draft, now time.Time) *domain.Draft {
	if d.IsStale(now.Add(-domain.ActiveStaleAfter)) {
		d.Status = domain.StatusFailed
		code := domain.ErrorTimeout
		d.ErrorCode = &code
	}
	return d
}
