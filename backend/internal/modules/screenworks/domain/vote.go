package domain

import (
	"errors"

	"github.com/google/uuid"
)

// Vote values of «Книга или экранизация?» (CHECK constraint of migration
// 000035; the same strings as the API).
const (
	VoteBook       = "book"
	VoteAdaptation = "adaptation"
	VoteSame       = "same"
)

// MinVerdictVotes is the number of votes from which the per-value counts of an
// adaptation are revealed.
const MinVerdictVotes = 5

var (
	ErrVoteValue          = errors.New("invalid vote value")
	ErrNotReader          = errors.New("only readers who have read the book can vote")
	ErrAdaptationNotFound = errors.New("adaptation not found")
)

// ParseVoteValue validates a vote value from a request.
func ParseVoteValue(v string) (string, error) {
	switch v {
	case VoteBook, VoteAdaptation, VoteSame:
		return v, nil
	}
	return "", ErrVoteValue
}

// VerdictCounts are the stored vote counts of one visible adaptation and the
// caller's own vote (nil without one).
type VerdictCounts struct {
	ScreenWorkID     uuid.UUID
	Votes            int
	BookBetter       int
	AdaptationBetter int
	Same             int
	MyVote           *string
}

// Verdict is the public verdict of one adaptation. The per-value counts are
// nil below MinVerdictVotes.
type Verdict struct {
	ScreenWorkID     uuid.UUID
	Votes            int
	BookBetter       *int
	AdaptationBetter *int
	Same             *int
	MyVote           *string
}

// Verdicts is the result of the verdicts endpoint.
type Verdicts struct {
	CanVote bool
	Items   []Verdict
}

// NewVerdict hides the per-value counts of a small sample.
func NewVerdict(c VerdictCounts) Verdict {
	v := Verdict{ScreenWorkID: c.ScreenWorkID, Votes: c.Votes, MyVote: c.MyVote}
	if c.Votes >= MinVerdictVotes {
		v.BookBetter, v.AdaptationBetter, v.Same = &c.BookBetter, &c.AdaptationBetter, &c.Same
	}
	return v
}
