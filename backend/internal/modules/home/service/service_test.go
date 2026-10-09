package service

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogdomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/home/repository"
	"github.com/google/uuid"
)

// fakeRepo is an in-memory repository.Repository.
type fakeRepo struct {
	pools      map[repository.PoolFilter][]uuid.UUID
	poolCalls  []repository.PoolFilter
	poolErr    error
	events     map[repository.MonthDay][]repository.CalendarEventRow
	eventCalls int
	eventsErr  error
	topBooks   map[uuid.UUID][]uuid.UUID
	anniv      []repository.AnniversaryRow
	newIDs     []uuid.UUID
	newErr     error
	topRated   []uuid.UUID
	topErr     error
	reviewIDs  []uuid.UUID
	reviewsErr error
	reviews    []repository.ReviewRow

	randomByTier map[int]*uuid.UUID
	randomCalls  []repository.PoolFilter

	// personal
	shelves     map[string]*repository.ShelfRef // by shelf type
	shelfBooks  map[uuid.UUID][]uuid.UUID       // by shelf ID
	shelfErr    error
	seeds       []uuid.UUID
	seedsErr    error
	excluded    map[uuid.UUID]struct{}
	precomputed []repository.ScoredID
	gotUserIDs  []uuid.UUID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		pools:        map[repository.PoolFilter][]uuid.UUID{},
		events:       map[repository.MonthDay][]repository.CalendarEventRow{},
		topBooks:     map[uuid.UUID][]uuid.UUID{},
		randomByTier: map[int]*uuid.UUID{},
	}
}

func (f *fakeRepo) ListPoolIDs(_ context.Context, p repository.PoolFilter) ([]uuid.UUID, error) {
	f.poolCalls = append(f.poolCalls, p)
	if f.poolErr != nil {
		return nil, f.poolErr
	}
	return f.pools[p], nil
}

func (f *fakeRepo) PickRandomBookID(_ context.Context, p repository.PoolFilter) (*uuid.UUID, error) {
	f.randomCalls = append(f.randomCalls, p)
	return f.randomByTier[len(f.randomCalls)-1], nil
}

func (f *fakeRepo) ListCalendarEvents(_ context.Context, days []repository.MonthDay) ([]repository.CalendarEventRow, error) {
	f.eventCalls++
	if f.eventsErr != nil {
		return nil, f.eventsErr
	}
	var rows []repository.CalendarEventRow
	for _, d := range days {
		rows = append(rows, f.events[d]...)
	}
	return rows, nil
}

func (f *fakeRepo) ListTopBooksByAuthors(_ context.Context, authorIDs []uuid.UUID, _ int) (map[uuid.UUID][]uuid.UUID, error) {
	result := map[uuid.UUID][]uuid.UUID{}
	for _, id := range authorIDs {
		result[id] = f.topBooks[id]
	}
	return result, nil
}

func (f *fakeRepo) ListAnniversaryBooks(context.Context, int, int, int) ([]repository.AnniversaryRow, error) {
	return f.anniv, nil
}

func (f *fakeRepo) ListNewBookIDs(context.Context, int) ([]uuid.UUID, error) {
	return f.newIDs, f.newErr
}

func (f *fakeRepo) ListTopRatedBookIDs(context.Context, float64, int, int) ([]uuid.UUID, error) {
	return f.topRated, f.topErr
}

func (f *fakeRepo) ListRecentReviewIDs(context.Context, int, int) ([]uuid.UUID, error) {
	return f.reviewIDs, f.reviewsErr
}

func (f *fakeRepo) GetReviewsByIDs(_ context.Context, ids []uuid.UUID) ([]repository.ReviewRow, error) {
	want := map[uuid.UUID]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var rows []repository.ReviewRow
	for _, r := range f.reviews {
		if want[r.ID] {
			rows = append(rows, r)
		}
	}
	return rows, nil
}

