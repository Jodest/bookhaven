package bookdata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

type fantLabRoute struct {
	status int
	body   string
}

func fantLabTestClient(t *testing.T, routes map[string]fantLabRoute, paths *[]string) (*FantLabClient, *fakeLimiter) {
	t.Helper()
	lim := &fakeLimiter{}
	c := NewFantLabClient(
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			*paths = append(*paths, req.URL.Path)
			r, ok := routes[req.URL.Path]
			if !ok {
				return newTestResponse(http.StatusNotFound, `{}`, nil), nil
			}
			return newTestResponse(r.status, r.body, nil), nil
		})}),
		withLimiter(lim),
		withSleep(func(time.Duration) {}),
		WithMaxRetries(0),
	)
	return c, lim
}

// fantLabWorkBody builds a /work/{id}/extended body with one paper block (key
// "10", like the live API) and one audio block with a same-named edition.
func fantLabWorkBody(workName string, paperEditions ...string) string {
	list := ""
	for i, e := range paperEditions {
		if i > 0 {
			list += ","
		}
		list += e
	}
	return fmt.Sprintf(`{"work_name":%q,"editions_blocks":{
"10":{"block":"paper","list":[%s]},
"20":{"block":"audio","list":[{"edition_id":900,"name":%q,"lang_code":"ru","year":2020,"pic_num":1}]}}}`, workName, list, workName)
}

func fantLabEd(id int, name, lang string, year int, pic string) string {
	return fmt.Sprintf(`{"edition_id":%d,"name":%q,"lang_code":%q,"year":%d%s}`, id, name, lang, year, pic)
}

const fantLabImg = `{"image":"/images/editions/big/1?r=1492544818"}`

