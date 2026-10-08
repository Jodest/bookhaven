package service

import (
	"context"
	"errors"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// Tests for docs/features/genre-status.md (status lifecycle + Wikidata QID).

func genreQID(g catalogDomain.Genre) string {
	if g.WikidataQID == nil {
		return ""
	}
	return *g.WikidataQID
}

func TestResolveOrCreateHarvestGenres_NewGenrePendingWithQID(t *testing.T) {
	svc, repo := newService()

	ids, err := svc.ResolveOrCreateHarvestGenres(context.Background(), []GenreRef{{Name: "Роман", QID: "Q8261"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || len(repo.genres) != 1 {
		t.Fatalf("expected 1 created+linked genre, ids=%v genres=%d", ids, len(repo.genres))
	}
	g := repo.genres[0]
	if g.Status != catalogDomain.GenreStatusPending || genreQID(g) != "Q8261" || g.Slug != "roman" || g.Name != "роман" {
		t.Fatalf("unexpected created genre: %+v (qid %q)", g, genreQID(g))
	}
}

func TestResolveOrCreateHarvestGenres_QIDMatchBeatsSlug(t *testing.T) {
	svc, repo := newService()
	byQID := uuid.New()
	bySlug := uuid.New()
	repo.genres = []catalogDomain.Genre{
		{ID: byQID, Name: "переименованный", Slug: "pereimenovannyy", Status: catalogDomain.GenreStatusPublished, WikidataQID: strPtr("Q8261")},
		{ID: bySlug, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPublished},
	}

	ids, err := svc.ResolveOrCreateHarvestGenres(context.Background(), []GenreRef{{Name: "Роман", QID: "Q8261"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != byQID.String() {
		t.Fatalf("expected QID match %s, got %v", byQID, ids)
	}
	if repo.genres[1].WikidataQID != nil {
		t.Fatalf("slug-namesake must stay untouched, got qid %q", genreQID(repo.genres[1]))
	}
	if len(repo.genres) != 2 {
		t.Fatalf("nothing must be created, got %d genres", len(repo.genres))
	}
}

func TestResolveOrCreateHarvestGenres_QIDMatchedPendingReused(t *testing.T) {
	svc, repo := newService()
	id := uuid.New()
	repo.genres = []catalogDomain.Genre{{ID: id, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPending, WikidataQID: strPtr("Q8261")}}

	for run := 0; run < 2; run++ {
		ids, err := svc.ResolveOrCreateHarvestGenres(context.Background(), []GenreRef{{Name: "Роман", QID: "Q8261"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 1 || ids[0] != id.String() {
			t.Fatalf("run %d: expected pending genre %s reused, got %v", run, id, ids)
		}
	}
	if len(repo.genres) != 1 {
		t.Fatalf("expected no duplicates, got %d genres", len(repo.genres))
	}
}

func TestResolveOrCreateHarvestGenres_QIDMatchedRejectedSkipped(t *testing.T) {
	svc, repo := newService()
	repo.genres = []catalogDomain.Genre{{ID: uuid.New(), Name: "мусор", Slug: "musor", Status: catalogDomain.GenreStatusRejected, WikidataQID: strPtr("Q1")}}

	ids, err := svc.ResolveOrCreateHarvestGenres(context.Background(), []GenreRef{{Name: "Другое имя", QID: "Q1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 || len(repo.genres) != 1 {
		t.Fatalf("tombstone must be skipped and nothing created, ids=%v genres=%d", ids, len(repo.genres))
	}
}

func TestResolveOrCreateHarvestGenres_SlugMatchedRejectedSkippedAndBackfilled(t *testing.T) {
	svc, repo := newService()
	repo.genres = []catalogDomain.Genre{{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusRejected}}

	ids, err := svc.ResolveOrCreateHarvestGenres(context.Background(), []GenreRef{{Name: "Роман", QID: "Q8261"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 || len(repo.genres) != 1 {
		t.Fatalf("rejected slug match must be skipped, ids=%v genres=%d", ids, len(repo.genres))
	}
	g := repo.genres[0]
	if genreQID(g) != "Q8261" || g.Status != catalogDomain.GenreStatusRejected {
		t.Fatalf("expected QID backfilled with status unchanged, got %+v", g)
	}
}

func TestResolveOrCreateHarvestGenres_SlugMatchBackfillsQID(t *testing.T) {
	svc, repo := newService()
	id := uuid.New()
	repo.genres = []catalogDomain.Genre{{ID: id, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPublished}}

	ids, err := svc.ResolveOrCreateHarvestGenres(context.Background(), []GenreRef{{Name: "Роман", QID: "Q8261"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != id.String() {
		t.Fatalf("expected slug match linked, got %v", ids)
	}
	g := repo.genres[0]
	if genreQID(g) != "Q8261" || g.Status != catalogDomain.GenreStatusPublished {
		t.Fatalf("expected QID backfilled, status unchanged; got %+v", g)
	}
}

func TestResolveOrCreateHarvestGenres_BackfillUniqueViolationStillLinks(t *testing.T) {
	svc, repo := newService()
	id := uuid.New()
	repo.genres = []catalogDomain.Genre{{ID: id, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPending}}
	repo.setGenreQIDErr = &pgconn.PgError{Code: "23505", ConstraintName: "idx_genres_wikidata_qid"}

	ids, err := svc.ResolveOrCreateHarvestGenres(context.Background(), []GenreRef{{Name: "Роман", QID: "Q8261"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != id.String() || repo.genres[0].WikidataQID != nil {
		t.Fatalf("expected link without backfill, ids=%v genre=%+v", ids, repo.genres[0])
	}
}

func TestResolveOrCreateHarvestGenres_SlugMatchDifferentQIDLinksUntouched(t *testing.T) {
	svc, repo := newService()
	id := uuid.New()
	repo.genres = []catalogDomain.Genre{{ID: id, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPending, WikidataQID: strPtr("Q1")}}

	ids, err := svc.ResolveOrCreateHarvestGenres(context.Background(), []GenreRef{{Name: "Роман", QID: "Q8261"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != id.String() {
		t.Fatalf("expected existing genre linked, got %v", ids)
	}
	if len(repo.genres) != 1 || genreQID(repo.genres[0]) != "Q1" || repo.setGenreQIDCalls != 0 {
		t.Fatalf("expected no namesake and the genre untouched, genres=%+v calls=%d", repo.genres, repo.setGenreQIDCalls)
	}
}

func TestResolveOrCreateHarvestGenres_NoQIDUsesSlug(t *testing.T) {
	svc, repo := newService()
	id := uuid.New()
	repo.genres = []catalogDomain.Genre{{ID: id, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPublished, WikidataQID: strPtr("Q8261")}}

	ids, err := svc.ResolveOrCreateHarvestGenres(context.Background(), []GenreRef{{Name: "Роман"}, {Name: "Повесть"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != id.String() {
		t.Fatalf("expected slug match + 1 created, got %v", ids)
	}
	created := repo.genres[1]
	if created.Status != catalogDomain.GenreStatusPending || created.WikidataQID != nil {
		t.Fatalf("expected QID-less pending genre, got %+v", created)
	}
	if repo.setGenreQIDCalls != 0 {
		t.Fatalf("no backfill expected without a QID")
	}
}

func TestResolveOrCreateHarvestGenres_Dedupe(t *testing.T) {
	svc, repo := newService()

	ids, err := svc.ResolveOrCreateHarvestGenres(context.Background(), []GenreRef{
		{Name: "Роман", QID: "Q8261"}, {Name: "  "}, {Name: "роман", QID: "Q8261"}, {Name: "Роман"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || len(repo.genres) != 1 {
		t.Fatalf("expected 1 genre, ids=%v genres=%d", ids, len(repo.genres))
	}
}

func TestResolveOrCreateHarvestGenres_AllSkipped(t *testing.T) {
	svc, repo := newService()
	repo.genres = []catalogDomain.Genre{
		{ID: uuid.New(), Name: "а", Slug: "a", Status: catalogDomain.GenreStatusRejected, WikidataQID: strPtr("Q1")},
		{ID: uuid.New(), Name: "б", Slug: "b", Status: catalogDomain.GenreStatusRejected},
	}

	ids, err := svc.ResolveOrCreateHarvestGenres(context.Background(), []GenreRef{{Name: "x", QID: "Q1"}, {Name: "б"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ids == nil || len(ids) != 0 {
		t.Fatalf("expected empty non-nil result, got %#v", ids)
	}
}

func TestResolveOrCreateGenreIDsByNames_CreatesPendingAndLinksRejected(t *testing.T) {
	svc, repo := newService()
	rejected := uuid.New()
	repo.genres = []catalogDomain.Genre{{ID: rejected, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusRejected}}

	ids, err := svc.ResolveOrCreateGenreIDsByNames(context.Background(), []string{"Роман", "Повесть"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != rejected.String() {
		t.Fatalf("expected rejected slug match linked + 1 created, got %v", ids)
	}
	if repo.genres[1].Status != catalogDomain.GenreStatusPending || repo.genres[1].WikidataQID != nil {
		t.Fatalf("expected import-created genre pending without QID, got %+v", repo.genres[1])
	}
}

func TestCreateGenre_ForcesPublishedAndValidatesQID(t *testing.T) {
	ctx := context.Background()

	t.Run("published, status ignored, qid normalized", func(t *testing.T) {
		svc, _ := newService()
		g, err := svc.CreateGenre(ctx, GenreInput{Name: "Роман", Status: strPtr(catalogDomain.GenreStatusRejected), WikidataQID: strPtr(" q8261 ")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if g.Status != catalogDomain.GenreStatusPublished || genreQID(*g) != "Q8261" {
			t.Fatalf("unexpected genre: %+v qid=%q", g, genreQID(*g))
		}
	})

	t.Run("empty qid creates without one", func(t *testing.T) {
		svc, _ := newService()
		g, err := svc.CreateGenre(ctx, GenreInput{Name: "Роман", WikidataQID: strPtr("")})
		if err != nil || g.WikidataQID != nil {
			t.Fatalf("expected no qid, got %+v err=%v", g, err)
		}
	})

	t.Run("invalid qid", func(t *testing.T) {
		svc, repo := newService()
		if _, err := svc.CreateGenre(ctx, GenreInput{Name: "Роман", WikidataQID: strPtr("X12")}); !errors.Is(err, ErrInvalidQID) {
			t.Fatalf("expected ErrInvalidQID, got %v", err)
		}
		if len(repo.genres) != 0 {
			t.Fatal("nothing must be created")
		}
	})

	t.Run("duplicate qid", func(t *testing.T) {
		svc, repo := newService()
		repo.genres = []catalogDomain.Genre{{ID: uuid.New(), Name: "другой", Slug: "drugoy", Status: catalogDomain.GenreStatusPending, WikidataQID: strPtr("Q8261")}}
		if _, err := svc.CreateGenre(ctx, GenreInput{Name: "Роман", WikidataQID: strPtr("Q8261")}); !errors.Is(err, ErrDuplicateGenreQID) {
			t.Fatalf("expected ErrDuplicateGenreQID, got %v", err)
		}
	})
}

func TestUpdateGenre_StatusAndQID(t *testing.T) {
	ctx := context.Background()
	setup := func() (*Service, *fakeRepository, uuid.UUID) {
		svc, repo := newService()
		id := uuid.New()
		repo.genres = []catalogDomain.Genre{
			{ID: id, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPending, WikidataQID: strPtr("Q8261")},
			{ID: uuid.New(), Name: "повесть", Slug: "povest", Status: catalogDomain.GenreStatusPublished, WikidataQID: strPtr("Q49084")},
		}
		return svc, repo, id
	}

	t.Run("valid status", func(t *testing.T) {
		svc, _, id := setup()
		g, err := svc.UpdateGenre(ctx, id, GenreInput{Name: "роман", Status: strPtr(catalogDomain.GenreStatusPublished)})
		if err != nil || g.Status != catalogDomain.GenreStatusPublished {
			t.Fatalf("expected published, got %+v err=%v", g, err)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		svc, _, id := setup()
		if _, err := svc.UpdateGenre(ctx, id, GenreInput{Name: "роман", Status: strPtr("bogus")}); !errors.Is(err, ErrInvalidGenreStatus) {
			t.Fatalf("expected ErrInvalidGenreStatus, got %v", err)
		}
	})

	t.Run("omitted status and qid are unchanged", func(t *testing.T) {
		svc, _, id := setup()
		g, err := svc.UpdateGenre(ctx, id, GenreInput{Name: "роман"})
		if err != nil || g.Status != catalogDomain.GenreStatusPending || genreQID(*g) != "Q8261" {
			t.Fatalf("expected unchanged, got %+v err=%v", g, err)
		}
	})

	t.Run("set qid", func(t *testing.T) {
		svc, _, id := setup()
		g, err := svc.UpdateGenre(ctx, id, GenreInput{Name: "роман", WikidataQID: strPtr("q1")})
		if err != nil || genreQID(*g) != "Q1" {
			t.Fatalf("expected Q1, got %+v err=%v", g, err)
		}
	})

	t.Run("keep own qid", func(t *testing.T) {
		svc, _, id := setup()
		if _, err := svc.UpdateGenre(ctx, id, GenreInput{Name: "роман", WikidataQID: strPtr("Q8261")}); err != nil {
			t.Fatalf("own qid must not conflict: %v", err)
		}
	})

	t.Run("clear qid", func(t *testing.T) {
		svc, _, id := setup()
		g, err := svc.UpdateGenre(ctx, id, GenreInput{Name: "роман", WikidataQID: strPtr("")})
		if err != nil || g.WikidataQID != nil {
			t.Fatalf("expected cleared, got %+v err=%v", g, err)
		}
	})

	t.Run("invalid qid", func(t *testing.T) {
		svc, _, id := setup()
		if _, err := svc.UpdateGenre(ctx, id, GenreInput{Name: "роман", WikidataQID: strPtr("Q12a")}); !errors.Is(err, ErrInvalidQID) {
			t.Fatalf("expected ErrInvalidQID, got %v", err)
		}
	})

	t.Run("duplicate qid", func(t *testing.T) {
		svc, _, id := setup()
		if _, err := svc.UpdateGenre(ctx, id, GenreInput{Name: "роман", WikidataQID: strPtr("Q49084")}); !errors.Is(err, ErrDuplicateGenreQID) {
			t.Fatalf("expected ErrDuplicateGenreQID, got %v", err)
		}
	})
}

func TestGenreDuplicateKeyError_MapsQIDConstraint(t *testing.T) {
	if err := genreDuplicateKeyError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_genres_wikidata_qid"}); !errors.Is(err, ErrDuplicateGenreQID) {
		t.Fatalf("expected ErrDuplicateGenreQID, got %v", err)
	}
	if err := genreDuplicateKeyError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_genres_slug"}); !errors.Is(err, ErrDuplicateSlug) {
		t.Fatalf("expected ErrDuplicateSlug, got %v", err)
	}
}

func TestListGenres_StatusFilter(t *testing.T) {
	svc, repo := newService()
	repo.genres = []catalogDomain.Genre{
		{ID: uuid.New(), Name: "а", Slug: "a", Status: catalogDomain.GenreStatusPending},
		{ID: uuid.New(), Name: "б", Slug: "b", Status: catalogDomain.GenreStatusPublished},
	}

	got, total, err := svc.ListGenres(context.Background(), "", catalogDomain.GenreStatusPending, 1, 20)
	if err != nil || total != 1 || len(got) != 1 || got[0].Slug != "a" {
		t.Fatalf("expected only pending genre, got %+v total=%d err=%v", got, total, err)
	}
	if _, _, err := svc.ListGenres(context.Background(), "", "bogus", 1, 20); !errors.Is(err, ErrInvalidGenreStatus) {
		t.Fatalf("expected ErrInvalidGenreStatus, got %v", err)
	}
}

func TestDashboardCounts_PendingGenres(t *testing.T) {
	svc, repo := newService()
	repo.genres = []catalogDomain.Genre{
		{ID: uuid.New(), Status: catalogDomain.GenreStatusPending},
		{ID: uuid.New(), Status: catalogDomain.GenreStatusPending},
		{ID: uuid.New(), Status: catalogDomain.GenreStatusPublished},
		{ID: uuid.New(), Status: catalogDomain.GenreStatusRejected},
	}

	stats, err := svc.DashboardCounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.PendingGenres != 2 || stats.Genres != 4 {
		t.Fatalf("expected pending_genres=2 genres=4, got %+v", stats)
	}
}
