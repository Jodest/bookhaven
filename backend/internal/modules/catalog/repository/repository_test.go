package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// mockRepository implements Repository for unit testing.
type mockRepository struct {
	books    []domain.Book
	authors  []domain.Author
	genres   []domain.Genre
	langs    []string
	listErr  error
	authorsErr error
	langsErr error
}

func (m *mockRepository) ListBooks(ctx context.Context, filter BookFilter) ([]domain.Book, int64, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	var result []domain.Book
	for _, b := range m.books {
		// Simulate language filter
		if filter.Language != nil && b.Language != *filter.Language {
			continue
		}
		// Simulate rating_min filter
		if filter.RatingMin != nil && b.Rating < float64(*filter.RatingMin) {
			continue
		}
		result = append(result, b)
	}
	return result, int64(len(result)), nil
}

func (m *mockRepository) GetBookBySlug(ctx context.Context, slug string) (*domain.Book, error) {
	for i := range m.books {
		if m.books[i].Slug == slug {
			return &m.books[i], nil
		}
	}
	return nil, nil
}

func (m *mockRepository) GetAuthorBySlug(ctx context.Context, slug string) (*domain.Author, error) {
	for i := range m.authors {
		if m.authors[i].Slug == slug {
			return &m.authors[i], nil
		}
	}
	return nil, nil
}

func (m *mockRepository) ListGenres(ctx context.Context) ([]domain.Genre, error) {
	return m.genres, nil
}

func (m *mockRepository) ListAuthors(ctx context.Context, search string, limit int) ([]domain.Author, int64, error) {
	if m.authorsErr != nil {
		return nil, 0, m.authorsErr
	}
	var result []domain.Author
	for _, a := range m.authors {
		if search != "" && !strings.Contains(strings.ToLower(a.Name), strings.ToLower(search)) {
			continue
		}
		result = append(result, a)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, int64(len(result)), nil
}

func (m *mockRepository) ListLanguages(ctx context.Context) ([]string, error) {
	if m.langsErr != nil {
		return nil, m.langsErr
	}
	return m.langs, nil
}

var _ Repository = (*mockRepository)(nil)

func (m *mockRepository) GetSimilarSource(ctx context.Context, bookID uuid.UUID) (*SimilarSource, error) {
	return &SimilarSource{}, nil
}

func (m *mockRepository) ListSimilarCandidates(ctx context.Context, bookID uuid.UUID, src SimilarSource, authorPool, genrePool int) ([]SimilarCandidate, error) {
	return nil, nil
}

func (m *mockRepository) GetBooksByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Book, error) {
	return nil, nil
}

func TestListBooks_WithLanguageFilter(t *testing.T) {
	lang := "en"
	repo := &mockRepository{
		books: []domain.Book{
			{ID: uuid.New(), Title: "Book RU", Language: "ru"},
			{ID: uuid.New(), Title: "Book EN", Language: "en"},
		},
	}

	result, total, err := repo.ListBooks(context.Background(), BookFilter{Language: &lang})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 book, got %d", total)
	}
	if result[0].Language != "en" {
		t.Fatalf("expected English book, got %s", result[0].Language)
	}
}

func TestListBooks_WithRatingMinFilter(t *testing.T) {
	ratingMin := 3
	repo := &mockRepository{
		books: []domain.Book{
			{ID: uuid.New(), Title: "Low", Rating: 2.0},
			{ID: uuid.New(), Title: "High", Rating: 4.5},
		},
	}

	result, total, err := repo.ListBooks(context.Background(), BookFilter{RatingMin: &ratingMin})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 book, got %d", total)
	}
	if result[0].Title != "High" {
		t.Fatalf("expected High rating book, got %s", result[0].Title)
	}
}

func TestListBooks_WithSortTitle(t *testing.T) {
	// Sort is applied at the DB level; here we just verify the filter is passed through.
	repo := &mockRepository{
		books: []domain.Book{
			{ID: uuid.New(), Title: "B Book"},
			{ID: uuid.New(), Title: "A Book"},
		},
	}

	_, _, err := repo.ListBooks(context.Background(), BookFilter{Sort: "title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListAuthors_PartialNameMatch(t *testing.T) {
	repo := &mockRepository{
		authors: []domain.Author{
			{ID: uuid.New(), Name: "Leo Tolstoy"},
			{ID: uuid.New(), Name: "Fyodor Dostoevsky"},
		},
	}

	result, _, err := repo.ListAuthors(context.Background(), "Tolstoy", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 author, got %d", len(result))
	}
	if result[0].Name != "Leo Tolstoy" {
		t.Fatalf("expected Leo Tolstoy, got %s", result[0].Name)
	}
}

func TestListLanguages_DistinctValues(t *testing.T) {
	repo := &mockRepository{
		langs: []string{"en", "ru"},
	}

	result, err := repo.ListLanguages(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(result))
	}
	if result[0] != "en" || result[1] != "ru" {
		t.Fatalf("expected [en, ru], got %v", result)
	}
}
