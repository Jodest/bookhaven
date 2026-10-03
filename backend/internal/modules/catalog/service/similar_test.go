package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/catalog/repository"
	"github.com/google/uuid"
)

func mustUUID(s string) uuid.UUID {
	return uuid.MustParse(s)
}

func idSeq(n int) uuid.UUID {
	return mustUUID(fmt.Sprintf("00000000-0000-0000-0000-%012d", n))
}

func assertIDs(t *testing.T, got, want []uuid.UUID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d ids %v, got %d ids %v", len(want), want, len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d: expected %s, got %s (full: %v)", i, want[i], got[i], got)
		}
	}
}

// --- rankSimilar ---

func TestRankSimilar_SameAuthorOutranksGenreOnlyWithMaxBonus(t *testing.T) {
	author := idSeq(900)
	sameAuthor := repository.SimilarCandidate{BookID: idSeq(1), SharedAuthors: 1, SharedGenres: 0, GenreCount: 3, Rating: 0, RatingsCount: 0, AuthorIDs: []uuid.UUID{author}}
	// Perfect genre overlap and maximum quality bonus.
	genreOnly := repository.SimilarCandidate{BookID: idSeq(2), SharedAuthors: 0, SharedGenres: 2, GenreCount: 2, Rating: 5, RatingsCount: 1_000_000, AuthorIDs: []uuid.UUID{idSeq(901)}}

	got := rankSimilar(2, []uuid.UUID{author}, []repository.SimilarCandidate{genreOnly, sameAuthor}, MaxSimilarLimit)
	assertIDs(t, got, []uuid.UUID{idSeq(1), idSeq(2)})
}

func TestRankSimilar_JaccardOrdering(t *testing.T) {
	// Source has 2 genres. A: 2 shared of 2 (J=1). B: 1 shared of 3 (J=1/4).
	a := repository.SimilarCandidate{BookID: idSeq(2), SharedGenres: 2, GenreCount: 2}
	b := repository.SimilarCandidate{BookID: idSeq(1), SharedGenres: 1, GenreCount: 3}

	got := rankSimilar(2, nil, []repository.SimilarCandidate{b, a}, MaxSimilarLimit)
	assertIDs(t, got, []uuid.UUID{idSeq(2), idSeq(1)})
}

func TestRankSimilar_QualityBonusBreaksContentTies(t *testing.T) {
	low := repository.SimilarCandidate{BookID: idSeq(1), SharedGenres: 1, GenreCount: 1, Rating: 3.0, RatingsCount: 10}
	highRating := repository.SimilarCandidate{BookID: idSeq(2), SharedGenres: 1, GenreCount: 1, Rating: 4.5, RatingsCount: 10}
	popular := repository.SimilarCandidate{BookID: idSeq(3), SharedGenres: 1, GenreCount: 1, Rating: 3.0, RatingsCount: 500}

	got := rankSimilar(1, nil, []repository.SimilarCandidate{low, highRating, popular}, MaxSimilarLimit)
	// highRating: +0.09 rating over low; popular: +0.3-0.24=+0.06 pop over low.
	assertIDs(t, got, []uuid.UUID{idSeq(2), idSeq(3), idSeq(1)})
}

func TestRankSimilar_PerAuthorCap(t *testing.T) {
	author := idSeq(900)
	other := idSeq(901)
	var cands []repository.SimilarCandidate
	for i := 1; i <= 5; i++ {
		cands = append(cands, repository.SimilarCandidate{
			BookID: idSeq(i), SharedAuthors: 1, RatingsCount: 100 - i, AuthorIDs: []uuid.UUID{author},
		})
	}
	// Co-authored with the capped author: blocked even though `other` has no books yet.
	cands = append(cands, repository.SimilarCandidate{
		BookID: idSeq(6), SharedAuthors: 1, RatingsCount: 1, AuthorIDs: []uuid.UUID{other, author},
	})
	// Only `other`: allowed.
	cands = append(cands, repository.SimilarCandidate{
		BookID: idSeq(7), SharedGenres: 1, GenreCount: 1, AuthorIDs: []uuid.UUID{other},
	})

	got := rankSimilar(1, []uuid.UUID{author}, cands, MaxSimilarLimit)
	assertIDs(t, got, []uuid.UUID{idSeq(1), idSeq(2), idSeq(7)})
}

