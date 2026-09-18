package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/admin/repository"
	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// fakeAdminService implements the methods used by Handler.
type fakeAdminService struct {
	books          []catalogDomain.Book
	authors        []catalogDomain.Author
	genres         []catalogDomain.Genre
	reviews        []reviewsDomain.Review
	dashboardErr   error
	createBookErr  error
	updateBookErr  error
	lastBookFilter repository.BookFilter
}

func (f *fakeAdminService) DashboardCounts(ctx context.Context) (books, hiddenBooks, authors, genres, pendingReviews, pendingImportReview int64, err error) {
	if f.dashboardErr != nil {
		return 0, 0, 0, 0, 0, 0, f.dashboardErr
	}
	return 10, 2, 5, 4, 3, 1, nil
}

func (f *fakeAdminService) ListBooks(ctx context.Context, filter repository.BookFilter) ([]catalogDomain.Book, int64, error) {
	f.lastBookFilter = filter
	return f.books, int64(len(f.books)), nil
}

func (f *fakeAdminService) GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error) {
	for i := range f.books {
		if f.books[i].ID == id {
			return &f.books[i], nil
		}
	}
	return nil, adminService.ErrBookNotFound
}

func (f *fakeAdminService) CreateBook(ctx context.Context, input adminService.BookInput) (*catalogDomain.Book, error) {
	if f.createBookErr != nil {
		return nil, f.createBookErr
	}
	if strings.TrimSpace(input.Title) == "" {
		return nil, adminService.ErrInvalidTitle
	}
	book := catalogDomain.Book{
		ID:       uuid.New(),
		Title:    input.Title,
		Slug:     "book",
		Language: input.Language,
		IsHidden: input.IsHidden,
	}
	f.books = append(f.books, book)
	return &book, nil
}

func (f *fakeAdminService) UpdateBook(ctx context.Context, id uuid.UUID, input adminService.BookInput) (*catalogDomain.Book, error) {
	if f.updateBookErr != nil {
		return nil, f.updateBookErr
	}
	for i := range f.books {
		if f.books[i].ID == id {
			f.books[i].Title = input.Title
			f.books[i].IsHidden = input.IsHidden
			return &f.books[i], nil
		}
	}
	return nil, adminService.ErrBookNotFound
}

func (f *fakeAdminService) ListAuthors(ctx context.Context, search string, page, limit int) ([]catalogDomain.Author, int64, error) {
	return f.authors, int64(len(f.authors)), nil
}

func (f *fakeAdminService) CreateAuthor(ctx context.Context, input adminService.AuthorInput) (*catalogDomain.Author, error) {
	if input.BirthDate != nil && *input.BirthDate == "not-a-date" {
		return nil, adminService.ErrInvalidDate
	}
	if input.DeathDate != nil && *input.DeathDate == "not-a-date" {
		return nil, adminService.ErrInvalidDate
	}
	author := catalogDomain.Author{ID: uuid.New(), Name: input.Name, Slug: "author"}
	f.authors = append(f.authors, author)
	return &author, nil
}

func (f *fakeAdminService) UpdateAuthor(ctx context.Context, id uuid.UUID, input adminService.AuthorInput) (*catalogDomain.Author, error) {
	if input.BirthDate != nil && *input.BirthDate == "not-a-date" {
		return nil, adminService.ErrInvalidDate
	}
	if input.DeathDate != nil && *input.DeathDate == "not-a-date" {
		return nil, adminService.ErrInvalidDate
	}
	for i := range f.authors {
		if f.authors[i].ID == id {
			f.authors[i].Name = input.Name
			return &f.authors[i], nil
		}
	}
	return nil, adminService.ErrAuthorNotFound
}

func (f *fakeAdminService) ListGenres(ctx context.Context, search string, page, limit int) ([]catalogDomain.Genre, int64, error) {
	return f.genres, int64(len(f.genres)), nil
}

func (f *fakeAdminService) CreateGenre(ctx context.Context, input adminService.GenreInput) (*catalogDomain.Genre, error) {
	genre := catalogDomain.Genre{ID: uuid.New(), Name: input.Name, Slug: "genre"}
	f.genres = append(f.genres, genre)
	return &genre, nil
}

func (f *fakeAdminService) UpdateGenre(ctx context.Context, id uuid.UUID, input adminService.GenreInput) (*catalogDomain.Genre, error) {
	for i := range f.genres {
		if f.genres[i].ID == id {
			f.genres[i].Name = input.Name
			return &f.genres[i], nil
		}
	}
	return nil, adminService.ErrGenreNotFound
}

