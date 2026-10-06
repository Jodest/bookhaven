package bookdata

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newWikidataTestClient(t *testing.T, responder roundTripFunc) *WikidataClient {
	t.Helper()
	return NewWikidataClient(
		WithHTTPClient(&http.Client{Transport: responder}),
		withLimiter(&fakeLimiter{}),
		withSleep(func(d time.Duration) {}),
		WithMaxRetries(0),
	)
}

const wikidataQueryFixture = `{
  "head": {"vars": ["item","itemLabel","authors","genres","isbn","published","language"]},
  "results": {
    "bindings": [
      {
        "item": {"type":"uri","value":"http://www.wikidata.org/entity/Q170583"},
        "itemLabel": {"type":"literal","value":"Война и мир","xml:lang":"ru"},
        "authors": {"type":"literal","value":"Лев Толстой"},
        "genres": {"type":"literal","value":"novel|historical novel"},
        "isbn": {"type":"literal","value":"978-0-14-044793-4"},
        "published": {"type":"literal","value":"1869-01-01T00:00:00Z"},
        "language": {"type":"literal","value":"Russian"}
      },
      {
        "item": {"type":"uri","value":"http://www.wikidata.org/entity/Q161531"},
        "itemLabel": {"type":"literal","value":"1984","xml:lang":"en"},
        "authors": {"type":"literal","value":"George Orwell"},
        "published": {"type":"literal","value":"1949-06-08T00:00:00Z"}
      }
    ]
  }
}`

func TestWikidataQuery_ParsesFixture(t *testing.T) {
	var gotQuery string
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotQuery = req.URL.Query().Get("query")
		return newTestResponse(http.StatusOK, wikidataQueryFixture, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	records, err := client.Query(context.Background(), "SELECT * WHERE { ?s ?p ?o }")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if gotQuery == "" {
		t.Fatal("expected query string param to be sent")
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}

	first := records[0]
	if first.Title != "Война и мир" {
		t.Errorf("Title = %q", first.Title)
	}
	if first.SourceName != "wikidata" {
		t.Errorf("SourceName = %q, want wikidata", first.SourceName)
	}
	if first.SourceID != "Q170583" {
		t.Errorf("SourceID = %q, want Q170583", first.SourceID)
	}
	if len(first.Authors) != 1 || first.Authors[0] != "Лев Толстой" {
		t.Errorf("Authors = %v", first.Authors)
	}
	if len(first.Genres) != 2 || first.Genres[0] != "novel" || first.Genres[1] != "historical novel" {
		t.Errorf("Genres = %v", first.Genres)
	}
	if first.ISBN == nil || *first.ISBN != "9780140447934" {
		t.Errorf("ISBN = %v, want 9780140447934 (dashes stripped)", first.ISBN)
	}
	if first.PublishedYear == nil || *first.PublishedYear != 1869 {
		t.Errorf("PublishedYear = %v, want 1869", first.PublishedYear)
	}
	if first.Language != "ru" {
		t.Errorf("Language = %q, want ru", first.Language)
	}

	second := records[1]
	if second.Title != "1984" {
		t.Errorf("second.Title = %q", second.Title)
	}
	if second.ISBN != nil {
		t.Errorf("second.ISBN = %v, want nil (missing optional field)", second.ISBN)
	}
	if len(second.Genres) != 0 {
		t.Errorf("second.Genres = %v, want empty (missing optional field)", second.Genres)
	}
}

func TestWikidataFindByTitleAuthor_BuildsQueryAndParsesSingleResult(t *testing.T) {
	var capturedQuery string
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedQuery = req.URL.Query().Get("query")
		return newTestResponse(http.StatusOK, `{
  "results": {"bindings": [
    {
      "item": {"type":"uri","value":"http://www.wikidata.org/entity/Q170583"},
      "itemLabel": {"type":"literal","value":"Война и мир"},
      "authors": {"type":"literal","value":"Лев Толстой"}
    }
  ]}
}`, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	record, err := client.FindByTitleAuthor(context.Background(), "Война и мир", "Толстой")
	if err != nil {
		t.Fatalf("FindByTitleAuthor: %v", err)
	}
	if record.SourceName != "wikidata" {
		t.Errorf("SourceName = %q, want wikidata", record.SourceName)
	}
	if !strings.Contains(capturedQuery, "Война и мир") {
		t.Errorf("query does not contain escaped title: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "Толстой") {
		t.Errorf("query does not contain escaped author: %s", capturedQuery)
	}
}

func TestWikidataFindByTitleAuthor_NoResults_ReturnsErrNotFound(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `{"results":{"bindings":[]}}`, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	_, err := client.FindByTitleAuthor(context.Background(), "Nonexistent Book Title XYZ", "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestWikidataQuery_MalformedJSON_ReturnsWrappedError(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `{not valid json`, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	_, err := client.Query(context.Background(), "SELECT * WHERE { ?s ?p ?o }")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parse wikidata response") {
		t.Errorf("err = %v, want it to mention parse failure", err)
	}
}

func TestWikidataQuery_EmptyQuery_ReturnsErrInvalidQuery(t *testing.T) {
	client := newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("should not perform HTTP request for an empty query")
		return nil, nil
	}))

	_, err := client.Query(context.Background(), "   ")
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("err = %v, want ErrInvalidQuery", err)
	}
}

