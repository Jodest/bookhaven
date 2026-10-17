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
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
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
	// bookAuthorIDs records the author IDs passed to each CreateBook call,
	// parallel to books.
	bookAuthorIDs [][]uuid.UUID
	// bookGenreIDs records the genre IDs passed to each CreateBook call,
	// parallel to books.
	bookGenreIDs [][]uuid.UUID
}

func (f *fakeAdminRepository) WithinTransaction(ctx context.Context, fn func(repo adminrepository.Repository) error) error {
	return fn(f)
}

var _ adminrepository.Repository = (*fakeAdminRepository)(nil)

func (f *fakeAdminRepository) DashboardCounts(ctx context.Context) (adminrepository.DashboardStats, error) {
	return adminrepository.DashboardStats{Books: int64(len(f.books)), Authors: int64(len(f.authors)), Genres: int64(len(f.genres))}, nil
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
	f.bookAuthorIDs = append(f.bookAuthorIDs, authorIDs)
	f.bookGenreIDs = append(f.bookGenreIDs, genreIDs)
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

func (f *fakeAdminRepository) ListAuthors(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Author, int64, error) {
	if status == "" {
		return f.authors, int64(len(f.authors)), nil
	}
	var out []catalogDomain.Author
	for _, a := range f.authors {
		if a.Status == status {
			out = append(out, a)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeAdminRepository) GetAuthorByExternalID(ctx context.Context, source, id string) (*catalogDomain.Author, error) {
	for i := range f.authors {
		a := f.authors[i]
		if a.ExternalSource != nil && *a.ExternalSource == source && a.ExternalID != nil && *a.ExternalID == id {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminRepository) SetAuthorExternalID(ctx context.Context, id uuid.UUID, source, externalID string) error {
	for i := range f.authors {
		if f.authors[i].ID == id {
			src, ext := source, externalID
			f.authors[i].ExternalSource = &src
			f.authors[i].ExternalID = &ext
			return nil
		}
	}
	return gorm.ErrRecordNotFound
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

func (f *fakeAdminRepository) ListAuthorsByExternalIDs(ctx context.Context, source string, ids []string) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeAdminRepository) AuthorSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	for _, a := range f.authors {
		if a.Slug == slug && (excludeID == nil || a.ID != *excludeID) {
			return true, nil
		}
	}
	return false, nil
}

// MergeAuthors is a minimal fake satisfying adminrepository.Repository;
// no existing test in this file exercises it.
func (f *fakeAdminRepository) MergeAuthors(ctx context.Context, duplicateID, canonicalID uuid.UUID) (int64, error) {
	for i := range f.authors {
		if f.authors[i].ID == duplicateID {
			f.authors = append(f.authors[:i], f.authors[i+1:]...)
			break
		}
	}
	return 0, nil
}

func (f *fakeAdminRepository) ListGenres(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Genre, int64, error) {
	return f.genres, int64(len(f.genres)), nil
}

func (f *fakeAdminRepository) GetGenreByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error) {
	// Mirrors the repository: alias rows are not found by id.
	for i := range f.genres {
		if f.genres[i].ID == id && !f.genres[i].IsAlias() {
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

func (f *fakeAdminRepository) GetGenreByQID(ctx context.Context, qid string) (*catalogDomain.Genre, error) {
	for i := range f.genres {
		if f.genres[i].WikidataQID != nil && *f.genres[i].WikidataQID == qid {
			return &f.genres[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminRepository) ListGenresByQIDs(ctx context.Context, qids []string) ([]catalogDomain.Genre, error) {
	return []catalogDomain.Genre{}, nil
}

func (f *fakeAdminRepository) SetGenreQID(ctx context.Context, id uuid.UUID, qid string) error {
	for i := range f.genres {
		if f.genres[i].ID == id {
			q := qid
			f.genres[i].WikidataQID = &q
			return nil
		}
	}
	return gorm.ErrRecordNotFound
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
	if book.Status != catalogDomain.BookStatusPending {
		t.Errorf("expected book to be created with status pending, got %q", book.Status)
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

func TestProcessRecord_TitleAuthorEnrichmentDescriptionFlowsThroughRussianFilter(t *testing.T) {
	// Regression check for docs/features/openlibrary-works-description.md:
	// a Description returned via the FindByTitleAuthor path (the search-
	// match/no-ISBN case, now the majority path once FetchWorkDescription is
	// wired into that client) must flow through the existing Russian-filter
	// logic exactly like a FindByISBN-sourced Description already does. No
	// need to fake the two-step Open Library Works API call here - that's
	// fully covered at the bookdata package level.
	admin, repo := newTestAdmin()
	ctx := context.Background()

	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
		// No ISBN: Wikidata predates ISBN for this classic, forcing the
		// FindByTitleAuthor (search) enrichment path instead of FindByISBN.
	}

	enrich := &fakeEnrichClient{
		byTitleAuthor: map[string]*bookdata.ExternalBookRecord{
			"Война и мир": {
				Title:       "Война и мир",
				Description: strPtr("Роман-эпопея о жизни русского общества в эпоху войн против Наполеона"),
				SourceName:  "openlibrary",
				SourceID:    "OL262461W",
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
	wantDescription := "Роман-эпопея о жизни русского общества в эпоху войн против Наполеона"
	if book.Description == nil || *book.Description != wantDescription {
		t.Errorf("Description = %v, want Open Library's (Russian) FindByTitleAuthor-sourced value %q", book.Description, wantDescription)
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

func TestRunHarvest_LatinScriptRuLabelTitleIsCreated(t *testing.T) {
	// A "ru"-tagged Wikidata label written in Latin script (e.g. Pelevin's
	// «S.N.U.F.F.») is the book's official Russian title, so harvest
	// exempts it from the title-language check - see
	// docs/features/latin-book-titles.md.
	admin, repo := newTestAdmin()

	records := []bookdata.ExternalBookRecord{
		{
			Title:      "S.N.U.F.F.",
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

	if result.Created != 2 {
		t.Errorf("Created = %d, want 2", result.Created)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("Errors = %v, want none", result.Errors)
	}
	if len(repo.books) != 2 || repo.books[0].Title != "S.N.U.F.F." {
		t.Fatalf("expected both books created with the Latin title preserved, got %+v", repo.books)
	}
}

func TestProcessRecord_LatinTitleBypassesOnlyTitleCheck(t *testing.T) {
	// processRecord sets BookInput.AllowNonRussianTitle: a Latin-script
	// title is created, but a non-Russian description would still be
	// rejected by CreateBook (the description check is not lifted).
	admin, repo := newTestAdmin()
	ctx := context.Background()

	rec := bookdata.ExternalBookRecord{
		Title:      "Empire V",
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q4380003",
	}

	outcome, err := processRecord(ctx, rec, nil, admin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != outcomeCreated {
		t.Fatalf("expected outcomeCreated, got %v", outcome)
	}
	if len(repo.books) != 1 || repo.books[0].Title != "Empire V" {
		t.Fatalf("expected 1 book titled %q, got %+v", "Empire V", repo.books)
	}

	rec2 := bookdata.ExternalBookRecord{
		Title:       "Empire W",
		Description: strPtr("An English description"),
		Language:    "ru",
		SourceName:  "wikidata",
		SourceID:    "Q4380004",
	}
	if _, err := processRecord(ctx, rec2, nil, admin); !errors.Is(err, adminservice.ErrDescriptionNotRussian) {
		t.Fatalf("expected ErrDescriptionNotRussian, got %v", err)
	}
}

func TestRunHarvest_SkippedRecordsMatchSkippedCount(t *testing.T) {
	admin, repo := newTestAdmin()

	// Q2 and Q4 already exist in the catalog, so processRecord skips them.
	source := "wikidata"
	existingQ2 := "Q2"
	existingQ4 := "Q4"
	repo.books = append(repo.books,
		catalogDomain.Book{ID: uuid.New(), Title: "Книга 2 (уже в каталоге)", Slug: "kniga-2", ExternalSource: &source, ExternalID: &existingQ2},
		catalogDomain.Book{ID: uuid.New(), Title: "Книга 4 (уже в каталоге)", Slug: "kniga-4", ExternalSource: &source, ExternalID: &existingQ4},
	)

	records := []bookdata.ExternalBookRecord{
		{Title: "Книга 1", Language: "ru", SourceName: "wikidata", SourceID: "Q1"},
		{Title: "Книга 2", Language: "ru", SourceName: "wikidata", SourceID: "Q2"},
		{Title: "Плохой ISBN", Language: "ru", SourceName: "wikidata", SourceID: "Q3", ISBN: strPtr("not-a-valid-isbn")},
		{Title: "Книга 4", Language: "ru", SourceName: "wikidata", SourceID: "Q4"},
		{Title: "Книга 5", Language: "ru", SourceName: "wikidata", SourceID: "Q5"},
	}

	handler := NewHandler(&fakeWikidataQuerier{records: records}, nil, admin)

	result, err := handler.runHarvest(context.Background(), HarvestPayload{SPARQLQuery: "SELECT * WHERE {}"}, nil)
	if err != nil {
		t.Fatalf("runHarvest returned unexpected top-level error: %v", err)
	}

	if result.Skipped != 2 {
		t.Fatalf("Skipped = %d, want 2", result.Skipped)
	}
	if len(result.SkippedRecords) != result.Skipped {
		t.Fatalf("len(SkippedRecords) = %d, want it to equal Skipped (%d)", len(result.SkippedRecords), result.Skipped)
	}

	wantSkipped := []HarvestSkippedRecord{
		{Index: 1, Title: "Книга 2"},
		{Index: 3, Title: "Книга 4"},
	}
	for i, want := range wantSkipped {
		if result.SkippedRecords[i] != want {
			t.Errorf("SkippedRecords[%d] = %+v, want %+v", i, result.SkippedRecords[i], want)
		}
	}

	if len(result.Errors) != 1 || result.Errors[0].Index != 2 {
		t.Fatalf("unexpected Errors: %+v", result.Errors)
	}
	if result.Created != 2 {
		t.Errorf("Created = %d, want 2", result.Created)
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

func TestRunHarvest_ReportsSkippedRecordsProgressPerRecord(t *testing.T) {
	admin, repo := newTestAdmin()

	source := "wikidata"
	existingQ2 := "Q2"
	repo.books = append(repo.books, catalogDomain.Book{ID: uuid.New(), Title: "Книга 2 (уже в каталоге)", Slug: "kniga-2", ExternalSource: &source, ExternalID: &existingQ2})

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
	if len(result.SkippedRecords) != 1 || result.SkippedRecords[0].Index != 1 {
		t.Fatalf("unexpected final SkippedRecords: %+v", result.SkippedRecords)
	}

	// 4 writes for 3 records (initial + one per record, see
	// TestRunHarvest_ReportsProgressPerRecord). SkippedRecords must appear
	// empty before record 1 is processed, and grow to 1 entry from the
	// write right after record 1 (index 1, Q2) onward - not just present in
	// the final result.
	if len(progress.writes) != 4 {
		t.Fatalf("expected 4 progress writes, got %d: %+v", len(progress.writes), progress.writes)
	}
	if len(progress.writes[0].SkippedRecords) != 0 {
		t.Fatalf("progress.writes[0].SkippedRecords = %+v, want empty before any record processed", progress.writes[0].SkippedRecords)
	}
	if len(progress.writes[1].SkippedRecords) != 0 {
		t.Fatalf("progress.writes[1].SkippedRecords = %+v, want empty after record 0 (a create)", progress.writes[1].SkippedRecords)
	}
	for i, w := range progress.writes[2:] {
		if len(w.SkippedRecords) != 1 || w.SkippedRecords[0].Index != 1 || w.SkippedRecords[0].Title != "Книга 2" {
			t.Errorf("progress.writes[%d].SkippedRecords = %+v, want [{1 Книга 2}]", i+2, w.SkippedRecords)
		}
	}
	if len(repo.books) != 3 {
		t.Fatalf("expected 3 books total (1 pre-existing + 2 newly created), got %d", len(repo.books))
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

func TestProcessRecord_UsesAuthorRefsQIDs(t *testing.T) {
	admin, repo := newTestAdmin()
	ctx := context.Background()

	renamedID := uuid.New()
	repo.authors = append(repo.authors, catalogDomain.Author{
		ID: renamedID, Name: "Lev Tolstoi", Slug: "lev-tolstoi", Status: catalogDomain.AuthorStatusPublished,
		ExternalSource: strPtr("wikidata"), ExternalID: strPtr("Q7243"),
	})

	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Authors:    []string{"Leo Tolstoy", "New Author"},
		AuthorRefs: []bookdata.ExternalAuthor{{Name: "Leo Tolstoy", QID: "Q7243"}, {Name: "New Author", QID: "Q99"}},
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
	}

	if _, err := processRecord(ctx, rec, nil, admin); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.authors) != 2 {
		t.Fatalf("expected QID match + 1 new author, got %d authors", len(repo.authors))
	}
	created := repo.authors[1]
	if created.Status != catalogDomain.AuthorStatusPending || created.ExternalID == nil || *created.ExternalID != "Q99" {
		t.Fatalf("expected new pending author with Q99, got %+v", created)
	}
	ids := repo.bookAuthorIDs[0]
	if len(ids) != 2 || ids[0] != renamedID {
		t.Fatalf("expected book linked to QID-matched author first, got %v", ids)
	}
}

func TestProcessRecord_FallsBackToAuthorNamesWithoutRefs(t *testing.T) {
	admin, repo := newTestAdmin()
	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Authors:    []string{"Leo Tolstoy"},
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
	}

	if _, err := processRecord(context.Background(), rec, nil, admin); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.authors) != 1 || repo.authors[0].Status != catalogDomain.AuthorStatusPending || repo.authors[0].ExternalID != nil {
		t.Fatalf("expected one QID-less pending author, got %+v", repo.authors)
	}
}

func TestProcessRecord_AllAuthorsRejectedStillCreatesHiddenBook(t *testing.T) {
	admin, repo := newTestAdmin()
	repo.authors = append(repo.authors, catalogDomain.Author{
		ID: uuid.New(), Name: "Junk", Slug: "junk", Status: catalogDomain.AuthorStatusRejected,
		ExternalSource: strPtr("wikidata"), ExternalID: strPtr("Q1"),
	})
	rec := bookdata.ExternalBookRecord{
		Title:      "Война и мир",
		Authors:    []string{"Some Label"},
		AuthorRefs: []bookdata.ExternalAuthor{{Name: "Some Label", QID: "Q1"}},
		Language:   "ru",
		SourceName: "wikidata",
		SourceID:   "Q170583",
	}

	outcome, err := processRecord(context.Background(), rec, nil, admin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != outcomeCreated || len(repo.books) != 1 || repo.books[0].Status != catalogDomain.BookStatusPending {
		t.Fatalf("expected one hidden book created, outcome=%v books=%+v", outcome, repo.books)
	}
	if len(repo.authors) != 1 {
		t.Fatalf("expected no new author, got %d", len(repo.authors))
	}
	if len(repo.bookAuthorIDs[0]) != 0 {
		t.Fatalf("expected no book_authors link to the rejected author, got %v", repo.bookAuthorIDs[0])
	}
}

func (f *fakeAdminRepository) ListAuthorsForModeration(ctx context.Context, filter adminrepository.AuthorModerationFilter) ([]adminrepository.AuthorModerationRow, int64, error) {
	return nil, 0, nil
}

func (f *fakeAdminRepository) ListAuthorSampleBooks(ctx context.Context, authorIDs []uuid.UUID) (map[uuid.UUID][]adminrepository.SampleBook, error) {
	return map[uuid.UUID][]adminrepository.SampleBook{}, nil
}

func (f *fakeAdminRepository) FindAuthorsBySurnames(ctx context.Context, surnames []string, excludeIDs []uuid.UUID) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeAdminRepository) SetAuthorStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	return false, nil
}

func (f *fakeAdminRepository) GetAuthorsByIDs(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Author, error) {
	var out []catalogDomain.Author
	for _, id := range ids {
		for _, a := range f.authors {
			if a.ID == id {
				out = append(out, a)
			}
		}
	}
	return out, nil
}

// FillAuthorEnrichment mirrors the repository's fill-empty-only semantics.
func (f *fakeAdminRepository) FillAuthorEnrichment(ctx context.Context, id uuid.UUID, fill adminrepository.AuthorEnrichmentFill) error {
	for i := range f.authors {
		a := &f.authors[i]
		if a.ID != id {
			continue
		}
		if fill.BirthDate != nil && a.BirthDate == nil {
			a.BirthDate = fill.BirthDate
		}
		if fill.DeathDate != nil && a.DeathDate == nil {
			a.DeathDate = fill.DeathDate
		}
		if p := fill.Photo; p != nil && (a.PhotoURL == nil || *a.PhotoURL == "") {
			url, license, source := p.URL, p.License, p.SourceURL
			a.PhotoURL, a.PhotoAuthor, a.PhotoLicense, a.PhotoLicenseURL, a.PhotoSourceURL = &url, p.Author, &license, p.LicenseURL, &source
		}
	}
	return nil
}

func (f *fakeAdminRepository) ListUsers(ctx context.Context, filter adminrepository.UserFilter) ([]usersDomain.User, int64, error) {
	return nil, 0, nil
}

func (f *fakeAdminRepository) ChangeUserRole(ctx context.Context, targetID uuid.UUID, decide adminrepository.RoleDecider) (*usersDomain.User, usersDomain.Role, error) {
	return nil, "", nil
}

func (f *fakeAdminRepository) ListGenresForModeration(ctx context.Context, filter adminrepository.GenreModerationFilter) ([]adminrepository.GenreModerationRow, int64, error) {
	return nil, 0, nil
}

func (f *fakeAdminRepository) ListGenreSampleBooks(ctx context.Context, genreIDs []uuid.UUID) (map[uuid.UUID][]adminrepository.SampleBook, error) {
	return map[uuid.UUID][]adminrepository.SampleBook{}, nil
}

func (f *fakeAdminRepository) ListGenreHintCandidates(ctx context.Context) ([]catalogDomain.Genre, error) {
	return nil, nil
}

func (f *fakeAdminRepository) SetGenreStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	return false, nil
}

func (f *fakeAdminRepository) SetBookStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	return false, nil
}

func (f *fakeAdminRepository) MergeGenres(ctx context.Context, sourceID, targetID uuid.UUID, actorID *uuid.UUID) (adminrepository.GenreMergeResult, error) {
	return adminrepository.GenreMergeResult{}, nil
}

func (f *fakeAdminRepository) MergeGenresBatch(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID, actorID *uuid.UUID) (adminrepository.GenreMergeResult, error) {
	return adminrepository.GenreMergeResult{}, nil
}

func (f *fakeAdminRepository) ListGenreMerges(ctx context.Context, page, limit int) ([]adminrepository.GenreMergeEntry, int64, error) {
	return nil, 0, nil
}

func (f *fakeAdminRepository) PreviewGenreMergeUndo(ctx context.Context, mergeID uuid.UUID) (adminrepository.GenreMergeUndoPreview, error) {
	return adminrepository.GenreMergeUndoPreview{}, nil
}

func (f *fakeAdminRepository) UndoGenreMerge(ctx context.Context, mergeID uuid.UUID, actorID uuid.UUID) (adminrepository.GenreMergeUndoResult, error) {
	return adminrepository.GenreMergeUndoResult{}, nil
}

func (f *fakeAdminRepository) PreviewGenreMerge(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID) (adminrepository.GenreMergePreview, error) {
	return adminrepository.GenreMergePreview{}, nil
}

func (f *fakeAdminRepository) ListBooksForModeration(ctx context.Context, filter adminrepository.BookModerationFilter) ([]catalogDomain.Book, int64, error) {
	return nil, 0, nil
}

func (f *fakeAdminRepository) FindBooksByNormalizedISBNs(ctx context.Context, isbns []string, excludeIDs []uuid.UUID) ([]adminrepository.BookHintCandidate, error) {
	return nil, nil
}

func (f *fakeAdminRepository) FindBooksByAuthorIDs(ctx context.Context, authorIDs, excludeIDs []uuid.UUID) ([]adminrepository.BookHintCandidate, error) {
	return nil, nil
}

// AI verdict repository stubs (unused by the harvest tests).
func (f *fakeAdminRepository) UpsertBookAIVerdict(ctx context.Context, v adminrepository.BookAIVerdict) error {
	return nil
}

func (f *fakeAdminRepository) GetBookAIVerdicts(ctx context.Context, bookIDs []uuid.UUID) (map[uuid.UUID]adminrepository.BookAIVerdict, error) {
	return map[uuid.UUID]adminrepository.BookAIVerdict{}, nil
}

func (f *fakeAdminRepository) ListAIReviewCandidateIDs(ctx context.Context, limit, promptVersion int) ([]uuid.UUID, error) {
	return nil, nil
}

func (f *fakeAdminRepository) CountAIReviewCandidates(ctx context.Context, promptVersion int) (int64, error) {
	return 0, nil
}

func (f *fakeAdminRepository) LoadBooksForReview(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Book, error) {
	return nil, nil
}

func (f *fakeAdminRepository) ListGenreAnalysisCandidates(ctx context.Context) ([]adminrepository.GenreModerationRow, error) {
	return nil, nil
}

func (f *fakeAdminRepository) GenresByIDs(ctx context.Context, ids []uuid.UUID) ([]adminrepository.GenreStateRow, error) {
	return nil, nil
}
