package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func insertAuthorFilm(t *testing.T, db *gorm.DB, authorID, workID uuid.UUID, relation string, hidden bool) {
	t.Helper()
	if err := db.Exec(`INSERT INTO author_screen_works (author_id, screen_work_id, relation, hidden) VALUES (?, ?, ?, ?)`,
		authorID, workID, relation, hidden).Error; err != nil {
		t.Fatalf("insert link: %v", err)
	}
}

type filmLinkRow struct {
	ScreenWorkID string
	Hidden       bool
}

func loadFilmLinks(t *testing.T, db *gorm.DB, authorID uuid.UUID) map[string]bool {
	t.Helper()
	var rows []filmLinkRow
	if err := db.Raw("SELECT screen_work_id, hidden FROM author_screen_works WHERE author_id = ?", authorID).Scan(&rows).Error; err != nil {
		t.Fatalf("load links: %v", err)
	}
	out := map[string]bool{}
	for _, r := range rows {
		out[r.ScreenWorkID] = r.Hidden
	}
	return out
}

func TestMergeAuthors_MovesFilmLinksAndKeepsHides(t *testing.T) {
	repo, db := newMergeQIDRepo(t)
	dup, canon := uuid.New(), uuid.New()
	insertAuthor(t, db, dup, "dup", "pending", nil)
	insertAuthor(t, db, canon, "canon", "published", nil)
	onlyDup, onlyDupHidden, bothDupHidden, bothCanonHidden, bothVisible := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	insertAuthorFilm(t, db, dup, onlyDup, "biopic", false)
	insertAuthorFilm(t, db, dup, onlyDupHidden, "documentary", true)
	insertAuthorFilm(t, db, dup, bothDupHidden, "biopic", true)
	insertAuthorFilm(t, db, canon, bothDupHidden, "biopic", false)
	insertAuthorFilm(t, db, dup, bothCanonHidden, "biopic", false)
	insertAuthorFilm(t, db, canon, bothCanonHidden, "biopic", true)
	insertAuthorFilm(t, db, dup, bothVisible, "biopic", false)
	insertAuthorFilm(t, db, canon, bothVisible, "biopic", false)
	book := uuid.New()
	if err := db.Exec("INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)", book, dup).Error; err != nil {
		t.Fatal(err)
	}

	reassigned, err := repo.MergeAuthors(context.Background(), dup, canon)
	if err != nil {
		t.Fatalf("MergeAuthors: %v", err)
	}
	if reassigned != 1 {
		t.Fatalf("books_reassigned = %d, want 1", reassigned)
	}
	got := loadFilmLinks(t, db, canon)
	want := map[string]bool{
		onlyDup.String(): false, onlyDupHidden.String(): true, bothDupHidden.String(): true,
		bothCanonHidden.String(): true, bothVisible.String(): false,
	}
	if len(got) != len(want) {
		t.Fatalf("links = %v, want %v", got, want)
	}
	for k, v := range want {
		if h, ok := got[k]; !ok || h != v {
			t.Errorf("link %s: hidden=%v present=%v, want hidden=%v", k, h, ok, v)
		}
	}
	if left := loadFilmLinks(t, db, dup); len(left) != 0 {
		t.Fatalf("duplicate keeps links: %v", left)
	}
}
