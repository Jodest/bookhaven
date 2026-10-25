package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/repository"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Runs the real service and repository SQL against SQLite to check the
// reading-journal transition table end to end (including transactions).
const journalDDL = `
CREATE TABLE books (id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'published');
CREATE TABLE shelves (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', type TEXT NOT NULL DEFAULT 'custom', description TEXT NOT NULL DEFAULT '', is_public BOOLEAN NOT NULL DEFAULT 1, created_at DATETIME, updated_at DATETIME);
CREATE TABLE shelf_items (shelf_id TEXT NOT NULL, book_id TEXT NOT NULL, added_at DATETIME, PRIMARY KEY (shelf_id, book_id));
CREATE TABLE book_reads (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, book_id TEXT NOT NULL, started_at DATETIME, finished_at DATETIME, abandoned_at DATETIME, created_at DATETIME NOT NULL, manual_end BOOLEAN NOT NULL DEFAULT 0, CHECK (finished_at IS NULL OR abandoned_at IS NULL));
CREATE UNIQUE INDEX uq_book_reads_open ON book_reads (user_id, book_id) WHERE finished_at IS NULL AND abandoned_at IS NULL;
`

type journalEnv struct {
	t      *testing.T
	db     *gorm.DB
	svc    *Service
	user   uuid.UUID
	book   uuid.UUID
	shelf  map[domain.ShelfType]uuid.UUID
	custom uuid.UUID
}

func newJournalEnv(t *testing.T) *journalEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec(journalDDL).Error; err != nil {
		t.Fatalf("schema: %v", err)
	}
	e := &journalEnv{t: t, db: db, user: uuid.New(), book: uuid.New(), shelf: map[domain.ShelfType]uuid.UUID{}, custom: uuid.New()}
	e.svc = NewService(repository.NewGormRepository(db))
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	tick := 0
	e.svc.now = func() time.Time {
		tick++
		return base.Add(time.Duration(tick) * time.Hour)
	}
	if err := db.Exec("INSERT INTO books (id) VALUES (?)", e.book).Error; err != nil {
		t.Fatal(err)
	}
	for _, typ := range []domain.ShelfType{domain.ShelfTypeWantToRead, domain.ShelfTypeReading, domain.ShelfTypeRead, domain.ShelfTypeAbandoned} {
		id := uuid.New()
		e.shelf[typ] = id
		if err := db.Exec("INSERT INTO shelves (id, user_id, name, type) VALUES (?, ?, ?, ?)", id, e.user, string(typ), string(typ)).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("INSERT INTO shelves (id, user_id, name, type) VALUES (?, ?, 'Custom', 'custom')", e.custom, e.user).Error; err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *journalEnv) add(typ domain.ShelfType) error {
	return e.svc.AddBook(context.Background(), e.user, e.shelf[typ], e.book)
}

func (e *journalEnv) remove(typ domain.ShelfType) error {
	return e.svc.RemoveBook(context.Background(), e.user, e.shelf[typ], e.book)
}

func (e *journalEnv) count(where string) int64 {
	e.t.Helper()
	var n int64
	if err := e.db.Raw("SELECT COUNT(*) FROM book_reads WHERE user_id = ? AND book_id = ? AND "+where, e.user, e.book).Scan(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *journalEnv) total() int64 {
	var n int64
	e.db.Raw("SELECT COUNT(*) FROM book_reads").Scan(&n)
	return n
}

func (e *journalEnv) open() int64      { return e.count("finished_at IS NULL AND abandoned_at IS NULL") }
func (e *journalEnv) finished() int64  { return e.count("finished_at IS NOT NULL") }
func (e *journalEnv) abandoned() int64 { return e.count("abandoned_at IS NOT NULL") }

func (e *journalEnv) must(err error) {
	e.t.Helper()
	if err != nil {
		e.t.Fatalf("unexpected error: %v", err)
	}
}

func (e *journalEnv) finishedAtList() []string {
	var out []string
	if err := e.db.Raw("SELECT CAST(finished_at AS TEXT) FROM book_reads WHERE finished_at IS NOT NULL ORDER BY finished_at ASC").Scan(&out).Error; err != nil {
		e.t.Fatal(err)
	}
	return out
}

func TestJournal_ReadingOpensAttemptOnce(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeReading))
	if e.open() != 1 {
		t.Fatalf("expected 1 open attempt, got %d", e.open())
	}
	if err := e.add(domain.ShelfTypeReading); !errors.Is(err, ErrBookAlreadyOnShelf) {
		t.Fatalf("expected 409 sentinel, got %v", err)
	}
	if e.open() != 1 {
		t.Fatalf("still exactly 1 open attempt expected, got %d", e.open())
	}
}

