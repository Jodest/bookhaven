package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeRecomputer struct {
	calls [][]uuid.UUID
}

func (f *fakeRecomputer) RecomputeCollections(_ context.Context, ids []uuid.UUID) {
	f.calls = append(f.calls, ids)
}

func TestValidateGenreMergeInput(t *testing.T) {
	ctx := context.Background()
	a, b, target := uuid.New(), uuid.New(), uuid.New()
	tooMany := make([]uuid.UUID, bulkGenreStatusCap+1)
	for i := range tooMany {
		tooMany[i] = uuid.New()
	}
	cases := map[string]struct {
		sources []uuid.UUID
		want    error
	}{
		"empty":     {nil, ErrGenreMergeNoSources},
		"duplicate": {[]uuid.UUID{a, b, a}, ErrGenreMergeDuplicateSource},
		"too large": {tooMany, ErrGenreMergeTooLarge},
		"target":    {[]uuid.UUID{a, target}, ErrCannotMergeGenreIntoSelf},
	}
	for name, tc := range cases {
		svc, repo := newService()
		if _, err := svc.MergeGenresBatch(ctx, tc.sources, target); !errors.Is(err, tc.want) {
			t.Errorf("batch %s: got %v, want %v", name, err, tc.want)
		}
		if _, err := svc.PreviewGenreMerge(ctx, tc.sources, target); !errors.Is(err, tc.want) {
			t.Errorf("preview %s: got %v, want %v", name, err, tc.want)
		}
		if repo.mergeBatchCalls != 0 || repo.previewGenreMergeCalls != 0 {
			t.Errorf("%s: repository must not be called", name)
		}
	}
}

func TestMergeGenresBatch_Service(t *testing.T) {
	ctx := context.Background()

	t.Run("not found maps and does not recompute", func(t *testing.T) {
		svc, repo := newService()
		rec := &fakeRecomputer{}
		WithCollectionRecomputer(rec)(svc)
		target := queueGenre("роман", catalogDomain.GenreStatusPublished)
		repo.genres = []catalogDomain.Genre{target}
		repo.mergeGenresAutoIDs = []uuid.UUID{uuid.New()}
		if _, err := svc.MergeGenresBatch(ctx, []uuid.UUID{uuid.New()}, target.ID); !errors.Is(err, ErrGenreNotFound) {
			t.Fatalf("expected ErrGenreNotFound, got %v", err)
		}
		repo.mergeBatchErr = errors.New("boom")
		if _, err := svc.MergeGenresBatch(ctx, []uuid.UUID{uuid.New()}, target.ID); err == nil || errors.Is(err, ErrGenreNotFound) {
			t.Fatalf("expected a plain error, got %v", err)
		}
		if len(rec.calls) != 0 {
			t.Fatal("recomputer must not be called on failure")
		}
	})

	t.Run("merges in order, recomputes auto collections, re-reads target", func(t *testing.T) {
		svc, repo := newService()
		rec := &fakeRecomputer{}
		WithCollectionRecomputer(rec)(svc)
		a := catalogDomain.Genre{ID: uuid.New(), Name: "романы", Slug: "romany", Status: "pending", WikidataQID: strPtr("Q1")}
		b := queueGenre("роман-эпопея", "pending")
		target := queueGenre("роман", catalogDomain.GenreStatusPublished)
		repo.genres = []catalogDomain.Genre{a, b, target}
		repo.mergeGenresBooks = 4
		repo.mergeGenresCollections = 2
		auto := uuid.New()
		repo.mergeGenresAutoIDs = []uuid.UUID{auto}

		out, err := svc.MergeGenresBatch(ctx, []uuid.UUID{a.ID, b.ID}, target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.SourcesMerged != 2 || out.BooksReassigned != 4 || out.CollectionsSwitched != 2 ||
			out.Genre.ID != target.ID || genreQID(*out.Genre) != "Q1" {
			t.Fatalf("unexpected outcome: %+v genre=%+v", out, out.Genre)
		}
		if len(repo.lastMergeBatchSources) != 2 || repo.lastMergeBatchSources[0] != a.ID || repo.lastMergeBatchSources[1] != b.ID {
			t.Fatalf("sources must keep request order: %v", repo.lastMergeBatchSources)
		}
		if len(rec.calls) != 1 || len(rec.calls[0]) != 1 || rec.calls[0][0] != auto {
			t.Fatalf("recomputer calls: %v", rec.calls)
		}
	})

	t.Run("nil recomputer and no auto collections are safe", func(t *testing.T) {
		svc, repo := newService()
		a, target := queueGenre("а", "pending"), queueGenre("б", "published")
		repo.genres = []catalogDomain.Genre{a, target}
		repo.mergeGenresAutoIDs = []uuid.UUID{uuid.New()}
		if _, err := svc.MergeGenresBatch(ctx, []uuid.UUID{a.ID}, target.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rec := &fakeRecomputer{}
		svc2, repo2 := newService()
		WithCollectionRecomputer(rec)(svc2)
		c, d := queueGenre("в", "pending"), queueGenre("г", "published")
		repo2.genres = []catalogDomain.Genre{c, d}
		if _, err := svc2.MergeGenresBatch(ctx, []uuid.UUID{c.ID}, d.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rec.calls) != 0 {
			t.Fatalf("no auto ids: recomputer must not be called, got %v", rec.calls)
		}
	})
}

func TestMergeGenres_RecomputesAutoCollections(t *testing.T) {
	svc, repo := newService()
	rec := &fakeRecomputer{}
	WithCollectionRecomputer(rec)(svc)
	a, target := queueGenre("а", "pending"), queueGenre("б", "published")
	repo.genres = []catalogDomain.Genre{a, target}
	repo.mergeGenresCollections = 3
	auto := uuid.New()
	repo.mergeGenresAutoIDs = []uuid.UUID{auto}

	_, _, switched, _, err := svc.MergeGenres(context.Background(), a.ID, target.ID)
	if err != nil || switched != 3 {
		t.Fatalf("switched=%d err=%v", switched, err)
	}
	if len(rec.calls) != 1 || rec.calls[0][0] != auto {
		t.Fatalf("recomputer calls: %v", rec.calls)
	}
}

func TestPreviewGenreMerge_Service(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService()
	src, target := uuid.New(), uuid.New()
	repo.previewGenreMerge = repository.GenreMergePreview{BooksReassigned: 3}

	got, err := svc.PreviewGenreMerge(ctx, []uuid.UUID{src}, target)
	if err != nil || got.BooksReassigned != 3 {
		t.Fatalf("preview: %+v %v", got, err)
	}

	repo.previewGenreMergeErr = gorm.ErrRecordNotFound
	if _, err := svc.PreviewGenreMerge(ctx, []uuid.UUID{src}, target); !errors.Is(err, ErrGenreNotFound) {
		t.Fatalf("expected ErrGenreNotFound, got %v", err)
	}
}
