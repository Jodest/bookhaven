package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
)

// fakeEnrichmentSource implements adminservice.AuthorEnrichmentSource.
type fakeEnrichmentSource struct {
	records    map[string]*bookdata.ExternalAuthorRecord
	fetchErr   error
	fetchCalls [][]string
}

func (f *fakeEnrichmentSource) FetchAuthors(ctx context.Context, qids []string) (map[string]*bookdata.ExternalAuthorRecord, error) {
	f.fetchCalls = append(f.fetchCalls, append([]string(nil), qids...))
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	out := map[string]*bookdata.ExternalAuthorRecord{}
	for _, q := range qids {
		if r, ok := f.records[q]; ok {
			out[q] = r
		}
	}
	return out, nil
}

func (f *fakeEnrichmentSource) FetchCommonsImages(ctx context.Context, files []string) (map[string]*bookdata.CommonsImage, error) {
	return map[string]*bookdata.CommonsImage{}, nil
}

func newEnrichingAdmin(src *fakeEnrichmentSource) (*adminservice.Service, *fakeAdminRepository) {
	repo := &fakeAdminRepository{}
	return adminservice.NewService(repo, adminservice.WithAuthorEnrichmentSource(src)), repo
}

func birthRecord(qid string) *bookdata.ExternalAuthorRecord {
	d := time.Date(1860, 1, 29, 0, 0, 0, 0, time.UTC)
	return &bookdata.ExternalAuthorRecord{QID: qid, BirthDate: &d}
}

func harvestRecord(sourceID, title string, isbn *string, authors ...bookdata.ExternalAuthor) bookdata.ExternalBookRecord {
	names := make([]string, 0, len(authors))
	for _, a := range authors {
		names = append(names, a.Name)
	}
	return bookdata.ExternalBookRecord{
		Title: title, Language: "ru", SourceName: "wikidata", SourceID: sourceID,
		ISBN: isbn, Authors: names, AuthorRefs: authors,
	}
}

func TestRunHarvest_EnrichesCreatedAuthorsOnceAfterLoop(t *testing.T) {
	src := &fakeEnrichmentSource{records: map[string]*bookdata.ExternalAuthorRecord{
		"Q5685": birthRecord("Q5685"),
		"Q7243": birthRecord("Q7243"),
	}}
	admin, repo := newEnrichingAdmin(src)

	records := []bookdata.ExternalBookRecord{
		harvestRecord("Q1", "Палата номер шесть", nil, bookdata.ExternalAuthor{Name: "Антон Чехов", QID: "Q5685"}),
		harvestRecord("Q2", "Чайка", nil, bookdata.ExternalAuthor{Name: "Антон Чехов", QID: "Q5685"}),
		// The book creation fails (invalid ISBN), but its new author is
		// still enriched.
		harvestRecord("Q3", "Война и мир", strPtr("not-a-valid-isbn"), bookdata.ExternalAuthor{Name: "Лев Толстой", QID: "Q7243"}),
		// A QID-less author is created but not enriched.
		harvestRecord("Q4", "Безымянная", nil, bookdata.ExternalAuthor{Name: "Неизвестный Автор"}),
	}
	progress := &fakeProgressWriter{}
	handler := NewHandler(&fakeWikidataQuerier{records: records}, nil, admin)

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, progress)
	if err != nil {
		t.Fatalf("runHarvest: %v", err)
	}
	if len(src.fetchCalls) != 1 || len(src.fetchCalls[0]) != 2 {
		t.Fatalf("expected one batched fetch of 2 QIDs, got %v", src.fetchCalls)
	}
	if result.AuthorsEnriched == nil || *result.AuthorsEnriched != 2 || result.AuthorEnrichmentErrors == nil || *result.AuthorEnrichmentErrors != 0 {
		t.Fatalf("counters = %v / %v", result.AuthorsEnriched, result.AuthorEnrichmentErrors)
	}
	for _, a := range repo.authors {
		if a.ExternalID != nil && a.BirthDate == nil {
			t.Fatalf("author %q not enriched", a.Name)
		}
	}

	// The final progress write carries the counters.
	last := progress.writes[len(progress.writes)-1]
	if last.AuthorsEnriched == nil || *last.AuthorsEnriched != 2 || last.AuthorEnrichmentErrors == nil || *last.AuthorEnrichmentErrors != 0 {
		t.Fatalf("final progress = %+v", last)
	}
}

func TestRunHarvest_EnrichmentFailureDoesNotFailJob(t *testing.T) {
	src := &fakeEnrichmentSource{fetchErr: bookdata.ErrSourceUnavailable}
	admin, _ := newEnrichingAdmin(src)
	records := []bookdata.ExternalBookRecord{
		harvestRecord("Q1", "Чайка", nil, bookdata.ExternalAuthor{Name: "Антон Чехов", QID: "Q5685"}),
	}
	handler := NewHandler(&fakeWikidataQuerier{records: records}, nil, admin)

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil)
	if err != nil {
		t.Fatalf("runHarvest must not fail: %v", err)
	}
	if result.Created != 1 || *result.AuthorsEnriched != 0 || *result.AuthorEnrichmentErrors != 1 {
		t.Fatalf("result = %+v enriched=%d errors=%d", result, *result.AuthorsEnriched, *result.AuthorEnrichmentErrors)
	}
}

func TestRunHarvest_NoEnrichmentSourceSkipsStep(t *testing.T) {
	admin, _ := newTestAdmin()
	records := []bookdata.ExternalBookRecord{
		harvestRecord("Q1", "Чайка", nil, bookdata.ExternalAuthor{Name: "Антон Чехов", QID: "Q5685"}),
	}
	handler := NewHandler(&fakeWikidataQuerier{records: records}, nil, admin)

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil)
	if err != nil {
		t.Fatalf("runHarvest: %v", err)
	}
	if result.AuthorsEnriched != nil || result.AuthorEnrichmentErrors != nil {
		t.Fatalf("counters must be absent without a source: %+v", result)
	}
	data, _ := json.Marshal(result)
	var decoded map[string]interface{}
	_ = json.Unmarshal(data, &decoded)
	if _, ok := decoded["authors_enriched"]; ok {
		t.Fatalf("authors_enriched must be omitted: %s", data)
	}
}
