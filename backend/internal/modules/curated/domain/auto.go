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
)

// Rule defaults and bounds.
const (
	DefaultMinRatings   = 3
	DefaultConfidence   = 10
	DefaultTrendingDays = 30
	MaxPeriodDays       = 3650
	MaxRuleParam        = 10000
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
// Confidence apply to the weighted rating only.
type Rule struct {
	Metric     string
	PeriodDays *int
	MinRatings *int
	Confidence *int
}

func intp(v int) *int { return &v }

// Normalize validates the rule and returns it with defaults filled in;
// MinRatings and Confidence are dropped for non-weighted metrics.
func (r Rule) Normalize() (Rule, error) {
	switch r.Metric {
	case MetricWeightedRating, MetricMostRead, MetricMostWanted, MetricTrending:
	default:
		return Rule{}, &RuleError{Msg: "rule.metric: unknown metric"}
	}
	if r.PeriodDays != nil && (*r.PeriodDays < 1 || *r.PeriodDays > MaxPeriodDays) {
		return Rule{}, &RuleError{Msg: "rule.period_days: must be between 1 and 3650"}
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
	if r.Metric == MetricWeightedRating {
		out.MinRatings, out.Confidence = intp(DefaultMinRatings), intp(DefaultConfidence)
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

// ScoredBook is a ranked snapshot entry.
type ScoredBook struct {
	BookID uuid.UUID
	Score  float64
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
