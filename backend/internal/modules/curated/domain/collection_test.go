package domain

import (
	"testing"

	"github.com/google/uuid"
)

func refs(pub ...bool) []BookRef {
	out := make([]BookRef, len(pub))
	for i, p := range pub {
		out[i] = BookRef{BookID: uuid.New(), Published: p}
	}
	return out
}

func places(r []RankedItem) []int {
	out := make([]int, len(r))
	for i, x := range r {
		out[i] = x.Place
	}
	return out
}

func eq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRank_UnpublishedShiftsUp(t *testing.T) {
	// size 3: A B C D, B unpublished -> A=1 C=2 D=3
	r := Rank(refs(true, false, true, true), 3)
	if !eq(places(r), []int{1, 0, 2, 3}) || r[1].State != StateHidden {
		t.Fatalf("got %v", places(r))
	}
}

func TestRank_RepublishedReturns(t *testing.T) {
	r := Rank(refs(true, true, true, true), 3)
	if !eq(places(r), []int{1, 2, 3, 0}) || r[3].State != StateReserve {
		t.Fatalf("got %v", places(r))
	}
}

func TestRank_ReserveExhausted(t *testing.T) {
	pub := make([]bool, 0, 102)
	for i := 0; i < 98; i++ {
		pub = append(pub, true)
	}
	pub = append(pub, false, false, false, false)
	r := Rank(refs(pub...), 100)
	shown := 0
	for _, x := range r {
		if x.State == StateRanked {
			shown++
		}
	}
	if shown != 98 || r[97].Place != 98 {
		t.Fatalf("shown %d", shown)
	}
	if ShownCount(98, 100) != 98 || ShownCount(150, 100) != 100 {
		t.Fatal("ShownCount")
	}
}

func TestRank_SizeShrink(t *testing.T) {
	items := refs(true, true, true, true)
	r := Rank(items, 2)
	if !eq(places(r), []int{1, 2, 0, 0}) || r[2].State != StateReserve {
		t.Fatalf("got %v", places(r))
	}
	if r[3].Position != 4 {
		t.Fatal("positions must stay")
	}
}

func TestRank_AllHidden(t *testing.T) {
	r := Rank(refs(false, false), 5)
	for _, x := range r {
		if x.State != StateHidden || x.Place != 0 {
			t.Fatalf("got %+v", x)
		}
	}
}
