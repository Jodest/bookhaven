package service

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

func (f *fakeRepository) ListAutoHarvestCandidates(ctx context.Context, limit int, notAfter time.Time) ([]catalogDomain.Author, error) {
	f.autoHarvestLimit = limit
	f.autoHarvestNotAfter = notAfter
	if f.autoHarvestErr != nil {
		return nil, f.autoHarvestErr
	}
	return f.autoHarvestCandidates, nil
}

func (f *fakeRepository) MarkAuthorsAutoHarvested(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	if f.markAutoHarvestErr != nil {
		return f.markAutoHarvestErr
	}
	f.markedAutoHarvested = append(f.markedAutoHarvested, ids...)
	f.markedAutoHarvestedAt = at
	return nil
}

func (f *fakeImportRepository) ListAutoHarvestCandidates(ctx context.Context, limit int, notAfter time.Time) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeImportRepository) MarkAuthorsAutoHarvested(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	return nil
}

func (f *fakeMergeAuthorsRepository) ListAutoHarvestCandidates(ctx context.Context, limit int, notAfter time.Time) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) MarkAuthorsAutoHarvested(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	return nil
}

func boolPtr(b bool) *bool { return &b }

func qidAuthor(id uuid.UUID, qid string) catalogDomain.Author {
	return catalogDomain.Author{
		ID: id, Name: "Leo Tolstoy", Slug: "leo-tolstoy",
		Status:         catalogDomain.AuthorStatusPublished,
		ExternalSource: strPtr(catalogDomain.AuthorExternalSourceWikidata),
		ExternalID:     strPtr(qid),
	}
}

func TestUpdateAuthor_AutoHarvest(t *testing.T) {
	stamp := time.Date(2026, 9, 1, 4, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		withQID bool
		initial bool
		input   *bool
		want    bool
		wantErr error
	}{
		{name: "set on author with QID", withQID: true, input: boolPtr(true), want: true},
		{name: "omitted keeps flag", withQID: true, initial: true, input: nil, want: true},
		{name: "clear is allowed", withQID: true, initial: true, input: boolPtr(false), want: false},
		{name: "clear allowed without QID", initial: true, input: boolPtr(false), want: false},
		{name: "set without QID rejected", input: boolPtr(true), wantErr: ErrAutoHarvestRequiresQID},
		{name: "already flagged without QID keeps flag", initial: true, input: boolPtr(true), want: true},
		{name: "omitted on flagged author without QID", initial: true, input: nil, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newService()
			id := uuid.New()
			a := catalogDomain.Author{ID: id, Name: "Leo Tolstoy", Slug: "leo-tolstoy", Status: catalogDomain.AuthorStatusPublished}
			if tt.withQID {
				a = qidAuthor(id, "Q7243")
			}
			a.AutoHarvest = tt.initial
			a.LastAutoHarvestedAt = &stamp
			repo.authors = []catalogDomain.Author{a}

			_, err := svc.UpdateAuthor(context.Background(), id, AuthorInput{
				Name: "Лев Толстой", Slug: "leo-tolstoy", Biography: strPtr("new bio"), AutoHarvest: tt.input,
			})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				if repo.authors[0].Name != "Leo Tolstoy" || repo.authors[0].AutoHarvest != tt.initial {
					t.Fatalf("author must stay unchanged on error, got %+v", repo.authors[0])
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := repo.authors[0]
			if got.AutoHarvest != tt.want {
				t.Fatalf("auto_harvest = %v, want %v", got.AutoHarvest, tt.want)
			}
			if got.LastAutoHarvestedAt == nil || !got.LastAutoHarvestedAt.Equal(stamp) {
				t.Fatalf("last_auto_harvested_at must be kept, got %v", got.LastAutoHarvestedAt)
			}
		})
	}
}

func TestCreateAuthor_IgnoresAutoHarvest(t *testing.T) {
	svc, _ := newService()
	a, err := svc.CreateAuthor(context.Background(), AuthorInput{Name: "Leo Tolstoy", AutoHarvest: boolPtr(true)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.AutoHarvest || a.LastAutoHarvestedAt != nil {
		t.Fatalf("a created author must not be flagged, got %+v", a)
	}
}

func TestListAutoHarvestCandidates(t *testing.T) {
	svc, repo := newService()
	now := time.Date(2026, 10, 13, 4, 0, 0, 0, time.UTC)
	id1, id2 := uuid.New(), uuid.New()
	repo.autoHarvestCandidates = []catalogDomain.Author{
		qidAuthor(id1, " Q7243 "),
		{ID: uuid.New(), Name: "No QID"},
		qidAuthor(id2, "Q1183296"),
	}

	got, err := svc.ListAutoHarvestCandidates(context.Background(), 10, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.autoHarvestLimit != 10 {
		t.Errorf("limit = %d, want 10", repo.autoHarvestLimit)
	}
	if want := now.Add(-30 * 24 * time.Hour); !repo.autoHarvestNotAfter.Equal(want) {
		t.Errorf("notAfter = %v, want %v", repo.autoHarvestNotAfter, want)
	}
	want := []AutoHarvestCandidate{{ID: id1, QID: "Q7243"}, {ID: id2, QID: "Q1183296"}}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidate %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestListAutoHarvestCandidates_ZeroLimit(t *testing.T) {
	svc, repo := newService()
	repo.autoHarvestCandidates = []catalogDomain.Author{qidAuthor(uuid.New(), "Q7243")}
	got, err := svc.ListAutoHarvestCandidates(context.Background(), 0, time.Now())
	if err != nil || len(got) != 0 {
		t.Fatalf("expected no candidates and no error, got %v, %v", got, err)
	}
}

func TestListAutoHarvestCandidates_RepoError(t *testing.T) {
	svc, repo := newService()
	repo.autoHarvestErr = errors.New("db down")
	if _, err := svc.ListAutoHarvestCandidates(context.Background(), 5, time.Now()); err == nil {
		t.Fatal("expected an error")
	}
}

func TestMarkAuthorsAutoHarvested(t *testing.T) {
	svc, repo := newService()
	at := time.Date(2026, 10, 13, 4, 0, 0, 0, time.UTC)
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	if err := svc.MarkAuthorsAutoHarvested(context.Background(), ids, at); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.markedAutoHarvested) != 2 || !repo.markedAutoHarvestedAt.Equal(at) {
		t.Fatalf("marks = %v at %v", repo.markedAutoHarvested, repo.markedAutoHarvestedAt)
	}
	if err := svc.MarkAuthorsAutoHarvested(context.Background(), nil, at); err != nil {
		t.Fatalf("empty ids must be a no-op, got %v", err)
	}
}
