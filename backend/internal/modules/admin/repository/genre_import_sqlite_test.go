package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestListGenresByQIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("returns matching genres including alias rows", func(t *testing.T) {
		f := newGenreModRepo(t)
		q1, q2, q3, q4 := "Q1", "Q2", "Q3", "Q4"
		published := f.genre("роман", "roman", "published", &q1)
		rejected := f.genre("мусор", "musor", "rejected", &q2)
		alias := f.alias("романы", "romany", published, &q3)
		f.genre("повесть", "povest", "pending", &q4) // not requested
		f.genre("без qid", "bez-qid", "pending", nil)

		got, err := f.repo.ListGenresByQIDs(ctx, []string{"Q1", "Q2", "Q3", "Q999"})
		if err != nil {
			t.Fatalf("ListGenresByQIDs: %v", err)
		}
		byID := map[uuid.UUID]string{}
		for _, g := range got {
			byID[g.ID] = g.Status
		}
		if len(got) != 3 || byID[published] != "published" || byID[rejected] != "rejected" || byID[alias] != "rejected" {
			t.Fatalf("unexpected result: %+v", got)
		}
		for _, g := range got {
			if g.ID == alias && (g.MergedIntoID == nil || *g.MergedIntoID != published) {
				t.Fatalf("alias row must keep merged_into_id, got %+v", g)
			}
		}
	})

	t.Run("empty input returns empty without querying", func(t *testing.T) {
		f := newGenreModRepo(t)
		sqlDB, err := f.db.DB()
		if err != nil {
			t.Fatalf("db: %v", err)
		}
		// A closed DB would fail any query, so success proves none ran.
		_ = sqlDB.Close()

		got, err := f.repo.ListGenresByQIDs(ctx, nil)
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("got=%v err=%v, want empty and nil", got, err)
		}
	})
}
