package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// fakeBulkPublishRepository is a dynamic in-memory repository.Repository
// used only by TestBulkPublishBooks. ListBooks re-implements the
// pending_review + search filtering logic (mirroring
// GormRepository.ListBooks/admin/repository/repository_test.go's
// mockRepository.filterBooks) so "search is passed through to the
// underlying filter" and "total is unaffected by Limit" can actually be
// exercised, unlike fakeImportRepository/fakeRepository's ListBooks
// (import_test.go, service_test.go), which just return every book
// regardless of filter.
type fakeBulkPublishRepository struct {
	books   []catalogDomain.Book
	authors []catalogDomain.Author
	genres  []catalogDomain.Genre

	lastFilter repository.BookFilter
	failTitles map[string]bool // UpdateBook fails for a book with this title
}

var _ repository.Repository = (*fakeBulkPublishRepository)(nil)

func (f *fakeBulkPublishRepository) DashboardCounts(ctx context.Context) (repository.DashboardStats, error) {
	return repository.DashboardStats{Books: int64(len(f.books)), HiddenBooks: 0, Authors: int64(len(f.authors)), Genres: int64(len(f.genres))}, nil
}

func (f *fakeBulkPublishRepository) ListBooks(ctx context.Context, filter repository.BookFilter) ([]catalogDomain.Book, int64, error) {
	f.lastFilter = filter

	var matched []catalogDomain.Book
	for _, b := range f.books {
		switch filter.Visibility {
		case "visible":
			if b.IsHidden {
				continue
			}
		case "hidden":
			if !b.IsHidden {
				continue
			}
		case "pending_review":
			if !b.IsHidden || b.ExternalSource == nil {
				continue
			}
		}
		if filter.Search != "" && !strings.Contains(strings.ToLower(b.Title), strings.ToLower(filter.Search)) {
			continue
		}
		matched = append(matched, b)
	}

	total := int64(len(matched))
	limit := filter.Limit
	if limit > 0 && limit < len(matched) {
		matched = matched[:limit]
	}
	return matched, total, nil
}

func (f *fakeBulkPublishRepository) GetBookByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error) {
	for i := range f.books {
		if f.books[i].ID == id {
			return &f.books[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeBulkPublishRepository) GetBookByExternalID(ctx context.Context, source, id string) (*catalogDomain.Book, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeBulkPublishRepository) CreateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	f.books = append(f.books, *book)
	return nil
}

func (f *fakeBulkPublishRepository) UpdateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	if f.failTitles[book.Title] {
		return errors.New("simulated update failure")
	}
	for i := range f.books {
		if f.books[i].ID == book.ID {
			f.books[i] = *book
			return nil
		}
	}
	return errors.New("record not found")
}

func (f *fakeBulkPublishRepository) BookSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	for _, b := range f.books {
		if b.Slug == slug && (excludeID == nil || b.ID != *excludeID) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeBulkPublishRepository) ListAuthors(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Author, int64, error) {
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

func (f *fakeBulkPublishRepository) GetAuthorByExternalID(ctx context.Context, source, id string) (*catalogDomain.Author, error) {
	for i := range f.authors {
		a := f.authors[i]
		if a.ExternalSource != nil && *a.ExternalSource == source && a.ExternalID != nil && *a.ExternalID == id {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeBulkPublishRepository) SetAuthorExternalID(ctx context.Context, id uuid.UUID, source, externalID string) error {
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

func (f *fakeBulkPublishRepository) GetAuthorByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error) {
	for i := range f.authors {
		if f.authors[i].ID == id {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeBulkPublishRepository) GetAuthorBySlug(ctx context.Context, slug string) (*catalogDomain.Author, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeBulkPublishRepository) CreateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	f.authors = append(f.authors, *author)
	return nil
}

func (f *fakeBulkPublishRepository) UpdateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	return nil
}

func (f *fakeBulkPublishRepository) ListAuthorsByExternalIDs(ctx context.Context, source string, ids []string) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeBulkPublishRepository) AuthorSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	return false, nil
}

// MergeAuthors is a minimal fake satisfying repository.Repository; no
// existing test in this file exercises it.
func (f *fakeBulkPublishRepository) MergeAuthors(ctx context.Context, duplicateID, canonicalID uuid.UUID) (int64, error) {
	for i := range f.authors {
		if f.authors[i].ID == duplicateID {
			f.authors = append(f.authors[:i], f.authors[i+1:]...)
			break
		}
	}
	return 0, nil
}

func (f *fakeBulkPublishRepository) ListGenres(ctx context.Context, search string, limit, offset int) ([]catalogDomain.Genre, int64, error) {
	return f.genres, int64(len(f.genres)), nil
}

func (f *fakeBulkPublishRepository) GetGenreByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error) {
	for i := range f.genres {
		if f.genres[i].ID == id {
			return &f.genres[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeBulkPublishRepository) GetGenreBySlug(ctx context.Context, slug string) (*catalogDomain.Genre, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeBulkPublishRepository) CreateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	f.genres = append(f.genres, *genre)
	return nil
}

func (f *fakeBulkPublishRepository) UpdateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	return nil
}

func (f *fakeBulkPublishRepository) GenreSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	return false, nil
}

func (f *fakeBulkPublishRepository) ListReviews(ctx context.Context, filter repository.ReviewFilter) ([]reviewsDomain.Review, int64, error) {
	return nil, 0, nil
}

func (f *fakeBulkPublishRepository) GetReviewByID(ctx context.Context, id uuid.UUID) (*reviewsDomain.Review, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeBulkPublishRepository) ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeBulkPublishRepository) AuthorsExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
	return true, nil
}

