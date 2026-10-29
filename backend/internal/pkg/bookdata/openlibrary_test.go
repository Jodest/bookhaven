package bookdata

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func newOpenLibraryTestClient(t *testing.T, responder roundTripFunc) *OpenLibraryClient {
	t.Helper()
	return NewOpenLibraryClient(
		WithHTTPClient(&http.Client{Transport: responder}),
		withLimiter(&fakeLimiter{}),
		withSleep(func(d time.Duration) {}),
		WithMaxRetries(0),
	)
}

const openLibraryBooksFixture = `{
  "ISBN:9780140447934": {
    "title": "War and Peace",
    "authors": [{"name": "Leo Tolstoy"}],
    "subjects": [{"name": "Russia"}, {"name": "War"}],
    "publish_date": "2005",
    "number_of_pages": 1392,
    "cover": {
      "small": "https://covers.openlibrary.org/b/id/12621906-S.jpg",
      "medium": "https://covers.openlibrary.org/b/id/12621906-M.jpg",
      "large": "https://covers.openlibrary.org/b/id/12621906-L.jpg"
    },
    "identifiers": {
      "isbn_13": ["9780140447934"]
    },
    "excerpts": [{"text": "Well, Prince, so Genoa and Lucca are now just family estates..."}],
    "key": "/books/OL7353617M"
  }
}`

func TestOpenLibraryFindByISBN_ParsesFixture(t *testing.T) {
	var capturedQuery string
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedQuery = req.URL.RawQuery
		return newTestResponse(http.StatusOK, openLibraryBooksFixture, nil), nil
	})
	client := newOpenLibraryTestClient(t, responder)

	record, err := client.FindByISBN(context.Background(), "978-0-14-044793-4")
	if err != nil {
		t.Fatalf("FindByISBN: %v", err)
	}
	if record.SourceName != "openlibrary" {
		t.Errorf("SourceName = %q, want openlibrary", record.SourceName)
	}
	if record.Title != "War and Peace" {
		t.Errorf("Title = %q", record.Title)
	}
	if len(record.Authors) != 1 || record.Authors[0] != "Leo Tolstoy" {
		t.Errorf("Authors = %v", record.Authors)
	}
	if len(record.AuthorRefs) != 0 {
		t.Errorf("AuthorRefs = %v, want empty for Open Library (names only)", record.AuthorRefs)
	}
	if len(record.Genres) != 2 {
		t.Errorf("Genres = %v", record.Genres)
	}
	if record.ISBN == nil || *record.ISBN != "9780140447934" {
		t.Errorf("ISBN = %v", record.ISBN)
	}
	if record.Description == nil || !strings.Contains(*record.Description, "Prince") {
		t.Errorf("Description = %v", record.Description)
	}
	if record.CoverURL == nil || !strings.Contains(*record.CoverURL, "12621906-L.jpg") {
		t.Errorf("CoverURL = %v", record.CoverURL)
	}
	if record.PageCount == nil || *record.PageCount != 1392 {
		t.Errorf("PageCount = %v", record.PageCount)
	}
	if record.PublishedYear == nil || *record.PublishedYear != 2005 {
		t.Errorf("PublishedYear = %v", record.PublishedYear)
	}
	if record.SourceID != "OL7353617M" {
		t.Errorf("SourceID = %q, want OL7353617M", record.SourceID)
	}
	if !strings.Contains(capturedQuery, "ISBN") {
		t.Errorf("expected bibkeys param in query: %s", capturedQuery)
	}
}

func TestOpenLibraryFindByISBN_UnknownISBN_ReturnsErrNotFound(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		// Open Library returns HTTP 200 with an empty JSON object for an
		// unknown ISBN, not a 404.
		return newTestResponse(http.StatusOK, `{}`, nil), nil
	})
	client := newOpenLibraryTestClient(t, responder)

	_, err := client.FindByISBN(context.Background(), "0000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestOpenLibraryFindByISBN_EmptyISBN_ReturnsErrInvalidQuery(t *testing.T) {
	client := newOpenLibraryTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("should not perform HTTP request for an empty ISBN")
		return nil, nil
	}))

	_, err := client.FindByISBN(context.Background(), "   ")
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("err = %v, want ErrInvalidQuery", err)
	}
}

const openLibrarySearchFixture = `{
  "numFound": 2,
  "docs": [
    {
      "title": "War and Peace",
      "author_name": ["Leo Tolstoy"],
      "isbn": ["9780140447934", "0140447938"],
      "first_publish_year": 1869,
      "subject": ["Russia", "Napoleonic Wars"],
      "cover_i": 12621906,
      "key": "/works/OL262461W",
      "language": ["rus"]
    },
    {
      "title": "War and Peace (abridged)",
      "author_name": ["Leo Tolstoy"],
      "first_publish_year": 1990,
      "key": "/works/OL999999W"
    }
  ]
}`

