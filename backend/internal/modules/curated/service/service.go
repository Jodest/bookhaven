package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/curated/domain"
	"github.com/bookhaven/backend/internal/modules/curated/repository"
	"github.com/bookhaven/backend/internal/pkg/slug"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	DefaultLimit = 50
	MaxLimit     = 100
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidationError is a 400-level input problem; its message names the field.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, a ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, a...)}
}

// Summary is a public collection with its read-time counters.
type Summary struct {
	Collection domain.Collection
	ShownCount int
	CoverURLs  []string
}

// PublicItem is one ranked place.
type PublicItem struct {
	Place int
	// Movement is nil for manual collections and without a baseline.
	Movement *domain.MovementInfo
	Book     catalogDomain.Book
}

// PublicPage is a page of a published collection.
type PublicPage struct {
	Summary Summary
	Items   []PublicItem
	Total   int
	Page    int
	Limit   int
}

// AdminListItem is a row of the admin collection table.
type AdminListItem struct {
	Collection domain.Collection
	BooksCount int
	ShownCount int
}

// AdminDetailItem is an item of the admin detail with its computed standing.
type AdminDetailItem struct {
	Position int
	// Place is 0 unless State is ranked.
	Place int
	State string
	// Score and Movement are set for automatic collections.
	Score    *float64
	Movement *domain.MovementInfo
	Book     catalogDomain.Book
}

// AdminDetail is the admin view of a collection with every book.
type AdminDetail struct {
	Collection domain.Collection
	BooksCount int
	ShownCount int
	Items      []AdminDetailItem
}

// CreateInput carries the fields of a new collection.
type CreateInput struct {
	Title       string
	Kind        string // "" means manual
	Rule        *domain.Rule
	Size        int
	Slug        string
	Description string
	Source      string
	SourceURL   string
	GenreID     *uuid.UUID
	IsPublished bool
}

// UpdateInput carries a partial update; nil means "leave as is". GenreIDSet
// with a nil GenreID clears the genre; empty Source/SourceURL clear them.
type UpdateInput struct {
	Kind        *string // rejected: the kind never changes
	Rule        *domain.Rule
	Title       *string
	Size        *int
	Slug        *string
	Description *string
	Source      *string
	SourceURL   *string
	GenreIDSet  bool
	GenreID     *uuid.UUID
	IsPublished *bool
}

// Service provides curated collection logic.
type Service struct {
	repo repository.Repository
	now  func() time.Time
	log  *zap.Logger
}

// NewService creates a Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo, now: time.Now, log: zap.NewNop()}
}

// WithLogger sets the logger used for non-fatal recompute failures.
func (s *Service) WithLogger(l *zap.Logger) *Service {
	if l != nil {
		s.log = l
	}
	return s
}

func (s *Service) validateTitle(t string) (string, error) {
	t = strings.TrimSpace(t)
	if n := utf8.RuneCountInString(t); n < 2 || n > 255 {
		return "", invalid("title: must be 2 to 255 characters")
	}
	return t, nil
}

func validateSize(n int) error {
	if n < 1 || n > domain.MaxSize {
		return invalid("size: must be between 1 and %d", domain.MaxSize)
	}
	return nil
}

func validateDescription(d string) (string, error) {
	d = strings.TrimSpace(d)
	if utf8.RuneCountInString(d) > 5000 {
		return "", invalid("description: must be at most 5000 characters")
	}
	return d, nil
}

func validateSource(v string) (*string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(v) > 255 {
		return nil, invalid("source: must be at most 255 characters")
	}
	return &v, nil
}

func validateSourceURL(v string) (*string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(v) > 500 {
		return nil, invalid("source_url: must be at most 500 characters")
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, invalid("source_url: must be an http(s) URL")
	}
	return &v, nil
}

func resolveSlug(raw, title string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = slug.Make(title)
		if len(raw) > 100 {
			raw = strings.Trim(raw[:100], "-")
		}
	}
	if raw == "" {
		return "", invalid("slug is required")
	}
	if len(raw) > 100 || !slugPattern.MatchString(raw) {
		return "", invalid("slug: must be lowercase letters, digits and single hyphens, at most 100 characters")
	}
	return raw, nil
}