func (f *fakeRepo) GetSystemShelf(_ context.Context, userID uuid.UUID, shelfType string) (*repository.ShelfRef, error) {
	f.gotUserIDs = append(f.gotUserIDs, userID)
	if f.shelfErr != nil {
		return nil, f.shelfErr
	}
	return f.shelves[shelfType], nil
}

func (f *fakeRepo) ListShelfBookIDs(_ context.Context, shelfID uuid.UUID, limit int) ([]uuid.UUID, error) {
	ids := f.shelfBooks[shelfID]
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	return ids, nil
}

func (f *fakeRepo) ListRecommendationSeeds(_ context.Context, userID uuid.UUID, _ int) ([]uuid.UUID, error) {
	f.gotUserIDs = append(f.gotUserIDs, userID)
	return f.seeds, f.seedsErr
}

func (f *fakeRepo) ListUserExcludedBookIDs(context.Context, uuid.UUID) (map[uuid.UUID]struct{}, error) {
	return f.excluded, nil
}

func (f *fakeRepo) AggregatePrecomputedSimilar(context.Context, uuid.UUID, []uuid.UUID, int) ([]repository.ScoredID, error) {
	return f.precomputed, nil
}

// fakeLoader returns visible books by ID.
type fakeLoader struct {
	books map[uuid.UUID]catalogdomain.Book
	calls int
}

func (l *fakeLoader) GetBooksByIDs(_ context.Context, ids []uuid.UUID) ([]catalogdomain.Book, error) {
	l.calls++
	var result []catalogdomain.Book
	for _, id := range ids {
		if b, ok := l.books[id]; ok && (b.Status == "" || b.IsPublished()) {
			result = append(result, b)
		}
	}
	return result, nil
}

func (l *fakeLoader) add(ids ...uuid.UUID) {
	for _, id := range ids {
		l.books[id] = catalogdomain.Book{ID: id, Title: "Book " + id.String()[:4], Slug: id.String()}
	}
}

type fakeCache struct {
	data   map[string][]byte
	getErr error
	sets   int
}

func (c *fakeCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	if c.getErr != nil {
		return nil, false, c.getErr
	}
	v, ok := c.data[key]
	return v, ok, nil
}

func (c *fakeCache) Set(_ context.Context, key string, val []byte, _ time.Duration) error {
	c.sets++
	c.data[key] = val
	return nil
}

// fixedNow is 2026-10-04 12:00 MSK.
func fixedNow() time.Time { return time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC) }

func newTestService(repo *fakeRepo, loader *fakeLoader, cache Cache) *Service {
	opts := []Option{WithClock(fixedNow)}
	if cache != nil {
		opts = append(opts, WithCache(cache))
	}
	return NewService(repo, loader, opts...)
}

func TestGetHome_BookOfTheDayFallbackTiers(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	only := uuid.New()
	loader.add(only)
	// Strict and tier 1 empty; tier 2 (cover only) has one book.
	repo.pools[bookOfTheDayTiers[2]] = []uuid.UUID{only}

	home, err := newTestService(repo, loader, nil).GetHome(context.Background())
	if err != nil {
		t.Fatalf("GetHome: %v", err)
	}
	if home.BookOfTheDay == nil || home.BookOfTheDay.Book.ID != only {
		t.Fatalf("expected the tier-2 book, got %+v", home.BookOfTheDay)
	}
	if len(repo.poolCalls) != 3 {
		t.Fatalf("expected 3 pool queries (strict, tier1, tier2), got %d", len(repo.poolCalls))
	}
	if home.Date != "2026-10-04" {
		t.Fatalf("unexpected date %s", home.Date)
	}
}

func TestGetHome_BookOfTheDayNilWhenAllTiersEmpty(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	home, err := newTestService(repo, loader, nil).GetHome(context.Background())
	if err != nil {
		t.Fatalf("GetHome: %v", err)
	}
	if home.BookOfTheDay != nil || home.Calendar != nil {
		t.Fatalf("expected nil blocks, got %+v", home)
	}
	if home.NewBooks == nil || home.TopRated == nil || home.RecentReviews == nil {
		t.Fatal("arrays must never be nil")
	}
	if len(repo.poolCalls) != len(bookOfTheDayTiers) {
		t.Fatalf("expected all %d tiers, got %d", len(bookOfTheDayTiers), len(repo.poolCalls))
	}
}

