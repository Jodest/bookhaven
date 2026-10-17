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

func TestActorFromContext(t *testing.T) {
	if got := actorFromContext(context.Background()); got != nil {
		t.Fatalf("no actor expected, got %v", got)
	}
	id := uuid.New()
	if got := actorFromContext(WithActor(context.Background(), id)); got == nil || *got != id {
		t.Fatalf("actor = %v, want %s", got, id)
	}
}

func TestMergeGenres_PassesActorAndReturnsMergeID(t *testing.T) {
	svc, repo := newService()
	a, target := queueGenre("а", "pending"), queueGenre("б", "published")
	repo.genres = []catalogDomain.Genre{a, target}
	repo.mergeJournalID = uuid.New()
	actor := uuid.New()

	_, _, _, mergeID, err := svc.MergeGenres(WithActor(context.Background(), actor), a.ID, target.ID)
	if err != nil || mergeID != repo.mergeJournalID {
		t.Fatalf("mergeID=%s err=%v", mergeID, err)
	}
	if repo.lastMergeActor == nil || *repo.lastMergeActor != actor {
		t.Fatalf("actor = %v, want %s", repo.lastMergeActor, actor)
	}

	b := queueGenre("в", "pending")
	repo.genres = append(repo.genres, b)
	out, err := svc.MergeGenresBatch(context.Background(), []uuid.UUID{b.ID}, target.ID)
	if err != nil || out.MergeID != repo.mergeJournalID {
		t.Fatalf("batch outcome %+v err=%v", out, err)
	}
	if repo.lastMergeActor != nil {
		t.Fatalf("without WithActor the actor must be nil, got %v", repo.lastMergeActor)
	}
}

func TestListGenreMerges_PagingDefaults(t *testing.T) {
	cases := []struct{ page, limit, wantPage, wantLimit int }{
		{0, 0, 1, 20},
		{3, 50, 3, 50},
		{1, 500, 1, 100},
		{-1, -5, 1, 20},
	}
	for _, tc := range cases {
		svc, repo := newService()
		got, err := svc.ListGenreMerges(context.Background(), tc.page, tc.limit)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.listMergesPage != tc.wantPage || repo.listMergesLimit != tc.wantLimit || got.Page != tc.wantPage || got.Limit != tc.wantLimit {
			t.Fatalf("page/limit = %d/%d, want %d/%d", repo.listMergesPage, repo.listMergesLimit, tc.wantPage, tc.wantLimit)
		}
	}
}

func TestUndoGenreMerge_Service(t *testing.T) {
	ctx := context.Background()
	mergeID, actor := uuid.New(), uuid.New()

	t.Run("not found and blocked are mapped", func(t *testing.T) {
		svc, repo := newService()
		repo.undoErr = gorm.ErrRecordNotFound
		if _, err := svc.UndoGenreMerge(ctx, mergeID, actor); !errors.Is(err, ErrGenreMergeNotFound) {
			t.Fatalf("undo: expected ErrGenreMergeNotFound, got %v", err)
		}
		if _, err := svc.PreviewGenreMergeUndo(ctx, mergeID); !errors.Is(err, ErrGenreMergeNotFound) {
			t.Fatalf("preview: expected ErrGenreMergeNotFound, got %v", err)
		}

		repo.undoErr = &repository.GenreMergeUndoBlockedError{Code: repository.GenreMergeUndoTargetMerged}
		var blocked *GenreMergeUndoBlockedError
		if _, err := svc.UndoGenreMerge(ctx, mergeID, actor); !errors.As(err, &blocked) || blocked.Code != "target_merged" {
			t.Fatalf("undo: expected blocked target_merged, got %v", err)
		}
		if _, err := svc.PreviewGenreMergeUndo(ctx, mergeID); !errors.As(err, &blocked) || blocked.Code != "target_merged" {
			t.Fatalf("preview: expected blocked target_merged, got %v", err)
		}
	})

	t.Run("other errors are wrapped", func(t *testing.T) {
		svc, repo := newService()
		repo.undoErr = errors.New("boom")
		_, err := svc.UndoGenreMerge(ctx, mergeID, actor)
		if err == nil || errors.Is(err, ErrGenreMergeNotFound) {
			t.Fatalf("expected a wrapped error, got %v", err)
		}
	})

	t.Run("recompute only after success, with auto ids", func(t *testing.T) {
		svc, repo := newService()
		rec := &fakeRecomputer{}
		WithCollectionRecomputer(rec)(svc)
		target := queueGenre("роман", "published")
		repo.genres = []catalogDomain.Genre{target}
		auto := uuid.New()

		repo.undoErr = &repository.GenreMergeUndoBlockedError{Code: repository.GenreMergeUndoAlreadyUndone}
		if _, err := svc.UndoGenreMerge(ctx, mergeID, actor); err == nil {
			t.Fatal("expected a blocked error")
		}
		if len(rec.calls) != 0 {
			t.Fatalf("recomputer called on failure: %v", rec.calls)
		}

		repo.undoErr = nil
		repo.undoResult = repository.GenreMergeUndoResult{
			TargetID: target.ID, SourcesRestored: 2, BooksReturned: 5, BooksRelinked: 1,
			CollectionsSwitchedBack: 3, AutoCollectionIDs: []uuid.UUID{auto},
		}
		out, err := svc.UndoGenreMerge(ctx, mergeID, actor)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Genre.ID != target.ID || out.SourcesRestored != 2 || out.BooksReturned != 5 || out.BooksRelinked != 1 || out.CollectionsSwitchedBack != 3 {
			t.Fatalf("unexpected outcome: %+v", out)
		}
		if repo.undoActor != actor {
			t.Fatalf("actor = %s, want %s", repo.undoActor, actor)
		}
		if len(rec.calls) != 1 || len(rec.calls[0]) != 1 || rec.calls[0][0] != auto {
			t.Fatalf("recomputer calls: %v", rec.calls)
		}
	})
}
