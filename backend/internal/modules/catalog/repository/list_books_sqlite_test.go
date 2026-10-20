package repository

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListBooks ordering and filter duplicate-safety (docs/features/catalog-pagination.md).
// Full-text search is Postgres-only and is not exercised here.

func newListBooksDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(catalogStatusDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func lbExec(t *testing.T, db *gorm.DB, q string, args ...interface{}) {
	t.Helper()
	if err := db.Exec(q, args...).Error; err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func lbGenre(t *testing.T, db *gorm.DB, slug string) uuid.UUID {
	id := uuid.New()
	lbExec(t, db, "INSERT INTO genres (id, name, slug, status, created_at) VALUES (?, ?, ?, ?, ?)",
		id, slug, slug, domain.GenreStatusPublished, time.Now())
	return id
}

func lbAuthor(t *testing.T, db *gorm.DB, slug string) uuid.UUID {
	id := uuid.New()
	now := time.Now()
	lbExec(t, db, "INSERT INTO authors (id, name, slug, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, slug, slug, domain.AuthorStatusPublished, now, now)
	return id
}

func lbBook(t *testing.T, db *gorm.DB, title string, year interface{}, created time.Time) uuid.UUID {
	id := uuid.New()
	lbExec(t, db, "INSERT INTO books (id, title, slug, status, rating, published_year, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, title, id.String(), domain.BookStatusPublished, 4.0, year, created, created)
	return id
}

func TestListBooks_MultiGenreNoDuplicates(t *testing.T) {
	db := newListBooksDB(t)
	a, b := lbGenre(t, db, "a"), lbGenre(t, db, "b")
	book := lbBook(t, db, "Книга", 2000, time.Now())
	lbExec(t, db, "INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?)", book, a)
	lbExec(t, db, "INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?)", book, b)

	repo := &GormRepository{db: db}
	got, total, err := repo.ListBooks(context.Background(), BookFilter{GenreID: []uuid.UUID{a, b}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("total=%d len=%d, want 1/1", total, len(got))
	}
}

func TestListBooks_AuthorAndGenreNoDuplicates(t *testing.T) {
	db := newListBooksDB(t)
	a, b := lbGenre(t, db, "a"), lbGenre(t, db, "b")
	x := lbAuthor(t, db, "x")
	book := lbBook(t, db, "Книга", 2000, time.Now())
	lbExec(t, db, "INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?)", book, a)
	lbExec(t, db, "INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?)", book, b)
	lbExec(t, db, "INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)", book, x)

	repo := &GormRepository{db: db}
	got, total, err := repo.ListBooks(context.Background(), BookFilter{AuthorID: &x, GenreID: []uuid.UUID{a, b}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("total=%d len=%d, want 1/1", total, len(got))
	}
}

func TestListBooks_DeterministicPaging(t *testing.T) {
	for _, tc := range []struct {
		sort     string
		nullYear bool
	}{{"rating", false}, {"new", false}, {"new", true}, {"title", false}} {
		name := tc.sort
		if tc.nullYear {
			name += "_null_year"
		}
		t.Run(name, func(t *testing.T) {
			db := newListBooksDB(t)
			created := time.Now()
			var ids []uuid.UUID
			var nullID uuid.UUID
			for i := 0; i < 7; i++ {
				var year interface{} = 2000
				if tc.nullYear && i == 0 {
					year = nil
				}
				id := lbBook(t, db, "Книга", year, created)
				if tc.nullYear && i == 0 {
					nullID = id
				}
				ids = append(ids, id)
			}
			repo := &GormRepository{db: db}
			var order []uuid.UUID
			for off := 0; off < 7; off += 3 {
				got, total, err := repo.ListBooks(context.Background(), BookFilter{Sort: tc.sort, Limit: 3, Offset: off})
				if err != nil {
					t.Fatal(err)
				}
				if total != 7 {
					t.Fatalf("total=%d, want 7", total)
				}
				for _, b := range got {
					order = append(order, b.ID)
				}
			}
			if len(order) != 7 {
				t.Fatalf("got %d books, want 7", len(order))
			}
			seen := map[uuid.UUID]bool{}
			for _, id := range order {
				if seen[id] {
					t.Fatalf("duplicate id %s", id)
				}
				seen[id] = true
			}
			for _, id := range ids {
				if !seen[id] {
					t.Fatalf("missing id %s", id)
				}
			}
			if tc.nullYear {
				if order[len(order)-1] != nullID {
					t.Fatalf("NULL published_year book must be last")
				}
				order = order[:len(order)-1]
			}
			if !sort.SliceIsSorted(order, func(i, j int) bool { return order[i].String() < order[j].String() }) {
				t.Fatalf("ties not ordered by id ASC: %v", order)
			}
		})
	}
}

func TestListBooks_HasAdaptation(t *testing.T) {
	db := newListBooksDB(t)
	lbExec(t, db, "CREATE TABLE book_screen_works (book_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, hidden BOOLEAN NOT NULL DEFAULT 0, PRIMARY KEY (book_id, screen_work_id))")
	g := lbGenre(t, db, "g")
	now := time.Now()
	visible := lbBook(t, db, "С экранизацией", 2000, now)
	onlyHidden := lbBook(t, db, "Скрытая", 2000, now)
	none := lbBook(t, db, "Без", 2000, now)
	mixed := lbBook(t, db, "Две", 2000, now)
	for _, b := range []uuid.UUID{visible, onlyHidden, mixed} {
		lbExec(t, db, "INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?)", b, g)
	}
	lbExec(t, db, "INSERT INTO book_screen_works (book_id, screen_work_id, hidden) VALUES (?, ?, 0)", visible, uuid.NewString())
	lbExec(t, db, "INSERT INTO book_screen_works (book_id, screen_work_id, hidden) VALUES (?, ?, 1)", onlyHidden, uuid.NewString())
	lbExec(t, db, "INSERT INTO book_screen_works (book_id, screen_work_id, hidden) VALUES (?, ?, 0)", mixed, uuid.NewString())
	lbExec(t, db, "INSERT INTO book_screen_works (book_id, screen_work_id, hidden) VALUES (?, ?, 0)", mixed, uuid.NewString())
	_ = none

	repo := &GormRepository{db: db}
	got, total, err := repo.ListBooks(context.Background(), BookFilter{HasAdaptation: true})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(got) != 2 {
		t.Fatalf("total=%d len=%d, want 2/2 (visible and mixed, once each)", total, len(got))
	}
	for _, b := range got {
		if b.ID == onlyHidden || b.ID == none {
			t.Fatalf("unexpected book %s", b.Title)
		}
	}
	_, total, err = repo.ListBooks(context.Background(), BookFilter{HasAdaptation: true, GenreID: []uuid.UUID{g}})
	if err != nil || total != 2 {
		t.Fatalf("with genre: total=%d err=%v", total, err)
	}
	_, total, _ = repo.ListBooks(context.Background(), BookFilter{})
	if total != 4 {
		t.Fatalf("unfiltered total=%d, want 4", total)
	}
}
