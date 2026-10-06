package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// fakeRepository implements repository.Repository for unit testing.
type fakeRepository struct {
	books            []catalogDomain.Book
	authors          []catalogDomain.Author
	genres           []catalogDomain.Genre
	reviews          []reviewsDomain.Review
	dashboardErr     error
	bookSlugExists   bool
	authorSlugExists bool
	genreSlugExists  bool
	authorsExist     bool
	genresExist      bool
	duplicateBookErr bool
	authorLookupErr  error
	genreLookupErr   error
}

func (f *fakeRepository) DashboardCounts(ctx context.Context) (repository.DashboardStats, error) {
	if f.dashboardErr != nil {
		return repository.DashboardStats{}, f.dashboardErr
	}
	var st repository.DashboardStats
	for _, b := range f.books {
		st.Books++
		if b.IsHidden {
			st.HiddenBooks++
		}
		if b.IsHidden && b.ExternalSource != nil {
			st.PendingImportReview++
		}
	}
	st.Authors = int64(len(f.authors))
	for _, a := range f.authors {
		if a.Status == catalogDomain.AuthorStatusPending {
			st.PendingAuthors++
		}
	}
	st.Genres = int64(len(f.genres))
	for _, r := range f.reviews {
		if r.IsHidden {
			st.PendingReviews++
		}
	}
	return st, nil
}

func (f *fakeRepository) ListBooks(ctx context.Context, filter repository.BookFilter) ([]catalogDomain.Book, int64, error) {
	return f.books, int64(len(f.books)), nil
}

