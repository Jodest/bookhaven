package domain

import (
	"errors"
	"strings"
	"testing"
)

func sp(s string) *string { return &s }
func ip(i int) *int       { return &i }

func TestNormalizeWorkFields_Errors(t *testing.T) {
	ok := WorkFields{Title: "Солярис", Kind: KindFilm}
	cases := []struct {
		name string
		mut  func(f *WorkFields)
		msg  string
	}{
		{"empty title", func(f *WorkFields) { f.Title = "   " }, "title is required"},
		{"long title", func(f *WorkFields) { f.Title = strings.Repeat("я", 501) }, "title is too long"},
		{"long original", func(f *WorkFields) { f.OriginalTitle = sp(strings.Repeat("a", 501)) }, "original title is too long"},
		{"bad kind", func(f *WorkFields) { f.Kind = "movie" }, "invalid kind"},
		{"empty kind", func(f *WorkFields) { f.Kind = "" }, "invalid kind"},
		{"year low", func(f *WorkFields) { f.Year = ip(1850) }, "year must be between 1880 and 2100"},
		{"year high", func(f *WorkFields) { f.Year = ip(2101) }, "year must be between 1880 and 2100"},
		{"long director", func(f *WorkFields) { f.Director = sp(strings.Repeat("d", 501)) }, "director is too long"},
		{"imdb link", func(f *WorkFields) { f.IMDbID = sp("https://www.imdb.com/title/tt0120737/") }, "invalid imdb id"},
		{"imdb no digits", func(f *WorkFields) { f.IMDbID = sp("tt") }, "invalid imdb id"},
		{"imdb too long", func(f *WorkFields) { f.IMDbID = sp("tt" + strings.Repeat("1", 19)) }, "invalid imdb id"},
		{"kinopoisk letters", func(f *WorkFields) { f.KinopoiskID = sp("12a") }, "invalid kinopoisk id"},
		{"kinopoisk link", func(f *WorkFields) { f.KinopoiskID = sp("https://www.kinopoisk.ru/film/43911/") }, "invalid kinopoisk id"},
		{"qid bad", func(f *WorkFields) { f.QID = sp("X12") }, "invalid qid"},
		{"qid no digits", func(f *WorkFields) { f.QID = sp("Q") }, "invalid qid"},
	}
	for _, c := range cases {
		f := ok
		c.mut(&f)
		_, err := NormalizeWorkFields(f)
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Msg != c.msg {
			t.Errorf("%s: got %v, want %q", c.name, err, c.msg)
		}
	}
}

func TestNormalizeWorkFields_Normalises(t *testing.T) {
	got, err := NormalizeWorkFields(WorkFields{
		Title: "  Солярис ", OriginalTitle: sp("Солярис"), Kind: KindFilm, Year: ip(1972),
		Director: sp("  "), IMDbID: sp(" tt0069293 "), KinopoiskID: sp(""), QID: sp(" q181069 "),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Солярис" || got.OriginalTitle != nil || got.Director != nil || got.KinopoiskID != nil {
		t.Errorf("blank/equal not nil: %+v", got)
	}
	if got.IMDbID == nil || *got.IMDbID != "tt0069293" || got.QID == nil || *got.QID != "Q181069" {
		t.Errorf("trim/upper: %+v", got)
	}
}

func TestNormalizeWorkFields_Boundaries(t *testing.T) {
	for _, y := range []int{1880, 2100} {
		if _, err := NormalizeWorkFields(WorkFields{Title: "x", Kind: KindSeries, Year: ip(y)}); err != nil {
			t.Errorf("year %d: %v", y, err)
		}
	}
	if _, err := NormalizeWorkFields(WorkFields{Title: strings.Repeat("я", 500), Kind: KindTeleplay}); err != nil {
		t.Errorf("500 runes: %v", err)
	}
}

func TestIsValidRelation(t *testing.T) {
	if !IsValidRelation("biopic") || !IsValidRelation("documentary") || IsValidRelation("") || IsValidRelation("x") {
		t.Error("relation validation")
	}
}
