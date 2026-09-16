package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/catalog/repository"
	"github.com/bookhaven/backend/internal/modules/catalog/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// fakeRepository implements repository.Repository backed by in-memory slices.
type fakeRepository struct {
	books     []domain.Book
	authors   []domain.Author
	genres    []domain.Genre
	languages []string

	lastFilter       repository.BookFilter
	lastAuthorSearch string
	lastAuthorLimit  int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{}
}

func (f *fakeRepository) ListBooks(ctx context.Context, filter repository.BookFilter) ([]domain.Book, int64, error) {
	f.lastFilter = filter
	return f.books, int64(len(f.books)), nil
}

func (f *fakeRepository) GetBookBySlug(ctx context.Context, slug string) (*domain.Book, error) {
	for i := range f.books {
		if f.books[i].Slug == slug {
			b := f.books[i]
			return &b, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) GetAuthorBySlug(ctx context.Context, slug string) (*domain.Author, error) {
	for i := range f.authors {
		if f.authors[i].Slug == slug {
			a := f.authors[i]
			return &a, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) ListGenres(ctx context.Context) ([]domain.Genre, error) {
	return f.genres, nil
}

func (f *fakeRepository) ListAuthors(ctx context.Context, search string, limit int) ([]domain.Author, int64, error) {
	f.lastAuthorSearch = search
	f.lastAuthorLimit = limit
	return f.authors, int64(len(f.authors)), nil
}

func (f *fakeRepository) ListLanguages(ctx context.Context) ([]string, error) {
	return f.languages, nil
}

func setupHandler() (*Handler, *fakeRepository) {
	repo := newFakeRepository()
	return NewHandler(service.NewService(repo)), repo
}

func newContext(method, target string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestListBooks_DefaultPagination(t *testing.T) {
	h, repo := setupHandler()
	repo.books = []domain.Book{
		{ID: uuid.New(), Title: "Book A", Slug: "book-a"},
	}

	c, rec := newContext(http.MethodGet, "/api/v1/books")

	if err := h.ListBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body listBooksResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Page != 1 {
		t.Fatalf("expected default page 1, got %d", body.Page)
	}
	if body.Limit != 20 {
		t.Fatalf("expected default limit 20, got %d", body.Limit)
	}
	if body.Total != 1 {
		t.Fatalf("expected total 1, got %d", body.Total)
	}
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(body.Items))
	}
}

func TestListBooks_LimitClampedAtMax(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/books?limit=500")

	if err := h.ListBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body listBooksResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Limit != 100 {
		t.Fatalf("expected limit clamped to 100, got %d", body.Limit)
	}
}

func TestListBooks_InvalidFilterParamsIgnored(t *testing.T) {
	h, repo := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/books?genre_id=bad-id&author_id=bad-id&rating_min=oops")

	if err := h.ListBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(repo.lastFilter.GenreID) != 0 {
		t.Fatalf("expected invalid genre_id to be ignored, got %v", repo.lastFilter.GenreID)
	}
	if repo.lastFilter.AuthorID != nil {
		t.Fatalf("expected invalid author_id to be ignored, got %v", repo.lastFilter.AuthorID)
	}
	if repo.lastFilter.RatingMin != nil {
		t.Fatalf("expected invalid rating_min to be ignored, got %v", *repo.lastFilter.RatingMin)
	}
}

func TestListBooks_RatingMinOutOfRangeIgnored(t *testing.T) {
	h, repo := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/books?rating_min=6")

	if err := h.ListBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.lastFilter.RatingMin != nil {
		t.Fatalf("expected out-of-range rating_min to be ignored, got %v", *repo.lastFilter.RatingMin)
	}
}

func TestGetBook_Found(t *testing.T) {
	h, repo := setupHandler()
	repo.books = []domain.Book{{ID: uuid.New(), Title: "Book A", Slug: "book-a"}}

	c, rec := newContext(http.MethodGet, "/api/v1/books/book-a")
	c.SetPath("/api/v1/books/:slug")
	c.SetParamNames("slug")
	c.SetParamValues("book-a")

	if err := h.GetBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body bookResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Title != "Book A" {
		t.Fatalf("expected Book A, got %s", body.Title)
	}
}

func TestGetBook_NotFound(t *testing.T) {
	h, _ := setupHandler()

	c, rec := newContext(http.MethodGet, "/api/v1/books/missing")
	c.SetPath("/api/v1/books/:slug")
	c.SetParamNames("slug")
	c.SetParamValues("missing")

	if err := h.GetBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "book not found" {
		t.Fatalf("expected 'book not found' error, got %v", body["error"])
	}
}

func TestGetAuthor_Found(t *testing.T) {
	h, repo := setupHandler()
	authorID := uuid.New()
	bookID := uuid.New()
	repo.authors = []domain.Author{
		{
			ID:   authorID,
			Name: "Author A",
			Slug: "author-a",
			Books: []domain.Book{
				{ID: bookID, Title: "Book A", Slug: "book-a"},
			},
		},
	}

	c, rec := newContext(http.MethodGet, "/api/v1/authors/author-a")
	c.SetPath("/api/v1/authors/:slug")
	c.SetParamNames("slug")
	c.SetParamValues("author-a")

	if err := h.GetAuthor(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body authorDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Name != "Author A" {
		t.Fatalf("expected Author A, got %s", body.Name)
	}
	if len(body.Books) != 1 || body.Books[0].Title != "Book A" {
		t.Fatalf("expected nested book Book A, got %+v", body.Books)
	}
}

func TestGetAuthor_NotFound(t *testing.T) {
	h, _ := setupHandler()

	c, rec := newContext(http.MethodGet, "/api/v1/authors/missing")
	c.SetPath("/api/v1/authors/:slug")
	c.SetParamNames("slug")
	c.SetParamValues("missing")

	if err := h.GetAuthor(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "author not found" {
		t.Fatalf("expected 'author not found' error, got %v", body["error"])
	}
}

func TestListAuthors_DefaultLimit(t *testing.T) {
	h, repo := setupHandler()
	repo.authors = []domain.Author{{ID: uuid.New(), Name: "Author A", Slug: "author-a"}}

	c, rec := newContext(http.MethodGet, "/api/v1/authors")

	if err := h.ListAuthors(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.lastAuthorLimit != 20 {
		t.Fatalf("expected default limit 20, got %d", repo.lastAuthorLimit)
	}

	var body listAuthorsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Total != 1 {
		t.Fatalf("expected total 1, got %d", body.Total)
	}
}

func TestListAuthors_SearchPassedThrough(t *testing.T) {
	h, repo := setupHandler()

	c, rec := newContext(http.MethodGet, "/api/v1/authors?search=Tolstoy")

	if err := h.ListAuthors(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.lastAuthorSearch != "Tolstoy" {
		t.Fatalf("expected search Tolstoy passed through, got %q", repo.lastAuthorSearch)
	}
}

func TestListGenres_ResponseShape(t *testing.T) {
	h, repo := setupHandler()
	repo.genres = []domain.Genre{
		{ID: uuid.New(), Name: "Fiction", Slug: "fiction"},
		{ID: uuid.New(), Name: "Poetry", Slug: "poetry"},
	}

	c, rec := newContext(http.MethodGet, "/api/v1/genres")

	if err := h.ListGenres(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(body["items"], &items); err != nil {
		t.Fatalf("failed to decode items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	var total float64
	if err := json.Unmarshal(body["total"], &total); err != nil {
		t.Fatalf("failed to decode total: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %v", total)
	}
}

func TestListLanguages_CodeMapping(t *testing.T) {
	h, repo := setupHandler()
	repo.languages = []string{"ru", "en", "fr"}

	c, rec := newContext(http.MethodGet, "/api/v1/languages")

	if err := h.ListLanguages(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body listLanguagesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.Items) != 3 {
		t.Fatalf("expected 3 languages, got %d", len(body.Items))
	}

	names := make(map[string]string, len(body.Items))
	for _, item := range body.Items {
		names[item.Code] = item.Name
	}
	if names["ru"] != "Русский" {
		t.Fatalf("expected ru -> Русский, got %q", names["ru"])
	}
	if names["en"] != "English" {
		t.Fatalf("expected en -> English, got %q", names["en"])
	}
	if names["fr"] != "fr" {
		t.Fatalf("expected unmapped code to pass through as its own name, got %q", names["fr"])
	}
}
