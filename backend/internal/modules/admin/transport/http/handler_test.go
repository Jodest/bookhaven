package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
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
	lastBookInput  *adminService.BookInput

	lookupRecord *bookdata.ExternalBookRecord
	lookupErr    error

	importReport     *adminService.ImportReport
	lastImportRows   []adminService.ImportRowInput
	lastImportStatus string

	bookBulkReport     *adminService.BookBulkStatusReport
	bookBulkErr        error
	bookBulkCalled     bool
	lastBookBulkIDs    []uuid.UUID
	lastBookBulkStatus string

	// Book moderation queue (handler_book_moderation_test.go).
	bookQueueItems      []adminService.BookQueueItem
	bookQueueErr        error
	lastBookQueueFilter repository.BookModerationFilter
	lastBookQueuePage   int
	lastBookQueueLimit  int

	mergeAuthorsErr           error
	mergeAuthorsBooksReturned int64
	lastMergeDuplicateID      uuid.UUID
	lastMergeCanonicalID      uuid.UUID

	lastAuthorStatusFilter string
	lastAuthorInput        *adminService.AuthorInput
	lastCreateAuthorInput  *adminService.AuthorInput

	lastGenreStatusFilter string
	lastGenreInput        *adminService.GenreInput
	genreErr              error

	getAuthorErr    error
	getAuthorCalled bool
	getGenreErr     error
	getGenreCalled  bool

	queueItems       []adminService.AuthorQueueItem
	queueErr         error
	lastQueueFilter  repository.AuthorModerationFilter
	lastQueuePage    int
	lastQueueLimit   int
	bulkStatusReport *adminService.BulkStatusReport
	bulkStatusErr    error
	bulkStatusCalled bool
	lastBulkIDs      []uuid.UUID
	lastBulkStatus   string

	genreQueueItems      []adminService.GenreQueueItem
	genreQueueErr        error
	lastGenreQueueFilter repository.GenreModerationFilter
	lastGenreQueuePage   int
	lastGenreQueueLimit  int
	genreBulkReport      *adminService.GenreBulkStatusReport
	genreBulkErr         error
	genreBulkCalled      bool
	lastGenreBulkIDs     []uuid.UUID
	lastGenreBulkStatus  string
	mergeGenresResult    *catalogDomain.Genre
	mergeGenresBooks     int64
	mergeGenresErr       error
	mergeGenresCalled    bool
	mergeGenresSwitched  int
	mergeGenresID        uuid.UUID

	// Merge journal doubles (handler_genre_merge_undo_test.go).
	mergeListPage, mergeListLimit int
	mergeListResult               *adminService.GenreMergePage
	mergeUndoPreview              *repository.GenreMergeUndoPreview
	mergeUndoOutcome              *adminService.GenreMergeUndoOutcome
	mergeUndoErr                  error
	mergeUndoActor                uuid.UUID

	mergeBatchCalled     bool
	lastMergeBatchSrc    []uuid.UUID
	lastMergeBatchTarget uuid.UUID
	mergeBatchErr        error
	mergeBatchOutcome    *adminService.GenreBatchMergeOutcome
	previewCalled        bool
	previewErr           error
	previewResult        *repository.GenreMergePreview
	lastMergeGenreSource uuid.UUID
	lastMergeGenreTarget uuid.UUID

	genreSearchResult  *adminService.GenreSearchResult
	genreSearchErr     error
	lastGenreSearchQ   string
	genreImportResult  *adminService.GenreImportResult
	genreImportErr     error
	lastGenreImportQID string

	importResult  *adminService.AuthorImportResult
	importErr     error
	lastImportQID string

	enrichReport  *adminService.AuthorEnrichmentReport
	enrichErr     error
	lastEnrichIDs []uuid.UUID

	users          []usersDomain.User
	listUsersErr   error
	lastUserSearch string
	lastUserRole   string
	lastUserPage   int
	lastUserLimit  int
	changeRoleUser *usersDomain.User
	changeRoleErr  error
	changeRoleCall bool
	lastActorID    uuid.UUID
	lastTargetID   uuid.UUID
	lastNewRole    string
}