func (f *fakeRepository) GetBookByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error) {
	for i := range f.books {
		if f.books[i].ID == id {
			return &f.books[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) GetBookByExternalID(ctx context.Context, source, id string) (*catalogDomain.Book, error) {
	for i := range f.books {
		if f.books[i].ExternalSource != nil && *f.books[i].ExternalSource == source &&
			f.books[i].ExternalID != nil && *f.books[i].ExternalID == id {
			return &f.books[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) CreateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	if f.duplicateBookErr {
		return &pgconn.PgError{Code: "23505"}
	}
	f.books = append(f.books, *book)
	return nil
}

func (f *fakeRepository) UpdateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	for i := range f.books {
		if f.books[i].ID == book.ID {
			f.books[i] = *book
			return nil
		}
	}
	return errors.New("record not found")
}

func (f *fakeRepository) BookSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	return f.bookSlugExists, nil
}

func (f *fakeRepository) ListAuthors(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Author, int64, error) {
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

func (f *fakeRepository) GetAuthorByExternalID(ctx context.Context, source, id string) (*catalogDomain.Author, error) {
	for i := range f.authors {
		a := f.authors[i]
		if a.ExternalSource != nil && *a.ExternalSource == source && a.ExternalID != nil && *a.ExternalID == id {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) SetAuthorExternalID(ctx context.Context, id uuid.UUID, source, externalID string) error {
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

func (f *fakeRepository) GetAuthorByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error) {
	for i := range f.authors {
		if f.authors[i].ID == id {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) GetAuthorBySlug(ctx context.Context, slug string) (*catalogDomain.Author, error) {
	if f.authorLookupErr != nil {
		return nil, f.authorLookupErr
	}
	for i := range f.authors {
		if f.authors[i].Slug == slug {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) CreateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	f.authors = append(f.authors, *author)
	return nil
}

func (f *fakeRepository) UpdateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	for i := range f.authors {
		if f.authors[i].ID == author.ID {
			f.authors[i] = *author
			return nil
		}
	}
	return errors.New("record not found")
}

func (f *fakeRepository) AuthorSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	return f.authorSlugExists, nil
}

// MergeAuthors is a minimal fake satisfying repository.Repository; no
// existing test in this file exercises it (TestMergeAuthors in
// merge_author_test.go uses its own dedicated fake with full dedupe
// semantics).
func (f *fakeRepository) MergeAuthors(ctx context.Context, duplicateID, canonicalID uuid.UUID) (int64, error) {
	for i := range f.authors {
		if f.authors[i].ID == duplicateID {
			f.authors = append(f.authors[:i], f.authors[i+1:]...)
			break
		}
	}
	return 0, nil
}

func (f *fakeRepository) ListGenres(ctx context.Context, search string, limit, offset int) ([]catalogDomain.Genre, int64, error) {
	return f.genres, int64(len(f.genres)), nil
}

func (f *fakeRepository) GetGenreByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error) {
	for i := range f.genres {
		if f.genres[i].ID == id {
			return &f.genres[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) GetGenreBySlug(ctx context.Context, slug string) (*catalogDomain.Genre, error) {
	if f.genreLookupErr != nil {
		return nil, f.genreLookupErr
	}
	for i := range f.genres {
		if f.genres[i].Slug == slug {
			return &f.genres[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) CreateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	f.genres = append(f.genres, *genre)
	return nil
}

func (f *fakeRepository) UpdateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	for i := range f.genres {
		if f.genres[i].ID == genre.ID {
			f.genres[i] = *genre
			return nil
		}
	}
	return errors.New("record not found")
}

func (f *fakeRepository) GenreSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	return f.genreSlugExists, nil
}

func (f *fakeRepository) ListReviews(ctx context.Context, filter repository.ReviewFilter) ([]reviewsDomain.Review, int64, error) {
	return f.reviews, int64(len(f.reviews)), nil
}

func (f *fakeRepository) GetReviewByID(ctx context.Context, id uuid.UUID) (*reviewsDomain.Review, error) {
	for i := range f.reviews {
		if f.reviews[i].ID == id {
			return &f.reviews[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error) {
	for i := range f.reviews {
		if f.reviews[i].ID == id {
			f.reviews[i].IsHidden = isHidden
			// Mirror the transaction behavior: recalc visible reviews_count for the book.
			visible := 0
			for _, r := range f.reviews {
				if r.BookID == f.reviews[i].BookID && !r.IsHidden {
					visible++
				}
			}
			for j := range f.books {
				if f.books[j].ID == f.reviews[i].BookID {
					f.books[j].ReviewsCount = visible
				}
			}
			return &f.reviews[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) AuthorsExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
	return f.authorsExist, nil
}

func (f *fakeRepository) GenresExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
	return f.genresExist, nil
}

func newService() (*Service, *fakeRepository) {
	repo := &fakeRepository{authorsExist: true, genresExist: true}
	return NewService(repo), repo
}

func TestDashboardCounts(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	stats, err := svc.DashboardCounts(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats != (repository.DashboardStats{}) {
		t.Fatalf("expected empty dashboard counts, got %+v", stats)
	}
}

func TestDashboardCounts_PendingImportReview(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()

	source := "wikidata"
	repo.books = []catalogDomain.Book{
		{ID: uuid.New(), IsHidden: true, ExternalSource: &source},
		{ID: uuid.New(), IsHidden: true},
		{ID: uuid.New(), IsHidden: false},
	}

	stats, err := svc.DashboardCounts(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.HiddenBooks != 2 {
		t.Fatalf("expected 2 hidden books, got %d", stats.HiddenBooks)
	}
	if stats.PendingImportReview != 1 {
		t.Fatalf("expected 1 pending import review book, got %d", stats.PendingImportReview)
	}
}

func TestCreateBook_Valid(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()

	authorID := uuid.New().String()
	repo.authors = append(repo.authors, catalogDomain.Author{ID: uuid.MustParse(authorID), Name: "Tolstoy", Slug: "tolstoy"})

	book, err := svc.CreateBook(ctx, BookInput{
		Title:     "Война и мир",
		Language:  "ru",
		AuthorIDs: []string{authorID},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if book.Slug != "voyna-i-mir" {
		t.Fatalf("expected slug voyna-i-mir, got %s", book.Slug)
	}
	if len(repo.books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(repo.books))
	}
}

func TestCreateBook_DuplicateSlug(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()
	repo.bookSlugExists = true

	_, err := svc.CreateBook(ctx, BookInput{Title: "Книга", Language: "ru"})
	if !errors.Is(err, ErrDuplicateSlug) {
		t.Fatalf("expected duplicate slug error, got %v", err)
	}
}

func TestCreateBook_InvalidISBN(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	isbn := "123"
	_, err := svc.CreateBook(ctx, BookInput{Title: "Книга", Language: "ru", ISBN: &isbn})
	if !errors.Is(err, ErrInvalidISBN) {
		t.Fatalf("expected invalid isbn error, got %v", err)
	}
}

func TestCreateBook_InvalidLanguage(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	_, err := svc.CreateBook(ctx, BookInput{Title: "Книга", Language: "xx"})
	if !errors.Is(err, ErrInvalidLanguage) {
		t.Fatalf("expected invalid language error, got %v", err)
	}
}

func TestCreateBook_InvalidYear(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	year := 3000
	_, err := svc.CreateBook(ctx, BookInput{Title: "Книга", Language: "ru", PublishedYear: &year})
	if !errors.Is(err, ErrInvalidYear) {
		t.Fatalf("expected invalid year error, got %v", err)
	}
}

func TestCreateBook_DuplicateKeyError(t *testing.T) {
	svc, repo := newService()
	repo.duplicateBookErr = true
	ctx := context.Background()

	_, err := svc.CreateBook(ctx, BookInput{Title: "Книга", Language: "ru"})
	if !errors.Is(err, ErrDuplicateSlug) {
		t.Fatalf("expected duplicate slug error, got %v", err)
	}
}

func TestCreateBook_MissingAuthors(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()
	repo.authorsExist = false

	_, err := svc.CreateBook(ctx, BookInput{Title: "Книга", Language: "ru", AuthorIDs: []string{uuid.New().String()}})
	if !errors.Is(err, ErrInvalidAuthorIDs) {
		t.Fatalf("expected invalid author ids error, got %v", err)
	}
}

func TestCreateBook_NonRussianTitle(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	_, err := svc.CreateBook(ctx, BookInput{Title: "War and Peace", Language: "ru"})
	if !errors.Is(err, ErrTitleNotRussian) {
		t.Fatalf("expected title not russian error, got %v", err)
	}
}

func TestCreateBook_NonRussianDescription(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	desc := "An English description"
	_, err := svc.CreateBook(ctx, BookInput{Title: "Книга", Language: "ru", Description: &desc})
	if !errors.Is(err, ErrDescriptionNotRussian) {
		t.Fatalf("expected description not russian error, got %v", err)
	}
}

func TestCreateBook_RussianTitleNilDescription_NoError(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	_, err := svc.CreateBook(ctx, BookInput{Title: "Книга", Language: "ru"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateBook_WikidataProducibleLanguageCodes_Succeed(t *testing.T) {
	// Regression guard: every code bookdata.wikidataLanguageCodes (see
	// backend/internal/pkg/bookdata/wikidata.go) can resolve a harvested
	// book's original-language field to must be accepted here, or a
	// correctly-identified original language fails validation with
	// ErrInvalidLanguage during harvest ("create book: language is
	// invalid"). ru/en/uk are covered by other tests; this covers the rest.
	for _, lang := range []string{"fr", "de", "es", "pl", "it", "pt"} {
		t.Run(lang, func(t *testing.T) {
			svc, _ := newService()
			ctx := context.Background()

			_, err := svc.CreateBook(ctx, BookInput{Title: "Книга", Language: lang})
			if err != nil {
				t.Fatalf("unexpected error for language %q: %v", lang, err)
			}
		})
	}
}

func TestCreateBook_EnglishWorkLanguageWithRussianText_Succeeds(t *testing.T) {
	// Regression guard: Language records the work's original language (e.g.
	// "en" for a foreign classic with Russian catalog metadata), which this
	// contract deliberately leaves unvalidated - see Scope boundaries in
	// docs/features/russian-only-content.md.
	svc, _ := newService()
	ctx := context.Background()

	desc := "Роман на английском языке в оригинале"
	_, err := svc.CreateBook(ctx, BookInput{Title: "О дивный новый мир", Language: "en", Description: &desc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateBook_NotFound(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	_, err := svc.UpdateBook(ctx, uuid.New(), BookInput{Title: "Книга", Language: "ru"})
	if !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("expected book not found error, got %v", err)
	}
}

func TestUpdateBook_NonRussianTitle(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()

	id := uuid.New()
	repo.books = []catalogDomain.Book{{ID: id, Title: "Книга", Slug: "kniga", Language: "ru"}}

	_, err := svc.UpdateBook(ctx, id, BookInput{Title: "Updated Book", Language: "ru"})
	if !errors.Is(err, ErrTitleNotRussian) {
		t.Fatalf("expected title not russian error, got %v", err)
	}
}

func TestUpdateBook_NonRussianDescription(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()

	id := uuid.New()
	repo.books = []catalogDomain.Book{{ID: id, Title: "Книга", Slug: "kniga", Language: "ru"}}

	desc := "An English description"
	_, err := svc.UpdateBook(ctx, id, BookInput{Title: "Книга", Language: "ru", Description: &desc})
	if !errors.Is(err, ErrDescriptionNotRussian) {
		t.Fatalf("expected description not russian error, got %v", err)
	}
}

func TestCreateBook_LatinTitleWithAllowFlag_Succeeds(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	book, err := svc.CreateBook(ctx, BookInput{Title: "Empire V", Language: "ru", AllowNonRussianTitle: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if book == nil || book.Title != "Empire V" {
		t.Fatalf("expected created book titled %q, got %+v", "Empire V", book)
	}
}

func TestCreateBook_AllowFlagDoesNotLiftDescriptionCheck(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	desc := "An English description"
	_, err := svc.CreateBook(ctx, BookInput{Title: "Empire V", Language: "ru", Description: &desc, AllowNonRussianTitle: true})
	if !errors.Is(err, ErrDescriptionNotRussian) {
		t.Fatalf("expected description not russian error, got %v", err)
	}
}

func TestCreateBook_AllowFlagDoesNotLiftEmptyTitleCheck(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	_, err := svc.CreateBook(ctx, BookInput{Title: "   ", Language: "ru", AllowNonRussianTitle: true})
	if !errors.Is(err, ErrInvalidTitle) {
		t.Fatalf("expected invalid title error, got %v", err)
	}
}

func TestUpdateBook_UnchangedLatinTitleWithoutFlag_Succeeds(t *testing.T) {
	// Grandfathering: a stored Latin-script title is not new text, so
	// editing other fields (or publishing) needs no opt-in flag.
	svc, repo := newService()
	ctx := context.Background()

	id := uuid.New()
	repo.books = []catalogDomain.Book{{ID: id, Title: "Empire V", Slug: "empire-v", Language: "ru", IsHidden: true}}

	book, err := svc.UpdateBook(ctx, id, BookInput{Title: "  Empire V  ", Slug: "empire-v", Language: "ru", IsHidden: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if book.IsHidden {
		t.Fatalf("expected book to be updated (is_hidden=false), got %+v", book)
	}
}

func TestUpdateBook_ChangedLatinTitle(t *testing.T) {
	newRepo := func() (*Service, uuid.UUID) {
		svc, repo := newService()
		id := uuid.New()
		repo.books = []catalogDomain.Book{{ID: id, Title: "Empire V", Slug: "empire-v", Language: "ru"}}
		return svc, id
	}

	t.Run("without flag is rejected", func(t *testing.T) {
		svc, id := newRepo()
		_, err := svc.UpdateBook(context.Background(), id, BookInput{Title: "Empire W", Slug: "empire-v", Language: "ru"})
		if !errors.Is(err, ErrTitleNotRussian) {
			t.Fatalf("expected title not russian error, got %v", err)
		}
	})

	t.Run("with flag succeeds", func(t *testing.T) {
		svc, id := newRepo()
		book, err := svc.UpdateBook(context.Background(), id, BookInput{Title: "Empire W", Slug: "empire-v", Language: "ru", AllowNonRussianTitle: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if book.Title != "Empire W" {
			t.Fatalf("expected title %q, got %q", "Empire W", book.Title)
		}
	})
}

func TestCreateAuthor_Valid(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()

	author, err := svc.CreateAuthor(ctx, AuthorInput{Name: "Leo Tolstoy"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if author.Slug != "leo-tolstoy" {
		t.Fatalf("expected slug leo-tolstoy, got %s", author.Slug)
	}
	if len(repo.authors) != 1 {
		t.Fatalf("expected 1 author, got %d", len(repo.authors))
	}
}

func TestCreateAuthor_DuplicateSlug(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()
	repo.authorSlugExists = true

	_, err := svc.CreateAuthor(ctx, AuthorInput{Name: "Author"})
	if !errors.Is(err, ErrDuplicateSlug) {
		t.Fatalf("expected duplicate slug error, got %v", err)
	}
}

func TestModerateReview_UpdatesReviewsCount(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()

	bookID := uuid.New()
	repo.books = []catalogDomain.Book{{ID: bookID, ReviewsCount: 2}}
	review1ID := uuid.New()
	review2ID := uuid.New()
	repo.reviews = []reviewsDomain.Review{
		{ID: review1ID, BookID: bookID, IsHidden: false},
		{ID: review2ID, BookID: bookID, IsHidden: false},
	}

	// Hide one review -> count drops to 1.
	review, err := svc.ModerateReview(ctx, review1ID, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !review.IsHidden {
		t.Fatalf("expected review to be hidden")
	}
	for _, b := range repo.books {
		if b.ID == bookID && b.ReviewsCount != 1 {
			t.Fatalf("expected reviews_count 1, got %d", b.ReviewsCount)
		}
	}

	// Hide same review again -> idempotent, count unchanged.
	_, err = svc.ModerateReview(ctx, review1ID, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, b := range repo.books {
		if b.ID == bookID && b.ReviewsCount != 1 {
			t.Fatalf("expected reviews_count to remain 1, got %d", b.ReviewsCount)
		}
	}

	// Restore it -> count returns to 2.
	_, err = svc.ModerateReview(ctx, review1ID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, b := range repo.books {
		if b.ID == bookID && b.ReviewsCount != 2 {
			t.Fatalf("expected reviews_count 2, got %d", b.ReviewsCount)
		}
	}
}

func TestCreateAuthor_InvalidDate(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	birth := "not-a-date"
	_, err := svc.CreateAuthor(ctx, AuthorInput{Name: "Author", BirthDate: &birth})
	if !errors.Is(err, ErrInvalidDate) {
		t.Fatalf("expected invalid date error, got %v", err)
	}
}

func TestCreateGenre_Valid(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()

	genre, err := svc.CreateGenre(ctx, GenreInput{Name: "Фантастика"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if genre.Slug != "fantastika" {
		t.Fatalf("expected slug fantastika, got %s", genre.Slug)
	}
	if genre.Name != "фантастика" {
		t.Fatalf("expected name to be lowercased to фантастика, got %s", genre.Name)
	}
	if len(repo.genres) != 1 {
		t.Fatalf("expected 1 genre, got %d", len(repo.genres))
	}
}

func TestCreateGenre_NormalizesFirstLetterCasing(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"uppercase cyrillic first letter", "Роман", "роман"},
		{"uppercase latin first letter", "Fantasy", "fantasy"},
		{"internal capitalization preserved", "Научная Фантастика", "научная Фантастика"},
		{"already lowercase is unaffected", "детектив", "детектив"},
		{"leading/trailing whitespace trimmed before normalizing", "  Роман  ", "роман"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService()
			ctx := context.Background()

			genre, err := svc.CreateGenre(ctx, GenreInput{Name: tc.input})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if genre.Name != tc.expected {
				t.Fatalf("expected name %q, got %q", tc.expected, genre.Name)
			}
		})
	}
}

func TestCreateGenre_EmptyName(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	_, err := svc.CreateGenre(ctx, GenreInput{Name: "   "})
	if !errors.Is(err, ErrInvalidGenreName) {
		t.Fatalf("expected ErrInvalidGenreName, got %v", err)
	}
}

func TestModerateReview_Idempotent(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()
	reviewID := uuid.New()
	repo.reviews = append(repo.reviews, reviewsDomain.Review{ID: reviewID, IsHidden: true})

	review, err := svc.ModerateReview(ctx, reviewID, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !review.IsHidden {
		t.Fatal("review should remain hidden")
	}

	review, err = svc.ModerateReview(ctx, reviewID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if review.IsHidden {
		t.Fatal("review should be restored")
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"War and Peace", "war-and-peace"},
		{"Война и мир", "voyna-i-mir"},
		{"Hello--World!", "hello-world"},
	}

	for _, tc := range cases {
		got := slugify(tc.input)
		if got != tc.expected {
			t.Errorf("slugify(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestIsValidISBN(t *testing.T) {
	valid := []string{"0-306-40615-2", "9780306406157"}
	invalid := []string{"123", "9780306406158"}

	for _, v := range valid {
		if !isValidISBN(v) {
			t.Errorf("expected %s to be valid", v)
		}
	}
	for _, v := range invalid {
		if isValidISBN(v) {
			t.Errorf("expected %s to be invalid", v)
		}
	}
}

func TestCreateAuthor_ForcesPublishedIgnoringInputStatus(t *testing.T) {
	svc, repo := newService()
	pending := catalogDomain.AuthorStatusPending

	author, err := svc.CreateAuthor(context.Background(), AuthorInput{Name: "Leo Tolstoy", Status: &pending})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if author.Status != catalogDomain.AuthorStatusPublished || repo.authors[0].Status != catalogDomain.AuthorStatusPublished {
		t.Fatalf("expected published author, got %q", author.Status)
	}
}

func TestUpdateAuthor_Status(t *testing.T) {
	rejected := catalogDomain.AuthorStatusRejected
	bogus := "deleted"
	tests := []struct {
		name    string
		status  *string
		want    string
		wantErr error
	}{
		{name: "valid status applied", status: &rejected, want: catalogDomain.AuthorStatusRejected},
		{name: "omitted status unchanged", status: nil, want: catalogDomain.AuthorStatusPending},
		{name: "invalid status rejected", status: &bogus, wantErr: ErrInvalidAuthorStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newService()
			id := uuid.New()
			repo.authors = []catalogDomain.Author{{ID: id, Name: "Leo Tolstoy", Slug: "leo-tolstoy", Status: catalogDomain.AuthorStatusPending}}

			_, err := svc.UpdateAuthor(context.Background(), id, AuthorInput{Name: "Leo Tolstoy", Slug: "leo-tolstoy", Status: tt.status})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				if repo.authors[0].Status != catalogDomain.AuthorStatusPending {
					t.Fatalf("status must stay unchanged on error, got %q", repo.authors[0].Status)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.authors[0].Status != tt.want {
				t.Fatalf("expected status %q, got %q", tt.want, repo.authors[0].Status)
			}
		})
	}
}

func TestListAuthors_StatusFilter(t *testing.T) {
	svc, repo := newService()
	repo.authors = []catalogDomain.Author{
		{ID: uuid.New(), Name: "A", Slug: "a", Status: catalogDomain.AuthorStatusPublished},
		{ID: uuid.New(), Name: "B", Slug: "b", Status: catalogDomain.AuthorStatusPending},
	}

	all, total, err := svc.ListAuthors(context.Background(), "", "", 1, 20)
	if err != nil || total != 2 || len(all) != 2 {
		t.Fatalf("expected all statuses, got %d/%d err=%v", len(all), total, err)
	}
	pending, total, err := svc.ListAuthors(context.Background(), "", catalogDomain.AuthorStatusPending, 1, 20)
	if err != nil || total != 1 || pending[0].Name != "B" {
		t.Fatalf("expected only pending author, got %+v err=%v", pending, err)
	}
	if _, _, err := svc.ListAuthors(context.Background(), "", "bogus", 1, 20); !errors.Is(err, ErrInvalidAuthorStatus) {
		t.Fatalf("expected ErrInvalidAuthorStatus, got %v", err)
	}
}

func TestDashboardCounts_PendingAuthors(t *testing.T) {
	svc, repo := newService()
	repo.authors = []catalogDomain.Author{
		{ID: uuid.New(), Status: catalogDomain.AuthorStatusPending},
		{ID: uuid.New(), Status: catalogDomain.AuthorStatusPending},
		{ID: uuid.New(), Status: catalogDomain.AuthorStatusPublished},
		{ID: uuid.New(), Status: catalogDomain.AuthorStatusRejected},
	}

	stats, err := svc.DashboardCounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.PendingAuthors != 2 || stats.Authors != 4 {
		t.Fatalf("expected 2 pending of 4 authors, got %+v", stats)
	}
}

func (f *fakeRepository) ListAuthorsForModeration(ctx context.Context, filter repository.AuthorModerationFilter) ([]repository.AuthorModerationRow, int64, error) {
	return nil, 0, nil
}

func (f *fakeRepository) ListAuthorSampleBooks(ctx context.Context, authorIDs []uuid.UUID) (map[uuid.UUID][]repository.SampleBook, error) {
	return map[uuid.UUID][]repository.SampleBook{}, nil
}

func (f *fakeRepository) FindAuthorsBySurnames(ctx context.Context, surnames []string, excludeIDs []uuid.UUID) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeRepository) SetAuthorStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	return false, nil
}
