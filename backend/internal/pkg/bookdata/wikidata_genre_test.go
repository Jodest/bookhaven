package bookdata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func newGenreTestClient(responder roundTripFunc) *WikidataClient {
	return NewWikidataClient(
		WithHTTPClient(&http.Client{Transport: responder}),
		withLimiter(&fakeLimiter{}),
		withSleep(func(d time.Duration) {}),
		WithMaxRetries(0),
		WithUserAgent("TestAgent/1.0"),
		WithBaseURL("https://wikidata.test/sparql"),
		WithSearchBaseURL("https://wikidata-search.test/w/api.php"),
	)
}

func TestWikidataSearchGenres_ParamsAndParsing(t *testing.T) {
	var gotURL *url.URL
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL
		return newTestResponse(http.StatusOK, `{"search":[
			{"id":"Q132311","label":"фэнтези","description":"жанр художественной литературы"},
			{"id":"Q1","match":{"text":"фэнтези (фильм)"}}
		],"success":1}`, nil), nil
	})
	client := newGenreTestClient(responder)

	got, err := client.SearchGenres(context.Background(), "  фэнтези ")
	if err != nil {
		t.Fatalf("SearchGenres: %v", err)
	}
	if gotURL.Host != "wikidata-search.test" {
		t.Errorf("host = %q", gotURL.Host)
	}
	q := gotURL.Query()
	want := map[string]string{"action": "wbsearchentities", "search": "фэнтези", "language": "ru", "uselang": "ru", "type": "item", "limit": "20"}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	wantCands := []GenreCandidate{
		{QID: "Q132311", Label: "фэнтези", Description: "жанр художественной литературы"},
		{QID: "Q1", Label: "фэнтези (фильм)"},
	}
	if !reflect.DeepEqual(got, wantCands) {
		t.Fatalf("candidates = %+v", got)
	}
}

func TestWikidataSearchGenres_EmptyQuery_NoCall(t *testing.T) {
	calls := 0
	client := newGenreTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return newTestResponse(http.StatusOK, `{"search":[]}`, nil), nil
	}))
	if _, err := client.SearchGenres(context.Background(), "   "); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("err = %v, want ErrInvalidQuery", err)
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want 0", calls)
	}
}

func boolBinding(v bool) string {
	return fmt.Sprintf(`{"datatype":"http://www.w3.org/2001/XMLSchema#boolean","type":"literal","value":"%t"}`, v)
}

func classificationRow(qid, ruLabel string, byClass, byFiction, nonLiterary bool) string {
	label := ""
	if ruLabel != "" {
		label = fmt.Sprintf(`"ruLabel":{"type":"literal","value":%q,"xml:lang":"ru"},`, ruLabel)
	}
	return fmt.Sprintf(`{"item":{"type":"uri","value":"http://www.wikidata.org/entity/%s"},%s"byClass":%s,"byFiction":%s,"nonLiterary":%s}`,
		qid, label, boolBinding(byClass), boolBinding(byFiction), boolBinding(nonLiterary))
}