func TestWikidataFindByISBN_BuildsPropertyQuery(t *testing.T) {
	var capturedQuery string
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedQuery = req.URL.Query().Get("query")
		return newTestResponse(http.StatusOK, `{
  "results": {"bindings": [
    {
      "item": {"type":"uri","value":"http://www.wikidata.org/entity/Q170583"},
      "itemLabel": {"type":"literal","value":"Война и мир"},
      "isbn": {"type":"literal","value":"9780140447934"}
    }
  ]}
}`, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	record, err := client.FindByISBN(context.Background(), "978-0-14-044793-4")
	if err != nil {
		t.Fatalf("FindByISBN: %v", err)
	}
	if record.ISBN == nil || *record.ISBN != "9780140447934" {
		t.Errorf("ISBN = %v", record.ISBN)
	}
	if !strings.Contains(capturedQuery, "9780140447934") {
		t.Errorf("query does not contain normalized ISBN: %s", capturedQuery)
	}
}

func TestWikidataFindByISBN_BadRequestStatus_ReturnsErrInvalidQuery(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusBadRequest, "bad sparql", nil), nil
	})
	client := newWikidataTestClient(t, responder)

	_, err := client.FindByISBN(context.Background(), "9780140447934")
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("err = %v, want ErrInvalidQuery", err)
	}
}

func TestWikidataClient_UsesConfiguredBaseURL(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "wikidata.test" {
			t.Errorf("host = %q, want wikidata.test", req.URL.Host)
		}
		return newTestResponse(http.StatusOK, `{"results":{"bindings":[]}}`, nil), nil
	})
	client := NewWikidataClient(
		WithHTTPClient(&http.Client{Transport: responder}),
		withLimiter(&fakeLimiter{}),
		withSleep(func(d time.Duration) {}),
		WithMaxRetries(0),
		WithBaseURL("https://wikidata.test/sparql"),
	)

	_, err := client.Query(context.Background(), "SELECT * WHERE { ?s ?p ?o }")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
}

const wikidataSearchFixture = `{
  "searchinfo": {"search": "Толстой"},
  "search": [
    {"id": "Q7200", "label": "Лев Толстой", "description": "русский писатель", "match": {"type": "label", "language": "ru", "text": "Лев Толстой"}},
    {"id": "Q171976", "label": "Алексей Николаевич Толстой", "description": "русский писатель", "match": {"type": "label", "language": "ru", "text": "Алексей Николаевич Толстой"}},
    {"id": "Q368757", "description": "русский поэт", "match": {"type": "alias", "language": "ru", "text": "А. К. Толстой"}}
  ],
  "success": 1
}`

