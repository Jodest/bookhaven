package repository

import (
	"context"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Only a published book can be reviewed (docs/features/book-status-model.md
// rule 4); review hiding is unchanged.
const reviewsBookStatusDDL = `
CREATE TABLE books (id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'pending');
CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT, avatar_url TEXT);
CREATE TABLE reviews (
	id TEXT PRIMARY KEY,
	book_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	rating INTEGER NOT NULL,
	content TEXT NOT NULL,
	likes_count INTEGER DEFAULT 0,
	is_hidden BOOLEAN NOT NULL DEFAULT FALSE,
	created_at DATETIME,
	updated_at DATETIME
);
`

func newReviewsStatusRepo(t *testing.T) (*GormRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(reviewsBookStatusDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &GormRepository{db: db}, db
}

func TestReviewsBookExists_OnlyPublished(t *testing.T) {
	repo, db := newReviewsStatusRepo(t)
	ctx := context.Background()

	published, pending, rejected := uuid.New(), uuid.New(), uuid.New()
	for id, status := range map[uuid.UUID]string{
		published: catalogDomain.BookStatusPublished,
		pending:   catalogDomain.BookStatusPending,
		rejected:  catalogDomain.BookStatusRejected,
	} {
		if err := db.Exec("INSERT INTO books (id, status) VALUES (?, ?)", id, status).Error; err != nil {
			t.Fatalf("insert book: %v", err)
		}
	}

	if ok, err := repo.BookExists(ctx, published); err != nil || !ok {
		t.Fatalf("published book must exist: %v %v", ok, err)
	}
	for _, id := range []uuid.UUID{pending, rejected, uuid.New()} {
		if ok, err := repo.BookExists(ctx, id); err != nil || ok {
			t.Fatalf("non-published/unknown book must not exist: %v %v", ok, err)
		}
	}
}

// TestListByBook_HiddenReviewStaysHidden is the review-hiding regression
// check: a hidden review on a published book is still excluded.
func TestListByBook_HiddenReviewStaysHidden(t *testing.T) {
	repo, db := newReviewsStatusRepo(t)
	book, user := uuid.New(), uuid.New()
	now := time.Now()
	if err := db.Exec("INSERT INTO books (id, status) VALUES (?, ?)", book, catalogDomain.BookStatusPublished).Error; err != nil {
		t.Fatalf("insert book: %v", err)
	}
	if err := db.Exec("INSERT INTO users (id, name) VALUES (?, 'u')", user).Error; err != nil {
		t.Fatalf("insert user: %v", err)
	}
	visible, hidden := uuid.New(), uuid.New()
	for id, isHidden := range map[uuid.UUID]bool{visible: false, hidden: true} {
		if err := db.Exec("INSERT INTO reviews (id, book_id, user_id, rating, content, is_hidden, created_at, updated_at) VALUES (?, ?, ?, 5, 'text', ?, ?, ?)",
			id, book, user, isHidden, now, now).Error; err != nil {
			t.Fatalf("insert review: %v", err)
		}
	}

	reviews, total, err := repo.ListByBook(context.Background(), book, nil, 20, 0)
	if err != nil || total != 1 || len(reviews) != 1 || reviews[0].ID != visible {
		t.Fatalf("expected only the visible review, got total=%d %+v err=%v", total, reviews, err)
	}
}
