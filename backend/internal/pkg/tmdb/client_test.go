package tmdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testKey = "SECRET-KEY-123"

func newTest(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(testKey, WithBaseURL(srv.URL))
}

func TestPoster_Found(t *testing.T) {
	var gotPath, gotKey, gotLang string
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey, gotLang = r.URL.Path, r.URL.Query().Get("api_key"), r.URL.Query().Get("language")
		_, _ = w.Write([]byte(`{"poster_path":"/abc_DEF-1.jpg"}`))
	})
	got, err := c.Poster(context.Background(), KindMovie, "603")
	if err != nil || got != "https://image.tmdb.org/t/p/w500/abc_DEF-1.jpg" {
		t.Fatalf("got %q, %v", got, err)
	}
	if gotPath != "/movie/603" || gotKey != testKey || gotLang != "ru-RU" {
		t.Errorf("request: %s key=%s lang=%s", gotPath, gotKey, gotLang)
	}
}

func TestPoster_NoPoster(t *testing.T) {
	for name, body := range map[string]string{
		"null":    `{"poster_path":null}`,
		"missing": `{}`,
		"invalid": `{"poster_path":"/../x.jpg"}`,
		"svg":     `{"poster_path":"/a.svg"}`,
	} {
		c := newTest(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
		got, err := c.Poster(context.Background(), KindTV, "1399")
		if err != nil || got != "" {
			t.Errorf("%s: got %q, %v", name, got, err)
		}
	}
}

func TestPoster_Statuses(t *testing.T) {
	cases := []struct {
		status  int
		wantErr error
	}{
		{http.StatusNotFound, nil},
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusTooManyRequests, ErrUnavailable},
		{http.StatusBadGateway, ErrUnavailable},
	}
	for _, tc := range cases {
		c := newTest(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status) })
		got, err := c.Poster(context.Background(), KindMovie, "1")
		if got != "" || !errors.Is(err, tc.wantErr) {
			t.Errorf("%d: got %q, %v", tc.status, got, err)
		}
		if err != nil && strings.Contains(err.Error(), testKey) {
			t.Errorf("%d: error leaks the key: %v", tc.status, err)
		}
	}
}

func TestPoster_ErrorsNeverContainKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // connection refused: the *url.Error would carry the URL
	c := New(testKey, WithBaseURL(url))
	_, err := c.Poster(context.Background(), KindMovie, "1")
	if !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), testKey) {
		t.Fatalf("err = %v", err)
	}

	c = newTest(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`not json`)) })
	_, err = c.Poster(context.Background(), KindMovie, "1")
	if !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), testKey) {
		t.Fatalf("bad json err = %v", err)
	}
}

func TestPoster_InvalidQueryNoCall(t *testing.T) {
	calls := 0
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
	for _, tc := range [][2]string{{"person", "1"}, {KindMovie, "../1"}, {KindMovie, ""}, {KindTV, "12345678901"}} {
		if _, err := c.Poster(context.Background(), tc[0], tc[1]); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("%v: err = %v", tc, err)
		}
	}
	if calls != 0 {
		t.Errorf("calls = %d", calls)
	}
}
