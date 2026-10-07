package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestListAuthorsByExternalIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("matches QIDs of the given source only", func(t *testing.T) {
		repo, db := newMergeQIDRepo(t)
		a, b, c, other := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		q1, q2, q3 := "Q1", "Q2", "Q3"
		insertAuthor(t, db, a, "a", "published", &q1)
		insertAuthor(t, db, b, "b", "rejected", &q2)
		insertAuthor(t, db, c, "c", "pending", nil)
		insertAuthor(t, db, uuid.New(), "d", "pending", &q3)
		// Same external_id under a different source must be ignored.
		if err := db.Exec("INSERT INTO authors (id, name, slug, status, external_source, external_id) VALUES (?, ?, ?, ?, ?, ?)",
			other, "other", "other", "published", "openlibrary", "Q1").Error; err != nil {
			t.Fatalf("insert: %v", err)
		}

		got, err := repo.ListAuthorsByExternalIDs(ctx, "wikidata", []string{"Q1", "Q2", "Q999"})
		if err != nil {
			t.Fatalf("ListAuthorsByExternalIDs: %v", err)
		}
		ids := map[uuid.UUID]string{}
		for _, au := range got {
			ids[au.ID] = au.Status
		}
		if len(got) != 2 || ids[a] != "published" || ids[b] != "rejected" {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("empty input returns empty without querying", func(t *testing.T) {
		repo, db := newMergeQIDRepo(t)
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("db: %v", err)
		}
		// A closed DB would fail any query, so success proves none ran.
		_ = sqlDB.Close()

		got, err := repo.ListAuthorsByExternalIDs(ctx, "wikidata", nil)
		if err != nil || len(got) != 0 {
			t.Fatalf("got=%v err=%v, want empty and nil", got, err)
		}
	})
}