func (f *fakeAdminService) ListUsers(ctx context.Context, search, role string, page, limit int) ([]usersDomain.User, int64, error) {
	f.lastUserSearch, f.lastUserRole, f.lastUserPage, f.lastUserLimit = search, role, page, limit
	if f.listUsersErr != nil {
		return nil, 0, f.listUsersErr
	}
	return f.users, int64(len(f.users)), nil
}

func (f *fakeAdminService) ChangeUserRole(ctx context.Context, actorID, targetID uuid.UUID, role string) (*usersDomain.User, error) {
	f.changeRoleCall = true
	f.lastActorID, f.lastTargetID, f.lastNewRole = actorID, targetID, role
	if f.changeRoleErr != nil {
		return nil, f.changeRoleErr
	}
	return f.changeRoleUser, nil
}

func (f *fakeAdminService) EnrichAuthorsFromWikidata(ctx context.Context, ids []uuid.UUID) (*adminService.AuthorEnrichmentReport, error) {
	f.lastEnrichIDs = ids
	return f.enrichReport, f.enrichErr
}

func (f *fakeAdminService) SearchWikidataGenres(ctx context.Context, query string) (*adminService.GenreSearchResult, error) {
	f.lastGenreSearchQ = query
	if f.genreSearchErr != nil {
		return nil, f.genreSearchErr
	}
	if f.genreSearchResult == nil {
		return &adminService.GenreSearchResult{ClassCheck: adminService.GenreClassCheckOK}, nil
	}
	return f.genreSearchResult, nil
}

func (f *fakeAdminService) ImportWikidataGenre(ctx context.Context, qid string) (*adminService.GenreImportResult, error) {
	f.lastGenreImportQID = qid
	if f.genreImportErr != nil {
		return nil, f.genreImportErr
	}
	return f.genreImportResult, nil
}

func (f *fakeAdminService) ImportWikidataAuthor(ctx context.Context, qid string) (*adminService.AuthorImportResult, error) {
	f.lastImportQID = qid
	if f.importErr != nil {
		return nil, f.importErr
	}
	return f.importResult, nil
}

func (f *fakeAdminService) ListAuthorModerationQueue(ctx context.Context, filter repository.AuthorModerationFilter, page, limit int) ([]adminService.AuthorQueueItem, int64, error) {
	f.lastQueueFilter = filter
	f.lastQueuePage = page
	f.lastQueueLimit = limit
	if f.queueErr != nil {
		return nil, 0, f.queueErr
	}
	return f.queueItems, int64(len(f.queueItems)), nil
}

func (f *fakeAdminService) BulkSetAuthorStatus(ctx context.Context, ids []uuid.UUID, status string) (adminService.BulkStatusReport, error) {
	f.bulkStatusCalled = true
	f.lastBulkIDs = ids
	f.lastBulkStatus = status
	if f.bulkStatusErr != nil {
		return adminService.BulkStatusReport{}, f.bulkStatusErr
	}
	if f.bulkStatusReport != nil {
		return *f.bulkStatusReport, nil
	}
	return adminService.BulkStatusReport{Processed: len(ids), Updated: len(ids), Errors: []adminService.BulkStatusRowError{}}, nil
}

func (f *fakeAdminService) DashboardCounts(ctx context.Context) (repository.DashboardStats, error) {
	if f.dashboardErr != nil {
		return repository.DashboardStats{}, f.dashboardErr
	}
	return repository.DashboardStats{
		Books: 10, PendingBooks: 2, RejectedBooks: 1, Authors: 5, Genres: 4, PendingReviews: 3, PendingAuthors: 7, PendingGenres: 6,
	}, nil
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
	f.lastBookInput = &input
	if f.createBookErr != nil {
		return nil, f.createBookErr
	}
	if strings.TrimSpace(input.Title) == "" {
		return nil, adminService.ErrInvalidTitle
	}
	status := catalogDomain.BookStatusPublished
	if input.Status != nil {
		status = *input.Status
	}
	book := catalogDomain.Book{
		ID:       uuid.New(),
		Title:    input.Title,
		Slug:     "book",
		Language: input.Language,
		Status:   status,
	}
	f.books = append(f.books, book)
	return &book, nil
}

