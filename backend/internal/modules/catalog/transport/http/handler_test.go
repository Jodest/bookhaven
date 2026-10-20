package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func (f *fakeRepository) GetSimilarSource(ctx context.Context, bookID uuid.UUID) (*repository.SimilarSource, error) {
	if f.similarSourceErr != nil {
		return nil, f.similarSourceErr
	}
	for _, b := range f.books {
		if b.ID != bookID || (b.Status != "" && !b.IsPublished()) {
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
		if want[b.ID] && (b.Status == "" || b.IsPublished()) {
			result = append(result, b)
		}
	}
	return result, nil
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
	publishedAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	repo.books = []domain.Book{{ID: uuid.New(), Title: "Book A", Slug: "book-a", Status: domain.BookStatusPublished, PublishedAt: &publishedAt}}

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
	// The first-publication time is admin-only (#245).
	if strings.Contains(rec.Body.String(), "published_at") {
		t.Fatalf("public book payload must not include published_at: %s", rec.Body.String())
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

// seedSimilarBooks adds a visible source book (one genre, one author) and n
// visible genre-only candidates, returning the source.
func seedSimilarBooks(repo *fakeRepository, n int) domain.Book {
	genre := domain.Genre{ID: uuid.New(), Name: "Фэнтези", Slug: "fantasy"}
	author := domain.Author{ID: uuid.New(), Name: "Author", Slug: "author"}
	cover := "https://example.com/cover.jpg"
	year := 1999
	src := domain.Book{ID: uuid.New(), Slug: "source", Title: "Source", Genres: []domain.Genre{genre}, Authors: []domain.Author{author}}
	repo.books = append(repo.books, src)
	for i := 0; i < n; i++ {
		b := domain.Book{
			ID: uuid.New(), Slug: fmt.Sprintf("book-%d", i), Title: fmt.Sprintf("Book %d", i),
			CoverURL: &cover, PublishedYear: &year, Rating: 4.2, RatingsCount: 1000 - i,
			Genres: []domain.Genre{genre},
		}
		repo.books = append(repo.books, b)
		repo.candidates = append(repo.candidates, repository.SimilarCandidate{
			BookID: b.ID, SharedGenres: 1, GenreCount: 1, Rating: b.Rating, RatingsCount: b.RatingsCount,
		})
	}
	return src
}

func callSimilar(t *testing.T, h *Handler, id, query string) *httptest.ResponseRecorder {
	t.Helper()
	c, rec := newContext(http.MethodGet, "/api/v1/books/"+id+"/similar"+query)
	c.SetParamNames("id")
	c.SetParamValues(id)
	if err := h.ListSimilarBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return rec
}

func TestListSimilarBooks_InvalidUUID(t *testing.T) {
	h, _ := setupHandler()
	rec := callSimilar(t, h, "not-a-uuid", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid book id") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestListSimilarBooks_UnknownBook404(t *testing.T) {
	h, _ := setupHandler()
	rec := callSimilar(t, h, uuid.New().String(), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "book not found") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestListSimilarBooks_HiddenBook404(t *testing.T) {
	h, repo := setupHandler()
	src := seedSimilarBooks(repo, 2)
	repo.books[0].Status = domain.BookStatusPending
	rec := callSimilar(t, h, src.ID.String(), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListSimilarBooks_ResponseShape(t *testing.T) {
	h, repo := setupHandler()
	src := seedSimilarBooks(repo, 2)
	rec := callSimilar(t, h, src.ID.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var raw map[string][]map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	items, ok := raw["items"]
	if !ok || len(items) != 2 {
		t.Fatalf("expected 2 items, got %v", raw)
	}
	for _, key := range []string{"id", "title", "slug", "cover_url", "published_year", "rating", "ratings_count", "authors", "genres"} {
		if _, ok := items[0][key]; !ok {
			t.Fatalf("expected key %q in item, got %v", key, items[0])
		}
	}
	if items[0]["slug"] != "book-0" {
		t.Fatalf("expected ranked first item book-0, got %v", items[0]["slug"])
	}
	if len(raw) != 1 {
		t.Fatalf("expected only the items field, got %v", raw)
	}
}

func TestListSimilarBooks_EmptyIsArrayNotNull(t *testing.T) {
	h, repo := setupHandler()
	src := domain.Book{ID: uuid.New(), Slug: "lonely"}
	repo.books = []domain.Book{src}
	rec := callSimilar(t, h, src.ID.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
		t.Fatalf("expected {\"items\":[]}, got %s", body)
	}
}

func TestListSimilarBooks_LimitDefaultAndClamp(t *testing.T) {
	cases := []struct {
		query string
		want  int
	}{
		{"", 8},
		{"?limit=abc", 8},
		{"?limit=0", 8},
		{"?limit=-1", 8},
		{"?limit=3", 3},
		{"?limit=20", 20},
		{"?limit=50", 20},
	}
	for _, tc := range cases {
		h, repo := setupHandler()
		src := seedSimilarBooks(repo, 25)
		rec := callSimilar(t, h, src.ID.String(), tc.query)
		if rec.Code != http.StatusOK {
			t.Fatalf("%q: expected 200, got %d", tc.query, rec.Code)
		}
		var body similarBooksResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%q: decode: %v", tc.query, err)
		}
		if len(body.Items) != tc.want {
			t.Fatalf("%q: expected %d items, got %d", tc.query, tc.want, len(body.Items))
		}
	}
}

func TestListSimilarBooks_RepoError500(t *testing.T) {
	h, repo := setupHandler()
	repo.similarSourceErr = fmt.Errorf("db down")
	rec := callSimilar(t, h, uuid.New().String(), "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "failed to get similar books") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestListBooks_HasAdaptationOnlyForTrue(t *testing.T) {
	for query, want := range map[string]bool{"?has_adaptation=true": true, "?has_adaptation=yes": false, "?has_adaptation=1": false, "": false} {
		h, repo := setupHandler()
		c, _ := newContext(http.MethodGet, "/api/v1/books"+query)
		if err := h.ListBooks(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.lastFilter.HasAdaptation != want {
			t.Errorf("%q: HasAdaptation = %v, want %v", query, repo.lastFilter.HasAdaptation, want)
		}
	}
}
