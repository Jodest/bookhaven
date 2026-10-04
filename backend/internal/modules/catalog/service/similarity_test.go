package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/catalog/repository"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const eps = 1e-9

func userSeq(n int) uuid.UUID { return idSeq(5000 + n) }

// readAll makes each user read every book in books.
func readAll(users []int, books ...uuid.UUID) []repository.Interaction {
	var out []repository.Interaction
	for _, u := range users {
		for _, b := range books {
			out = append(out, repository.Interaction{UserID: userSeq(u), BookID: b})
		}
	}
	return out
}

func rng(from, to int) []int {
	var out []int
	for i := from; i <= to; i++ {
		out = append(out, i)
	}
	return out
}

func findPartner(list []CoPartner, id uuid.UUID) (CoPartner, bool) {
	for _, p := range list {
		if p.BookID == id {
			return p, true
		}
	}
	return CoPartner{}, false
}

// --- coReading ---

func TestCoReading_KThreshold(t *testing.T) {
	a, b, c := idSeq(1), idSeq(2), idSeq(3)
	inter := append(readAll(rng(1, 2), a, b), readAll(rng(3, 5), a, c)...)

	partners, _, _ := coReading(inter, MinCoReaders, MaxUserInteractions)
	if _, ok := findPartner(partners[a], b); ok {
		t.Fatal("co = 2 must not produce a partner")
	}
	if _, ok := partners[b]; ok {
		t.Fatal("book with only co = 2 must have no partners")
	}
	if p, ok := findPartner(partners[a], c); !ok || p.Co != 3 {
		t.Fatalf("co = 3 must produce a partner a->c, got %+v %v", p, ok)
	}
	if p, ok := findPartner(partners[c], a); !ok || p.Co != 3 {
		t.Fatalf("co = 3 must produce a partner c->a, got %+v %v", p, ok)
	}
}

func TestCoReading_CosineValues(t *testing.T) {
	a, b := idSeq(1), idSeq(2)
	filler := idSeq(99)
	// Users 1-3 read a and b; users 4-5 read a (+ filler so they count);
	// user 6 reads b (+ filler). n(a)=5, n(b)=4, co=3.
	inter := readAll(rng(1, 3), a, b)
	inter = append(inter, readAll(rng(4, 5), a, filler)...)
	inter = append(inter, readAll([]int{6}, b, filler)...)

	partners, readers, used := coReading(inter, MinCoReaders, MaxUserInteractions)
	if readers[a] != 5 || readers[b] != 4 || used != 6 {
		t.Fatalf("readers a=%d b=%d used=%d", readers[a], readers[b], used)
	}
	want := 3 / math.Sqrt(5*4)
	p, ok := findPartner(partners[a], b)
	if !ok || math.Abs(p.Score-want) > eps {
		t.Fatalf("expected cosine %v, got %+v", want, p)
	}
	q, _ := findPartner(partners[b], a)
	if math.Abs(q.Score-want) > eps {
		t.Fatalf("cosine must be symmetric, got %v", q.Score)
	}
}

func TestCoReading_BestsellerBelowNiche(t *testing.T) {
	src, niche, best := idSeq(1), idSeq(2), idSeq(3)
	// Users 1-3 read src, niche, best (co = 3 for both); users 4-20 read
	// only best (+ a filler book each, so they count).
	inter := readAll(rng(1, 3), src, niche, best)
	for u := 4; u <= 20; u++ {
		inter = append(inter, readAll([]int{u}, best, idSeq(100+u))...)
	}
	partners, _, _ := coReading(inter, MinCoReaders, MaxUserInteractions)
	list := partners[src]
	if len(list) != 2 || list[0].BookID != niche || list[1].BookID != best {
		t.Fatalf("expected niche before bestseller, got %+v", list)
	}
	if list[0].Co != list[1].Co {
		t.Fatalf("expected equal co, got %+v", list)
	}
}

