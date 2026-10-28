package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const votesDDL = `
CREATE TABLE shelves (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, type TEXT NOT NULL);
CREATE TABLE shelf_items (shelf_id TEXT NOT NULL, book_id TEXT NOT NULL, added_at DATETIME, PRIMARY KEY (shelf_id, book_id));
CREATE TABLE book_reads (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, book_id TEXT NOT NULL, started_at DATETIME, finished_at DATETIME, abandoned_at DATETIME, created_at DATETIME, manual_end BOOLEAN NOT NULL DEFAULT 0);
CREATE TABLE adaptation_votes (user_id TEXT NOT NULL, book_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, value TEXT NOT NULL CHECK (value IN ('book','adaptation','same')), created_at DATETIME, updated_at DATETIME,
  PRIMARY KEY (user_id, book_id, screen_work_id),
  FOREIGN KEY (book_id, screen_work_id) REFERENCES book_screen_works (book_id, screen_work_id) ON DELETE CASCADE);
`

func setupVotes(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	svc, db := setupAuthors(t, nil, nil)
	if err := db.Exec(votesDDL).Error; err != nil {
		t.Fatal(err)
	}
	// SQLite enforces the composite cascade only with this pragma.
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	return svc, db
}

// mkLink creates a manual adaptation link of the book and returns the work id.
func mkLink(t *testing.T, svc *Service, book uuid.UUID, title string) uuid.UUID {
	t.Helper()
	res, err := svc.AddBookAdaptation(context.Background(), book, nil, newWork(title))
	if err != nil {
		t.Fatal(err)
	}
	return res.Item.ID
}

// mkShelfUser creates a user with the given shelf type holding the book.
func mkShelfUser(t *testing.T, db *gorm.DB, shelfType string, book uuid.UUID) uuid.UUID {
	t.Helper()
	user, shelf := uuid.New(), uuid.New()
	if err := db.Exec("INSERT INTO shelves (id, user_id, type) VALUES (?, ?, ?)", shelf, user, shelfType).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO shelf_items (shelf_id, book_id, added_at) VALUES (?, ?, ?)", shelf, book, time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if shelfType == "read" {
		// Voting eligibility comes from the reading journal.
		if err := db.Exec("INSERT INTO book_reads (id, user_id, book_id, finished_at, created_at) VALUES (?, ?, ?, ?, ?)", uuid.New(), user, book, time.Now(), time.Now()).Error; err != nil {
			t.Fatal(err)
		}
	}
	return user
}

func seedVotes(t *testing.T, db *gorm.DB, book, work uuid.UUID, value string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := db.Exec("INSERT INTO adaptation_votes (user_id, book_id, screen_work_id, value, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
			uuid.New(), book, work, value, time.Now(), time.Now()).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestVote_FirstChangeAndVerdict(t *testing.T) {
	svc, db := setupVotes(t)
	ctx := context.Background()
	book := mkBook(t, db, "published", "")
	work := mkLink(t, svc, book, "Фильм")
	user := mkShelfUser(t, db, "read", book)

	v, err := svc.Vote(ctx, user, book, work, "book")
	if err != nil || v.MyVote == nil || *v.MyVote != "book" || v.Votes != 1 || v.BookBetter == nil || *v.BookBetter != 1 {
		t.Fatalf("first = %+v, %v", v, err)
	}
	v, err = svc.Vote(ctx, user, book, work, "same")
	if err != nil || *v.MyVote != "same" || v.Votes != 1 {
		t.Fatalf("change = %+v, %v", v, err)
	}
	if _, err := svc.Vote(ctx, user, book, work, "film"); !errors.Is(err, domain.ErrVoteValue) {
		t.Errorf("invalid value: %v", err)
	}
}

func TestVote_SameValueKeepsUpdatedAt(t *testing.T) {
	svc, db := setupVotes(t)
	ctx := context.Background()
	book := mkBook(t, db, "published", "")
	work := mkLink(t, svc, book, "Фильм")
	user := mkShelfUser(t, db, "read", book)

	if _, err := svc.Vote(ctx, user, book, work, "book"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE adaptation_votes SET updated_at = ?", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	old := scalar(t, db, "SELECT updated_at FROM adaptation_votes")
	if _, err := svc.Vote(ctx, user, book, work, "book"); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, db, "SELECT updated_at FROM adaptation_votes"); got != old {
		t.Errorf("updated_at moved on an identical vote: %q != %q", got, old)
	}
	if _, err := svc.Vote(ctx, user, book, work, "same"); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, db, "SELECT updated_at FROM adaptation_votes"); got == old {
		t.Error("updated_at did not move on a changed vote")
	}
}

