package bookdata

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

const wikidataEntityFixture = `{
  "entities": {
    "Q7200": {
      "type": "item",
      "id": "Q7200",
      "labels": {"ru": {"language": "ru", "value": "Лев Николаевич Толстой"}},
      "claims": {
        "P31": [
          {"mainsnak": {"snaktype": "value", "property": "P31", "datavalue": {"value": {"entity-type": "item", "numeric-id": 5, "id": "Q5"}, "type": "wikibase-entityid"}}},
          {"mainsnak": {"snaktype": "novalue", "property": "P31"}},
          {"mainsnak": {"snaktype": "somevalue", "property": "P31"}},
          {"mainsnak": {"snaktype": "value", "property": "P31", "datavalue": {"value": {"entity-type": "item", "numeric-id": 61002, "id": "Q61002"}, "type": "wikibase-entityid"}}}
        ],
        "P106": [
          {"mainsnak": {"snaktype": "value", "property": "P106", "datavalue": {"value": {"entity-type": "item", "id": "Q36180"}, "type": "wikibase-entityid"}}}
        ]
      }
    }
  },
  "success": 1
}`

func TestWikidataFetchAuthor_ParsesLabelAndInstanceOf(t *testing.T) {
	var gotURL *url.URL
	var gotUA string
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL
		gotUA = req.Header.Get("User-Agent")
		return newTestResponse(http.StatusOK, wikidataEntityFixture, nil), nil
	})
	client := NewWikidataClient(
		WithHTTPClient(&http.Client{Transport: responder}),
		withLimiter(&fakeLimiter{}),
		withSleep(func(d time.Duration) {}),
		WithMaxRetries(0),
		WithUserAgent("TestAgent/1.0"),
		WithBaseURL("https://wikidata.test/sparql"),
		WithSearchBaseURL("https://wikidata-search.test/w/api.php"),
	)

	record, err := client.FetchAuthor(context.Background(), " Q7200 ")
	if err != nil {
		t.Fatalf("FetchAuthor: %v", err)
	}
	if gotURL.Host != "wikidata-search.test" {
		t.Errorf("host = %q, want wikidata-search.test", gotURL.Host)
	}
	if gotUA != "TestAgent/1.0" {
		t.Errorf("User-Agent = %q, want TestAgent/1.0", gotUA)
	}
	query := gotURL.Query()
	want := map[string]string{
		"action":    "wbgetentities",
		"ids":       "Q7200",
		"props":     "labels|claims",
		"languages": "ru",
		"format":    "json",
	}
	for k, v := range want {
		if query.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, query.Get(k), v)
		}
	}

	if record.QID != "Q7200" || record.Name != "Лев Николаевич Толстой" {
		t.Errorf("record = %+v", record)
	}
	if !reflect.DeepEqual(record.InstanceOf, []string{"Q5", "Q61002"}) {
		t.Errorf("InstanceOf = %v, want [Q5 Q61002] (novalue/somevalue skipped)", record.InstanceOf)
	}
}

func TestWikidataFetchAuthor_NoRuLabel_ReturnsEmptyName(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `{"entities":{"Q42":{"id":"Q42","labels":{},"claims":{}}},"success":1}`, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	record, err := client.FetchAuthor(context.Background(), "Q42")
	if err != nil {
		t.Fatalf("FetchAuthor: %v", err)
	}
	if record.Name != "" || record.QID != "Q42" || len(record.InstanceOf) != 0 {
		t.Errorf("record = %+v, want QID Q42 with empty name and no P31", record)
	}
}

func TestWikidataFetchAuthor_Redirect_ReturnsCanonicalQID(t *testing.T) {
	body := `{"entities":{"Q100":{"id":"Q200","labels":{"ru":{"value":"Автор"}},"claims":{}}},"redirects":{"from":"Q100","to":"Q200"},"success":1}`
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, body, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	record, err := client.FetchAuthor(context.Background(), "Q100")
	if err != nil {
		t.Fatalf("FetchAuthor: %v", err)
	}
	if record.QID != "Q200" {
		t.Errorf("QID = %q, want canonical Q200", record.QID)
	}
}

func TestWikidataFetchAuthor_MissingEntity_ReturnsErrNotFound(t *testing.T) {
	bodies := map[string]string{
		"missing key":    `{"entities":{"Q999999999":{"id":"Q999999999","missing":""}},"success":1}`,
		"no-such-entity": `{"error":{"code":"no-such-entity","info":"Could not find an entity with the ID \"Q999999999\"."}}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return newTestResponse(http.StatusOK, body, nil), nil
			})
			client := newWikidataTestClient(t, responder)

			_, err := client.FetchAuthor(context.Background(), "Q999999999")
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestWikidataFetchAuthor_InvalidQID_ReturnsErrInvalidQueryWithoutHTTP(t *testing.T) {
	client := newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("should not perform HTTP request for an invalid QID")
		return nil, nil
	}))

	for _, qid := range []string{"", "  ", "q7200", "P31", "Q7200|Q42", "Q"} {
		if _, err := client.FetchAuthor(context.Background(), qid); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("FetchAuthor(%q) err = %v, want ErrInvalidQuery", qid, err)
		}
	}
}

func TestWikidataFetchAuthor_BadRequestStatus_ReturnsErrInvalidQuery(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusBadRequest, "bad", nil), nil
	})
	client := newWikidataTestClient(t, responder)

	if _, err := client.FetchAuthor(context.Background(), "Q7200"); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("err = %v, want ErrInvalidQuery", err)
	}
}

func TestWikidataFetchAuthor_ServerError_ReturnsErrSourceUnavailable(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusBadGateway, "", nil), nil
	})
	client := newWikidataTestClient(t, responder)

	if _, err := client.FetchAuthor(context.Background(), "Q7200"); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("err = %v, want ErrSourceUnavailable", err)
	}
}

func TestWikidataFetchAuthor_MalformedJSON_ReturnsWrappedError(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `{not valid json`, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	_, err := client.FetchAuthor(context.Background(), "Q7200")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parse wikidata entity response") {
		t.Errorf("err = %v, want it to mention parse failure", err)
	}
}
