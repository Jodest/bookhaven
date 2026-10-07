package bookdata

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestJulianToGregorian(t *testing.T) {
	cases := []struct {
		y, m, d          int
		wantY, wantM, wD int
	}{
		{1799, 5, 26, 1799, 6, 6},   // Pushkin
		{1828, 8, 28, 1828, 9, 9},   // Tolstoy
		{1900, 2, 29, 1900, 3, 13},  // Julian-only leap day (offset is still 12 days)
		{1900, 3, 1, 1900, 3, 14},   // offset becomes 13 days
		{1910, 11, 7, 1910, 11, 20}, // Tolstoy (death)
		{1582, 10, 5, 1582, 10, 15}, // Gregorian reform
		{1918, 1, 31, 1918, 2, 13},  // last Julian day in Russia
	}
	for _, c := range cases {
		y, m, d := julianToGregorian(c.y, c.m, c.d)
		if y != c.wantY || m != c.wantM || d != c.wD {
			t.Errorf("julianToGregorian(%d-%02d-%02d) = %d-%02d-%02d, want %d-%02d-%02d",
				c.y, c.m, c.d, y, m, d, c.wantY, c.wantM, c.wD)
		}
	}
}

// dateClaim builds a P569/P570 statement.
func dateClaim(rank, timeStr string, precision int, calendar string) wbEntityClaim {
	raw := fmt.Sprintf(`{"rank":%q,"mainsnak":{"snaktype":"value","datavalue":{"value":{"time":%q,"timezone":0,"before":0,"after":0,"precision":%d,"calendarmodel":%q},"type":"time"}}}`,
		rank, timeStr, precision, calendar)
	var c wbEntityClaim
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		panic(err)
	}
	return c
}

func noValueClaim() wbEntityClaim {
	var c wbEntityClaim
	_ = json.Unmarshal([]byte(`{"rank":"normal","mainsnak":{"snaktype":"somevalue"}}`), &c)
	return c
}

func TestSelectClaimDate(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	greg, jul := calendarGregorian, calendarJulian
	day := func(y, m, d int) *time.Time {
		v := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
		return &v
	}

	cases := []struct {
		name         string
		claims       []wbEntityClaim
		want         *time.Time
		wantUnusable bool
	}{
		{"no statements", nil, nil, false},
		{"only somevalue", []wbEntityClaim{noValueClaim()}, nil, false},
		{"gregorian day", []wbEntityClaim{dateClaim("normal", "+1828-09-09T00:00:00Z", 11, greg)}, day(1828, 9, 9), false},
		{"julian converted", []wbEntityClaim{dateClaim("normal", "+1799-05-26T00:00:00Z", 11, jul)}, day(1799, 6, 6), false},
		{"month precision ignored", []wbEntityClaim{dateClaim("normal", "+1828-09-00T00:00:00Z", 10, greg)}, nil, true},
		{"year precision ignored", []wbEntityClaim{dateClaim("normal", "+1828-00-00T00:00:00Z", 9, greg)}, nil, true},
		{"BCE ignored", []wbEntityClaim{dateClaim("normal", "-0043-03-15T00:00:00Z", 11, jul)}, nil, true},
		{"year zero ignored", []wbEntityClaim{dateClaim("normal", "+0000-03-15T00:00:00Z", 11, greg)}, nil, true},
		{"deprecated ignored", []wbEntityClaim{
			dateClaim("deprecated", "+1910-11-10T00:00:00Z", 11, greg),
			dateClaim("normal", "+1910-11-20T00:00:00Z", 11, greg),
		}, day(1910, 11, 20), false},
		{"only deprecated → unusable", []wbEntityClaim{dateClaim("deprecated", "+1910-11-10T00:00:00Z", 11, greg)}, nil, true},
		{"preferred beats normal", []wbEntityClaim{
			dateClaim("normal", "+1799-05-27T00:00:00Z", 11, jul),
			dateClaim("preferred", "+1799-05-26T00:00:00Z", 11, jul),
			dateClaim("normal", "+1799-06-06T00:00:00Z", 11, greg),
		}, day(1799, 6, 6), false},
		{"imprecise preferred falls through to normal", []wbEntityClaim{
			dateClaim("preferred", "+1799-00-00T00:00:00Z", 9, greg),
			dateClaim("normal", "+1799-06-06T00:00:00Z", 11, greg),
		}, day(1799, 6, 6), false},
		{"conflict in chosen group", []wbEntityClaim{
			dateClaim("normal", "+1799-06-06T00:00:00Z", 11, greg),
			dateClaim("normal", "+1799-06-07T00:00:00Z", 11, greg),
		}, nil, true},
		{"julian + gregorian same day", []wbEntityClaim{
			dateClaim("normal", "+1828-08-28T00:00:00Z", 11, jul),
			dateClaim("normal", "+1828-09-09T00:00:00Z", 11, greg),
		}, day(1828, 9, 9), false},
		{"unknown calendar skipped", []wbEntityClaim{dateClaim("normal", "+1828-09-09T00:00:00Z", 11, "http://www.wikidata.org/entity/Q12345")}, nil, true},
		{"invalid gregorian day skipped", []wbEntityClaim{dateClaim("normal", "+1900-02-29T00:00:00Z", 11, greg)}, nil, true},
		{"julian leap day converted", []wbEntityClaim{dateClaim("normal", "+1900-02-29T00:00:00Z", 11, jul)}, day(1900, 3, 13), false},
		{"future date skipped", []wbEntityClaim{dateClaim("normal", "+2030-01-01T00:00:00Z", 11, greg)}, nil, true},
		{"padded year", []wbEntityClaim{dateClaim("normal", "+00000001828-09-09T00:00:00Z", 11, greg)}, day(1828, 9, 9), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, unusable := selectClaimDate(c.claims, now)
			if unusable != c.wantUnusable {
				t.Errorf("unusable = %v, want %v", unusable, c.wantUnusable)
			}
			switch {
			case c.want == nil && got != nil:
				t.Errorf("date = %v, want nil", got)
			case c.want != nil && (got == nil || !got.Equal(*c.want)):
				t.Errorf("date = %v, want %v", got, c.want)
			}
		})
	}
}

