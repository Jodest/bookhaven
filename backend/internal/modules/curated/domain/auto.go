package domain

import (
	"sort"
	"strings"

	"github.com/google/uuid"
)

// Auto-collection ranking metrics.
const (
	MetricWeightedRating = "weighted_rating"
	MetricMostRead       = "most_read"
	MetricMostWanted     = "most_wanted"
	MetricTrending       = "trending"
	// Verdict metrics score a book by readers' «Книга или экранизация?» votes.
	MetricBookBeatsAdaptation = "book_beats_adaptation"
	MetricAdaptationBeatsBook = "adaptation_beats_book"
)

// IsVerdictMetric reports whether the metric is scored by adaptation votes.
func IsVerdictMetric(m string) bool {
	return m == MetricBookBeatsAdaptation || m == MetricAdaptationBeatsBook
}

// Rule defaults and bounds.
const (
	DefaultMinRatings = 3
	DefaultConfidence = 10
	// DefaultMinVerdictVotes is the default and the floor of the verdict
	// metrics' minimum. Intentionally independent of screenworks MinVerdictVotes
	// (1); the curated floor stays 5.
	DefaultMinVerdictVotes = 5
	DefaultTrendingDays    = 30
	MaxPeriodDays          = 3650
	MaxRuleParam           = 10000
	// SnapshotReserve is how many books beyond the size a snapshot keeps.
	SnapshotReserve = 20
)

// Movement directions.
const (
	MoveUp   = "up"
	MoveDown = "down"
	MoveSame = "same"
	MoveNew  = "new"
)

// RuleError is an invalid rule field; Msg names the field.
type RuleError struct{ Msg string }

func (e *RuleError) Error() string { return e.Msg }

// Rule is the ranking rule of an automatic collection. MinRatings and
// Confidence apply to the weighted rating and the verdict metrics (for which
// MinRatings is the minimum votes of the source adaptation).
type Rule struct {
	Metric     string
	PeriodDays *int
	MinRatings *int
	Confidence *int
}

func intp(v int) *int { return &v }

// Normalize validates the rule and returns it with defaults filled in;
// MinRatings and Confidence are dropped for metrics that do not use them.
func (r Rule) Normalize() (Rule, error) {
	switch r.Metric {
	case MetricWeightedRating, MetricMostRead, MetricMostWanted, MetricTrending,
		MetricBookBeatsAdaptation, MetricAdaptationBeatsBook:
	default:
		return Rule{}, &RuleError{Msg: "rule.metric: unknown metric"}
	}
	if r.PeriodDays != nil && (*r.PeriodDays < 1 || *r.PeriodDays > MaxPeriodDays) {
		return Rule{}, &RuleError{Msg: "rule.period_days: must be between 1 and 3650"}
	}
	verdict := IsVerdictMetric(r.Metric)
	if verdict && r.MinRatings != nil && (*r.MinRatings < DefaultMinVerdictVotes || *r.MinRatings > MaxRuleParam) {
		return Rule{}, &RuleError{Msg: "rule.min_ratings: must be between 5 and 10000"}
	}
	if r.MinRatings != nil && (*r.MinRatings < 1 || *r.MinRatings > MaxRuleParam) {
		return Rule{}, &RuleError{Msg: "rule.min_ratings: must be between 1 and 10000"}
	}
	if r.Confidence != nil && (*r.Confidence < 1 || *r.Confidence > MaxRuleParam) {
		return Rule{}, &RuleError{Msg: "rule.confidence: must be between 1 and 10000"}
	}
	out := Rule{Metric: r.Metric}
	if r.PeriodDays != nil {
		out.PeriodDays = intp(*r.PeriodDays)
	} else if r.Metric == MetricTrending {
		out.PeriodDays = intp(DefaultTrendingDays)
	}
	if r.Metric == MetricWeightedRating || verdict {
		def := DefaultMinRatings
		if verdict {
			def = DefaultMinVerdictVotes
		}
		out.MinRatings, out.Confidence = intp(def), intp(DefaultConfidence)
		if r.MinRatings != nil {
			out.MinRatings = intp(*r.MinRatings)
		}
		if r.Confidence != nil {
			out.Confidence = intp(*r.Confidence)
		}
	}
	return out, nil
}

