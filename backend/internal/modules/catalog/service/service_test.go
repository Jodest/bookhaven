package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/catalog/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var errRepoFailure = errors.New("repo failure")

// fakeRepository implements repository.Repository backed by in-memory slices.
type fakeRepository struct {
	books     []domain.Book
	authors   []domain.Author
	genres    []domain.Genre
	languages []string

	listBooksErr     error
	getBookErr       error
	getAuthorErr     error
	listGenresErr    error
	listAuthorsErr   error
	listLanguagesErr error

	lastFilter       repository.BookFilter
	lastAuthorSearch string
	lastAuthorLimit  int

	candidates       []repository.SimilarCandidate
	similarSourceErr error
	candidatesErr    error
	candidateCalls   int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{}
}

func (f *fakeRepository) ListBooks(ctx context.Context, filter repository.BookFilter) ([]domain.Book, int64, error) {
	f.lastFilter = filter
	if f.listBooksErr != nil {
		return nil, 0, f.listBooksErr
	}
	return f.books, int64(len(f.books)), nil
}

func (f *fakeRepository) GetBookBySlug(ctx context.Context, slug string) (*domain.Book, error) {
	if f.getBookErr != nil {
		return nil, f.getBookErr
	}
	for i := range f.books {
		if f.books[i].Slug == slug {
			b := f.books[i]
			return &b, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) GetAuthorBySlug(ctx context.Context, slug string) (*domain.Author, error) {
	if f.getAuthorErr != nil {
		return nil, f.getAuthorErr
	}
	for i := range f.authors {
		if f.authors[i].Slug == slug {
			a := f.authors[i]
			return &a, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) ListGenres(ctx context.Context) ([]domain.Genre, error) {
	if f.listGenresErr != nil {
		return nil, f.listGenresErr
	}
	return f.genres, nil
}

func (f *fakeRepository) ListAuthors(ctx context.Context, search string, limit int) ([]domain.Author, int64, error) {
	f.lastAuthorSearch = search
	f.lastAuthorLimit = limit
	if f.listAuthorsErr != nil {
		return nil, 0, f.listAuthorsErr
	}
	var result []domain.Author
	for _, a := range f.authors {
		if search != "" && !strings.Contains(strings.ToLower(a.Name), strings.ToLower(search)) {
			continue
		}
		result = append(result, a)
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, int64(len(result)), nil
}

func (f *fakeRepository) ListLanguages(ctx context.Context) ([]string, error) {
	if f.listLanguagesErr != nil {
		return nil, f.listLanguagesErr
	}
	return f.languages, nil
}

func (f *fakeRepository) GetSimilarSource(ctx context.Context, bookID uuid.UUID) (*repository.SimilarSource, error) {
	if f.similarSourceErr != nil {
		return nil, f.similarSourceErr
	}
	for _, b := range f.books {
		if b.ID != bookID || b.IsHidden {
			continue
		}
		src := &repository.SimilarSource{}
		for _, a := range b.Authors {
			src.AuthorIDs = append(src.AuthorIDs, a.ID)
		}
		for _, g := range b.Genres {
			src.GenreIDs = append(src.GenreIDs, g.ID)
		}
		return src, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) ListSimilarCandidates(ctx context.Context, bookID uuid.UUID, src repository.SimilarSource, authorPool, genrePool int) ([]repository.SimilarCandidate, error) {
	f.candidateCalls++
	if f.candidatesErr != nil {
		return nil, f.candidatesErr
	}
	if len(src.AuthorIDs) == 0 && len(src.GenreIDs) == 0 {
		return nil, nil
	}
	return f.candidates, nil
}

func (f *fakeRepository) GetBooksByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Book, error) {
	want := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	var result []domain.Book
	// Iterate in reverse to make sure callers do not rely on result order.
	for i := len(f.books) - 1; i >= 0; i-- {
		b := f.books[i]
		if want[b.ID] && !b.IsHidden {
			result = append(result, b)
		}
	}
	return result, nil
}

func setupTestService() (*Service, *fakeRepository) {
	repo := newFakeRepository()
	return NewService(repo), repo
}

func TestListBooks_Success(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.books = []domain.Book{
		{ID: uuid.New(), Title: "Book A", Slug: "book-a"},
		{ID: uuid.New(), Title: "Book B", Slug: "book-b"},
	}

	filter := repository.BookFilter{Search: "test", Limit: 10}
	books, total, err := svc.ListBooks(ctx, filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(books) != 2 {
		t.Fatalf("expected 2 books, got %d", len(books))
	}
	if repo.lastFilter.Search != "test" {
		t.Fatalf("expected filter to be passed through, got %+v", repo.lastFilter)
	}
}

func TestListBooks_RepoError(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.listBooksErr = errRepoFailure

	_, _, err := svc.ListBooks(ctx, repository.BookFilter{})
	if !errors.Is(err, errRepoFailure) {
		t.Fatalf("expected wrapped errRepoFailure, got %v", err)
	}
}

func TestGetBookBySlug_Found(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.books = []domain.Book{{ID: uuid.New(), Title: "Book A", Slug: "book-a"}}

	book, err := svc.GetBookBySlug(ctx, "book-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if book.Title != "Book A" {
		t.Fatalf("expected Book A, got %s", book.Title)
	}
}

func TestGetBookBySlug_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	_, err := svc.GetBookBySlug(ctx, "missing")
	if !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("expected ErrBookNotFound, got %v", err)
	}
}

func TestGetBookBySlug_OtherError(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.getBookErr = errRepoFailure

	_, err := svc.GetBookBySlug(ctx, "book-a")
	if errors.Is(err, ErrBookNotFound) {
		t.Fatal("did not expect ErrBookNotFound for a generic repo error")
	}
	if !errors.Is(err, errRepoFailure) {
		t.Fatalf("expected wrapped errRepoFailure, got %v", err)
	}
}

func TestGetAuthorBySlug_Found(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.authors = []domain.Author{{ID: uuid.New(), Name: "Author A", Slug: "author-a"}}

	author, err := svc.GetAuthorBySlug(ctx, "author-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if author.Name != "Author A" {
		t.Fatalf("expected Author A, got %s", author.Name)
	}
}

func TestGetAuthorBySlug_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	_, err := svc.GetAuthorBySlug(ctx, "missing")
	if !errors.Is(err, ErrAuthorNotFound) {
		t.Fatalf("expected ErrAuthorNotFound, got %v", err)
	}
}

func TestGetAuthorBySlug_OtherError(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.getAuthorErr = errRepoFailure

	_, err := svc.GetAuthorBySlug(ctx, "author-a")
	if errors.Is(err, ErrAuthorNotFound) {
		t.Fatal("did not expect ErrAuthorNotFound for a generic repo error")
	}
	if !errors.Is(err, errRepoFailure) {
		t.Fatalf("expected wrapped errRepoFailure, got %v", err)
	}
}

func TestListGenres_Success(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.genres = []domain.Genre{{ID: uuid.New(), Name: "Fiction", Slug: "fiction"}}

	genres, err := svc.ListGenres(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(genres) != 1 {
		t.Fatalf("expected 1 genre, got %d", len(genres))
	}
}

func TestListGenres_RepoError(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.listGenresErr = errRepoFailure

	_, err := svc.ListGenres(ctx)
	if !errors.Is(err, errRepoFailure) {
		t.Fatalf("expected wrapped errRepoFailure, got %v", err)
	}
}

func TestListAuthors_WithSearchAndLimit(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.authors = []domain.Author{
		{ID: uuid.New(), Name: "Leo Tolstoy", Slug: "leo-tolstoy"},
		{ID: uuid.New(), Name: "Fyodor Dostoevsky", Slug: "fyodor-dostoevsky"},
	}

	authors, total, err := svc.ListAuthors(ctx, "Tolstoy", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(authors) != 1 || authors[0].Name != "Leo Tolstoy" {
		t.Fatalf("expected Leo Tolstoy, got %+v", authors)
	}
	if repo.lastAuthorSearch != "Tolstoy" || repo.lastAuthorLimit != 5 {
		t.Fatalf("expected search/limit to be passed through, got %q/%d", repo.lastAuthorSearch, repo.lastAuthorLimit)
	}
}

func TestListAuthors_WithoutSearch(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.authors = []domain.Author{
		{ID: uuid.New(), Name: "Leo Tolstoy", Slug: "leo-tolstoy"},
		{ID: uuid.New(), Name: "Fyodor Dostoevsky", Slug: "fyodor-dostoevsky"},
	}

	authors, total, err := svc.ListAuthors(ctx, "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(authors) != 2 {
		t.Fatalf("expected 2 authors, got %d", len(authors))
	}
}

func TestListAuthors_RepoError(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.listAuthorsErr = errRepoFailure

	_, _, err := svc.ListAuthors(ctx, "", 20)
	if !errors.Is(err, errRepoFailure) {
		t.Fatalf("expected wrapped errRepoFailure, got %v", err)
	}
}

func TestListLanguages_Success(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.languages = []string{"en", "ru"}

	languages, err := svc.ListLanguages(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(languages) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(languages))
	}
}

func TestListLanguages_RepoError(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	repo.listLanguagesErr = errRepoFailure

	_, err := svc.ListLanguages(ctx)
	if !errors.Is(err, errRepoFailure) {
		t.Fatalf("expected wrapped errRepoFailure, got %v", err)
	}
}

func TestParseUUID_Valid(t *testing.T) {
	id := uuid.New()
	parsed, err := ParseUUID(id.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed != id {
		t.Fatalf("expected %s, got %s", id, parsed)
	}
}

func TestParseUUID_Invalid(t *testing.T) {
	_, err := ParseUUID("not-a-uuid")
	if err == nil {
		t.Fatal("expected error for invalid uuid")
	}
}