func TestRankSimilar_CoAuthoredBlockedWhenSecondAuthorCapped(t *testing.T) {
	a, b := idSeq(900), idSeq(901)
	cands := []repository.SimilarCandidate{
		{BookID: idSeq(1), SharedAuthors: 1, RatingsCount: 30, AuthorIDs: []uuid.UUID{b}},
		{BookID: idSeq(2), SharedAuthors: 1, RatingsCount: 20, AuthorIDs: []uuid.UUID{b}},
		{BookID: idSeq(3), SharedAuthors: 1, RatingsCount: 10, AuthorIDs: []uuid.UUID{a, b}},
		{BookID: idSeq(4), SharedAuthors: 1, RatingsCount: 5, AuthorIDs: []uuid.UUID{a}},
	}
	got := rankSimilar(0, []uuid.UUID{a, b}, cands, MaxSimilarLimit)
	assertIDs(t, got, []uuid.UUID{idSeq(1), idSeq(2), idSeq(4)})
}

func TestRankSimilar_BooksWithoutAuthorsNotCapped(t *testing.T) {
	var cands []repository.SimilarCandidate
	for i := 1; i <= 5; i++ {
		cands = append(cands, repository.SimilarCandidate{BookID: idSeq(i), SharedGenres: 1, GenreCount: 1})
	}
	got := rankSimilar(1, nil, cands, MaxSimilarLimit)
	if len(got) != 5 {
		t.Fatalf("expected all 5 author-less books, got %d", len(got))
	}
}

func TestRankSimilar_TruncatesToMax(t *testing.T) {
	var cands []repository.SimilarCandidate
	for i := 1; i <= 30; i++ {
		cands = append(cands, repository.SimilarCandidate{BookID: idSeq(i), SharedGenres: 1, GenreCount: 1, RatingsCount: i})
	}
	got := rankSimilar(1, nil, cands, MaxSimilarLimit)
	if len(got) != MaxSimilarLimit {
		t.Fatalf("expected %d ids, got %d", MaxSimilarLimit, len(got))
	}
	got = rankSimilar(1, nil, cands, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 ids, got %d", len(got))
	}
}

func TestRankSimilar_DeterministicTieBreak(t *testing.T) {
	// Equal score; ratings_count DESC first, then book ID ascending.
	c1 := repository.SimilarCandidate{BookID: idSeq(3), SharedGenres: 1, GenreCount: 1, Rating: 4, RatingsCount: 0}
	c2 := repository.SimilarCandidate{BookID: idSeq(1), SharedGenres: 1, GenreCount: 1, Rating: 4, RatingsCount: 0}
	c3 := repository.SimilarCandidate{BookID: idSeq(2), SharedGenres: 1, GenreCount: 1, Rating: 4, RatingsCount: 0}

	want := []uuid.UUID{idSeq(1), idSeq(2), idSeq(3)}
	assertIDs(t, rankSimilar(1, nil, []repository.SimilarCandidate{c1, c2, c3}, MaxSimilarLimit), want)
	assertIDs(t, rankSimilar(1, nil, []repository.SimilarCandidate{c3, c1, c2}, MaxSimilarLimit), want)

	// Same score but more ratings wins (popBonus is capped, so large counts tie on score).
	big := repository.SimilarCandidate{BookID: idSeq(9), SharedGenres: 1, GenreCount: 1, RatingsCount: 100_000}
	bigger := repository.SimilarCandidate{BookID: idSeq(8), SharedGenres: 1, GenreCount: 1, RatingsCount: 200_000}
	small := repository.SimilarCandidate{BookID: idSeq(7), SharedGenres: 1, GenreCount: 1, RatingsCount: 50_000}
	assertIDs(t, rankSimilar(1, nil, []repository.SimilarCandidate{small, big, bigger}, MaxSimilarLimit),
		[]uuid.UUID{idSeq(8), idSeq(9), idSeq(7)})
}

