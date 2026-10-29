package bookdata

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestFetchFantLabWorkIDs(t *testing.T) {
	body := `{"results":{"bindings":[
{"item":{"type":"uri","value":"http://www.wikidata.org/entity/Q1"},"fantlabIds":{"type":"literal","value":"363|12"}},
{"item":{"type":"uri","value":"http://www.wikidata.org/entity/Q2"},"fantlabIds":{"type":"literal","value":""}}
]}}`
	var calls int
	var gotURL *url.URL
	got, err := adaptClient(t, body, &calls, &gotURL).FetchFantLabWorkIDs(context.Background(), []string{"Q1", "Q2", "Q3"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
	q := gotURL.Query().Get("query")
	if !strings.Contains(q, "wd:Q1 wd:Q2 wd:Q3") || !strings.Contains(q, "wdt:P7439") {
		t.Fatalf("query = %s", q)
	}
	want := map[string][]string{"Q1": {"363", "12"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFetchFantLabWorkIDs_InvalidInput(t *testing.T) {
	var calls int
	c := adaptClient(t, `{}`, &calls, nil)
	tooMany := make([]string, MaxFantLabIDBooks+1)
	for i := range tooMany {
		tooMany[i] = "Q1"
	}
	for _, in := range [][]string{nil, {"x"}, {"Q1", "q2"}, tooMany} {
		if _, err := c.FetchFantLabWorkIDs(context.Background(), in); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("%v: err = %v, want ErrInvalidQuery", in, err)
		}
	}
	if calls != 0 {
		t.Errorf("calls = %d, want 0", calls)
	}
}
