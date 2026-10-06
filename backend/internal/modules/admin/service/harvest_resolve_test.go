package service

import (
	"context"
	"errors"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

func TestResolveOrCreateAuthorIDsByNames(t *testing.T) {
	t.Run("reuses existing author by slug, creates missing ones", func(t *testing.T) {
		svc, repo := newService()
		ctx := context.Background()

		existingID := uuid.New()
		repo.authors = append(repo.authors, catalogDomain.Author{ID: existingID, Name: "Leo Tolstoy", Slug: "leo-tolstoy"})

		ids, err := svc.ResolveOrCreateAuthorIDsByNames(ctx, []string{"Leo Tolstoy", "Fyodor Dostoevsky"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 2 {
			t.Fatalf("expected 2 resolved ids, got %d: %v", len(ids), ids)
		}
		if ids[0] != existingID.String() {
			t.Errorf("expected first id to be the existing author %s, got %s", existingID, ids[0])
		}
		if len(repo.authors) != 2 {
			t.Fatalf("expected exactly 1 new author to be created, repo has %d authors", len(repo.authors))
		}
	})

	t.Run("skips blank names and de-duplicates resolved ids", func(t *testing.T) {
		svc, repo := newService()
		ctx := context.Background()

		ids, err := svc.ResolveOrCreateAuthorIDsByNames(ctx, []string{"  ", "Anton Chekhov", "Anton Chekhov"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 1 {
			t.Fatalf("expected 1 resolved id (blank skipped, duplicate collapsed), got %d: %v", len(ids), ids)
		}
		if len(repo.authors) != 1 {
			t.Fatalf("expected exactly 1 author created, got %d", len(repo.authors))
		}
	})

	t.Run("propagates lookup errors other than not-found", func(t *testing.T) {
		svc, repo := newService()
		ctx := context.Background()
		repo.authorLookupErr = errors.New("boom")

		_, err := svc.ResolveOrCreateAuthorIDsByNames(ctx, []string{"Someone"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestResolveOrCreateGenreIDsByNames(t *testing.T) {
	t.Run("reuses existing genre by slug, creates missing ones", func(t *testing.T) {
		svc, repo := newService()
		ctx := context.Background()

		existingID := uuid.New()
		repo.genres = append(repo.genres, catalogDomain.Genre{ID: existingID, Name: "Novel", Slug: "novel"})

		ids, err := svc.ResolveOrCreateGenreIDsByNames(ctx, []string{"Novel", "Fantasy"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 2 {
			t.Fatalf("expected 2 resolved ids, got %d: %v", len(ids), ids)
		}
		if ids[0] != existingID.String() {
			t.Errorf("expected first id to be the existing genre %s, got %s", existingID, ids[0])
		}
		if len(repo.genres) != 2 {
			t.Fatalf("expected exactly 1 new genre to be created, repo has %d genres", len(repo.genres))
		}
	})

	t.Run("normalizes first-letter casing of newly created genres", func(t *testing.T) {
		svc, repo := newService()
		ctx := context.Background()

		_, err := svc.ResolveOrCreateGenreIDsByNames(ctx, []string{"Роман"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repo.genres) != 1 {
			t.Fatalf("expected 1 genre to be created, got %d", len(repo.genres))
		}
		if repo.genres[0].Name != "роман" {
			t.Fatalf("expected created genre name to be lowercased to роман, got %s", repo.genres[0].Name)
		}
	})
}

func TestFindBookByExternalID(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		svc, repo := newService()
		ctx := context.Background()

		source := "wikidata"
		externalID := "Q170583"
		repo.books = append(repo.books, catalogDomain.Book{
			ID:             uuid.New(),
			Title:          "War and Peace",
			ExternalSource: &source,
			ExternalID:     &externalID,
		})

		book, err := svc.FindBookByExternalID(ctx, "wikidata", "Q170583")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if book.Title != "War and Peace" {
			t.Errorf("Title = %q", book.Title)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc, _ := newService()
		ctx := context.Background()

		_, err := svc.FindBookByExternalID(ctx, "wikidata", "Q999999")
		if !errors.Is(err, ErrBookNotFound) {
			t.Fatalf("expected ErrBookNotFound, got %v", err)
		}
	})
}

func strPtr(s string) *string { return &s }

func wikidataAuthor(name, slug, status, qid string) catalogDomain.Author {
	a := catalogDomain.Author{ID: uuid.New(), Name: name, Slug: slug, Status: status}
	if qid != "" {
		a.ExternalSource = strPtr(catalogDomain.AuthorExternalSourceWikidata)
		a.ExternalID = strPtr(qid)
	}
	return a
}

func TestResolveOrCreateAuthorIDsByNames_CreatesPublished(t *testing.T) {
	svc, repo := newService()
	if _, err := svc.ResolveOrCreateAuthorIDsByNames(context.Background(), []string{"Anton Chekhov"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.authors) != 1 || repo.authors[0].Status != catalogDomain.AuthorStatusPublished {
		t.Fatalf("expected one published author, got %+v", repo.authors)
	}
	if repo.authors[0].ExternalSource != nil || repo.authors[0].ExternalID != nil {
		t.Fatalf("expected no external id on import-created author, got %+v", repo.authors[0])
	}
}

func TestResolveOrCreateAuthorIDsByNames_LinksRejectedBySlug(t *testing.T) {
	svc, repo := newService()
	rejected := wikidataAuthor("Anton Chekhov", "anton-chekhov", catalogDomain.AuthorStatusRejected, "")
	repo.authors = append(repo.authors, rejected)

	ids, err := svc.ResolveOrCreateAuthorIDsByNames(context.Background(), []string{"Anton Chekhov"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != rejected.ID.String() {
		t.Fatalf("expected import to link the rejected author as-is, got %v", ids)
	}
}

func TestResolveOrCreateHarvestAuthors(t *testing.T) {
	ctx := context.Background()

	t.Run("new author is pending with QID", func(t *testing.T) {
		svc, repo := newService()
		ids, err := svc.ResolveOrCreateHarvestAuthors(ctx, []AuthorRef{{Name: "Leo Tolstoy", QID: "Q7243"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 1 || len(repo.authors) != 1 {
			t.Fatalf("expected 1 created author, ids=%v authors=%d", ids, len(repo.authors))
		}
		a := repo.authors[0]
		if a.Status != catalogDomain.AuthorStatusPending || a.Slug != "leo-tolstoy" {
			t.Fatalf("unexpected author %+v", a)
		}
		if a.ExternalSource == nil || *a.ExternalSource != "wikidata" || a.ExternalID == nil || *a.ExternalID != "Q7243" {
			t.Fatalf("expected wikidata/Q7243 identity, got %+v", a)
		}
	})

	t.Run("new author without QID has no identity", func(t *testing.T) {
		svc, repo := newService()
		if _, err := svc.ResolveOrCreateHarvestAuthors(ctx, []AuthorRef{{Name: "Leo Tolstoy"}}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.authors[0].ExternalSource != nil || repo.authors[0].Status != catalogDomain.AuthorStatusPending {
			t.Fatalf("unexpected author %+v", repo.authors[0])
		}
	})

	t.Run("QID match beats slug match", func(t *testing.T) {
		svc, repo := newService()
		renamed := wikidataAuthor("Lev Tolstoi", "lev-tolstoi", catalogDomain.AuthorStatusPublished, "Q7243")
		slugTwin := wikidataAuthor("Leo Tolstoy", "leo-tolstoy", catalogDomain.AuthorStatusPublished, "")
		repo.authors = append(repo.authors, renamed, slugTwin)

		ids, err := svc.ResolveOrCreateHarvestAuthors(ctx, []AuthorRef{{Name: "Leo Tolstoy", QID: "Q7243"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 1 || ids[0] != renamed.ID.String() {
			t.Fatalf("expected QID-matched author %s, got %v", renamed.ID, ids)
		}
		if repo.authors[1].ExternalID != nil {
			t.Fatalf("slug twin must not be touched, got %+v", repo.authors[1])
		}
	})

	t.Run("QID-matched rejected author is skipped, nothing created", func(t *testing.T) {
		svc, repo := newService()
		repo.authors = append(repo.authors, wikidataAuthor("Junk", "junk", catalogDomain.AuthorStatusRejected, "Q1"))

		ids, err := svc.ResolveOrCreateHarvestAuthors(ctx, []AuthorRef{{Name: "Some Label", QID: "Q1"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 0 || len(repo.authors) != 1 {
			t.Fatalf("expected skip with nothing created, ids=%v authors=%d", ids, len(repo.authors))
		}
	})

	t.Run("slug-matched rejected author is skipped and QID backfilled", func(t *testing.T) {
		svc, repo := newService()
		repo.authors = append(repo.authors, wikidataAuthor("Junk Name", "junk-name", catalogDomain.AuthorStatusRejected, ""))

		ids, err := svc.ResolveOrCreateHarvestAuthors(ctx, []AuthorRef{{Name: "Junk Name", QID: "Q9"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 0 || len(repo.authors) != 1 {
			t.Fatalf("expected skip with nothing created, ids=%v authors=%d", ids, len(repo.authors))
		}
		a := repo.authors[0]
		if a.ExternalID == nil || *a.ExternalID != "Q9" || a.Status != catalogDomain.AuthorStatusRejected {
			t.Fatalf("expected QID backfilled and status unchanged, got %+v", a)
		}
	})

	t.Run("slug match without QID gets QID backfilled, status unchanged", func(t *testing.T) {
		svc, repo := newService()
		existing := wikidataAuthor("Leo Tolstoy", "leo-tolstoy", catalogDomain.AuthorStatusPublished, "")
		repo.authors = append(repo.authors, existing)

		ids, err := svc.ResolveOrCreateHarvestAuthors(ctx, []AuthorRef{{Name: "Leo Tolstoy", QID: "Q7243"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 1 || ids[0] != existing.ID.String() || len(repo.authors) != 1 {
			t.Fatalf("expected existing author linked, ids=%v authors=%d", ids, len(repo.authors))
		}
		a := repo.authors[0]
		if a.ExternalSource == nil || *a.ExternalSource != "wikidata" || a.ExternalID == nil || *a.ExternalID != "Q7243" {
			t.Fatalf("expected backfilled QID, got %+v", a)
		}
		if a.Status != catalogDomain.AuthorStatusPublished {
			t.Fatalf("status must not change on backfill, got %q", a.Status)
		}
	})

	t.Run("slug match with different QID creates namesake with suffixed slug", func(t *testing.T) {
		svc, repo := newService()
		other := wikidataAuthor("Ivan Petrov", "ivan-petrov", catalogDomain.AuthorStatusPublished, "Q100")
		repo.authors = append(repo.authors, other)

		ids, err := svc.ResolveOrCreateHarvestAuthors(ctx, []AuthorRef{{Name: "Ivan Petrov", QID: "Q12345"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 1 || ids[0] == other.ID.String() || len(repo.authors) != 2 {
			t.Fatalf("expected a new namesake author, ids=%v authors=%d", ids, len(repo.authors))
		}
		created := repo.authors[1]
		if created.Slug != "ivan-petrov-q12345" || created.Status != catalogDomain.AuthorStatusPending || *created.ExternalID != "Q12345" {
			t.Fatalf("unexpected namesake %+v", created)
		}
		if *repo.authors[0].ExternalID != "Q100" {
			t.Fatalf("existing namesake must keep its QID, got %+v", repo.authors[0])
		}
	})

	t.Run("no QID input links slug match regardless of its QID", func(t *testing.T) {
		svc, repo := newService()
		existing := wikidataAuthor("Leo Tolstoy", "leo-tolstoy", catalogDomain.AuthorStatusPending, "Q7243")
		repo.authors = append(repo.authors, existing)

		ids, err := svc.ResolveOrCreateHarvestAuthors(ctx, []AuthorRef{{Name: "Leo Tolstoy"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 1 || ids[0] != existing.ID.String() || len(repo.authors) != 1 {
			t.Fatalf("expected existing author linked, ids=%v authors=%d", ids, len(repo.authors))
		}
	})

	t.Run("dedupes and skips blank names; re-run creates no duplicates", func(t *testing.T) {
		svc, repo := newService()
		refs := []AuthorRef{{Name: " "}, {Name: "Leo Tolstoy", QID: "Q7243"}, {Name: "Leo Tolstoy", QID: "Q7243"}}
		ids, err := svc.ResolveOrCreateHarvestAuthors(ctx, refs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		again, err := svc.ResolveOrCreateHarvestAuthors(ctx, refs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 1 || len(again) != 1 || ids[0] != again[0] || len(repo.authors) != 1 {
			t.Fatalf("expected one author across runs, ids=%v again=%v authors=%d", ids, again, len(repo.authors))
		}
	})

	t.Run("propagates lookup errors", func(t *testing.T) {
		svc, repo := newService()
		repo.authorLookupErr = errors.New("boom")
		if _, err := svc.ResolveOrCreateHarvestAuthors(ctx, []AuthorRef{{Name: "Someone"}}); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