func TestJournal_ReadClosesOpenAttempt(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeReading))
	e.must(e.add(domain.ShelfTypeRead))
	if e.open() != 0 || e.finished() != 1 {
		t.Fatalf("expected 0 open / 1 finished, got %d / %d", e.open(), e.finished())
	}
	if e.count("started_at IS NOT NULL AND finished_at IS NOT NULL") != 1 {
		t.Fatal("the closed attempt must keep its start date")
	}
}

func TestJournal_ReadWithoutOpenInsertsFinished(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeRead))
	if e.finished() != 1 || e.count("started_at IS NULL") != 1 {
		t.Fatal("expected one finished attempt without a start date")
	}
}

func TestJournal_DoubleAddReadKeepsOneFinished(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeRead))
	if err := e.add(domain.ShelfTypeRead); !errors.Is(err, ErrBookAlreadyOnShelf) {
		t.Fatalf("expected ErrBookAlreadyOnShelf, got %v", err)
	}
	if e.finished() != 1 {
		t.Fatalf("expected 1 finished row, got %d", e.finished())
	}
}

func TestJournal_RereadKeepsHistory(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeRead))
	e.must(e.add(domain.ShelfTypeReading)) // read again
	if e.finished() != 1 || e.open() != 1 {
		t.Fatalf("expected 1 finished + 1 open, got %d + %d", e.finished(), e.open())
	}
	sum, err := e.svc.ReadSummary(context.Background(), e.user, e.book)
	e.must(err)
	if sum.Count != 1 || !sum.Rereading || sum.LastReadAt == nil {
		t.Fatalf("unexpected summary %+v", sum)
	}
	e.must(e.add(domain.ShelfTypeRead))
	if e.finished() != 2 || e.open() != 0 {
		t.Fatalf("expected 2 finished, got %d finished %d open", e.finished(), e.open())
	}
	sum, _ = e.svc.ReadSummary(context.Background(), e.user, e.book)
	if sum.Count != 2 || sum.Rereading {
		t.Fatalf("unexpected summary %+v", sum)
	}
}

func TestJournal_AbandonedClosesOpen(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeReading))
	e.must(e.add(domain.ShelfTypeAbandoned))
	if e.open() != 0 || e.abandoned() != 1 || e.finished() != 0 {
		t.Fatalf("expected closed as abandoned, got open=%d abandoned=%d finished=%d", e.open(), e.abandoned(), e.finished())
	}
}

func TestJournal_AbandonedWithoutOpenIsNoop(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeWantToRead))
	e.must(e.add(domain.ShelfTypeAbandoned))
	if e.total() != 0 {
		t.Fatalf("expected no journal rows, got %d", e.total())
	}
}

func TestJournal_AbandonedKeepsFinishedHistory(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeRead))
	e.must(e.add(domain.ShelfTypeReading))
	e.must(e.add(domain.ShelfTypeAbandoned))
	if e.finished() != 1 || e.abandoned() != 1 || e.open() != 0 {
		t.Fatalf("got finished=%d abandoned=%d open=%d", e.finished(), e.abandoned(), e.open())
	}
}

func TestJournal_WantToReadDeletesOpen(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeReading))
	e.must(e.add(domain.ShelfTypeWantToRead))
	if e.open() != 0 {
		t.Fatal("open attempt must be deleted")
	}
}

func TestJournal_RemoveReadingDeletesOpen(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeRead))
	e.must(e.add(domain.ShelfTypeReading))
	e.must(e.remove(domain.ShelfTypeReading))
	if e.open() != 0 || e.finished() != 1 {
		t.Fatalf("got open=%d finished=%d", e.open(), e.finished())
	}
}

func TestJournal_UndoReadDeletesOnlyLatestFinished(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeRead))
	e.must(e.add(domain.ShelfTypeReading))
	e.must(e.add(domain.ShelfTypeRead))
	before := e.finishedAtList()
	e.must(e.remove(domain.ShelfTypeRead))
	after := e.finishedAtList()
	if len(before) != 2 || len(after) != 1 {
		t.Fatalf("expected 2 -> 1 finished, got %d -> %d", len(before), len(after))
	}
	if after[0] != before[0] {
		t.Fatalf("the oldest finished must survive: %v vs %v", after, before)
	}
}

