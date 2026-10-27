package bookdata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func lit(v string) string { return fmt.Sprintf(`{"type":"literal","value":%q}`, v) }

// adaptRow builds one SPARQL result row; fields maps variable -> literal.
func adaptRow(book, work string, fields map[string]string) string {
	parts := []string{
		`"book":{"type":"uri","value":"http://www.wikidata.org/entity/` + book + `"}`,
		`"work":{"type":"uri","value":"http://www.wikidata.org/entity/` + work + `"}`,
	}
	for k, v := range fields {
		parts = append(parts, fmt.Sprintf("%q:%s", k, lit(v)))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func adaptResponse(rows ...string) string {
	return `{"results":{"bindings":[` + strings.Join(rows, ",") + `]}}`
}

func adaptClient(t *testing.T, body string, calls *int, gotURL **url.URL) *WikidataClient {
	t.Helper()
	return newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if calls != nil {
			*calls++
		}
		if gotURL != nil {
			*gotURL = req.URL
		}
		return newTestResponse(http.StatusOK, body, nil), nil
	}))
}

func TestClassifyAdaptation(t *testing.T) {
	cases := []struct {
		name    string
		classes []string
		kind    string
		ok      bool
	}{
		{"film", []string{"Q11424"}, ScreenKindFilm, true},
		{"feature film", []string{"Q24869"}, ScreenKindFilm, true},
		{"tv film", []string{"Q506240"}, ScreenKindTVFilm, true},
		{"series", []string{"Q5398426"}, ScreenKindSeries, true},
		{"miniseries", []string{"Q1259759"}, ScreenKindSeries, true},
		{"animated film", []string{"Q202866"}, ScreenKindAnimatedFilm, true},
		{"animated series", []string{"Q581714"}, ScreenKindAnimatedSeries, true},
		{"animated tv series", []string{"Q117467246", "Q5398426"}, ScreenKindAnimatedSeries, true},
		{"teleplay beats tv film", []string{"Q506240", "Q356055"}, ScreenKindTeleplay, true},
		{"film series excluded", []string{"Q24856"}, "", false},
		{"film and film series excluded", []string{"Q11424", "Q24856"}, "", false},
		{"video game excluded", []string{"Q7889"}, "", false},
		{"board game excluded", []string{"Q131436"}, "", false},
		{"list article excluded", []string{"Q13406463"}, "", false},
		{"episode excluded", []string{"Q21191270"}, "", false},
		{"series of works excluded", []string{"Q7725310", "Q11424"}, "", false},
		{"unknown class", []string{"Q93204"}, "", false},
		{"empty", nil, "", false},
	}
	for _, tc := range cases {
		kind, ok := classifyAdaptation(tc.classes)
		if kind != tc.kind || ok != tc.ok {
			t.Errorf("%s: got (%q,%v), want (%q,%v)", tc.name, kind, ok, tc.kind, tc.ok)
		}
	}
}

