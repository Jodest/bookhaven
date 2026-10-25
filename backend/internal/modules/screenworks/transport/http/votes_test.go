package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/screenworks/repository"
	"github.com/bookhaven/backend/internal/modules/screenworks/service"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const ddl = `
CREATE TABLE books (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT, created_at DATETIME, updated_at DATETIME);
CREATE TABLE screen_works (id TEXT PRIMARY KEY, title TEXT NOT NULL, original_title TEXT, kind TEXT NOT NULL, year INTEGER, director TEXT, imdb_id TEXT, kinopoisk_id TEXT, external_source TEXT, external_id TEXT, manually_edited_at DATETIME, created_at DATETIME, updated_at DATETIME);
CREATE TABLE book_screen_works (book_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, hidden BOOLEAN NOT NULL DEFAULT 0, source TEXT NOT NULL DEFAULT 'wikidata', created_at DATETIME, updated_at DATETIME, PRIMARY KEY (book_id, screen_work_id));
CREATE TABLE shelves (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, type TEXT NOT NULL);
CREATE TABLE shelf_items (shelf_id TEXT NOT NULL, book_id TEXT NOT NULL, added_at DATETIME, PRIMARY KEY (shelf_id, book_id));
CREATE TABLE book_reads (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, book_id TEXT NOT NULL, started_at DATETIME, finished_at DATETIME, abandoned_at DATETIME, created_at DATETIME, manual_end BOOLEAN NOT NULL DEFAULT 0);
CREATE TABLE adaptation_votes (user_id TEXT NOT NULL, book_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, value TEXT NOT NULL, created_at DATETIME, updated_at DATETIME, PRIMARY KEY (user_id, book_id, screen_work_id));
`