func (s *Service) checkGenre(ctx context.Context, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	g, err := s.repo.GetGenre(ctx, *id)
	if err != nil {
		return fmt.Errorf("get genre: %w", err)
	}
	if g == nil || g.Status == catalogDomain.GenreStatusRejected || g.IsAlias() {
		return domain.ErrInvalidGenre
	}
	return nil
}

// normalizeRule validates a rule and maps a rule error to a validation error.
func normalizeRule(r domain.Rule) (domain.Rule, error) {
	n, err := r.Normalize()
	if err != nil {
		return domain.Rule{}, invalid("%s", err.Error())
	}
	return n, nil
}

// Create makes a manual or automatic collection. An automatic one gets its
// first snapshot at once; a failed computation is logged, not returned.
func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.Collection, error) {
	kind := in.Kind
	if kind == "" {
		kind = domain.KindManual
	}
	if kind != domain.KindManual && kind != domain.KindAuto {
		return nil, invalid("kind: must be manual or auto")
	}
	var rule domain.Rule
	switch {
	case kind == domain.KindAuto && in.Rule == nil:
		return nil, invalid("rule: required for automatic collections")
	case kind == domain.KindManual && in.Rule != nil:
		return nil, invalid("rule: only for automatic collections")
	case kind == domain.KindAuto:
		var err error
		if rule, err = normalizeRule(*in.Rule); err != nil {
			return nil, err
		}
	}
	title, err := s.validateTitle(in.Title)
	if err != nil {
		return nil, err
	}
	if err := validateSize(in.Size); err != nil {
		return nil, err
	}
	sl, err := resolveSlug(in.Slug, title)
	if err != nil {
		return nil, err
	}
	desc, err := validateDescription(in.Description)
	if err != nil {
		return nil, err
	}
	source, err := validateSource(in.Source)
	if err != nil {
		return nil, err
	}
	sourceURL, err := validateSourceURL(in.SourceURL)
	if err != nil {
		return nil, err
	}
	if err := s.checkGenre(ctx, in.GenreID); err != nil {
		return nil, err
	}

	c := &domain.Collection{
		ID: uuid.New(), Slug: sl, Title: title, Description: desc, Kind: kind,
		Size: in.Size, Source: source, SourceURL: sourceURL, GenreID: in.GenreID,
		IsPublished: in.IsPublished,
	}
	if kind == domain.KindAuto {
		c.SetRule(rule)
	}
	if in.IsPublished {
		now := s.now()
		c.PublishedAt = &now
	}
	if err := s.repo.Create(ctx, c); err != nil {
		if errors.Is(err, domain.ErrSlugTaken) {
			return nil, err
		}
		return nil, fmt.Errorf("create collection: %w", err)
	}
	if kind == domain.KindAuto {
		s.recomputeLogged(ctx, c.ID)
	}
	return s.repo.GetByID(ctx, c.ID)
}

// recomputeLogged recomputes with a reset baseline; a failure keeps the
// collection as it is (no or old snapshot) and is only logged.
func (s *Service) recomputeLogged(ctx context.Context, id uuid.UUID) {
	if err := s.recompute(ctx, id, true); err != nil {
		s.log.Error("curated: recompute failed", zap.String("collection_id", id.String()), zap.Error(err))
	}
}

