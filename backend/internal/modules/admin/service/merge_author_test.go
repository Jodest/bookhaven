package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// fakeMergeAuthorsRepository is a dynamic in-memory repository.Repository
// used only by TestMergeAuthors. Unlike the other fakes in this package
// (whose MergeAuthors is a stub just satisfying the interface), this one
// re-implements GormRepository.MergeAuthors' three-step reassign/dedupe/
// delete semantics over an in-memory books[i].Authors slice (modeling
// book_authors rows per book), so the dedupe-on-overlap business rule
// (docs/features/duplicate-author-merge.md) can actually be exercised.
type fakeMergeAuthorsRepository struct {
	books   []catalogDomain.Book
	authors []catalogDomain.Author
}

func (f *fakeMergeAuthorsRepository) WithinTransaction(ctx context.Context, fn func(repo repository.Repository) error) error {
	return fn(f)
}

var _ repository.Repository = (*fakeMergeAuthorsRepository)(nil)

func (f *fakeMergeAuthorsRepository) DashboardCounts(ctx context.Context) (repository.DashboardStats, error) {
	return repository.DashboardStats{Books: int64(len(f.books)), Authors: int64(len(f.authors)), Genres: 0}, nil
}

func (f *fakeMergeAuthorsRepository) ListBooks(ctx context.Context, filter repository.BookFilter) ([]catalogDomain.Book, int64, error) {
	return f.books, int64(len(f.books)), nil
}

