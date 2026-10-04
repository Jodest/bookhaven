// Package service composes the home page payload (see
// docs/features/home-page-content.md).
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	catalogdomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/home/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// homeCacheKeyPrefix is versioned so a payload shape change can bump it.
const homeCacheKeyPrefix = "home:v1:"

// ErrAllBlocksFailed is returned when every block of the public home
// payload failed.
var ErrAllBlocksFailed = errors.New("all home blocks failed")

// BookLoader hydrates visible books by ID (hidden IDs are not returned).
// Satisfied by the catalog repository.Repository.
type BookLoader interface {
	GetBooksByIDs(ctx context.Context, ids []uuid.UUID) ([]catalogdomain.Book, error)
}

// SimilarLister returns visible books similar to a book (precomputed, then
// the stage-1 cache, then live). Satisfied by *catalogservice.Service.
type SimilarLister interface {
	ListSimilarBooks(ctx context.Context, bookID uuid.UUID, limit int) ([]catalogdomain.Book, error)
}

// Cache is a fail-fast byte cache. Get errors are treated as a miss and Set
// errors are ignored.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
}

// Service builds the home page payloads.
type Service struct {
	repo    repository.Repository
	books   BookLoader
	cache   Cache
	similar SimilarLister
	logger  *zap.Logger
	now     func() time.Time
}

// Option configures optional Service dependencies.
type Option func(*Service)

// WithCache enables the Redis cache of the public payload selection.
func WithCache(c Cache) Option {
	return func(s *Service) { s.cache = c }
}

// WithSimilarLister enables the tier-2 top-up of personal recommendations.
func WithSimilarLister(l SimilarLister) Option {
	return func(s *Service) { s.similar = l }
}

// WithLogger sets the logger used for per-block failures.
func WithLogger(l *zap.Logger) Option {
	return func(s *Service) {
		if l != nil {
			s.logger = l
		}
	}
}