func TestCoReading_HeavyUserIgnored(t *testing.T) {
	a, b := idSeq(1), idSeq(2)
	inter := readAll(rng(1, 3), a, b)
	// User 4 has 501 interactions including a and b.
	heavy := []uuid.UUID{a, b}
	for i := 0; i < 499; i++ {
		heavy = append(heavy, idSeq(1000+i))
	}
	inter = append(inter, readAll([]int{4}, heavy...)...)

	partners, readers, used := coReading(inter, MinCoReaders, MaxUserInteractions)
	if readers[a] != 3 || readers[b] != 3 {
		t.Fatalf("heavy user must not count in n(x): a=%d b=%d", readers[a], readers[b])
	}
	if used != 3 {
		t.Fatalf("expected 3 users used, got %d", used)
	}
	if p, _ := findPartner(partners[a], b); p.Co != 3 {
		t.Fatalf("expected co 3, got %+v", p)
	}
	if _, ok := readers[idSeq(1000)]; ok {
		t.Fatal("heavy user's books must not be counted")
	}

	// Exactly 500 is still allowed.
	inter = readAll(rng(1, 3), a, b)
	inter = append(inter, readAll([]int{4}, heavy[:500]...)...)
	_, readers, _ = coReading(inter, MinCoReaders, MaxUserInteractions)
	if readers[a] != 4 {
		t.Fatalf("user with 500 interactions must count, got n(a)=%d", readers[a])
	}
}

func TestCoReading_SingleInteractionUserContributesNothing(t *testing.T) {
	a, b := idSeq(1), idSeq(2)
	inter := readAll(rng(1, 3), a, b)
	inter = append(inter, readAll([]int{4}, a)...)

	partners, readers, used := coReading(inter, MinCoReaders, MaxUserInteractions)
	if readers[a] != 3 || used != 3 {
		t.Fatalf("single-interaction user must be dropped: n(a)=%d used=%d", readers[a], used)
	}
	if p, _ := findPartner(partners[a], b); math.Abs(p.Score-1) > eps {
		t.Fatalf("expected cosine 1, got %+v", p)
	}
}

func TestCoReading_DuplicatesDoNotDoubleCount(t *testing.T) {
	a, b := idSeq(1), idSeq(2)
	inter := readAll(rng(1, 2), a, b)
	// User 2 again (e.g. both shelf and rating) - still co = 2.
	inter = append(inter, readAll([]int{2}, a, b)...)
	partners, readers, _ := coReading(inter, MinCoReaders, MaxUserInteractions)
	if len(partners) != 0 || readers[a] != 2 {
		t.Fatalf("duplicates must not count: partners=%v n(a)=%d", partners, readers[a])
	}
}

func TestCoReading_Empty(t *testing.T) {
	partners, readers, used := coReading(nil, MinCoReaders, MaxUserInteractions)
	if partners == nil || readers == nil || len(partners) != 0 || len(readers) != 0 || used != 0 {
		t.Fatalf("expected empty maps, got %v %v %d", partners, readers, used)
	}
}

// --- blendSimilar ---

var blendSrcID = idSeq(1000)

func TestCoWeight(t *testing.T) {
	cases := []struct {
		n    int
		want float64
	}{{0, 0}, {3, 0.09}, {10, 0.3}, {20, 0.6}, {100, 0.6}}
	for _, tc := range cases {
		if got := coWeight(tc.n); math.Abs(got-tc.want) > eps {
			t.Errorf("coWeight(%d) = %v, want %v", tc.n, got, tc.want)
		}
	}
}