func TestFantLabClient_FindSelfTitledEditionCover(t *testing.T) {
	ctx := context.Background()

	t.Run("self-titled hit confirmed by edition card", func(t *testing.T) {
		var paths []string
		c, lim := fantLabTestClient(t, map[string]fantLabRoute{
			"/work/5/extended": {200, fantLabWorkBody("Пикник на обочине", fantLabEd(77, "Пикник на обочине", "ru", 2008, `,"pic_num":1`))},
			"/edition/77":      {200, fantLabImg},
		}, &paths)
		u, used, err := c.FindSelfTitledEditionCover(ctx, "Пикник на обочине", []string{"5"})
		if err != nil || u != "https://fantlab.ru/images/editions/big/77" || used != "5/77" {
			t.Fatalf("got %q %q %v", u, used, err)
		}
		if !reflect.DeepEqual(paths, []string{"/work/5/extended", "/edition/77"}) || lim.waitCalls != 2 {
			t.Fatalf("paths = %v waits=%d", paths, lim.waitCalls)
		}
	})

	t.Run("work name mismatch", func(t *testing.T) {
		var paths []string
		c, _ := fantLabTestClient(t, map[string]fantLabRoute{
			"/work/5/extended": {200, fantLabWorkBody("Другое имя", fantLabEd(77, "Пикник на обочине", "ru", 2008, `,"pic_num":1`))},
		}, &paths)
		_, _, err := c.FindSelfTitledEditionCover(ctx, "Пикник на обочине", []string{"5"})
		if !errors.Is(err, ErrNotFound) || len(paths) != 1 {
			t.Fatalf("err=%v paths=%v", err, paths)
		}
	})

	t.Run("collection, english and audio editions skipped", func(t *testing.T) {
		var paths []string
		c, _ := fantLabTestClient(t, map[string]fantLabRoute{
			"/work/5/extended": {200, fantLabWorkBody("Пикник",
				fantLabEd(1, "Пикник. Сборник", "ru", 2008, `,"pic_num":1`),
				fantLabEd(2, "Пикник", "en", 2008, `,"pic_num":1`),
				fantLabEd(3, "", "ru", 2008, `,"pic_num":1`))},
		}, &paths)
		_, _, err := c.FindSelfTitledEditionCover(ctx, "Пикник", []string{"5"})
		// the audio block holds a same-named edition (900): it must not be requested
		if !errors.Is(err, ErrNotFound) || !reflect.DeepEqual(paths, []string{"/work/5/extended"}) {
			t.Fatalf("err=%v paths=%v", err, paths)
		}
	})

	t.Run("newest first, falls back to older with image", func(t *testing.T) {
		var paths []string
		c, _ := fantLabTestClient(t, map[string]fantLabRoute{
			"/work/5/extended": {200, fantLabWorkBody("Книга",
				fantLabEd(30, "Книга", "ru", 2008, ``),
				fantLabEd(20, "Книга", "ru", 2015, ``),
				fantLabEd(10, "Книга", "ru", 0, ``))},
			"/edition/20": {200, `{"image":null}`},
			"/edition/30": {200, fantLabImg},
		}, &paths)
		u, used, err := c.FindSelfTitledEditionCover(ctx, "Книга", []string{"5"})
		if err != nil || used != "5/30" || u != "https://fantlab.ru/images/editions/big/30" {
			t.Fatalf("got %q %q %v", u, used, err)
		}
		if !reflect.DeepEqual(paths, []string{"/work/5/extended", "/edition/20", "/edition/30"}) {
			t.Fatalf("paths = %v", paths)
		}
	})

	t.Run("pic_num zero needs no request", func(t *testing.T) {
		var paths []string
		c, _ := fantLabTestClient(t, map[string]fantLabRoute{
			"/work/5/extended": {200, fantLabWorkBody("Книга", fantLabEd(30, "Книга", "ru", 2008, `,"pic_num":0`))},
		}, &paths)
		_, _, err := c.FindSelfTitledEditionCover(ctx, "Книга", []string{"5"})
		if !errors.Is(err, ErrNotFound) || len(paths) != 1 {
			t.Fatalf("err=%v paths=%v", err, paths)
		}
	})

	t.Run("404 on first id then hit on second", func(t *testing.T) {
		var paths []string
		c, _ := fantLabTestClient(t, map[string]fantLabRoute{
			"/work/8/extended": {200, fantLabWorkBody("Книга", fantLabEd(40, "Книга", "ru", 2001, `,"pic_num":2`))},
			"/edition/40":      {200, fantLabImg},
		}, &paths)
		_, used, err := c.FindSelfTitledEditionCover(ctx, "Книга", []string{"work8", "3"})
		if err != nil || used != "8/40" {
			t.Fatalf("used=%q err=%v", used, err)
		}
		if paths[0] != "/work/3/extended" {
			t.Fatalf("paths = %v", paths)
		}
	})

	t.Run("503 is an error, not a miss", func(t *testing.T) {
		var paths []string
		c, _ := fantLabTestClient(t, map[string]fantLabRoute{"/work/5/extended": {503, `{}`}}, &paths)
		_, _, err := c.FindSelfTitledEditionCover(ctx, "Книга", []string{"5"})
		if err == nil || errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("at most four requests per call", func(t *testing.T) {
		var paths []string
		body := fantLabWorkBody("Книга",
			fantLabEd(1, "Книга", "ru", 2001, ``), fantLabEd(2, "Книга", "ru", 2002, ``), fantLabEd(3, "Книга", "ru", 2003, ``))
		c, _ := fantLabTestClient(t, map[string]fantLabRoute{
			"/work/1/extended": {200, body},
			"/work/2/extended": {200, body},
			"/edition/1":       {200, `{"image":""}`},
			"/edition/2":       {200, `{"image":""}`},
			"/edition/3":       {200, `{"image":""}`},
		}, &paths)
		_, _, err := c.FindSelfTitledEditionCover(ctx, "Книга", []string{"1", "2"})
		if !errors.Is(err, ErrNotFound) || len(paths) != MaxFantLabRequestsPerCall {
			t.Fatalf("err=%v paths=%v", err, paths)
		}
	})

	t.Run("empty title never matches", func(t *testing.T) {
		var paths []string
		c, _ := fantLabTestClient(t, nil, &paths)
		if _, _, err := c.FindSelfTitledEditionCover(ctx, " !! ", []string{"5"}); !errors.Is(err, ErrNotFound) || len(paths) != 0 {
			t.Fatalf("err=%v paths=%v", err, paths)
		}
	})
}

func TestFantLabClient_RequestBudget(t *testing.T) {
	var paths []string
	c, _ := fantLabTestClient(t, map[string]fantLabRoute{
		"/work/5/extended": {200, fantLabWorkBody("Книга", fantLabEd(30, "Книга", "ru", 2008, `,"pic_num":1`))},
		"/edition/30":      {200, fantLabImg},
	}, &paths)
	c.SetRequestBudget(2)
	if _, _, err := c.FindSelfTitledEditionCover(context.Background(), "Книга", []string{"5"}); err != nil {
		t.Fatal(err)
	}
	paths = nil
	_, _, err := c.FindSelfTitledEditionCover(context.Background(), "Книга", []string{"5"})
	if !errors.Is(err, ErrBudgetExhausted) || len(paths) != 0 {
		t.Fatalf("err=%v paths=%v, want ErrBudgetExhausted with no request", err, paths)
	}
	c.SetRequestBudget(-1)
	if _, _, err := c.FindSelfTitledEditionCover(context.Background(), "Книга", []string{"5"}); err != nil {
		t.Fatalf("unlimited again: %v", err)
	}
}

func TestNewFantLabClient_Defaults(t *testing.T) {
	c := NewFantLabClient()
	if c.cfg.baseURL != "https://api.fantlab.ru" || c.cfg.maxRetries != 1 {
		t.Fatalf("baseURL=%q retries=%d", c.cfg.baseURL, c.cfg.maxRetries)
	}
	bl, ok := c.cfg.limiter.(*budgetLimiter)
	if !ok {
		t.Fatalf("limiter = %T", c.cfg.limiter)
	}
	lim, ok := bl.inner.(*rate.Limiter)
	if !ok || lim.Limit() != rate.Every(1200*time.Millisecond) || lim.Burst() != 1 {
		t.Fatalf("inner limiter = %#v", bl.inner)
	}
}