// Update applies a partial update.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*domain.Collection, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, wrapGet(err)
	}
	if in.Kind != nil {
		return nil, invalid("kind: cannot be changed")
	}
	before := domain.RuleOf(c)
	sizeBefore, genreBefore := c.Size, c.GenreID
	if in.Rule != nil {
		if !c.IsAuto() {
			return nil, invalid("rule: only for automatic collections")
		}
		n, err := normalizeRule(*in.Rule)
		if err != nil {
			return nil, err
		}
		c.SetRule(n)
	}
	if in.Title != nil {
		t, err := s.validateTitle(*in.Title)
		if err != nil {
			return nil, err
		}
		c.Title = t
	}
	if in.Slug != nil {
		sl, err := resolveSlug(*in.Slug, c.Title)
		if err != nil {
			return nil, err
		}
		c.Slug = sl
	}
	if in.Size != nil {
		if err := validateSize(*in.Size); err != nil {
			return nil, err
		}
		c.Size = *in.Size
	}
	if in.Description != nil {
		d, err := validateDescription(*in.Description)
		if err != nil {
			return nil, err
		}
		c.Description = d
	}
	if in.Source != nil {
		v, err := validateSource(*in.Source)
		if err != nil {
			return nil, err
		}
		c.Source = v
	}
	if in.SourceURL != nil {
		v, err := validateSourceURL(*in.SourceURL)
		if err != nil {
			return nil, err
		}
		c.SourceURL = v
	}
	if in.GenreIDSet {
		if err := s.checkGenre(ctx, in.GenreID); err != nil {
			return nil, err
		}
		c.GenreID = in.GenreID
	}
	if in.IsPublished != nil {
		c.IsPublished = *in.IsPublished
		if c.IsPublished && c.PublishedAt == nil {
			now := s.now()
			c.PublishedAt = &now
		}
	}
	c.Genre = nil
	if err := s.repo.Update(ctx, c); err != nil {
		if errors.Is(err, domain.ErrSlugTaken) {
			return nil, err
		}
		return nil, fmt.Errorf("update collection: %w", err)
	}
	if c.IsAuto() && autoInputsChanged(before, domain.RuleOf(c), sizeBefore, c.Size, genreBefore, c.GenreID) {
		s.recomputeLogged(ctx, id)
	}
	return s.repo.GetByID(ctx, id)
}

func autoInputsChanged(before, after *domain.Rule, sizeBefore, sizeAfter int, genreBefore, genreAfter *uuid.UUID) bool {
	if sizeBefore != sizeAfter {
		return true
	}
	if (genreBefore == nil) != (genreAfter == nil) || (genreBefore != nil && *genreBefore != *genreAfter) {
		return true
	}
	if before == nil || after == nil {
		return before != after
	}
	return !before.Equal(*after)
}

// Recompute is the admin "recompute now" action: it replaces the snapshot of
// an automatic collection, keeping the current one as the movement baseline.
func (s *Service) Recompute(ctx context.Context, id uuid.UUID) (*AdminDetail, error) {
	if err := s.recompute(ctx, id, false); err != nil {
		if isDomainErr(err) {
			return nil, err
		}
		return nil, fmt.Errorf("recompute collection: %w", err)
	}
	return s.GetAdmin(ctx, id)
}

func (s *Service) recompute(ctx context.Context, id uuid.UUID, reset bool) error {
	now := s.now()
	return s.repo.Recompute(ctx, id, reset, now, func(ctx context.Context, c domain.Collection, src repository.AutoSource) ([]domain.ScoredBook, error) {
		rule := domain.RuleOf(&c)
		if rule == nil {
			return nil, domain.ErrNotAutomatic
		}
		var since *time.Time
		if rule.PeriodDays != nil {
			t := now.AddDate(0, 0, -*rule.PeriodDays)
			since = &t
		}
		rows, err := src.AutoRows(ctx, c, since)
		if err != nil {
			return nil, fmt.Errorf("aggregate: %w", err)
		}
		return domain.RankAuto(rows, *rule, domain.SnapshotLimit(c.Size)), nil
	})
}

// RecomputeAll recomputes every automatic collection for the worker. A
// collection that fails keeps its snapshot and does not stop the others; only
// failing to list them is returned.
func (s *Service) RecomputeAll(ctx context.Context) error {
	ids, err := s.repo.AutoCollectionIDs(ctx)
	if err != nil {
		return fmt.Errorf("list auto collections: %w", err)
	}
	for _, id := range ids {
		if err := s.recompute(ctx, id, false); err != nil {
			s.log.Error("curated: scheduled recompute failed", zap.String("collection_id", id.String()), zap.Error(err))
		}
	}
	return nil
}

