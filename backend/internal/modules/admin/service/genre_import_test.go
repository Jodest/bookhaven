package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// Genre import from Wikidata (docs/features/genre-wikidata-import.md).

type fakeGenreSource struct {
	candidates []bookdata.GenreCandidate
	searchErr  error
	searchQs   []string

	classes       map[string]bookdata.GenreClassification
	classifyErr   error
	classifyBlock bool // wait for the deadline instead of answering
	classifyCalls [][]string

	records    map[string]*bookdata.ExternalGenreRecord
	fetchErr   error
	fetchCalls []string
}

func (f *fakeGenreSource) SearchGenres(ctx context.Context, q string) ([]bookdata.GenreCandidate, error) {
	f.searchQs = append(f.searchQs, q)
	return f.candidates, f.searchErr
}

func (f *fakeGenreSource) ClassifyGenres(ctx context.Context, qids []string) (map[string]bookdata.GenreClassification, error) {
	f.classifyCalls = append(f.classifyCalls, qids)
	if f.classifyBlock {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.classifyErr != nil {
		return nil, f.classifyErr
	}
	out := map[string]bookdata.GenreClassification{}
	for _, q := range qids {
		if c, ok := f.classes[q]; ok {
			out[q] = c
		}
	}
	return out, nil
}

func (f *fakeGenreSource) FetchGenre(ctx context.Context, qid string) (*bookdata.ExternalGenreRecord, error) {
	f.fetchCalls = append(f.fetchCalls, qid)
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	if r, ok := f.records[qid]; ok {
		return r, nil
	}
	return nil, bookdata.ErrNotFound
}

func newGenreImportService() (*Service, *fakeRepository, *fakeGenreSource) {
	repo := &fakeRepository{authorsExist: true, genresExist: true}
	src := &fakeGenreSource{classes: map[string]bookdata.GenreClassification{}, records: map[string]*bookdata.ExternalGenreRecord{}}
	return NewService(repo, WithGenreWikidataSource(src)), repo, src
}

func qidGenre(name, slug, status, qid string) catalogDomain.Genre {
	g := catalogDomain.Genre{ID: uuid.New(), Name: name, Slug: slug, Status: status}
	if qid != "" {
		q := qid
		g.WikidataQID = &q
	}
	return g
}

func literaryRecord(qid, name string) *bookdata.ExternalGenreRecord {
	return &bookdata.ExternalGenreRecord{QID: qid, Name: name, InstanceOf: []string{"Q223393"}}
}

// --- search ---

func TestSearchWikidataGenres_FiltersAndLabels(t *testing.T) {
	svc, _, src := newGenreImportService()
	src.candidates = []bookdata.GenreCandidate{
		{QID: "Q1", Label: "фэнтези", Description: "жанр"},
		{QID: "Q2", Label: "фэнтези", Description: "фильм"},
		{QID: "Q3", Label: "fantasy-ish", Description: ""},
	}
	src.classes = map[string]bookdata.GenreClassification{
		"Q1": {RuLabel: "фэнтези", IsLiteraryGenre: true},
		"Q2": {RuLabel: "фэнтези (фильм)", IsLiteraryGenre: false},
		"Q3": {RuLabel: "", IsLiteraryGenre: true},
	}

	res, err := svc.SearchWikidataGenres(context.Background(), "  фэнтези ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.searchQs[0] != "фэнтези" {
		t.Errorf("query passed = %q", src.searchQs[0])
	}
	if res.ClassCheck != GenreClassCheckOK || len(res.Items) != 2 {
		t.Fatalf("unexpected result %+v", res)
	}
	if res.Items[0].QID != "Q1" || res.Items[0].RuLabel == nil || *res.Items[0].RuLabel != "фэнтези" ||
		res.Items[0].LiteraryGenre == nil || !*res.Items[0].LiteraryGenre {
		t.Errorf("unexpected first item %+v", res.Items[0])
	}
	if res.Items[1].QID != "Q3" || res.Items[1].RuLabel != nil {
		t.Errorf("Q3 must be kept with a nil ru label, got %+v", res.Items[1])
	}
	if len(src.classifyCalls) != 1 || len(src.classifyCalls[0]) != 3 {
		t.Errorf("expected one classification of all candidates, got %v", src.classifyCalls)
	}
}

func TestSearchWikidataGenres_CapsAfterFiltering(t *testing.T) {
	svc, _, src := newGenreImportService()
	for i := 1; i <= 20; i++ {
		qid := fmt.Sprintf("Q%d", i)
		src.candidates = append(src.candidates, bookdata.GenreCandidate{QID: qid, Label: qid})
		// Every odd candidate is a genre: 10 of 20.
		src.classes[qid] = bookdata.GenreClassification{RuLabel: "жанр", IsLiteraryGenre: i%2 == 1}
	}
	src.classes["Q2"] = bookdata.GenreClassification{RuLabel: "жанр", IsLiteraryGenre: true} // 11 genres

	res, err := svc.SearchWikidataGenres(context.Background(), "жанр")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Items) != 10 {
		t.Fatalf("expected 10 items, got %d", len(res.Items))
	}
	if res.Items[0].QID != "Q1" || res.Items[1].QID != "Q2" || res.Items[9].QID != "Q17" {
		t.Fatalf("search order not kept: %v %v %v", res.Items[0].QID, res.Items[1].QID, res.Items[9].QID)
	}
}

