package bookdata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// wikidataTolstoyFixture mirrors the live Q7243 claims observed during the
// manual verification (Julian preferred + Gregorian normal statements).
const wikidataTolstoyFixture = `{
  "entities": {
    "Q7243": {
      "id": "Q7243",
      "labels": {"ru": {"language": "ru", "value": "Лев Николаевич Толстой"}},
      "claims": {
        "P31": [{"rank":"normal","mainsnak": {"snaktype": "value", "datavalue": {"value": {"id": "Q5"}, "type": "wikibase-entityid"}}}],
        "P569": [
          {"rank":"preferred","mainsnak":{"snaktype":"value","datavalue":{"value":{"time":"+1828-08-28T00:00:00Z","timezone":0,"before":0,"after":0,"precision":11,"calendarmodel":"http://www.wikidata.org/entity/Q1985786"},"type":"time"}}},
          {"rank":"normal","mainsnak":{"snaktype":"value","datavalue":{"value":{"time":"+1828-09-09T00:00:00Z","timezone":0,"before":0,"after":0,"precision":11,"calendarmodel":"http://www.wikidata.org/entity/Q1985727"},"type":"time"}}}
        ],
        "P570": [
          {"rank":"preferred","mainsnak":{"snaktype":"value","datavalue":{"value":{"time":"+1910-11-07T00:00:00Z","timezone":0,"before":0,"after":0,"precision":11,"calendarmodel":"http://www.wikidata.org/entity/Q1985786"},"type":"time"}}},
          {"rank":"deprecated","mainsnak":{"snaktype":"value","datavalue":{"value":{"time":"+1910-11-10T00:00:00Z","timezone":0,"before":0,"after":0,"precision":11,"calendarmodel":"http://www.wikidata.org/entity/Q1985727"},"type":"time"}}}
        ],
        "P18": [{"rank":"normal","mainsnak":{"snaktype":"value","datavalue":{"value":"L.N.Tolstoy Prokudin-Gorsky.jpg","type":"string"}}}]
      }
    }
  },
  "success": 1
}`

func TestWikidataFetchAuthor_ParsesDatesAndPhoto(t *testing.T) {
	client := newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, wikidataTolstoyFixture, nil), nil
	}))

	rec, err := client.FetchAuthor(context.Background(), "Q7243")
	if err != nil {
		t.Fatalf("FetchAuthor: %v", err)
	}
	wantBirth := time.Date(1828, 9, 9, 0, 0, 0, 0, time.UTC)
	wantDeath := time.Date(1910, 11, 20, 0, 0, 0, 0, time.UTC)
	if rec.BirthDate == nil || !rec.BirthDate.Equal(wantBirth) {
		t.Errorf("BirthDate = %v, want %v", rec.BirthDate, wantBirth)
	}
	if rec.DeathDate == nil || !rec.DeathDate.Equal(wantDeath) {
		t.Errorf("DeathDate = %v, want %v", rec.DeathDate, wantDeath)
	}
	if rec.BirthDateUnusable || rec.DeathDateUnusable {
		t.Errorf("unexpected unusable flags: %+v", rec)
	}
	if rec.PhotoFile != "L.N.Tolstoy Prokudin-Gorsky.jpg" {
		t.Errorf("PhotoFile = %q", rec.PhotoFile)
	}
}

func TestWikidataFetchAuthors_OneBatchedRequest(t *testing.T) {
	var calls int
	var gotIDs, gotUA string
	body := `{"entities":{
		"Q1":{"id":"Q1","labels":{"ru":{"value":"Первый"}},"claims":{}},
		"Q100":{"id":"Q200","labels":{"ru":{"value":"Перенаправленный"}},"claims":{}},
		"Q3":{"id":"Q3","missing":""}
	},"success":1}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotIDs = r.URL.Query().Get("ids")
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	client := NewWikidataClient(
		withLimiter(&fakeLimiter{}), withSleep(func(time.Duration) {}), WithMaxRetries(0),
		WithUserAgent("TestAgent/1.0"), WithSearchBaseURL(srv.URL),
	)

	got, err := client.FetchAuthors(context.Background(), []string{"Q1", " Q100 ", "Q3", "Q1"})
	if err != nil {
		t.Fatalf("FetchAuthors: %v", err)
	}
	if calls != 1 || gotIDs != "Q1|Q100|Q3" || gotUA != "TestAgent/1.0" {
		t.Fatalf("calls=%d ids=%q ua=%q", calls, gotIDs, gotUA)
	}
	if len(got) != 2 || got["Q1"].Name != "Первый" {
		t.Fatalf("result = %+v", got)
	}
	if got["Q100"] == nil || got["Q100"].QID != "Q200" {
		t.Fatalf("redirect must be keyed by the requested QID with the canonical QID, got %+v", got["Q100"])
	}
	if _, ok := got["Q3"]; ok {
		t.Fatal("missing entity must be absent")
	}
}

func TestWikidataFetchAuthors_NoSuchEntityDropsCulpritAndRetries(t *testing.T) {
	var requests []string
	client := newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		ids := req.URL.Query().Get("ids")
		requests = append(requests, ids)
		if strings.Contains(ids, "Q999") {
			return newTestResponse(http.StatusOK, `{"error":{"code":"no-such-entity","id":"Q999"}}`, nil), nil
		}
		return newTestResponse(http.StatusOK, `{"entities":{"Q1":{"id":"Q1","claims":{}}}}`, nil), nil
	}))

	got, err := client.FetchAuthors(context.Background(), []string{"Q1", "Q999"})
	if err != nil {
		t.Fatalf("FetchAuthors: %v", err)
	}
	if len(requests) != 2 || requests[1] != "Q1" || got["Q1"] == nil || len(got) != 1 {
		t.Fatalf("requests=%v got=%v", requests, got)
	}
}

func TestWikidataFetchAuthors_InvalidInputWithoutHTTP(t *testing.T) {
	client := newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("no HTTP call expected")
		return nil, nil
	}))
	tooMany := make([]string, 51)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("Q%d", i+1)
	}
	for name, ids := range map[string][]string{
		"empty":    nil,
		"too many": tooMany,
		"invalid":  {"Q1", "P31"},
	} {
		if _, err := client.FetchAuthors(context.Background(), ids); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("%s: err = %v, want ErrInvalidQuery", name, err)
		}
	}
}

func TestWikidataFetchAuthors_ServerError(t *testing.T) {
	client := newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusServiceUnavailable, "", nil), nil
	}))
	if _, err := client.FetchAuthors(context.Background(), []string{"Q1"}); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("err = %v, want ErrSourceUnavailable", err)
	}
}
