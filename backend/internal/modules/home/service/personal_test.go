package service

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogdomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	"github.com/bookhaven/backend/internal/modules/home/repository"
	"github.com/google/uuid"
)

// fakeSimilar is a SimilarLister returning canned lists per seed.
type fakeSimilar struct {
	lists map[uuid.UUID][]uuid.UUID
	errs  map[uuid.UUID]error
	calls []uuid.UUID
}

func (f *fakeSimilar) ListSimilarBooks(_ context.Context, bookID uuid.UUID, _ int) ([]catalogdomain.Book, error) {
	f.calls = append(f.calls, bookID)
	if err := f.errs[bookID]; err != nil {
		return nil, err
	}
	var books []catalogdomain.Book
	for _, id := range f.lists[bookID] {
		books = append(books, catalogdomain.Book{ID: id})
	}
	return books, nil
}

func newPersonalService(repo *fakeRepo, loader *fakeLoader, similar SimilarLister, now func() time.Time) *Service {
	opts := []Option{WithClock(now)}
	if similar != nil {
		opts = append(opts, WithSimilarLister(similar))
	}
	return NewService(repo, loader, opts...)
}

func newPersonalFakes() (*fakeRepo, *fakeLoader) {
	repo := newFakeRepo()
	repo.shelves = map[string]*repository.ShelfRef{}
	repo.shelfBooks = map[uuid.UUID][]uuid.UUID{}
	return repo, &fakeLoader{books: map[uuid.UUID]catalogdomain.Book{}}
}

func scored(ids ...uuid.UUID) []repository.ScoredID {
	rows := make([]repository.ScoredID, 0, len(ids))
	for i, id := range ids {
		rows = append(rows, repository.ScoredID{ID: id, Score: float64(len(ids) - i), Count: 1})
	}
	return rows
}

func TestGetPersonalHome_NoSeedsNoRecommendations(t *testing.T) {
	repo, loader := newPersonalFakes()
	similar := &fakeSimilar{}
	home, err := newPersonalService(repo, loader, similar, fixedNow).GetPersonalHome(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetPersonalHome: %v", err)
	}
	if home.Recommendations == nil || len(home.Recommendations) != 0 {
		t.Fatalf("expected empty recommendations, got %+v", home.Recommendations)
	}
	if len(similar.calls) != 0 {
		t.Fatal("no seeds must not call the similar lister")
	}
}

func TestGetPersonalHome_MissingShelvesGiveNilBlocks(t *testing.T) {
	repo, loader := newPersonalFakes()
	user := uuid.New()
	home, err := newPersonalService(repo, loader, nil, fixedNow).GetPersonalHome(context.Background(), user)
	if err != nil {
		t.Fatalf("GetPersonalHome: %v", err)
	}
	if home.ContinueReading != nil || home.WantToReadPick != nil {
		t.Fatalf("expected nil shelf blocks, got %+v", home)
	}
	if len(repo.gotUserIDs) == 0 {
		t.Fatal("expected repository calls")
	}
	for _, got := range repo.gotUserIDs {
		if got != user {
			t.Fatalf("repository called with %s, want %s", got, user)
		}
	}
}

func TestGetPersonalHome_ContinueReading(t *testing.T) {
	repo, loader := newPersonalFakes()
	shelf := uuid.New()
	books := pool(3)
	loader.add(books[0], books[2]) // books[1] was hidden since the selection
	repo.shelves[shelfTypeReading] = &repository.ShelfRef{ID: shelf, Total: 3}
	repo.shelfBooks[shelf] = books

	home, err := newPersonalService(repo, loader, nil, fixedNow).GetPersonalHome(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetPersonalHome: %v", err)
	}
	cr := home.ContinueReading
	if cr == nil || cr.ShelfID != shelf || cr.Total != 3 || len(cr.Books) != 2 || cr.Books[0].ID != books[0] || cr.Books[1].ID != books[2] {
		t.Fatalf("unexpected continue reading %+v", cr)
	}
	if loader.calls != 1 {
		t.Fatalf("expected one hydration query, got %d", loader.calls)
	}
}

func TestGetPersonalHome_NoTopUpWhenTier1Enough(t *testing.T) {
	repo, loader := newPersonalFakes()
	repo.seeds = pool(5)
	tier1 := pool(RecommendationsTopUpBelow)
	loader.add(tier1...)
	repo.precomputed = scored(tier1...)
	similar := &fakeSimilar{}

	home, err := newPersonalService(repo, loader, similar, fixedNow).GetPersonalHome(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetPersonalHome: %v", err)
	}
	if len(similar.calls) != 0 {
		t.Fatalf("tier 2 must not run with %d tier-1 books", len(tier1))
	}
	if len(home.Recommendations) != len(tier1) || home.Recommendations[0].ID != tier1[0] {
		t.Fatalf("unexpected recommendations %+v", home.Recommendations)
	}
}

