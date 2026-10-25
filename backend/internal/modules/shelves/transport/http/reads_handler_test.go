package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type readsEnv struct {
	h    *Handler
	repo *fakeRepository
	user uuid.UUID
	book uuid.UUID
}

func newReadsEnv() *readsEnv {
	h, repo := setupHandler()
	e := &readsEnv{h: h, repo: repo, user: uuid.New(), book: uuid.New()}
	repo.books[e.book] = &catalogDomain.Book{ID: e.book, Title: "B", Slug: "b"}
	return e
}

func (e *readsEnv) call(fn echo.HandlerFunc, method, target, body, id string, user *uuid.UUID) *httptest.ResponseRecorder {
	ec := echo.New()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := ec.NewContext(req, rec)
	if id != "" {
		c.SetParamNames("id")
		c.SetParamValues(id)
	}
	if user != nil {
		c.Set("user_id", user.String())
	}
	if err := fn(c); err != nil {
		panic(err)
	}
	return rec
}

func (e *readsEnv) seedFinished(user uuid.UUID, start *time.Time, finish time.Time) uuid.UUID {
	id := uuid.New()
	e.repo.reads = append(e.repo.reads, domain.BookRead{ID: id, UserID: user, BookID: e.book, StartedAt: start, FinishedAt: &finish, CreatedAt: finish})
	return id
}

func (e *readsEnv) seedOpen(user uuid.UUID) uuid.UUID {
	id := uuid.New()
	st := time.Now().UTC().Add(-48 * time.Hour)
	e.repo.reads = append(e.repo.reads, domain.BookRead{ID: id, UserID: user, BookID: e.book, StartedAt: &st, CreatedAt: st})
	return id
}

func TestReadsList(t *testing.T) {
	e := newReadsEnv()
	d := time.Date(2019, 3, 1, 12, 0, 0, 0, time.UTC)
	e.seedFinished(e.user, nil, d)
	e.seedFinished(uuid.New(), nil, d)

	if rec := e.call(e.h.ListReads, http.MethodGet, "/reads?book_id="+e.book.String(), "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", rec.Code)
	}
	if rec := e.call(e.h.ListReads, http.MethodGet, "/reads?book_id=zzz", "", "", &e.user); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad book_id: %d", rec.Code)
	}
	rec := e.call(e.h.ListReads, http.MethodGet, "/reads?book_id="+e.book.String(), "", "", &e.user)
	var out struct{ Items []map[string]any }
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != 200 {
		t.Fatalf("code %d err %v", rec.Code, err)
	}
	if len(out.Items) != 1 || out.Items[0]["state"] != "finished" || out.Items[0]["started_at"] != nil {
		t.Fatalf("unexpected %v", out.Items)
	}
	// A book nobody read answers an empty array, not null.
	rec = e.call(e.h.ListReads, http.MethodGet, "/reads?book_id="+uuid.NewString(), "", "", &e.user)
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestReadsCreate(t *testing.T) {
	e := newReadsEnv()
	body := `{"book_id":"` + e.book.String() + `","started_on":"2015-04-01","finished_on":"2015-04-20"}`
	rec := e.call(e.h.CreateRead, http.MethodPost, "/reads", body, "", &e.user)
	if rec.Code != http.StatusCreated {
		t.Fatalf("code %d %s", rec.Code, rec.Body.String())
	}
	var r map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	if r["state"] != "finished" || r["finished_at"] != "2015-04-20T12:00:00Z" {
		t.Fatalf("unexpected %v", r)
	}
	if _, ok := r["manual_end"]; ok {
		t.Fatal("manual_end must not be exposed")
	}
	cases := map[string]int{
		`{"book_id":"` + e.book.String() + `","finished_on":"2999-01-01"}`:                           400,
		`{"book_id":"` + e.book.String() + `","started_on":"2020-05-10","finished_on":"2020-05-01"}`: 400,
		`{"book_id":"` + e.book.String() + `"}`:                                                      400,
		`{"book_id":"` + uuid.NewString() + `","finished_on":"2020-05-01"}`:                          404,
		`{"book_id":"nope","finished_on":"2020-05-01"}`:                                              400,
	}
	for b, want := range cases {
		if rec := e.call(e.h.CreateRead, http.MethodPost, "/reads", b, "", &e.user); rec.Code != want {
			t.Errorf("%s: got %d want %d", b, rec.Code, want)
		}
	}
	if rec := e.call(e.h.CreateRead, http.MethodPost, "/reads", body, "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", rec.Code)
	}
}

func TestReadsUpdate(t *testing.T) {
	e := newReadsEnv()
	start := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC)
	fin := e.seedFinished(e.user, &start, time.Date(2020, 6, 1, 12, 0, 0, 0, time.UTC))
	open := e.seedOpen(e.user)
	foreign := e.seedFinished(uuid.New(), nil, start)

	patch := func(id, body string) *httptest.ResponseRecorder {
		return e.call(e.h.UpdateRead, http.MethodPatch, "/reads/"+id, body, id, &e.user)
	}
	// null clears the start, an absent key leaves it alone
	rec := patch(fin.String(), `{"started_on":null}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"started_at":null`) || !strings.Contains(rec.Body.String(), `2020-06-01T12:00:00Z`) {
		t.Fatalf("clear start: %d %s", rec.Code, rec.Body.String())
	}
	if rec := patch(fin.String(), `{}`); rec.Code != 400 {
		t.Fatalf("empty body: %d", rec.Code)
	}
	if rec := patch(fin.String(), `{"abandoned_on":"2020-06-02"}`); rec.Code != 400 {
		t.Fatalf("outcome switch: %d", rec.Code)
	}
	if rec := patch(open.String(), `{"finished_on":"2020-06-02"}`); rec.Code != 400 {
		t.Fatalf("finish via history: %d", rec.Code)
	}
	if rec := patch(open.String(), `{"started_on":null}`); rec.Code != 400 {
		t.Fatalf("null start on open: %d", rec.Code)
	}
	if rec := patch(open.String(), `{"started_on":"2026-01-02"}`); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"state":"open"`) {
		t.Fatalf("open start: %d %s", rec.Code, rec.Body.String())
	}
	if rec := patch(foreign.String(), `{"started_on":"2019-01-01"}`); rec.Code != 404 {
		t.Fatalf("foreign: %d", rec.Code)
	}
	if rec := patch("not-a-uuid", `{"started_on":"2019-01-01"}`); rec.Code != 404 {
		t.Fatalf("malformed id: %d", rec.Code)
	}
}

func TestReadsDelete(t *testing.T) {
	e := newReadsEnv()
	d := time.Date(2020, 6, 1, 12, 0, 0, 0, time.UTC)
	fin := e.seedFinished(e.user, nil, d)
	open := e.seedOpen(e.user)
	foreign := e.seedFinished(uuid.New(), nil, d)

	del := func(id string, user *uuid.UUID) int {
		return e.call(e.h.DeleteRead, http.MethodDelete, "/reads/"+id, "", id, user).Code
	}
	if c := del(open.String(), &e.user); c != 409 {
		t.Fatalf("open: %d", c)
	}
	if c := del(foreign.String(), &e.user); c != 404 {
		t.Fatalf("foreign: %d", c)
	}
	if c := del(fin.String(), nil); c != 401 {
		t.Fatalf("no token: %d", c)
	}
	if c := del(fin.String(), &e.user); c != 200 {
		t.Fatalf("closed: %d", c)
	}
	if len(e.repo.reads) != 2 {
		t.Fatalf("rows left: %d", len(e.repo.reads))
	}
}