func TestOpenLibraryFindByTitleAuthor_ParsesFixture(t *testing.T) {
	var capturedQuery string
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedQuery = req.URL.RawQuery
		return newTestResponse(http.StatusOK, openLibrarySearchFixture, nil), nil
	})
	client := newOpenLibraryTestClient(t, responder)

	record, err := client.FindByTitleAuthor(context.Background(), "War and Peace", "Tolstoy")
	if err != nil {
		t.Fatalf("FindByTitleAuthor: %v", err)
	}
	if record.SourceName != "openlibrary" {
		t.Errorf("SourceName = %q, want openlibrary", record.SourceName)
	}
	if record.Title != "War and Peace" {
		t.Errorf("Title = %q, want first/best match 'War and Peace'", record.Title)
	}
	if record.PublishedYear == nil || *record.PublishedYear != 1869 {
		t.Errorf("PublishedYear = %v, want 1869", record.PublishedYear)
	}
	if record.ISBN == nil || *record.ISBN != "9780140447934" {
		t.Errorf("ISBN = %v", record.ISBN)
	}
	if record.CoverURL == nil || !strings.Contains(*record.CoverURL, "12621906") {
		t.Errorf("CoverURL = %v", record.CoverURL)
	}
	if record.Language != "ru" {
		t.Errorf("Language = %q, want ru (mapped from rus)", record.Language)
	}
	if record.SourceID != "OL262461W" {
		t.Errorf("SourceID = %q, want OL262461W", record.SourceID)
	}
	if !strings.Contains(capturedQuery, "title=") {
		t.Errorf("expected title param in query: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "author=") {
		t.Errorf("expected author param in query: %s", capturedQuery)
	}
}

func TestOpenLibraryFindByTitleAuthor_DocMissingAuthorAndSubject_ReturnsEmptySlicesNotNil(t *testing.T) {
	// Regression test: a search result doc lacking "author_name"/"subject"
	// (common - Open Library doesn't guarantee either field) must produce
	// non-nil Authors/Genres. A nil slice here serializes to JSON null and
	// crashed the admin lookup-assistant UI in production
	// (book-form.tsx:254, "Cannot read properties of null (reading
	// 'length')") since the frontend calls .length on both unconditionally.
	fixture := `{
  "numFound": 1,
  "docs": [
    {
      "title": "Земля обетованная",
      "key": "/works/OL1W"
    }
  ]
}`
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, fixture, nil), nil
	})
	client := newOpenLibraryTestClient(t, responder)

	record, err := client.FindByTitleAuthor(context.Background(), "Земля обетованная", "")
	if err != nil {
		t.Fatalf("FindByTitleAuthor: %v", err)
	}
	if record.Authors == nil {
		t.Error("Authors is nil, want non-nil empty slice")
	}
	if len(record.Authors) != 0 {
		t.Errorf("Authors = %v, want empty", record.Authors)
	}
	if record.Genres == nil {
		t.Error("Genres is nil, want non-nil empty slice")
	}
	if len(record.Genres) != 0 {
		t.Errorf("Genres = %v, want empty", record.Genres)
	}
}

func TestOpenLibraryFindByTitleAuthor_NoResults_ReturnsErrNotFound(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `{"numFound":0,"docs":[]}`, nil), nil
	})
	client := newOpenLibraryTestClient(t, responder)

	_, err := client.FindByTitleAuthor(context.Background(), "Nonexistent Book Title XYZ", "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestOpenLibraryFindByTitleAuthor_EmptyTitle_ReturnsErrInvalidQuery(t *testing.T) {
	client := newOpenLibraryTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("should not perform HTTP request for an empty title")
		return nil, nil
	}))

	_, err := client.FindByTitleAuthor(context.Background(), "  ", "someone")
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("err = %v, want ErrInvalidQuery", err)
	}
}

const openLibraryWorkFixturePlainString = `{
  "key": "/works/OL262461W",
  "title": "War and Peace",
  "description": "A sweeping novel about Russian society during the Napoleonic era."
}`

const openLibraryWorkFixtureTypedText = `{
  "key": "/works/OL262461W",
  "title": "War and Peace",
  "description": {"type": "/type/text", "value": "A sweeping novel about Russian society during the Napoleonic era."}
}`

const openLibraryWorkFixtureNoDescription = `{
  "key": "/works/OL262461W",
  "title": "War and Peace"
}`

func TestFetchWorkDescription_PlainStringShape(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "/works/OL262461W.json") {
			t.Errorf("unexpected request path: %s", req.URL.Path)
		}
		return newTestResponse(http.StatusOK, openLibraryWorkFixturePlainString, nil), nil
	})
	client := newOpenLibraryTestClient(t, responder)

	desc, err := client.FetchWorkDescription(context.Background(), "OL262461W")
	if err != nil {
		t.Fatalf("FetchWorkDescription: %v", err)
	}
	if desc == nil || !strings.Contains(*desc, "Napoleonic") {
		t.Errorf("Description = %v", desc)
	}
}

