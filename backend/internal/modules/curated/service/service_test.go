package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/curated/domain"
	"github.com/bookhaven/backend/internal/modules/curated/repository"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const ddl = `
CREATE TABLE books (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', slug TEXT NOT NULL DEFAULT '', description TEXT, cover_url TEXT, isbn TEXT, published_year INTEGER, page_count INTEGER, language TEXT DEFAULT 'ru', rating REAL DEFAULT 0, ratings_count INTEGER DEFAULT 0, reviews_count INTEGER DEFAULT 0, status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT, created_at DATETIME, updated_at DATETIME, published_at DATETIME);
CREATE TABLE authors (id TEXT PRIMARY KEY, name TEXT, slug TEXT, status TEXT);
CREATE TABLE genres (id TEXT PRIMARY KEY, name TEXT, slug TEXT, status TEXT, merged_into_id TEXT);
CREATE TABLE book_authors (book_id TEXT NOT NULL, author_id TEXT NOT NULL);
CREATE TABLE book_genres (book_id TEXT NOT NULL, genre_id TEXT NOT NULL);
CREATE TABLE curated_collections (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL DEFAULT 'manual', size INTEGER NOT NULL, source TEXT, source_url TEXT, genre_id TEXT, is_published BOOLEAN NOT NULL DEFAULT 0, published_at DATETIME, created_at DATETIME, updated_at DATETIME);
CREATE TABLE curated_collection_items (collection_id TEXT NOT NULL, book_id TEXT NOT NULL, position INTEGER NOT NULL, added_at DATETIME, PRIMARY KEY (collection_id, book_id));
`

func setup(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatal(err)
	}
	return NewService(repository.NewGormRepository(db)), db
}

