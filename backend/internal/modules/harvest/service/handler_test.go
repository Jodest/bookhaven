package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	adminrepository "github.com/bookhaven/backend/internal/modules/admin/repository"
	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
)

// fakeAdminRepository implements adminrepository.Repository for unit
// testing, following this repo's hand-rolled-fake convention (see
// admin/service/service_test.go).
type fakeAdminRepository struct {
	books   []catalogDomain.Book
	authors []catalogDomain.Author
	genres  []catalogDomain.Genre
}

var _ adminrepository.Repository = (*fakeAdminRepository)(nil)

func (f *fakeAdminRepository) DashboardCounts(ctx context.Context) (books, hiddenBooks, authors, genres, pendingReviews, pendingImportReview int64, err error) {
	return int64(len(f.books)), 0, int64(len(f.authors)), int64(len(f.genres)), 0, 0, nil
}

func (f *fakeAdminRepository) ListBooks(ctx context.Context, filter adminrepository.BookFilter) ([]catalogDomain.Book, int64, error) {
	return f.books, int64(len(f.books)), nil
}

func (f *fakeAdminRepository) GetBookByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error) {
	for i := range f.books {
		if f.books[i].ID == id {
			return &f.books[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminRepository) GetBookByExternalID(ctx context.Context, source, id string) (*catalogDomain.Book, error) {
	for i := range f.books {
		b := f.books[i]
		if b.ExternalSource != nil && *b.ExternalSource == source && b.ExternalID != nil && *b.ExternalID == id {
			return &f.books[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminRepository) CreateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	f.books = append(f.books, *book)
	return nil
}

func (f *fakeAdminRepository) UpdateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	for i := range f.books {
		if f.books[i].ID == book.ID {
			f.books[i] = *book
			return nil
		}
	}
	return errors.New("record not found")
}

func (f *fakeAdminRepository) BookSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	for _, b := range f.books {
		if b.Slug == slug && (excludeID == nil || b.ID != *excludeID) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeAdminRepository) ListAuthors(ctx context.Context, search string, limit, offset int) ([]catalogDomain.Author, int64, error) {
	return f.authors, int64(len(f.authors)), nil
}

func (f *fakeAdminRepository) GetAuthorByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error) {
	for i := range f.authors {
		if f.authors[i].ID == id {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminRepository) GetAuthorBySlug(ctx context.Context, slug string) (*catalogDomain.Author, error) {
	for i := range f.authors {
		if f.authors[i].Slug == slug {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminRepository) CreateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	f.authors = append(f.authors, *author)
	return nil
}

func (f *fakeAdminRepository) UpdateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	for i := range f.authors {
		if f.authors[i].ID == author.ID {
			f.authors[i] = *author
			return nil
		}
	}
	return errors.New("record not found")
}

func (f *fakeAdminRepository) AuthorSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	for _, a := range f.authors {
		if a.Slug == slug && (excludeID == nil || a.ID != *excludeID) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeAdminRepository) ListGenres(ctx context.Context, search string, limit, offset int) ([]catalogDomain.Genre, int64, error) {
	return f.genres, int64(len(f.genres)), nil
}

func (f *fakeAdminRepository) GetGenreByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error) {
	for i := range f.genres {
		if f.genres[i].ID == id {
			return &f.genres[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminRepository) GetGenreBySlug(ctx context.Context, slug string) (*catalogDomain.Genre, error) {
	for i := range f.genres {
		if f.genres[i].Slug == slug {
			return &f.genres[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminRepository) CreateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	f.genres = append(f.genres, *genre)
	return nil
}

func (f *fakeAdminRepository) UpdateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	for i := range f.genres {
		if f.genres[i].ID == genre.ID {
			f.genres[i] = *genre
			return nil
		}
	}
	return errors.New("record not found")
}

func (f *fakeAdminRepository) GenreSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	for _, g := range f.genres {
		if g.Slug == slug && (excludeID == nil || g.ID != *excludeID) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeAdminRepository) ListReviews(ctx context.Context, filter adminrepository.ReviewFilter) ([]reviewsDomain.Review, int64, error) {
	return nil, 0, nil
}

func (f *fakeAdminRepository) GetReviewByID(ctx context.Context, id uuid.UUID) (*reviewsDomain.Review, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminRepository) ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminRepository) AuthorsExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
	for _, id := range ids {
		found := false
		for _, a := range f.authors {
			if a.ID == id {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

func (f *fakeAdminRepository) GenresExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
	for _, id := range ids {
		found := false
		for _, g := range f.genres {
			if g.ID == id {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

func newTestAdmin() (*adminservice.Service, *fakeAdminRepository) {
	repo := &fakeAdminRepository{}
	return adminservice.NewService(repo), repo
}

// fakeWikidataQuerier implements WikidataQuerier for tests.
type fakeWikidataQuerier struct {
	records []bookdata.ExternalBookRecord
	err     error
}

func (f *fakeWikidataQuerier) Query(ctx context.Context, sparql string) ([]bookdata.ExternalBookRecord, error) {
	return f.records, f.err
}

// fakeEnrichClient implements bookdata.Client for tests, with canned
// responses keyed by the ISBN/title passed in.
type fakeEnrichClient struct {
	byISBN        map[string]*bookdata.ExternalBookRecord
	byTitleAuthor map[string]*bookdata.ExternalBookRecord
}

func (f *fakeEnrichClient) SourceName() string { return "openlibrary" }

func (f *fakeEnrichClient) FindByISBN(ctx context.Context, isbn string) (*bookdata.ExternalBookRecord, error) {
	if rec, ok := f.byISBN[isbn]; ok {
		return rec, nil
	}
	return nil, bookdata.ErrNotFound
}

func (f *fakeEnrichClient) FindByTitleAuthor(ctx context.Context, title, author string) (*bookdata.ExternalBookRecord, error) {
	if rec, ok := f.byTitleAuthor[title]; ok {
		return rec, nil
	}
	return nil, bookdata.ErrNotFound
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func TestProcessRecord_CreatesHiddenBook(t *testing.T) {
	admin, repo := newTestAdmin()
	ctx := context.Background()

	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Authors:    []string{"Leo Tolstoy"},
		Genres:     []string{"Novel"},
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
	}

	outcome, err := processRecord(ctx, rec, nil, admin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != outcomeCreated {
		t.Fatalf("expected outcomeCreated, got %v", outcome)
	}
	if len(repo.books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(repo.books))
	}
	book := repo.books[0]
	if !book.IsHidden {
		t.Error("expected book to be created with IsHidden = true")
	}
	if book.ExternalSource == nil || *book.ExternalSource != "wikidata" {
		t.Errorf("ExternalSource = %v, want wikidata", book.ExternalSource)
	}
	if book.ExternalID == nil || *book.ExternalID != "Q170583" {
		t.Errorf("ExternalID = %v, want Q170583", book.ExternalID)
	}
}

func TestProcessRecord_SkipsExistingExternalID(t *testing.T) {
	admin, repo := newTestAdmin()
	ctx := context.Background()

	source := "wikidata"
	externalID := "Q170583"
	repo.books = append(repo.books, catalogDomain.Book{
		ID:             uuid.New(),
		Title:          "War and Peace",
		Slug:           "war-and-peace",
		ExternalSource: &source,
		ExternalID:     &externalID,
	})

	rec := bookdata.ExternalBookRecord{
		Title:      "War and Peace",
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
	}

	outcome, err := processRecord(ctx, rec, nil, admin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != outcomeSkipped {
		t.Fatalf("expected outcomeSkipped, got %v", outcome)
	}
	if len(repo.books) != 1 {
		t.Fatalf("expected book count to stay at 1, got %d", len(repo.books))
	}
}

func TestProcessRecord_EnrichmentOnlyFillsEmptyFields(t *testing.T) {
	admin, repo := newTestAdmin()
	ctx := context.Background()

	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
		ISBN:       strPtr("9780140447934"),
		CoverURL:   strPtr("https://wikidata.example/cover.jpg"),
		// Description and PageCount left nil: Wikidata didn't report them.
	}

	enrich := &fakeEnrichClient{
		byISBN: map[string]*bookdata.ExternalBookRecord{
			"9780140447934": {
				Title:       "Война и мир",
				Description: strPtr("Роман-эпопея о жизни русского общества в эпоху войн против Наполеона"),
				CoverURL:    strPtr("https://openlibrary.example/should-not-win.jpg"),
				PageCount:   intPtr(1392),
			},
		},
	}

	outcome, err := processRecord(ctx, rec, enrich, admin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != outcomeCreated {
		t.Fatalf("expected outcomeCreated, got %v", outcome)
	}

	book := repo.books[0]
	if book.CoverURL == nil || *book.CoverURL != "https://wikidata.example/cover.jpg" {
		t.Errorf("CoverURL = %v, want Wikidata's own value preserved", book.CoverURL)
	}
	wantDescription := "Роман-эпопея о жизни русского общества в эпоху войн против Наполеона"
	if book.Description == nil || *book.Description != wantDescription {
		t.Errorf("Description = %v, want Open Library's (Russian) enrichment value %q", book.Description, wantDescription)
	}
	if book.PageCount == nil || *book.PageCount != 1392 {
		t.Errorf("PageCount = %v, want 1392", book.PageCount)
	}
}

func TestProcessRecord_NonRussianEnrichmentDescriptionIsDropped(t *testing.T) {
	admin, repo := newTestAdmin()
	ctx := context.Background()

	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
		ISBN:       strPtr("9780140447934"),
	}

	enrich := &fakeEnrichClient{
		byISBN: map[string]*bookdata.ExternalBookRecord{
			"9780140447934": {
				Title:       "War and Peace",
				Description: strPtr("An English-language excerpt from Open Library"),
			},
		},
	}

	outcome, err := processRecord(ctx, rec, enrich, admin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != outcomeCreated {
		t.Fatalf("expected outcomeCreated (non-Russian Description must not reject the record), got %v", outcome)
	}

	book := repo.books[0]
	if book.Description != nil {
		t.Errorf("Description = %v, want nil (non-Russian enrichment text silently dropped)", *book.Description)
	}
}

func TestProcessRecord_EmptyLanguageDefaultsToRu(t *testing.T) {
	admin, repo := newTestAdmin()
	ctx := context.Background()

	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Language:   "",
		SourceName: "wikidata",
		SourceID:   "Q170583",
	}

	outcome, err := processRecord(ctx, rec, nil, admin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != outcomeCreated {
		t.Fatalf("expected outcomeCreated, got %v", outcome)
	}
	if repo.books[0].Language != "ru" {
		t.Errorf("Language = %q, want default %q", repo.books[0].Language, "ru")
	}
}

func TestProcessRecord_ResolvesAuthorsAndGenresBySlug(t *testing.T) {
	admin, repo := newTestAdmin()
	ctx := context.Background()

	existingAuthorID := uuid.New()
	repo.authors = append(repo.authors, catalogDomain.Author{ID: existingAuthorID, Name: "Leo Tolstoy", Slug: "leo-tolstoy"})

	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Authors:    []string{"Leo Tolstoy", "New Author"},
		Genres:     []string{"Novel"},
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
	}

	outcome, err := processRecord(ctx, rec, nil, admin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != outcomeCreated {
		t.Fatalf("expected outcomeCreated, got %v", outcome)
	}

	// Existing author reused, not duplicated; one new author created.
	if len(repo.authors) != 2 {
		t.Fatalf("expected 2 authors total (1 reused + 1 created), got %d", len(repo.authors))
	}
	foundExisting := false
	for _, a := range repo.authors {
		if a.ID == existingAuthorID {
			foundExisting = true
		}
	}
	if !foundExisting {
		t.Error("expected existing author to be reused, not recreated")
	}

	if len(repo.genres) != 1 {
		t.Fatalf("expected 1 genre created, got %d", len(repo.genres))
	}
}

func TestRunHarvest_PartialFailureDoesNotAbortBatch(t *testing.T) {
	admin, repo := newTestAdmin()

	records := []bookdata.ExternalBookRecord{
		{
			Title:      "Хорошая книга",
			Language:   "ru",
			SourceName: "wikidata",
			SourceID:   "Q1",
		},
		{
			Title:      "Плохой ISBN",
			Language:   "ru",
			SourceName: "wikidata",
			SourceID:   "Q2",
			ISBN:       strPtr("not-a-valid-isbn"),
		},
	}

	handler := NewHandler(&fakeWikidataQuerier{records: records}, nil, admin)

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil)
	if err != nil {
		t.Fatalf("runHarvest returned unexpected top-level error: %v", err)
	}

	if len(repo.books) != 1 {
		t.Fatalf("expected 1 successfully created book, got %d", len(repo.books))
	}
	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2", result.Processed)
	}
	if result.Created != 1 {
		t.Errorf("Created = %d, want 1", result.Created)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("Errors = %v, want 1 entry", result.Errors)
	}
	if result.Errors[0].Index != 1 || result.Errors[0].Title != "Плохой ISBN" {
		t.Errorf("unexpected error entry: %+v", result.Errors[0])
	}
}

func TestRunHarvest_NonRussianTitlePastSPARQLFilterIsAPerRecordError(t *testing.T) {
	// Simulates a data-quality edge case (e.g. a "ru"-tagged Wikidata label
	// that is not actually Cyrillic) that slips past the SPARQL-level
	// filter - the Cyrillic-ratio check on Title is defense-in-depth for
	// exactly this case.
	admin, repo := newTestAdmin()

	records := []bookdata.ExternalBookRecord{
		{
			Title:      "Non-Russian Title",
			Language:   "ru",
			SourceName: "wikidata",
			SourceID:   "Q1",
		},
		{
			Title:      "Хорошая книга",
			Language:   "ru",
			SourceName: "wikidata",
			SourceID:   "Q2",
		},
	}

	handler := NewHandler(&fakeWikidataQuerier{records: records}, nil, admin)

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil)
	if err != nil {
		t.Fatalf("runHarvest returned unexpected top-level error: %v", err)
	}

	if len(repo.books) != 1 {
		t.Fatalf("expected 1 successfully created book (the other record still processes), got %d", len(repo.books))
	}
	if result.Created != 1 {
		t.Errorf("Created = %d, want 1", result.Created)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("Errors = %v, want 1 entry", result.Errors)
	}
	if result.Errors[0].Index != 0 || result.Errors[0].Title != "Non-Russian Title" {
		t.Errorf("unexpected error entry: %+v", result.Errors[0])
	}
}

// fakeProgressWriter implements progressWriter for tests, capturing every
// Write call's decoded HarvestResult so tests can assert on the sequence of
// progress reports. An optional err makes every Write call fail, to assert
// that a progress-write failure never fails the harvest job itself.
type fakeProgressWriter struct {
	writes []HarvestResult
	err    error
}

func (f *fakeProgressWriter) Write(data []byte) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	var result HarvestResult
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, err
	}
	f.writes = append(f.writes, result)
	return len(data), nil
}

func TestRunHarvest_ReportsProgressPerRecord(t *testing.T) {
	admin, _ := newTestAdmin()

	records := []bookdata.ExternalBookRecord{
		{Title: "Книга 1", Language: "ru", SourceName: "wikidata", SourceID: "Q1"},
		{Title: "Книга 2", Language: "ru", SourceName: "wikidata", SourceID: "Q2"},
		{Title: "Книга 3", Language: "ru", SourceName: "wikidata", SourceID: "Q3"},
	}
	handler := NewHandler(&fakeWikidataQuerier{records: records}, nil, admin)

	progress := &fakeProgressWriter{}
	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, progress)
	if err != nil {
		t.Fatalf("runHarvest returned unexpected error: %v", err)
	}
	if result.Total != 3 || result.Processed != 3 {
		t.Fatalf("unexpected final result: %+v", result)
	}

	// One write immediately after the query (Total known, Processed: 0), plus
	// one per record - 4 total for 3 records.
	if len(progress.writes) != 4 {
		t.Fatalf("expected 4 progress writes, got %d: %+v", len(progress.writes), progress.writes)
	}
	if progress.writes[0].Total != 3 || progress.writes[0].Processed != 0 {
		t.Fatalf("first progress write = %+v, want Total=3 Processed=0", progress.writes[0])
	}
	for i, want := range []int{1, 2, 3} {
		if progress.writes[i+1].Processed != want {
			t.Errorf("progress.writes[%d].Processed = %d, want %d", i+1, progress.writes[i+1].Processed, want)
		}
		if progress.writes[i+1].Total != 3 {
			t.Errorf("progress.writes[%d].Total = %d, want 3", i+1, progress.writes[i+1].Total)
		}
	}
}

func TestRunHarvest_NilProgressWriterDoesNotPanic(t *testing.T) {
	admin, _ := newTestAdmin()
	records := []bookdata.ExternalBookRecord{
		{Title: "Книга 1", Language: "ru", SourceName: "wikidata", SourceID: "Q1"},
	}
	handler := NewHandler(&fakeWikidataQuerier{records: records}, nil, admin)

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil)
	if err != nil {
		t.Fatalf("runHarvest returned unexpected error with nil progress writer: %v", err)
	}
	if result.Processed != 1 || result.Total != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunHarvest_ProgressWriteErrorDoesNotFailHarvest(t *testing.T) {
	admin, repo := newTestAdmin()
	records := []bookdata.ExternalBookRecord{
		{Title: "Книга 1", Language: "ru", SourceName: "wikidata", SourceID: "Q1"},
		{Title: "Книга 2", Language: "ru", SourceName: "wikidata", SourceID: "Q2"},
	}
	handler := NewHandler(&fakeWikidataQuerier{records: records}, nil, admin)

	progress := &fakeProgressWriter{err: errors.New("redis unavailable")}
	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, progress)
	if err != nil {
		t.Fatalf("runHarvest returned unexpected error despite progress-write failures: %v", err)
	}
	if result.Processed != 2 || result.Created != 2 {
		t.Fatalf("unexpected result despite progress-write failures: %+v", result)
	}
	if len(repo.books) != 2 {
		t.Fatalf("expected both books created despite progress-write failures, got %d", len(repo.books))
	}
	if len(progress.writes) != 0 {
		t.Fatalf("expected no successful writes captured, got %d", len(progress.writes))
	}
}

// TestHandleTask_PayloadDecoding constructs a real asynq.Task via
// asynq.NewTask (no Redis needed to build one) and asserts the handler
// entry point rejects a malformed/empty payload before ever touching
// wikidata or the admin service - exercising the asynq.Task type itself
// for the payload (de)serialization contract.
func TestHandleTask_PayloadDecoding(t *testing.T) {
	admin, _ := newTestAdmin()
	handler := NewHandler(&fakeWikidataQuerier{}, nil, admin)

	payload, err := json.Marshal(HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	task := asynq.NewTask(TaskTypeHarvestWikidataQuery, payload)

	var decoded HarvestPayload
	if err := json.Unmarshal(task.Payload(), &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded.SPARQLQuery != "SELECT * WHERE {}" {
		t.Fatalf("decoded SPARQLQuery = %q", decoded.SPARQLQuery)
	}

	// The empty-query branch is reachable through HandleTask itself (it
	// returns before ever needing a ResultWriter), so exercise it directly.
	emptyPayload, _ := json.Marshal(HarvestPayload{SPARQLQuery: ""})
	emptyTask := asynq.NewTask(TaskTypeHarvestWikidataQuery, emptyPayload)
	if err := handler.HandleTask(context.Background(), emptyTask); !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("expected empty query to be non-retryable, got %v", err)
	}
}

func TestHandleTask_MalformedPayloadIsNonRetryable(t *testing.T) {
	admin, _ := newTestAdmin()
	handler := NewHandler(&fakeWikidataQuerier{}, nil, admin)

	task := asynq.NewTask(TaskTypeHarvestWikidataQuery, []byte("{not valid json"))

	err := handler.HandleTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for malformed payload")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("expected error to wrap asynq.SkipRetry, got %v", err)
	}
}

func TestHandleTask_EmptyQueryIsNonRetryable(t *testing.T) {
	admin, _ := newTestAdmin()
	handler := NewHandler(&fakeWikidataQuerier{}, nil, admin)

	payload, _ := json.Marshal(HarvestPayload{SPARQLQuery: "   "})
	task := asynq.NewTask(TaskTypeHarvestWikidataQuery, payload)

	err := handler.HandleTask(context.Background(), task)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("expected error to wrap asynq.SkipRetry, got %v", err)
	}
}

func TestHandleTask_WikidataQueryErrorIsRetryable(t *testing.T) {
	admin, _ := newTestAdmin()
	handler := NewHandler(&fakeWikidataQuerier{err: bookdata.ErrSourceUnavailable}, nil, admin)

	payload, _ := json.Marshal(HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"})
	task := asynq.NewTask(TaskTypeHarvestWikidataQuery, payload)

	err := handler.HandleTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, asynq.SkipRetry) {
		t.Error("expected a retryable error (not wrapping asynq.SkipRetry) for a source-unavailable failure")
	}
}
