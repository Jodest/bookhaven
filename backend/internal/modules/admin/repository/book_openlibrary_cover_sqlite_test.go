package repository

import (
	"context"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
)

func TestListBooksWithOpenLibraryCover(t *testing.T) {
	repo, db := newBookStatusRepo(t)
	ctx := context.Background()
	src := "wikidata"

	ol := insertStatusBook(t, db, catalogDomain.BookStatusPending, &src)
	olManual := insertStatusBook(t, db, catalogDomain.BookStatusPublished, nil)
	other := insertStatusBook(t, db, catalogDomain.BookStatusPublished, &src)
	none := insertStatusBook(t, db, catalogDomain.BookStatusPublished, &src)
	for id, url := range map[string]string{
		ol.String():       "https://covers.openlibrary.org/b/id/9157583-L.jpg",
		olManual.String(): "https://covers.openlibrary.org/b/id/9157583-L.jpg",
		other.String():    "https://example.com/c.jpg",
	} {
		if err := db.Exec("UPDATE books SET cover_url = ? WHERE id = ?", url, id).Error; err != nil {
			t.Fatal(err)
		}
	}
	authorID := "11111111-1111-1111-1111-111111111111"
	if err := db.Exec("INSERT INTO authors (id, name, slug) VALUES (?, 'Автор', 'a')", authorID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)", ol, authorID).Error; err != nil {
		t.Fatal(err)
	}
	_ = none

	books, err := repo.ListBooksWithOpenLibraryCover(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("got %d books, want 2 (any status/source with an OL cover-id link)", len(books))
	}
	for _, b := range books {
		if b.ID == ol && (len(b.Authors) != 1 || b.Authors[0].Name != "Автор") {
			t.Errorf("authors not preloaded: %+v", b.Authors)
		}
	}
}
