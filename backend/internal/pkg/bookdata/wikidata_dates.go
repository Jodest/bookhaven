package bookdata

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Wikidata calendar models of a time value (P569/P570).
const (
	calendarGregorian = "http://www.wikidata.org/entity/Q1985727"
	calendarJulian    = "http://www.wikidata.org/entity/Q1985786"
)

// wikidataDayPrecision is the time-value precision for a full date.
const wikidataDayPrecision = 11

// wbTimeValue is the subset of a Wikibase "time" datavalue used here. In
// the wbgetentities JSON the "time" string is expressed in the calendar
// named by "calendarmodel" (verified live, see
// docs/features/author-wikidata-enrichment.md Implementation Notes).
type wbTimeValue struct {
	Time          string `json:"time"`
	Precision     int    `json:"precision"`
	CalendarModel string `json:"calendarmodel"`
}

// rankGroups is the rank order used for statement selection; deprecated
// statements are never considered.
var rankGroups = []string{"preferred", "normal"}

// claimRank normalizes a statement rank (missing → normal).
func claimRank(c wbEntityClaim) string {
	if c.Rank == "" {
		return "normal"
	}
	return c.Rank
}

// selectClaimDate picks a day-precision Gregorian date from P569/P570
// statements (docs/features/author-wikidata-enrichment.md rule 2): the
// highest rank group (preferred over normal, deprecated ignored) holding
// at least one usable value wins; within it every usable value must
// resolve to the same date, otherwise the field is skipped (conflict).
//
// unusable reports that value statements exist but none produced a date.
func selectClaimDate(claims []wbEntityClaim, now time.Time) (date *time.Time, unusable bool) {
	hasValue := false
	for _, c := range claims {
		if c.Mainsnak.SnakType == "value" && c.Mainsnak.DataValue != nil {
			hasValue = true
		}
	}
	for _, rank := range rankGroups {
		var chosen *time.Time
		conflict := false
		for _, c := range claims {
			if c.Mainsnak.SnakType != "value" || c.Mainsnak.DataValue == nil || claimRank(c) != rank {
				continue
			}
			var v wbTimeValue
			if err := json.Unmarshal(c.Mainsnak.DataValue.Value, &v); err != nil {
				continue
			}
			d, ok := usableDate(v, now)
			if !ok {
				continue
			}
			if chosen == nil {
				chosen = &d
			} else if !chosen.Equal(d) {
				conflict = true
			}
		}
		if chosen != nil {
			if conflict {
				return nil, true
			}
			return chosen, false
		}
	}
	return nil, hasValue
}

// usableDate converts one time value into a Gregorian UTC-midnight date,
// or reports false when it is not usable: precision other than day, a
// non-positive year, an unknown calendar model, an invalid day, or a date
// in the future. Julian values are converted via the Julian Day Number.
func usableDate(v wbTimeValue, now time.Time) (time.Time, bool) {
	if v.Precision != wikidataDayPrecision {
		return time.Time{}, false
	}
	y, m, d, ok := parseWikidataTime(v.Time)
	if !ok {
		return time.Time{}, false
	}
	switch v.CalendarModel {
	case calendarGregorian:
	case calendarJulian:
		if !validJulianDate(y, m, d) {
			return time.Time{}, false
		}
		y, m, d = julianToGregorian(y, m, d)
	default:
		return time.Time{}, false
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if t.Year() != y || int(t.Month()) != m || t.Day() != d {
		return time.Time{}, false
	}
	if t.After(now) {
		return time.Time{}, false
	}
	return t, true
}

// parseWikidataTime parses "+YYYY-MM-DDT00:00:00Z" (the year may carry
// extra digits). Negative years, year 0 and a zero month/day are rejected.
func parseWikidataTime(s string) (year, month, day int, ok bool) {
	if !strings.HasPrefix(s, "+") {
		return 0, 0, 0, false
	}
	datePart, _, _ := strings.Cut(s[1:], "T")
	parts := strings.Split(datePart, "-")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var err error
	if year, err = strconv.Atoi(parts[0]); err != nil || year <= 0 || year > 9999 {
		return 0, 0, 0, false
	}
	if month, err = strconv.Atoi(parts[1]); err != nil || month < 1 || month > 12 {
		return 0, 0, 0, false
	}
	if day, err = strconv.Atoi(parts[2]); err != nil || day < 1 || day > 31 {
		return 0, 0, 0, false
	}
	return year, month, day, true
}

// validJulianDate reports whether y-m-d exists in the Julian calendar
// (every fourth year is a leap year).
func validJulianDate(y, m, d int) bool {
	days := [...]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if m < 1 || m > 12 || d < 1 {
		return false
	}
	limit := days[m-1]
	if m == 2 && y%4 == 0 {
		limit = 29
	}
	return d <= limit
}

// julianToGregorian converts a (valid, positive-year) proleptic Julian
// calendar date to the Gregorian calendar via the Julian Day Number.
func julianToGregorian(y, m, d int) (int, int, int) {
	a := (14 - m) / 12
	yy := y + 4800 - a
	mm := m + 12*a - 3
	jdn := d + (153*mm+2)/5 + 365*yy + yy/4 - 32083

	// JDN → Gregorian (Richards' algorithm).
	b := jdn + 32044
	c := (4*b + 3) / 146097
	e := b - 146097*c/4
	f := (4*e + 3) / 1461
	g := e - 1461*f/4
	h := (5*g + 2) / 153
	day := g - (153*h+2)/5 + 1
	month := h + 3 - 12*(h/10)
	year := 100*c + f - 4800 + h/10
	return year, month, day
}

// selectClaimFile returns the P18 filename of the first value statement in
// the highest non-deprecated rank group, or "".
func selectClaimFile(claims []wbEntityClaim) string {
	for _, rank := range rankGroups {
		for _, c := range claims {
			if claimRank(c) != rank || c.Mainsnak.SnakType != "value" || c.Mainsnak.DataValue == nil {
				continue
			}
			var name string
			if err := json.Unmarshal(c.Mainsnak.DataValue.Value, &name); err != nil {
				continue
			}
			if name = strings.TrimSpace(name); name != "" {
				return name
			}
		}
	}
	return ""
}
