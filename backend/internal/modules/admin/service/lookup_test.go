package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/pkg/bookdata"
)

// fakeLookupClient implements bookdata.Client for LookupBook tests, with
// canned responses keyed by ISBN or "title|author".
type fakeLookupClient struct {
	byISBN        map[string]*bookdata.ExternalBookRecord
	byTitleAuthor map[string]*bookdata.ExternalBookRecord
	err           error
}

func (f *fakeLookupClient) SourceName() string { return "openlibrary" }

func (f *fakeLookupClient) FindByISBN(ctx context.Context, isbn string) (*bookdata.ExternalBookRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	if rec, ok := f.byISBN[isbn]; ok {
		return rec, nil
	}
	return nil, bookdata.ErrNotFound
}

func (f *fakeLookupClient) FindByTitleAuthor(ctx context.Context, title, author string) (*bookdata.ExternalBookRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	if rec, ok := f.byTitleAuthor[title+"|"+author]; ok {
		return rec, nil
	}
	return nil, bookdata.ErrNotFound
}

func TestLookupBook(t *testing.T) {
	t.Run("isbn path returns the matching record", func(t *testing.T) {
		client := &fakeLookupClient{byISBN: map[string]*bookdata.ExternalBookRecord{
			"9780000000000": {Title: "War and Peace", SourceName: "openlibrary", SourceID: "OL1"},
		}}
		svc := NewService(&fakeRepository{}, WithLookupClient(client))

		rec, err := svc.LookupBook(context.Background(), "9780000000000", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Title != "War and Peace" {
			t.Errorf("Title = %q, want %q", rec.Title, "War and Peace")
		}
	})

	t.Run("title+author path returns the matching record", func(t *testing.T) {
		client := &fakeLookupClient{byTitleAuthor: map[string]*bookdata.ExternalBookRecord{
			"Anna Karenina|Leo Tolstoy": {Title: "Anna Karenina"},
		}}
		svc := NewService(&fakeRepository{}, WithLookupClient(client))

		rec, err := svc.LookupBook(context.Background(), "", "Anna Karenina", "Leo Tolstoy")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Title != "Anna Karenina" {
			t.Errorf("Title = %q, want %q", rec.Title, "Anna Karenina")
		}
	})

	t.Run("author is optional for the title path", func(t *testing.T) {
		client := &fakeLookupClient{byTitleAuthor: map[string]*bookdata.ExternalBookRecord{
			"Anna Karenina|": {Title: "Anna Karenina"},
		}}
		svc := NewService(&fakeRepository{}, WithLookupClient(client))

		rec, err := svc.LookupBook(context.Background(), "", "Anna Karenina", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Title != "Anna Karenina" {
			t.Errorf("Title = %q, want %q", rec.Title, "Anna Karenina")
		}
	})

	t.Run("not-found passthrough", func(t *testing.T) {
		client := &fakeLookupClient{}
		svc := NewService(&fakeRepository{}, WithLookupClient(client))

		_, err := svc.LookupBook(context.Background(), "0000000000", "", "")
		if !errors.Is(err, bookdata.ErrNotFound) {
			t.Fatalf("expected bookdata.ErrNotFound, got %v", err)
		}
	})

	t.Run("rate-limited/unavailable passthrough", func(t *testing.T) {
		client := &fakeLookupClient{err: bookdata.ErrRateLimited}
		svc := NewService(&fakeRepository{}, WithLookupClient(client))

		_, err := svc.LookupBook(context.Background(), "0000000000", "", "")
		if !errors.Is(err, bookdata.ErrRateLimited) {
			t.Fatalf("expected bookdata.ErrRateLimited, got %v", err)
		}
	})

	t.Run("no client configured", func(t *testing.T) {
		svc, _ := newService()

		_, err := svc.LookupBook(context.Background(), "0000000000", "", "")
		if !errors.Is(err, ErrLookupUnavailable) {
			t.Fatalf("expected ErrLookupUnavailable, got %v", err)
		}
	})

	t.Run("empty params is a validation error", func(t *testing.T) {
		svc := NewService(&fakeRepository{}, WithLookupClient(&fakeLookupClient{}))

		_, err := svc.LookupBook(context.Background(), "", "  ", "")
		if !errors.Is(err, ErrLookupQueryRequired) {
			t.Fatalf("expected ErrLookupQueryRequired, got %v", err)
		}
	})
}