func TestSearchWikidataGenres_ClassificationFallback(t *testing.T) {
	cases := map[string]func(src *fakeGenreSource){
		"error":    func(src *fakeGenreSource) { src.classifyErr = bookdata.ErrSourceUnavailable },
		"deadline": func(src *fakeGenreSource) { src.classifyBlock = true },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _, src := newGenreImportService()
			svc.genreClassifyTimeout = 10 * time.Millisecond
			for i := 1; i <= 12; i++ {
				qid := fmt.Sprintf("Q%d", i)
				src.candidates = append(src.candidates, bookdata.GenreCandidate{QID: qid, Label: qid})
			}
			setup(src)

			res, err := svc.SearchWikidataGenres(context.Background(), "x")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.ClassCheck != GenreClassCheckUnavailable || len(res.Items) != 10 {
				t.Fatalf("expected 10 unfiltered items with unavailable, got %s/%d", res.ClassCheck, len(res.Items))
			}
			for _, it := range res.Items {
				if it.RuLabel != nil || it.LiteraryGenre != nil {
					t.Fatalf("unknown classification must give nil fields, got %+v", it)
				}
			}
		})
	}
}

func TestSearchWikidataGenres_ZeroResults_NoClassify(t *testing.T) {
	svc, repo, src := newGenreImportService()
	res, err := svc.SearchWikidataGenres(context.Background(), "ничего")
	if err != nil || res.ClassCheck != GenreClassCheckOK || res.Items == nil || len(res.Items) != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(src.classifyCalls) != 0 || repo.listGenresByQIDsCalls != 0 {
		t.Fatalf("expected no classify and no marker query, got %d/%d", len(src.classifyCalls), repo.listGenresByQIDsCalls)
	}
}

