package repository

import (
	"context"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
)

// A full-row UpdateBook (Save) must not touch the read-only cover copy columns
// set by the covers worker (#489, "->" GORM tag).
func TestUpdateBook_FullSaveKeepsCoverCopyColumns(t *testing.T) {
	repo, db := newBookStatusRepo(t)
	ctx := context.Background()
	id := insertStatusBook(t, db, catalogDomain.BookStatusPublished, nil)
	if err := db.Exec(`UPDATE books SET cover_url = 'https://covers.openlibrary.org/b/id/1-L.jpg',
		cover_source_url = 'https://covers.openlibrary.org/b/id/1-L.jpg', cover_path = 'ab/x.jpg',
		cover_status = 'stored', cover_attempts = 2 WHERE id = ?`, id).Error; err != nil {
		t.Fatal(err)
	}

	// Stale in-memory copy without any cover state, as an admin form would hold.
	var stale catalogDomain.Book
	if err := db.First(&stale, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	stale.CoverPath, stale.CoverStatus, stale.CoverSourceURL, stale.CoverAttempts = nil, nil, nil, 0
	stale.Title = "Новое название"
	if err := repo.UpdateBook(ctx, &stale, nil, nil); err != nil {
		t.Fatalf("UpdateBook: %v", err)
	}

	var got catalogDomain.Book
	if err := db.First(&got, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if got.Title != "Новое название" {
		t.Fatalf("title not saved: %q", got.Title)
	}
	if got.CoverPath == nil || *got.CoverPath != "ab/x.jpg" || got.CoverStatus == nil || *got.CoverStatus != "stored" ||
		got.CoverSourceURL == nil || got.CoverAttempts != 2 {
		t.Fatalf("cover copy columns overwritten: %+v", got)
	}
}
