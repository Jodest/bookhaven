package service

import (
	"context"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
)

type fakeSyncer struct {
	hasSource bool
	added     int
	failed    int
	calls     [][]uuid.UUID
}

func (f *fakeSyncer) HasSyncSource() bool { return f.hasSource }

func (f *fakeSyncer) SyncCreatedBooks(_ context.Context, ids []uuid.UUID) (int, int) {
	f.calls = append(f.calls, append([]uuid.UUID(nil), ids...))
	return f.added, f.failed
}

func adaptationRecords() []bookdata.ExternalBookRecord {
	return []bookdata.ExternalBookRecord{
		harvestRecord("Q1", "Чайка", nil),
		harvestRecord("Q2", "Вишнёвый сад", nil),
	}
}

func TestRunHarvest_SyncsAdaptationsOfCreatedBooks(t *testing.T) {
	admin, repo := newTestAdmin()
	source := "wikidata"
	existingID := "Q2"
	repo.books = append(repo.books, catalogDomain.Book{
		ID: uuid.New(), Title: "Вишнёвый сад", Slug: "vishnevyj-sad", ExternalSource: &source, ExternalID: &existingID,
	})
	syncer := &fakeSyncer{hasSource: true, added: 3, failed: 1}
	progress := &fakeProgressWriter{}
	handler := NewHandler(&fakeWikidataQuerier{records: adaptationRecords()}, nil, admin, WithAdaptationSyncer(syncer))

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, progress)
	if err != nil {
		t.Fatal(err)
	}
	if result.Created != 1 || result.Skipped != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(syncer.calls) != 1 || len(syncer.calls[0]) != 1 {
		t.Fatalf("only the created book must be synced, calls = %v", syncer.calls)
	}
	created := repo.books[len(repo.books)-1]
	if syncer.calls[0][0] != created.ID {
		t.Fatalf("synced %v, created %v", syncer.calls[0][0], created.ID)
	}
	if result.AdaptationsAdded == nil || *result.AdaptationsAdded != 3 || result.AdaptationSyncErrors == nil || *result.AdaptationSyncErrors != 1 {
		t.Fatalf("counters = %v / %v", result.AdaptationsAdded, result.AdaptationSyncErrors)
	}
	last := progress.writes[len(progress.writes)-1]
	if last.AdaptationsAdded == nil || *last.AdaptationsAdded != 3 {
		t.Fatalf("final progress = %+v", last)
	}
}

func TestRunHarvest_AdaptationSyncFailureDoesNotFailJob(t *testing.T) {
	admin, _ := newTestAdmin()
	syncer := &fakeSyncer{hasSource: true, failed: 2}
	handler := NewHandler(&fakeWikidataQuerier{records: adaptationRecords()}, nil, admin, WithAdaptationSyncer(syncer))

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil)
	if err != nil {
		t.Fatalf("job must not fail: %v", err)
	}
	if result.Created != 2 || *result.AdaptationsAdded != 0 || *result.AdaptationSyncErrors != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunHarvest_NoSyncerOrSourceLeavesCountersNil(t *testing.T) {
	admin, _ := newTestAdmin()
	for name, h := range map[string]*Handler{
		"nil syncer": NewHandler(&fakeWikidataQuerier{records: adaptationRecords()}, nil, admin),
		"no source":  NewHandler(&fakeWikidataQuerier{records: adaptationRecords()}, nil, admin, WithAdaptationSyncer(&fakeSyncer{})),
	} {
		result, err := h.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.AdaptationsAdded != nil || result.AdaptationSyncErrors != nil {
			t.Errorf("%s: counters must be nil: %+v", name, result)
		}
	}
}
