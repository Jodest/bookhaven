package service

import (
	"hash/fnv"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/bookhaven/backend/internal/modules/home/repository"
	"github.com/google/uuid"
)

// Home page rules (see docs/features/home-page-content.md, Business Rules).
const (
	// GoodRatingMin is the "well-rated" rating threshold.
	GoodRatingMin = 4.0
	// GoodRatingsCountMin is the minimum ratings count for "well-rated".
	GoodRatingsCountMin = 3
	// RowLimit is the number of books in the "new" and "top-rated" rows.
	RowLimit                = 10
	CalendarMaxAuthors      = 6
	CalendarBooksPerAuthor  = 3
	CalendarLookaheadDays   = 7
	AnniversaryStep         = 25
	AnniversaryLimit        = 6
	RecentReviewsLimit      = 3
	ReviewExcerptRunes      = 280
	DescriptionExcerptRunes = 300
	// ReviewMinLength skips one-liners ("ок, норм") in fresh reviews.
	ReviewMinLength = 40
	HomeCacheTTL    = 10 * time.Minute

	// bookOfTheDaySalt salts the daily pick of the book of the day.
	bookOfTheDaySalt = "botd"
)

// moscow is the fixed UTC+3 zone that defines "today". Moscow has had no
// DST since 2014; a fixed zone avoids a tzdata dependency in the container.
var moscow = time.FixedZone("MSK", 3*60*60)

// civilDate returns t's Moscow calendar date as midnight UTC, so date
// arithmetic (AddDate) and formatting never depend on the host zone.
func civilDate(t time.Time) time.Time {
	m := t.In(moscow)
	return time.Date(m.Year(), m.Month(), m.Day(), 0, 0, 0, 0, time.UTC)
}

// dateString formats a civil date as YYYY-MM-DD.
func dateString(d time.Time) string {
	return d.Format("2006-01-02")
}

// dayNumber is the number of days since 1970-01-01 of a civil date.
func dayNumber(d time.Time) int64 {
	return d.Unix() / 86400
}

func dailyKey(salt string, day time.Time, id uuid.UUID) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(salt + "|" + dateString(day) + "|" + id.String()))
	return h.Sum64()
}

// smallestKey returns the pool ID with the smallest daily key, skipping
// excluded IDs. ok is false when nothing is left.
func smallestKey(pool []uuid.UUID, day time.Time, salt string, exclude map[uuid.UUID]struct{}) (uuid.UUID, bool) {
	var best uuid.UUID
	var bestKey uint64
	found := false
	for _, id := range pool {
		if _, skip := exclude[id]; skip {
			continue
		}
		k := dailyKey(salt, day, id)
		if !found || k < bestKey || (k == bestKey && id.String() < best.String()) {
			best, bestKey, found = id, k, true
		}
	}
	return best, found
}

// pickDaily deterministically picks one ID from pool for the civil date
// day. Every ID gets the key fnv64a(salt|YYYY-MM-DD|id) and the smallest
// key wins. To guarantee no repeat on consecutive days against the same
// pool without unbounded recursion into previous days:
//   - even days take the smallest key;
//   - odd days take the smallest key excluding the picks of both
//     neighbouring (even) days;
//   - a two-element pool alternates by day parity (in a salted order).
//
// It returns false for an empty pool.
func pickDaily(pool []uuid.UUID, day time.Time, salt string) (uuid.UUID, bool) {
	switch len(pool) {
	case 0:
		return uuid.Nil, false
	case 1:
		return pool[0], true
	}
	dn := dayNumber(day)
	if len(pool) == 2 {
		a, b := pool[0], pool[1]
		ka, kb := dailyKey(salt, time.Time{}, a), dailyKey(salt, time.Time{}, b)
		if kb < ka || (kb == ka && b.String() < a.String()) {
			a, b = b, a
		}
		if dn%2 == 0 {
			return a, true
		}
		return b, true
	}
	if dn%2 == 0 {
		return smallestKey(pool, day, salt, nil)
	}
	exclude := make(map[uuid.UUID]struct{}, 2)
	if prev, ok := smallestKey(pool, day.AddDate(0, 0, -1), salt, nil); ok {
		exclude[prev] = struct{}{}
	}
	if next, ok := smallestKey(pool, day.AddDate(0, 0, 1), salt, nil); ok {
		exclude[next] = struct{}{}
	}
	return smallestKey(pool, day, salt, exclude)
}

func isLeap(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// calendarDays returns the (month, day) pairs matched on civil date d. In a
// non-leap year Feb 28 also matches Feb 29.
func calendarDays(d time.Time) []repository.MonthDay {
	days := []repository.MonthDay{{Month: int(d.Month()), Day: d.Day()}}
	if d.Month() == time.February && d.Day() == 28 && !isLeap(d.Year()) {
		days = append(days, repository.MonthDay{Month: 2, Day: 29})
	}
	return days
}

// eventYears is the number of years since the event in the given year.
// Callers drop results <= 0 (future dates are bad data).
func eventYears(event time.Time, year int) int {
	return year - event.Year()
}

// calendarEvent is one selected calendar event (cached at ID level).
type calendarEvent struct {
	Author  repository.CalendarEventRow `json:"author"`
	Years   int                         `json:"years"`
	BookIDs []uuid.UUID                 `json:"book_ids"`
}

// orderCalendarEvents sorts in place: round events (years % 25 == 0)
// first, then visible-book count DESC, then name ASC.
func orderCalendarEvents(events []calendarEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		ri := events[i].Years%AnniversaryStep == 0
		rj := events[j].Years%AnniversaryStep == 0
		if ri != rj {
			return ri
		}
		if events[i].Author.VisibleBooks != events[j].Author.VisibleBooks {
			return events[i].Author.VisibleBooks > events[j].Author.VisibleBooks
		}
		if events[i].Author.Name != events[j].Author.Name {
			return events[i].Author.Name < events[j].Author.Name
		}
		return events[i].Author.Type < events[j].Author.Type
	})
}

// isAnniversary reports whether age is a positive multiple of step.
func isAnniversary(age, step int) bool {
	return age > 0 && step > 0 && age%step == 0
}

// anniversaryRank orders anniversaries: multiples of 100 (0) before
// multiples of 50 (1) before the rest (2).
func anniversaryRank(age int) int {
	switch {
	case age%100 == 0:
		return 0
	case age%50 == 0:
		return 1
	default:
		return 2
	}
}

// trailingCut lists characters trimmed from the end of a cut excerpt.
func trailingCut(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune(".,;:!?-–—…(«\"'", r)
}

// excerpt shortens s to at most limit runes (plus "…"). The cut happens at
// the last whitespace before the limit (a single long word is hard-cut),
// trailing punctuation is trimmed and "…" is appended only when the text
// was cut.
func excerpt(s string, limit int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if limit <= 0 || len(runes) <= limit {
		return s
	}
	cut := runes[:limit]
	// When the limit falls exactly on a word boundary the last word is kept.
	if !unicode.IsSpace(runes[limit]) {
		if idx := lastSpace(cut); idx > 0 {
			cut = cut[:idx]
		}
	}
	out := strings.TrimRightFunc(string(cut), trailingCut)
	if out == "" {
		out = string(runes[:limit])
	}
	return out + "…"
}

func lastSpace(rs []rune) int {
	for i := len(rs) - 1; i >= 0; i-- {
		if unicode.IsSpace(rs[i]) {
			return i
		}
	}
	return -1
}