func TestWikidataSearchAuthors_ParsesFixture(t *testing.T) {
	var gotURL *url.URL
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL
		return newTestResponse(http.StatusOK, wikidataSearchFixture, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	candidates, err := client.SearchAuthors(context.Background(), "Толстой")
	if err != nil {
		t.Fatalf("SearchAuthors: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("len(candidates) = %d, want 3", len(candidates))
	}

	if gotURL.Query().Get("action") != "wbsearchentities" {
		t.Errorf("action = %q, want wbsearchentities", gotURL.Query().Get("action"))
	}
	if gotURL.Query().Get("search") != "Толстой" {
		t.Errorf("search = %q, want Толстой", gotURL.Query().Get("search"))
	}
	if gotURL.Query().Get("type") != "item" {
		t.Errorf("type = %q, want item", gotURL.Query().Get("type"))
	}
	if gotURL.Query().Get("limit") != "10" {
		t.Errorf("limit = %q, want 10", gotURL.Query().Get("limit"))
	}

	first := candidates[0]
	if first.QID != "Q7200" || first.Label != "Лев Толстой" || first.Description != "русский писатель" {
		t.Errorf("first candidate = %+v", first)
	}

	// Third result has no top-level "label" - falls back to match.text.
	third := candidates[2]
	if third.QID != "Q368757" {
		t.Errorf("third.QID = %q, want Q368757", third.QID)
	}
	if third.Label != "А. К. Толстой" {
		t.Errorf("third.Label = %q, want fallback to match.text", third.Label)
	}
}

func TestWikidataSearchAuthors_ZeroResults_ReturnsEmptySliceNoError(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `{"search": [], "success": 1}`, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	candidates, err := client.SearchAuthors(context.Background(), "Совершенно Несуществующее Имя Xyzzy")
	if err != nil {
		t.Fatalf("SearchAuthors: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %v, want empty", candidates)
	}
}

func TestWikidataSearchAuthors_EmptyQuery_ReturnsErrInvalidQuery(t *testing.T) {
	client := newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("should not perform HTTP request for an empty query")
		return nil, nil
	}))

	_, err := client.SearchAuthors(context.Background(), "   ")
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("err = %v, want ErrInvalidQuery", err)
	}
}

func TestWikidataSearchAuthors_BadRequestStatus_ReturnsErrInvalidQuery(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusBadRequest, "bad search", nil), nil
	})
	client := newWikidataTestClient(t, responder)

	_, err := client.SearchAuthors(context.Background(), "Толстой")
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("err = %v, want ErrInvalidQuery", err)
	}
}

func TestWikidataSearchAuthors_MalformedJSON_ReturnsWrappedError(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `{not valid json`, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	_, err := client.SearchAuthors(context.Background(), "Толстой")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parse wikidata search response") {
		t.Errorf("err = %v, want it to mention parse failure", err)
	}
}

func TestWikidataSearchAuthors_UsesConfiguredSearchBaseURL_IndependentOfBaseURL(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "wikidata-search.test" {
			t.Errorf("host = %q, want wikidata-search.test", req.URL.Host)
		}
		return newTestResponse(http.StatusOK, `{"search":[],"success":1}`, nil), nil
	})
	client := NewWikidataClient(
		WithHTTPClient(&http.Client{Transport: responder}),
		withLimiter(&fakeLimiter{}),
		withSleep(func(d time.Duration) {}),
		WithMaxRetries(0),
		WithBaseURL("https://wikidata.test/sparql"),
		WithSearchBaseURL("https://wikidata-search.test/w/api.php"),
	)

	_, err := client.SearchAuthors(context.Background(), "Толстой")
	if err != nil {
		t.Fatalf("SearchAuthors: %v", err)
	}
}