func TestGetHome_DescriptionExcerpt(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	id := uuid.New()
	desc := "Короткое описание."
	loader.books[id] = catalogdomain.Book{ID: id, Description: &desc}
	repo.pools[bookOfTheDayTiers[0]] = []uuid.UUID{id}

	home, _ := newTestService(repo, loader, nil).GetHome(context.Background())
	if home.BookOfTheDay == nil || home.BookOfTheDay.DescriptionExcerpt == nil || *home.BookOfTheDay.DescriptionExcerpt != desc {
		t.Fatalf("unexpected excerpt %+v", home.BookOfTheDay)
	}
}

func TestGetHome_CalendarLookahead(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	author := uuid.New()
	book := uuid.New()
	loader.add(book)
	repo.events[repository.MonthDay{Month: 10, Day: 7}] = []repository.CalendarEventRow{{
		AuthorID: author, Name: "Автор", Slug: "avtor", Type: "birth",
		EventDate: day(1900, 10, 7), VisibleBooks: 1,
	}}
	repo.topBooks[author] = []uuid.UUID{book}

	home, err := newTestService(repo, loader, nil).GetHome(context.Background())
	if err != nil {
		t.Fatalf("GetHome: %v", err)
	}
	cal := home.Calendar
	if cal == nil || cal.IsToday || cal.Date != "2026-10-07" {
		t.Fatalf("expected lookahead to 2026-10-07, got %+v", cal)
	}
	if len(cal.Events) != 1 || cal.Events[0].Years != 126 || cal.Events[0].Type != "birth" {
		t.Fatalf("unexpected events %+v", cal.Events)
	}
	if len(cal.Events[0].Books) != 1 || cal.Events[0].Books[0].ID != book {
		t.Fatalf("expected the author's book, got %+v", cal.Events[0].Books)
	}
	if repo.eventCalls != 4 {
		t.Fatalf("expected 4 day queries (today + 3), got %d", repo.eventCalls)
	}
}

func TestGetHome_CalendarTodayDropsFutureYears(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	future, present := uuid.New(), uuid.New()
	fb, pb := uuid.New(), uuid.New()
	loader.add(fb, pb)
	repo.topBooks[future] = []uuid.UUID{fb}
	repo.topBooks[present] = []uuid.UUID{pb}
	repo.events[repository.MonthDay{Month: 10, Day: 4}] = []repository.CalendarEventRow{
		{AuthorID: future, Name: "Будущий", Type: "birth", EventDate: day(2030, 10, 4), VisibleBooks: 1},
		{AuthorID: present, Name: "Настоящий", Type: "death", EventDate: day(1926, 10, 4), VisibleBooks: 1},
	}
	home, _ := newTestService(repo, loader, nil).GetHome(context.Background())
	if home.Calendar == nil || !home.Calendar.IsToday || len(home.Calendar.Events) != 1 {
		t.Fatalf("expected one today event, got %+v", home.Calendar)
	}
	if e := home.Calendar.Events[0]; e.Author.Name != "Настоящий" || e.Years != 100 || e.Type != "death" {
		t.Fatalf("unexpected event %+v", e)
	}
}

func TestGetHome_CalendarDropsEventWhoseBooksWereHidden(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	author, book := uuid.New(), uuid.New()
	loader.add(book)
	repo.topBooks[author] = []uuid.UUID{book}
	repo.events[repository.MonthDay{Month: 10, Day: 4}] = []repository.CalendarEventRow{{
		AuthorID: author, Name: "Автор", Slug: "avtor", Type: "birth",
		EventDate: day(1900, 10, 4), VisibleBooks: 1,
	}}
	cache := &fakeCache{data: map[string][]byte{}}
	svc := newTestService(repo, loader, cache)
	if home, _ := svc.GetHome(context.Background()); home.Calendar == nil {
		t.Fatal("expected a calendar before hiding the book")
	}

	// Hide the author's only book after the selection was cached.
	hidden := loader.books[book]
	hidden.Status = catalogdomain.BookStatusPending
	loader.books[book] = hidden

	home, err := svc.GetHome(context.Background())
	if err != nil {
		t.Fatalf("GetHome: %v", err)
	}
	if home.Calendar != nil {
		t.Fatalf("expected the event without visible books dropped, got %+v", home.Calendar)
	}
}

