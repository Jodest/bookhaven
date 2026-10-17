package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Genre moderation queue, merge and alias resolution
// (docs/features/genre-moderation-queue.md).

func queueGenre(name, status string) catalogDomain.Genre {
	return catalogDomain.Genre{ID: uuid.New(), Name: name, Slug: uuid.NewString(), Status: status}
}

func aliasOf(target uuid.UUID, name, slug string, qid *string) catalogDomain.Genre {
	t := target
	return catalogDomain.Genre{ID: uuid.New(), Name: name, Slug: slug, Status: catalogDomain.GenreStatusRejected, WikidataQID: qid, MergedIntoID: &t}
}

func TestNormalizeGenreName(t *testing.T) {
	cases := map[string]string{
		"Роман": "роман",
		"  Научная   ФАНТАСТИКА ": "научная фантастика",
		"Ёлочные игрушки":         "елочные игрушки",
		"sci-fi":                  "sci fi",
		"роман (жанр)":            "роман жанр",
		"!!!":                     "",
	}
	for in, want := range cases {
		if got := normalizeGenreName(in); got != want {
			t.Errorf("normalizeGenreName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenreStem(t *testing.T) {
	cases := map[string]string{
		"романы":      "роман",
		"роман":       "роман",
		"научная":     "научн",
		"научной":     "научн",
		"фантастика":  "фантастик",
		"фантастики":  "фантастик",
		"детективами": "детектив", // longest suffix "ами", not "и"
		"повестями":   "повест",
		"драма":       "драм",  // exactly 4 runes remain
		"мифы":        "мифы",  // only 3 would remain: kept
		"эпос":        "эпос",  // no suffix
		"ода":         "ода",   // too short
		"детского":    "детск", // "ого"
	}
	for in, want := range cases {
		if got := genreStem(in); got != want {
			t.Errorf("genreStem(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenreMatchKind(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{"Роман", "роман!", GenreMatchExactName},
		{"роман", "романы", GenreMatchSameStem},
		{"научная фантастика", "научной фантастики", GenreMatchSameStem},
		{"фантастика", "научная фантастика", GenreMatchSharedWord},
		{"драма", "семейная драма", ""}, // shared stem "драм" is shorter than 5 runes
		{"роман", "повесть", ""},
		{"", "роман", ""},
	}
	for _, tc := range cases {
		if got := genreMatchKind(tc.a, tc.b); got != tc.want {
			t.Errorf("genreMatchKind(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestBuildGenreHints(t *testing.T) {
	row := queueGenre("роман", catalogDomain.GenreStatusPending)

	t.Run("excludes self rejected and alias, empty slice", func(t *testing.T) {
		alias := aliasOf(uuid.New(), "роман", "roman-alias", nil)
		got := buildGenreHints(row, []catalogDomain.Genre{
			row,
			queueGenre("роман", catalogDomain.GenreStatusRejected),
			alias,
			queueGenre("повесть", catalogDomain.GenreStatusPublished),
		})
		if got == nil || len(got) != 0 {
			t.Fatalf("expected empty non-nil hints, got %#v", got)
		}
	})

	t.Run("ordering and cap", func(t *testing.T) {
		exact := queueGenre("Роман", catalogDomain.GenreStatusPending)
		stemPub := queueGenre("романы", catalogDomain.GenreStatusPublished)
		stemPend := queueGenre("романа", catalogDomain.GenreStatusPending)
		shared := []catalogDomain.Genre{
			queueGenre("исторический роман", catalogDomain.GenreStatusPublished),
			queueGenre("любовный роман", catalogDomain.GenreStatusPublished),
			queueGenre("авантюрный роман", catalogDomain.GenreStatusPending),
		}
		candidates := append([]catalogDomain.Genre{}, shared...)
		candidates = append(candidates, stemPend, exact, stemPub, exact) // exact twice: deduped
		got := buildGenreHints(row, candidates)
		if len(got) != maxGenreHints {
			t.Fatalf("expected %d hints, got %d", maxGenreHints, len(got))
		}
		wantIDs := []uuid.UUID{exact.ID, stemPub.ID, stemPend.ID, shared[0].ID, shared[1].ID}
		for i, h := range got {
			if h.Genre.ID != wantIDs[i] {
				t.Fatalf("hint %d = %s (%s), want %v", i, h.Genre.Name, h.Match, wantIDs[i])
			}
		}
		if got[0].Match != GenreMatchExactName || got[1].Match != GenreMatchSameStem || got[3].Match != GenreMatchSharedWord {
			t.Fatalf("unexpected match kinds: %+v", got)
		}
	})
}

func TestListGenreModerationQueue_FiltersAndClamp(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()

	for _, f := range []repository.GenreModerationFilter{{Status: "deleted"}, {QID: "maybe"}} {
		if _, _, err := svc.ListGenreModerationQueue(ctx, f, 1, 20); !errors.Is(err, ErrInvalidModerationFilter) {
			t.Fatalf("filter %+v: expected ErrInvalidModerationFilter, got %v", f, err)
		}
	}

	items, total, err := svc.ListGenreModerationQueue(ctx, repository.GenreModerationFilter{QID: "with", Search: "ром"}, 3, 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := repo.lastGenreModFilter
	if f.Status != catalogDomain.GenreStatusPending || f.QID != "with" || f.Search != "ром" || f.Limit != genreQueueMaxLimit || f.Offset != 2*genreQueueMaxLimit {
		t.Fatalf("unexpected filter: %+v", f)
	}
	if items == nil || len(items) != 0 || total != 0 {
		t.Fatalf("expected empty non-nil page, got %#v total=%d", items, total)
	}
	if repo.genreSampleCalls != 0 || repo.genreHintCalls != 0 {
		t.Fatalf("an empty page must not query samples or candidates")
	}

	if _, _, err := svc.ListGenreModerationQueue(ctx, repository.GenreModerationFilter{}, 0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := repo.lastGenreModFilter; f.Limit != genreQueueDefaultLimit || f.Offset != 0 {
		t.Fatalf("expected default limit, got %+v", f)
	}
}

func TestListGenreModerationQueue_SamplesAndHints(t *testing.T) {
	svc, repo := newService()
	romany := queueGenre("романы", catalogDomain.GenreStatusPending)
	romanyPage := queueGenre("Романа", catalogDomain.GenreStatusPending)
	roman := queueGenre("роман", catalogDomain.GenreStatusPublished)
	rejected := queueGenre("роман", catalogDomain.GenreStatusRejected)
	alias := aliasOf(roman.ID, "романчик", "romanchik", nil)
	repo.genres = []catalogDomain.Genre{romany, romanyPage, roman, rejected, alias}
	repo.genreModRows = []repository.GenreModerationRow{
		{Genre: romany, BooksTotal: 3, BooksVisible: 1},
		{Genre: romanyPage},
	}
	bookID := uuid.New()
	repo.genreSamples = map[uuid.UUID][]repository.SampleBook{romany.ID: {{ID: bookID, Title: "Война и мир"}}}

	items, total, err := svc.ListGenreModerationQueue(context.Background(), repository.GenreModerationFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected 2 items, got %d (total %d)", len(items), total)
	}
	if repo.genreSampleCalls != 1 || repo.genreHintCalls != 1 {
		t.Fatalf("expected one sample and one candidate query, got %d/%d", repo.genreSampleCalls, repo.genreHintCalls)
	}
	first := items[0]
	if first.BooksTotal != 3 || first.BooksVisible != 1 || len(first.SampleBooks) != 1 || first.SampleBooks[0].ID != bookID {
		t.Fatalf("unexpected book context: %+v", first)
	}
	if items[1].SampleBooks == nil {
		t.Fatal("expected non-nil empty samples")
	}
	// роман (published) first, then the other page row; rejected and alias excluded.
	var got []uuid.UUID
	for _, h := range first.Similar {
		got = append(got, h.Genre.ID)
	}
	if !reflect.DeepEqual(got, []uuid.UUID{roman.ID, romanyPage.ID}) {
		t.Fatalf("unexpected hints: %+v", first.Similar)
	}
}

func TestBulkSetGenreStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid status", func(t *testing.T) {
		svc, _ := newService()
		if _, err := svc.BulkSetGenreStatus(ctx, []uuid.UUID{uuid.New()}, "deleted"); !errors.Is(err, ErrInvalidGenreStatus) {
			t.Fatalf("expected ErrInvalidGenreStatus, got %v", err)
		}
	})

	t.Run("too many", func(t *testing.T) {
		svc, repo := newService()
		ids := make([]uuid.UUID, bulkGenreStatusCap+1)
		for i := range ids {
			ids[i] = uuid.New()
		}
		repo.genres = []catalogDomain.Genre{{ID: ids[0], Status: catalogDomain.GenreStatusPending}}
		if _, err := svc.BulkSetGenreStatus(ctx, ids, catalogDomain.GenreStatusPublished); !errors.Is(err, ErrBulkStatusTooLarge) {
			t.Fatalf("expected ErrBulkStatusTooLarge, got %v", err)
		}
		if repo.genres[0].Status != catalogDomain.GenreStatusPending {
			t.Fatal("nothing must change when over the cap")
		}
	})

	t.Run("dedupe, unknown, alias and failure are row errors", func(t *testing.T) {
		svc, repo := newService()
		qid := "Q1"
		g := catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPending, WikidataQID: &qid}
		failing := queueGenre("повесть", catalogDomain.GenreStatusPending)
		alias := aliasOf(g.ID, "романы", "romany", nil)
		repo.genres = []catalogDomain.Genre{g, failing, alias}
		repo.setGenreStatusErr = map[uuid.UUID]error{failing.ID: errors.New("db down")}
		unknown := uuid.New()

		report, err := svc.BulkSetGenreStatus(ctx, []uuid.UUID{g.ID, unknown, g.ID, alias.ID, failing.ID}, catalogDomain.GenreStatusRejected)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Processed != 4 || report.Updated != 1 || len(report.Errors) != 3 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if report.Errors[0].GenreID != unknown.String() || report.Errors[0].Message != "genre not found" ||
			report.Errors[1].GenreID != alias.ID.String() || report.Errors[1].Message != "genre not found" ||
			report.Errors[2].GenreID != failing.ID.String() {
			t.Fatalf("unexpected row errors: %+v", report.Errors)
		}
		got := repo.genres[0]
		if got.Status != catalogDomain.GenreStatusRejected || got.Name != "роман" || got.Slug != "roman" || genreQID(got) != "Q1" {
			t.Fatalf("only the status must change: %+v", got)
		}
	})
}

func TestMergeGenres_Service(t *testing.T) {
	ctx := context.Background()

	t.Run("self merge", func(t *testing.T) {
		svc, repo := newService()
		id := uuid.New()
		if _, _, _, _, err := svc.MergeGenres(ctx, id, id); !errors.Is(err, ErrCannotMergeGenreIntoSelf) {
			t.Fatalf("expected ErrCannotMergeGenreIntoSelf, got %v", err)
		}
		if repo.mergeGenresCalls != 0 {
			t.Fatal("repository must not be called")
		}
	})

	t.Run("missing or alias is not found", func(t *testing.T) {
		svc, repo := newService()
		target := queueGenre("роман", catalogDomain.GenreStatusPublished)
		alias := aliasOf(target.ID, "романы", "romany", nil)
		repo.genres = []catalogDomain.Genre{target, alias}
		for name, pair := range map[string][2]uuid.UUID{
			"missing source": {uuid.New(), target.ID},
			"missing target": {target.ID, uuid.New()},
			"alias source":   {alias.ID, target.ID},
			"alias target":   {target.ID, alias.ID},
		} {
			if _, _, _, _, err := svc.MergeGenres(ctx, pair[0], pair[1]); !errors.Is(err, ErrGenreNotFound) {
				t.Fatalf("%s: expected ErrGenreNotFound, got %v", name, err)
			}
		}
		if repo.mergeGenresCalls != 0 {
			t.Fatal("repository merge must not be called")
		}
	})

	t.Run("concurrent merge maps to not found", func(t *testing.T) {
		svc, repo := newService()
		a, b := queueGenre("а", "pending"), queueGenre("б", "pending")
		repo.genres = []catalogDomain.Genre{a, b}
		repo.mergeGenresErr = gorm.ErrRecordNotFound
		if _, _, _, _, err := svc.MergeGenres(ctx, a.ID, b.ID); !errors.Is(err, ErrGenreNotFound) {
			t.Fatalf("expected ErrGenreNotFound, got %v", err)
		}
	})

	t.Run("carries the QID onto a target without one", func(t *testing.T) {
		svc, repo := newService()
		source := catalogDomain.Genre{ID: uuid.New(), Name: "романы", Slug: "romany", Status: "pending", WikidataQID: strPtr("Q123")}
		target := catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: "published"}
		repo.genres = []catalogDomain.Genre{source, target}
		repo.mergeGenresBooks = 2

		got, books, _, _, err := svc.MergeGenres(ctx, source.ID, target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if books != 2 || got.ID != target.ID || genreQID(*got) != "Q123" || got.Name != "роман" || got.Status != "published" {
			t.Fatalf("unexpected result: %+v books=%d", got, books)
		}
	})

	t.Run("target keeps its own QID", func(t *testing.T) {
		svc, repo := newService()
		source := catalogDomain.Genre{ID: uuid.New(), Name: "романы", Slug: "romany", Status: "pending", WikidataQID: strPtr("Q123")}
		target := catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: "published", WikidataQID: strPtr("Q8261")}
		repo.genres = []catalogDomain.Genre{source, target}

		got, _, _, _, err := svc.MergeGenres(ctx, source.ID, target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if genreQID(*got) != "Q8261" {
			t.Fatalf("expected target QID kept, got %q", genreQID(*got))
		}
	})
}

func TestResolveOrCreateHarvestGenres_Aliases(t *testing.T) {
	ctx := context.Background()

	t.Run("QID match on an alias links the target", func(t *testing.T) {
		svc, repo := newService()
		target := catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPublished, WikidataQID: strPtr("Q8261")}
		alias := aliasOf(target.ID, "романы", "romany", strPtr("Q123"))
		repo.genres = []catalogDomain.Genre{target, alias}

		res, err := svc.resolveHarvestGenre(ctx, "что угодно", "Q123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Genre.ID != target.ID || res.Outcome != GenreOutcomeExisting || res.Match != GenreMatchQID || res.ViaAlias == nil || res.ViaAlias.ID != alias.ID {
			t.Fatalf("unexpected resolution: %+v", res)
		}
		if len(repo.genres) != 2 || repo.setGenreQIDCalls != 0 {
			t.Fatalf("nothing must be created or written: genres=%d setQID=%d", len(repo.genres), repo.setGenreQIDCalls)
		}
	})

	for _, qid := range []string{"", "Q555"} {
		t.Run("slug match on an alias links the target, no backfill, qid="+qid, func(t *testing.T) {
			svc, repo := newService()
			target := catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPending}
			alias := aliasOf(target.ID, "романы", "romany", nil)
			repo.genres = []catalogDomain.Genre{target, alias}

			res, err := svc.resolveHarvestGenre(ctx, "Романы", qid)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Genre.ID != target.ID || res.Outcome != GenreOutcomeExisting || res.Match != GenreMatchSlug || res.ViaAlias == nil || res.QIDBackfilled {
				t.Fatalf("unexpected resolution: %+v", res)
			}
			if repo.setGenreQIDCalls != 0 || repo.genres[0].WikidataQID != nil || repo.genres[1].WikidataQID != nil || len(repo.genres) != 2 {
				t.Fatalf("no QID backfill and no new genre expected: %+v", repo.genres)
			}
		})
	}

	t.Run("alias with a rejected target is skipped", func(t *testing.T) {
		svc, repo := newService()
		target := catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusRejected}
		alias := aliasOf(target.ID, "романы", "romany", strPtr("Q123"))
		repo.genres = []catalogDomain.Genre{target, alias}

		for _, ref := range []GenreRef{{Name: "романы"}, {Name: "x", QID: "Q123"}} {
			ids, err := svc.ResolveOrCreateHarvestGenres(ctx, []GenreRef{ref})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(ids) != 0 || len(repo.genres) != 2 {
				t.Fatalf("ref %+v: expected skip, ids=%v genres=%d", ref, ids, len(repo.genres))
			}
		}
	})

	t.Run("alias is checked before the rejected check", func(t *testing.T) {
		// The alias itself is rejected; a published target must still be linked.
		svc, repo := newService()
		target := catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPublished}
		alias := aliasOf(target.ID, "романы", "romany", nil)
		repo.genres = []catalogDomain.Genre{target, alias}

		ids, err := svc.ResolveOrCreateHarvestGenres(ctx, []GenreRef{{Name: "романы", QID: "Q9"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 1 || ids[0] != target.ID.String() {
			t.Fatalf("expected the target linked, got %v", ids)
		}
	})

	t.Run("refs for alias and target produce one id", func(t *testing.T) {
		svc, repo := newService()
		target := catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPublished}
		alias := aliasOf(target.ID, "романы", "romany", nil)
		repo.genres = []catalogDomain.Genre{target, alias}

		ids, err := svc.ResolveOrCreateHarvestGenres(ctx, []GenreRef{{Name: "роман"}, {Name: "романы"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 1 || ids[0] != target.ID.String() {
			t.Fatalf("expected one target id, got %v", ids)
		}
	})

	t.Run("broken chain is an error", func(t *testing.T) {
		svc, repo := newService()
		final := catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPublished}
		middle := aliasOf(final.ID, "романа", "romana", nil)
		alias := aliasOf(middle.ID, "романы", "romany", nil)
		repo.genres = []catalogDomain.Genre{final, middle, alias}

		if _, err := svc.ResolveOrCreateHarvestGenres(ctx, []GenreRef{{Name: "романы"}}); err == nil {
			t.Fatal("expected an error for a two-hop alias chain")
		}
		dangling := aliasOf(uuid.New(), "повести", "povesti", nil)
		repo.genres = []catalogDomain.Genre{dangling}
		if _, err := svc.ResolveOrCreateHarvestGenres(ctx, []GenreRef{{Name: "повести"}}); err == nil {
			t.Fatal("expected an error for a missing target")
		}
	})
}

func TestResolveOrCreateGenreIDsByNames_AliasLinksTarget(t *testing.T) {
	svc, repo := newService()
	target := catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusRejected}
	alias := aliasOf(target.ID, "романы", "romany", nil)
	repo.genres = []catalogDomain.Genre{target, alias}

	ids, err := svc.ResolveOrCreateGenreIDsByNames(context.Background(), []string{"Романы", "роман"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// File import links the target whatever its status, deduped.
	if len(ids) != 1 || ids[0] != target.ID.String() || len(repo.genres) != 2 {
		t.Fatalf("expected the target linked once, got %v (genres %d)", ids, len(repo.genres))
	}
}

func TestDashboardCounts_ExcludesGenreAliases(t *testing.T) {
	svc, repo := newService()
	target := queueGenre("роман", catalogDomain.GenreStatusPublished)
	repo.genres = []catalogDomain.Genre{target, aliasOf(target.ID, "романы", "romany", nil)}
	stats, err := svc.DashboardCounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Genres != 1 || stats.PendingGenres != 0 {
		t.Fatalf("expected aliases excluded, got %+v", stats)
	}
}
