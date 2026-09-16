package bookdata

import (
	"context"
	"errors"
	"net/http"
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