func TestGetHome_CalendarNilWhenWindowEmpty(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	home, _ := newTestService(repo, loader, nil).GetHome(context.Background())
	if home.Calendar != nil {
		t.Fatalf("expected nil calendar, got %+v", home.Calendar)
	}
	if repo.eventCalls != CalendarLookaheadDays+1 {
		t.Fatalf("expected %d day queries, got %d", CalendarLookaheadDays+1, repo.eventCalls)
	}
}

func TestGetHome_AnniversariesOnly(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	b25, b100 := uuid.New(), uuid.New()
	loader.add(b25, b100)
	repo.anniv = []repository.AnniversaryRow{{BookID: b25, Years: 25}, {BookID: b100, Years: 100}}

	home, _ := newTestService(repo, loader, nil).GetHome(context.Background())
	cal := home.Calendar
	if cal == nil || len(cal.Events) != 0 || len(cal.Anniversaries) != 2 {
		t.Fatalf("unexpected calendar %+v", cal)
	}
	if cal.Anniversaries[0].Book.ID != b100 {
		t.Fatal("expected the 100-year anniversary first")
	}
}

func TestGetHome_CacheHitSkipsSelectionButHydratesLive(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	a, b := uuid.New(), uuid.New()
	loader.add(a, b)
	repo.newIDs = []uuid.UUID{a, b}
	cache := &fakeCache{data: map[string][]byte{}}
	svc := newTestService(repo, loader, cache)

	if _, err := svc.GetHome(context.Background()); err != nil {
		t.Fatalf("first GetHome: %v", err)
	}
	if cache.sets != 1 || cache.data["home:v1:2026-10-04"] == nil {
		t.Fatalf("expected one cache set under home:v1:2026-10-04, got %d", cache.sets)
	}
	calls := len(repo.poolCalls) + repo.eventCalls

	// Hide a book after caching; it must disappear on the next request.
	hidden := loader.books[b]
	hidden.Status = catalogdomain.BookStatusPending
	loader.books[b] = hidden

	home, err := svc.GetHome(context.Background())
	if err != nil {
		t.Fatalf("second GetHome: %v", err)
	}
	if len(repo.poolCalls)+repo.eventCalls != calls {
		t.Fatal("expected a cache hit to skip the selection queries")
	}
	if loader.calls != 2 {
		t.Fatalf("expected live hydration on the hit, got %d loader calls", loader.calls)
	}
	if len(home.NewBooks) != 1 || home.NewBooks[0].ID != a {
		t.Fatalf("expected the hidden book dropped, got %+v", home.NewBooks)
	}
}

func TestGetHome_CacheErrorIsMiss(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	a := uuid.New()
	loader.add(a)
	repo.newIDs = []uuid.UUID{a}
	cache := &fakeCache{data: map[string][]byte{}, getErr: errors.New("redis down")}

	home, err := newTestService(repo, loader, cache).GetHome(context.Background())
	if err != nil {
		t.Fatalf("GetHome: %v", err)
	}
	if len(home.NewBooks) != 1 {
		t.Fatalf("expected computed blocks, got %+v", home.NewBooks)
	}
}

