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

func vrow(book uuid.UUID, title string, work uuid.UUID, workTitle string, year *int, votes, bk, ad int64) VerdictRow {
	return VerdictRow{BookID: book, Title: title, RatingsCount: 1, ScreenWorkID: work, WorkTitle: workTitle, WorkYear: year,
		Votes: votes, BookVotes: bk, AdaptationVotes: ad}
}

func vrule(metric string) Rule {
	r, _ := Rule{Metric: metric}.Normalize()
	return r
}

func TestRankVerdicts_WarAndPeaceExample(t *testing.T) {
	b, w67, w16 := uuid.New(), uuid.New(), uuid.New()
	rows := []VerdictRow{
		vrow(b, "Война и мир", w67, "Война и мир", ip(1967), 40, 36, 3),
		vrow(b, "Война и мир", w16, "Война и мир", ip(2016), 10, 5, 4),
	}
	got := RankVerdicts(rows, vrule(MetricBookBeatsAdaptation), 10)
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if math.Abs(got[0].Score-0.884) > 1e-9 {
		t.Fatalf("score %v", got[0].Score)
	}
	src := got[0].Source
	if src == nil || src.ScreenWorkID != w67 || src.Votes != 40 || math.Abs(src.Share-0.9) > 1e-9 {
		t.Fatalf("source %+v", src)
	}
}

func TestRankVerdicts_SourceTieBreak(t *testing.T) {
	b, old, young, none := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	rows := []VerdictRow{
		vrow(b, "К", young, "Б", ip(2016), 12, 8, 2),
		vrow(b, "К", none, "А", nil, 12, 8, 2),
		vrow(b, "К", old, "В", ip(1967), 12, 8, 2),
	}
	got := RankVerdicts(rows, vrule(MetricBookBeatsAdaptation), 10)
	if len(got) != 1 || got[0].Source.ScreenWorkID != old {
		t.Fatalf("equal votes must pick the oldest year: %+v", got)
	}
	got = RankVerdicts(rows[:2], vrule(MetricBookBeatsAdaptation), 10)
	if got[0].Source.ScreenWorkID != young {
		t.Fatalf("a work without a year goes last: %+v", got)
	}
}

func TestRankVerdicts_MinimumAndNoFallback(t *testing.T) {
	b, w1, w2 := uuid.New(), uuid.New(), uuid.New()
	// The most-voted adaptation has 4 votes; the other has fewer, so nothing passes.
	rows := []VerdictRow{vrow(b, "К", w1, "А", ip(2000), 4, 4, 0), vrow(b, "К", w2, "Б", ip(2001), 3, 3, 0)}
	if got := RankVerdicts(rows, vrule(MetricBookBeatsAdaptation), 10); len(got) != 0 {
		t.Fatalf("below the minimum must not rank: %+v", got)
	}
	rule := Rule{Metric: MetricBookBeatsAdaptation, MinRatings: ip(10), Confidence: ip(10)}
	rows = []VerdictRow{vrow(b, "К", w1, "А", ip(2000), 9, 9, 0)}
	if got := RankVerdicts(rows, rule, 10); len(got) != 0 {
		t.Fatalf("custom minimum 10: %+v", got)
	}
}

func TestRankVerdicts_LeaderGateAndMirror(t *testing.T) {
	tie, sameMajor, adWins := uuid.New(), uuid.New(), uuid.New()
	rows := []VerdictRow{
		vrow(tie, "Т", uuid.New(), "а", nil, 10, 4, 4),       // 4/4/2: tie
		vrow(sameMajor, "С", uuid.New(), "б", nil, 10, 4, 1), // 4/1/5: same leads
		vrow(adWins, "А", uuid.New(), "в", nil, 10, 1, 8),    // adaptation leads
	}
	book := RankVerdicts(rows, vrule(MetricBookBeatsAdaptation), 10)
	if len(book) != 0 {
		t.Fatalf("book_beats_adaptation: %+v", book)
	}
	ad := RankVerdicts(rows, vrule(MetricAdaptationBeatsBook), 10)
	if len(ad) != 1 || ad[0].BookID != adWins {
		t.Fatalf("adaptation_beats_book: %+v", ad)
	}
	if math.Abs(ad[0].Source.Share-0.8) > 1e-9 {
		t.Fatalf("share %v", ad[0].Source.Share)
	}
}

func TestRankVerdicts_TieOrder(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	// Same score (same k/v), differing votes and ratings counts.
	ra := vrow(a, "А", uuid.New(), "x", nil, 10, 8, 0)
	rb := vrow(b, "Б", uuid.New(), "x", nil, 20, 16, 0)
	rc := vrow(c, "В", uuid.New(), "x", nil, 10, 8, 0)
	ra.RatingsCount, rc.RatingsCount = 1, 5
	// Different v gives different scores with C = 0.8, k/v = 0.8: all equal to 0.8.
	got := RankVerdicts([]VerdictRow{ra, rb, rc}, vrule(MetricBookBeatsAdaptation), 10)
	if len(got) != 3 || got[0].BookID != b || got[1].BookID != c || got[2].BookID != a {
		t.Fatalf("order: %+v", got)
	}
	if lim := RankVerdicts([]VerdictRow{ra, rb, rc}, vrule(MetricBookBeatsAdaptation), 2); len(lim) != 2 {
		t.Fatalf("limit: %d", len(lim))
	}
}

func TestRuleNormalize_Verdict(t *testing.T) {
	for _, m := range []string{MetricBookBeatsAdaptation, MetricAdaptationBeatsBook} {
		r, err := Rule{Metric: m}.Normalize()
		if err != nil || *r.MinRatings != 5 || *r.Confidence != 10 || r.PeriodDays != nil {
			t.Fatalf("%s defaults: %+v %v", m, r, err)
		}
		r, err = Rule{Metric: m, MinRatings: ip(20), Confidence: ip(3), PeriodDays: ip(30)}.Normalize()
		if err != nil || *r.MinRatings != 20 || *r.Confidence != 3 || *r.PeriodDays != 30 {
			t.Fatalf("%s custom: %+v %v", m, r, err)
		}
		if _, err := (Rule{Metric: m, MinRatings: ip(4)}).Normalize(); err == nil || err.Error() != "rule.min_ratings: must be between 5 and 10000" {
			t.Fatalf("%s min 4: %v", m, err)
		}
		if _, err := (Rule{Metric: m, MinRatings: ip(10001)}).Normalize(); err == nil {
			t.Fatalf("%s min 10001: no error", m)
		}
	}
	if r, _ := (Rule{Metric: MetricWeightedRating, MinRatings: ip(2)}).Normalize(); *r.MinRatings != 2 {
		t.Fatalf("weighted keeps the 1 floor: %+v", r)
	}
}
