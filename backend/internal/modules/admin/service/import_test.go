package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// fakeImportRepository is a dynamic in-memory repository.Repository used
// only by TestImportBooks. Unlike fakeRepository (service_test.go), which
// uses static canned booleans for slug/relationship existence checks
// (convenient for single-call tests), this fake computes them from records
// actually created during the test - required for scenarios like "two rows
// with the same title in one batch", where the second row must only be
// detected as a duplicate *because* the first row was actually created
// earlier in the same ImportBooks call. Mirrors
// harvest/service/handler_test.go's fakeAdminRepository, which needed the
// same dynamic behavior for an analogous reason.
type fakeImportRepository struct {
	books   []catalogDomain.Book
	authors []catalogDomain.Author
	genres  []catalogDomain.Genre
	// createAuthorErr, when set, is consulted before each CreateAuthor; a
	// non-nil result fails that creation (partial resolver failure tests).
	createAuthorErr func(name string) error
	// createBookErr, when set, is consulted before each CreateBook; a
	// non-nil result fails that creation (post-check failure tests).
	createBookErr func(book *catalogDomain.Book) error
	// transactions counts WithinTransaction calls.
	transactions int
}

// WithinTransaction simulates a real transaction: it snapshots books,
// authors and genres and restores them when fn fails, so rollback tests
// observe what a database would.
func (f *fakeImportRepository) WithinTransaction(ctx context.Context, fn func(repo repository.Repository) error) error {
	f.transactions++
	books := append([]catalogDomain.Book(nil), f.books...)
	authors := append([]catalogDomain.Author(nil), f.authors...)
	genres := append([]catalogDomain.Genre(nil), f.genres...)
	if err := fn(f); err != nil {
		f.books, f.authors, f.genres = books, authors, genres
		return err
	}
	return nil
}

var _ repository.Repository = (*fakeImportRepository)(nil)

func (f *fakeImportRepository) DashboardCounts(ctx context.Context) (repository.DashboardStats, error) {
	return repository.DashboardStats{Books: int64(len(f.books)), Authors: int64(len(f.authors)), Genres: int64(len(f.genres))}, nil
}

func (f *fakeImportRepository) ListBooks(ctx context.Context, filter repository.BookFilter) ([]catalogDomain.Book, int64, error) {
	return f.books, int64(len(f.books)), nil
}