func TestGetHome_OneBlockFailsOthersIntactNoCache(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	a, top := uuid.New(), uuid.New()
	loader.add(a, top)
	repo.newErr = errors.New("db error")
	repo.topRated = []uuid.UUID{top}
	repo.pools[bookOfTheDayTiers[0]] = []uuid.UUID{a}
	cache := &fakeCache{data: map[string][]byte{}}

	home, err := newTestService(repo, loader, cache).GetHome(context.Background())
	if err != nil {
		t.Fatalf("GetHome: %v", err)
	}
	if len(home.NewBooks) != 0 || home.NewBooks == nil {
		t.Fatalf("expected an empty non-nil new_books, got %+v", home.NewBooks)
	}
	if len(home.TopRated) != 1 || home.BookOfTheDay == nil {
		t.Fatal("expected the other blocks intact")
	}
	if cache.sets != 0 {
		t.Fatal("a response with a failed block must not be cached")
	}
}

func TestGetHome_AllBlocksFail(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	dbErr := errors.New("db down")
	repo.poolErr, repo.eventsErr, repo.newErr, repo.topErr, repo.reviewsErr = dbErr, dbErr, dbErr, dbErr, dbErr

	if _, err := newTestService(repo, loader, nil).GetHome(context.Background()); !errors.Is(err, ErrAllBlocksFailed) {
		t.Fatalf("expected ErrAllBlocksFailed, got %v", err)
	}
}

func TestGetHome_RecentReviewsHydrated(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	book, hiddenBook := uuid.New(), uuid.New()
	loader.add(book)
	r1, r2, r3 := uuid.New(), uuid.New(), uuid.New()
	repo.reviewIDs = []uuid.UUID{r1, r2, r3}
	name := "Анна"
	repo.reviews = []repository.ReviewRow{
		{ID: r1, BookID: book, Rating: 5, Content: "Отличная книга, прочитала за один вечер и не пожалела.", UserName: &name},
		{ID: r2, BookID: hiddenBook, Rating: 4, Content: "Отзыв на скрытую книгу, его быть не должно в выдаче."},
		// r3 is hidden: not returned by GetReviewsByIDs.
	}

	home, _ := newTestService(repo, loader, nil).GetHome(context.Background())
	if len(home.RecentReviews) != 1 {
		t.Fatalf("expected one review, got %+v", home.RecentReviews)
	}
	got := home.RecentReviews[0]
	if got.ID != r1 || got.Book.ID != book || got.User.Name == nil || *got.User.Name != "Анна" {
		t.Fatalf("unexpected review %+v", got)
	}
}

func TestRandomBook_TierChainAndFilters(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	picked := uuid.New()
	loader.add(picked)
	repo.randomByTier[1] = &picked // strict empty, tier 1 hits
	genre, exclude := uuid.New(), uuid.New()

	book, err := newTestService(repo, loader, nil).RandomBook(context.Background(), &genre, &exclude)
	if err != nil {
		t.Fatalf("RandomBook: %v", err)
	}
	if book == nil || book.ID != picked {
		t.Fatalf("expected the tier-1 book, got %+v", book)
	}
	if len(repo.randomCalls) != 2 {
		t.Fatalf("expected 2 tier calls, got %d", len(repo.randomCalls))
	}
	strict := repo.randomCalls[0]
	if strict.MinRating != GoodRatingMin || strict.MinRatingsCount != GoodRatingsCountMin || strict.RequireCover {
		t.Fatalf("unexpected strict tier %+v", strict)
	}
	for _, c := range repo.randomCalls {
		if c.GenreID == nil || *c.GenreID != genre || c.ExcludeID == nil || *c.ExcludeID != exclude {
			t.Fatalf("genre/exclude not passed through: %+v", c)
		}
	}
}

func TestRandomBook_NothingFound(t *testing.T) {
	repo := newFakeRepo()
	loader := &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
	book, err := newTestService(repo, loader, nil).RandomBook(context.Background(), nil, nil)
	if err != nil || book != nil {
		t.Fatalf("expected nil, got %+v err=%v", book, err)
	}
	if len(repo.randomCalls) != len(randomTiers) {
		t.Fatalf("expected all %d tiers, got %d", len(randomTiers), len(repo.randomCalls))
	}
}