func TestFetchWorkDescription_TypedTextShape(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, openLibraryWorkFixtureTypedText, nil), nil
	})
	client := newOpenLibraryTestClient(t, responder)

	desc, err := client.FetchWorkDescription(context.Background(), "OL262461W")
	if err != nil {
		t.Fatalf("FetchWorkDescription: %v", err)
	}
	if desc == nil || !strings.Contains(*desc, "Napoleonic") {
		t.Errorf("Description = %v", desc)
	}
}

func TestFetchWorkDescription_NoDescriptionField_ReturnsNilNil(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, openLibraryWorkFixtureNoDescription, nil), nil
	})
	client := newOpenLibraryTestClient(t, responder)

	desc, err := client.FetchWorkDescription(context.Background(), "OL262461W")
	if err != nil {
		t.Fatalf("FetchWorkDescription: %v", err)
	}
	if desc != nil {
		t.Errorf("Description = %v, want nil", desc)
	}
}

func TestFetchWorkDescription_NotFound_ReturnsErrNotFound(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusNotFound, `{"error": "notfound"}`, nil), nil
	})
	client := newOpenLibraryTestClient(t, responder)

	desc, err := client.FetchWorkDescription(context.Background(), "OL0W")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if desc != nil {
		t.Errorf("Description = %v, want nil on error", desc)
	}
}

func TestFetchWorkDescription_MalformedJSON_ReturnsError(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `not json`, nil), nil
	})
	client := newOpenLibraryTestClient(t, responder)

	desc, err := client.FetchWorkDescription(context.Background(), "OL262461W")
	if err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
	if desc != nil {
		t.Errorf("Description = %v, want nil on error", desc)
	}
}

func TestOpenLibraryFindByTitleAuthor_WorkDescriptionLookupEnabled_FetchesAndPopulatesDescription(t *testing.T) {
	var requestPaths []string
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestPaths = append(requestPaths, req.URL.Path)
		if strings.Contains(req.URL.Path, "/works/") {
			return newTestResponse(http.StatusOK, openLibraryWorkFixturePlainString, nil), nil
		}
		return newTestResponse(http.StatusOK, openLibrarySearchFixture, nil), nil
	})
	client := NewOpenLibraryClient(
		WithHTTPClient(&http.Client{Transport: responder}),
		withLimiter(&fakeLimiter{}),
		withSleep(func(d time.Duration) {}),
		WithMaxRetries(0),
		WithWorkDescriptionLookup(),
	)

	record, err := client.FindByTitleAuthor(context.Background(), "War and Peace", "Tolstoy")
	if err != nil {
		t.Fatalf("FindByTitleAuthor: %v", err)
	}
	if len(requestPaths) != 2 {
		t.Fatalf("requestPaths = %v, want exactly 2 requests (search + works)", requestPaths)
	}
	if !strings.Contains(requestPaths[1], "/works/OL262461W.json") {
		t.Errorf("second request path = %q, want /works/OL262461W.json", requestPaths[1])
	}
	if record.Description == nil || !strings.Contains(*record.Description, "Napoleonic") {
		t.Errorf("Description = %v", record.Description)
	}
}

func TestOpenLibraryFindByTitleAuthor_WorkDescriptionLookupDisabled_MakesOnlyOneRequest(t *testing.T) {
	requestCount := 0
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		return newTestResponse(http.StatusOK, openLibrarySearchFixture, nil), nil
	})
	// Mirrors the admin lookup-assistant client (backend/cmd/api/main.go):
	// no WithWorkDescriptionLookup() option, so this must make exactly one
	// outbound Open Library call, per catalog-manual-import.md.
	client := newOpenLibraryTestClient(t, responder)

	record, err := client.FindByTitleAuthor(context.Background(), "War and Peace", "Tolstoy")
	if err != nil {
		t.Fatalf("FindByTitleAuthor: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("requestCount = %d, want 1 (no follow-up call by default)", requestCount)
	}
	if record.Description != nil {
		t.Errorf("Description = %v, want nil (no follow-up call by default)", record.Description)
	}
}