func TestFetchAdaptations_FiltersAndParses(t *testing.T) {
	body := adaptResponse(
		adaptRow("Q74287", "Q74229", map[string]string{"classes": "Q11424", "ruLabel": "Хоббит", "year": "1977", "imdbIds": "tt0077689", "kpIds": "12345", "directors": "Жюль Басс|Арти Ранкин"}),
		adaptRow("Q74287", "Q74331", map[string]string{"classes": "Q24856", "ruLabel": "Хоббит (серия фильмов)"}),
		adaptRow("Q74287", "Q15709637", map[string]string{"classes": "Q7889", "ruLabel": "The Hobbit (игра)"}),
		adaptRow("Q74287", "Q2824095", map[string]string{"classes": "Q13406463", "ruLabel": "Экранизации"}),
		adaptRow("Q74287", "Q9999991", map[string]string{"classes": "Q11424|Q24856", "ruLabel": "Both"}),
		adaptRow("Q74287", "Q1187415", map[string]string{"classes": "Q506240|Q356055", "ruLabel": "Хоббит", "year": "1985"}),
		adaptRow("Q74287", "Q9999992", map[string]string{"classes": "Q581714", "ruLabel": "Мультсериал", "year": "1990"}),
		adaptRow("Q1", "Q9999993", map[string]string{"classes": "Q11424", "enLabel": "The Master and Margarita", "year": "1972"}),
	)
	var calls int
	var gotURL *url.URL
	got, err := adaptClient(t, body, &calls, &gotURL).FetchAdaptations(context.Background(), []string{"Q74287", "Q1"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if q := gotURL.Query().Get("query"); !strings.Contains(q, "wd:Q74287 wd:Q1") || !strings.Contains(q, "wdt:P144") {
		t.Fatalf("query = %s", q)
	}
	hobbit := got["Q74287"]
	if len(hobbit) != 3 {
		t.Fatalf("hobbit = %+v", hobbit)
	}
	byQID := map[string]ExternalScreenWork{}
	for _, w := range hobbit {
		byQID[w.QID] = w
	}
	film := byQID["Q74229"]
	if film.Kind != ScreenKindFilm || film.Title != "Хоббит" || film.Year == nil || *film.Year != 1977 ||
		film.IMDbID != "tt0077689" || film.KinopoiskID != "12345" || film.Director != "Арти Ранкин, Жюль Басс" {
		t.Fatalf("film = %+v", film)
	}
	if byQID["Q1187415"].Kind != ScreenKindTeleplay {
		t.Fatalf("teleplay = %+v", byQID["Q1187415"])
	}
	if byQID["Q9999992"].Kind != ScreenKindAnimatedSeries {
		t.Fatalf("animated series = %+v", byQID["Q9999992"])
	}
	for _, excluded := range []string{"Q74331", "Q15709637", "Q2824095", "Q9999991"} {
		if _, ok := byQID[excluded]; ok {
			t.Errorf("%s must be excluded", excluded)
		}
	}
	if w := got["Q1"]; len(w) != 1 || w[0].Title != "The Master and Margarita" {
		t.Fatalf("english fallback = %+v", w)
	}
}

func TestFetchAdaptations_TitleFallbackAndOriginal(t *testing.T) {
	body := adaptResponse(
		adaptRow("Q1", "Q10", map[string]string{"classes": "Q11424", "origTitle": "Original"}),
		adaptRow("Q1", "Q11", map[string]string{"classes": "Q11424", "ruLabel": "Русское", "origTitle": "Original"}),
		adaptRow("Q1", "Q12", map[string]string{"classes": "Q11424", "ruLabel": "   "}),
		adaptRow("Q1", "Q13", map[string]string{"classes": "Q11424", "enLabel": "Same", "origTitle": "Same"}),
	)
	got, err := adaptClient(t, body, nil, nil).FetchAdaptations(context.Background(), []string{"Q1"})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]ExternalScreenWork{}
	for _, w := range got["Q1"] {
		by[w.QID] = w
	}
	if len(by) != 3 {
		t.Fatalf("works = %+v", by)
	}
	if w := by["Q10"]; w.Title != "Original" || w.OriginalTitle != "" {
		t.Errorf("Q10 = %+v", w)
	}
	if w := by["Q11"]; w.Title != "Русское" || w.OriginalTitle != "Original" {
		t.Errorf("Q11 = %+v", w)
	}
	if _, ok := by["Q12"]; ok {
		t.Error("work without a title must be dropped")
	}
	if w := by["Q13"]; w.OriginalTitle != "" {
		t.Errorf("Q13 = %+v", w)
	}
}

func TestFetchAdaptations_YearDedupeAndIDValidation(t *testing.T) {
	body := adaptResponse(
		adaptRow("Q1", "Q10", map[string]string{"classes": "Q11424", "ruLabel": "A", "year": "1967", "imdbIds": "bad|tt123", "kpIds": "abc|777"}),
		adaptRow("Q1", "Q10", map[string]string{"classes": "Q11424", "ruLabel": "A", "year": "1972"}),
		adaptRow("Q1", "Q11", map[string]string{"classes": "Q11424", "ruLabel": "B", "year": "1500", "imdbIds": "nm123", "kpIds": "12a"}),
		adaptRow("Q1", "Q12", map[string]string{"classes": "Q11424", "ruLabel": "C", "directors": "D|C|B|A|A"}),
	)
	got, err := adaptClient(t, body, nil, nil).FetchAdaptations(context.Background(), []string{"Q1"})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]ExternalScreenWork{}
	for _, w := range got["Q1"] {
		by[w.QID] = w
	}
	if len(got["Q1"]) != 3 {
		t.Fatalf("works = %+v", got["Q1"])
	}
	if w := by["Q10"]; w.Year == nil || *w.Year != 1967 || w.IMDbID != "tt123" || w.KinopoiskID != "777" {
		t.Errorf("Q10 = %+v", w)
	}
	if w := by["Q11"]; w.Year != nil || w.IMDbID != "" || w.KinopoiskID != "" {
		t.Errorf("Q11 = %+v", w)
	}
	if w := by["Q12"]; w.Director != "A, B, C" {
		t.Errorf("Q12 director = %q", w.Director)
	}
}

