package service

import (
	"context"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
)

// Harvest genre resolution (docs/features/genre-status.md rule 5).

func TestProcessRecord_UsesGenreRefsQIDs(t *testing.T) {
	admin, repo := newTestAdmin()
	renamedID := uuid.New()
	repo.genres = append(repo.genres, catalogDomain.Genre{
		ID: renamedID, Name: "переименованный роман", Slug: "pereimenovannyy-roman",
		Status: catalogDomain.GenreStatusPublished, WikidataQID: strPtr("Q8261"),
	})

	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Genres:     []string{"роман", "эпопея"},
		GenreRefs:  []bookdata.ExternalGenre{{Name: "роман", QID: "Q8261"}, {Name: "эпопея", QID: "Q99"}},
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
	}

	if _, err := processRecord(context.Background(), rec, nil, admin); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.genres) != 2 {
		t.Fatalf("expected QID match + 1 new genre, got %d genres", len(repo.genres))
	}
	created := repo.genres[1]
	if created.Status != catalogDomain.GenreStatusPending || created.WikidataQID == nil || *created.WikidataQID != "Q99" {
		t.Fatalf("expected new pending genre with Q99, got %+v", created)
	}
	ids := repo.bookGenreIDs[0]
	if len(ids) != 2 || ids[0] != renamedID {
		t.Fatalf("expected book linked to the QID-matched genre first, got %v", ids)
	}

	// Re-running the same record (new source ID) creates no duplicates.
	rec.SourceID = "Q170584"
	rec.Title = "Анна Каренина"
	if _, err := processRecord(context.Background(), rec, nil, admin); err != nil {
		t.Fatalf("unexpected error on re-run: %v", err)
	}
	if len(repo.genres) != 2 {
		t.Fatalf("expected no duplicate genres on re-run, got %d", len(repo.genres))
	}
}

func TestProcessRecord_FallsBackToGenreNamesWithoutRefs(t *testing.T) {
	admin, repo := newTestAdmin()
	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Genres:     []string{"Роман"},
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
	}

	if _, err := processRecord(context.Background(), rec, nil, admin); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.genres) != 1 || repo.genres[0].Status != catalogDomain.GenreStatusPending || repo.genres[0].WikidataQID != nil {
		t.Fatalf("expected one QID-less pending genre, got %+v", repo.genres)
	}
}

func TestProcessRecord_AllGenresRejectedStillCreatesHiddenBook(t *testing.T) {
	admin, repo := newTestAdmin()
	repo.genres = append(repo.genres,
		catalogDomain.Genre{ID: uuid.New(), Name: "мусор", Slug: "musor", Status: catalogDomain.GenreStatusRejected, WikidataQID: strPtr("Q1")},
		catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusRejected},
	)
	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Genres:     []string{"другое", "роман"},
		GenreRefs:  []bookdata.ExternalGenre{{Name: "другое", QID: "Q1"}, {Name: "роман", QID: "Q8261"}},
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
	}

	outcome, err := processRecord(context.Background(), rec, nil, admin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != outcomeCreated || len(repo.books) != 1 || repo.books[0].Status != catalogDomain.BookStatusPending {
		t.Fatalf("expected one hidden book created, outcome=%v books=%+v", outcome, repo.books)
	}
	if len(repo.genres) != 2 {
		t.Fatalf("expected no new genre, got %d", len(repo.genres))
	}
	if len(repo.bookGenreIDs[0]) != 0 {
		t.Fatalf("expected no book_genres link to rejected genres, got %v", repo.bookGenreIDs[0])
	}
	if q := repo.genres[1].WikidataQID; q == nil || *q != "Q8261" {
		t.Fatalf("expected the slug-matched tombstone to get its QID backfilled, got %v", q)
	}
}

// TestProcessRecord_GenreAliasLinksTarget: a genre ref that hits an alias
// row (a merged genre, docs/features/genre-moderation-queue.md rule 5)
// links the alias target, by QID and by slug, and creates nothing.
func TestProcessRecord_GenreAliasLinksTarget(t *testing.T) {
	admin, repo := newTestAdmin()
	targetID := uuid.New()
	target := targetID
	repo.genres = append(repo.genres,
		catalogDomain.Genre{ID: targetID, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPublished},
		catalogDomain.Genre{ID: uuid.New(), Name: "романы", Slug: "romany", Status: catalogDomain.GenreStatusRejected, WikidataQID: strPtr("Q123"), MergedIntoID: &target},
	)

	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Genres:     []string{"романы", "что-то"},
		GenreRefs:  []bookdata.ExternalGenre{{Name: "романы"}, {Name: "что-то", QID: "Q123"}},
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
	}
	if _, err := processRecord(context.Background(), rec, nil, admin); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.genres) != 2 {
		t.Fatalf("expected no new genre, got %d", len(repo.genres))
	}
	ids := repo.bookGenreIDs[0]
	if len(ids) != 1 || ids[0] != targetID {
		t.Fatalf("expected the book linked to the alias target once, got %v", ids)
	}
	if repo.genres[0].WikidataQID != nil {
		t.Fatalf("no QID must be backfilled onto the target, got %v", *repo.genres[0].WikidataQID)
	}
}