func (f *fakeAdminService) UpdateBook(ctx context.Context, id uuid.UUID, input adminService.BookInput) (*catalogDomain.Book, error) {
	f.lastBookInput = &input
	if f.updateBookErr != nil {
		return nil, f.updateBookErr
	}
	for i := range f.books {
		if f.books[i].ID == id {
			f.books[i].Title = input.Title
			if input.Status != nil {
				f.books[i].Status = *input.Status
			}
			return &f.books[i], nil
		}
	}
	return nil, adminService.ErrBookNotFound
}

func (f *fakeAdminService) ListAuthors(ctx context.Context, search, status string, page, limit int) ([]catalogDomain.Author, int64, error) {
	f.lastAuthorStatusFilter = status
	return f.authors, int64(len(f.authors)), nil
}

func (f *fakeAdminService) GetAuthor(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error) {
	f.getAuthorCalled = true
	if f.getAuthorErr != nil {
		return nil, f.getAuthorErr
	}
	for i := range f.authors {
		if f.authors[i].ID == id {
			return &f.authors[i], nil
		}
	}
	return nil, adminService.ErrAuthorNotFound
}

func (f *fakeAdminService) CreateAuthor(ctx context.Context, input adminService.AuthorInput) (*catalogDomain.Author, error) {
	if input.BirthDate != nil && *input.BirthDate == "not-a-date" {
		return nil, adminService.ErrInvalidDate
	}
	if input.DeathDate != nil && *input.DeathDate == "not-a-date" {
		return nil, adminService.ErrInvalidDate
	}
	f.lastCreateAuthorInput = &input
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
	f.lastAuthorInput = &input
	if input.Status != nil && !catalogDomain.IsValidAuthorStatus(*input.Status) {
		return nil, adminService.ErrInvalidAuthorStatus
	}
	for i := range f.authors {
		if f.authors[i].ID == id {
			if input.AutoHarvest != nil && *input.AutoHarvest && f.authors[i].ExternalID == nil {
				return nil, adminService.ErrAutoHarvestRequiresQID
			}
			f.authors[i].Name = input.Name
			if input.Status != nil {
				f.authors[i].Status = *input.Status
			}
			if input.AutoHarvest != nil {
				f.authors[i].AutoHarvest = *input.AutoHarvest
			}
			return &f.authors[i], nil
		}
	}
	return nil, adminService.ErrAuthorNotFound
}

func (f *fakeAdminService) ListGenres(ctx context.Context, search, status string, page, limit int) ([]catalogDomain.Genre, int64, error) {
	f.lastGenreStatusFilter = status
	return f.genres, int64(len(f.genres)), nil
}

func (f *fakeAdminService) GetGenre(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error) {
	f.getGenreCalled = true
	if f.getGenreErr != nil {
		return nil, f.getGenreErr
	}
	for i := range f.genres {
		if f.genres[i].ID == id {
			return &f.genres[i], nil
		}
	}
	return nil, adminService.ErrGenreNotFound
}

func (f *fakeAdminService) CreateGenre(ctx context.Context, input adminService.GenreInput) (*catalogDomain.Genre, error) {
	f.lastGenreInput = &input
	if f.genreErr != nil {
		return nil, f.genreErr
	}
	genre := catalogDomain.Genre{ID: uuid.New(), Name: input.Name, Slug: "genre", Status: catalogDomain.GenreStatusPublished, WikidataQID: input.WikidataQID}
	f.genres = append(f.genres, genre)
	return &genre, nil
}