func TestSearchWikidataGenres_ExistingMarkers(t *testing.T) {
	svc, repo, src := newGenreImportService()
	src.candidates = []bookdata.GenreCandidate{{QID: "Q1"}, {QID: "Q2"}, {QID: "Q3"}}
	for _, q := range []string{"Q1", "Q2", "Q3"} {
		src.classes[q] = bookdata.GenreClassification{RuLabel: "жанр", IsLiteraryGenre: true}
	}
	direct := qidGenre("роман", "roman", catalogDomain.GenreStatusPending, "Q1")
	target := qidGenre("фэнтези", "fentezi", catalogDomain.GenreStatusPublished, "")
	q2 := "Q2"
	alias := aliasOf(target.ID, "фентези", "fentezi-2", &q2)
	repo.genres = []catalogDomain.Genre{direct, target, alias}

	res, err := svc.SearchWikidataGenres(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.listGenresByQIDsCalls != 1 {
		t.Fatalf("expected one marker query, got %d", repo.listGenresByQIDsCalls)
	}
	m1, m2, m3 := res.Items[0].Existing, res.Items[1].Existing, res.Items[2].Existing
	if m1 == nil || m1.Genre.ID != direct.ID || m1.ViaAlias != nil {
		t.Errorf("direct marker = %+v", m1)
	}
	if m2 == nil || m2.Genre.ID != target.ID || m2.ViaAlias == nil || m2.ViaAlias.ID != alias.ID {
		t.Errorf("alias marker = %+v", m2)
	}
	if m3 != nil {
		t.Errorf("Q3 must have no marker, got %+v", m3)
	}
}

func TestSearchWikidataGenres_MarkerFailuresGiveNil(t *testing.T) {
	t.Run("lookup error", func(t *testing.T) {
		svc, repo, src := newGenreImportService()
		src.candidates = []bookdata.GenreCandidate{{QID: "Q1"}}
		src.classes["Q1"] = bookdata.GenreClassification{RuLabel: "роман", IsLiteraryGenre: true}
		repo.genres = []catalogDomain.Genre{qidGenre("роман", "roman", catalogDomain.GenreStatusPublished, "Q1")}
		repo.listGenresByQIDsErr = errors.New("db down")

		res, err := svc.SearchWikidataGenres(context.Background(), "роман")
		if err != nil || len(res.Items) != 1 || res.Items[0].Existing != nil {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("broken alias", func(t *testing.T) {
		svc, repo, src := newGenreImportService()
		src.candidates = []bookdata.GenreCandidate{{QID: "Q1"}}
		src.classes["Q1"] = bookdata.GenreClassification{RuLabel: "роман", IsLiteraryGenre: true}
		q1 := "Q1"
		repo.genres = []catalogDomain.Genre{aliasOf(uuid.New(), "роман", "roman", &q1)} // target missing

		res, err := svc.SearchWikidataGenres(context.Background(), "роман")
		if err != nil || len(res.Items) != 1 || res.Items[0].Existing != nil {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
}

func TestSearchWikidataGenres_Errors(t *testing.T) {
	t.Run("nil source", func(t *testing.T) {
		svc, _ := newService()
		if _, err := svc.SearchWikidataGenres(context.Background(), "x"); !errors.Is(err, ErrGenreImportUnavailable) {
			t.Fatalf("err = %v, want ErrGenreImportUnavailable", err)
		}
	})
	t.Run("search failure passes through", func(t *testing.T) {
		svc, _, src := newGenreImportService()
		src.searchErr = bookdata.ErrRateLimited
		if _, err := svc.SearchWikidataGenres(context.Background(), "x"); !errors.Is(err, bookdata.ErrRateLimited) {
			t.Fatalf("err = %v, want ErrRateLimited", err)
		}
	})
}

// --- import ---

func TestImportWikidataGenre_QIDMatchShortCircuits(t *testing.T) {
	ctx := context.Background()
	for _, status := range []string{catalogDomain.GenreStatusPending, catalogDomain.GenreStatusPublished} {
		t.Run(status, func(t *testing.T) {
			svc, repo, src := newGenreImportService()
			existing := qidGenre("роман", "roman", status, "Q8261")
			repo.genres = append(repo.genres, existing)

			res, err := svc.ImportWikidataGenre(ctx, " q8261 ")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Outcome != GenreOutcomeExisting || res.Match != GenreMatchQID || res.Genre.ID != existing.ID || res.ViaAlias != nil {
				t.Fatalf("unexpected result %+v", res)
			}
			if len(src.fetchCalls) != 0 || len(src.classifyCalls) != 0 {
				t.Fatalf("no Wikidata call expected, got fetch=%v classify=%v", src.fetchCalls, src.classifyCalls)
			}
		})
	}

	t.Run("rejected", func(t *testing.T) {
		svc, repo, src := newGenreImportService()
		repo.genres = append(repo.genres, qidGenre("мусор", "musor", catalogDomain.GenreStatusRejected, "Q1"))
		res, err := svc.ImportWikidataGenre(ctx, "Q1")
		if err != nil || res.Outcome != GenreOutcomeRejected || res.Match != GenreMatchQID {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if len(src.fetchCalls) != 0 || len(repo.genres) != 1 {
			t.Fatalf("expected no fetch and nothing created")
		}
	})

	t.Run("alias followed", func(t *testing.T) {
		for _, targetStatus := range []string{catalogDomain.GenreStatusPublished, catalogDomain.GenreStatusRejected} {
			svc, repo, src := newGenreImportService()
			target := qidGenre("фэнтези", "fentezi", targetStatus, "")
			q := "Q5"
			alias := aliasOf(target.ID, "фентези", "fentezi-2", &q)
			repo.genres = append(repo.genres, target, alias)

			res, err := svc.ImportWikidataGenre(ctx, "Q5")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			wantOutcome := GenreOutcomeExisting
			if targetStatus == catalogDomain.GenreStatusRejected {
				wantOutcome = GenreOutcomeRejected
			}
			if res.Outcome != wantOutcome || res.Match != GenreMatchQID || res.Genre.ID != target.ID ||
				res.ViaAlias == nil || res.ViaAlias.ID != alias.ID {
				t.Fatalf("%s: unexpected result %+v", targetStatus, res)
			}
			if len(src.fetchCalls) != 0 {
				t.Fatalf("no fetch expected")
			}
		}
	})

	t.Run("nil source still short-circuits", func(t *testing.T) {
		svc, repo := newService()
		repo.genres = append(repo.genres, qidGenre("роман", "roman", catalogDomain.GenreStatusPublished, "Q8261"))
		res, err := svc.ImportWikidataGenre(ctx, "Q8261")
		if err != nil || res.Outcome != GenreOutcomeExisting {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})

	t.Run("nil source without a match", func(t *testing.T) {
		svc, _ := newService()
		if _, err := svc.ImportWikidataGenre(ctx, "Q8261"); !errors.Is(err, ErrGenreImportUnavailable) {
			t.Fatalf("err = %v, want ErrGenreImportUnavailable", err)
		}
	})
}

func TestImportWikidataGenre_InvalidQID(t *testing.T) {
	svc, _, src := newGenreImportService()
	for _, qid := range []string{"", "  ", "роман", "Q", "P31", "Q1 Q2"} {
		if _, err := svc.ImportWikidataGenre(context.Background(), qid); !errors.Is(err, ErrInvalidQID) {
			t.Errorf("%q: err = %v, want ErrInvalidQID", qid, err)
		}
	}
	if len(src.fetchCalls) != 0 {
		t.Fatalf("no fetch expected, got %v", src.fetchCalls)
	}
}

func TestImportWikidataGenre_Redirect(t *testing.T) {
	ctx := context.Background()

	t.Run("canonical QID already in BookHaven", func(t *testing.T) {
		svc, repo, src := newGenreImportService()
		existing := qidGenre("роман", "roman", catalogDomain.GenreStatusPublished, "Q200")
		repo.genres = append(repo.genres, existing)
		src.records["Q100"] = literaryRecord("Q200", "роман")

		res, err := svc.ImportWikidataGenre(ctx, "Q100")
		if err != nil || res.Outcome != GenreOutcomeExisting || res.Match != GenreMatchQID || res.Genre.ID != existing.ID {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})

	t.Run("created with the canonical QID", func(t *testing.T) {
		svc, repo, src := newGenreImportService()
		src.records["Q100"] = literaryRecord("Q200", "роман")

		res, err := svc.ImportWikidataGenre(ctx, "Q100")
		if err != nil || res.Outcome != GenreOutcomeCreated {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if got := repo.genres[0].WikidataQID; got == nil || *got != "Q200" {
			t.Fatalf("stored QID = %v, want Q200", got)
		}
	})
}

func TestImportWikidataGenre_FetchErrors(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		err  error
		want error
	}{
		{bookdata.ErrNotFound, ErrWikidataEntityNotFound},
		{bookdata.ErrInvalidQuery, ErrInvalidQID},
		{bookdata.ErrRateLimited, bookdata.ErrRateLimited},
		{bookdata.ErrSourceUnavailable, bookdata.ErrSourceUnavailable},
	}
	for _, tc := range cases {
		svc, _, src := newGenreImportService()
		src.fetchErr = tc.err
		if _, err := svc.ImportWikidataGenre(ctx, "Q1"); !errors.Is(err, tc.want) {
			t.Errorf("%v: err = %v, want %v", tc.err, err, tc.want)
		}
	}
}

func TestImportWikidataGenre_NoRussianLabel(t *testing.T) {
	for _, name := range []string{"", "   ", "Fantasy"} {
		svc, repo, src := newGenreImportService()
		src.records["Q1"] = literaryRecord("Q1", name)
		if _, err := svc.ImportWikidataGenre(context.Background(), "Q1"); !errors.Is(err, ErrNoRussianLabel) {
			t.Errorf("%q: err = %v, want ErrNoRussianLabel", name, err)
		}
		if len(repo.genres) != 0 || len(src.classifyCalls) != 0 {
			t.Errorf("%q: nothing must be created or classified", name)
		}
	}
}

func TestImportWikidataGenre_ClassCheck(t *testing.T) {
	ctx := context.Background()

	t.Run("local hit skips the query", func(t *testing.T) {
		for _, rec := range []*bookdata.ExternalGenreRecord{
			{QID: "Q1", Name: "роман", InstanceOf: []string{"Q223393"}},
			{QID: "Q1", Name: "роман", SubclassOf: []string{"Q4263830"}},
		} {
			svc, _, src := newGenreImportService()
			src.records["Q1"] = rec
			res, err := svc.ImportWikidataGenre(ctx, "Q1")
			if err != nil || res.Outcome != GenreOutcomeCreated {
				t.Fatalf("res=%+v err=%v", res, err)
			}
			if len(src.classifyCalls) != 0 {
				t.Fatalf("no classification expected, got %v", src.classifyCalls)
			}
		}
	})

	t.Run("local miss, classified literary", func(t *testing.T) {
		svc, repo, src := newGenreImportService()
		src.records["Q132311"] = &bookdata.ExternalGenreRecord{QID: "Q132311", Name: "Фэнтези", InstanceOf: []string{"Q20076756"}}
		src.classes["Q132311"] = bookdata.GenreClassification{RuLabel: "фэнтези", IsLiteraryGenre: true}

		res, err := svc.ImportWikidataGenre(ctx, "Q132311")
		if err != nil || res.Outcome != GenreOutcomeCreated || res.Match != "" {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if len(src.classifyCalls) != 1 || src.classifyCalls[0][0] != "Q132311" {
			t.Fatalf("expected one classification of Q132311, got %v", src.classifyCalls)
		}
		g := repo.genres[0]
		if g.Status != catalogDomain.GenreStatusPending || g.WikidataQID == nil || *g.WikidataQID != "Q132311" || g.Name != "фэнтези" {
			t.Fatalf("created genre = %+v", g)
		}
	})

	t.Run("classified not literary", func(t *testing.T) {
		svc, repo, src := newGenreImportService()
		src.records["Q11399"] = &bookdata.ExternalGenreRecord{QID: "Q11399", Name: "рок-музыка", InstanceOf: []string{"Q188451"}}
		src.classes["Q11399"] = bookdata.GenreClassification{RuLabel: "рок-музыка", IsLiteraryGenre: false}
		if _, err := svc.ImportWikidataGenre(ctx, "Q11399"); !errors.Is(err, ErrNotALiteraryGenre) {
			t.Fatalf("err = %v, want ErrNotALiteraryGenre", err)
		}
		if len(repo.genres) != 0 {
			t.Fatalf("nothing must be created")
		}
	})

	t.Run("classification failure is a source error", func(t *testing.T) {
		for name, setup := range map[string]func(*fakeGenreSource){
			"unavailable": func(s *fakeGenreSource) { s.classifyErr = bookdata.ErrSourceUnavailable },
			"rate limit":  func(s *fakeGenreSource) { s.classifyErr = bookdata.ErrRateLimited },
			"parse":       func(s *fakeGenreSource) { s.classifyErr = errors.New("bookdata: parse wikidata response") },
			"deadline":    func(s *fakeGenreSource) { s.classifyBlock = true },
		} {
			svc, repo, src := newGenreImportService()
			svc.genreClassifyTimeout = 10 * time.Millisecond
			src.records["Q1"] = &bookdata.ExternalGenreRecord{QID: "Q1", Name: "фэнтези"}
			setup(src)
			_, err := svc.ImportWikidataGenre(ctx, "Q1")
			if !errors.Is(err, bookdata.ErrSourceUnavailable) && !errors.Is(err, bookdata.ErrRateLimited) {
				t.Errorf("%s: err = %v, want a bookdata source error", name, err)
			}
			if len(repo.genres) != 0 {
				t.Errorf("%s: nothing must be created", name)
			}
		}
	})
}

func TestImportWikidataGenre_SlugResolution(t *testing.T) {
	ctx := context.Background()
	setup := func(existing catalogDomain.Genre) (*Service, *fakeRepository) {
		svc, repo, src := newGenreImportService()
		repo.genres = append(repo.genres, existing)
		src.records["Q7"] = literaryRecord("Q7", "Роман")
		return svc, repo
	}

	t.Run("without QID: backfilled", func(t *testing.T) {
		existing := qidGenre("роман", slugify("Роман"), catalogDomain.GenreStatusPublished, "")
		svc, repo := setup(existing)
		res, err := svc.ImportWikidataGenre(ctx, "Q7")
		if err != nil || res.Outcome != GenreOutcomeExisting || res.Match != GenreMatchSlug || !res.QIDBackfilled || res.QIDMismatch != nil {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		g := repo.genres[0]
		if g.WikidataQID == nil || *g.WikidataQID != "Q7" || g.Status != catalogDomain.GenreStatusPublished || len(repo.genres) != 1 {
			t.Fatalf("genre = %+v (count %d)", g, len(repo.genres))
		}
	})

	t.Run("rejected: backfilled tombstone", func(t *testing.T) {
		existing := qidGenre("роман", slugify("Роман"), catalogDomain.GenreStatusRejected, "")
		svc, repo := setup(existing)
		res, err := svc.ImportWikidataGenre(ctx, "Q7")
		if err != nil || res.Outcome != GenreOutcomeRejected || res.Match != GenreMatchSlug || !res.QIDBackfilled {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if g := repo.genres[0]; g.WikidataQID == nil || *g.WikidataQID != "Q7" || g.Status != catalogDomain.GenreStatusRejected {
			t.Fatalf("genre = %+v", g)
		}
	})

	t.Run("different QID: untouched with qid_mismatch", func(t *testing.T) {
		existing := qidGenre("роман", slugify("Роман"), catalogDomain.GenreStatusPublished, "Q99")
		svc, repo := setup(existing)
		res, err := svc.ImportWikidataGenre(ctx, "Q7")
		if err != nil || res.Outcome != GenreOutcomeExisting || res.Match != GenreMatchSlug || res.QIDBackfilled ||
			res.QIDMismatch == nil || *res.QIDMismatch != "Q99" {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if repo.setGenreQIDCalls != 0 || len(repo.genres) != 1 || *repo.genres[0].WikidataQID != "Q99" {
			t.Fatalf("nothing must be written")
		}
	})

	t.Run("slug alias: canonical, no backfill", func(t *testing.T) {
		target := qidGenre("роман", "roman-target", catalogDomain.GenreStatusPublished, "")
		alias := aliasOf(target.ID, "роман", slugify("Роман"), nil)
		svc, repo, src := newGenreImportService()
		repo.genres = append(repo.genres, target, alias)
		src.records["Q7"] = literaryRecord("Q7", "Роман")

		res, err := svc.ImportWikidataGenre(ctx, "Q7")
		if err != nil || res.Outcome != GenreOutcomeExisting || res.Match != GenreMatchSlug || res.Genre.ID != target.ID ||
			res.ViaAlias == nil || res.ViaAlias.ID != alias.ID || res.QIDBackfilled || res.QIDMismatch != nil {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if repo.setGenreQIDCalls != 0 {
			t.Fatalf("no backfill expected")
		}
	})
}

// duplicateOnCreateRepo fails CreateGenre with the queued errors (one per
// call), running onErr before each to simulate a concurrent writer.
type duplicateOnCreateRepo struct {
	*fakeRepository
	errs  []error
	onErr func(f *fakeRepository)
}

func (r *duplicateOnCreateRepo) CreateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	if len(r.errs) > 0 {
		err := r.errs[0]
		r.errs = r.errs[1:]
		if r.onErr != nil {
			r.onErr(r.fakeRepository)
		}
		return err
	}
	return r.fakeRepository.CreateGenre(ctx, genre)
}

func TestImportWikidataGenre_DuplicateRetry(t *testing.T) {
	ctx := context.Background()
	dup := &pgconn.PgError{Code: "23505", ConstraintName: "idx_genres_slug"}

	t.Run("retry finds the concurrently created genre", func(t *testing.T) {
		repo := &fakeRepository{authorsExist: true, genresExist: true}
		var concurrent catalogDomain.Genre
		wrapped := &duplicateOnCreateRepo{fakeRepository: repo, errs: []error{dup}, onErr: func(f *fakeRepository) {
			concurrent = qidGenre("роман", slugify("Роман"), catalogDomain.GenreStatusPending, "Q7")
			f.genres = append(f.genres, concurrent)
		}}
		src := &fakeGenreSource{records: map[string]*bookdata.ExternalGenreRecord{"Q7": literaryRecord("Q7", "Роман")}}
		svc := NewService(wrapped, WithGenreWikidataSource(src))

		res, err := svc.ImportWikidataGenre(ctx, "Q7")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Outcome != GenreOutcomeExisting || res.Match != GenreMatchQID || res.Genre.ID != concurrent.ID {
			t.Fatalf("unexpected result %+v", res)
		}
		if len(src.fetchCalls) != 1 {
			t.Fatalf("the retry must not call Wikidata again, got %v", src.fetchCalls)
		}
	})

	t.Run("second duplicate fails", func(t *testing.T) {
		repo := &fakeRepository{authorsExist: true, genresExist: true}
		wrapped := &duplicateOnCreateRepo{fakeRepository: repo, errs: []error{dup, dup}}
		src := &fakeGenreSource{records: map[string]*bookdata.ExternalGenreRecord{"Q7": literaryRecord("Q7", "Роман")}}
		svc := NewService(wrapped, WithGenreWikidataSource(src))

		if _, err := svc.ImportWikidataGenre(ctx, "Q7"); !errors.Is(err, ErrDuplicateSlug) {
			t.Fatalf("err = %v, want ErrDuplicateSlug", err)
		}
	})
}
