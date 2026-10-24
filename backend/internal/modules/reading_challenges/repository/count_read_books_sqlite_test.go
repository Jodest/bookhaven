package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const readsDDL = `
CREATE TABLE book_reads (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, book_id TEXT NOT NULL, started_at DATETIME, finished_at DATETIME, abandoned_at DATETIME, created_at DATETIME);
`

// The challenge counts distinct books finished within a calendar year from
// the reading journal, not from the "read" shelf.
func TestCountReadBooks_FromJournal(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(readsDDL).Error; err != nil {
		t.Fatal(err)
	}
	repo := &GormRepository{db: db}
	ctx := context.Background()
	user, other := uuid.New(), uuid.New()
	bookA, bookB, bookC := uuid.New(), uuid.New(), uuid.New()

	finish := func(u, b uuid.UUID, at time.Time) {
		t.Helper()
		if err := db.Exec("INSERT INTO book_reads (id, user_id, book_id, finished_at, created_at) VALUES (?, ?, ?, ?, ?)", uuid.New(), u, b, at, at).Error; err != nil {
			t.Fatal(err)
		}
	}
	count := func(u uuid.UUID, year int) int64 {
		t.Helper()
		n, err := repo.CountReadBooks(ctx, u, year)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	// A: read in 2024 and re-read in 2025 -> counts in both years.
	finish(user, bookA, time.Date(2024, 12, 31, 23, 0, 0, 0, time.UTC))
	finish(user, bookA, time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))
	// B: finished twice in 2025 -> counts once.
	finish(user, bookB, time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC))
	finish(user, bookB, time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC))
	// C: an open attempt (re-reading) and an abandoned one do not count.
	if err := db.Exec("INSERT INTO book_reads (id, user_id, book_id, started_at, created_at) VALUES (?, ?, ?, ?, ?)", uuid.New(), user, bookC, time.Now(), time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO book_reads (id, user_id, book_id, abandoned_at, created_at) VALUES (?, ?, ?, ?, ?)", uuid.New(), user, bookC, time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC), time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	// Another user's reads never leak in.
	finish(other, bookA, time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC))

	if got := count(user, 2024); got != 1 {
		t.Errorf("2024 = %d, want 1", got)
	}
	if got := count(user, 2025); got != 2 {
		t.Errorf("2025 = %d, want 2 (A and B once each)", got)
	}
	if got := count(user, 2026); got != 0 {
		t.Errorf("2026 = %d, want 0", got)
	}

	// Undo of the latest finished read of A lowers only the year it belonged to.
	if err := db.Exec("DELETE FROM book_reads WHERE user_id = ? AND book_id = ? AND finished_at >= ?", user, bookA, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)).Error; err != nil {
		t.Fatal(err)
	}
	if got := count(user, 2025); got != 1 {
		t.Errorf("2025 after undo = %d, want 1", got)
	}
	if got := count(user, 2024); got != 1 {
		t.Errorf("2024 after undo = %d, want 1", got)
	}
}