func TestRankSimilar_ZeroDenominatorNoNaN(t *testing.T) {
	c := repository.SimilarCandidate{BookID: idSeq(1), SharedAuthors: 1, SharedGenres: 0, GenreCount: 0}
	score := similarScore(0, true, c)
	if math.IsNaN(score) || math.IsInf(score, 0) {
		t.Fatalf("expected finite score, got %v", score)
	}
	if score != AuthorWeight {
		t.Fatalf("expected score %v, got %v", AuthorWeight, score)
	}
	got := rankSimilar(0, nil, []repository.SimilarCandidate{c}, MaxSimilarLimit)
	assertIDs(t, got, []uuid.UUID{idSeq(1)})
}

func TestRankSimilar_EmptyIsNonNil(t *testing.T) {
	got := rankSimilar(0, nil, nil, MaxSimilarLimit)
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty non-nil slice, got %#v", got)
	}
}

func TestRankSimilar_NonPositiveLimitIsEmpty(t *testing.T) {
	cands := []repository.SimilarCandidate{{BookID: uuid.New(), SharedGenres: 1, GenreCount: 1}}
	for _, limit := range []int{0, -1} {
		got := rankSimilar(1, nil, cands, limit)
		if got == nil || len(got) != 0 {
			t.Fatalf("limit %d: expected empty non-nil slice, got %#v", limit, got)
		}
	}
}

// --- ListSimilarBooks ---

type fakeSimilarCache struct {
	data    map[uuid.UUID][]uuid.UUID
	getErr  error
	setErr  error
	gets    int
	sets    int
	lastSet []uuid.UUID
	lastTTL time.Duration
}

func newFakeSimilarCache() *fakeSimilarCache {
	return &fakeSimilarCache{data: map[uuid.UUID][]uuid.UUID{}}
}

func (c *fakeSimilarCache) Get(ctx context.Context, bookID uuid.UUID) ([]uuid.UUID, bool, error) {
	c.gets++
	if c.getErr != nil {
		return nil, false, c.getErr
	}
	ids, ok := c.data[bookID]
	return ids, ok, nil
}

func (c *fakeSimilarCache) Set(ctx context.Context, bookID uuid.UUID, ids []uuid.UUID, ttl time.Duration) error {
	c.sets++
	c.lastSet = ids
	c.lastTTL = ttl
	if c.setErr != nil {
		return c.setErr
	}
	c.data[bookID] = ids
	return nil
}

// seedSimilar creates a source book (one genre) plus n visible genre-only
// candidate books with distinct authors, ranked by descending ratings count.
func seedSimilar(repo *fakeRepository, n int) (domain.Book, []uuid.UUID) {
	genre := domain.Genre{ID: idSeq(800), Name: "G", Slug: "g"}
	src := domain.Book{ID: idSeq(1000), Slug: "src", Genres: []domain.Genre{genre}}
	repo.books = append(repo.books, src)
	var want []uuid.UUID
	for i := 1; i <= n; i++ {
		id := idSeq(i)
		repo.books = append(repo.books, domain.Book{ID: id, Slug: fmt.Sprintf("b%d", i), Genres: []domain.Genre{genre}})
		repo.candidates = append(repo.candidates, repository.SimilarCandidate{
			BookID: id, SharedGenres: 1, GenreCount: 1, RatingsCount: 1000 - i,
		})
		want = append(want, id)
	}
	return src, want
}

func bookIDs(books []domain.Book) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(books))
	for _, b := range books {
		ids = append(ids, b.ID)
	}
	return ids
}

func TestListSimilarBooks_NotFound(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo)
	_, err := svc.ListSimilarBooks(context.Background(), uuid.New(), 8)
	if !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("expected ErrBookNotFound, got %v", err)
	}
}

func TestListSimilarBooks_HiddenSourceNotFoundEvenWithWarmCache(t *testing.T) {
	repo := newFakeRepository()
	cache := newFakeSimilarCache()
	svc := NewService(repo, WithSimilarCache(cache))
	src, want := seedSimilar(repo, 3)
	cache.data[src.ID] = want
	repo.books[0].IsHidden = true

	_, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("expected ErrBookNotFound, got %v", err)
	}
}