func TestVote_Rejections(t *testing.T) {
	svc, db := setupVotes(t)
	ctx := context.Background()
	book := mkBook(t, db, "published", "")
	work := mkLink(t, svc, book, "Фильм")
	wantToRead := mkShelfUser(t, db, "want_to_read", book)
	reader := mkShelfUser(t, db, "read", book)

	if _, err := svc.Vote(ctx, wantToRead, book, work, "book"); !errors.Is(err, domain.ErrNotReader) {
		t.Errorf("not a reader: %v", err)
	}
	if count(t, db, "adaptation_votes") != 0 {
		t.Error("vote stored for a non-reader")
	}
	if _, err := svc.Vote(ctx, reader, book, uuid.New(), "book"); !errors.Is(err, domain.ErrAdaptationNotFound) {
		t.Errorf("unlinked work: %v", err)
	}
	if _, err := svc.Vote(ctx, reader, uuid.New(), work, "book"); !errors.Is(err, domain.ErrBookNotFound) {
		t.Errorf("unknown book: %v", err)
	}
	if _, err := svc.SetHidden(ctx, book, work, true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Vote(ctx, reader, book, work, "book"); !errors.Is(err, domain.ErrAdaptationNotFound) {
		t.Errorf("hidden link: %v", err)
	}
	draft := mkBook(t, db, "pending", "")
	draftWork := mkLink(t, svc, draft, "Фильм 2")
	draftReader := mkShelfUser(t, db, "read", draft)
	if _, err := svc.Vote(ctx, draftReader, draft, draftWork, "book"); !errors.Is(err, domain.ErrBookNotFound) {
		t.Errorf("unpublished book: %v", err)
	}
	if _, err := svc.ListVerdicts(ctx, draft, nil); !errors.Is(err, domain.ErrBookNotFound) {
		t.Errorf("unpublished verdicts: %v", err)
	}
}

func TestListVerdicts_MinimumAndCanVote(t *testing.T) {
	svc, db := setupVotes(t)
	ctx := context.Background()
	book := mkBook(t, db, "published", "")
	few := mkLink(t, svc, book, "Мало")
	many := mkLink(t, svc, book, "Много")
	seedVotes(t, db, book, few, "book", 4)
	seedVotes(t, db, book, many, "book", 18)
	seedVotes(t, db, book, many, "adaptation", 3)
	seedVotes(t, db, book, many, "same", 2)
	reader := mkShelfUser(t, db, "read", book)
	if _, err := svc.Vote(ctx, reader, book, few, "adaptation"); err != nil {
		t.Fatal(err)
	}

	guest, err := svc.ListVerdicts(ctx, book, nil)
	if err != nil || guest.CanVote || len(guest.Items) != 2 {
		t.Fatalf("guest = %+v, %v", guest, err)
	}
	for _, it := range guest.Items {
		if it.MyVote != nil {
			t.Error("guest sees my_vote")
		}
		switch it.ScreenWorkID {
		case many:
			if it.Votes != 23 || it.BookBetter == nil || *it.BookBetter != 18 || *it.AdaptationBetter != 3 || *it.Same != 2 {
				t.Errorf("many = %+v", it)
			}
		case few:
			if it.Votes != 5 || it.BookBetter == nil {
				t.Errorf("few at the minimum = %+v", it)
			}
		}
	}

	other := mkShelfUser(t, db, "want_to_read", book)
	res, err := svc.ListVerdicts(ctx, book, &other)
	if err != nil || res.CanVote {
		t.Fatalf("non-reader = %+v, %v", res, err)
	}
	res, err = svc.ListVerdicts(ctx, book, &reader)
	if err != nil || !res.CanVote {
		t.Fatalf("reader = %+v, %v", res, err)
	}
	for _, it := range res.Items {
		if it.ScreenWorkID == few && (it.MyVote == nil || *it.MyVote != "adaptation") {
			t.Errorf("my_vote = %v", it.MyVote)
		}
	}
}

func TestListVerdicts_BelowMinimumHidesCounts(t *testing.T) {
	svc, db := setupVotes(t)
	book := mkBook(t, db, "published", "")
	mkLink(t, svc, book, "Фильм")
	res, err := svc.ListVerdicts(context.Background(), book, nil)
	if err != nil {
		t.Fatal(err)
	}
	it := res.Items[0]
	if it.Votes != 0 || it.BookBetter != nil || it.AdaptationBetter != nil || it.Same != nil {
		t.Fatalf("item = %+v", it)
	}
}

func TestVotes_HideKeepsDeleteRemoves(t *testing.T) {
	svc, db := setupVotes(t)
	ctx := context.Background()
	book := mkBook(t, db, "published", "")
	work := mkLink(t, svc, book, "Фильм")
	seedVotes(t, db, book, work, "book", 6)

	if _, err := svc.SetHidden(ctx, book, work, true); err != nil {
		t.Fatal(err)
	}
	if res, _ := svc.ListVerdicts(ctx, book, nil); len(res.Items) != 0 {
		t.Error("hidden link is in the verdicts")
	}
	if count(t, db, "adaptation_votes") != 6 {
		t.Fatal("hide deleted votes")
	}
	if _, err := svc.SetHidden(ctx, book, work, false); err != nil {
		t.Fatal(err)
	}
	if res, _ := svc.ListVerdicts(ctx, book, nil); len(res.Items) != 1 || res.Items[0].Votes != 6 {
		t.Errorf("after show = %+v", res.Items)
	}

	if err := svc.DeleteBookAdaptation(ctx, book, work); err != nil {
		t.Fatal(err)
	}
	if count(t, db, "adaptation_votes") != 0 {
		t.Error("deleting the link kept its votes")
	}
}

func TestVotes_KeptAfterUndoOfOnlyReadAndUnvoteIdempotent(t *testing.T) {
	svc, db := setupVotes(t)
	ctx := context.Background()
	book := mkBook(t, db, "published", "")
	work := mkLink(t, svc, book, "Фильм")
	user := mkShelfUser(t, db, "read", book)
	if _, err := svc.Vote(ctx, user, book, work, "book"); err != nil {
		t.Fatal(err)
	}

	// A re-reader moves the book to «Читаю»: the finished reading stays, so
	// the right to vote stays too.
	if err := db.Exec("DELETE FROM shelf_items").Error; err != nil {
		t.Fatal(err)
	}
	res, err := svc.ListVerdicts(ctx, book, &user)
	if err != nil || !res.CanVote || res.Items[0].Votes != 1 || res.Items[0].MyVote == nil {
		t.Fatalf("re-reader = %+v, %v", res, err)
	}
	if _, err := svc.Vote(ctx, user, book, work, "same"); err != nil {
		t.Fatalf("re-reader vote change: %v", err)
	}

	// Undo of the only finished reading removes the right to vote.
	if err := db.Exec("DELETE FROM book_reads").Error; err != nil {
		t.Fatal(err)
	}
	res, err = svc.ListVerdicts(ctx, book, &user)
	if err != nil || res.CanVote || res.Items[0].Votes != 1 || res.Items[0].MyVote == nil {
		t.Fatalf("after undo = %+v, %v", res, err)
	}
	if _, err := svc.Vote(ctx, user, book, work, "same"); !errors.Is(err, domain.ErrNotReader) {
		t.Errorf("vote change after undo: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := svc.Unvote(ctx, user, book, work); err != nil {
			t.Fatal(err)
		}
	}
	if count(t, db, "adaptation_votes") != 0 {
		t.Error("vote not removed")
	}
}

// A reading added by hand in the reading history grants the right to vote
// (no «Прочитано» shelf needed), and deleting the only reading removes it.
func TestVote_HandAddedReadGrantsAndDeleteRevokes(t *testing.T) {
	svc, db := setupVotes(t)
	ctx := context.Background()
	book := mkBook(t, db, "published", "")
	work := mkLink(t, svc, book, "Фильм")
	user := uuid.New()

	if _, err := svc.Vote(ctx, user, book, work, "book"); !errors.Is(err, domain.ErrNotReader) {
		t.Fatalf("before the reading: %v", err)
	}
	readID := uuid.New()
	d := time.Date(2015, 4, 20, 12, 0, 0, 0, time.UTC)
	if err := db.Exec("INSERT INTO book_reads (id, user_id, book_id, finished_at, created_at, manual_end) VALUES (?, ?, ?, ?, ?, 1)", readID, user, book, d, time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Vote(ctx, user, book, work, "book"); err != nil {
		t.Fatalf("hand-added reading must grant voting: %v", err)
	}
	if err := db.Exec("DELETE FROM book_reads WHERE id = ?", readID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Vote(ctx, user, book, work, "same"); !errors.Is(err, domain.ErrNotReader) {
		t.Fatalf("after deleting the only reading: %v", err)
	}
}
