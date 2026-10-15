package service

import (
	"context"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

type fakeFilmSyncer struct {
	hasSource bool
	failed    int
	calls     [][]uuid.UUID
}

func (f *fakeFilmSyncer) SyncCreatedAuthors(_ context.Context, ids []uuid.UUID) (int, int) {
	f.calls = append(f.calls, ids)
	return 2, f.failed
}

func (f *fakeFilmSyncer) HasAuthorSyncSource() bool { return f.hasSource }

func TestImportWikidataAuthor_CreatedSyncsFilms(t *testing.T) {
	svc, _, _ := newImportEnrichService(tolstoyRecord(), true)
	syncer := &fakeFilmSyncer{hasSource: true}
	WithAuthorFilmSyncer(syncer)(svc)

	res, err := svc.ImportWikidataAuthor(context.Background(), "Q7243")
	if err != nil || res.Outcome != AuthorOutcomeCreated {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(syncer.calls) != 1 || len(syncer.calls[0]) != 1 || syncer.calls[0][0] != res.Author.ID {
		t.Fatalf("calls = %v", syncer.calls)
	}
}

func TestImportWikidataAuthor_FilmSyncFailureKeepsCreated(t *testing.T) {
	svc, _, _ := newImportEnrichService(tolstoyRecord(), true)
	syncer := &fakeFilmSyncer{hasSource: true, failed: 1}
	WithAuthorFilmSyncer(syncer)(svc)

	res, err := svc.ImportWikidataAuthor(context.Background(), "Q7243")
	if err != nil || res.Outcome != AuthorOutcomeCreated || res.Enrichment == nil {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(syncer.calls) != 1 {
		t.Fatalf("calls = %v", syncer.calls)
	}
}

func TestImportWikidataAuthor_ExistingDoesNotSyncFilms(t *testing.T) {
	svc, repo, _ := newImportEnrichService(tolstoyRecord(), true)
	repo.authors = append(repo.authors, wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPublished, "Q7243"))
	syncer := &fakeFilmSyncer{hasSource: true}
	WithAuthorFilmSyncer(syncer)(svc)

	res, err := svc.ImportWikidataAuthor(context.Background(), "Q7243")
	if err != nil || res.Outcome != AuthorOutcomeExisting {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(syncer.calls) != 0 {
		t.Fatalf("existing triggered a sync: %v", syncer.calls)
	}
}

func TestImportWikidataAuthor_NoSyncerOrSourceIsNoop(t *testing.T) {
	svc, _, _ := newImportEnrichService(tolstoyRecord(), true)
	if res, err := svc.ImportWikidataAuthor(context.Background(), "Q7243"); err != nil || res.Outcome != AuthorOutcomeCreated {
		t.Fatalf("nil syncer: res=%+v err=%v", res, err)
	}
	svc2, _, _ := newImportEnrichService(tolstoyRecord(), true)
	syncer := &fakeFilmSyncer{hasSource: false}
	WithAuthorFilmSyncer(syncer)(svc2)
	if _, err := svc2.ImportWikidataAuthor(context.Background(), "Q7243"); err != nil {
		t.Fatal(err)
	}
	if len(syncer.calls) != 0 {
		t.Fatalf("unconfigured source triggered a sync: %v", syncer.calls)
	}
}
