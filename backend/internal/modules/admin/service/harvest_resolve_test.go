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