func TestListSimilarBooks_CacheMissComputesAndSets(t *testing.T) {
	repo := newFakeRepository()
	cache := newFakeSimilarCache()
	svc := NewService(repo, WithSimilarCache(cache))
	src, want := seedSimilar(repo, 25)

	books, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.candidateCalls != 1 {
		t.Fatalf("expected 1 candidate query, got %d", repo.candidateCalls)
	}
	if cache.sets != 1 {
		t.Fatalf("expected 1 cache Set, got %d", cache.sets)
	}
	assertIDs(t, cache.lastSet, want[:MaxSimilarLimit])
	if cache.lastTTL != 6*time.Hour {
		t.Fatalf("expected 6h TTL, got %v", cache.lastTTL)
	}
	assertIDs(t, bookIDs(books), want[:8])
}

func TestListSimilarBooks_CacheHitSkipsCandidates(t *testing.T) {
	repo := newFakeRepository()
	cache := newFakeSimilarCache()
	svc := NewService(repo, WithSimilarCache(cache))
	src, want := seedSimilar(repo, 5)
	cached := []uuid.UUID{want[4], want[0]}
	cache.data[src.ID] = cached

	books, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.candidateCalls != 0 {
		t.Fatalf("expected no candidate query on hit, got %d", repo.candidateCalls)
	}
	if cache.sets != 0 {
		t.Fatalf("expected no Set on hit, got %d", cache.sets)
	}
	assertIDs(t, bookIDs(books), cached)
}

func TestListSimilarBooks_CacheErrorsFailOpen(t *testing.T) {
	repo := newFakeRepository()
	cache := newFakeSimilarCache()
	cache.getErr = errors.New("redis down")
	cache.setErr = errors.New("redis down")
	svc := NewService(repo, WithSimilarCache(cache))
	src, want := seedSimilar(repo, 3)

	books, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if err != nil {
		t.Fatalf("expected cache errors to be ignored, got %v", err)
	}
	if repo.candidateCalls != 1 || cache.sets != 1 {
		t.Fatalf("expected fallthrough compute + attempted Set, got calls=%d sets=%d", repo.candidateCalls, cache.sets)
	}
	assertIDs(t, bookIDs(books), want)
}

func TestListSimilarBooks_HitWithNowHiddenBookDropsIt(t *testing.T) {
	repo := newFakeRepository()
	cache := newFakeSimilarCache()
	svc := NewService(repo, WithSimilarCache(cache))
	src, want := seedSimilar(repo, 4)
	cache.data[src.ID] = want
	// Hide want[1] (books[0] is the source).
	repo.books[2].IsHidden = true

	books, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertIDs(t, bookIDs(books), []uuid.UUID{want[0], want[2], want[3]})
}

func TestListSimilarBooks_LimitSlicingAndClamp(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo, WithSimilarCache(newFakeSimilarCache()))
	src, want := seedSimilar(repo, 25)
	ctx := context.Background()

	cases := []struct {
		limit int
		want  int
	}{{8, 8}, {0, DefaultSimilarLimit}, {-3, DefaultSimilarLimit}, {50, MaxSimilarLimit}, {3, 3}}
	for _, tc := range cases {
		books, err := svc.ListSimilarBooks(ctx, src.ID, tc.limit)
		if err != nil {
			t.Fatalf("limit %d: unexpected error: %v", tc.limit, err)
		}
		assertIDs(t, bookIDs(books), want[:tc.want])
	}
}

func TestListSimilarBooks_NilCacheWorks(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo)
	src, want := seedSimilar(repo, 2)

	books, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertIDs(t, bookIDs(books), want)
}

func TestListSimilarBooks_SourceWithoutAuthorsOrGenresIsEmpty(t *testing.T) {
	repo := newFakeRepository()
	cache := newFakeSimilarCache()
	svc := NewService(repo, WithSimilarCache(cache))
	src := domain.Book{ID: idSeq(1000), Slug: "lonely"}
	repo.books = []domain.Book{src}

	books, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if books == nil || len(books) != 0 {
		t.Fatalf("expected empty non-nil result, got %#v", books)
	}
	if cache.sets != 1 || cache.lastSet == nil || len(cache.lastSet) != 0 {
		t.Fatalf("expected empty list to be cached, got sets=%d last=%#v", cache.sets, cache.lastSet)
	}
}

func TestListSimilarBooks_CandidateErrorPropagates(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo)
	src, _ := seedSimilar(repo, 1)
	repo.candidatesErr = errRepoFailure

	_, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if err == nil || errors.Is(err, ErrBookNotFound) {
		t.Fatalf("expected wrapped repo error, got %v", err)
	}
}
