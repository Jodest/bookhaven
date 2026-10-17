package repository

import (
	"context"
	"testing"

	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const shelvesPrivacyDDL = `
CREATE TABLE books (id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'published');
CREATE TABLE shelves (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', type TEXT NOT NULL DEFAULT 'custom', description TEXT NOT NULL DEFAULT '', is_public BOOLEAN NOT NULL DEFAULT 1, created_at DATETIME, updated_at DATETIME);
CREATE TABLE shelf_items (shelf_id TEXT NOT NULL, book_id TEXT NOT NULL, added_at DATETIME, PRIMARY KEY (shelf_id, book_id));
`

// GORM skips zero values for columns that have a default; creating a private
// shelf must still store is_public = false.
func TestCreatePrivateShelfPersistsFalse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(shelvesPrivacyDDL).Error; err != nil {
		t.Fatalf("schema: %v", err)
	}
	repo := &GormRepository{db: db}
	ctx := context.Background()
	user := uuid.New()

	priv := &domain.Shelf{ID: uuid.New(), UserID: user, Name: "Тайная", Type: domain.ShelfTypeCustom, Description: "заметки", IsPublic: false}
	pub := &domain.Shelf{ID: uuid.New(), UserID: user, Name: "Открытая", Type: domain.ShelfTypeCustom, IsPublic: true}
	for _, s := range []*domain.Shelf{priv, pub} {
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	got, err := repo.GetByID(ctx, priv.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.IsPublic {
		t.Error("private shelf was stored as public")
	}
	if got.Description != "заметки" {
		t.Errorf("description = %q", got.Description)
	}
	gotPub, _ := repo.GetByID(ctx, pub.ID)
	if !gotPub.IsPublic {
		t.Error("public shelf stored as private")
	}

	list, err := repo.ListByUser(ctx, user)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, s := range list {
		if s.ID == priv.ID && (s.IsPublic || s.Description != "заметки") {
			t.Errorf("ListByUser lost fields: %+v", s)
		}
	}
}
