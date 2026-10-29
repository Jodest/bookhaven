package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/bookhaven/backend/internal/pkg/bookdata"
)

const fantLabCover = "https://fantlab.ru/images/editions/big/77"

// fakeFantLab records its calls in the shared order slice of the enrich fake.
type fakeFantLab struct {
	enrich *fakeEnrichClient
	cover  string
	err    error
	calls  [][]string
	titles []string
	budget int
}

func (f *fakeFantLab) FindSelfTitledEditionCover(ctx context.Context, title string, workIDs []string) (string, string, error) {
	f.enrich.order = append(f.enrich.order, "fantlab")
	f.calls = append(f.calls, workIDs)
	f.titles = append(f.titles, title)
	if f.err != nil {
		return "", "", f.err
	}
	if f.cover == "" {
		return "", "", bookdata.ErrNotFound
	}
	return f.cover, "5/77", nil
}

func (f *fakeFantLab) SetRequestBudget(n int) { f.budget = n }

func fantLabRecord(sourceID string) bookdata.ExternalBookRecord {
	return bookdata.ExternalBookRecord{
		Title:          "Хоббит",
		Language:       "ru",
		SourceName:     "wikidata",
		SourceID:       sourceID,
		FantLabWorkIDs: []string{"5"},
	}
}

func processWithFantLab(t *testing.T, rec bookdata.ExternalBookRecord, enrich *fakeEnrichClient, fl FantLabCoverFinder) *fakeAdminRepository {
	t.Helper()
	admin, repo := newTestAdmin()
	if _, _, _, err := processHarvestRecord(context.Background(), rec, enrich, fl, admin); err != nil {
		t.Fatalf("err: %v", err)
	}
	return repo
}

func TestProcessRecord_FantLabCoverBlocksTitleCoverButDescriptionMerges(t *testing.T) {
	enrich := &fakeEnrichClient{
		byTitleAuthor: map[string]*bookdata.ExternalBookRecord{
			"Хоббит": {CoverURL: strPtr(titleCover), Description: strPtr("Повесть о хоббите Бильбо Бэггинсе и его путешествии")},
		},
	}
	fl := &fakeFantLab{enrich: enrich, cover: fantLabCover}
	repo := processWithFantLab(t, fantLabRecord("Q20"), enrich, fl)
	if c := repo.books[0].CoverURL; c == nil || *c != fantLabCover {
		t.Errorf("CoverURL = %v, want the FantLab cover", c)
	}
	if repo.books[0].Description == nil {
		t.Error("title search must still fill the description")
	}
	if want := []string{"fantlab", "title"}; !reflect.DeepEqual(enrich.order, want) {
		t.Errorf("call order = %v, want %v", enrich.order, want)
	}
	if fl.titles[0] != "Хоббит" || !reflect.DeepEqual(fl.calls[0], []string{"5"}) {
		t.Errorf("FantLab call = %v %v", fl.titles, fl.calls)
	}
}

func TestProcessRecord_OpenLibraryIDCoverSkipsFantLab(t *testing.T) {
	enrich := &fakeEnrichClient{olidCover: idCover}
	fl := &fakeFantLab{enrich: enrich, cover: fantLabCover}
	rec := fantLabRecord("Q21")
	rec.OpenLibraryIDs = []string{"OL27479W"}
	repo := processWithFantLab(t, rec, enrich, fl)
	if c := repo.books[0].CoverURL; c == nil || *c != idCover {
		t.Errorf("CoverURL = %v, want the ID cover", c)
	}
	if len(fl.calls) != 0 {
		t.Errorf("FantLab called: %v", fl.calls)
	}
}

func TestProcessRecord_FantLabOrderAfterISBNAndIDs(t *testing.T) {
	enrich := &fakeEnrichClient{
		byTitleAuthor: map[string]*bookdata.ExternalBookRecord{"Хоббит": {CoverURL: strPtr(titleCover)}},
	}
	fl := &fakeFantLab{enrich: enrich}
	rec := fantLabRecord("Q22")
	rec.OpenLibraryIDs = []string{"OL27479W"}
	repo := processWithFantLab(t, rec, enrich, fl)
	if c := repo.books[0].CoverURL; c == nil || *c != titleCover {
		t.Errorf("CoverURL = %v, want the title-search cover", c)
	}
	if want := []string{"ids", "fantlab", "title"}; !reflect.DeepEqual(enrich.order, want) {
		t.Errorf("call order = %v, want %v", enrich.order, want)
	}
}

func TestProcessRecord_FantLabErrorFallsThroughToTitleSearch(t *testing.T) {
	for name, ferr := range map[string]error{"source": bookdata.ErrSourceUnavailable, "budget": bookdata.ErrBudgetExhausted} {
		t.Run(name, func(t *testing.T) {
			enrich := &fakeEnrichClient{
				byTitleAuthor: map[string]*bookdata.ExternalBookRecord{"Хоббит": {CoverURL: strPtr(titleCover)}},
			}
			fl := &fakeFantLab{enrich: enrich, err: ferr}
			repo := processWithFantLab(t, fantLabRecord("Q23"), enrich, fl)
			if c := repo.books[0].CoverURL; c == nil || *c != titleCover {
				t.Errorf("CoverURL = %v, want the title-search cover", c)
			}
		})
	}
}

func TestProcessRecord_FantLabSkippedWithoutIDsOrFinder(t *testing.T) {
	enrich := &fakeEnrichClient{}
	fl := &fakeFantLab{enrich: enrich, cover: fantLabCover}
	rec := fantLabRecord("Q24")
	rec.FantLabWorkIDs = nil
	repo := processWithFantLab(t, rec, enrich, fl)
	if len(fl.calls) != 0 || repo.books[0].CoverURL != nil {
		t.Errorf("no IDs: calls=%v cover=%v", fl.calls, repo.books[0].CoverURL)
	}
	// nil finder: the step is off
	repo = processWithFantLab(t, fantLabRecord("Q25"), &fakeEnrichClient{}, nil)
	if repo.books[0].CoverURL != nil {
		t.Errorf("nil finder: cover = %v", repo.books[0].CoverURL)
	}
}

func TestRunHarvest_ResetsFantLabBudgetPerJob(t *testing.T) {
	admin, _ := newTestAdmin()
	enrich := &fakeEnrichClient{}
	fl := &fakeFantLab{enrich: enrich, err: bookdata.ErrBudgetExhausted}
	h := NewHandler(&fakeWikidataQuerier{records: []bookdata.ExternalBookRecord{fantLabRecord("Q30")}}, enrich, admin, WithFantLabCovers(fl))
	if _, err := h.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil); err != nil {
		t.Fatal(err)
	}
	if fl.budget != MaxFantLabRequestsPerJob || MaxFantLabRequestsPerJob != 250 {
		t.Errorf("budget = %d, want 250", fl.budget)
	}
	if len(fl.calls) != 1 {
		t.Errorf("FantLab calls = %d, want 1", len(fl.calls))
	}
}
