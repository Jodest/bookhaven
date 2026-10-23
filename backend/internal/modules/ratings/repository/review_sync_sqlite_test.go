package repository

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestSyncReviewRatingInTx(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE reviews (
		id TEXT PRIMARY KEY, book_id TEXT NOT NULL, user_id TEXT NOT NULL,
		rating INTEGER NOT NULL, content TEXT NOT NULL, is_hidden BOOLEAN NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL)`).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}

	book, otherBook := uuid.New(), uuid.New()
	user, other := uuid.New(), uuid.New()
	ins := func(b, u uuid.UUID, hidden bool) {
		if err := db.Exec(`INSERT INTO reviews (id, book_id, user_id, rating, content, is_hidden, updated_at)
			VALUES (?, ?, ?, 5, 'text', ?, '2020-01-01')`, uuid.New(), b, u, hidden).Error; err != nil {
			t.Fatalf("insert review: %v", err)
		}
	}
	ins(book, user, false)
	ins(book, other, false)
	ins(otherBook, user, true)

	if err := syncReviewRatingInTx(db, book, user, 2); err != nil {
		t.Fatalf("sync: %v", err)
	}

	type row struct {
		Rating    int
		Content   string
		UpdatedAt string
	}
	get := func(b, u uuid.UUID) row {
		var r row
		if err := db.Raw(`SELECT rating, content, updated_at FROM reviews WHERE book_id = ? AND user_id = ?`, b, u).Scan(&r).Error; err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := get(book, user); r.Rating != 2 || r.Content != "text" || r.UpdatedAt != "2020-01-01" {
		t.Fatalf("own review not synced correctly: %+v", r)
	}
	if r := get(book, other); r.Rating != 5 {
		t.Fatalf("other user's review changed: %+v", r)
	}
	if r := get(otherBook, user); r.Rating != 5 {
		t.Fatalf("other book's review changed: %+v", r)
	}

	// hidden review is synced too
	if err := syncReviewRatingInTx(db, otherBook, user, 3); err != nil {
		t.Fatal(err)
	}
	if r := get(otherBook, user); r.Rating != 3 {
		t.Fatalf("hidden review not synced: %+v", r)
	}

	// no review -> no row created
	if err := syncReviewRatingInTx(db, uuid.New(), uuid.New(), 4); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Raw(`SELECT COUNT(*) FROM reviews`).Scan(&n)
	if n != 3 {
		t.Fatalf("row count = %d, want 3", n)
	}
}