func TestBlendSimilar_ComponentsAndWeight(t *testing.T) {
	src := repository.SimilarSource{GenreIDs: []uuid.UUID{idSeq(800)}}
	contentOnly := repository.SimilarCandidate{BookID: idSeq(1), SharedGenres: 1, GenreCount: 1}
	both := repository.SimilarCandidate{BookID: idSeq(2), SharedGenres: 1, GenreCount: 1}
	partners := []CoPartner{
		{BookID: idSeq(2), Co: 4, Score: 0.5},
		{BookID: idSeq(3), Co: 3, Score: 0.8},
	}
	for _, n := range []int{3, 20, 100} {
		rows := blendSimilar(blendSrcID, src, []repository.SimilarCandidate{contentOnly, both}, partners, n,
			map[uuid.UUID]int{}, map[uuid.UUID][]uuid.UUID{}, MaxSimilarLimit)
		if len(rows) != 3 {
			t.Fatalf("n=%d: expected 3 rows, got %+v", n, rows)
		}
		w := coWeight(n)
		content := GenreWeight / MaxContentScore
		byID := map[uuid.UUID]repository.SimilarityRow{}
		for _, r := range rows {
			byID[r.SimilarBookID] = r
		}
		co := byID[idSeq(1)]
		if math.Abs(co.ContentScore-content) > eps || co.CoScore != 0 || co.CoReaders != 0 ||
			math.Abs(co.Score-(1-w)*content) > eps {
			t.Errorf("n=%d content-only row wrong: %+v", n, co)
		}
		bo := byID[idSeq(2)]
		if math.Abs(bo.Score-((1-w)*content+w*0.5)) > eps || bo.CoReaders != 4 {
			t.Errorf("n=%d both row wrong: %+v", n, bo)
		}
		cp := byID[idSeq(3)]
		if cp.ContentScore != 0 || math.Abs(cp.Score-w*0.8) > eps || cp.CoReaders != 3 {
			t.Errorf("n=%d co-only row wrong: %+v", n, cp)
		}
	}
}

func TestBlendSimilar_WeightShiftsOrder(t *testing.T) {
	src := repository.SimilarSource{GenreIDs: []uuid.UUID{idSeq(800)}}
	contentOnly := repository.SimilarCandidate{BookID: idSeq(1), SharedGenres: 1, GenreCount: 1}
	partners := []CoPartner{{BookID: idSeq(2), Co: 3, Score: 1}}

	// Few readers: content (2/5.6 ≈ 0.357·0.91) beats co (1·0.09).
	rows := blendSimilar(blendSrcID, src, []repository.SimilarCandidate{contentOnly}, partners, 3, nil, nil, MaxSimilarLimit)
	if rows[0].SimilarBookID != idSeq(1) {
		t.Fatalf("n=3: expected content first, got %+v", rows)
	}
	// Many readers: co (0.6) beats content (0.357·0.4).
	rows = blendSimilar(blendSrcID, src, []repository.SimilarCandidate{contentOnly}, partners, 20, nil, nil, MaxSimilarLimit)
	if rows[0].SimilarBookID != idSeq(2) {
		t.Fatalf("n=20: expected co first, got %+v", rows)
	}
}

func TestBlendSimilar_AuthorCapForCoOnly(t *testing.T) {
	author := idSeq(900)
	src := repository.SimilarSource{AuthorIDs: []uuid.UUID{author}}
	cands := []repository.SimilarCandidate{
		{BookID: idSeq(1), SharedAuthors: 1, AuthorIDs: []uuid.UUID{author}},
		{BookID: idSeq(2), SharedAuthors: 1, AuthorIDs: []uuid.UUID{author}},
	}
	// Co-only partner by the same author: blocked by the cap via the map.
	partners := []CoPartner{{BookID: idSeq(3), Co: 3, Score: 0.1}, {BookID: idSeq(4), Co: 3, Score: 0.05}}
	authors := map[uuid.UUID][]uuid.UUID{idSeq(3): {author}, idSeq(4): {idSeq(901)}}
	rows := blendSimilar(blendSrcID, src, cands, partners, 20, nil, authors, MaxSimilarLimit)
	got := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		got = append(got, r.SimilarBookID)
	}
	assertIDs(t, got, []uuid.UUID{idSeq(1), idSeq(2), idSeq(4)})
}