func TestWikidataClassifyGenres_QueryAndVerdicts(t *testing.T) {
	var gotQuery, gotUA, gotHost string
	rows := []string{
		classificationRow("Q8261", "роман", true, false, false),       // literary root
		classificationRow("Q186424", "детектив", false, true, false),  // fiction genre
		classificationRow("Q11399", "рок-музыка", false, false, true), // music genre
		classificationRow("Q130232", "драма", false, true, true),      // fiction but film genre
		classificationRow("Q40831", "комедия", true, false, true),     // literary root wins
		classificationRow("Q5", "", false, false, false),              // nothing, no ru label
		classificationRow("Q999", "чужой", true, false, false),        // not requested: ignored
	}
	body := `{"head":{"vars":["item","ruLabel","byClass","byFiction","nonLiterary"]},"results":{"bindings":[` + strings.Join(rows, ",") + `]}}`
	client := newGenreTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotQuery = req.URL.Query().Get("query")
		gotUA = req.Header.Get("User-Agent")
		gotHost = req.URL.Host
		return newTestResponse(http.StatusOK, body, nil), nil
	}))

	qids := []string{"Q8261", "Q186424", "Q11399", "Q130232", "Q40831", "Q5", "Q77", "Q8261"}
	got, err := client.ClassifyGenres(context.Background(), qids)
	if err != nil {
		t.Fatalf("ClassifyGenres: %v", err)
	}
	if gotHost != "wikidata.test" || gotUA != "TestAgent/1.0" {
		t.Errorf("host = %q, User-Agent = %q", gotHost, gotUA)
	}

	if !strings.Contains(gotQuery, "VALUES ?item { wd:Q8261 wd:Q186424 wd:Q11399 wd:Q130232 wd:Q40831 wd:Q5 wd:Q77 }") {
		t.Errorf("VALUES must hold exactly the (deduplicated) QIDs:\n%s", gotQuery)
	}
	for _, part := range []string{
		`FILTER(?root IN (wd:Q223393, wd:Q4263830))`,
		`FILTER(?fiction = wd:Q108465955)`,
		`FILTER(?other IN (wd:Q188451, wd:Q201658))`,
		`FILTER(LANG(?ruLabel) = "ru")`,
	} {
		if !strings.Contains(gotQuery, part) {
			t.Errorf("query lacks %q:\n%s", part, gotQuery)
		}
	}
	if n := strings.Count(gotQuery, "BIND(EXISTS { ?item (wdt:P31|wdt:P279)/wdt:P279* "); n != 3 {
		t.Errorf("expected 3 upward class walks from ?item, got %d:\n%s", n, gotQuery)
	}
	if regexp.MustCompile(`wdt:P279\*\s+wd:Q`).MatchString(gotQuery) || strings.Contains(gotQuery, "Q483394") {
		t.Errorf("query must not walk a root's subtree nor use genre (Q483394):\n%s", gotQuery)
	}

	want := map[string]GenreClassification{
		"Q8261":   {RuLabel: "роман", IsLiteraryGenre: true},
		"Q186424": {RuLabel: "детектив", IsLiteraryGenre: true},
		"Q11399":  {RuLabel: "рок-музыка", IsLiteraryGenre: false},
		"Q130232": {RuLabel: "драма", IsLiteraryGenre: false},
		"Q40831":  {RuLabel: "комедия", IsLiteraryGenre: true},
		"Q5":      {RuLabel: "", IsLiteraryGenre: false},
		"Q77":     {}, // missing from the bindings
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("classification = %+v\nwant %+v", got, want)
	}
}

func TestWikidataClassifyGenres_InvalidInput_NoCall(t *testing.T) {
	calls := 0
	client := newGenreTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return newTestResponse(http.StatusOK, `{"results":{"bindings":[]}}`, nil), nil
	}))
	tooMany := make([]string, MaxGenresPerClassification+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("Q%d", i+1)
	}
	for name, qids := range map[string][]string{
		"empty":    nil,
		"too many": tooMany,
		"invalid":  {"Q1", "Q2 } . ?x ?y ?z . {"},
		"lower":    {"q1"},
	} {
		if _, err := client.ClassifyGenres(context.Background(), qids); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("%s: err = %v, want ErrInvalidQuery", name, err)
		}
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want 0", calls)
	}
}

func TestWikidataClassifyGenres_Errors(t *testing.T) {
	t.Run("5xx", func(t *testing.T) {
		client := newGenreTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return newTestResponse(http.StatusServiceUnavailable, "", nil), nil
		}))
		if _, err := client.ClassifyGenres(context.Background(), []string{"Q1"}); !errors.Is(err, ErrSourceUnavailable) {
			t.Fatalf("err = %v, want ErrSourceUnavailable", err)
		}
	})
	t.Run("malformed JSON", func(t *testing.T) {
		client := newGenreTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return newTestResponse(http.StatusOK, `{not json`, nil), nil
		}))
		_, err := client.ClassifyGenres(context.Background(), []string{"Q1"})
		if err == nil || !strings.Contains(err.Error(), "parse wikidata response") {
			t.Fatalf("err = %v, want a wrapped parse error", err)
		}
	})
}