type fixture struct {
	h    *Handler
	db   *gorm.DB
	book uuid.UUID
	work uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatal(err)
	}
	f := &fixture{
		h:    NewHandler(service.NewService(repository.NewGormRepository(db), nil, nil)),
		db:   db,
		book: uuid.New(),
		work: uuid.New(),
	}
	now := time.Now()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO books (id, title, status, created_at, updated_at) VALUES (?, 'Книга', 'published', ?, ?)", []any{f.book, now, now}},
		{"INSERT INTO screen_works (id, title, kind, created_at, updated_at) VALUES (?, 'Фильм', 'film', ?, ?)", []any{f.work, now, now}},
		{"INSERT INTO book_screen_works (book_id, screen_work_id, created_at, updated_at) VALUES (?, ?, ?, ?)", []any{f.book, f.work, now, now}},
	} {
		if err := db.Exec(q.sql, q.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f *fixture) makeReader(t *testing.T) uuid.UUID {
	t.Helper()
	user, shelf := uuid.New(), uuid.New()
	if err := f.db.Exec("INSERT INTO shelves (id, user_id, type) VALUES (?, ?, 'read')", shelf, user).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("INSERT INTO shelf_items (shelf_id, book_id, added_at) VALUES (?, ?, ?)", shelf, f.book, time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("INSERT INTO book_reads (id, user_id, book_id, finished_at, created_at) VALUES (?, ?, ?, ?, ?)", uuid.New(), user, f.book, time.Now(), time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

// call runs a handler on a route with the given params and optional user.
func call(h echo.HandlerFunc, method, body string, user *uuid.UUID, names []string, values []string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames(names...)
	c.SetParamValues(values...)
	if user != nil {
		c.Set("user_id", user.String())
	}
	_ = h(c)
	return rec
}

func errMsg(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body["error"]
}

var voteParams = []string{"id", "screen_work_id"}

func TestVoteAdaptation(t *testing.T) {
	f := newFixture(t)
	reader := f.makeReader(t)
	other := uuid.New()

	vote := func(user *uuid.UUID, book, work, body string) *httptest.ResponseRecorder {
		return call(f.h.VoteAdaptation, http.MethodPut, body, user, voteParams, []string{book, work})
	}
	b, w := f.book.String(), f.work.String()

	if rec := vote(nil, b, w, `{"value":"book"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("guest = %d", rec.Code)
	}
	if rec := vote(&reader, "x", w, `{"value":"book"}`); rec.Code != http.StatusBadRequest || errMsg(t, rec) != "invalid book id" {
		t.Errorf("bad book id = %d", rec.Code)
	}
	if rec := vote(&reader, b, "x", `{"value":"book"}`); rec.Code != http.StatusBadRequest || errMsg(t, rec) != "invalid screen work id" {
		t.Errorf("bad work id = %d", rec.Code)
	}
	if rec := vote(&reader, b, w, `{"value":"film"}`); rec.Code != http.StatusBadRequest || errMsg(t, rec) != "invalid vote value" {
		t.Errorf("bad value = %d", rec.Code)
	}
	if rec := vote(&reader, b, w, ``); rec.Code != http.StatusBadRequest || errMsg(t, rec) != "invalid vote value" {
		t.Errorf("missing value = %d", rec.Code)
	}
	if rec := vote(&other, b, w, `{"value":"book"}`); rec.Code != http.StatusForbidden || errMsg(t, rec) != "only readers who have read the book can vote" {
		t.Errorf("not a reader = %d", rec.Code)
	}
	if rec := vote(&reader, uuid.NewString(), w, `{"value":"book"}`); rec.Code != http.StatusNotFound || errMsg(t, rec) != "book not found" {
		t.Errorf("unknown book = %d", rec.Code)
	}
	if rec := vote(&reader, b, uuid.NewString(), `{"value":"book"}`); rec.Code != http.StatusNotFound || errMsg(t, rec) != "adaptation not found" {
		t.Errorf("unknown work = %d", rec.Code)
	}

	rec := vote(&reader, b, w, `{"value":"adaptation"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("vote = %d %s", rec.Code, rec.Body.String())
	}
	var got verdictResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ScreenWorkID != w || got.VotesCount != 1 || got.MyVote == nil || *got.MyVote != "adaptation" || got.BookBetter != nil {
		t.Errorf("response = %+v", got)
	}
}

func TestUnvoteAdaptation(t *testing.T) {
	f := newFixture(t)
	reader := f.makeReader(t)
	b, w := f.book.String(), f.work.String()
	unvote := func(user *uuid.UUID, book, work string) *httptest.ResponseRecorder {
		return call(f.h.UnvoteAdaptation, http.MethodDelete, "", user, voteParams, []string{book, work})
	}

	if rec := unvote(nil, b, w); rec.Code != http.StatusUnauthorized {
		t.Errorf("guest = %d", rec.Code)
	}
	if rec := unvote(&reader, "x", w); rec.Code != http.StatusBadRequest {
		t.Errorf("bad book id = %d", rec.Code)
	}
	if rec := unvote(&reader, b, "x"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad work id = %d", rec.Code)
	}
	if rec := call(f.h.VoteAdaptation, http.MethodPut, `{"value":"book"}`, &reader, voteParams, []string{b, w}); rec.Code != http.StatusOK {
		t.Fatalf("vote = %d", rec.Code)
	}
	for i := 0; i < 2; i++ {
		if rec := unvote(&reader, b, w); rec.Code != http.StatusNoContent {
			t.Errorf("unvote #%d = %d", i, rec.Code)
		}
	}
	var n int64
	f.db.Table("adaptation_votes").Count(&n)
	if n != 0 {
		t.Errorf("votes left = %d", n)
	}
}

func TestListBookVerdicts(t *testing.T) {
	f := newFixture(t)
	reader := f.makeReader(t)
	b := f.book.String()
	list := func(user *uuid.UUID, book string) *httptest.ResponseRecorder {
		return call(f.h.ListBookVerdicts, http.MethodGet, "", user, []string{"id"}, []string{book})
	}

	if rec := list(nil, "x"); rec.Code != http.StatusBadRequest || errMsg(t, rec) != "invalid book id" {
		t.Errorf("bad id = %d", rec.Code)
	}
	if rec := list(nil, uuid.NewString()); rec.Code != http.StatusNotFound || errMsg(t, rec) != "book not found" {
		t.Errorf("unknown = %d", rec.Code)
	}
	if rec := call(f.h.VoteAdaptation, http.MethodPut, `{"value":"same"}`, &reader, voteParams, []string{b, f.work.String()}); rec.Code != http.StatusOK {
		t.Fatalf("vote = %d", rec.Code)
	}

	var guest verdictsResponse
	rec := list(nil, b)
	if rec.Code != http.StatusOK {
		t.Fatalf("guest = %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &guest); err != nil {
		t.Fatal(err)
	}
	if guest.CanVote || guest.MinVotes != 5 || len(guest.Items) != 1 || guest.Items[0].MyVote != nil || guest.Items[0].VotesCount != 1 {
		t.Errorf("guest = %+v", guest)
	}

	var mine verdictsResponse
	rec = list(&reader, b)
	if err := json.Unmarshal(rec.Body.Bytes(), &mine); err != nil {
		t.Fatal(err)
	}
	if !mine.CanVote || mine.Items[0].MyVote == nil || *mine.Items[0].MyVote != "same" {
		t.Errorf("reader = %+v", mine)
	}
	if !strings.Contains(rec.Body.String(), `"book_better":null`) {
		t.Errorf("counts must be null below the minimum: %s", rec.Body.String())
	}
}