func TestBlendSimilar_TruncatesPositionsAndExcludesSource(t *testing.T) {
	src := repository.SimilarSource{GenreIDs: []uuid.UUID{idSeq(800)}}
	var cands []repository.SimilarCandidate
	for i := 1; i <= 15; i++ {
		cands = append(cands, repository.SimilarCandidate{BookID: idSeq(i), SharedGenres: 1, GenreCount: 1, RatingsCount: i})
	}
	cands = append(cands, repository.SimilarCandidate{BookID: blendSrcID, SharedGenres: 1, GenreCount: 1})
	var partners []CoPartner
	for i := 16; i <= 30; i++ {
		partners = append(partners, CoPartner{BookID: idSeq(i), Co: 3, Score: 0.5})
	}
	partners = append(partners, CoPartner{BookID: blendSrcID, Co: 9, Score: 1})

	rows := blendSimilar(blendSrcID, src, cands, partners, 10, nil, nil, MaxSimilarLimit)
	if len(rows) != MaxSimilarLimit {
		t.Fatalf("expected %d rows, got %d", MaxSimilarLimit, len(rows))
	}
	for i, r := range rows {
		if r.Position != i {
			t.Fatalf("row %d has position %d", i, r.Position)
		}
		if r.BookID != blendSrcID {
			t.Fatalf("row %d has book_id %s", i, r.BookID)
		}
		if r.SimilarBookID == blendSrcID {
			t.Fatal("source book must never be in its own list")
		}
	}
	if rows := blendSimilar(blendSrcID, src, cands, partners, 10, nil, nil, 0); len(rows) != 0 {
		t.Fatalf("max 0 must give no rows, got %d", len(rows))
	}
}

func TestBlendSimilar_DeterministicTieBreak(t *testing.T) {
	src := repository.SimilarSource{}
	partners := []CoPartner{
		{BookID: idSeq(3), Co: 3, Score: 0.5},
		{BookID: idSeq(1), Co: 3, Score: 0.5},
		{BookID: idSeq(2), Co: 3, Score: 0.5},
		{BookID: idSeq(4), Co: 3, Score: 0.5},
	}
	rc := map[uuid.UUID]int{idSeq(4): 10}
	want := []uuid.UUID{idSeq(4), idSeq(1), idSeq(2), idSeq(3)}
	for range 2 {
		rows := blendSimilar(blendSrcID, src, nil, partners, 20, rc, nil, MaxSimilarLimit)
		got := make([]uuid.UUID, 0, len(rows))
		for _, r := range rows {
			got = append(got, r.SimilarBookID)
		}
		assertIDs(t, got, want)
		partners[0], partners[3] = partners[3], partners[0]
	}
}

// --- SimilarityJob ---

// fakeSimilarityStore is a hand-rolled SimilarityStore fake.
type fakeSimilarityStore struct {
	interactions []repository.Interaction
	stats        []repository.BookStat
	authors      map[uuid.UUID][]uuid.UUID

	interErr, statsErr, authorsErr, replaceErr error

	replaceCalls int
	replaced     []repository.SimilarityRow
}

func (f *fakeSimilarityStore) ListInteractions(context.Context) ([]repository.Interaction, error) {
	return f.interactions, f.interErr
}
func (f *fakeSimilarityStore) ListVisibleBookStats(context.Context) ([]repository.BookStat, error) {
	return f.stats, f.statsErr
}
func (f *fakeSimilarityStore) ListBookAuthorIDs(context.Context) (map[uuid.UUID][]uuid.UUID, error) {
	return f.authors, f.authorsErr
}
func (f *fakeSimilarityStore) ReplaceSimilarities(_ context.Context, rows []repository.SimilarityRow) error {
	f.replaceCalls++
	f.replaced = rows
	return f.replaceErr
}

// jobRepo wraps fakeRepository and records the stage-1 calls per book.
type jobRepo struct {
	*fakeRepository
	sourceCalls map[uuid.UUID]int
	candCalls   map[uuid.UUID]int
	onSource    func()
}

func newJobRepo() *jobRepo {
	return &jobRepo{fakeRepository: newFakeRepository(), sourceCalls: map[uuid.UUID]int{}, candCalls: map[uuid.UUID]int{}}
}

func (r *jobRepo) GetSimilarSource(ctx context.Context, id uuid.UUID) (*repository.SimilarSource, error) {
	r.sourceCalls[id]++
	if r.onSource != nil {
		r.onSource()
	}
	return r.fakeRepository.GetSimilarSource(ctx, id)
}