func (f *fakeAdminService) UpdateGenre(ctx context.Context, id uuid.UUID, input adminService.GenreInput) (*catalogDomain.Genre, error) {
	f.lastGenreInput = &input
	if f.genreErr != nil {
		return nil, f.genreErr
	}
	for i := range f.genres {
		if f.genres[i].ID == id {
			f.genres[i].Name = input.Name
			if input.Status != nil {
				f.genres[i].Status = *input.Status
			}
			if input.WikidataQID != nil {
				q := *input.WikidataQID
				f.genres[i].WikidataQID = &q
			}
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

func (f *fakeAdminService) LookupBook(ctx context.Context, isbn, query, author string) (*bookdata.ExternalBookRecord, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	return f.lookupRecord, nil
}

func (f *fakeAdminService) ImportBooks(ctx context.Context, rows []adminService.ImportRowInput, defaultStatus string) adminService.ImportReport {
	f.lastImportRows = rows
	f.lastImportStatus = defaultStatus
	if f.importReport != nil {
		return *f.importReport
	}
	return adminService.ImportReport{
		Processed:      len(rows),
		Created:        len(rows),
		Errors:         []adminService.ImportRowError{},
		CreatedAuthors: []adminService.ImportCreatedEntity{},
		CreatedGenres:  []adminService.ImportCreatedEntity{},
	}
}

func (f *fakeAdminService) BulkSetBookStatus(ctx context.Context, ids []uuid.UUID, status string) (adminService.BookBulkStatusReport, error) {
	f.bookBulkCalled = true
	f.lastBookBulkIDs = ids
	f.lastBookBulkStatus = status
	if f.bookBulkErr != nil {
		return adminService.BookBulkStatusReport{}, f.bookBulkErr
	}
	if f.bookBulkReport != nil {
		return *f.bookBulkReport, nil
	}
	return adminService.BookBulkStatusReport{Processed: len(ids), Updated: len(ids), Errors: []adminService.BookBulkStatusRowError{}}, nil
}

func (f *fakeAdminService) MergeAuthors(ctx context.Context, duplicateID, canonicalID uuid.UUID) (*catalogDomain.Author, int64, error) {
	f.lastMergeDuplicateID = duplicateID
	f.lastMergeCanonicalID = canonicalID
	if f.mergeAuthorsErr != nil {
		return nil, 0, f.mergeAuthorsErr
	}
	for i := range f.authors {
		if f.authors[i].ID == canonicalID {
			return &f.authors[i], f.mergeAuthorsBooksReturned, nil
		}
	}
	return nil, 0, adminService.ErrAuthorNotFound
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
	if body.Books != 10 || body.PendingBooks != 2 || body.RejectedBooks != 1 || body.PendingReviews != 3 || body.PendingAuthors != 7 || body.PendingGenres != 6 {
		t.Fatalf("unexpected dashboard counts: %+v", body)
	}
}

func TestListBooks_StatusFilter(t *testing.T) {
	h, svc := setupHandler()
	svc.books = []catalogDomain.Book{{ID: uuid.New(), Title: "Book", Language: "ru", Status: catalogDomain.BookStatusPending}}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/books?status=pending&visibility=hidden", nil)
	setAdminUser(c)

	if err := h.ListBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.lastBookFilter.Status != catalogDomain.BookStatusPending {
		t.Fatalf("expected status filter %q, got %q", "pending", svc.lastBookFilter.Status)
	}
}

func TestListBooks_InvalidStatus_BadRequest(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/admin/books?status=hidden", nil)
	setAdminUser(c)

	if err := h.ListBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListBooks_GenreAuthorSourceFilters(t *testing.T) {
	h, svc := setupHandler()
	svc.books = []catalogDomain.Book{{ID: uuid.New(), Title: "Book", Language: "ru"}}

	genreID := uuid.New()
	authorID := uuid.New()

	c, rec := newContext(http.MethodGet, "/api/v1/admin/books?genre_id="+genreID.String()+"&author_id="+authorID.String()+"&source=harvested", nil)
	setAdminUser(c)

	if err := h.ListBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.lastBookFilter.GenreID == nil || *svc.lastBookFilter.GenreID != genreID {
		t.Fatalf("expected genre_id filter %q, got %v", genreID, svc.lastBookFilter.GenreID)
	}
	if svc.lastBookFilter.AuthorID == nil || *svc.lastBookFilter.AuthorID != authorID {
		t.Fatalf("expected author_id filter %q, got %v", authorID, svc.lastBookFilter.AuthorID)
	}
	if svc.lastBookFilter.Source != "harvested" {
		t.Fatalf("expected source filter %q, got %q", "harvested", svc.lastBookFilter.Source)
	}
}

func TestListBooks_InvalidGenreAndAuthorID_Ignored(t *testing.T) {
	h, svc := setupHandler()
	svc.books = []catalogDomain.Book{{ID: uuid.New(), Title: "Book", Language: "ru"}}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/books?genre_id=bad-id&author_id=bad-id", nil)
	setAdminUser(c)

	if err := h.ListBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.lastBookFilter.GenreID != nil {
		t.Fatalf("expected invalid genre_id to be ignored, got %v", svc.lastBookFilter.GenreID)
	}
	if svc.lastBookFilter.AuthorID != nil {
		t.Fatalf("expected invalid author_id to be ignored, got %v", svc.lastBookFilter.AuthorID)
	}
}

func TestListBooks_Admin(t *testing.T) {
	h, svc := setupHandler()
	source := "wikidata"
	svc.books = []catalogDomain.Book{{ID: uuid.New(), Title: "Book", Language: "ru", Status: catalogDomain.BookStatusPending, ExternalSource: &source}}

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
	if body.Items[0].Status != catalogDomain.BookStatusPending {
		t.Fatalf("expected status %q, got %q", "pending", body.Items[0].Status)
	}
	if strings.Contains(rec.Body.String(), "is_hidden") {
		t.Fatalf("book response must not contain is_hidden: %s", rec.Body.String())
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

func TestCreateBook_NonRussianTitle(t *testing.T) {
	h, svc := setupHandler()
	svc.createBookErr = adminService.ErrTitleNotRussian
	c, rec := newContext(http.MethodPost, "/api/v1/admin/books", []byte(`{"title":"Book","language":"ru"}`))
	setAdminUser(c)

	if err := h.CreateBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body["error"] != adminService.ErrTitleNotRussian.Error() {
		t.Fatalf("unexpected error message: %q", body["error"])
	}
}

func TestCreateBook_AllowNonRussianTitleIsPassedThrough(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"flag true", `{"title":"Empire V","language":"ru","allow_non_russian_title":true}`, true},
		{"flag omitted", `{"title":"Empire V","language":"ru"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, svc := setupHandler()
			c, rec := newContext(http.MethodPost, "/api/v1/admin/books", []byte(tc.body))
			setAdminUser(c)

			if err := h.CreateBook(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != http.StatusCreated {
				t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
			}
			if svc.lastBookInput == nil || svc.lastBookInput.AllowNonRussianTitle != tc.want {
				t.Fatalf("expected AllowNonRussianTitle=%v, got %+v", tc.want, svc.lastBookInput)
			}
		})
	}
}

func TestUpdateBook_AllowNonRussianTitleIsPassedThrough(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"flag true", `{"title":"Empire V","language":"ru","allow_non_russian_title":true}`, true},
		{"flag omitted", `{"title":"Empire V","language":"ru"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, svc := setupHandler()
			id := uuid.New()
			svc.books = []catalogDomain.Book{{ID: id, Title: "Empire V", Slug: "empire-v", Language: "ru"}}
			c, rec := newContext(http.MethodPatch, "/api/v1/admin/books/"+id.String(), []byte(tc.body))
			c.SetPath("/api/v1/admin/books/:id")
			c.SetParamNames("id")
			c.SetParamValues(id.String())
			setAdminUser(c)

			if err := h.UpdateBook(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			if svc.lastBookInput == nil || svc.lastBookInput.AllowNonRussianTitle != tc.want {
				t.Fatalf("expected AllowNonRussianTitle=%v, got %+v", tc.want, svc.lastBookInput)
			}
		})
	}
}

func TestCreateBook_NonRussianDescription(t *testing.T) {
	h, svc := setupHandler()
	svc.createBookErr = adminService.ErrDescriptionNotRussian
	c, rec := newContext(http.MethodPost, "/api/v1/admin/books", []byte(`{"title":"Книга","language":"ru","description":"An English description"}`))
	setAdminUser(c)

	if err := h.CreateBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body["error"] != adminService.ErrDescriptionNotRussian.Error() {
		t.Fatalf("unexpected error message: %q", body["error"])
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

func TestUpdateBook_NonRussianTitle(t *testing.T) {
	h, svc := setupHandler()
	svc.updateBookErr = adminService.ErrTitleNotRussian
	c, rec := newContext(http.MethodPatch, "/api/v1/admin/books/"+uuid.New().String(), []byte(`{"title":"Updated","language":"ru"}`))
	c.SetPath("/api/v1/admin/books/:id")
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())
	setAdminUser(c)

	if err := h.UpdateBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body["error"] != adminService.ErrTitleNotRussian.Error() {
		t.Fatalf("unexpected error message: %q", body["error"])
	}
}

func TestUpdateBook_NonRussianDescription(t *testing.T) {
	h, svc := setupHandler()
	svc.updateBookErr = adminService.ErrDescriptionNotRussian
	c, rec := newContext(http.MethodPatch, "/api/v1/admin/books/"+uuid.New().String(), []byte(`{"title":"Книга","language":"ru","description":"An English description"}`))
	c.SetPath("/api/v1/admin/books/:id")
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())
	setAdminUser(c)

	if err := h.UpdateBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body["error"] != adminService.ErrDescriptionNotRussian.Error() {
		t.Fatalf("unexpected error message: %q", body["error"])
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

func TestLookupBook_Success(t *testing.T) {
	h, svc := setupHandler()
	svc.lookupRecord = &bookdata.ExternalBookRecord{Title: "War and Peace", SourceName: "openlibrary", SourceID: "OL123"}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/books/lookup?isbn=9780000000000", nil)
	setAdminUser(c)

	if err := h.LookupBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body lookupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body.Title != "War and Peace" {
		t.Errorf("Title = %q", body.Title)
	}
}

func TestLookupBook_NilAuthorsGenres_SerializeAsEmptyArraysNotNull(t *testing.T) {
	// Regression test: bookdata.ExternalBookRecord.Authors/Genres are nil
	// here (the underlying client left them unset, e.g. an Open Library
	// search doc with no "author_name"/"subject" field) - the JSON
	// response must still contain [] for both, not null, since the admin
	// lookup-assistant frontend calls .length on both unconditionally
	// (book-form.tsx crashed with "Cannot read properties of null
	// (reading 'length')" in production before this fix).
	h, svc := setupHandler()
	svc.lookupRecord = &bookdata.ExternalBookRecord{Title: "Земля обетованная", SourceName: "openlibrary", SourceID: "OL1"}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/books/lookup?isbn=9780000000000", nil)
	setAdminUser(c)

	if err := h.LookupBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"authors":[]`) {
		t.Errorf("expected \"authors\":[] in response body, got: %s", body)
	}
	if !strings.Contains(body, `"genres":[]`) {
		t.Errorf("expected \"genres\":[] in response body, got: %s", body)
	}
}

func TestLookupBook_ValidationError(t *testing.T) {
	h, svc := setupHandler()
	svc.lookupErr = adminService.ErrLookupQueryRequired

	c, rec := newContext(http.MethodGet, "/api/v1/admin/books/lookup", nil)
	setAdminUser(c)

	if err := h.LookupBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestLookupBook_NotFound(t *testing.T) {
	h, svc := setupHandler()
	svc.lookupErr = bookdata.ErrNotFound

	c, rec := newContext(http.MethodGet, "/api/v1/admin/books/lookup?isbn=0000000000", nil)
	setAdminUser(c)

	if err := h.LookupBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestMergeAuthors_Success(t *testing.T) {
	h, svc := setupHandler()
	duplicateID := uuid.New()
	canonicalID := uuid.New()
	svc.authors = []catalogDomain.Author{{ID: canonicalID, Name: "Лев Толстой", Slug: "tolstoy"}}
	svc.mergeAuthorsBooksReturned = 3

	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/"+duplicateID.String()+"/merge", []byte(`{"canonical_id":"`+canonicalID.String()+`"}`))
	c.SetPath("/api/v1/admin/authors/:id/merge")
	c.SetParamNames("id")
	c.SetParamValues(duplicateID.String())
	setAdminUser(c)

	if err := h.MergeAuthors(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastMergeDuplicateID != duplicateID || svc.lastMergeCanonicalID != canonicalID {
		t.Fatalf("expected duplicate/canonical IDs to be passed through, got dup=%v canonical=%v", svc.lastMergeDuplicateID, svc.lastMergeCanonicalID)
	}

	var body mergeAuthorsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body.BooksReassigned != 3 {
		t.Fatalf("expected books_reassigned=3, got %d", body.BooksReassigned)
	}
	if body.Author.ID != canonicalID.String() || body.Author.Name != "Лев Толстой" {
		t.Fatalf("unexpected canonical author in response: %+v", body.Author)
	}
}

func TestMergeAuthors_SelfMerge_BadRequest(t *testing.T) {
	h, svc := setupHandler()
	svc.mergeAuthorsErr = adminService.ErrCannotMergeIntoSelf
	id := uuid.New()

	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/"+id.String()+"/merge", []byte(`{"canonical_id":"`+id.String()+`"}`))
	c.SetPath("/api/v1/admin/authors/:id/merge")
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	setAdminUser(c)

	if err := h.MergeAuthors(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestMergeAuthors_NotFound(t *testing.T) {
	h, svc := setupHandler()
	svc.mergeAuthorsErr = adminService.ErrAuthorNotFound
	duplicateID := uuid.New()
	canonicalID := uuid.New()

	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/"+duplicateID.String()+"/merge", []byte(`{"canonical_id":"`+canonicalID.String()+`"}`))
	c.SetPath("/api/v1/admin/authors/:id/merge")
	c.SetParamNames("id")
	c.SetParamValues(duplicateID.String())
	setAdminUser(c)

	if err := h.MergeAuthors(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestMergeAuthors_InvalidCanonicalID_BadRequest(t *testing.T) {
	h, _ := setupHandler()
	duplicateID := uuid.New()

	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/"+duplicateID.String()+"/merge", []byte(`{"canonical_id":"not-a-uuid"}`))
	c.SetPath("/api/v1/admin/authors/:id/merge")
	c.SetParamNames("id")
	c.SetParamValues(duplicateID.String())
	setAdminUser(c)

	if err := h.MergeAuthors(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestLookupBook_SourceUnavailable(t *testing.T) {
	h, svc := setupHandler()
	svc.lookupErr = bookdata.ErrRateLimited

	c, rec := newContext(http.MethodGet, "/api/v1/admin/books/lookup?isbn=0000000000", nil)
	setAdminUser(c)

	if err := h.LookupBook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestListAuthors_StatusInResponseAndFilterPassedThrough(t *testing.T) {
	h, svc := setupHandler()
	qid := "Q7243"
	src := "wikidata"
	svc.authors = []catalogDomain.Author{{ID: uuid.New(), Name: "Лев Толстой", Slug: "lev-tolstoy", Status: catalogDomain.AuthorStatusPending, ExternalSource: &src, ExternalID: &qid}}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/authors?status=pending", nil)
	setAdminUser(c)

	if err := h.ListAuthors(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.lastAuthorStatusFilter != "pending" {
		t.Fatalf("expected status filter pending, got %q", svc.lastAuthorStatusFilter)
	}
	var body struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	item := body.Items[0]
	if item["status"] != "pending" || item["external_source"] != "wikidata" || item["external_id"] != "Q7243" {
		t.Fatalf("unexpected author item: %+v", item)
	}
}

func TestListAuthors_NullExternalIDsSerializeAsNull(t *testing.T) {
	h, svc := setupHandler()
	svc.authors = []catalogDomain.Author{{ID: uuid.New(), Name: "A", Slug: "a", Status: catalogDomain.AuthorStatusPublished}}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/authors", nil)
	setAdminUser(c)
	if err := h.ListAuthors(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rec.Body.String(), `"external_source":null`) || !strings.Contains(rec.Body.String(), `"external_id":null`) {
		t.Fatalf("expected explicit null external ids, got %s", rec.Body.String())
	}
	if svc.lastAuthorStatusFilter != "" {
		t.Fatalf("expected no status filter, got %q", svc.lastAuthorStatusFilter)
	}
}

func TestListAuthors_InvalidStatus_BadRequest(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/admin/authors?status=bogus", nil)
	setAdminUser(c)

	if err := h.ListAuthors(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateAuthor_Status(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantCode   int
		wantStatus string
		wantNil    bool
	}{
		{name: "valid status", body: `{"name":"A","status":"rejected"}`, wantCode: http.StatusOK, wantStatus: "rejected"},
		{name: "omitted status unchanged", body: `{"name":"A"}`, wantCode: http.StatusOK, wantStatus: "pending", wantNil: true},
		{name: "invalid status", body: `{"name":"A","status":"deleted"}`, wantCode: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, svc := setupHandler()
			id := uuid.New()
			svc.authors = []catalogDomain.Author{{ID: id, Name: "A", Slug: "a", Status: catalogDomain.AuthorStatusPending}}

			c, rec := newContext(http.MethodPatch, "/api/v1/admin/authors/"+id.String(), []byte(tt.body))
			c.SetPath("/api/v1/admin/authors/:id")
			c.SetParamNames("id")
			c.SetParamValues(id.String())
			setAdminUser(c)

			if err := h.UpdateAuthor(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d: %s", tt.wantCode, rec.Code, rec.Body.String())
			}
			if tt.wantNil && svc.lastAuthorInput.Status != nil {
				t.Fatalf("expected nil status input, got %q", *svc.lastAuthorInput.Status)
			}
			if tt.wantCode != http.StatusOK {
				return
			}
			var body authorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode: %v", err)
			}
			if body.Status != tt.wantStatus {
				t.Fatalf("expected status %q, got %q", tt.wantStatus, body.Status)
			}
		})
	}
}

func TestGetBook_EmbeddedAuthorsIncludeStatus(t *testing.T) {
	a := catalogDomain.Author{ID: uuid.New(), Name: "A", Slug: "a", Status: catalogDomain.AuthorStatusPending}
	resp := toBookResponse(catalogDomain.Book{ID: uuid.New(), Title: "T", Authors: []catalogDomain.Author{a}})
	if len(resp.Authors) != 1 || resp.Authors[0].Status != "pending" {
		t.Fatalf("expected embedded author status pending, got %+v", resp.Authors)
	}
}

// The admin record keeps the source link (never /covers/...) and describes the
// copy in cover_copy (#489).
func TestListBooks_CoverCopy(t *testing.T) {
	src := "https://covers.openlibrary.org/b/id/1-L.jpg"
	other := "https://example.org/x.jpg"
	path := "ab/" + strings.Repeat("ab", 32) + ".jpg"
	stored, failed, errCode := catalogDomain.CoverStatusStored, catalogDomain.CoverStatusFailed, "host_not_allowed"
	storedAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	h, svc := setupHandler()
	svc.books = []catalogDomain.Book{
		{ID: uuid.New(), Title: "Stored", Language: "ru", CoverURL: &src, CoverSourceURL: &src, CoverPath: &path, CoverStatus: &stored, CoverStoredAt: &storedAt},
		{ID: uuid.New(), Title: "Failed", Language: "ru", CoverURL: &other, CoverSourceURL: &other, CoverStatus: &failed, CoverError: &errCode},
		{ID: uuid.New(), Title: "Changed", Language: "ru", CoverURL: &other, CoverSourceURL: &src, CoverPath: &path, CoverStatus: &stored},
		{ID: uuid.New(), Title: "NoCover", Language: "ru"},
	}
	c, rec := newContext(http.MethodGet, "/api/v1/admin/books", nil)
	setAdminUser(c)
	if err := h.ListBooks(c); err != nil {
		t.Fatal(err)
	}
	var body listBooksResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rec.Body.String(), "/covers/") {
		t.Fatalf("admin record must never expose our copy path: %s", rec.Body.String())
	}
	it := body.Items
	if it[0].CoverURL == nil || *it[0].CoverURL != src || it[0].CoverCopy == nil || it[0].CoverCopy.Status != "stored" || it[0].CoverCopy.StoredAt == nil {
		t.Fatalf("stored: %+v", it[0])
	}
	if it[1].CoverCopy == nil || it[1].CoverCopy.Status != "failed" || it[1].CoverCopy.Error == nil || *it[1].CoverCopy.Error != "host_not_allowed" || it[1].CoverCopy.StoredAt != nil {
		t.Fatalf("failed: %+v", it[1].CoverCopy)
	}
	if it[2].CoverCopy == nil || it[2].CoverCopy.Status != "pending" {
		t.Fatalf("a copy of another link is pending: %+v", it[2].CoverCopy)
	}
	if it[3].CoverCopy != nil || !strings.Contains(rec.Body.String(), `"cover_copy":null`) {
		t.Fatalf("no cover: %+v", it[3].CoverCopy)
	}
}

// A link on a link-only host is shown as it is: cover_copy is linked with no
// error and no stored_at.
func TestToCoverCopy_Linked(t *testing.T) {
	link := "https://fantlab.ru/images/editions/big/77"
	linked := catalogDomain.CoverStatusLinked
	got := toCoverCopy(catalogDomain.Book{CoverURL: &link, CoverSourceURL: &link, CoverStatus: &linked})
	if got == nil || got.Status != "linked" || got.Error != nil || got.StoredAt != nil {
		t.Fatalf("linked: %+v", got)
	}
}