func (f *fakeMergeAuthorsRepository) GetBookByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error) {
	for i := range f.books {
		if f.books[i].ID == id {
			return &f.books[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMergeAuthorsRepository) GetBookByExternalID(ctx context.Context, source, id string) (*catalogDomain.Book, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMergeAuthorsRepository) CreateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	f.books = append(f.books, *book)
	return nil
}

func (f *fakeMergeAuthorsRepository) UpdateBook(ctx context.Context, book *catalogDomain.Book, authorIDs, genreIDs []uuid.UUID) error {
	return nil
}

func (f *fakeMergeAuthorsRepository) ListBooksWithOpenLibraryCover(ctx context.Context) ([]catalogDomain.Book, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) BookSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	return false, nil
}

func (f *fakeMergeAuthorsRepository) ListAuthors(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Author, int64, error) {
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

func (f *fakeMergeAuthorsRepository) GetAuthorByExternalID(ctx context.Context, source, id string) (*catalogDomain.Author, error) {
	for i := range f.authors {
		a := f.authors[i]
		if a.ExternalSource != nil && *a.ExternalSource == source && a.ExternalID != nil && *a.ExternalID == id {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMergeAuthorsRepository) SetAuthorExternalID(ctx context.Context, id uuid.UUID, source, externalID string) error {
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

func (f *fakeMergeAuthorsRepository) GetAuthorByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error) {
	for i := range f.authors {
		if f.authors[i].ID == id {
			return &f.authors[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMergeAuthorsRepository) GetAuthorBySlug(ctx context.Context, slug string) (*catalogDomain.Author, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMergeAuthorsRepository) CreateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	f.authors = append(f.authors, *author)
	return nil
}

func (f *fakeMergeAuthorsRepository) UpdateAuthor(ctx context.Context, author *catalogDomain.Author) error {
	for i := range f.authors {
		if f.authors[i].ID == author.ID {
			f.authors[i] = *author
			return nil
		}
	}
	return errors.New("record not found")
}

func (f *fakeMergeAuthorsRepository) ListAuthorsByExternalIDs(ctx context.Context, source string, ids []string) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) AuthorSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	return false, nil
}

// MergeAuthors mirrors GormRepository.MergeAuthors' three steps over the
// in-memory books[i].Authors slice:
//  1. for each book linked to duplicateID but not canonicalID, swap the
//     duplicate author entry for the canonical one (reassign);
//  2. for each book linked to both, drop the duplicate entry instead
//     (dedupe-on-overlap);
//  3. remove the duplicate author record itself.
func (f *fakeMergeAuthorsRepository) MergeAuthors(ctx context.Context, duplicateID, canonicalID uuid.UUID) (int64, error) {
	var canonicalAuthor *catalogDomain.Author
	for i := range f.authors {
		if f.authors[i].ID == canonicalID {
			canonicalAuthor = &f.authors[i]
			break
		}
	}

	var reassigned int64
	for i := range f.books {
		authors := f.books[i].Authors
		hasDup, hasCanonical := false, false
		for _, a := range authors {
			if a.ID == duplicateID {
				hasDup = true
			}
			if a.ID == canonicalID {
				hasCanonical = true
			}
		}
		if !hasDup {
			continue
		}

		next := make([]catalogDomain.Author, 0, len(authors))
		for _, a := range authors {
			if a.ID == duplicateID {
				if hasCanonical {
					// Step 2: book already linked to both - drop the
					// duplicate row instead of reassigning it.
					continue
				}
				// Step 1: reassign to canonical.
				if canonicalAuthor != nil {
					a = *canonicalAuthor
				} else {
					a.ID = canonicalID
				}
				reassigned++
			}
			next = append(next, a)
		}
		f.books[i].Authors = next
	}

	// Step 3: delete the duplicate author row.
	var dupSource, dupExtID *string
	for i := range f.authors {
		if f.authors[i].ID == duplicateID {
			dupSource, dupExtID = f.authors[i].ExternalSource, f.authors[i].ExternalID
			f.authors = append(f.authors[:i], f.authors[i+1:]...)
			break
		}
	}

	// Step 4: carry the duplicate's external identity to a canonical
	// without one.
	if dupSource != nil && dupExtID != nil {
		for i := range f.authors {
			if f.authors[i].ID == canonicalID && f.authors[i].ExternalSource == nil {
				f.authors[i].ExternalSource, f.authors[i].ExternalID = dupSource, dupExtID
			}
		}
	}

	return reassigned, nil
}

func (f *fakeMergeAuthorsRepository) ListGenres(ctx context.Context, search, status string, limit, offset int) ([]catalogDomain.Genre, int64, error) {
	return nil, 0, nil
}

func (f *fakeMergeAuthorsRepository) GetGenreByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMergeAuthorsRepository) GetGenreBySlug(ctx context.Context, slug string) (*catalogDomain.Genre, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMergeAuthorsRepository) GetGenreByQID(ctx context.Context, qid string) (*catalogDomain.Genre, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMergeAuthorsRepository) ListGenresByQIDs(ctx context.Context, qids []string) ([]catalogDomain.Genre, error) {
	return []catalogDomain.Genre{}, nil
}

func (f *fakeMergeAuthorsRepository) SetGenreQID(ctx context.Context, id uuid.UUID, qid string) error {
	return nil
}

func (f *fakeMergeAuthorsRepository) CreateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	return nil
}

func (f *fakeMergeAuthorsRepository) UpdateGenre(ctx context.Context, genre *catalogDomain.Genre) error {
	return nil
}

func (f *fakeMergeAuthorsRepository) GenreSlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	return false, nil
}

func (f *fakeMergeAuthorsRepository) ListReviews(ctx context.Context, filter repository.ReviewFilter) ([]reviewsDomain.Review, int64, error) {
	return nil, 0, nil
}

func (f *fakeMergeAuthorsRepository) GetReviewByID(ctx context.Context, id uuid.UUID) (*reviewsDomain.Review, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMergeAuthorsRepository) ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMergeAuthorsRepository) AuthorsExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
	return true, nil
}

func (f *fakeMergeAuthorsRepository) GenresExist(ctx context.Context, ids []uuid.UUID) (bool, error) {
	return true, nil
}

func newMergeAuthorsService() (*Service, *fakeMergeAuthorsRepository) {
	repo := &fakeMergeAuthorsRepository{}
	return NewService(repo), repo
}