func (r *jobRepo) ListSimilarCandidates(ctx context.Context, id uuid.UUID, src repository.SimilarSource, ap, gp int) ([]repository.SimilarCandidate, error) {
	r.candCalls[id]++
	return r.fakeRepository.ListSimilarCandidates(ctx, id, src, ap, gp)
}

// jobFixture: books 1-3 are co-read by users 1-3; book 4 is read by user 4
// together with book 1 only (co = 1, no partner).
func jobFixture() (*jobRepo, *fakeSimilarityStore) {
	repo := newJobRepo()
	for i := 1; i <= 4; i++ {
		repo.books = append(repo.books, domain.Book{ID: idSeq(i), Genres: []domain.Genre{{ID: idSeq(800)}}})
	}
	store := &fakeSimilarityStore{
		interactions: append(readAll(rng(1, 3), idSeq(1), idSeq(2), idSeq(3)), readAll([]int{4}, idSeq(1), idSeq(4))...),
		authors:      map[uuid.UUID][]uuid.UUID{},
	}
	return repo, store
}

func TestSimilarityJob_NoPartnersClearsTable(t *testing.T) {
	repo := newJobRepo()
	store := &fakeSimilarityStore{interactions: readAll(rng(1, 2), idSeq(1), idSeq(2))}
	stats, err := NewSimilarityJob(repo, store, nil).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.replaceCalls != 1 || store.replaced != nil {
		t.Fatalf("expected ReplaceSimilarities(nil), got calls=%d rows=%v", store.replaceCalls, store.replaced)
	}
	if len(repo.sourceCalls) != 0 || stats.Rows != 0 {
		t.Fatalf("expected no stage-1 queries, got %v", repo.sourceCalls)
	}
}

func TestSimilarityJob_OnlyBooksWithPartnersQueried(t *testing.T) {
	repo, store := jobFixture()
	job := NewSimilarityJob(repo, store, nil)
	fixed := time.Date(2026, 10, 4, 3, 30, 0, 0, time.UTC)
	job.now = func() time.Time { return fixed }

	stats, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if repo.sourceCalls[idSeq(i)] != 1 || repo.candCalls[idSeq(i)] != 1 {
			t.Fatalf("book %d: expected 1 source + 1 candidates query, got %d/%d", i, repo.sourceCalls[idSeq(i)], repo.candCalls[idSeq(i)])
		}
	}
	if repo.sourceCalls[idSeq(4)] != 0 || repo.candCalls[idSeq(4)] != 0 {
		t.Fatal("book without partners must not be queried")
	}
	if store.replaceCalls != 1 || stats.BooksWithRows != 3 || stats.Rows != len(store.replaced) || stats.Rows != 6 {
		t.Fatalf("unexpected stats %+v, rows %d", stats, len(store.replaced))
	}
	for _, r := range store.replaced {
		if r.BookID == idSeq(4) || r.SimilarBookID == idSeq(4) {
			t.Fatalf("book 4 (co = 1) must not appear: %+v", r)
		}
		if !r.ComputedAt.Equal(fixed) {
			t.Fatalf("expected computed_at %v, got %v", fixed, r.ComputedAt)
		}
	}
}

func TestSimilarityJob_SourceNotFoundSkipped(t *testing.T) {
	repo, store := jobFixture()
	repo.books[1].IsHidden = true // book 2 hidden mid-run
	stats, err := NewSimilarityJob(repo, store, nil).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Skipped != 1 || stats.BooksWithRows != 2 {
		t.Fatalf("expected 1 skipped and 2 books with rows, got %+v", stats)
	}
	for _, r := range store.replaced {
		if r.BookID == idSeq(2) {
			t.Fatalf("skipped source must have no rows: %+v", r)
		}
	}
}

