package service

import (
	"context"
	"errors"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

type enqueueCall struct {
	id     uuid.UUID
	reason string
}

type fakeCoverEnqueuer struct {
	calls []enqueueCall
	err   error
	onHit func(id uuid.UUID)
}

func (f *fakeCoverEnqueuer) EnqueueMirror(_ context.Context, id uuid.UUID, reason string) error {
	f.calls = append(f.calls, enqueueCall{id, reason})
	if f.onHit != nil {
		f.onHit(id)
	}
	return f.err
}

const testCover = "https://covers.openlibrary.org/b/id/1-L.jpg"

func strp(s string) *string { return &s }

func TestCreateBook_EnqueuesCover(t *testing.T) {
	repo := &fakeRepository{authorsExist: true, genresExist: true}
	enq := &fakeCoverEnqueuer{}
	svc := NewService(repo, WithCoverEnqueuer(enq))

	book, err := svc.CreateBook(context.Background(), BookInput{Title: "Война и мир", Language: "ru", CoverURL: strp(testCover)})
	if err != nil {
		t.Fatal(err)
	}
	if len(enq.calls) != 1 || enq.calls[0].id != book.ID || enq.calls[0].reason != "save" {
		t.Fatalf("calls=%+v", enq.calls)
	}

	// No cover link, no call.
	enq.calls = nil
	if _, err := svc.CreateBook(context.Background(), BookInput{Title: "Идиот", Language: "ru"}); err != nil {
		t.Fatal(err)
	}
	if len(enq.calls) != 0 {
		t.Fatalf("unexpected calls: %+v", enq.calls)
	}
}

func TestCreateBook_EnqueueErrorDoesNotFailSave(t *testing.T) {
	repo := &fakeRepository{authorsExist: true, genresExist: true}
	enq := &fakeCoverEnqueuer{err: errors.New("redis down")}
	svc := NewService(repo, WithCoverEnqueuer(enq))

	book, err := svc.CreateBook(context.Background(), BookInput{Title: "Война и мир", Language: "ru", CoverURL: strp(testCover)})
	if err != nil || book == nil {
		t.Fatalf("save must succeed: %v", err)
	}
	if len(enq.calls) != 1 || len(repo.books) != 1 {
		t.Fatalf("calls=%d books=%d", len(enq.calls), len(repo.books))
	}
}

func TestUpdateBook_EnqueuesOnlyWithoutStoredCopy(t *testing.T) {
	id := uuid.New()
	stored := catalogDomain.Book{
		ID: id, Title: "Война и мир", Slug: "voyna-i-mir", Language: "ru", Status: catalogDomain.BookStatusPublished,
		CoverURL: strp(testCover), CoverSourceURL: strp(testCover),
		CoverStatus: strp(catalogDomain.CoverStatusStored), CoverPath: strp("ab/x.jpg"),
	}
	repo := &fakeRepository{authorsExist: true, genresExist: true, books: []catalogDomain.Book{stored}}
	enq := &fakeCoverEnqueuer{}
	svc := NewService(repo, WithCoverEnqueuer(enq))
	ctx := context.Background()
	in := BookInput{Title: "Война и мир", Language: "ru", CoverURL: strp(testCover)}

	// Same link, copy stored: nothing to queue.
	if _, err := svc.UpdateBook(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	if len(enq.calls) != 0 {
		t.Fatalf("same stored link queued: %+v", enq.calls)
	}

	// New link: queue a copy.
	in.CoverURL = strp("https://archive.org/new.jpg")
	if _, err := svc.UpdateBook(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	if len(enq.calls) != 1 || enq.calls[0].id != id {
		t.Fatalf("calls=%+v", enq.calls)
	}

	// Cleared link: nothing to copy.
	enq.calls = nil
	in.CoverURL = nil
	if _, err := svc.UpdateBook(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	if len(enq.calls) != 0 {
		t.Fatalf("cleared link queued: %+v", enq.calls)
	}
}

func TestUpdateBook_FailedCopyIsQueuedAgain(t *testing.T) {
	id := uuid.New()
	failed := catalogDomain.Book{
		ID: id, Title: "Война и мир", Slug: "voyna-i-mir", Language: "ru", Status: catalogDomain.BookStatusPublished,
		CoverURL: strp(testCover), CoverSourceURL: strp(testCover), CoverStatus: strp(catalogDomain.CoverStatusFailed),
	}
	repo := &fakeRepository{authorsExist: true, genresExist: true, books: []catalogDomain.Book{failed}}
	enq := &fakeCoverEnqueuer{}
	svc := NewService(repo, WithCoverEnqueuer(enq))
	if _, err := svc.UpdateBook(context.Background(), id, BookInput{Title: "Война и мир", Language: "ru", CoverURL: strp(testCover)}); err != nil {
		t.Fatal(err)
	}
	if len(enq.calls) != 1 {
		t.Fatalf("calls=%+v", enq.calls)
	}
}

func TestImportBooks_EnqueuesAfterCommit(t *testing.T) {
	repo := &fakeImportRepository{}
	enq := &fakeCoverEnqueuer{}
	// At enqueue time the book must already be committed (present in the repo).
	enq.onHit = func(id uuid.UUID) {
		for _, b := range repo.books {
			if b.ID == id {
				return
			}
		}
		t.Errorf("enqueued %s before the book was committed", id)
	}
	svc := NewService(repo, WithCoverEnqueuer(enq))
	rows := []ImportRowInput{
		{Title: "Война и мир", Authors: []string{"Лев Толстой"}, Language: "ru", CoverURL: strp(testCover)},
		{Title: "Идиот", Authors: []string{"Фёдор Достоевский"}, Language: "ru", CoverURL: strp("https://archive.org/b.jpg")},
		{Title: "Бесы", Authors: []string{"Фёдор Достоевский"}, Language: "ru", CoverURL: strp("https://archive.org/c.jpg")},
		{Title: "Без обложки", Authors: []string{"Автор"}, Language: "ru"},
		{Title: "Без автора", Language: "ru", CoverURL: strp("https://archive.org/d.jpg")}, // fails: no authors
	}
	report := svc.ImportBooks(context.Background(), rows, "")
	if report.Created != 4 || len(report.Errors) != 1 {
		t.Fatalf("report=%+v", report)
	}
	if len(enq.calls) != 3 {
		t.Fatalf("calls=%d", len(enq.calls))
	}
	seen := map[uuid.UUID]bool{}
	for _, c := range enq.calls {
		seen[c.id] = true
	}
	if len(seen) != 3 {
		t.Fatalf("duplicate enqueues: %+v", enq.calls)
	}
}

func TestImportBooks_RolledBackRowIsNotQueued(t *testing.T) {
	repo := &fakeImportRepository{createBookErr: func(*catalogDomain.Book) error { return errors.New("boom") }}
	enq := &fakeCoverEnqueuer{}
	svc := NewService(repo, WithCoverEnqueuer(enq))
	report := svc.ImportBooks(context.Background(), []ImportRowInput{
		{Title: "Война и мир", Authors: []string{"Лев Толстой"}, Language: "ru", CoverURL: strp(testCover)},
	}, "")
	if report.Created != 0 || len(report.Errors) != 1 || len(enq.calls) != 0 {
		t.Fatalf("report=%+v calls=%d", report, len(enq.calls))
	}
}