// WithClock overrides time.Now (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// NewService creates a home Service.
func NewService(repo repository.Repository, books BookLoader, opts ...Option) *Service {
	s := &Service{repo: repo, books: books, logger: zap.NewNop(), now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// BookOfTheDay is the featured book with a short description.
type BookOfTheDay struct {
	Book               catalogdomain.Book
	DescriptionExcerpt *string
}

// CalendarEvent is an author's birth or death anniversary.
type CalendarEvent struct {
	Type   string // "birth" or "death"
	Years  int
	Author repository.CalendarEventRow
	Books  []catalogdomain.Book
}

// Anniversary is a book celebrating a round age this year.
type Anniversary struct {
	Years int
	Book  catalogdomain.Book
}

// Calendar is the literary calendar block.
type Calendar struct {
	Date          string // YYYY-MM-DD of the events (today or a lookahead day)
	IsToday       bool
	Events        []CalendarEvent
	Anniversaries []Anniversary
}

// ReviewUser is the public author of a review.
type ReviewUser struct {
	ID        uuid.UUID
	Name      *string
	AvatarURL *string
}

// RecentReview is a fresh review excerpt with its book.
type RecentReview struct {
	ID         uuid.UUID
	Excerpt    string
	Rating     int
	LikesCount int
	CreatedAt  time.Time
	User       ReviewUser
	Book       catalogdomain.Book
}

// Home is the public home payload. Slices are never nil.
type Home struct {
	Date          string
	BookOfTheDay  *BookOfTheDay
	Calendar      *Calendar
	NewBooks      []catalogdomain.Book
	TopRated      []catalogdomain.Book
	RecentReviews []RecentReview
}

// selection is the ID-level public payload stored in the cache. Books and
// reviews are always hydrated live from it.
type selection struct {
	BookOfTheDay *uuid.UUID   `json:"book_of_the_day,omitempty"`
	Calendar     *calendarSel `json:"calendar,omitempty"`
	NewBooks     []uuid.UUID  `json:"new_books"`
	TopRated     []uuid.UUID  `json:"top_rated"`
	ReviewIDs    []uuid.UUID  `json:"review_ids"`
}

type calendarSel struct {
	Date          string           `json:"date"`
	IsToday       bool             `json:"is_today"`
	Events        []calendarEvent  `json:"events"`
	Anniversaries []anniversarySel `json:"anniversaries"`
}

type anniversarySel struct {
	Years int       `json:"years"`
	ID    uuid.UUID `json:"id"`
}

// GetHome returns the public home payload. A failed block is logged and
// comes back empty; the selection is cached only when no block failed. It
// returns ErrAllBlocksFailed when every block failed.
func (s *Service) GetHome(ctx context.Context) (*Home, error) {
	today := civilDate(s.now())
	key := homeCacheKeyPrefix + dateString(today)

	sel, ok := s.readCache(ctx, key)
	if !ok {
		var failed int
		sel, failed = s.computeSelection(ctx, today)
		if failed == homeBlockCount {
			return nil, ErrAllBlocksFailed
		}
		if failed == 0 {
			s.writeCache(ctx, key, sel)
		}
	}
	return s.hydrate(ctx, today, sel)
}

func (s *Service) readCache(ctx context.Context, key string) (*selection, bool) {
	if s.cache == nil {
		return nil, false
	}
	raw, ok, err := s.cache.Get(ctx, key)
	if err != nil {
		s.logger.Warn("home cache get failed", zap.String("key", key), zap.Error(err))
		return nil, false
	}
	if !ok {
		return nil, false
	}
	var sel selection
	if err := json.Unmarshal(raw, &sel); err != nil {
		s.logger.Warn("home cache decode failed", zap.String("key", key), zap.Error(err))
		return nil, false
	}
	return &sel, true
}

func (s *Service) writeCache(ctx context.Context, key string, sel *selection) {
	if s.cache == nil {
		return
	}
	raw, err := json.Marshal(sel)
	if err != nil {
		s.logger.Warn("home cache encode failed", zap.Error(err))
		return
	}
	// A failed cache write must never fail the request.
	if err := s.cache.Set(ctx, key, raw, HomeCacheTTL); err != nil {
		s.logger.Warn("home cache set failed", zap.String("key", key), zap.Error(err))
	}
}

// homeBlockCount is the number of independently computed public blocks.
const homeBlockCount = 5

// computeSelection computes every block independently and returns the
// number of failed blocks.
func (s *Service) computeSelection(ctx context.Context, today time.Time) (*selection, int) {
	sel := &selection{NewBooks: []uuid.UUID{}, TopRated: []uuid.UUID{}, ReviewIDs: []uuid.UUID{}}
	failed := 0
	fail := func(block string, err error) {
		failed++
		s.logger.Error("home block failed", zap.String("block", block), zap.Error(err))
	}

	if id, err := s.selectBookOfTheDay(ctx, today); err != nil {
		fail("book_of_the_day", err)
	} else {
		sel.BookOfTheDay = id
	}

	if cal, err := s.selectCalendar(ctx, today); err != nil {
		fail("calendar", err)
	} else {
		sel.Calendar = cal
	}

	if ids, err := s.repo.ListNewBookIDs(ctx, RowLimit); err != nil {
		fail("new_books", err)
	} else if ids != nil {
		sel.NewBooks = ids
	}

	if ids, err := s.repo.ListTopRatedBookIDs(ctx, GoodRatingMin, GoodRatingsCountMin, RowLimit); err != nil {
		fail("top_rated", err)
	} else if ids != nil {
		sel.TopRated = ids
	}

	if ids, err := s.repo.ListRecentReviewIDs(ctx, ReviewMinLength, RecentReviewsLimit); err != nil {
		fail("recent_reviews", err)
	} else if ids != nil {
		sel.ReviewIDs = ids
	}

	return sel, failed
}

// bookOfTheDayTiers is the strict pool followed by the fallback tiers.
var bookOfTheDayTiers = []repository.PoolFilter{
	{RequireCover: true, MinRating: GoodRatingMin, MinRatingsCount: GoodRatingsCountMin},
	{RequireCover: true, MinRatingsCount: 1},
	{RequireCover: true},
	{},
}

func (s *Service) selectBookOfTheDay(ctx context.Context, today time.Time) (*uuid.UUID, error) {
	for _, tier := range bookOfTheDayTiers {
		pool, err := s.repo.ListPoolIDs(ctx, tier)
		if err != nil {
			return nil, fmt.Errorf("list book of the day pool: %w", err)
		}
		if id, ok := pickDaily(pool, today, bookOfTheDaySalt); ok {
			return &id, nil
		}
	}
	return nil, nil
}

// selectCalendar finds today's events (or the nearest day with events in
// the lookahead window) and this year's book anniversaries. It returns nil
// when both are empty.
func (s *Service) selectCalendar(ctx context.Context, today time.Time) (*calendarSel, error) {
	cal := &calendarSel{
		Date:          dateString(today),
		IsToday:       true,
		Events:        []calendarEvent{},
		Anniversaries: []anniversarySel{},
	}

	for offset := 0; offset <= CalendarLookaheadDays; offset++ {
		day := today.AddDate(0, 0, offset)
		rows, err := s.repo.ListCalendarEvents(ctx, calendarDays(day))
		if err != nil {
			return nil, fmt.Errorf("list calendar events: %w", err)
		}
		events := make([]calendarEvent, 0, len(rows))
		for _, row := range rows {
			years := eventYears(row.EventDate, day.Year())
			if years <= 0 {
				continue
			}
			events = append(events, calendarEvent{Author: row, Years: years, BookIDs: []uuid.UUID{}})
		}
		if len(events) == 0 {
			continue
		}
		orderCalendarEvents(events)
		if len(events) > CalendarMaxAuthors {
			events = events[:CalendarMaxAuthors]
		}

		authorIDs := make([]uuid.UUID, 0, len(events))
		seen := make(map[uuid.UUID]struct{}, len(events))
		for _, e := range events {
			if _, dup := seen[e.Author.AuthorID]; !dup {
				seen[e.Author.AuthorID] = struct{}{}
				authorIDs = append(authorIDs, e.Author.AuthorID)
			}
		}
		top, err := s.repo.ListTopBooksByAuthors(ctx, authorIDs, CalendarBooksPerAuthor)
		if err != nil {
			return nil, fmt.Errorf("list calendar author books: %w", err)
		}
		for i := range events {
			if ids := top[events[i].Author.AuthorID]; ids != nil {
				events[i].BookIDs = ids
			}
		}

		cal.Date = dateString(day)
		cal.IsToday = offset == 0
		cal.Events = events
		break
	}

	rows, err := s.repo.ListAnniversaryBooks(ctx, today.Year(), AnniversaryStep, AnniversaryLimit)
	if err != nil {
		return nil, fmt.Errorf("list anniversary books: %w", err)
	}
	for _, row := range rows {
		if isAnniversary(row.Years, AnniversaryStep) {
			cal.Anniversaries = append(cal.Anniversaries, anniversarySel{Years: row.Years, ID: row.BookID})
		}
	}
	// The SQL already orders by rank then rating; the stable sort keeps
	// that order and guards the rank rule.
	sort.SliceStable(cal.Anniversaries, func(i, j int) bool {
		return anniversaryRank(cal.Anniversaries[i].Years) < anniversaryRank(cal.Anniversaries[j].Years)
	})
	if len(cal.Anniversaries) > AnniversaryLimit {
		cal.Anniversaries = cal.Anniversaries[:AnniversaryLimit]
	}

	if len(cal.Events) == 0 && len(cal.Anniversaries) == 0 {
		return nil, nil
	}
	return cal, nil
}

// hydrate reads reviews and books live (one query each) and drops IDs
// that are no longer visible.
func (s *Service) hydrate(ctx context.Context, today time.Time, sel *selection) (*Home, error) {
	home := &Home{
		Date:          dateString(today),
		NewBooks:      []catalogdomain.Book{},
		TopRated:      []catalogdomain.Book{},
		RecentReviews: []RecentReview{},
	}

	var reviews []repository.ReviewRow
	if len(sel.ReviewIDs) > 0 {
		rows, err := s.repo.GetReviewsByIDs(ctx, sel.ReviewIDs)
		if err != nil {
			s.logger.Error("home block failed", zap.String("block", "recent_reviews"), zap.Error(err))
		} else {
			reviews = rows
		}
	}

	ids := make([]uuid.UUID, 0)
	seen := make(map[uuid.UUID]struct{})
	add := func(id uuid.UUID) {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	if sel.BookOfTheDay != nil {
		add(*sel.BookOfTheDay)
	}
	if sel.Calendar != nil {
		for _, e := range sel.Calendar.Events {
			for _, id := range e.BookIDs {
				add(id)
			}
		}
		for _, a := range sel.Calendar.Anniversaries {
			add(a.ID)
		}
	}
	for _, id := range sel.NewBooks {
		add(id)
	}
	for _, id := range sel.TopRated {
		add(id)
	}
	for _, r := range reviews {
		add(r.BookID)
	}

	byID := make(map[uuid.UUID]catalogdomain.Book, len(ids))
	if len(ids) > 0 {
		books, err := s.books.GetBooksByIDs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("hydrate home books: %w", err)
		}
		for _, b := range books {
			byID[b.ID] = b
		}
	}
	pick := func(list []uuid.UUID) []catalogdomain.Book {
		result := make([]catalogdomain.Book, 0, len(list))
		for _, id := range list {
			if b, ok := byID[id]; ok {
				result = append(result, b)
			}
		}
		return result
	}

	if sel.BookOfTheDay != nil {
		if b, ok := byID[*sel.BookOfTheDay]; ok {
			botd := &BookOfTheDay{Book: b}
			if b.Description != nil {
				if ex := excerpt(*b.Description, DescriptionExcerptRunes); ex != "" {
					botd.DescriptionExcerpt = &ex
				}
			}
			home.BookOfTheDay = botd
		}
	}

	if sel.Calendar != nil {
		cal := &Calendar{
			Date:          sel.Calendar.Date,
			IsToday:       sel.Calendar.IsToday,
			Events:        make([]CalendarEvent, 0, len(sel.Calendar.Events)),
			Anniversaries: make([]Anniversary, 0, len(sel.Calendar.Anniversaries)),
		}
		for _, e := range sel.Calendar.Events {
			books := pick(e.BookIDs)
			// All of the author's books were hidden after the selection was
			// cached: drop the event rather than show an author with no books.
			if len(books) == 0 {
				continue
			}
			cal.Events = append(cal.Events, CalendarEvent{
				Type:   e.Author.Type,
				Years:  e.Years,
				Author: e.Author,
				Books:  books,
			})
		}
		for _, a := range sel.Calendar.Anniversaries {
			if b, ok := byID[a.ID]; ok {
				cal.Anniversaries = append(cal.Anniversaries, Anniversary{Years: a.Years, Book: b})
			}
		}
		if len(cal.Events) > 0 || len(cal.Anniversaries) > 0 {
			home.Calendar = cal
		}
	}

	home.NewBooks = pick(sel.NewBooks)
	home.TopRated = pick(sel.TopRated)

	reviewByID := make(map[uuid.UUID]repository.ReviewRow, len(reviews))
	for _, r := range reviews {
		reviewByID[r.ID] = r
	}
	for _, id := range sel.ReviewIDs {
		r, ok := reviewByID[id]
		if !ok {
			continue // hidden since caching
		}
		b, ok := byID[r.BookID]
		if !ok {
			continue // book hidden since caching
		}
		home.RecentReviews = append(home.RecentReviews, RecentReview{
			ID:         r.ID,
			Excerpt:    excerpt(r.Content, ReviewExcerptRunes),
			Rating:     r.Rating,
			LikesCount: r.LikesCount,
			CreatedAt:  r.CreatedAt,
			User:       ReviewUser{ID: r.UserID, Name: r.UserName, AvatarURL: r.UserAvatarURL},
			Book:       b,
		})
	}

	return home, nil
}

// randomTiers is the "well-rated" pool (no cover requirement) followed by
// the fallback tiers.
var randomTiers = []repository.PoolFilter{
	{MinRating: GoodRatingMin, MinRatingsCount: GoodRatingsCountMin},
	{MinRatingsCount: 1},
	{},
}

// RandomBook returns one random visible book, optionally restricted to a
// genre and excluding one book, or nil when nothing matches. It is never
// cached.
func (s *Service) RandomBook(ctx context.Context, genreID, excludeID *uuid.UUID) (*catalogdomain.Book, error) {
	for _, tier := range randomTiers {
		tier.GenreID = genreID
		tier.ExcludeID = excludeID
		id, err := s.repo.PickRandomBookID(ctx, tier)
		if err != nil {
			return nil, fmt.Errorf("pick random book: %w", err)
		}
		if id == nil {
			continue
		}
		books, err := s.books.GetBooksByIDs(ctx, []uuid.UUID{*id})
		if err != nil {
			return nil, fmt.Errorf("hydrate random book: %w", err)
		}
		if len(books) == 0 {
			return nil, nil // hidden between the pick and the read
		}
		b := books[0]
		return &b, nil
	}
	return nil, nil
}