func TestSimilarityJob_ErrorsDoNotWrite(t *testing.T) {
	boom := errors.New("boom")
	cases := map[string]func(*jobRepo, *fakeSimilarityStore){
		"interactions": func(_ *jobRepo, s *fakeSimilarityStore) { s.interErr = boom },
		"stats":        func(_ *jobRepo, s *fakeSimilarityStore) { s.statsErr = boom },
		"authors":      func(_ *jobRepo, s *fakeSimilarityStore) { s.authorsErr = boom },
		"source":       func(r *jobRepo, _ *fakeSimilarityStore) { r.similarSourceErr = boom },
		"candidates":   func(r *jobRepo, _ *fakeSimilarityStore) { r.candidatesErr = boom },
	}
	for name, mutate := range cases {
		repo, store := jobFixture()
		mutate(repo, store)
		_, err := NewSimilarityJob(repo, store, nil).Run(context.Background())
		if !errors.Is(err, boom) {
			t.Errorf("%s: expected wrapped error, got %v", name, err)
		}
		if store.replaceCalls != 0 {
			t.Errorf("%s: ReplaceSimilarities must not be called", name)
		}
	}

	repo, store := jobFixture()
	store.replaceErr = boom
	if _, err := NewSimilarityJob(repo, store, nil).Run(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("expected replace error, got %v", err)
	}
}

func TestSimilarityJob_CancelledContextAbortsBeforeWrite(t *testing.T) {
	repo, store := jobFixture()
	ctx, cancel := context.WithCancel(context.Background())
	repo.onSource = cancel // cancel while processing the first book
	_, err := NewSimilarityJob(repo, store, nil).Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if store.replaceCalls != 0 {
		t.Fatal("ReplaceSimilarities must not be called after cancel")
	}
}

// --- asynq glue ---