func (f *fakeBulkPublishRepository) GenresExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
	return true, nil
}

func newBulkPublishService() (*Service, *fakeBulkPublishRepository) {
	repo := &fakeBulkPublishRepository{}
	return NewService(repo), repo
}

func pendingBook(title string) catalogDomain.Book {
	source := "wikidata"
	return catalogDomain.Book{
		ID:             uuid.New(),
		Title:          title,
		Slug:           strings.ToLower(strings.ReplaceAll(title, " ", "-")),
		Language:       "ru",
		IsHidden:       true,
		ExternalSource: &source,
	}
}

func TestBulkPublishBooks(t *testing.T) {
	t.Run("zero matching rows returns an empty report, not an error", func(t *testing.T) {
		svc, _ := newBulkPublishService()

		report, err := svc.BulkPublishBooks(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Processed != 0 || report.Published != 0 || len(report.Errors) != 0 {
			t.Fatalf("expected empty report, got %+v", report)
		}
	})

	t.Run("all-succeed: every pending_review book is published via UpdateBook", func(t *testing.T) {
		svc, repo := newBulkPublishService()
		repo.books = []catalogDomain.Book{
			pendingBook("Книга Один"),
			pendingBook("Книга Два"),
		}

		report, err := svc.BulkPublishBooks(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Processed != 2 || report.Published != 2 || len(report.Errors) != 0 {
			t.Fatalf("unexpected report: %+v", report)
		}
		for _, b := range repo.books {
			if b.IsHidden {
				t.Fatalf("expected book %q to be published (is_hidden=false), got %+v", b.Title, b)
			}
		}
	})

	t.Run("Latin-script title publishes via the unchanged-title rule", func(t *testing.T) {
		svc, repo := newBulkPublishService()
		repo.books = []catalogDomain.Book{pendingBook("Empire V")}

		report, err := svc.BulkPublishBooks(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Processed != 1 || report.Published != 1 || len(report.Errors) != 0 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if repo.books[0].IsHidden {
			t.Fatalf("expected %q to be published, got %+v", "Empire V", repo.books[0])
		}
	})

	t.Run("partial failure: one row's UpdateBook error is recorded and the rest of the batch still proceeds", func(t *testing.T) {
		svc, repo := newBulkPublishService()
		repo.books = []catalogDomain.Book{
			pendingBook("Хорошая книга"),
			pendingBook("Плохая книга"),
		}
		repo.failTitles = map[string]bool{"Плохая книга": true}

		report, err := svc.BulkPublishBooks(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Processed != 2 || report.Published != 1 || len(report.Errors) != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if report.Errors[0].Title != "Плохая книга" {
			t.Fatalf("expected the failing row's title in the error, got %+v", report.Errors[0])
		}
	})

	t.Run("too-large: more than bulkPublishCap matches returns the sentinel error and publishes nothing", func(t *testing.T) {
		svc, repo := newBulkPublishService()
		for i := 0; i < bulkPublishCap+1; i++ {
			repo.books = append(repo.books, pendingBook("Книга"))
		}

		report, err := svc.BulkPublishBooks(context.Background(), "")
		if !errors.Is(err, ErrBulkPublishTooLarge) {
			t.Fatalf("expected ErrBulkPublishTooLarge, got %v", err)
		}
		if report.Processed != 0 || report.Published != 0 {
			t.Fatalf("expected a zero report alongside the error, got %+v", report)
		}
		for _, b := range repo.books {
			if !b.IsHidden {
				t.Fatal("expected no book to be published when the batch is too large")
			}
		}
	})

	t.Run("search is passed through to the underlying pending_review filter", func(t *testing.T) {
		svc, repo := newBulkPublishService()
		repo.books = []catalogDomain.Book{
			pendingBook("Война и мир"),
			pendingBook("Анна Каренина"),
		}

		report, err := svc.BulkPublishBooks(context.Background(), "Война")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Processed != 1 || report.Published != 1 {
			t.Fatalf("expected search to narrow the batch to 1 row, got %+v", report)
		}
		if repo.lastFilter.Visibility != "pending_review" {
			t.Fatalf("expected the service to hardcode Visibility=pending_review, got %q", repo.lastFilter.Visibility)
		}
		if repo.lastFilter.Search != "Война" {
			t.Fatalf("expected search to be passed through, got %q", repo.lastFilter.Search)
		}
		for _, b := range repo.books {
			if b.Title == "Анна Каренина" && b.IsHidden != true {
				t.Fatal("expected the non-matching search result to remain unpublished")
			}
		}
	})

	t.Run("non-pending-review books are never touched even if present in the repository", func(t *testing.T) {
		svc, repo := newBulkPublishService()
		visible := catalogDomain.Book{ID: uuid.New(), Title: "Обычная книга", Slug: "obychnaya-kniga", Language: "ru", IsHidden: false}
		hiddenManual := catalogDomain.Book{ID: uuid.New(), Title: "Скрытая книга", Slug: "skrytaya-kniga", Language: "ru", IsHidden: true}
		repo.books = []catalogDomain.Book{visible, hiddenManual, pendingBook("Импортированная книга")}

		report, err := svc.BulkPublishBooks(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Processed != 1 || report.Published != 1 {
			t.Fatalf("expected only the pending_review book to be processed, got %+v", report)
		}
	})
}

func (f *fakeBulkPublishRepository) ListAuthorsForModeration(ctx context.Context, filter repository.AuthorModerationFilter) ([]repository.AuthorModerationRow, int64, error) {
	return nil, 0, nil
}

func (f *fakeBulkPublishRepository) ListAuthorSampleBooks(ctx context.Context, authorIDs []uuid.UUID) (map[uuid.UUID][]repository.SampleBook, error) {
	return map[uuid.UUID][]repository.SampleBook{}, nil
}

func (f *fakeBulkPublishRepository) FindAuthorsBySurnames(ctx context.Context, surnames []string, excludeIDs []uuid.UUID) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeBulkPublishRepository) SetAuthorStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	return false, nil
}

func (f *fakeBulkPublishRepository) GetAuthorsByIDs(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeBulkPublishRepository) FillAuthorEnrichment(ctx context.Context, id uuid.UUID, fill repository.AuthorEnrichmentFill) error {
	return nil
}
