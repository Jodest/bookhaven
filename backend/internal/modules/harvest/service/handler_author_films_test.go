package service

import (
	"context"
	"testing"

	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
)

type fakeFilmSyncer struct {
	hasSource bool
	added     int
	failed    int
	calls     [][]uuid.UUID
}

func (f *fakeFilmSyncer) HasAuthorSyncSource() bool { return f.hasSource }

func (f *fakeFilmSyncer) SyncCreatedAuthors(_ context.Context, ids []uuid.UUID) (int, int) {
	f.calls = append(f.calls, append([]uuid.UUID(nil), ids...))
	return f.added, f.failed
}

func authorRecords() []bookdata.ExternalBookRecord {
	return []bookdata.ExternalBookRecord{
		harvestRecord("Q1", "Чайка", nil, bookdata.ExternalAuthor{Name: "Антон Чехов", QID: "Q5685"}),
		harvestRecord("Q2", "Вишнёвый сад", nil, bookdata.ExternalAuthor{Name: "Антон Чехов", QID: "Q5685"}),
	}
}

func TestRunHarvest_SyncsFilmsOfCreatedAuthors(t *testing.T) {
	admin, _ := newTestAdmin()
	syncer := &fakeFilmSyncer{hasSource: true, added: 4, failed: 1}
	progress := &fakeProgressWriter{}
	handler := NewHandler(&fakeWikidataQuerier{records: authorRecords()}, nil, admin, WithAuthorFilmSyncer(syncer))

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, progress)
	if err != nil {
		t.Fatal(err)
	}
	if len(syncer.calls) != 1 || len(syncer.calls[0]) != 1 {
		t.Fatalf("the created author must be synced once, calls = %v", syncer.calls)
	}
	if result.AuthorFilmsAdded == nil || *result.AuthorFilmsAdded != 4 || result.AuthorFilmSyncErrors == nil || *result.AuthorFilmSyncErrors != 1 {
		t.Fatalf("counters = %v / %v", result.AuthorFilmsAdded, result.AuthorFilmSyncErrors)
	}
	if last := progress.writes[len(progress.writes)-1]; last.AuthorFilmsAdded == nil || *last.AuthorFilmsAdded != 4 {
		t.Fatalf("final progress = %+v", last)
	}
}

func TestRunHarvest_AuthorFilmSyncFailureDoesNotFailJob(t *testing.T) {
	admin, _ := newTestAdmin()
	syncer := &fakeFilmSyncer{hasSource: true, failed: 1}
	handler := NewHandler(&fakeWikidataQuerier{records: authorRecords()}, nil, admin, WithAuthorFilmSyncer(syncer))

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil)
	if err != nil {
		t.Fatalf("job must not fail: %v", err)
	}
	if result.Created != 2 || *result.AuthorFilmsAdded != 0 || *result.AuthorFilmSyncErrors != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunHarvest_NoCreatedAuthorsSkipsFilmSyncCall(t *testing.T) {
	admin, _ := newTestAdmin()
	syncer := &fakeFilmSyncer{hasSource: true, added: 9}
	handler := NewHandler(&fakeWikidataQuerier{records: adaptationRecords()}, nil, admin, WithAuthorFilmSyncer(syncer))

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(syncer.calls) != 0 || result.AuthorFilmsAdded == nil || *result.AuthorFilmsAdded != 0 || *result.AuthorFilmSyncErrors != 0 {
		t.Fatalf("calls=%v result=%+v", syncer.calls, result)
	}
}

func TestRunHarvest_NoFilmSyncerOrSourceLeavesCountersNil(t *testing.T) {
	admin, _ := newTestAdmin()
	for name, h := range map[string]*Handler{
		"nil syncer": NewHandler(&fakeWikidataQuerier{records: authorRecords()}, nil, admin),
		"no source":  NewHandler(&fakeWikidataQuerier{records: authorRecords()}, nil, admin, WithAuthorFilmSyncer(&fakeFilmSyncer{})),
	} {
		result, err := h.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.AuthorFilmsAdded != nil || result.AuthorFilmSyncErrors != nil {
			t.Errorf("%s: counters must be nil: %+v", name, result)
		}
	}
}