const wikidataGenreEntityFixture = `{
  "entities": {
    "Q132311": {
      "id": "Q132311",
      "labels": {"ru": {"language": "ru", "value": "фэнтези"}},
      "claims": {
        "P31": [
          {"mainsnak": {"snaktype": "value", "property": "P31", "datavalue": {"value": {"entity-type": "item", "id": "Q20076756"}, "type": "wikibase-entityid"}}},
          {"mainsnak": {"snaktype": "novalue", "property": "P31"}}
        ],
        "P279": [
          {"mainsnak": {"snaktype": "value", "property": "P279", "datavalue": {"value": {"entity-type": "item", "id": "Q1057172"}, "type": "wikibase-entityid"}}},
          {"mainsnak": {"snaktype": "somevalue", "property": "P279"}},
          {"mainsnak": {"snaktype": "value", "property": "P279", "datavalue": {"value": {"entity-type": "item", "id": "Q223393"}, "type": "wikibase-entityid"}}}
        ]
      }
    }
  },
  "success": 1
}`

func TestWikidataFetchGenre_ParsesLabelAndClaims(t *testing.T) {
	var gotURL *url.URL
	client := newGenreTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL
		return newTestResponse(http.StatusOK, wikidataGenreEntityFixture, nil), nil
	}))

	rec, err := client.FetchGenre(context.Background(), " Q132311 ")
	if err != nil {
		t.Fatalf("FetchGenre: %v", err)
	}
	q := gotURL.Query()
	if gotURL.Host != "wikidata-search.test" || q.Get("action") != "wbgetentities" || q.Get("ids") != "Q132311" ||
		q.Get("props") != "labels|claims" || q.Get("languages") != "ru" {
		t.Errorf("unexpected request %s", gotURL)
	}
	want := &ExternalGenreRecord{QID: "Q132311", Name: "фэнтези", InstanceOf: []string{"Q20076756"}, SubclassOf: []string{"Q1057172", "Q223393"}}
	if !reflect.DeepEqual(rec, want) {
		t.Fatalf("record = %+v, want %+v", rec, want)
	}
}

func TestWikidataFetchGenre_RedirectMissingInvalid(t *testing.T) {
	t.Run("redirect", func(t *testing.T) {
		client := newGenreTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return newTestResponse(http.StatusOK, `{"entities":{"Q100":{"id":"Q200","labels":{},"claims":{}}},"success":1}`, nil), nil
		}))
		rec, err := client.FetchGenre(context.Background(), "Q100")
		if err != nil || rec.QID != "Q200" || rec.Name != "" {
			t.Fatalf("rec = %+v err = %v, want canonical Q200 without a name", rec, err)
		}
	})
	t.Run("missing", func(t *testing.T) {
		for _, body := range []string{
			`{"entities":{"Q9":{"id":"Q9","missing":""}},"success":1}`,
			`{"error":{"code":"no-such-entity","info":"x"}}`,
		} {
			client := newGenreTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return newTestResponse(http.StatusOK, body, nil), nil
			}))
			if _, err := client.FetchGenre(context.Background(), "Q9"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("%s: err = %v, want ErrNotFound", body, err)
			}
		}
	})
	t.Run("invalid", func(t *testing.T) {
		calls := 0
		client := newGenreTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			return newTestResponse(http.StatusOK, `{}`, nil), nil
		}))
		for _, qid := range []string{"", "fantasy", "Q1|Q2"} {
			if _, err := client.FetchGenre(context.Background(), qid); !errors.Is(err, ErrInvalidQuery) {
				t.Fatalf("%q: err = %v, want ErrInvalidQuery", qid, err)
			}
		}
		if calls != 0 {
			t.Fatalf("calls = %d, want 0", calls)
		}
	})
}
