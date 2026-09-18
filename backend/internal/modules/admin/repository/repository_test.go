package repository

import (
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// mockRepository re-implements the BookFilter.Visibility filtering logic
// (mirroring GormRepository.ListBooks' switch statement) for unit testing,
// following this repo's hand-rolled-fake convention (see
// catalog/repository/repository_test.go).
type mockRepository struct {
	books []catalogDomain.Book
}

func (m *mockRepository) filterBooks(filter BookFilter) []catalogDomain.Book {
	var result []catalogDomain.Book
	for _, b := range m.books {
		switch filter.Visibility {
		case "visible":
			if b.IsHidden {
				continue
			}
		case "hidden":
			if !b.IsHidden {
				continue
			}
		case "pending_review":
			if !b.IsHidden || b.ExternalSource == nil {
				continue
			}
		}
		result = append(result, b)
	}
	return result
}

func TestListBooks_PendingReviewFilter_HiddenAndImportedOnly(t *testing.T) {
	source := "wikidata"
	repo := &mockRepository{
		books: []catalogDomain.Book{
			{ID: uuid.New(), Title: "Visible", IsHidden: false},
			{ID: uuid.New(), Title: "Hidden, manual", IsHidden: true},
			{ID: uuid.New(), Title: "Hidden, imported", IsHidden: true, ExternalSource: &source},
		},
	}

	result := repo.filterBooks(BookFilter{Visibility: "pending_review"})
	if len(result) != 1 {
		t.Fatalf("expected 1 pending_review book, got %d", len(result))
	}
	if result[0].Title != "Hidden, imported" {
		t.Fatalf("expected the hidden+imported book, got %q", result[0].Title)
	}
}

func TestListBooks_HiddenFilter_IsSupersetOfPendingReview(t *testing.T) {
	source := "wikidata"
	repo := &mockRepository{
		books: []catalogDomain.Book{
			{ID: uuid.New(), Title: "Visible", IsHidden: false},
			{ID: uuid.New(), Title: "Hidden, manual", IsHidden: true},
			{ID: uuid.New(), Title: "Hidden, imported", IsHidden: true, ExternalSource: &source},
		},
	}

	result := repo.filterBooks(BookFilter{Visibility: "hidden"})
	if len(result) != 2 {
		t.Fatalf("expected 2 hidden books (superset, unchanged behavior), got %d", len(result))
	}
}

func TestListBooks_NoVisibilityFilter_ReturnsAll(t *testing.T) {
	repo := &mockRepository{
		books: []catalogDomain.Book{
			{ID: uuid.New(), Title: "A", IsHidden: false},
			{ID: uuid.New(), Title: "B", IsHidden: true},
		},
	}

	result := repo.filterBooks(BookFilter{})
	if len(result) != 2 {
		t.Fatalf("expected 2 books, got %d", len(result))
	}
}
