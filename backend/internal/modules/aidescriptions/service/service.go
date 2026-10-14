package service

import (
	"context"
	"errors"
	"time"

	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	"github.com/bookhaven/backend/internal/modules/aidescriptions/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Repository is the storage the service and the task handlers use
// (satisfied by *repository.GormRepository).
type Repository interface {
	Upsert(ctx context.Context, d domain.Description) error
	Get(ctx context.Context, bookID uuid.UUID) (*domain.Item, error)
	List(ctx context.Context, status string, page, limit int) ([]domain.Item, int64, error)
	CountDrafts(ctx context.Context) (int64, error)
	CountMissing(ctx context.Context) (int64, error)
	ListMissingCandidateIDs(ctx context.Context, limit int) ([]uuid.UUID, error)
	Approve(ctx context.Context, bookID, userID uuid.UUID, text *string, now time.Time) error
	Discard(ctx context.Context, bookID, userID uuid.UUID, now time.Time) error
}

// BookLoader loads books with their authors and genres (satisfied by
// *adminservice.Service).
type BookLoader interface {
	LoadBooksWithRelations(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Book, error)
}

// Errors of Approve and Discard (see repository).
var (
	ErrNotFound          = repository.ErrNotFound
	ErrNotReviewable     = repository.ErrNotReviewable
	ErrHasOwnDescription = repository.ErrHasOwnDescription
)

// ErrInvalidStatus: an unknown list status filter (400).
var ErrInvalidStatus = errors.New("invalid status")

// Status is the feature switch, the model and the review counters.
type Status struct {
	Enabled bool
	Model   string
	// Missing is the number of books an all_missing run would pick (no
	// cap).
	Missing int64
	// Drafts is the number of drafts whose book has no own description.
	Drafts int64
}

// Service reads and reviews AI descriptions (API side; it never calls the
// model).
type Service struct {
	repo   Repository
	books  BookLoader
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

// NewService builds a Service. active is config.AIDescriptionsActive()
// (the switch the worker uses too), model the configured model id.
func NewService(repo Repository, books BookLoader, active bool, model string, opts ...Option) *Service {
	s := &Service{repo: repo, books: books, active: active, model: model, now: time.Now, log: zap.NewNop()}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) clock() time.Time { return s.now().UTC() }

// Active reports whether generation is switched on.
func (s *Service) Active() bool { return s.active }

// Now is the service clock (UTC); the handler stamps queued_at with it.
func (s *Service) Now() time.Time { return s.clock() }

// Status returns the switch, the model and the counters.
func (s *Service) Status(ctx context.Context) (*Status, error) {
	missing, err := s.repo.CountMissing(ctx)
	if err != nil {
		return nil, err
	}
	drafts, err := s.repo.CountDrafts(ctx)
	if err != nil {
		return nil, err
	}
	return &Status{Enabled: s.active, Model: s.model, Missing: missing, Drafts: drafts}, nil
}

// ListResult is one page of the review list (Page and Limit as applied).
type ListResult struct {
	Items []domain.Item
	Total int64
	Page  int
	Limit int
}

// List returns one page of AI descriptions in status (default draft),
// newest first, with authors and the stale flag computed from the loaded
// books. page < 1 becomes 1; limit is clamped to 1..MaxListLimit
// (DefaultListLimit when <= 0). An unknown status → ErrInvalidStatus.
func (s *Service) List(ctx context.Context, status string, page, limit int) (*ListResult, error) {
	if status == "" {
		status = domain.StatusDraft
	}
	if !domain.IsStatus(status) {
		return nil, ErrInvalidStatus
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = domain.DefaultListLimit
	}
	if limit > domain.MaxListLimit {
		limit = domain.MaxListLimit
	}
	items, total, err := s.repo.List(ctx, status, page, limit)
	if err != nil {
		return nil, err
	}
	if err := s.enrich(ctx, items); err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total, Page: page, Limit: limit}, nil
}

// Get returns the book's AI description, or nil when there is none.
func (s *Service) Get(ctx context.Context, bookID uuid.UUID) (*domain.Item, error) {
	item, err := s.repo.Get(ctx, bookID)
	if err != nil || item == nil {
		return nil, err
	}
	items := []domain.Item{*item}
	if err := s.enrich(ctx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

// Approve approves the book's draft or approved AI description. A non-nil
// text is validated first (ValidateEditedText → ErrInvalidText) and
// replaces the stored one. Errors: ErrInvalidText, ErrNotFound,
// ErrNotReviewable, ErrHasOwnDescription. No book field changes.
func (s *Service) Approve(ctx context.Context, bookID, userID uuid.UUID, text *string) (*domain.Item, error) {
	if text != nil {
		clean, err := ValidateEditedText(*text)
		if err != nil {
			return nil, err
		}
		text = &clean
	}
	if err := s.repo.Approve(ctx, bookID, userID, text, s.clock()); err != nil {
		return nil, err
	}
	s.log.Info("AI description approved",
		zap.String("book_id", bookID.String()), zap.String("user_id", userID.String()), zap.Bool("text_given", text != nil))
	return s.Get(ctx, bookID)
}

// Discard discards the book's draft or approved AI description. Errors:
// ErrNotFound, ErrNotReviewable.
func (s *Service) Discard(ctx context.Context, bookID, userID uuid.UUID) (*domain.Item, error) {
	if err := s.repo.Discard(ctx, bookID, userID, s.clock()); err != nil {
		return nil, err
	}
	s.log.Info("AI description discarded",
		zap.String("book_id", bookID.String()), zap.String("user_id", userID.String()))
	return s.Get(ctx, bookID)
}

// enrich fills authors and stale from the books (one loader call).
func (s *Service) enrich(ctx context.Context, items []domain.Item) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.BookID)
	}
	books, err := s.books.LoadBooksWithRelations(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[uuid.UUID]catalogDomain.Book, len(books))
	for _, b := range books {
		byID[b.ID] = b
	}
	for i := range items {
		b, ok := byID[items[i].BookID]
		if !ok {
			continue
		}
		in := InputOf(b)
		items[i].Book.Authors = in.Authors
		items[i].Stale = items[i].PromptVersion != PromptVersion || items[i].InputHash != InputHash(in)
	}
	return nil
}