func TestHandleTask_RunsAndPropagatesError(t *testing.T) {
	repo, store := jobFixture()
	job := NewSimilarityJob(repo, store, nil)
	if err := job.HandleTask(context.Background(), NewRecomputeSimilaritiesTask()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.replaceCalls != 1 {
		t.Fatalf("expected Run to write once, got %d", store.replaceCalls)
	}
	boom := errors.New("boom")
	store.interErr = boom
	if err := job.HandleTask(context.Background(), NewRecomputeSimilaritiesTask()); !errors.Is(err, boom) {
		t.Fatalf("expected propagated error, got %v", err)
	}
}

func TestRecomputeTaskOptions(t *testing.T) {
	var queue string
	var maxRetry int
	var timeout, unique time.Duration
	for _, opt := range RecomputeTaskOptions() {
		switch opt.Type() {
		case asynq.QueueOpt:
			queue = opt.Value().(string)
		case asynq.MaxRetryOpt:
			maxRetry = opt.Value().(int)
		case asynq.TimeoutOpt:
			timeout = opt.Value().(time.Duration)
		case asynq.UniqueOpt:
			unique = opt.Value().(time.Duration)
		}
	}
	if queue != "similarity" || SimilarityQueue != "similarity" {
		t.Errorf("queue = %q", queue)
	}
	if maxRetry != 2 || timeout != 30*time.Minute || unique != 6*time.Hour {
		t.Errorf("maxRetry=%d timeout=%v unique=%v", maxRetry, timeout, unique)
	}
	if TaskTypeRecomputeSimilarities != "similarity:recompute" || NewRecomputeSimilaritiesTask().Type() != TaskTypeRecomputeSimilarities {
		t.Error("unexpected task type")
	}
}

// --- ListSimilarBooks with a precomputed reader ---

// fakePrecomputedReader is a hand-rolled PrecomputedSimilarReader fake.
type fakePrecomputedReader struct {
	ids   map[uuid.UUID][]uuid.UUID
	err   error
	calls int
}

func (f *fakePrecomputedReader) ListPrecomputedSimilar(_ context.Context, bookID uuid.UUID, limit int) ([]uuid.UUID, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	ids := f.ids[bookID]
	return ids[:min(limit, len(ids))], nil
}

func TestListSimilarBooks_PrecomputedWins(t *testing.T) {
	repo := newFakeRepository()
	cache := newFakeSimilarCache()
	src, want := seedSimilar(repo, 5)
	cache.data[src.ID] = want
	pre := &fakePrecomputedReader{ids: map[uuid.UUID][]uuid.UUID{src.ID: {want[3], want[1]}}}
	svc := NewService(repo, WithSimilarCache(cache), WithPrecomputedSimilar(pre))

	books, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertIDs(t, bookIDs(books), []uuid.UUID{want[3], want[1]})
	if cache.gets != 0 || cache.sets != 0 || repo.candidateCalls != 0 {
		t.Fatalf("expected no cache/candidate access, got gets=%d sets=%d cands=%d", cache.gets, cache.sets, repo.candidateCalls)
	}
}

func TestListSimilarBooks_EmptyPrecomputedFallsBack(t *testing.T) {
	repo := newFakeRepository()
	cache := newFakeSimilarCache()
	src, want := seedSimilar(repo, 3)
	pre := &fakePrecomputedReader{}
	svc := NewService(repo, WithSimilarCache(cache), WithPrecomputedSimilar(pre))

	books, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertIDs(t, bookIDs(books), want)
	if pre.calls != 1 || cache.gets != 1 || repo.candidateCalls != 1 || cache.sets != 1 {
		t.Fatalf("expected stage-1 fallback, got pre=%d gets=%d cands=%d sets=%d", pre.calls, cache.gets, repo.candidateCalls, cache.sets)
	}
}

func TestListSimilarBooks_PrecomputedErrorIsError(t *testing.T) {
	repo := newFakeRepository()
	cache := newFakeSimilarCache()
	src, _ := seedSimilar(repo, 3)
	pre := &fakePrecomputedReader{err: errRepoFailure}
	svc := NewService(repo, WithSimilarCache(cache), WithPrecomputedSimilar(pre))

	_, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if !errors.Is(err, errRepoFailure) || errors.Is(err, ErrBookNotFound) {
		t.Fatalf("expected wrapped reader error, got %v", err)
	}
	if cache.gets != 0 || repo.candidateCalls != 0 {
		t.Fatal("reader error must not fall back")
	}
}

func TestListSimilarBooks_NilReaderIsStage1(t *testing.T) {
	repo := newFakeRepository()
	cache := newFakeSimilarCache()
	src, want := seedSimilar(repo, 3)
	svc := NewService(repo, WithSimilarCache(cache), WithPrecomputedSimilar(nil))

	books, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertIDs(t, bookIDs(books), want)
	if cache.gets != 1 || repo.candidateCalls != 1 {
		t.Fatalf("expected stage-1 path, got gets=%d cands=%d", cache.gets, repo.candidateCalls)
	}
}

func TestListSimilarBooks_PrecomputedHiddenDroppedAndLimit(t *testing.T) {
	repo := newFakeRepository()
	src, want := seedSimilar(repo, 4)
	repo.books[2].IsHidden = true // want[1]
	pre := &fakePrecomputedReader{ids: map[uuid.UUID][]uuid.UUID{src.ID: {want[0], want[1], want[2], want[3]}}}
	svc := NewService(repo, WithPrecomputedSimilar(pre))

	books, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertIDs(t, bookIDs(books), []uuid.UUID{want[0], want[2], want[3]})

	books, err = svc.ListSimilarBooks(context.Background(), src.ID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertIDs(t, bookIDs(books), []uuid.UUID{want[0]})
}

func TestListSimilarBooks_HiddenSourceNotFoundBeforePrecomputed(t *testing.T) {
	repo := newFakeRepository()
	src, want := seedSimilar(repo, 2)
	repo.books[0].IsHidden = true
	pre := &fakePrecomputedReader{ids: map[uuid.UUID][]uuid.UUID{src.ID: want}}
	svc := NewService(repo, WithPrecomputedSimilar(pre))

	_, err := svc.ListSimilarBooks(context.Background(), src.ID, 8)
	if !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("expected ErrBookNotFound, got %v", err)
	}
	if pre.calls != 0 {
		t.Fatal("precomputed read must happen after the visibility check")
	}
}
