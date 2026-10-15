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

func filmRow(author, work string, fields map[string]string) string {
	parts := []string{
		`"author":{"type":"uri","value":"http://www.wikidata.org/entity/` + author + `"}`,
		`"work":{"type":"uri","value":"http://www.wikidata.org/entity/` + work + `"}`,
	}
	for k, v := range fields {
		parts = append(parts, fmt.Sprintf("%q:%s", k, lit(v)))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func TestClassifyAuthorFilm(t *testing.T) {
	cases := []struct {
		name            string
		classes, genres []string
		kind, relation  string
		ok              bool
	}{
		{"film with biographical genre", []string{"Q11424"}, []string{"Q645928"}, ScreenKindFilm, AuthorFilmRelationBiopic, true},
		{"tv film biopic", []string{"Q506240"}, []string{"Q645928"}, ScreenKindTVFilm, AuthorFilmRelationBiopic, true},
		{"film without genre", []string{"Q11424"}, nil, ScreenKindFilm, AuthorFilmRelationBiopic, true},
		{"documentary class", []string{"Q93204"}, nil, ScreenKindFilm, AuthorFilmRelationDocumentary, true},
		{"documentary genre", []string{"Q11424"}, []string{"Q93204"}, ScreenKindFilm, AuthorFilmRelationDocumentary, true},
		{"biographical film class", []string{"Q645928"}, nil, ScreenKindFilm, AuthorFilmRelationBiopic, true},
		{"documentary tv film", []string{"Q133731263"}, nil, ScreenKindTVFilm, AuthorFilmRelationDocumentary, true},
		{"documentary tv series", []string{"Q133731271"}, nil, ScreenKindSeries, AuthorFilmRelationDocumentary, true},
		{"series keeps priority over film", []string{"Q5398426", "Q93204"}, nil, ScreenKindSeries, AuthorFilmRelationDocumentary, true},
		{"episode excluded", []string{"Q21191270", "Q133731271"}, nil, "", "", false},
		{"film series excluded", []string{"Q24856", "Q11424"}, []string{"Q93204"}, "", "", false},
		{"list article excluded", []string{"Q13406463"}, nil, "", "", false},
		{"book is not a film", []string{"Q571"}, nil, "", "", false},
	}
	for _, tc := range cases {
		kind, rel, ok := classifyAuthorFilm(tc.classes, tc.genres)
		if kind != tc.kind || rel != tc.relation || ok != tc.ok {
			t.Errorf("%s: got (%q,%q,%v), want (%q,%q,%v)", tc.name, kind, rel, ok, tc.kind, tc.relation, tc.ok)
		}
	}
}

func TestFetchAuthorFilms_QueryAndParse(t *testing.T) {
	body := adaptResponse(
		filmRow("Q892", "Q42689885", map[string]string{"classes": "Q11424", "genres": "Q645928|Q130232", "enLabel": "Tolkien", "year": "2019", "imdbIds": "tt3416828", "directors": "Доме Каруко"}),
		filmRow("Q892", "Q42689885", map[string]string{"classes": "Q11424", "enLabel": "Tolkien", "year": "2020"}),
		filmRow("Q892", "Q500", map[string]string{"classes": "Q11424", "genres": "Q93204", "ruLabel": "Документалка", "year": "2001"}),
		filmRow("Q892", "Q501", map[string]string{"classes": "Q21191270|Q133731271", "ruLabel": "Эпизод"}),
		filmRow("Q892", "Q502", map[string]string{"classes": "Q11424", "ruLabel": "  "}),
		filmRow("Q34660", "Q1343175", map[string]string{"classes": "Q506240", "genres": "Q645928", "enLabel": "Magic Beyond Words", "origTitle": "Magic Beyond Words: The JK Rowling Story", "year": "2011"}),
	)
	var calls int
	var gotURL *url.URL
	got, err := adaptClient(t, body, &calls, &gotURL).FetchAuthorFilms(context.Background(), []string{"Q892", "Q34660"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
	q := gotURL.Query().Get("query")
	for _, want := range []string{"wd:Q892 wd:Q34660", "wdt:P921 ?author", "FILTER EXISTS", "VALUES ?allowed", "wd:Q11424", "wd:Q93204", "wd:Q645928", "wd:Q133731271", "wdt:P136"} {
		if !strings.Contains(q, want) {
			t.Errorf("query lacks %q", want)
		}
	}
	tolkien := got["Q892"]
	if len(tolkien) != 2 {
		t.Fatalf("tolkien = %+v", tolkien)
	}
	by := map[string]ExternalAuthorFilm{}
	for _, f := range tolkien {
		by[f.QID] = f
	}
	if f := by["Q42689885"]; f.Kind != ScreenKindFilm || f.Relation != AuthorFilmRelationBiopic || f.Year == nil || *f.Year != 2019 || f.IMDbID != "tt3416828" || f.Director != "Доме Каруко" {
		t.Errorf("Q42689885 = %+v", f)
	}
	if f := by["Q500"]; f.Relation != AuthorFilmRelationDocumentary || f.Title != "Документалка" {
		t.Errorf("Q500 = %+v", f)
	}
	rowling := got["Q34660"]
	if len(rowling) != 1 || rowling[0].Kind != ScreenKindTVFilm || rowling[0].Relation != AuthorFilmRelationBiopic ||
		rowling[0].Title != "Magic Beyond Words" || rowling[0].OriginalTitle != "Magic Beyond Words: The JK Rowling Story" {
		t.Errorf("rowling = %+v", rowling)
	}
}

func TestFetchAuthorFilms_InvalidInputNoCall(t *testing.T) {
	big := make([]string, 51)
	for i := range big {
		big[i] = fmt.Sprintf("Q%d", i+1)
	}
	for name, qids := range map[string][]string{
		"empty": nil, "bad qid": {"Q1", "wd:Q2"}, "too many": big,
	} {
		var calls int
		_, err := adaptClient(t, adaptResponse(), &calls, nil).FetchAuthorFilms(context.Background(), qids)
		if !errors.Is(err, ErrInvalidQuery) || calls != 0 {
			t.Errorf("%s: err=%v calls=%d", name, err, calls)
		}
	}
}

func TestFetchAuthorFilms_ServerError(t *testing.T) {
	client := newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusInternalServerError, "boom", nil), nil
	}))
	if _, err := client.FetchAuthorFilms(context.Background(), []string{"Q1"}); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("err = %v", err)
	}
}
