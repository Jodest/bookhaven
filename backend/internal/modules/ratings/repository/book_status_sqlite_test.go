package repository

import (
	"context"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Only a published book can be rated (docs/features/book-status-model.md
// rule 4).
func TestRatingsBookExists_OnlyPublished(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec("CREATE TABLE books (id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'pending')").Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	repo := &GormRepository{db: db}
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
