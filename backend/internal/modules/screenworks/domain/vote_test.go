package domain

import "testing"

func TestParseVoteValue(t *testing.T) {
	for _, ok := range []string{"book", "adaptation", "same"} {
		if got, err := ParseVoteValue(ok); err != nil || got != ok {
			t.Errorf("%q: got %q, %v", ok, got, err)
		}
	}
	for _, bad := range []string{"", "film", "Book", " book"} {
		if _, err := ParseVoteValue(bad); err != ErrVoteValue {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestNewVerdictMinimum(t *testing.T) {
	below := NewVerdict(VerdictCounts{Votes: MinVerdictVotes - 1, BookBetter: 4})
	if below.Votes != 4 || below.BookBetter != nil || below.AdaptationBetter != nil || below.Same != nil {
		t.Fatalf("below = %+v", below)
	}
	at := NewVerdict(VerdictCounts{Votes: MinVerdictVotes, BookBetter: 5})
	if at.BookBetter == nil || *at.BookBetter != 5 || at.Same == nil || *at.Same != 0 {
		t.Fatalf("at = %+v", at)
	}
}
