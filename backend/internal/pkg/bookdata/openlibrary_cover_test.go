package bookdata

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
)

type coverRoute = struct {
	status int
	body   string
}

func coverTestClient(t *testing.T, routes map[string]coverRoute, paths *[]string) *OpenLibraryClient {
	t.Helper()
	return newOpenLibraryTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		*paths = append(*paths, req.URL.Path)
		r, ok := routes[req.URL.Path]
		if !ok {
			return newTestResponse(http.StatusNotFound, `{}`, nil), nil
		}
		return newTestResponse(r.status, r.body, nil), nil
	}))
}

func TestFindCoverByOpenLibraryIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("work hit", func(t *testing.T) {
		var paths []string
		c := coverTestClient(t, map[string]coverRoute{"/works/OL1W.json": {200, `{"covers":[555],"description":"x"}`}}, &paths)
		u, used, err := c.FindCoverByOpenLibraryIDs(ctx, []string{"OL1W"})
		if err != nil || used != "OL1W" || u != "https://covers.openlibrary.org/b/id/555-L.jpg" {
			t.Fatalf("got %q %q %v", u, used, err)
		}
	})

	t.Run("edition path", func(t *testing.T) {
		var paths []string
		c := coverTestClient(t, map[string]coverRoute{"/books/OL9M.json": {200, `{"covers":[77]}`}}, &paths)
		_, used, err := c.FindCoverByOpenLibraryIDs(ctx, []string{"OL9M"})
		if err != nil || used != "OL9M" || !reflect.DeepEqual(paths, []string{"/books/OL9M.json"}) {
			t.Fatalf("used=%q err=%v paths=%v", used, err, paths)
		}
	})

	t.Run("first positive cover", func(t *testing.T) {
		var paths []string
		c := coverTestClient(t, map[string]coverRoute{"/works/OL1W.json": {200, `{"covers":[-1,0,123]}`}}, &paths)
		u, _, err := c.FindCoverByOpenLibraryIDs(ctx, []string{"OL1W"})
		if err != nil || u != "https://covers.openlibrary.org/b/id/123-L.jpg" {
			t.Fatalf("got %q %v", u, err)
		}
	})

	t.Run("first 404 then hit, works first", func(t *testing.T) {
		var paths []string
		c := coverTestClient(t, map[string]coverRoute{
			"/works/OL2W.json": {200, `{"covers":[8]}`},
			"/books/OL1M.json": {200, `{"covers":[9]}`},
		}, &paths)
		_, used, err := c.FindCoverByOpenLibraryIDs(ctx, []string{"OL1M", "OL2W", "OL1W"})
		if err != nil || used != "OL2W" {
			t.Fatalf("used=%q err=%v", used, err)
		}
		if want := []string{"/works/OL1W.json", "/works/OL2W.json"}; !reflect.DeepEqual(paths, want) {
			t.Fatalf("paths = %v, want %v", paths, want)
		}
	})

	t.Run("all empty is not found", func(t *testing.T) {
		var paths []string
		c := coverTestClient(t, map[string]coverRoute{
			"/works/OL1W.json": {200, `{"covers":[]}`},
			"/books/OL1M.json": {200, `{"covers":[-1]}`},
		}, &paths)
		_, _, err := c.FindCoverByOpenLibraryIDs(ctx, []string{"OL1W", "OL1M"})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("5xx on the only id is an error", func(t *testing.T) {
		var paths []string
		c := coverTestClient(t, map[string]coverRoute{"/works/OL1W.json": {503, `x`}}, &paths)
		_, _, err := c.FindCoverByOpenLibraryIDs(ctx, []string{"OL1W"})
		if err == nil || errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want source error", err)
		}
	})

	t.Run("invalid ids make no request", func(t *testing.T) {
		var paths []string
		c := coverTestClient(t, nil, &paths)
		_, _, err := c.FindCoverByOpenLibraryIDs(ctx, []string{"foo", "OL12", ""})
		if !errors.Is(err, ErrNotFound) || len(paths) != 0 {
			t.Fatalf("err=%v paths=%v", err, paths)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		var paths []string
		c := coverTestClient(t, nil, &paths)
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, _, err := c.FindCoverByOpenLibraryIDs(cctx, []string{"OL1W"})
		if !errors.Is(err, context.Canceled) || len(paths) != 0 {
			t.Fatalf("err=%v paths=%v", err, paths)
		}
	})
}