func TestGetPersonalHome_TopUpFromRecentSeedsSkipsNotFound(t *testing.T) {
	repo, loader := newPersonalFakes()
	seeds := pool(5)
	repo.seeds = seeds
	t1 := uuid.New()
	repo.precomputed = scored(t1)
	s0, s2 := pool(2), pool(2)
	loader.add(t1)
	loader.add(s0...)
	loader.add(s2...)
	similar := &fakeSimilar{
		lists: map[uuid.UUID][]uuid.UUID{seeds[0]: s0, seeds[2]: s2},
		errs:  map[uuid.UUID]error{seeds[1]: catalogservice.ErrBookNotFound},
	}

	home, err := newPersonalService(repo, loader, similar, fixedNow).GetPersonalHome(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetPersonalHome: %v", err)
	}
	if len(similar.calls) != RecommendationsTopUpSeeds {
		t.Fatalf("expected %d top-up calls, got %d", RecommendationsTopUpSeeds, len(similar.calls))
	}
	for i, c := range similar.calls {
		if c != seeds[i] {
			t.Fatalf("top-up must use the most recent seeds in order, got %v", similar.calls)
		}
	}
	want := []uuid.UUID{t1, s0[0], s2[0], s0[1], s2[1]}
	if len(home.Recommendations) != len(want) {
		t.Fatalf("unexpected recommendations %+v", home.Recommendations)
	}
	for i, id := range want {
		if home.Recommendations[i].ID != id {
			t.Fatalf("position %d: got %s, want %s", i, home.Recommendations[i].ID, id)
		}
	}
}

func TestGetPersonalHome_ExcludedNeverAppear(t *testing.T) {
	repo, loader := newPersonalFakes()
	seeds := pool(2)
	repo.seeds = seeds
	onShelf, rated, fresh := uuid.New(), uuid.New(), uuid.New()
	repo.excluded = map[uuid.UUID]struct{}{onShelf: {}, rated: {}}
	repo.precomputed = scored(rated)
	loader.add(onShelf, rated, fresh, seeds[0], seeds[1])
	similar := &fakeSimilar{lists: map[uuid.UUID][]uuid.UUID{
		seeds[0]: {onShelf, seeds[1], fresh},
		seeds[1]: {rated, seeds[0]},
	}}

	home, err := newPersonalService(repo, loader, similar, fixedNow).GetPersonalHome(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetPersonalHome: %v", err)
	}
	if len(home.Recommendations) != 1 || home.Recommendations[0].ID != fresh {
		t.Fatalf("expected only the fresh book, got %+v", home.Recommendations)
	}
}

func TestGetPersonalHome_WantToReadPickStablePerDayAndUser(t *testing.T) {
	repo, loader := newPersonalFakes()
	shelf := uuid.New()
	books := pool(20)
	loader.add(books...)
	repo.shelves[shelfTypeWantToRead] = &repository.ShelfRef{ID: shelf, Total: 20}
	repo.shelfBooks[shelf] = books
	user := uuid.New()

	pickAt := func(now func() time.Time) uuid.UUID {
		t.Helper()
		home, err := newPersonalService(repo, loader, nil, now).GetPersonalHome(context.Background(), user)
		if err != nil {
			t.Fatalf("GetPersonalHome: %v", err)
		}
		p := home.WantToReadPick
		if p == nil || p.Book == nil || p.ShelfID != shelf || p.Total != 20 {
			t.Fatalf("unexpected pick %+v", p)
		}
		return p.Book.ID
	}

	morning := pickAt(fixedNow)
	evening := pickAt(func() time.Time { return fixedNow().Add(10 * time.Hour) })
	if morning != evening {
		t.Fatal("the pick must be stable within a day")
	}
	if next := pickAt(func() time.Time { return fixedNow().Add(24 * time.Hour) }); next == morning {
		t.Fatal("the pick must change on the next day")
	}
	want, _ := pickDaily(books, civilDate(fixedNow()), wantToReadSaltPrefix+user.String())
	if morning != want {
		t.Fatal("the pick must be salted per user")
	}
}

func TestGetPersonalHome_EmptyWantToReadShelf(t *testing.T) {
	repo, loader := newPersonalFakes()
	repo.shelves[shelfTypeWantToRead] = &repository.ShelfRef{ID: uuid.New()}
	home, err := newPersonalService(repo, loader, nil, fixedNow).GetPersonalHome(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetPersonalHome: %v", err)
	}
	if home.WantToReadPick == nil || home.WantToReadPick.Book != nil {
		t.Fatalf("expected a pick block with a nil book, got %+v", home.WantToReadPick)
	}
}

func TestGetPersonalHome_BlockFailures(t *testing.T) {
	repo, loader := newPersonalFakes()
	repo.shelfErr = errors.New("db")
	rec := uuid.New()
	repo.seeds = []uuid.UUID{uuid.New()}
	repo.precomputed = scored(rec)
	loader.add(rec)

	home, err := newPersonalService(repo, loader, nil, fixedNow).GetPersonalHome(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("failed shelf blocks must not fail the request: %v", err)
	}
	if home.ContinueReading != nil || home.WantToReadPick != nil || len(home.Recommendations) != 1 {
		t.Fatalf("unexpected payload %+v", home)
	}

	repo.seedsErr = errors.New("db")
	if _, err := newPersonalService(repo, loader, nil, fixedNow).GetPersonalHome(context.Background(), uuid.New()); !errors.Is(err, ErrAllPersonalBlocksFailed) {
		t.Fatalf("expected ErrAllPersonalBlocksFailed, got %v", err)
	}
}
