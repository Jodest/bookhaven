package service

import (
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

func TestGenreHintPairs_KindsAndDedup(t *testing.T) {
	svc := &Service{}
	mk := func(name string) catalogDomain.Genre { return catalogDomain.Genre{ID: uuid.New(), Name: name} }
	roman, romany := mk("роман"), mk("романы")
	exactA, exactB := mk("Фэнтези"), mk("фэнтези")
	word1, word2 := mk("Научная фантастика"), mk("Космическая фантастика")
	alone := mk("Поэзия")

	pairs := svc.GenreHintPairs(nil, []catalogDomain.Genre{roman, romany, exactA, exactB, word1, word2, alone})
	got := map[[2]uuid.UUID]string{}
	for _, p := range pairs {
		a, b := p.A, p.B
		if a.String() > b.String() {
			a, b = b, a
		}
		if _, dup := got[[2]uuid.UUID{a, b}]; dup {
			t.Fatalf("duplicate pair %v", p)
		}
		got[[2]uuid.UUID{a, b}] = p.Match
	}
	kind := func(x, y catalogDomain.Genre) string {
		a, b := x.ID, y.ID
		if a.String() > b.String() {
			a, b = b, a
		}
		return got[[2]uuid.UUID{a, b}]
	}
	if kind(exactA, exactB) != GenreMatchExactName {
		t.Fatalf("exact: %q", kind(exactA, exactB))
	}
	if kind(roman, romany) != GenreMatchSameStem {
		t.Fatalf("stem: %q", kind(roman, romany))
	}
	if kind(word1, word2) != GenreMatchSharedWord {
		t.Fatalf("word: %q", kind(word1, word2))
	}
	for _, p := range pairs {
		if p.A == alone.ID || p.B == alone.ID {
			t.Fatalf("unrelated genre hinted: %v", p)
		}
	}
}
