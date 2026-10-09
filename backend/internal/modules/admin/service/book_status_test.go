package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// Book status model (docs/features/book-status-model.md).

func TestCreateBook_Status(t *testing.T) {
	ctx := context.Background()

	t.Run("omitted defaults to published", func(t *testing.T) {
		svc, repo := newService()
		book, err := svc.CreateBook(ctx, BookInput{Title: "Война и мир", Language: "ru"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if book.Status != catalogDomain.BookStatusPublished || repo.books[0].Status != catalogDomain.BookStatusPublished {
			t.Fatalf("expected published, got %q", book.Status)
		}
	})

	t.Run("explicit status is kept", func(t *testing.T) {
		for _, st := range []string{catalogDomain.BookStatusPending, catalogDomain.BookStatusRejected, catalogDomain.BookStatusPublished} {
			svc, _ := newService()
			book, err := svc.CreateBook(ctx, BookInput{Title: "Война и мир", Language: "ru", Status: strPtr(st)})
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", st, err)
			}
			if book.Status != st {
				t.Fatalf("expected %q, got %q", st, book.Status)
			}
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		for _, st := range []string{"", "hidden", "Published"} {
			svc, repo := newService()
			_, err := svc.CreateBook(ctx, BookInput{Title: "Война и мир", Language: "ru", Status: strPtr(st)})
			if !errors.Is(err, ErrInvalidBookStatus) {
				t.Fatalf("%q: expected ErrInvalidBookStatus, got %v", st, err)
			}
			if len(repo.books) != 0 {
				t.Fatalf("%q: nothing must be created", st)
			}
		}
	})
}

func TestUpdateBook_Status(t *testing.T) {
	ctx := context.Background()
	setup := func() (*Service, *fakeRepository, uuid.UUID) {
		svc, repo := newService()
		id := uuid.New()
		repo.books = []catalogDomain.Book{{ID: id, Title: "Война и мир", Slug: "voyna-i-mir", Language: "ru", Status: catalogDomain.BookStatusPending}}
		return svc, repo, id
	}
	input := func(status *string) BookInput {
		return BookInput{Title: "Война и мир", Slug: "voyna-i-mir", Language: "ru", Status: status}
	}

	t.Run("omitted keeps the status", func(t *testing.T) {
		svc, _, id := setup()
		book, err := svc.UpdateBook(ctx, id, input(nil))
		if err != nil || book.Status != catalogDomain.BookStatusPending {
			t.Fatalf("expected pending unchanged, got %+v err=%v", book, err)
		}
	})

	t.Run("changed", func(t *testing.T) {
		svc, _, id := setup()
		book, err := svc.UpdateBook(ctx, id, input(strPtr(catalogDomain.BookStatusRejected)))
		if err != nil || book.Status != catalogDomain.BookStatusRejected {
			t.Fatalf("expected rejected, got %+v err=%v", book, err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		svc, repo, id := setup()
		_, err := svc.UpdateBook(ctx, id, input(strPtr("hidden")))
		if !errors.Is(err, ErrInvalidBookStatus) {
			t.Fatalf("expected ErrInvalidBookStatus, got %v", err)
		}
		if repo.books[0].Status != catalogDomain.BookStatusPending {
			t.Fatal("status must not change on an invalid value")
		}
	})
}

func TestListBooks_InvalidStatus(t *testing.T) {
	svc, _ := newService()
	if _, _, err := svc.ListBooks(context.Background(), repository.BookFilter{Status: "hidden"}); !errors.Is(err, ErrInvalidBookStatus) {
		t.Fatalf("expected ErrInvalidBookStatus, got %v", err)
	}
	if _, _, err := svc.ListBooks(context.Background(), repository.BookFilter{Status: catalogDomain.BookStatusPending}); err != nil {
		t.Fatalf("valid status: unexpected error %v", err)
	}
}

func TestBulkSetBookStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid status", func(t *testing.T) {
		svc, _ := newService()
		if _, err := svc.BulkSetBookStatus(ctx, []uuid.UUID{uuid.New()}, "hidden"); !errors.Is(err, ErrInvalidBookStatus) {
			t.Fatalf("expected ErrInvalidBookStatus, got %v", err)
		}
	})

	t.Run("too many", func(t *testing.T) {
		svc, repo := newService()
		ids := make([]uuid.UUID, bulkBookStatusCap+1)
		for i := range ids {
			ids[i] = uuid.New()
		}
		repo.books = []catalogDomain.Book{{ID: ids[0], Status: catalogDomain.BookStatusPending}}
		if _, err := svc.BulkSetBookStatus(ctx, ids, catalogDomain.BookStatusPublished); !errors.Is(err, ErrBulkStatusTooLarge) {
			t.Fatalf("expected ErrBulkStatusTooLarge, got %v", err)
		}
		if repo.books[0].Status != catalogDomain.BookStatusPending {
			t.Fatal("nothing must change when over the cap")
		}
	})

	t.Run("exactly the cap after dedupe is accepted", func(t *testing.T) {
		svc, _ := newService()
		ids := make([]uuid.UUID, 0, bulkBookStatusCap+1)
		for i := 0; i < bulkBookStatusCap; i++ {
			ids = append(ids, uuid.New())
		}
		ids = append(ids, ids[0])
		report, err := svc.BulkSetBookStatus(ctx, ids, catalogDomain.BookStatusPublished)
		if err != nil || report.Processed != bulkBookStatusCap {
			t.Fatalf("expected %d processed, got %+v err=%v", bulkBookStatusCap, report, err)
		}
	})

	t.Run("dedupe, unknown and failure are row errors", func(t *testing.T) {
		svc, repo := newService()
		title := "Название"
		ok := catalogDomain.Book{ID: uuid.New(), Title: title, Slug: "nazvanie", Language: "ru", Status: catalogDomain.BookStatusPending}
		failing := catalogDomain.Book{ID: uuid.New(), Title: "Другая", Slug: "drugaya", Status: catalogDomain.BookStatusPending}
		repo.books = []catalogDomain.Book{ok, failing}
		repo.setBookStatusErr = map[uuid.UUID]error{failing.ID: errors.New("boom")}
		unknown := uuid.New()

		report, err := svc.BulkSetBookStatus(ctx, []uuid.UUID{ok.ID, ok.ID, unknown, failing.ID}, catalogDomain.BookStatusPublished)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Processed != 3 || report.Updated != 1 || len(report.Errors) != 2 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if report.Errors[0].BookID != unknown.String() || report.Errors[0].Message != ErrBookNotFound.Error() {
			t.Fatalf("unexpected unknown-id row error: %+v", report.Errors[0])
		}
		if report.Errors[1].BookID != failing.ID.String() || report.Errors[1].Message != "boom" {
			t.Fatalf("unexpected failure row error: %+v", report.Errors[1])
		}
		if repo.books[0].Status != catalogDomain.BookStatusPublished || repo.books[0].Title != title || repo.books[0].Slug != "nazvanie" {
			t.Fatalf("only status must change, got %+v", repo.books[0])
		}
	})
}