func TestWikidataSearchAuthors_RateLimiterAndRetryReused(t *testing.T) {
	// Confirms SearchAuthors goes through the same cfg.doRequest plumbing as
	// Query - same rate limiter Wait call, same retry-on-429 behavior - not a
	// new bespoke HTTP client.
	attempts := 0
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return newTestResponse(http.StatusTooManyRequests, "", nil), nil
		}
		return newTestResponse(http.StatusOK, `{"search":[],"success":1}`, nil), nil
	})
	limiter := &fakeLimiter{}
	client := NewWikidataClient(
		WithHTTPClient(&http.Client{Transport: responder}),
		withLimiter(limiter),
		withSleep(func(d time.Duration) {}),
		WithMaxRetries(1),
	)

	_, err := client.SearchAuthors(context.Background(), "Толстой")
	if err != nil {
		t.Fatalf("SearchAuthors: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2 (initial 429 + retry)", attempts)
	}
	if limiter.waitCalls != 2 {
		t.Fatalf("limiter.waitCalls = %d, want 2", limiter.waitCalls)
	}
}

func TestBuildAuthorListQuery(t *testing.T) {
	t.Run("valid QIDs produce a query with VALUES/class-filter/label-filter/LIMIT", func(t *testing.T) {
		query, err := BuildAuthorListQuery([]string{"Q7200", "Q1183296"}, 250)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, want := range []string{
			"VALUES ?author { wd:Q7200 wd:Q1183296 }",
			"wdt:P31/wdt:P279* wd:Q7725634",
			"?item wdt:P50 ?author",
			`FILTER(LANG(?itemLabel) = "ru")`,
			`FILTER(LANG(?authorLabel) = "ru")`,
			"LIMIT 250",
		} {
			if !strings.Contains(query, want) {
				t.Errorf("query does not contain %q:\n%s", want, query)
			}
		}
	})

	t.Run("dedupes repeated QIDs", func(t *testing.T) {
		query, err := BuildAuthorListQuery([]string{"Q7200", "Q7200", " Q7200 "}, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Count(query, "wd:Q7200") != 1 {
			t.Errorf("expected deduped single wd:Q7200, got query:\n%s", query)
		}
	})

	t.Run("invalid QID format errors", func(t *testing.T) {
		for _, bad := range []string{"q7200", "Q", "Q12a", "7200", "wd:Q7200", ""} {
			if bad == "" {
				continue // empty entries are skipped, not errors, unless the whole list is empty
			}
			_, err := BuildAuthorListQuery([]string{bad}, 10)
			if !errors.Is(err, ErrInvalidQuery) {
				t.Errorf("BuildAuthorListQuery(%q) err = %v, want ErrInvalidQuery", bad, err)
			}
		}
	})

	t.Run("empty list errors", func(t *testing.T) {
		_, err := BuildAuthorListQuery(nil, 10)
		if !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("err = %v, want ErrInvalidQuery", err)
		}
		_, err = BuildAuthorListQuery([]string{"  ", ""}, 10)
		if !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("err = %v, want ErrInvalidQuery for blank-only list", err)
		}
	})

	t.Run("limit <= 0 applies the default", func(t *testing.T) {
		query, err := BuildAuthorListQuery([]string{"Q7200"}, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(query, "LIMIT 500") {
			t.Errorf("expected default LIMIT 500, got query:\n%s", query)
		}

		query, err = BuildAuthorListQuery([]string{"Q7200"}, -5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(query, "LIMIT 500") {
			t.Errorf("expected default LIMIT 500 for negative input, got query:\n%s", query)
		}
	})

	t.Run("limit above the upper bound is clamped, not rejected", func(t *testing.T) {
		query, err := BuildAuthorListQuery([]string{"Q7200"}, 999999)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(query, "LIMIT 5000") {
			t.Errorf("expected clamped LIMIT 5000, got query:\n%s", query)
		}
	})
}

func TestParseAuthorPairs(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []ExternalAuthor
	}{
		{
			name:  "multiple paired authors",
			value: "Q7243\tЛев Толстой|Q42\tДуглас Адамс",
			want:  []ExternalAuthor{{Name: "Лев Толстой", QID: "Q7243"}, {Name: "Дуглас Адамс", QID: "Q42"}},
		},
		{
			name:  "label with spaces and punctuation stays paired",
			value: "Q1\tА. С. Пушкин, мл. (поэт)|Q2\tБ-Б",
			want:  []ExternalAuthor{{Name: "А. С. Пушкин, мл. (поэт)", QID: "Q1"}, {Name: "Б-Б", QID: "Q2"}},
		},
		{
			name:  "missing QID keeps name",
			value: "\tБез Кода|Просто Имя",
			want:  []ExternalAuthor{{Name: "Без Кода"}, {Name: "Просто Имя"}},
		},
		{
			name:  "invalid QID dropped, name kept",
			value: "http://x/Q1\tИмя|q5\tДругое",
			want:  []ExternalAuthor{{Name: "Имя"}, {Name: "Другое"}},
		},
		{
			name:  "empty names skipped",
			value: "Q1\t|Q2\t  |Q3\tЕсть",
			want:  []ExternalAuthor{{Name: "Есть", QID: "Q3"}},
		},
		{
			name:  "empty aggregate",
			value: "",
			want:  []ExternalAuthor{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAuthorPairs(tt.value)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestWikidataQuery_PairedAuthors_PopulatesAuthorsAndRefs(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `{
  "results": {"bindings": [
    {
      "item": {"type":"uri","value":"http://www.wikidata.org/entity/Q1"},
      "itemLabel": {"type":"literal","value":"Двенадцать стульев"},
      "authors": {"type":"literal","value":"Q19365\tИльф, Илья|Q312517\tЕвгений Петров"}
    },
    {
      "item": {"type":"uri","value":"http://www.wikidata.org/entity/Q2"},
      "itemLabel": {"type":"literal","value":"Без авторов"},
      "authors": {"type":"literal","value":""}
    }
  ]}
}`, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	records, err := client.Query(context.Background(), "SELECT * WHERE { ?s ?p ?o }")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	first := records[0]
	wantRefs := []ExternalAuthor{{Name: "Ильф, Илья", QID: "Q19365"}, {Name: "Евгений Петров", QID: "Q312517"}}
	if len(first.AuthorRefs) != 2 || first.AuthorRefs[0] != wantRefs[0] || first.AuthorRefs[1] != wantRefs[1] {
		t.Errorf("AuthorRefs = %+v, want %+v", first.AuthorRefs, wantRefs)
	}
	if len(first.Authors) != 2 || first.Authors[0] != "Ильф, Илья" || first.Authors[1] != "Евгений Петров" {
		t.Errorf("Authors = %v, want names of AuthorRefs", first.Authors)
	}
	if len(records[1].Authors) != 0 || len(records[1].AuthorRefs) != 0 {
		t.Errorf("empty aggregate: Authors = %v, AuthorRefs = %v", records[1].Authors, records[1].AuthorRefs)
	}
}

func TestQueryBuilders_UsePairedAuthorAggregate(t *testing.T) {
	const paired = `GROUP_CONCAT(DISTINCT CONCAT(STRAFTER(STR(?author), "/entity/"), "\t", ?authorLabel); separator="|") AS ?authors`
	listQuery, err := BuildAuthorListQuery([]string{"Q7243"}, 10)
	if err != nil {
		t.Fatalf("BuildAuthorListQuery: %v", err)
	}
	queries := map[string]string{
		"title+author": buildTitleAuthorQuery("Война и мир", "Толстой"),
		"title only":   buildTitleAuthorQuery("Война и мир", ""),
		"isbn":         buildISBNQuery("9780140447934"),
		"author list":  listQuery,
	}
	for name, q := range queries {
		if !strings.Contains(q, paired) {
			t.Errorf("%s query lacks paired author aggregate:\n%s", name, q)
		}
	}
}