func book(t *testing.T, db *gorm.DB, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	if err := db.Exec("INSERT INTO books (id, title, slug, status, created_at, updated_at) VALUES (?, 'К', ?, ?, ?, ?)", id, id.String(), status, now, now).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func genre(t *testing.T, db *gorm.DB, status string, mergedInto *uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Exec("INSERT INTO genres (id, name, slug, status, merged_into_id) VALUES (?, 'g', ?, ?, ?)", id, id.String(), status, mergedInto).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func isValidation(err error) bool {
	var v *ValidationError
	return errors.As(err, &v)
}

func TestCreate_Defaults(t *testing.T) {
	s, _ := setup(t)
	c, err := s.Create(context.Background(), CreateInput{Title: "Топ 100 фантастики", Size: 100})
	if err != nil {
		t.Fatal(err)
	}
	if c.Slug != "top-100-fantastiki" || c.Kind != domain.KindManual || c.IsPublished || c.PublishedAt != nil {
		t.Fatalf("got %+v", c)
	}
}

func TestCreate_Validation(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	rejected := genre(t, db, catalogDomain.GenreStatusRejected, nil)
	target := genre(t, db, catalogDomain.GenreStatusPublished, nil)
	alias := genre(t, db, catalogDomain.GenreStatusRejected, &target)
	missing := uuid.New()
	long := strings.Repeat("я", 256)
	cases := []struct {
		name string
		in   CreateInput
	}{
		{"short title", CreateInput{Title: "a", Size: 5}},
		{"long title", CreateInput{Title: long, Size: 5}},
		{"size 0", CreateInput{Title: "Ok title", Size: 0}},
		{"size 1001", CreateInput{Title: "Ok title", Size: 1001}},
		{"bad slug", CreateInput{Title: "Ok title", Size: 5, Slug: "Bad Slug"}},
		{"empty derived slug", CreateInput{Title: "!!", Size: 5}},
		{"long description", CreateInput{Title: "Ok title", Size: 5, Description: strings.Repeat("a", 5001)}},
		{"long source", CreateInput{Title: "Ok title", Size: 5, Source: strings.Repeat("a", 256)}},
		{"bad url", CreateInput{Title: "Ok title", Size: 5, SourceURL: "ftp://x.org"}},
		{"url no host", CreateInput{Title: "Ok title", Size: 5, SourceURL: "http://"}},
	}
	for _, tc := range cases {
		if _, err := s.Create(ctx, tc.in); !isValidation(err) {
			t.Errorf("%s: want validation error, got %v", tc.name, err)
		}
	}
	for name, g := range map[string]uuid.UUID{"missing": missing, "rejected": rejected, "merged": alias} {
		g := g
		if _, err := s.Create(ctx, CreateInput{Title: "Ok title", Size: 5, GenreID: &g}); !errors.Is(err, domain.ErrInvalidGenre) {
			t.Errorf("genre %s: %v", name, err)
		}
	}
	ok := genre(t, db, catalogDomain.GenreStatusPending, nil)
	if _, err := s.Create(ctx, CreateInput{Title: "Ok title", Size: 5, GenreID: &ok, SourceURL: "https://x.org/a"}); err != nil {
		t.Fatalf("valid: %v", err)
	}
}

func TestSlugConflict(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	if _, err := s.Create(ctx, CreateInput{Title: "One", Size: 1, Slug: "same"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, CreateInput{Title: "Two", Size: 1, Slug: "same"}); !errors.Is(err, domain.ErrSlugTaken) {
		t.Fatalf("got %v", err)
	}
	b, _ := s.Create(ctx, CreateInput{Title: "Three", Size: 1})
	same := "same"
	if _, err := s.Update(ctx, b.ID, UpdateInput{Slug: &same}); !errors.Is(err, domain.ErrSlugTaken) {
		t.Fatalf("update got %v", err)
	}
}

func TestPublishedAtKept(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	c, _ := s.Create(ctx, CreateInput{Title: "Ok title", Size: 3})
	yes, no := true, false
	c, err := s.Update(ctx, c.ID, UpdateInput{IsPublished: &yes})
	if err != nil || c.PublishedAt == nil {
		t.Fatalf("publish: %v %+v", err, c)
	}
	first := *c.PublishedAt
	c, _ = s.Update(ctx, c.ID, UpdateInput{IsPublished: &no})
	if c.IsPublished || c.PublishedAt == nil || !c.PublishedAt.Equal(first) {
		t.Fatal("unpublish must keep published_at")
	}
	s.now = func() time.Time { return first.Add(time.Hour) }
	c, _ = s.Update(ctx, c.ID, UpdateInput{IsPublished: &yes})
	if !c.PublishedAt.Equal(first) {
		t.Fatal("republish must keep the first time")
	}
}

func TestUpdate_ClearsGenreAndSource(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	g := genre(t, db, catalogDomain.GenreStatusPublished, nil)
	c, err := s.Create(ctx, CreateInput{Title: "Ok title", Size: 3, GenreID: &g, Source: "BBC", SourceURL: "https://bbc.co.uk"})
	if err != nil || c.GenreID == nil || c.Source == nil {
		t.Fatalf("create: %v %+v", err, c)
	}
	empty := ""
	c, err = s.Update(ctx, c.ID, UpdateInput{GenreIDSet: true, Source: &empty, SourceURL: &empty})
	if err != nil || c.GenreID != nil || c.Source != nil || c.SourceURL != nil {
		t.Fatalf("clear: %v %+v", err, c)
	}
	if _, err := s.Update(ctx, uuid.New(), UpdateInput{}); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Fatalf("not found: %v", err)
	}
}

func TestAddBookRules(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	c, _ := s.Create(ctx, CreateInput{Title: "Ok title", Size: 2})
	pub := book(t, db, catalogDomain.BookStatusPublished)
	pend := book(t, db, catalogDomain.BookStatusPending)
	rej := book(t, db, catalogDomain.BookStatusRejected)

	if err := s.AddBook(ctx, c.ID, pub, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.AddBook(ctx, c.ID, pend, nil); err != nil {
		t.Fatalf("pending must be accepted: %v", err)
	}
	if err := s.AddBook(ctx, c.ID, rej, nil); !errors.Is(err, domain.ErrBookRejected) {
		t.Fatalf("rejected: %v", err)
	}
	if err := s.AddBook(ctx, c.ID, uuid.New(), nil); !errors.Is(err, domain.ErrBookNotFound) {
		t.Fatalf("missing book: %v", err)
	}
	if err := s.AddBook(ctx, c.ID, pub, nil); !errors.Is(err, domain.ErrBookAlreadyIn) {
		t.Fatalf("dup: %v", err)
	}
	if err := s.AddBook(ctx, uuid.New(), pub, nil); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Fatalf("missing collection: %v", err)
	}
	if err := s.MoveBook(ctx, c.ID, pub, 0); !errors.Is(err, domain.ErrInvalidPosition) {
		t.Fatalf("position: %v", err)
	}
	if err := s.MoveBook(ctx, c.ID, rej, 1); !errors.Is(err, domain.ErrBookNotInList) {
		t.Fatalf("move missing: %v", err)
	}
	if err := s.RemoveBook(ctx, c.ID, rej); !errors.Is(err, domain.ErrBookNotInList) {
		t.Fatalf("remove missing: %v", err)
	}
}

func TestAddBook_Full(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	c, _ := s.Create(ctx, CreateInput{Title: "Ok title", Size: 5})
	now := time.Now()
	for i := 0; i < domain.MaxBooks; i++ {
		id := book(t, db, catalogDomain.BookStatusPending)
		if err := db.Exec("INSERT INTO curated_collection_items (collection_id, book_id, position, added_at) VALUES (?, ?, ?, ?)", c.ID, id, i+1, now).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AddBook(ctx, c.ID, book(t, db, catalogDomain.BookStatusPublished), nil); !errors.Is(err, domain.ErrCollectionFull) {
		t.Fatalf("full: %v", err)
	}
}

func TestGetAdmin_HiddenInMiddle(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	c, _ := s.Create(ctx, CreateInput{Title: "Ok title", Size: 2})
	a := book(t, db, catalogDomain.BookStatusPublished)
	b := book(t, db, catalogDomain.BookStatusPending)
	cc := book(t, db, catalogDomain.BookStatusPublished)
	d := book(t, db, catalogDomain.BookStatusPublished)
	for _, id := range []uuid.UUID{a, b, cc, d} {
		if err := s.AddBook(ctx, c.ID, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	det, err := s.GetAdmin(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		state string
		place int
	}{{domain.StateRanked, 1}, {domain.StateHidden, 0}, {domain.StateRanked, 2}, {domain.StateReserve, 0}}
	for i, w := range want {
		if det.Items[i].State != w.state || det.Items[i].Place != w.place || det.Items[i].Position != i+1 {
			t.Fatalf("item %d: %+v", i, det.Items[i])
		}
	}
	if det.ShownCount != 2 || det.BooksCount != 4 {
		t.Fatalf("counts %d %d", det.ShownCount, det.BooksCount)
	}
}

func TestGetPublic_PlacesAndShrink(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	yes := true
	c, _ := s.Create(ctx, CreateInput{Title: "Ok title", Size: 3, Slug: "top", IsPublished: yes})
	ids := make([]uuid.UUID, 4)
	for i := range ids {
		ids[i] = book(t, db, catalogDomain.BookStatusPublished)
		_ = s.AddBook(ctx, c.ID, ids[i], nil)
	}
	// unpublish B -> A C D
	db.Exec("UPDATE books SET status = 'pending' WHERE id = ?", ids[1])
	res, err := s.GetPublic(ctx, "top", 1, 50)
	if err != nil || res.Total != 3 || len(res.Items) != 3 {
		t.Fatalf("%v %+v", err, res)
	}
	if res.Items[1].Book.ID != ids[2] || res.Items[1].Place != 2 || res.Items[2].Book.ID != ids[3] {
		t.Fatal("shifted order")
	}
	// republish -> A B C
	db.Exec("UPDATE books SET status = 'published' WHERE id = ?", ids[1])
	res, _ = s.GetPublic(ctx, "top", 1, 50)
	if len(res.Items) != 3 || res.Items[1].Book.ID != ids[1] {
		t.Fatal("republished returns")
	}
	// page 2 with limit 2 keeps global numbering and caps at size
	res, _ = s.GetPublic(ctx, "top", 2, 2)
	if len(res.Items) != 1 || res.Items[0].Place != 3 {
		t.Fatalf("page 2: %+v", res.Items)
	}
	// limit clamp
	res, _ = s.GetPublic(ctx, "top", 1, 1000)
	if res.Limit != MaxLimit {
		t.Fatal("limit clamp")
	}
	// shrink size
	two := 2
	_, _ = s.Update(ctx, c.ID, UpdateInput{Size: &two})
	res, _ = s.GetPublic(ctx, "top", 1, 50)
	if len(res.Items) != 2 || res.Total != 2 {
		t.Fatal("shrink")
	}
	// draft and unknown are 404
	no := false
	_, _ = s.Update(ctx, c.ID, UpdateInput{IsPublished: &no})
	if _, err := s.GetPublic(ctx, "top", 1, 50); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Fatalf("draft: %v", err)
	}
	if _, err := s.GetPublic(ctx, "nope", 1, 50); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestDelete(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	c, _ := s.Create(ctx, CreateInput{Title: "Ok title", Size: 1})
	if err := s.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, c.ID); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Fatalf("got %v", err)
	}
}