func (f *fakeAdminService) ListReviews(ctx context.Context, filter repository.ReviewFilter) ([]reviewsDomain.Review, int64, error) {
	return f.reviews, int64(len(f.reviews)), nil
}

func (f *fakeAdminService) ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error) {
	for i := range f.reviews {
		if f.reviews[i].ID == id {
			f.reviews[i].IsHidden = isHidden
			return &f.reviews[i], nil
		}
	}
	return nil, adminService.ErrReviewNotFound
}

func setupHandler() (*Handler, *fakeAdminService) {
	svc := &fakeAdminService{}
	return NewHandler(svc), svc
}

func newContext(method, target string, body []byte) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func setAdminUser(c echo.Context) {
	c.Set("user", &usersDomain.User{Role: usersDomain.RoleAdmin})
}

func TestDashboard_Admin(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/admin/dashboard", nil)
	setAdminUser(c)

	if err := h.Dashboard(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body dashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body.Books != 10 || body.HiddenBooks != 2 || body.PendingReviews != 3 || body.PendingImportReview != 1 {
		t.Fatalf("unexpected dashboard counts: %+v", body)
	}
}

func TestListBooks_PendingReviewVisibilityFilter(t *testing.T) {
	h, svc := setupHandler()
	svc.books = []catalogDomain.Book{{ID: uuid.New(), Title: "Book", Language: "ru"}}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/books?visibility=pending_review", nil)
	setAdminUser(c)

	if err := h.ListBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.lastBookFilter.Visibility != "pending_review" {
		t.Fatalf("expected visibility filter %q, got %q", "pending_review", svc.lastBookFilter.Visibility)
	}
}

func TestListBooks_Admin(t *testing.T) {
	h, svc := setupHandler()
	source := "wikidata"
	svc.books = []catalogDomain.Book{{ID: uuid.New(), Title: "Book", Language: "ru", IsHidden: true, ExternalSource: &source}}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/books", nil)
	setAdminUser(c)

	if err := h.ListBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body listBooksResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 book, got %d", len(body.Items))
	}
	if body.Items[0].ExternalSource == nil || *body.Items[0].ExternalSource != "wikidata" {
		t.Fatalf("expected external_source %q, got %+v", "wikidata", body.Items[0].ExternalSource)
	}
}

func TestCreateBook_ValidationError(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodPost, "/api/v1/admin/books", []byte(`{"title":"","language":"ru"}`))
	setAdminUser(c)

	if err := h.CreateBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestCreateBook_Conflict(t *testing.T) {
	h, svc := setupHandler()
	svc.createBookErr = adminService.ErrDuplicateSlug
	c, rec := newContext(http.MethodPost, "/api/v1/admin/books", []byte(`{"title":"Book","language":"ru"}`))
	setAdminUser(c)

	if err := h.CreateBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestUpdateBook_NotFound(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodPatch, "/api/v1/admin/books/"+uuid.New().String(), []byte(`{"title":"Updated","language":"ru"}`))
	c.SetPath("/api/v1/admin/books/:id")
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())
	setAdminUser(c)

	if err := h.UpdateBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCreateAuthor_InvalidDate(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors", []byte(`{"name":"Author","birth_date":"not-a-date"}`))
	setAdminUser(c)

	if err := h.CreateAuthor(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestCreateAuthor_Success(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors", []byte(`{"name":"Tolstoy"}`))
	setAdminUser(c)

	if err := h.CreateAuthor(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestCreateGenre_Success(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodPost, "/api/v1/admin/genres", []byte(`{"name":"Novel"}`))
	setAdminUser(c)

	if err := h.CreateGenre(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestModerateReview_Success(t *testing.T) {
	h, svc := setupHandler()
	reviewID := uuid.New()
	svc.reviews = []reviewsDomain.Review{{ID: reviewID, IsHidden: false}}

	c, rec := newContext(http.MethodPatch, "/api/v1/admin/reviews/"+reviewID.String(), []byte(`{"is_hidden":true}`))
	c.SetPath("/api/v1/admin/reviews/:id")
	c.SetParamNames("id")
	c.SetParamValues(reviewID.String())
	setAdminUser(c)

	if err := h.ModerateReview(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body reviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !body.IsHidden {
		t.Fatal("expected review to be hidden")
	}
}

func TestModerateReview_NotFound(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodPatch, "/api/v1/admin/reviews/"+uuid.New().String(), []byte(`{"is_hidden":true}`))
	c.SetPath("/api/v1/admin/reviews/:id")
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())
	setAdminUser(c)

	if err := h.ModerateReview(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