func TestJournal_DoubleRemoveReadIsNotFoundAndDeletesOne(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeRead))
	e.must(e.add(domain.ShelfTypeReading))
	e.must(e.add(domain.ShelfTypeRead))
	e.must(e.remove(domain.ShelfTypeRead))
	if err := e.remove(domain.ShelfTypeRead); !errors.Is(err, ErrBookNotOnShelf) {
		t.Fatalf("expected ErrBookNotOnShelf, got %v", err)
	}
	if e.finished() != 1 {
		t.Fatalf("exactly one finished must remain, got %d", e.finished())
	}
}

func TestJournal_RemoveWantToReadAndAbandonedAreNoops(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeRead))
	e.must(e.add(domain.ShelfTypeWantToRead))
	e.must(e.remove(domain.ShelfTypeWantToRead))
	e.must(e.add(domain.ShelfTypeAbandoned))
	e.must(e.remove(domain.ShelfTypeAbandoned))
	if e.finished() != 1 {
		t.Fatalf("finished history must be untouched, got %d", e.finished())
	}
}

func TestJournal_CustomShelfIsNoop(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.svc.AddBook(context.Background(), e.user, e.custom, e.book))
	e.must(e.svc.RemoveBook(context.Background(), e.user, e.custom, e.book))
	if e.total() != 0 {
		t.Fatalf("expected no journal rows, got %d", e.total())
	}
}

func TestJournal_AtMostOneOpenRow(t *testing.T) {
	e := newJournalEnv(t)
	now := time.Now()
	repo := repository.NewGormRepository(e.db)
	ctx := context.Background()
	if err := repo.OpenRead(ctx, e.user, e.book, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.OpenRead(ctx, e.user, e.book, now); err != nil {
		t.Fatal(err)
	}
	if e.open() != 1 {
		t.Fatalf("expected 1 open row, got %d", e.open())
	}
	// The partial unique index rejects a second open row inserted directly.
	err := e.db.Exec("INSERT INTO book_reads (id, user_id, book_id, created_at) VALUES (?, ?, ?, ?)", uuid.New(), e.user, e.book, now).Error
	if err == nil {
		t.Fatal("expected unique violation for a second open attempt")
	}
}

func TestJournal_SummaryEmptyHasNullLastRead(t *testing.T) {
	e := newJournalEnv(t)
	sum, err := e.svc.ReadSummary(context.Background(), e.user, e.book)
	e.must(err)
	if sum.Count != 0 || sum.LastReadAt != nil || sum.Rereading {
		t.Fatalf("unexpected summary %+v", sum)
	}
	e.must(e.add(domain.ShelfTypeReading))
	sum, _ = e.svc.ReadSummary(context.Background(), e.user, e.book)
	if sum.Rereading {
		t.Fatal("first read in progress is not a re-read")
	}
}

func TestJournal_ReadCountsBatch(t *testing.T) {
	e := newJournalEnv(t)
	e.must(e.add(domain.ShelfTypeRead))
	e.must(e.add(domain.ShelfTypeReading))
	e.must(e.add(domain.ShelfTypeRead))
	repo := repository.NewGormRepository(e.db)
	counts, err := repo.ReadCounts(context.Background(), e.user, []uuid.UUID{e.book, uuid.New()})
	e.must(err)
	if len(counts) != 1 || counts[e.book].Count != 2 || counts[e.book].LastReadAt == nil {
		t.Fatalf("unexpected counts %+v", counts)
	}
	other, _ := repo.ReadCounts(context.Background(), uuid.New(), []uuid.UUID{e.book})
	if len(other) != 0 {
		t.Fatal("counts must be scoped to the user")
	}
}

func TestJournal_AbandonedShelfCannotBeDeleted(t *testing.T) {
	e := newJournalEnv(t)
	err := e.svc.DeleteShelf(context.Background(), e.user, e.shelf[domain.ShelfTypeAbandoned])
	if !errors.Is(err, ErrDefaultShelfImmutable) {
		t.Fatalf("expected ErrDefaultShelfImmutable, got %v", err)
	}
}