func TestOpenLibraryFindByTitleAuthor_WorkDescriptionLookupFails_StillReturnsBaseRecord(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/works/") {
			return newTestResponse(http.StatusInternalServerError, `internal error`, nil), nil
		}
		return newTestResponse(http.StatusOK, openLibrarySearchFixture, nil), nil
	})
	client := NewOpenLibraryClient(
		WithHTTPClient(&http.Client{Transport: responder}),
		withLimiter(&fakeLimiter{}),
		withSleep(func(d time.Duration) {}),
		WithMaxRetries(0),
		WithWorkDescriptionLookup(),
	)

	record, err := client.FindByTitleAuthor(context.Background(), "War and Peace", "Tolstoy")
	if err != nil {
		t.Fatalf("FindByTitleAuthor: %v, want success even though the works follow-up call fails", err)
	}
	if record.Title != "War and Peace" {
		t.Errorf("Title = %q, want base record still populated", record.Title)
	}
	if record.Description != nil {
		t.Errorf("Description = %v, want nil after a failed follow-up call", record.Description)
	}
}

func TestCoverURL_ProducesExpectedURL(t *testing.T) {
	got := CoverURL("9780140447934", CoverIDTypeISBN, CoverSizeLarge)
	want := "https://covers.openlibrary.org/b/isbn/9780140447934-L.jpg"
	if got != want {
		t.Errorf("CoverURL = %q, want %q", got, want)
	}
}

func TestCoverURL_ByNumericID(t *testing.T) {
	got := CoverURL("12621906", CoverIDTypeID, CoverSizeMedium)
	want := "https://covers.openlibrary.org/b/id/12621906-M.jpg"
	if got != want {
		t.Errorf("CoverURL = %q, want %q", got, want)
	}
}

func TestCoverURL_NoNetworkCall(t *testing.T) {
	// CoverURL is a pure function: calling it must not require an HTTP
	// client, RoundTripper, or context at all. This test exists mainly as
	// documentation/compile-time proof of that contract.
	_ = CoverURL("x", CoverIDTypeOLID, CoverSizeSmall)
}

func TestOpenLibraryFindByTitleAuthor_IgnoredQuery_ReturnsErrNotFound(t *testing.T) {
	client := newOpenLibraryTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `{"numFound":907849,"docs":[{"title":"I, Robot","key":"/works/OL1W"}]}`, nil), nil
	}))
	_, err := client.FindByTitleAuthor(context.Background(), "История Петра I", "Пушкин")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

const olMatchFixture = `{"numFound": 3, "docs": [
  {"title": "I, Robot", "author_name": ["Isaac Asimov"], "key": "/works/OL1W", "cover_i": 1},
  {"title": "White Fang", "author_name": ["Jack London"], "key": "/works/OL2W", "cover_i": 2},
  {"title": "White Fang", "author_name": ["Jack London"], "key": "/works/OL3W", "cover_i": 3}
]}`

func TestOpenLibraryFindMatchByTitleAuthor_PicksMatchingDocAndFetchesOnlyItsDescription(t *testing.T) {
	var paths []string
	client := NewOpenLibraryClient(
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			paths = append(paths, req.URL.Path)
			if req.URL.Path == "/works/OL2W.json" {
				return newTestResponse(http.StatusOK, `{"description":"desc"}`, nil), nil
			}
			return newTestResponse(http.StatusOK, olMatchFixture, nil), nil
		})}),
		withLimiter(&fakeLimiter{}),
		withSleep(func(d time.Duration) {}),
		WithMaxRetries(0),
		WithWorkDescriptionLookup(),
	)
	rec, err := client.FindMatchByTitleAuthor(context.Background(), TitleAuthorQuery{
		Title: "Белый Клык", AltTitles: []string{"White Fang"}, Authors: []string{"Джек Лондон", "Jack London"},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rec.SourceID != "OL2W" || rec.CoverURL == nil || !strings.Contains(*rec.CoverURL, "/id/2-L") {
		t.Fatalf("unexpected record: %+v", rec)
	}
	if rec.Description == nil || *rec.Description != "desc" {
		t.Errorf("description = %v", rec.Description)
	}
	for _, p := range paths {
		if strings.HasPrefix(p, "/works/") && p != "/works/OL2W.json" {
			t.Errorf("unexpected work request %s", p)
		}
	}
}

func TestOpenLibraryFindMatchByTitleAuthor_NoMatchAndIgnoredQuery(t *testing.T) {
	cases := map[string]string{
		"unrelated":  `{"numFound": 2, "docs": [{"title":"I, Robot","author_name":["Isaac Asimov"],"key":"/works/OL1W"},{"title":"Dune","author_name":["Frank Herbert"],"key":"/works/OL2W"}]}`,
		"numFound":   `{"numFound": 907849, "docs": [{"title":"White Fang","author_name":["Jack London"],"key":"/works/OL1W"}]}`,
		"empty docs": `{"numFound": 0, "docs": []}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			client := newOpenLibraryTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return newTestResponse(http.StatusOK, body, nil), nil
			}))
			_, err := client.FindMatchByTitleAuthor(context.Background(), TitleAuthorQuery{
				Title: "White Fang", Authors: []string{"Jack London"},
			})
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
		})
	}
}
