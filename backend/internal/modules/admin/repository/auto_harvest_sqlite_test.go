package repository

import (
	"context"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// autoAuthor inserts an author for the scheduled-harvest selection tests.
func (f *modFixture) autoAuthor(name, status string, qid *string, flagged bool, last *time.Time) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	var source *string
	if qid != nil {
		s := catalogDomain.AuthorExternalSourceWikidata
		source = &s
	}
	if err := f.db.Exec(
		"INSERT INTO authors (id, name, slug, status, external_source, external_id, auto_harvest, last_auto_harvested_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, name, id.String(), status, source, qid, flagged, last, f.base, f.base,
	).Error; err != nil {
		f.t.Fatalf("insert author: %v", err)
	}
	return id
}

func qidPtr(s string) *string { return &s }

func TestListAutoHarvestCandidates_EligibilityAndOrder(t *testing.T) {
	f := newModerationRepo(t)
	now := time.Date(2026, 10, 13, 4, 0, 0, 0, time.UTC)
	notAfter := now.Add(-30 * 24 * time.Hour)
	old := now.Add(-60 * 24 * time.Hour)
	older := now.Add(-90 * 24 * time.Hour)
	recent := now.Add(-5 * 24 * time.Hour)
	pub := catalogDomain.AuthorStatusPublished

	neverB := f.autoAuthor("Bulgakov", pub, qidPtr("Q835"), true, nil)
	neverA := f.autoAuthor("Akhmatova", pub, qidPtr("Q80113"), true, nil)
	oldest := f.autoAuthor("Zamyatin", pub, qidPtr("Q309769"), true, &older)
	oldOne := f.autoAuthor("Chekhov", pub, qidPtr("Q5685"), true, &old)
	// Ineligible: recent, not flagged, pending, rejected, no QID.
	f.autoAuthor("Recent", pub, qidPtr("Q1"), true, &recent)
	f.autoAuthor("Unflagged", pub, qidPtr("Q2"), false, nil)
	f.autoAuthor("Pending", catalogDomain.AuthorStatusPending, qidPtr("Q3"), true, nil)
	f.autoAuthor("Rejected", catalogDomain.AuthorStatusRejected, qidPtr("Q4"), true, nil)
	f.autoAuthor("NoQID", pub, nil, true, nil)

	got, err := f.repo.ListAutoHarvestCandidates(context.Background(), 10, notAfter)
	if err != nil {
		t.Fatalf("ListAutoHarvestCandidates: %v", err)
	}
	want := []uuid.UUID{neverA, neverB, oldest, oldOne}
	if len(got) != len(want) {
		names := make([]string, 0, len(got))
		for _, a := range got {
			names = append(names, a.Name)
		}
		t.Fatalf("got %d candidates %v, want %d", len(got), names, len(want))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Errorf("candidate %d = %s, want %s", i, got[i].Name, want[i])
		}
	}

	limited, err := f.repo.ListAutoHarvestCandidates(context.Background(), 2, notAfter)
	if err != nil {
		t.Fatalf("ListAutoHarvestCandidates(limit 2): %v", err)
	}
	if len(limited) != 2 || limited[0].ID != neverA || limited[1].ID != neverB {
		t.Fatalf("limit 2 must keep the never-harvested authors first, got %+v", limited)
	}
}

func TestMarkAuthorsAutoHarvested_AndUpdateKeepsStamp(t *testing.T) {
	f := newModerationRepo(t)
	pub := catalogDomain.AuthorStatusPublished
	a := f.autoAuthor("Bulgakov", pub, qidPtr("Q835"), true, nil)
	b := f.autoAuthor("Akhmatova", pub, qidPtr("Q80113"), true, nil)
	untouched := f.autoAuthor("Chekhov", pub, qidPtr("Q5685"), true, nil)
	ctx := context.Background()

	// Load before the stamp, save after it: the stamp must survive.
	loaded, err := f.repo.GetAuthorByID(ctx, a)
	if err != nil {
		t.Fatalf("GetAuthorByID: %v", err)
	}

	at := time.Date(2026, 10, 13, 4, 0, 0, 0, time.UTC)
	if err := f.repo.MarkAuthorsAutoHarvested(ctx, []uuid.UUID{a, b}, at); err != nil {
		t.Fatalf("MarkAuthorsAutoHarvested: %v", err)
	}
	if err := f.repo.MarkAuthorsAutoHarvested(ctx, nil, at); err != nil {
		t.Fatalf("empty ids must be a no-op: %v", err)
	}

	bio := "new bio"
	loaded.Biography = &bio
	if err := f.repo.UpdateAuthor(ctx, loaded); err != nil {
		t.Fatalf("UpdateAuthor: %v", err)
	}

	for _, id := range []uuid.UUID{a, b} {
		got, err := f.repo.GetAuthorByID(ctx, id)
		if err != nil {
			t.Fatalf("GetAuthorByID: %v", err)
		}
		if got.LastAutoHarvestedAt == nil || !got.LastAutoHarvestedAt.Equal(at) {
			t.Errorf("%s: last_auto_harvested_at = %v, want %v", got.Name, got.LastAutoHarvestedAt, at)
		}
		if !got.AutoHarvest {
			t.Errorf("%s: auto_harvest must stay set", got.Name)
		}
	}
	got, err := f.repo.GetAuthorByID(ctx, untouched)
	if err != nil {
		t.Fatalf("GetAuthorByID: %v", err)
	}
	if got.LastAutoHarvestedAt != nil {
		t.Errorf("unmarked author was stamped: %v", got.LastAutoHarvestedAt)
	}
}