func TestFetchAdaptations_InvalidInputNoCall(t *testing.T) {
	big := make([]string, 51)
	for i := range big {
		big[i] = fmt.Sprintf("Q%d", i+1)
	}
	for name, qids := range map[string][]string{
		"empty": nil, "bad qid": {"Q1", "wd:Q2"}, "lowercase": {"q1"}, "too many": big,
	} {
		var calls int
		_, err := adaptClient(t, adaptResponse(), &calls, nil).FetchAdaptations(context.Background(), qids)
		if !errors.Is(err, ErrInvalidQuery) || calls != 0 {
			t.Errorf("%s: err=%v calls=%d", name, err, calls)
		}
	}
}

func TestFetchAdaptations_ServerError(t *testing.T) {
	client := newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusInternalServerError, "boom", nil), nil
	}))
	_, err := client.FetchAdaptations(context.Background(), []string{"Q1"})
	if !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestCommonsFileName(t *testing.T) {
	cases := map[string]string{
		"http://commons.wikimedia.org/wiki/Special:FilePath/Hobbit%20poster.jpg":                "Hobbit poster.jpg",
		"http://commons.wikimedia.org/wiki/Special:FilePath/%D0%A4%D0%B8%D0%BB%D1%8C%D0%BC.png": "Фильм.png",
		"http://commons.wikimedia.org/wiki/Special:FilePath/":                                   "",
		"http://commons.wikimedia.org/wiki/Special:FilePath/a%7Cb.jpg":                          "",
		"http://commons.wikimedia.org/wiki/Special:FilePath/%zz":                                "",
		"http://example.com/x.jpg":                                                              "",
		"":                                                                                      "",
	}
	for in, want := range cases {
		if got := commonsFileName(in); got != want {
			t.Errorf("commonsFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFetchAdaptations_PosterFields(t *testing.T) {
	const fp = "http://commons.wikimedia.org/wiki/Special:FilePath/"
	body := adaptResponse(
		adaptRow("Q1", "Q10", map[string]string{
			"classes": "Q11424", "ruLabel": "A",
			"posterFile": fp + "Poster%20A.jpg", "imageFile": fp + "Still.jpg",
			"tmdbMovie": "603", "tmdbTv": "1399",
		}),
		adaptRow("Q1", "Q11", map[string]string{
			"classes": "Q11424", "ruLabel": "B", "tmdbMovie": "12a", "tmdbTv": "12345678901",
		}),
	)
	got, err := adaptClient(t, body, nil, nil).FetchAdaptations(context.Background(), []string{"Q1"})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]ExternalScreenWork{}
	for _, w := range got["Q1"] {
		by[w.QID] = w
	}
	if w := by["Q10"]; w.PosterFile != "Poster A.jpg" || w.ImageFile != "Still.jpg" || w.TMDBMovieID != "603" || w.TMDBTVID != "1399" {
		t.Errorf("Q10 = %+v", w)
	}
	if w := by["Q11"]; w.PosterFile != "" || w.TMDBMovieID != "" || w.TMDBTVID != "" {
		t.Errorf("Q11 = %+v", w)
	}
	q := buildAdaptationsQuery([]string{"Q1"})
	for _, p := range []string{"P3383", "P18", "P4947", "P4983"} {
		if !strings.Contains(q, p) {
			t.Errorf("query lacks %s", p)
		}
	}
}
