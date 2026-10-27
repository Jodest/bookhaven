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
	zero := NewVerdict(VerdictCounts{Votes: MinVerdictVotes - 1})
	if zero.Votes != 0 || zero.BookBetter != nil || zero.AdaptationBetter != nil || zero.Same != nil {
		t.Fatalf("zero = %+v", zero)
	}
	one := NewVerdict(VerdictCounts{Votes: MinVerdictVotes, BookBetter: 1})
	if MinVerdictVotes != 1 || one.BookBetter == nil || *one.BookBetter != 1 || one.Same == nil || *one.Same != 0 {
		t.Fatalf("one = %+v", one)
	}
}