func wrapGet(err error) error {
	if errors.Is(err, domain.ErrCollectionNotFound) {
		return err
	}
	return fmt.Errorf("get collection: %w", err)
}

// Delete removes a collection and its book list.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	ok, err := s.repo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("delete collection: %w", err)
	}
	if !ok {
		return domain.ErrCollectionNotFound
	}
	return nil
}

// ListAdmin lists every collection, draft or published.
func (s *Service) ListAdmin(ctx context.Context) ([]AdminListItem, error) {
	cs, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	counts, err := s.repo.Counts(ctx, ids(cs))
	if err != nil {
		return nil, fmt.Errorf("count books: %w", err)
	}
	out := make([]AdminListItem, 0, len(cs))
	for _, c := range cs {
		cn := counts[c.ID]
		out = append(out, AdminListItem{Collection: c, BooksCount: cn.Books, ShownCount: domain.ShownCount(cn.Published, c.Size)})
	}
	return out, nil
}

func ids(cs []domain.Collection) []uuid.UUID {
	out := make([]uuid.UUID, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

// GetAdmin returns the collection with every book and its standing.
func (s *Service) GetAdmin(ctx context.Context, id uuid.UUID) (*AdminDetail, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, wrapGet(err)
	}
	items, err := s.repo.AdminItems(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	refs := make([]domain.BookRef, len(items))
	for i, it := range items {
		refs[i] = domain.BookRef{BookID: it.Book.ID, Published: it.Book.IsPublished()}
	}
	ranked := domain.Rank(refs, c.Size)
	d := &AdminDetail{Collection: *c, BooksCount: len(items), Items: make([]AdminDetailItem, 0, len(items))}
	for i, it := range items {
		if ranked[i].State == domain.StateRanked {
			d.ShownCount++
		}
		d.Items = append(d.Items, AdminDetailItem{
			Position: ranked[i].Position, Place: ranked[i].Place, State: ranked[i].State, Book: it.Book,
			Score: it.Score, Movement: movementOf(c, it.PreviousPosition, it.Position),
		})
	}
	return d, nil
}

// movementOf is the movement of a stored position; nil for manual
// collections and for automatic ones without a baseline.
func movementOf(c *domain.Collection, prev *int, pos int) *domain.MovementInfo {
	if !c.IsAuto() {
		return nil
	}
	return domain.Movement(c.BaselineAt != nil, prev, pos)
}

// requireManual loads the collection and rejects automatic ones: their books
// come from the rule, never from hand edits.
func (s *Service) requireManual(ctx context.Context, id uuid.UUID) error {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return wrapGet(err)
	}
	if c.IsAuto() {
		return domain.ErrCollectionAutomatic
	}
	return nil
}

// AddBook queues a catalog book (pending or published) in the collection.
func (s *Service) AddBook(ctx context.Context, id, bookID uuid.UUID, position *int) error {
	if err := s.requireManual(ctx, id); err != nil {
		return err
	}
	b, err := s.repo.GetBook(ctx, bookID)
	if err != nil {
		return fmt.Errorf("get book: %w", err)
	}
	if b == nil {
		return domain.ErrBookNotFound
	}
	if b.Status == catalogDomain.BookStatusRejected {
		return domain.ErrBookRejected
	}
	if err := s.repo.AddBook(ctx, id, bookID, position); err != nil {
		if isDomainErr(err) {
			return err
		}
		return fmt.Errorf("add book: %w", err)
	}
	return nil
}

// RemoveBook removes a book from the collection.
func (s *Service) RemoveBook(ctx context.Context, id, bookID uuid.UUID) error {
	if err := s.requireManual(ctx, id); err != nil {
		return err
	}
	if err := s.repo.RemoveBook(ctx, id, bookID); err != nil {
		if isDomainErr(err) {
			return err
		}
		return fmt.Errorf("remove book: %w", err)
	}
	return nil
}

// MoveBook moves a book to a 1-based position (clamped to the list length).
func (s *Service) MoveBook(ctx context.Context, id, bookID uuid.UUID, position int) error {
	if position < 1 {
		return domain.ErrInvalidPosition
	}
	if err := s.requireManual(ctx, id); err != nil {
		return err
	}
	if err := s.repo.MoveBook(ctx, id, bookID, position); err != nil {
		if isDomainErr(err) {
			return err
		}
		return fmt.Errorf("move book: %w", err)
	}
	return nil
}

func isDomainErr(err error) bool {
	for _, e := range []error{
		domain.ErrCollectionNotFound, domain.ErrBookNotInList, domain.ErrBookAlreadyIn,
		domain.ErrCollectionFull, domain.ErrBookNotFound, domain.ErrBookRejected, domain.ErrTooManyLines,
		domain.ErrCollectionAutomatic, domain.ErrNotAutomatic,
	} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// MaxIndexLimit caps the optional limit of the public index.
const MaxIndexLimit = 50

// ListPublic returns published collections newest first and the total number
// of published collections. limit <= 0 means no limit; it is capped at
// MaxIndexLimit. Summaries (one cover query each) are built only for the
// returned collections.
func (s *Service) ListPublic(ctx context.Context, limit int) ([]Summary, int, error) {
	cs, err := s.repo.ListPublished(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list published: %w", err)
	}
	total := len(cs)
	if limit > MaxIndexLimit {
		limit = MaxIndexLimit
	}
	if limit > 0 && len(cs) > limit {
		cs = cs[:limit]
	}
	counts, err := s.repo.Counts(ctx, ids(cs))
	if err != nil {
		return nil, 0, fmt.Errorf("count books: %w", err)
	}
	out := make([]Summary, 0, len(cs))
	for _, c := range cs {
		sm, err := s.summary(ctx, c, counts[c.ID])
		if err != nil {
			return nil, 0, err
		}
		out = append(out, sm)
	}
	return out, total, nil
}

// ListBookPlaces returns the places of a published book in published
// collections. A missing or non-published book is ErrBookNotFound.
func (s *Service) ListBookPlaces(ctx context.Context, bookID uuid.UUID) ([]repository.BookPlace, error) {
	b, err := s.repo.GetBook(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("get book: %w", err)
	}
	if b == nil || b.Status != catalogDomain.BookStatusPublished {
		return nil, domain.ErrBookNotFound
	}
	places, err := s.repo.BookPlaces(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("book places: %w", err)
	}
	return places, nil
}

func (s *Service) summary(ctx context.Context, c domain.Collection, cn repository.Counts) (Summary, error) {
	covers, err := s.repo.Covers(ctx, c.ID, c.Size)
	if err != nil {
		return Summary{}, fmt.Errorf("covers: %w", err)
	}
	return Summary{Collection: c, ShownCount: domain.ShownCount(cn.Published, c.Size), CoverURLs: covers}, nil
}

// GetPublic returns one page of a published collection's ranked places.
func (s *Service) GetPublic(ctx context.Context, slugValue string, page, limit int) (*PublicPage, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	c, err := s.repo.GetPublishedBySlug(ctx, slugValue)
	if err != nil {
		return nil, wrapGet(err)
	}
	counts, err := s.repo.Counts(ctx, []uuid.UUID{c.ID})
	if err != nil {
		return nil, fmt.Errorf("count books: %w", err)
	}
	sm, err := s.summary(ctx, *c, counts[c.ID])
	if err != nil {
		return nil, err
	}
	offset := (page - 1) * limit
	lim := limit
	if rest := c.Size - offset; rest < lim {
		lim = rest
	}
	res := &PublicPage{Summary: sm, Items: []PublicItem{}, Total: sm.ShownCount, Page: page, Limit: limit}
	if lim <= 0 {
		return res, nil
	}
	books, err := s.repo.PublishedPage(ctx, c.ID, lim, offset)
	if err != nil {
		return nil, fmt.Errorf("published page: %w", err)
	}
	for i, b := range books {
		res.Items = append(res.Items, PublicItem{
			Place: offset + i + 1, Book: b.Book,
			Movement: movementOf(c, b.PreviousPosition, b.Position),
		})
	}
	return res, nil
}