func TestParseAuthorEntity_DeathBeforeBirthDropped(t *testing.T) {
	entity := wbEntity{
		ID: "Q1",
		Claims: map[string][]wbEntityClaim{
			"P569": {dateClaim("normal", "+1900-01-01T00:00:00Z", 11, calendarGregorian)},
			"P570": {dateClaim("normal", "+1890-01-01T00:00:00Z", 11, calendarGregorian)},
		},
	}
	rec := parseAuthorEntity(entity, time.Now())
	if rec.BirthDate == nil || rec.DeathDate != nil || !rec.DeathDateUnusable || rec.BirthDateUnusable {
		t.Fatalf("record = %+v, want birth kept and death dropped as unusable", rec)
	}
}

func TestSelectClaimFile(t *testing.T) {
	parse := func(raw string) []wbEntityClaim {
		var claims []wbEntityClaim
		if err := json.Unmarshal([]byte(raw), &claims); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		return claims
	}
	cases := map[string]struct {
		raw  string
		want string
	}{
		"none": {`[]`, ""},
		"first normal in order": {`[
			{"rank":"normal","mainsnak":{"snaktype":"value","datavalue":{"value":"A.jpg","type":"string"}}},
			{"rank":"normal","mainsnak":{"snaktype":"value","datavalue":{"value":"B.jpg","type":"string"}}}]`, "A.jpg"},
		"preferred wins": {`[
			{"rank":"normal","mainsnak":{"snaktype":"value","datavalue":{"value":"A.jpg","type":"string"}}},
			{"rank":"preferred","mainsnak":{"snaktype":"value","datavalue":{"value":"B.jpg","type":"string"}}}]`, "B.jpg"},
		"deprecated and somevalue skipped": {`[
			{"rank":"deprecated","mainsnak":{"snaktype":"value","datavalue":{"value":"A.jpg","type":"string"}}},
			{"rank":"preferred","mainsnak":{"snaktype":"somevalue"}},
			{"rank":"normal","mainsnak":{"snaktype":"value","datavalue":{"value":{"bad":1},"type":"string"}}},
			{"rank":"normal","mainsnak":{"snaktype":"value","datavalue":{"value":"C.jpg","type":"string"}}}]`, "C.jpg"},
	}
	for name, c := range cases {
		if got := selectClaimFile(parse(c.raw)); got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
}
