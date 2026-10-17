package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

type fakeSimilarInvalidator struct {
	calls int
	err   error
}

func (f *fakeSimilarInvalidator) Invalidate(context.Context) error {
	f.calls++
	return f.err
}

func TestBulkSetBookStatus_InvalidatesSimilar(t *testing.T) {
	svc, repo := newService()
	inv := &fakeSimilarInvalidator{}
	WithSimilarInvalidator(inv)(svc)
	b := catalogDomain.Book{ID: uuid.New(), Status: catalogDomain.BookStatusPending}
	repo.books = []catalogDomain.Book{b}

	if _, err := svc.BulkSetBookStatus(context.Background(), []uuid.UUID{b.ID}, catalogDomain.BookStatusPublished); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.calls != 1 {
		t.Fatalf("expected 1 invalidation, got %d", inv.calls)
	}

	// Nothing updated (unknown id): no invalidation.
	if _, err := svc.BulkSetBookStatus(context.Background(), []uuid.UUID{uuid.New()}, catalogDomain.BookStatusPublished); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.calls != 1 {
		t.Fatalf("expected no extra invalidation, got %d", inv.calls)
	}
}

func TestBulkSetBookStatus_InvalidatorFailureDoesNotFail(t *testing.T) {
	svc, repo := newService()
	inv := &fakeSimilarInvalidator{err: errors.New("redis down")}
	WithSimilarInvalidator(inv)(svc)
	b := catalogDomain.Book{ID: uuid.New(), Status: catalogDomain.BookStatusPending}
	repo.books = []catalogDomain.Book{b}

	report, err := svc.BulkSetBookStatus(context.Background(), []uuid.UUID{b.ID}, catalogDomain.BookStatusPublished)
	if err != nil || report.Updated != 1 {
		t.Fatalf("expected success despite invalidator error, got %+v err=%v", report, err)
	}
	if inv.calls != 1 {
		t.Fatalf("expected invalidator to be called, got %d", inv.calls)
	}
}

func TestMergeGenres_InvalidatesSimilar(t *testing.T) {
	svc, repo := newService()
	inv := &fakeSimilarInvalidator{err: errors.New("redis down")}
	WithSimilarInvalidator(inv)(svc)
	a, target := queueGenre("а", "pending"), queueGenre("б", "published")
	repo.genres = []catalogDomain.Genre{a, target}

	if _, _, _, _, err := svc.MergeGenres(context.Background(), a.ID, target.ID); err != nil {
		t.Fatalf("merge must succeed even when the invalidator fails: %v", err)
	}
	if inv.calls != 1 {
		t.Fatalf("expected 1 invalidation, got %d", inv.calls)
	}
}

func TestNilSimilarInvalidatorIsSafe(t *testing.T) {
	svc, _ := newService()
	svc.invalidateSimilar(context.Background())
}

// txTrackingRepo records whether a transaction callback is running.
type txTrackingRepo struct {
	*fakeImportRepository
	inTx bool
}

func (r *txTrackingRepo) WithinTransaction(ctx context.Context, fn func(repo repository.Repository) error) error {
	r.inTx = true
	defer func() { r.inTx = false }()
	return r.fakeImportRepository.WithinTransaction(ctx, fn)
}

type txAwareInvalidator struct {
	repo           *txTrackingRepo
	calls          int
	calledInsideTx bool
}

func (i *txAwareInvalidator) Invalidate(context.Context) error {
	i.calls++
	if i.repo.inTx {
		i.calledInsideTx = true
	}
	return nil
}

func TestImportBooks_InvalidatesSimilarOnceAfterCommit(t *testing.T) {
	repo := &txTrackingRepo{fakeImportRepository: &fakeImportRepository{}}
	svc := NewService(repo)
	inv := &txAwareInvalidator{repo: repo}
	WithSimilarInvalidator(inv)(svc)

	rows := []ImportRowInput{{Title: "Война и мир", Authors: []string{"Leo Tolstoy"}, Genres: []string{"Novel"}, Language: "ru"}}
	report := svc.ImportBooks(context.Background(), rows, "")
	if report.Created != 1 {
		t.Fatalf("expected 1 created, got %+v", report)
	}
	if inv.calls != 1 || inv.calledInsideTx {
		t.Fatalf("expected exactly one invalidation after commit, calls=%d insideTx=%v", inv.calls, inv.calledInsideTx)
	}
}

func TestImportBooks_NoInvalidationWhenTransactionFails(t *testing.T) {
	repo := &txTrackingRepo{fakeImportRepository: &fakeImportRepository{
		createBookErr: func(*catalogDomain.Book) error { return errors.New("boom") },
	}}
	svc := NewService(repo)
	inv := &txAwareInvalidator{repo: repo}
	WithSimilarInvalidator(inv)(svc)

	rows := []ImportRowInput{{Title: "Война и мир", Authors: []string{"Leo Tolstoy"}, Language: "ru"}}
	report := svc.ImportBooks(context.Background(), rows, "")
	if report.Created != 0 {
		t.Fatalf("expected nothing created, got %+v", report)
	}
	if inv.calls != 0 {
		t.Fatalf("expected no invalidation on rollback, got %d", inv.calls)
	}
}

func TestImportBooks_PendingRowsDoNotInvalidate(t *testing.T) {
	repo := &txTrackingRepo{fakeImportRepository: &fakeImportRepository{}}
	svc := NewService(repo)
	inv := &txAwareInvalidator{repo: repo}
	WithSimilarInvalidator(inv)(svc)

	rows := []ImportRowInput{{Title: "Война и мир", Authors: []string{"Leo Tolstoy"}, Language: "ru"}}
	svc.ImportBooks(context.Background(), rows, catalogDomain.BookStatusPending)
	if inv.calls != 0 {
		t.Fatalf("expected no invalidation for pending import, got %d", inv.calls)
	}
}
