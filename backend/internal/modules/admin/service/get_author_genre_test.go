package service

import (
	"context"
	"errors"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// getByIDErrRepository makes GetAuthorByID / GetGenreByID fail with err.
type getByIDErrRepository struct {
	*fakeRepository
	err error
}

func (r *getByIDErrRepository) GetAuthorByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error) {
	return nil, r.err
}

func (r *getByIDErrRepository) GetGenreByID(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error) {
	return nil, r.err
}

func TestGetAuthor(t *testing.T) {
	ctx := context.Background()

	t.Run("found in any status", func(t *testing.T) {
		svc, repo := newService()
		for _, status := range []string{catalogDomain.AuthorStatusPublished, catalogDomain.AuthorStatusPending, catalogDomain.AuthorStatusRejected} {
			id := uuid.New()
			repo.authors = append(repo.authors, catalogDomain.Author{ID: id, Name: "A", Slug: "a-" + status, Status: status})
			got, err := svc.GetAuthor(ctx, id)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", status, err)
			}
			if got.ID != id || got.Status != status {
				t.Fatalf("%s: unexpected author %+v", status, got)
			}
		}
	})

	t.Run("not found maps to ErrAuthorNotFound", func(t *testing.T) {
		svc, _ := newService()
		if _, err := svc.GetAuthor(ctx, uuid.New()); !errors.Is(err, ErrAuthorNotFound) {
			t.Fatalf("expected ErrAuthorNotFound, got %v", err)
		}
	})

	t.Run("other error is wrapped", func(t *testing.T) {
		repoErr := errors.New("db down")
		svc := NewService(&getByIDErrRepository{fakeRepository: &fakeRepository{}, err: repoErr})
		_, err := svc.GetAuthor(ctx, uuid.New())
		if !errors.Is(err, repoErr) || errors.Is(err, ErrAuthorNotFound) {
			t.Fatalf("expected wrapped repo error, got %v", err)
		}
	})
}

func TestGetGenre(t *testing.T) {
	ctx := context.Background()

	t.Run("found in any status", func(t *testing.T) {
		svc, repo := newService()
		for _, status := range []string{catalogDomain.GenreStatusPublished, catalogDomain.GenreStatusPending, catalogDomain.GenreStatusRejected} {
			id := uuid.New()
			repo.genres = append(repo.genres, catalogDomain.Genre{ID: id, Name: "g", Slug: "g-" + status, Status: status})
			got, err := svc.GetGenre(ctx, id)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", status, err)
			}
			if got.ID != id || got.Status != status {
				t.Fatalf("%s: unexpected genre %+v", status, got)
			}
		}
	})

	t.Run("not found maps to ErrGenreNotFound", func(t *testing.T) {
		svc, _ := newService()
		if _, err := svc.GetGenre(ctx, uuid.New()); !errors.Is(err, ErrGenreNotFound) {
			t.Fatalf("expected ErrGenreNotFound, got %v", err)
		}
	})

	t.Run("alias id maps to ErrGenreNotFound", func(t *testing.T) {
		svc, repo := newService()
		target := uuid.New()
		alias := uuid.New()
		repo.genres = []catalogDomain.Genre{
			{ID: target, Name: "t", Slug: "t", Status: catalogDomain.GenreStatusPublished},
			{ID: alias, Name: "a", Slug: "a", Status: catalogDomain.GenreStatusPublished, MergedIntoID: &target},
		}
		if _, err := svc.GetGenre(ctx, alias); !errors.Is(err, ErrGenreNotFound) {
			t.Fatalf("expected ErrGenreNotFound, got %v", err)
		}
	})

	t.Run("other error is wrapped", func(t *testing.T) {
		repoErr := errors.New("db down")
		svc := NewService(&getByIDErrRepository{fakeRepository: &fakeRepository{}, err: repoErr})
		_, err := svc.GetGenre(ctx, uuid.New())
		if !errors.Is(err, repoErr) || errors.Is(err, ErrGenreNotFound) {
			t.Fatalf("expected wrapped repo error, got %v", err)
		}
	})
}
