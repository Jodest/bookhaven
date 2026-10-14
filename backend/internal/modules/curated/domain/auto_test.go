package domain

import (
	"math"
	"testing"

	"github.com/google/uuid"
)

func ip(v int) *int { return &v }

func ratedRow(title string, count int64, avg float64) AutoRow {
	return AutoRow{
		BookID: uuid.New(), Title: title, RatingsCount: int(count),
		RatingSum: int64(math.Round(avg * float64(count))), RatingCount: count,
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func TestRankAuto_ManyRatingsBeatFewPerfect(t *testing.T) {
	a, b, c := ratedRow("A", 10, 4.8), ratedRow("B", 2, 5), ratedRow("C", 8, 3.0)
	rule := Rule{Metric: MetricWeightedRating, MinRatings: ip(1), Confidence: ip(10)}
	got := RankAuto([]AutoRow{c, b, a}, rule, 10)
	if len(got) != 3 || got[0].BookID != a.BookID || got[1].BookID != b.BookID || got[2].BookID != c.BookID {
		t.Fatalf("order: %+v", got)
	}
	if !near(got[0].Score, 4.45) || !near(got[1].Score, 4.25) || !near(got[2].Score, 3.61) {
		t.Fatalf("scores: %+v", got)
	}
}

func TestRankAuto_BelowMinimumStillCountsInSiteAverage(t *testing.T) {
	a, b, c := ratedRow("A", 10, 4.8), ratedRow("B", 2, 5), ratedRow("C", 8, 3.0)
	rule := Rule{Metric: MetricWeightedRating, MinRatings: ip(3), Confidence: ip(10)}
	got := RankAuto([]AutoRow{a, b, c}, rule, 10)
	if len(got) != 2 || got[0].BookID != a.BookID || got[1].BookID != c.BookID {
		t.Fatalf("order: %+v", got)
	}
	// B is excluded but still in C=4.1, so the scores are unchanged.
	if !near(got[0].Score, 4.45) || !near(got[1].Score, 3.61) {
		t.Fatalf("scores: %+v", got)
	}
}

func TestRankAuto_ActivityEligibility(t *testing.T) {
	rows := []AutoRow{
		{BookID: uuid.New(), Title: "A", SignalCount: 5},
		{BookID: uuid.New(), Title: "B", SignalCount: 2},
		{BookID: uuid.New(), Title: "C", SignalCount: 0},
	}
	got := RankAuto(rows, Rule{Metric: MetricMostRead}, 10)
	if len(got) != 2 || got[0].BookID != rows[0].BookID || got[0].Score != 5 || got[1].BookID != rows[1].BookID {
		t.Fatalf("got %+v", got)
	}
}

func TestRankAuto_TieOrderAndLimit(t *testing.T) {
	idLow, idHigh := uuid.MustParse("00000000-0000-0000-0000-000000000001"), uuid.MustParse("00000000-0000-0000-0000-000000000002")
	rows := []AutoRow{
		{BookID: idHigh, Title: "Same", RatingsCount: 1, SignalCount: 3},
		{BookID: idLow, Title: "Same", RatingsCount: 1, SignalCount: 3},
		{BookID: uuid.New(), Title: "Beta", RatingsCount: 1, SignalCount: 3},
		{BookID: uuid.New(), Title: "Zed", RatingsCount: 9, SignalCount: 3},
	}
	got := RankAuto(rows, Rule{Metric: MetricMostWanted}, 10)
	want := []uuid.UUID{rows[3].BookID, rows[2].BookID, idLow, idHigh}
	for i, w := range want {
		if got[i].BookID != w {
			t.Fatalf("pos %d: got %v want %v", i, got[i].BookID, w)
		}
	}
	if lim := RankAuto(rows, Rule{Metric: MetricMostWanted}, 2); len(lim) != 2 {
		t.Fatalf("limit: %d", len(lim))
	}
}

func TestSnapshotLimit(t *testing.T) {
	for size, want := range map[int]int{1000: 1000, 100: 120, 990: 1000, 1: 21} {
		if got := SnapshotLimit(size); got != want {
			t.Errorf("size %d: got %d want %d", size, got, want)
		}
	}
}

func TestMovement(t *testing.T) {
	if Movement(false, ip(3), 1) != nil {
		t.Fatal("no baseline must be nil")
	}
	cases := []struct {
		prev *int
		pos  int
		dir  string
		by   int
	}{
		{ip(7), 4, MoveUp, 3},
		{ip(2), 4, MoveDown, 2},
		{ip(5), 5, MoveSame, 0},
		{nil, 10, MoveNew, 0},
	}
	for _, c := range cases {
		m := Movement(true, c.prev, c.pos)
		if m == nil || m.Direction != c.dir || m.By != c.by {
			t.Errorf("prev %v pos %d: got %+v", c.prev, c.pos, m)
		}
	}
}

func TestRuleNormalize(t *testing.T) {
	r, err := Rule{Metric: MetricWeightedRating}.Normalize()
	if err != nil || *r.MinRatings != 3 || *r.Confidence != 10 || r.PeriodDays != nil {
		t.Fatalf("weighted defaults: %+v %v", r, err)
	}
	r, err = Rule{Metric: MetricTrending, MinRatings: ip(5)}.Normalize()
	if err != nil || *r.PeriodDays != 30 || r.MinRatings != nil || r.Confidence != nil {
		t.Fatalf("trending defaults: %+v %v", r, err)
	}
	r, _ = Rule{Metric: MetricMostRead}.Normalize()
	if r.PeriodDays != nil {
		t.Fatalf("most_read period: %+v", r)
	}
	bad := map[string]Rule{
		"rule.metric: unknown metric":                   {Metric: "x"},
		"rule.period_days: must be between 1 and 3650":  {Metric: MetricMostRead, PeriodDays: ip(0)},
		"rule.min_ratings: must be between 1 and 10000": {Metric: MetricWeightedRating, MinRatings: ip(10001)},
		"rule.confidence: must be between 1 and 10000":  {Metric: MetricWeightedRating, Confidence: ip(0)},
	}
	for want, rule := range bad {
		_, err := rule.Normalize()
		if err == nil {
			t.Errorf("%s: no error", want)
			continue
		}
		if err.Error() != want {
			t.Errorf("got %q want %q", err.Error(), want)
		}
	}
}