func TestMergeAuthors(t *testing.T) {
	t.Run("simple reassignment with no overlapping books", func(t *testing.T) {
		svc, repo := newMergeAuthorsService()
		duplicate := catalogDomain.Author{ID: uuid.New(), Name: "Л. Н. Толстой", Slug: "l-n-tolstoy"}
		canonical := catalogDomain.Author{ID: uuid.New(), Name: "Лев Толстой", Slug: "tolstoy"}
		repo.authors = []catalogDomain.Author{duplicate, canonical}
		book1 := catalogDomain.Book{ID: uuid.New(), Title: "Война и мир", Authors: []catalogDomain.Author{duplicate}}
		book2 := catalogDomain.Book{ID: uuid.New(), Title: "Анна Каренина", Authors: []catalogDomain.Author{duplicate}}
		repo.books = []catalogDomain.Book{book1, book2}

		result, booksReassigned, err := svc.MergeAuthors(context.Background(), duplicate.ID, canonical.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if booksReassigned != 2 {
			t.Fatalf("expected 2 books reassigned, got %d", booksReassigned)
		}
		if result.ID != canonical.ID {
			t.Fatalf("expected canonical author returned, got %+v", result)
		}
		for _, b := range repo.books {
			if len(b.Authors) != 1 || b.Authors[0].ID != canonical.ID {
				t.Fatalf("expected book %q to be linked only to canonical, got %+v", b.Title, b.Authors)
			}
		}
	})

	t.Run("overlap dedupe: a book linked to both ends up linked exactly once to canonical", func(t *testing.T) {
		svc, repo := newMergeAuthorsService()
		duplicate := catalogDomain.Author{ID: uuid.New(), Name: "Дубликат", Slug: "dup"}
		canonical := catalogDomain.Author{ID: uuid.New(), Name: "Канонический", Slug: "canon"}
		repo.authors = []catalogDomain.Author{duplicate, canonical}
		overlapping := catalogDomain.Book{ID: uuid.New(), Title: "В соавторстве", Authors: []catalogDomain.Author{duplicate, canonical}}
		soloOnDuplicate := catalogDomain.Book{ID: uuid.New(), Title: "Только дубликат", Authors: []catalogDomain.Author{duplicate}}
		repo.books = []catalogDomain.Book{overlapping, soloOnDuplicate}

		_, booksReassigned, err := svc.MergeAuthors(context.Background(), duplicate.ID, canonical.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Only soloOnDuplicate counts as reassigned; overlapping's
		// duplicate row is dropped, not reassigned.
		if booksReassigned != 1 {
			t.Fatalf("expected 1 book reassigned (the non-overlapping one), got %d", booksReassigned)
		}

		for _, b := range repo.books {
			canonicalCount := 0
			for _, a := range b.Authors {
				if a.ID == duplicate.ID {
					t.Fatalf("book %q still references the duplicate author after merge", b.Title)
				}
				if a.ID == canonical.ID {
					canonicalCount++
				}
			}
			if canonicalCount != 1 {
				t.Fatalf("expected book %q to be linked exactly once to canonical, got %d links", b.Title, canonicalCount)
			}
		}
	})

	t.Run("merging into self is rejected with no changes", func(t *testing.T) {
		svc, repo := newMergeAuthorsService()
		author := catalogDomain.Author{ID: uuid.New(), Name: "Автор", Slug: "author"}
		repo.authors = []catalogDomain.Author{author}

		_, _, err := svc.MergeAuthors(context.Background(), author.ID, author.ID)
		if !errors.Is(err, ErrCannotMergeIntoSelf) {
			t.Fatalf("expected ErrCannotMergeIntoSelf, got %v", err)
		}
		if len(repo.authors) != 1 {
			t.Fatalf("expected no changes, got %+v", repo.authors)
		}
	})

	t.Run("missing duplicate author returns ErrAuthorNotFound, no changes", func(t *testing.T) {
		svc, repo := newMergeAuthorsService()
		canonical := catalogDomain.Author{ID: uuid.New(), Name: "Канонический", Slug: "canon"}
		repo.authors = []catalogDomain.Author{canonical}

		_, _, err := svc.MergeAuthors(context.Background(), uuid.New(), canonical.ID)
		if !errors.Is(err, ErrAuthorNotFound) {
			t.Fatalf("expected ErrAuthorNotFound, got %v", err)
		}
		if len(repo.authors) != 1 {
			t.Fatalf("expected no changes, got %+v", repo.authors)
		}
	})

	t.Run("missing canonical author returns ErrAuthorNotFound, no changes", func(t *testing.T) {
		svc, repo := newMergeAuthorsService()
		duplicate := catalogDomain.Author{ID: uuid.New(), Name: "Дубликат", Slug: "dup"}
		repo.authors = []catalogDomain.Author{duplicate}

		_, _, err := svc.MergeAuthors(context.Background(), duplicate.ID, uuid.New())
		if !errors.Is(err, ErrAuthorNotFound) {
			t.Fatalf("expected ErrAuthorNotFound, got %v", err)
		}
		if len(repo.authors) != 1 {
			t.Fatalf("expected no changes, got %+v", repo.authors)
		}
	})

	t.Run("duplicate author row is gone after merge", func(t *testing.T) {
		svc, repo := newMergeAuthorsService()
		duplicate := catalogDomain.Author{ID: uuid.New(), Name: "Дубликат", Slug: "dup"}
		canonical := catalogDomain.Author{ID: uuid.New(), Name: "Канонический", Slug: "canon"}
		repo.authors = []catalogDomain.Author{duplicate, canonical}

		if _, _, err := svc.MergeAuthors(context.Background(), duplicate.ID, canonical.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repo.authors) != 1 || repo.authors[0].ID != canonical.ID {
			t.Fatalf("expected only the canonical author to remain, got %+v", repo.authors)
		}
	})

	t.Run("canonical author's own fields are unchanged by the merge", func(t *testing.T) {
		svc, repo := newMergeAuthorsService()
		duplicate := catalogDomain.Author{ID: uuid.New(), Name: "Дубликат", Slug: "dup"}
		bio := "Оригинальная биография"
		canonical := catalogDomain.Author{ID: uuid.New(), Name: "Канонический", Slug: "canon", Biography: &bio}
		repo.authors = []catalogDomain.Author{duplicate, canonical}

		result, _, err := svc.MergeAuthors(context.Background(), duplicate.ID, canonical.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "Канонический" || result.Slug != "canon" || result.Biography == nil || *result.Biography != bio {
			t.Fatalf("expected canonical author's fields unchanged, got %+v", result)
		}
	})

	t.Run("QID carries to a canonical author without one", func(t *testing.T) {
		svc, repo := newMergeAuthorsService()
		src, qid := "wikidata", "Q7243"
		duplicate := catalogDomain.Author{ID: uuid.New(), Name: "Дубликат", Slug: "dup", Status: catalogDomain.AuthorStatusPending, ExternalSource: &src, ExternalID: &qid}
		canonical := catalogDomain.Author{ID: uuid.New(), Name: "Канонический", Slug: "canon", Status: catalogDomain.AuthorStatusPublished}
		repo.authors = []catalogDomain.Author{duplicate, canonical}

		result, _, err := svc.MergeAuthors(context.Background(), duplicate.ID, canonical.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ExternalID == nil || *result.ExternalID != "Q7243" || result.ExternalSource == nil || *result.ExternalSource != "wikidata" {
			t.Fatalf("expected QID carried to canonical in result, got %+v", result)
		}
		if repo.authors[0].ExternalID == nil || *repo.authors[0].ExternalID != "Q7243" {
			t.Fatalf("expected QID carried to canonical in repo, got %+v", repo.authors[0])
		}
		if result.Status != catalogDomain.AuthorStatusPublished {
			t.Fatalf("canonical status must be unchanged, got %q", result.Status)
		}
	})

	t.Run("canonical with its own QID keeps it", func(t *testing.T) {
		svc, repo := newMergeAuthorsService()
		src, dupQID, canonQID := "wikidata", "Q1", "Q2"
		duplicate := catalogDomain.Author{ID: uuid.New(), Name: "Дубликат", Slug: "dup", ExternalSource: &src, ExternalID: &dupQID}
		canonical := catalogDomain.Author{ID: uuid.New(), Name: "Канонический", Slug: "canon", ExternalSource: &src, ExternalID: &canonQID}
		repo.authors = []catalogDomain.Author{duplicate, canonical}

		result, _, err := svc.MergeAuthors(context.Background(), duplicate.ID, canonical.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *result.ExternalID != "Q2" || *repo.authors[0].ExternalID != "Q2" {
			t.Fatalf("expected canonical to keep Q2, got result=%v repo=%v", *result.ExternalID, *repo.authors[0].ExternalID)
		}
	})
}

func (f *fakeMergeAuthorsRepository) ListAuthorsForModeration(ctx context.Context, filter repository.AuthorModerationFilter) ([]repository.AuthorModerationRow, int64, error) {
	return nil, 0, nil
}

func (f *fakeMergeAuthorsRepository) ListAuthorSampleBooks(ctx context.Context, authorIDs []uuid.UUID) (map[uuid.UUID][]repository.SampleBook, error) {
	return map[uuid.UUID][]repository.SampleBook{}, nil
}

func (f *fakeMergeAuthorsRepository) FindAuthorsBySurnames(ctx context.Context, surnames []string, excludeIDs []uuid.UUID) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) SetAuthorStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	return false, nil
}

func (f *fakeMergeAuthorsRepository) GetAuthorsByIDs(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) FillAuthorEnrichment(ctx context.Context, id uuid.UUID, fill repository.AuthorEnrichmentFill) error {
	return nil
}

func (f *fakeMergeAuthorsRepository) ListUsers(ctx context.Context, filter repository.UserFilter) ([]usersDomain.User, int64, error) {
	return nil, 0, nil
}

func (f *fakeMergeAuthorsRepository) ChangeUserRole(ctx context.Context, targetID uuid.UUID, decide repository.RoleDecider) (*usersDomain.User, usersDomain.Role, error) {
	return nil, "", nil
}

func (f *fakeMergeAuthorsRepository) ListGenresForModeration(ctx context.Context, filter repository.GenreModerationFilter) ([]repository.GenreModerationRow, int64, error) {
	return nil, 0, nil
}

func (f *fakeMergeAuthorsRepository) ListGenreSampleBooks(ctx context.Context, genreIDs []uuid.UUID) (map[uuid.UUID][]repository.SampleBook, error) {
	return map[uuid.UUID][]repository.SampleBook{}, nil
}

func (f *fakeMergeAuthorsRepository) ListGenreHintCandidates(ctx context.Context) ([]catalogDomain.Genre, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) SetGenreStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	return false, nil
}

func (f *fakeMergeAuthorsRepository) SetBookStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	return false, nil
}

func (f *fakeMergeAuthorsRepository) MergeGenres(ctx context.Context, sourceID, targetID uuid.UUID, actorID *uuid.UUID) (repository.GenreMergeResult, error) {
	return repository.GenreMergeResult{}, nil
}

func (f *fakeMergeAuthorsRepository) MergeGenresBatch(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID, actorID *uuid.UUID) (repository.GenreMergeResult, error) {
	return repository.GenreMergeResult{}, nil
}

func (f *fakeMergeAuthorsRepository) ListGenreMerges(ctx context.Context, page, limit int) ([]repository.GenreMergeEntry, int64, error) {
	return nil, 0, nil
}

func (f *fakeMergeAuthorsRepository) PreviewGenreMergeUndo(ctx context.Context, mergeID uuid.UUID) (repository.GenreMergeUndoPreview, error) {
	return repository.GenreMergeUndoPreview{}, nil
}

func (f *fakeMergeAuthorsRepository) UndoGenreMerge(ctx context.Context, mergeID uuid.UUID, actorID uuid.UUID) (repository.GenreMergeUndoResult, error) {
	return repository.GenreMergeUndoResult{}, nil
}

func (f *fakeMergeAuthorsRepository) PreviewGenreMerge(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID) (repository.GenreMergePreview, error) {
	return repository.GenreMergePreview{}, nil
}

func (f *fakeMergeAuthorsRepository) ListBooksForModeration(ctx context.Context, filter repository.BookModerationFilter) ([]catalogDomain.Book, int64, error) {
	return nil, 0, nil
}

func (f *fakeMergeAuthorsRepository) FindBooksByNormalizedISBNs(ctx context.Context, isbns []string, excludeIDs []uuid.UUID) ([]repository.BookHintCandidate, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) FindBooksByAuthorIDs(ctx context.Context, authorIDs, excludeIDs []uuid.UUID) ([]repository.BookHintCandidate, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) ListGenreAnalysisCandidates(ctx context.Context) ([]repository.GenreModerationRow, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) GenresByIDs(ctx context.Context, ids []uuid.UUID) ([]repository.GenreStateRow, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) ListWikidataBooksWithoutCover(ctx context.Context) ([]catalogDomain.Book, error) {
	return nil, nil
}