// Equal reports whether two normalized rules are identical.
func (r Rule) Equal(o Rule) bool {
	eq := func(a, b *int) bool {
		if a == nil || b == nil {
			return a == b
		}
		return *a == *b
	}
	return r.Metric == o.Metric && eq(r.PeriodDays, o.PeriodDays) &&
		eq(r.MinRatings, o.MinRatings) && eq(r.Confidence, o.Confidence)
}

// RuleOf returns the stored rule of an automatic collection, nil for manual.
func RuleOf(c *Collection) *Rule {
	if c == nil || !c.IsAuto() || c.RuleMetric == nil {
		return nil
	}
	return &Rule{Metric: *c.RuleMetric, PeriodDays: c.RulePeriodDays, MinRatings: c.RuleMinRatings, Confidence: c.RuleConfidence}
}

// SetRule stores a (normalized) rule into the collection columns.
func (c *Collection) SetRule(r Rule) {
	m := r.Metric
	c.RuleMetric, c.RulePeriodDays, c.RuleMinRatings, c.RuleConfidence = &m, r.PeriodDays, r.MinRatings, r.Confidence
}

// SnapshotLimit is the number of books a snapshot stores for a size.
func SnapshotLimit(size int) int {
	if n := size + SnapshotReserve; n < MaxBooks {
		return n
	}
	return MaxBooks
}

// AutoRow is the per-book aggregate the ranking works from. Rating metrics
// use RatingSum/RatingCount, activity metrics SignalCount.
type AutoRow struct {
	BookID uuid.UUID
	Title  string
	// RatingsCount is the book's total rating count (tie-break).
	RatingsCount int
	RatingSum    int64
	RatingCount  int64
	SignalCount  int64
}

// VerdictSource is the adaptation a verdict-scored book was ranked by.
type VerdictSource struct {
	ScreenWorkID uuid.UUID
	Votes        int
	// Share is the raw share (0-1) of the metric's value among Votes.
	Share float64
}

// ScoredBook is a ranked snapshot entry. Source is set for verdict metrics.
type ScoredBook struct {
	BookID uuid.UUID
	Score  float64
	Source *VerdictSource
}

// VerdictRow is the vote aggregate of one visible adaptation of a published
// book. BookVotes and AdaptationVotes count the values «book» and
// «adaptation»; the rest of Votes are «same».
type VerdictRow struct {
	BookID          uuid.UUID
	Title           string
	RatingsCount    int
	ScreenWorkID    uuid.UUID
	WorkTitle       string
	WorkYear        *int
	Votes           int64
	BookVotes       int64
	AdaptationVotes int64
}