func (f *fakeImportRepository) GetBookByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error) {
	for i := range f.books {
		if f.books[i].ID == id {
			return &f.books[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeImportRepository) GetBookByExternalID(ctx context.Context, source, id string) (*catalogDomain.Book, error) {
	for i := range f.books {
		b := f.books[i]
		if b.ExternalSource != nil && *b.ExternalSource == source && b.ExternalID != nil && *b.ExternalID == id {
			return &f.books[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeImportRepository) CreateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	if f.createBookErr != nil {
		if err := f.createBookErr(book); err != nil {
			return err
		}
	}
	f.books = append(f.books, *book)
	return nil
}

func (f *fakeImportRepository) UpdateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	for i := range f.books {
		if f.books[i].ID == book.ID {
			f.books[i] = *book
			return nil
		}
	}
	return errors.New("record not found")
}

func (f *fakeImportRepository) ListBooksWithOpenLibraryCover(ctx context.Context) ([]catalogDomain.Book, error) {
	return nil, nil
}

func (f *fakeImportRepository) BookSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	for _, b := range f.books {
		if b.Slug == slug && (excludeID == nil || b.ID != *excludeID) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeImportRepository) ListAuthors(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Author, int64, error) {
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

func (f *fakeImportRepository) GetAuthorByExternalID(ctx context.Context, source, id string) (*catalogDomain.Author, error) {
	for i := range f.authors {
		a := f.authors[i]
		if a.ExternalSource != nil && *a.ExternalSource == source && a.ExternalID != nil && *a.ExternalID == id {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeImportRepository) SetAuthorExternalID(ctx context.Context, id uuid.UUID, source, externalID string) error {
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

func (f *fakeImportRepository) GetAuthorByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error) {
	for i := range f.authors {
		if f.authors[i].ID == id {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeImportRepository) GetAuthorBySlug(ctx context.Context, slug string) (*catalogDomain.Author, error) {
	for i := range f.authors {
		if f.authors[i].Slug == slug {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeImportRepository) CreateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	if f.createAuthorErr != nil {
		if err := f.createAuthorErr(author.Name); err != nil {
			return err
		}
	}
	f.authors = append(f.authors, *author)
	return nil
}

func (f *fakeImportRepository) UpdateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	for i := range f.authors {
		if f.authors[i].ID == author.ID {
			f.authors[i] = *author
			return nil
		}
	}
	return errors.New("record not found")
}

func (f *fakeImportRepository) ListAuthorsByExternalIDs(ctx context.Context, source string, ids []string) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeImportRepository) AuthorSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	for _, a := range f.authors {
		if a.Slug == slug && (excludeID == nil || a.ID != *excludeID) {
			return true, nil
		}
	}
	return false, nil
}

// MergeAuthors is a minimal fake satisfying repository.Repository; no
// existing test in this file exercises it.
func (f *fakeImportRepository) MergeAuthors(ctx context.Context, duplicateID, canonicalID uuid.UUID) (int64, error) {
	for i := range f.authors {
		if f.authors[i].ID == duplicateID {
			f.authors = append(f.authors[:i], f.authors[i+1:]...)
			break
		}
	}
	return 0, nil
}

func (f *fakeImportRepository) ListGenres(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Genre, int64, error) {
	return f.genres, int64(len(f.genres)), nil
}

func (f *fakeImportRepository) GetGenreByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error) {
	for i := range f.genres {
		if f.genres[i].ID == id {
			return &f.genres[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeImportRepository) GetGenreBySlug(ctx context.Context, slug string) (*catalogDomain.Genre, error) {
	for i := range f.genres {
		if f.genres[i].Slug == slug {
			return &f.genres[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeImportRepository) GetGenreByQID(ctx context.Context, qid string) (*catalogDomain.Genre, error) {
	for i := range f.genres {
		if f.genres[i].WikidataQID != nil && *f.genres[i].WikidataQID == qid {
			return &f.genres[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeImportRepository) ListGenresByQIDs(ctx context.Context, qids []string) ([]catalogDomain.Genre, error) {
	return []catalogDomain.Genre{}, nil
}

func (f *fakeImportRepository) SetGenreQID(ctx context.Context, id uuid.UUID, qid string) error {
	for i := range f.genres {
		if f.genres[i].ID == id {
			q := qid
			f.genres[i].WikidataQID = &q
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *fakeImportRepository) CreateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	f.genres = append(f.genres, *genre)
	return nil
}

func (f *fakeImportRepository) UpdateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	for i := range f.genres {
		if f.genres[i].ID == genre.ID {
			f.genres[i] = *genre
			return nil
		}
	}
	return errors.New("record not found")
}

func (f *fakeImportRepository) GenreSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	for _, g := range f.genres {
		if g.Slug == slug && (excludeID == nil || g.ID != *excludeID) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeImportRepository) ListReviews(ctx context.Context, filter repository.ReviewFilter) ([]reviewsDomain.Review, int64, error) {
	return nil, 0, nil
}

func (f *fakeImportRepository) GetReviewByID(ctx context.Context, id uuid.UUID) (*reviewsDomain.Review, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeImportRepository) ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeImportRepository) AuthorsExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
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

func (f *fakeImportRepository) GenresExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
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

func newImportService() (*Service, *fakeImportRepository) {
	repo := &fakeImportRepository{}
	return NewService(repo), repo
}

func TestImportBooks(t *testing.T) {
	t.Run("empty input returns a zero report", func(t *testing.T) {
		svc, _ := newImportService()

		report := svc.ImportBooks(context.Background(), nil, "")

		if report.Processed != 0 || report.Created != 0 || report.SkippedExisting != 0 || len(report.Errors) != 0 {
			t.Fatalf("expected zero report, got %+v", report)
		}
	})

	t.Run("success: creates a book and resolves authors/genres by name", func(t *testing.T) {
		svc, repo := newImportService()
		rows := []ImportRowInput{
			{Title: "Война и мир", Authors: []string{"Leo Tolstoy"}, Genres: []string{"Novel"}, Language: "ru"},
		}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Processed != 1 || report.Created != 1 || report.SkippedExisting != 0 || len(report.Errors) != 0 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if len(repo.books) != 1 || repo.books[0].Title != "Война и мир" {
			t.Fatalf("expected book to be created, got %+v", repo.books)
		}
		if repo.books[0].Status != catalogDomain.BookStatusPublished {
			t.Fatalf("expected default batch status published, got %q", repo.books[0].Status)
		}
		if repo.books[0].PublishedAt == nil {
			t.Fatal("expected a published import to set the first-publication time")
		}
		if repo.books[0].ExternalSource != nil || repo.books[0].ExternalID != nil {
			t.Fatalf("expected external_source/external_id to stay nil, got %v / %v", repo.books[0].ExternalSource, repo.books[0].ExternalID)
		}
		if len(repo.authors) != 1 || repo.authors[0].Name != "Leo Tolstoy" {
			t.Fatalf("expected author to be autocreated, got %+v", repo.authors)
		}
		if repo.authors[0].Status != catalogDomain.AuthorStatusPublished {
			t.Fatalf("expected import-created author to be published, got %q", repo.authors[0].Status)
		}
		if len(repo.genres) != 1 || repo.genres[0].Name != "novel" {
			t.Fatalf("expected genre to be autocreated with normalized casing, got %+v", repo.genres)
		}
	})

	t.Run("language defaults to ru when omitted", func(t *testing.T) {
		svc, repo := newImportService()
		rows := []ImportRowInput{{Title: "Книга", Authors: []string{"Author"}}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Created != 1 {
			t.Fatalf("expected 1 created, got %+v", report)
		}
		if repo.books[0].Language != "ru" {
			t.Fatalf("expected default language ru, got %q", repo.books[0].Language)
		}
	})

	t.Run("batch status pending is respected", func(t *testing.T) {
		svc, repo := newImportService()
		rows := []ImportRowInput{{Title: "Книга", Authors: []string{"Author"}}, {Title: "Другая книга", Authors: []string{"Author"}}}

		report := svc.ImportBooks(context.Background(), rows, catalogDomain.BookStatusPending)

		if report.Created != 2 {
			t.Fatalf("expected 2 created, got %+v", report)
		}
		for _, b := range repo.books {
			if b.Status != catalogDomain.BookStatusPending {
				t.Fatalf("expected every book to be pending per the batch status, got %q", b.Status)
			}
			if b.PublishedAt != nil {
				t.Fatalf("expected a pending import to leave published_at nil, got %v", b.PublishedAt)
			}
		}
	})

	t.Run("zero resolvable authors is a row error (E-02)", func(t *testing.T) {
		svc, repo := newImportService()
		rows := []ImportRowInput{{Title: "Book", Authors: []string{"   "}}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Processed != 1 || report.Created != 0 || len(report.Errors) != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if report.Errors[0].Row != 1 || report.Errors[0].Title != "Book" {
			t.Fatalf("unexpected error entry: %+v", report.Errors[0])
		}
		if len(repo.books) != 0 {
			t.Fatalf("expected no book created, got %+v", repo.books)
		}
	})

	t.Run("missing title is a row error, reusing CreateBook validation", func(t *testing.T) {
		svc, _ := newImportService()
		rows := []ImportRowInput{{Title: "", Authors: []string{"Author"}}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Created != 0 || len(report.Errors) != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("invalid ISBN is a row error, reusing CreateBook validation", func(t *testing.T) {
		svc, _ := newImportService()
		badISBN := "123"
		rows := []ImportRowInput{{Title: "Книга", Authors: []string{"Author"}, ISBN: &badISBN}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Created != 0 || len(report.Errors) != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("duplicate slug is skipped_existing, not an error, and does not modify the existing book", func(t *testing.T) {
		svc, repo := newImportService()
		repo.books = append(repo.books, catalogDomain.Book{ID: uuid.New(), Title: "Existing", Slug: "book"})
		rows := []ImportRowInput{{Title: "Книга", Slug: "book", Authors: []string{"Author"}}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.SkippedExisting != 1 || report.Created != 0 || len(report.Errors) != 0 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if len(repo.books) != 1 || repo.books[0].Title != "Existing" {
			t.Fatalf("expected the existing book to be untouched, got %+v", repo.books)
		}
	})

	t.Run("two rows with the same title in one file: first created, second naturally skipped", func(t *testing.T) {
		svc, repo := newImportService()
		rows := []ImportRowInput{
			{Title: "Книга", Authors: []string{"Author"}},
			{Title: "Книга", Authors: []string{"Author"}},
		}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Processed != 2 || report.Created != 1 || report.SkippedExisting != 1 || len(report.Errors) != 0 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if len(repo.books) != 1 {
			t.Fatalf("expected exactly 1 book created, got %d", len(repo.books))
		}
	})

	t.Run("mixed batch: partial success alongside a skip and an error", func(t *testing.T) {
		svc, repo := newImportService()
		repo.books = append(repo.books, catalogDomain.Book{ID: uuid.New(), Title: "Existing", Slug: "existing-book"})
		rows := []ImportRowInput{
			{Title: "Новая книга", Authors: []string{"Author"}},
			{Title: "Существующая книга", Slug: "existing-book", Authors: []string{"Author"}},
			{Title: "", Authors: []string{"Author"}},
		}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Processed != 3 || report.Created != 1 || report.SkippedExisting != 1 || len(report.Errors) != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if report.Errors[0].Row != 3 {
			t.Fatalf("expected the error to be reported on row 3, got %+v", report.Errors[0])
		}
	})

	t.Run("non-Russian title is a row error, reusing CreateBook validation, other rows still succeed", func(t *testing.T) {
		svc, repo := newImportService()
		rows := []ImportRowInput{
			{Title: "Non-Russian Title", Authors: []string{"Author"}},
			{Title: "Книга", Authors: []string{"Author"}},
		}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Processed != 2 || report.Created != 1 || len(report.Errors) != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if report.Errors[0].Row != 1 || report.Errors[0].Title != "Non-Russian Title" {
			t.Fatalf("unexpected error entry: %+v", report.Errors[0])
		}
		if report.Errors[0].Message != ErrTitleNotRussian.Error() {
			t.Fatalf("unexpected error message: %q", report.Errors[0].Message)
		}
		if len(repo.books) != 1 || repo.books[0].Title != "Книга" {
			t.Fatalf("expected exactly 1 book created, got %+v", repo.books)
		}
	})
}

// TestImportBooks_CreatedEntities covers the report's created_authors /
// created_genres (openspec change report-import-created-entities) and the
// rule that a row changes the catalog only when its book is created
// (openspec change import-validate-before-create).
func TestImportBooks_CreatedEntities(t *testing.T) {
	t.Run("new author and new genre are listed with their row and stored name", func(t *testing.T) {
		svc, repo := newImportService()
		rows := []ImportRowInput{{Title: "Книга", Authors: []string{" Иван Новиков "}, Genres: []string{"Киберпанк"}}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Created != 1 {
			t.Fatalf("expected 1 created, got %+v", report)
		}
		wantAuthors := []ImportCreatedEntity{{ID: repo.authors[0].ID.String(), Name: "Иван Новиков", Row: 1}}
		if !reflect.DeepEqual(report.CreatedAuthors, wantAuthors) {
			t.Fatalf("created_authors = %+v, want %+v", report.CreatedAuthors, wantAuthors)
		}
		if repo.genres[0].Name != "киберпанк" {
			t.Fatalf("expected the stored (normalized) genre name, got %q", repo.genres[0].Name)
		}
		wantGenres := []ImportCreatedEntity{{ID: repo.genres[0].ID.String(), Name: "киберпанк", Row: 1}}
		if !reflect.DeepEqual(report.CreatedGenres, wantGenres) {
			t.Fatalf("created_genres = %+v, want %+v", report.CreatedGenres, wantGenres)
		}
	})

	t.Run("existing entities and a merged genre are not listed", func(t *testing.T) {
		svc, repo := newImportService()
		repo.authors = append(repo.authors, catalogDomain.Author{ID: uuid.New(), Name: "Лев Толстой", Slug: slugify("Лев Толстой"), Status: catalogDomain.AuthorStatusPublished})
		targetID := uuid.New()
		repo.genres = append(repo.genres,
			catalogDomain.Genre{ID: targetID, Name: "роман", Slug: slugify("роман"), Status: catalogDomain.GenreStatusPublished},
			catalogDomain.Genre{ID: uuid.New(), Name: "старый роман", Slug: slugify("старый роман"), Status: catalogDomain.GenreStatusRejected, MergedIntoID: &targetID},
		)
		rows := []ImportRowInput{{Title: "Книга", Authors: []string{"Лев Толстой"}, Genres: []string{"роман", "старый роман"}}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Created != 1 || len(report.Errors) != 0 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if report.CreatedAuthors == nil || len(report.CreatedAuthors) != 0 {
			t.Fatalf("expected empty created_authors, got %#v", report.CreatedAuthors)
		}
		if report.CreatedGenres == nil || len(report.CreatedGenres) != 0 {
			t.Fatalf("expected empty created_genres, got %#v", report.CreatedGenres)
		}
		if len(repo.genres) != 2 || len(repo.authors) != 1 {
			t.Fatalf("expected nothing created, got %d genres / %d authors", len(repo.genres), len(repo.authors))
		}
	})

	t.Run("same new author on several rows is listed once with the first row", func(t *testing.T) {
		svc, repo := newImportService()
		rows := []ImportRowInput{
			{Title: "Первая", Authors: []string{"Другой Автор"}},
			{Title: "Вторая", Authors: []string{"Новый Автор"}},
			{Title: "Третья", Authors: []string{"Другой Автор"}},
			{Title: "Четвёртая", Authors: []string{"Другой Автор"}},
			{Title: "Пятая", Authors: []string{"Новый Автор"}},
		}

		report := svc.ImportBooks(context.Background(), rows, "")

		if len(repo.authors) != 2 {
			t.Fatalf("expected 2 authors created, got %+v", repo.authors)
		}
		want := []ImportCreatedEntity{
			{ID: repo.authors[0].ID.String(), Name: "Другой Автор", Row: 1},
			{ID: repo.authors[1].ID.String(), Name: "Новый Автор", Row: 2},
		}
		if !reflect.DeepEqual(report.CreatedAuthors, want) {
			t.Fatalf("created_authors = %+v, want %+v", report.CreatedAuthors, want)
		}
	})

	t.Run("a row that fails validation creates no author or genre", func(t *testing.T) {
		svc, repo := newImportService()
		badISBN := "123"
		rows := []ImportRowInput{
			{Title: "Первая", Authors: []string{"Автор"}},
			{Title: "Вторая", Authors: []string{"Автор"}},
			{Title: "Третья", Authors: []string{"Сирота"}, Genres: []string{"новый жанр"}, ISBN: &badISBN},
		}

		report := svc.ImportBooks(context.Background(), rows, "")

		if len(report.Errors) != 1 || report.Errors[0].Row != 3 || report.Errors[0].Message != ErrInvalidISBN.Error() {
			t.Fatalf("expected one invalid-ISBN error on row 3, got %+v", report.Errors)
		}
		if len(repo.authors) != 1 || repo.authors[0].Name != "Автор" {
			t.Fatalf("expected only the valid rows' author in the catalog, got %+v", repo.authors)
		}
		if len(repo.genres) != 0 {
			t.Fatalf("expected no genre created by the failed row, got %+v", repo.genres)
		}
		want := []ImportCreatedEntity{{ID: repo.authors[0].ID.String(), Name: "Автор", Row: 1}}
		if !reflect.DeepEqual(report.CreatedAuthors, want) {
			t.Fatalf("created_authors = %+v, want %+v", report.CreatedAuthors, want)
		}
		if len(report.CreatedGenres) != 0 {
			t.Fatalf("expected empty created_genres, got %+v", report.CreatedGenres)
		}
		if repo.transactions != 2 {
			t.Fatalf("expected the failed row to open no transaction, got %d transactions", repo.transactions)
		}
	})

	t.Run("a skipped_existing row creates no author or genre", func(t *testing.T) {
		svc, repo := newImportService()
		repo.books = append(repo.books, catalogDomain.Book{ID: uuid.New(), Title: "Existing", Slug: "existing-book"})
		repo.authors = append(repo.authors, catalogDomain.Author{ID: uuid.New(), Name: "Автор", Slug: slugify("Автор"), Status: catalogDomain.AuthorStatusPublished})
		rows := []ImportRowInput{
			{Title: "Новая", Authors: []string{"Автор"}},
			{Title: "Существующая", Slug: "existing-book", Authors: []string{"Новый Автор"}, Genres: []string{"новый жанр"}},
		}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.SkippedExisting != 1 || report.Created != 1 || len(report.Errors) != 0 {
			t.Fatalf("expected the second row skipped, got %+v", report)
		}
		if len(repo.genres) != 0 || len(repo.authors) != 1 {
			t.Fatalf("expected nothing created by the skipped row, got %d genres / %d authors", len(repo.genres), len(repo.authors))
		}
		if repo.books[0].Title != "Existing" {
			t.Fatalf("expected the existing book unchanged, got %+v", repo.books[0])
		}
		if len(report.CreatedGenres) != 0 || len(report.CreatedAuthors) != 0 {
			t.Fatalf("expected empty created arrays, got %+v / %+v", report.CreatedAuthors, report.CreatedGenres)
		}
	})

	t.Run("a row without authors creates no genre", func(t *testing.T) {
		svc, repo := newImportService()
		rows := []ImportRowInput{{Title: "Книга", Authors: []string{" ", ""}, Genres: []string{"новый жанр"}}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if len(report.Errors) != 1 || report.Errors[0].Message != errImportNoAuthors.Error() {
			t.Fatalf("expected a no-authors row error, got %+v", report.Errors)
		}
		if len(repo.genres) != 0 || len(report.CreatedGenres) != 0 {
			t.Fatalf("expected no genre created, got %+v / %+v", repo.genres, report.CreatedGenres)
		}
	})

	t.Run("new author first named on a failed row is listed with the creating row", func(t *testing.T) {
		svc, repo := newImportService()
		badISBN := "123"
		rows := []ImportRowInput{
			{Title: "Первая", Authors: []string{"Автор"}},
			{Title: "Вторая", Authors: []string{"Новый Автор"}, ISBN: &badISBN},
			{Title: "Третья", Authors: []string{"Автор"}},
			{Title: "Четвёртая", Authors: []string{"Автор"}},
			{Title: "Пятая", Authors: []string{"Новый Автор"}},
		}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Created != 4 || len(report.Errors) != 1 || report.Errors[0].Row != 2 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if len(repo.authors) != 2 {
			t.Fatalf("expected 2 authors created, got %+v", repo.authors)
		}
		want := []ImportCreatedEntity{
			{ID: repo.authors[0].ID.String(), Name: "Автор", Row: 1},
			{ID: repo.authors[1].ID.String(), Name: "Новый Автор", Row: 5},
		}
		if !reflect.DeepEqual(report.CreatedAuthors, want) {
			t.Fatalf("created_authors = %+v, want %+v", report.CreatedAuthors, want)
		}
	})

	t.Run("an existing slug with an invalid ISBN is still skipped_existing", func(t *testing.T) {
		svc, repo := newImportService()
		repo.books = append(repo.books, catalogDomain.Book{ID: uuid.New(), Title: "Existing", Slug: "existing-book"})
		badISBN := "123"
		rows := []ImportRowInput{{Title: "Книга", Slug: "existing-book", Authors: []string{"Новый Автор"}, ISBN: &badISBN}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.SkippedExisting != 1 || len(report.Errors) != 0 {
			t.Fatalf("expected the row skipped, got %+v", report)
		}
		if len(repo.authors) != 0 || len(report.CreatedAuthors) != 0 {
			t.Fatalf("expected no author created, got %+v / %+v", repo.authors, report.CreatedAuthors)
		}
	})

	t.Run("valid rows around a failing row are unaffected", func(t *testing.T) {
		svc, repo := newImportService()
		rows := []ImportRowInput{
			{Title: "Первая", Authors: []string{"Первый Автор"}},
			{Title: "", Authors: []string{"Сирота"}},
			{Title: "Третья", Authors: []string{"Третий Автор"}},
		}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Created != 2 || len(report.Errors) != 1 || report.Errors[0].Row != 2 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if len(repo.books) != 2 || len(repo.authors) != 2 {
			t.Fatalf("expected 2 books / 2 authors, got %d / %d", len(repo.books), len(repo.authors))
		}
		want := []ImportCreatedEntity{
			{ID: repo.authors[0].ID.String(), Name: "Первый Автор", Row: 1},
			{ID: repo.authors[1].ID.String(), Name: "Третий Автор", Row: 3},
		}
		if !reflect.DeepEqual(report.CreatedAuthors, want) {
			t.Fatalf("created_authors = %+v, want %+v", report.CreatedAuthors, want)
		}
	})

	t.Run("empty file returns empty arrays, not nil", func(t *testing.T) {
		svc, _ := newImportService()

		report := svc.ImportBooks(context.Background(), []ImportRowInput{}, "")

		if report.CreatedAuthors == nil || report.CreatedGenres == nil {
			t.Fatalf("expected non-nil empty arrays, got %#v / %#v", report.CreatedAuthors, report.CreatedGenres)
		}
		if len(report.CreatedAuthors) != 0 || len(report.CreatedGenres) != 0 {
			t.Fatalf("expected empty arrays, got %+v", report)
		}
	})

	t.Run("partial resolver failure rolls back the already-created author", func(t *testing.T) {
		svc, repo := newImportService()
		repo.createAuthorErr = func(name string) error {
			if name == "Второй" {
				return errors.New("boom")
			}
			return nil
		}
		rows := []ImportRowInput{{Title: "Книга", Authors: []string{"Первый", "Второй"}}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if len(report.Errors) != 1 || report.Errors[0].Row != 1 {
			t.Fatalf("expected a row error, got %+v", report.Errors)
		}
		if len(repo.authors) != 0 || len(repo.books) != 0 {
			t.Fatalf("expected the row rolled back, got %+v / %+v", repo.authors, repo.books)
		}
		if len(report.CreatedAuthors) != 0 {
			t.Fatalf("expected empty created_authors, got %+v", report.CreatedAuthors)
		}
	})

	t.Run("book creation failure rolls back the row's new author and genre", func(t *testing.T) {
		svc, repo := newImportService()
		repo.createBookErr = func(book *catalogDomain.Book) error {
			if book.Title == "Сбой" {
				return errors.New("db down")
			}
			return nil
		}
		rows := []ImportRowInput{
			{Title: "Сбой", Authors: []string{"Иван Новиков"}, Genres: []string{"киберпанк"}},
			{Title: "Книга", Authors: []string{"Другой Автор"}},
		}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.Created != 1 || len(report.Errors) != 1 || report.Errors[0].Row != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if len(repo.authors) != 1 || repo.authors[0].Name != "Другой Автор" || len(repo.genres) != 0 {
			t.Fatalf("expected the failed row rolled back, got %+v / %+v", repo.authors, repo.genres)
		}
		want := []ImportCreatedEntity{{ID: repo.authors[0].ID.String(), Name: "Другой Автор", Row: 2}}
		if !reflect.DeepEqual(report.CreatedAuthors, want) || len(report.CreatedGenres) != 0 {
			t.Fatalf("unexpected created entities: %+v / %+v", report.CreatedAuthors, report.CreatedGenres)
		}
	})

	t.Run("book taken concurrently is skipped_existing and leaves nothing behind", func(t *testing.T) {
		svc, repo := newImportService()
		repo.createBookErr = func(book *catalogDomain.Book) error {
			return &pgconn.PgError{Code: "23505"}
		}
		rows := []ImportRowInput{{Title: "Книга", Authors: []string{"Иван Новиков"}, Genres: []string{"киберпанк"}}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.SkippedExisting != 1 || report.Created != 0 || len(report.Errors) != 0 {
			t.Fatalf("expected the row skipped, got %+v", report)
		}
		if len(repo.authors) != 0 || len(repo.genres) != 0 || len(repo.books) != 0 {
			t.Fatalf("expected nothing left behind, got %d authors / %d genres / %d books", len(repo.authors), len(repo.genres), len(repo.books))
		}
		if len(report.CreatedAuthors) != 0 || len(report.CreatedGenres) != 0 {
			t.Fatalf("expected empty created arrays, got %+v / %+v", report.CreatedAuthors, report.CreatedGenres)
		}
	})

	t.Run("duplicate slug from author creation is a row error, not a skip", func(t *testing.T) {
		svc, repo := newImportService()
		repo.createAuthorErr = func(name string) error {
			return &pgconn.PgError{Code: "23505"}
		}
		rows := []ImportRowInput{{Title: "Книга", Authors: []string{"Иван Новиков"}, Genres: []string{"киберпанк"}}}

		report := svc.ImportBooks(context.Background(), rows, "")

		if report.SkippedExisting != 0 || len(report.Errors) != 1 {
			t.Fatalf("expected a row error, got %+v", report)
		}
		if !strings.Contains(report.Errors[0].Message, ErrDuplicateSlug.Error()) {
			t.Fatalf("expected the row error to carry the author's duplicate slug, got %q", report.Errors[0].Message)
		}
		if len(repo.authors) != 0 || len(repo.genres) != 0 || len(repo.books) != 0 {
			t.Fatalf("expected nothing left behind, got %d authors / %d genres / %d books", len(repo.authors), len(repo.genres), len(repo.books))
		}
	})
}

// TestImportRowInput_HasNoTitleLanguageBypass guards that bulk CSV/JSON
// import can never lift the Russian-title check: ImportRowInput (the only
// thing the CSV/JSON parsers produce) must not carry an
// AllowNonRussianTitle-style field, so a JSON row with
// "allow_non_russian_title": true is silently ignored and the title is
// still validated (see the non-Russian title subtest in TestImportBooks
// and docs/features/latin-book-titles.md).
func TestImportRowInput_HasNoTitleLanguageBypass(t *testing.T) {
	typ := reflect.TypeOf(ImportRowInput{})
	if _, ok := typ.FieldByName("AllowNonRussianTitle"); ok {
		t.Fatal("ImportRowInput must not expose AllowNonRussianTitle: bulk import has no title-language bypass")
	}
}

func (f *fakeImportRepository) ListAuthorsForModeration(ctx context.Context, filter repository.AuthorModerationFilter) ([]repository.AuthorModerationRow, int64, error) {
	return nil, 0, nil
}

func (f *fakeImportRepository) ListAuthorSampleBooks(ctx context.Context, authorIDs []uuid.UUID) (map[uuid.UUID][]repository.SampleBook, error) {
	return map[uuid.UUID][]repository.SampleBook{}, nil
}

func (f *fakeImportRepository) FindAuthorsBySurnames(ctx context.Context, surnames []string, excludeIDs []uuid.UUID) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeImportRepository) SetAuthorStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	return false, nil
}

func (f *fakeImportRepository) GetAuthorsByIDs(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeImportRepository) FillAuthorEnrichment(ctx context.Context, id uuid.UUID, fill repository.AuthorEnrichmentFill) error {
	return nil
}

func (f *fakeImportRepository) ListUsers(ctx context.Context, filter repository.UserFilter) ([]usersDomain.User, int64, error) {
	return nil, 0, nil
}

func (f *fakeImportRepository) ChangeUserRole(ctx context.Context, targetID uuid.UUID, decide repository.RoleDecider) (*usersDomain.User, usersDomain.Role, error) {
	return nil, "", nil
}

func (f *fakeImportRepository) ListGenresForModeration(ctx context.Context, filter repository.GenreModerationFilter) ([]repository.GenreModerationRow, int64, error) {
	return nil, 0, nil
}

func (f *fakeImportRepository) ListGenreSampleBooks(ctx context.Context, genreIDs []uuid.UUID) (map[uuid.UUID][]repository.SampleBook, error) {
	return map[uuid.UUID][]repository.SampleBook{}, nil
}

func (f *fakeImportRepository) ListGenreHintCandidates(ctx context.Context) ([]catalogDomain.Genre, error) {
	return nil, nil
}

func (f *fakeImportRepository) SetGenreStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	return false, nil
}

func (f *fakeImportRepository) SetBookStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	return false, nil
}

func (f *fakeImportRepository) MergeGenres(ctx context.Context, sourceID, targetID uuid.UUID, actorID *uuid.UUID) (repository.GenreMergeResult, error) {
	return repository.GenreMergeResult{}, nil
}

func (f *fakeImportRepository) MergeGenresBatch(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID, actorID *uuid.UUID) (repository.GenreMergeResult, error) {
	return repository.GenreMergeResult{}, nil
}

func (f *fakeImportRepository) ListGenreMerges(ctx context.Context, page, limit int) ([]repository.GenreMergeEntry, int64, error) {
	return nil, 0, nil
}

func (f *fakeImportRepository) PreviewGenreMergeUndo(ctx context.Context, mergeID uuid.UUID) (repository.GenreMergeUndoPreview, error) {
	return repository.GenreMergeUndoPreview{}, nil
}

func (f *fakeImportRepository) UndoGenreMerge(ctx context.Context, mergeID uuid.UUID, actorID uuid.UUID) (repository.GenreMergeUndoResult, error) {
	return repository.GenreMergeUndoResult{}, nil
}

func (f *fakeImportRepository) PreviewGenreMerge(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID) (repository.GenreMergePreview, error) {
	return repository.GenreMergePreview{}, nil
}

func (f *fakeImportRepository) ListBooksForModeration(ctx context.Context, filter repository.BookModerationFilter) ([]catalogDomain.Book, int64, error) {
	return nil, 0, nil
}

func (f *fakeImportRepository) FindBooksByNormalizedISBNs(ctx context.Context, isbns []string, excludeIDs []uuid.UUID) ([]repository.BookHintCandidate, error) {
	return nil, nil
}

func (f *fakeImportRepository) FindBooksByAuthorIDs(ctx context.Context, authorIDs, excludeIDs []uuid.UUID) ([]repository.BookHintCandidate, error) {
	return nil, nil
}

func (f *fakeImportRepository) ListGenreAnalysisCandidates(ctx context.Context) ([]repository.GenreModerationRow, error) {
	return nil, nil
}

func (f *fakeImportRepository) GenresByIDs(ctx context.Context, ids []uuid.UUID) ([]repository.GenreStateRow, error) {
	return nil, nil
}
