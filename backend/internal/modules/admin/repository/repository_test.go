package repository

import (
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// mockRepository re-implements the BookFilter filtering logic (mirroring
// GormRepository.ListBooks) for unit testing,
// following this repo's hand-rolled-fake convention (see
// catalog/repository/repository_test.go).
type mockRepository struct {
	books []catalogDomain.Book
}

func (m *mockRepository) filterBooks(filter BookFilter) []catalogDomain.Book {
	var result []catalogDomain.Book
	for _, b := range m.books {
		if filter.Status != "" && b.Status != filter.Status {
			continue
		}

		if filter.GenreID != nil && !hasGenre(b, *filter.GenreID) {
			continue
		}
		if filter.AuthorID != nil && !hasAuthor(b, *filter.AuthorID) {
			continue
		}

		switch filter.Source {
		case "manual":
			if b.ExternalSource != nil {
				continue
			}
		case "harvested":
			if b.ExternalSource == nil {
				continue
			}
		}

		result = append(result, b)
	}
	return result
}

func hasGenre(b catalogDomain.Book, genreID uuid.UUID) bool {
	for _, g := range b.Genres {
		if g.ID == genreID {
			return true
		}
	}
	return false
}

func hasAuthor(b catalogDomain.Book, authorID uuid.UUID) bool {
	for _, a := range b.Authors {
		if a.ID == authorID {
			return true
		}
	}
	return false
}

func TestListBooks_PendingStatusFilter_IncludesManualAndImported(t *testing.T) {
	source := "wikidata"
	repo := &mockRepository{
		books: []catalogDomain.Book{
			{ID: uuid.New(), Title: "Published", Status: catalogDomain.BookStatusPublished},
			{ID: uuid.New(), Title: "Pending, manual", Status: catalogDomain.BookStatusPending},
			{ID: uuid.New(), Title: "Pending, imported", Status: catalogDomain.BookStatusPending, ExternalSource: &source},
			{ID: uuid.New(), Title: "Rejected", Status: catalogDomain.BookStatusRejected},
		},
	}

	result := repo.filterBooks(BookFilter{Status: catalogDomain.BookStatusPending})
	if len(result) != 2 {
		t.Fatalf("expected 2 pending books (manual + imported), got %d", len(result))
	}
}

func TestListBooks_NoStatusFilter_ReturnsAll(t *testing.T) {
	repo := &mockRepository{
		books: []catalogDomain.Book{
			{ID: uuid.New(), Title: "A", Status: catalogDomain.BookStatusPublished},
			{ID: uuid.New(), Title: "B", Status: catalogDomain.BookStatusPending},
		},
	}

	result := repo.filterBooks(BookFilter{})
	if len(result) != 2 {
		t.Fatalf("expected 2 books, got %d", len(result))
	}
}

func TestListBooks_GenreIDFilter_ReturnsOnlyMatchingBooks(t *testing.T) {
	fantasy := catalogDomain.Genre{ID: uuid.New(), Name: "Fantasy"}
	scifi := catalogDomain.Genre{ID: uuid.New(), Name: "Sci-Fi"}
	repo := &mockRepository{
		books: []catalogDomain.Book{
			{ID: uuid.New(), Title: "Fantasy Book", Genres: []catalogDomain.Genre{fantasy}},
			{ID: uuid.New(), Title: "Sci-Fi Book", Genres: []catalogDomain.Genre{scifi}},
		},
	}

	result := repo.filterBooks(BookFilter{GenreID: &fantasy.ID})
	if len(result) != 1 {
		t.Fatalf("expected 1 book matching genre, got %d", len(result))
	}
	if result[0].Title != "Fantasy Book" {
		t.Fatalf("expected Fantasy Book, got %q", result[0].Title)
	}
}

func TestListBooks_AuthorIDFilter_ReturnsOnlyMatchingBooks(t *testing.T) {
	authorA := catalogDomain.Author{ID: uuid.New(), Name: "Author A"}
	authorB := catalogDomain.Author{ID: uuid.New(), Name: "Author B"}
	repo := &mockRepository{
		books: []catalogDomain.Book{
			{ID: uuid.New(), Title: "Book A", Authors: []catalogDomain.Author{authorA}},
			{ID: uuid.New(), Title: "Book B", Authors: []catalogDomain.Author{authorB}},
		},
	}

	result := repo.filterBooks(BookFilter{AuthorID: &authorA.ID})
	if len(result) != 1 {
		t.Fatalf("expected 1 book matching author, got %d", len(result))
	}
	if result[0].Title != "Book A" {
		t.Fatalf("expected Book A, got %q", result[0].Title)
	}
}

func TestListBooks_SourceManual_ExcludesHarvestedBooks(t *testing.T) {
	source := "wikidata"
	repo := &mockRepository{
		books: []catalogDomain.Book{
			{ID: uuid.New(), Title: "Manual", ExternalSource: nil},
			{ID: uuid.New(), Title: "Harvested", ExternalSource: &source},
		},
	}

	result := repo.filterBooks(BookFilter{Source: "manual"})
	if len(result) != 1 {
		t.Fatalf("expected 1 manual book, got %d", len(result))
	}
	if result[0].Title != "Manual" {
		t.Fatalf("expected Manual, got %q", result[0].Title)
	}
}

func TestListBooks_SourceHarvested_ExcludesManualBooks(t *testing.T) {
	source := "wikidata"
	repo := &mockRepository{
		books: []catalogDomain.Book{
			{ID: uuid.New(), Title: "Manual", ExternalSource: nil},
			{ID: uuid.New(), Title: "Harvested", ExternalSource: &source},
		},
	}

	result := repo.filterBooks(BookFilter{Source: "harvested"})
	if len(result) != 1 {
		t.Fatalf("expected 1 harvested book, got %d", len(result))
	}
	if result[0].Title != "Harvested" {
		t.Fatalf("expected Harvested, got %q", result[0].Title)
	}
}

func TestListBooks_SourceHarvestedAndStatusPublished_ReturnsPublishedHarvestedBooks(t *testing.T) {
	source := "wikidata"
	repo := &mockRepository{
		books: []catalogDomain.Book{
			{ID: uuid.New(), Title: "Harvested, pending", Status: catalogDomain.BookStatusPending, ExternalSource: &source},
			{ID: uuid.New(), Title: "Harvested, published", Status: catalogDomain.BookStatusPublished, ExternalSource: &source},
			{ID: uuid.New(), Title: "Manual, published", Status: catalogDomain.BookStatusPublished},
		},
	}

	// Source and status are independent filters, ANDed together.
	result := repo.filterBooks(BookFilter{Source: "harvested", Status: catalogDomain.BookStatusPublished})
	if len(result) != 1 {
		t.Fatalf("expected 1 published harvested book, got %d", len(result))
	}
	if result[0].Title != "Harvested, published" {
		t.Fatalf("expected Harvested, published, got %q", result[0].Title)
	}
}