// RankVerdicts scores books by their source adaptation (the one with the most
// votes; ties go to the oldest year, no year last, then title, then id). A book
// is ranked when the source has at least the rule's minimum votes and the
// metric's value strictly leads the other two values. The score is
// (k + m*C) / (v + m). Order: score, source votes, ratings count, title, id.
func RankVerdicts(rows []VerdictRow, rule Rule, limit int) []ScoredBook {
	minV, m := int64(DefaultMinVerdictVotes), float64(DefaultConfidence)
	if rule.MinRatings != nil {
		minV = int64(*rule.MinRatings)
	}
	if rule.Confidence != nil {
		m = float64(*rule.Confidence)
	}
	value := func(r VerdictRow) int64 {
		if rule.Metric == MetricAdaptationBeatsBook {
			return r.AdaptationVotes
		}
		return r.BookVotes
	}
	var sumK, sumV int64
	for _, r := range rows {
		sumK += value(r)
		sumV += r.Votes
	}
	var c float64
	if sumV > 0 {
		c = float64(sumK) / float64(sumV)
	}

	less := func(a, b VerdictRow) bool { // a is listed before b on the book page
		if (a.WorkYear == nil) != (b.WorkYear == nil) {
			return a.WorkYear != nil
		}
		if a.WorkYear != nil && *a.WorkYear != *b.WorkYear {
			return *a.WorkYear < *b.WorkYear
		}
		if at, bt := strings.ToLower(a.WorkTitle), strings.ToLower(b.WorkTitle); at != bt {
			return at < bt
		}
		return a.ScreenWorkID.String() < b.ScreenWorkID.String()
	}
	source := map[uuid.UUID]VerdictRow{}
	for _, r := range rows {
		cur, ok := source[r.BookID]
		if !ok || r.Votes > cur.Votes || (r.Votes == cur.Votes && less(r, cur)) {
			source[r.BookID] = r
		}
	}

	type scored struct {
		row   VerdictRow
		score float64
	}
	var out []scored
	for _, r := range source {
		if r.Votes < minV || r.Votes < 1 {
			continue
		}
		same := r.Votes - r.BookVotes - r.AdaptationVotes
		k := value(r)
		other := r.AdaptationVotes
		if rule.Metric == MetricAdaptationBeatsBook {
			other = r.BookVotes
		}
		if k <= other || k <= same {
			continue
		}
		out = append(out, scored{r, (float64(k) + m*c) / (float64(r.Votes) + m)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.score != b.score {
			return a.score > b.score
		}
		if a.row.Votes != b.row.Votes {
			return a.row.Votes > b.row.Votes
		}
		if a.row.RatingsCount != b.row.RatingsCount {
			return a.row.RatingsCount > b.row.RatingsCount
		}
		if at, bt := strings.ToLower(a.row.Title), strings.ToLower(b.row.Title); at != bt {
			return at < bt
		}
		return a.row.BookID.String() < b.row.BookID.String()
	})
	if limit >= 0 && len(out) > limit {
		out = out[:limit]
	}
	res := make([]ScoredBook, len(out))
	for i, s := range out {
		k := value(s.row)
		res[i] = ScoredBook{BookID: s.row.BookID, Score: s.score, Source: &VerdictSource{
			ScreenWorkID: s.row.ScreenWorkID, Votes: int(s.row.Votes), Share: float64(k) / float64(s.row.Votes),
		}}
	}
	return res
}

// RankAuto scores the rows by the rule, drops ineligible books, orders them
// (score desc, ratings count desc, title asc, id asc) and keeps limit.
func RankAuto(rows []AutoRow, rule Rule, limit int) []ScoredBook {
	type scored struct {
		row   AutoRow
		score float64
	}
	var out []scored
	if rule.Metric == MetricWeightedRating {
		var sum, cnt int64
		for _, r := range rows {
			sum += r.RatingSum
			cnt += r.RatingCount
		}
		minR, m := int64(DefaultMinRatings), float64(DefaultConfidence)
		if rule.MinRatings != nil {
			minR = int64(*rule.MinRatings)
		}
		if rule.Confidence != nil {
			m = float64(*rule.Confidence)
		}
		var c float64
		if cnt > 0 {
			c = float64(sum) / float64(cnt)
		}
		for _, r := range rows {
			if r.RatingCount < minR || r.RatingCount == 0 {
				continue
			}
			v := float64(r.RatingCount)
			out = append(out, scored{r, (float64(r.RatingSum) + m*c) / (v + m)})
		}
	} else {
		for _, r := range rows {
			if r.SignalCount < 1 {
				continue
			}
			out = append(out, scored{r, float64(r.SignalCount)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.score != b.score {
			return a.score > b.score
		}
		if a.row.RatingsCount != b.row.RatingsCount {
			return a.row.RatingsCount > b.row.RatingsCount
		}
		if at, bt := strings.ToLower(a.row.Title), strings.ToLower(b.row.Title); at != bt {
			return at < bt
		}
		return a.row.BookID.String() < b.row.BookID.String()
	})
	if limit >= 0 && len(out) > limit {
		out = out[:limit]
	}
	res := make([]ScoredBook, len(out))
	for i, s := range out {
		res[i] = ScoredBook{BookID: s.row.BookID, Score: s.score}
	}
	return res
}

// MovementInfo is a position change between two snapshots.
type MovementInfo struct {
	Direction string
	By        int
}

// Movement compares a snapshot position with the baseline. It returns nil
// when the collection has no baseline.
func Movement(hasBaseline bool, prev *int, pos int) *MovementInfo {
	if !hasBaseline {
		return nil
	}
	switch {
	case prev == nil:
		return &MovementInfo{Direction: MoveNew}
	case *prev > pos:
		return &MovementInfo{Direction: MoveUp, By: *prev - pos}
	case *prev < pos:
		return &MovementInfo{Direction: MoveDown, By: pos - *prev}
	default:
		return &MovementInfo{Direction: MoveSame}
	}
}
