package repository

import (
	"context"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
)

func TestListWikidataBooksWithoutCover(t *testing.T) {
	repo, db := newBookStatusRepo(t)
	ctx := context.Background()
	src := "wikidata"
	other := "openlibrary"

	nullCover := insertStatusBook(t, db, catalogDomain.BookStatusPending, &src)
	emptyCover := insertStatusBook(t, db, catalogDomain.BookStatusPublished, &src)
	withCover := insertStatusBook(t, db, catalogDomain.BookStatusPublished, &src)
	rejected := insertStatusBook(t, db, catalogDomain.BookStatusRejected, &src)
	nonWikidata := insertStatusBook(t, db, catalogDomain.BookStatusPublished, &other)
	manual := insertStatusBook(t, db, catalogDomain.BookStatusPublished, nil)
	_, _, _ = rejected, nonWikidata, manual

	if err := db.Exec("UPDATE books SET cover_url = '' WHERE id = ?", emptyCover).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE books SET cover_url = 'https://example.com/c.jpg' WHERE id = ?", withCover).Error; err != nil {
		t.Fatal(err)
	}
	authorID := "11111111-1111-1111-1111-111111111111"
	if err := db.Exec("INSERT INTO authors (id, name, slug) VALUES (?, 'Автор', 'a')", authorID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)", nullCover, authorID).Error; err != nil {
		t.Fatal(err)
	}

	books, err := repo.ListWikidataBooksWithoutCover(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("got %d books, want 2 (wikidata, no cover, not rejected)", len(books))
	}
	got := map[string]catalogDomain.Book{}
	for _, b := range books {
		got[b.ID.String()] = b
	}
	if _, ok := got[nullCover.String()]; !ok {
		t.Error("NULL cover book missing")
	}
	if _, ok := got[emptyCover.String()]; !ok {
		t.Error("empty cover book missing")
	}
	if b := got[nullCover.String()]; len(b.Authors) != 1 || b.Authors[0].Name != "Автор" {
		t.Errorf("authors not preloaded: %+v", b.Authors)
	}
}
