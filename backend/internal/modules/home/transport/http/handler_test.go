package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	catalogdomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/home/repository"
	"github.com/bookhaven/backend/internal/modules/home/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type fakeHomeService struct {
	home       *service.Home
	homeErr    error
	random     *catalogdomain.Book
	randomErr  error
	gotGenre   *uuid.UUID
	gotExclude *uuid.UUID

	personal    *service.PersonalHome
	personalErr error
	gotUserID   *uuid.UUID
}

func (f *fakeHomeService) GetPersonalHome(_ context.Context, userID uuid.UUID) (*service.PersonalHome, error) {
	f.gotUserID = &userID
	return f.personal, f.personalErr
}

func (f *fakeHomeService) GetHome(context.Context) (*service.Home, error) {
	return f.home, f.homeErr
}

func (f *fakeHomeService) RandomBook(_ context.Context, genreID, excludeID *uuid.UUID) (*catalogdomain.Book, error) {
	f.gotGenre, f.gotExclude = genreID, excludeID
	return f.random, f.randomErr
}

func serve(t *testing.T, h echo.HandlerFunc, target string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	if err := h(e.NewContext(req, rec)); err != nil {
		t.Fatalf("handler: %v", err)
	}
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestGetHome_EmptyShape(t *testing.T) {
	svc := &fakeHomeService{home: &service.Home{
		Date:          "2026-10-04",
		NewBooks:      []catalogdomain.Book{},
		TopRated:      []catalogdomain.Book{},
		RecentReviews: []service.RecentReview{},
	}}
	rec := serve(t, NewHandler(svc).GetHome, "/api/v1/home")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := decode(t, rec)
	if body["date"] != "2026-10-04" {
		t.Fatalf("unexpected date %v", body["date"])
	}
	for _, k := range []string{"book_of_the_day", "calendar"} {
		v, ok := body[k]
		if !ok || v != nil {
			t.Fatalf("expected %s: null, got %v (present=%v)", k, v, ok)
		}
	}
	for _, k := range []string{"new_books", "top_rated", "recent_reviews"} {
		arr, ok := body[k].([]interface{})
		if !ok || len(arr) != 0 {
			t.Fatalf("expected %s: [], got %v", k, body[k])
		}
	}
}

func TestGetHome_FullShapeSnakeCase(t *testing.T) {
	bookID := uuid.New()
	cover := "https://example.com/c.jpg"
	excerpt := "Описание"
	book := catalogdomain.Book{
		ID: bookID, Title: "Книга", Slug: "kniga", CoverURL: &cover, Rating: 4.5, RatingsCount: 3,
		Authors: []catalogdomain.Author{{ID: uuid.New(), Name: "Автор", Slug: "avtor"}},
	}
	birth := time.Date(1814, 10, 15, 0, 0, 0, 0, time.UTC)
	death := time.Date(1841, 7, 27, 0, 0, 0, 0, time.UTC)
	name := "Анна"
	svc := &fakeHomeService{home: &service.Home{
		Date:         "2026-10-04",
		BookOfTheDay: &service.BookOfTheDay{Book: book, DescriptionExcerpt: &excerpt},
		Calendar: &service.Calendar{
			Date: "2026-10-15", IsToday: false,
			Events: []service.CalendarEvent{{
				Type: "birth", Years: 212,
				Author: repository.CalendarEventRow{AuthorID: uuid.New(), Name: "Михаил Лермонтов", Slug: "mikhail-lermontov", BirthDate: &birth, DeathDate: &death},
				Books:  []catalogdomain.Book{book},
			}},
			Anniversaries: []service.Anniversary{{Years: 100, Book: book}},
		},
		NewBooks: []catalogdomain.Book{book},
		TopRated: []catalogdomain.Book{},
		RecentReviews: []service.RecentReview{{
			ID: uuid.New(), Excerpt: "Текст", Rating: 5, LikesCount: 2, CreatedAt: time.Now(),
			User: service.ReviewUser{ID: uuid.New(), Name: &name},
			Book: book,
		}},
	}}
	body := decode(t, serve(t, NewHandler(svc).GetHome, "/api/v1/home"))

	botd := body["book_of_the_day"].(map[string]interface{})
	if botd["description_excerpt"] != "Описание" || botd["book"].(map[string]interface{})["cover_url"] != cover {
		t.Fatalf("unexpected book_of_the_day %v", botd)
	}
	cal := body["calendar"].(map[string]interface{})
	if cal["is_today"] != false || cal["date"] != "2026-10-15" {
		t.Fatalf("unexpected calendar %v", cal)
	}
	ev := cal["events"].([]interface{})[0].(map[string]interface{})
	author := ev["author"].(map[string]interface{})
	if author["birth_date"] != "1814-10-15" || author["death_date"] != "1841-07-27" || author["photo_url"] != nil {
		t.Fatalf("unexpected author %v", author)
	}
	if ev["years"].(float64) != 212 || ev["type"] != "birth" {
		t.Fatalf("unexpected event %v", ev)
	}
	ann := cal["anniversaries"].([]interface{})[0].(map[string]interface{})
	if ann["years"].(float64) != 100 {
		t.Fatalf("unexpected anniversary %v", ann)
	}
	rv := body["recent_reviews"].([]interface{})[0].(map[string]interface{})
	if rv["likes_count"].(float64) != 2 || rv["created_at"] == nil {
		t.Fatalf("unexpected review %v", rv)
	}
	user := rv["user"].(map[string]interface{})
	if user["name"] != "Анна" {
		t.Fatalf("unexpected user %v", user)
	}
	if _, ok := user["avatar_url"]; !ok {
		t.Fatal("avatar_url must be present (null)")
	}
	rb := rv["book"].(map[string]interface{})
	if rb["slug"] != "kniga" || rb["cover_url"] != cover {
		t.Fatalf("unexpected review book %v", rb)
	}
	if _, ok := rv["content"]; ok {
		t.Fatal("full content must never be sent")
	}
	authors := rb["authors"].([]interface{})
	if a := authors[0].(map[string]interface{}); a["name"] != "Автор" || a["slug"] != "avtor" {
		t.Fatalf("unexpected review book authors %v", authors)
	}
}

func TestGetHome_ServiceError500(t *testing.T) {
	svc := &fakeHomeService{homeErr: service.ErrAllBlocksFailed}
	rec := serve(t, NewHandler(svc).GetHome, "/api/v1/home")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestRandomBook_BadUUID(t *testing.T) {
	for _, target := range []string{"/api/v1/home/random?genre_id=nope", "/api/v1/home/random?exclude_id=123"} {
		rec := serve(t, NewHandler(&fakeHomeService{}).RandomBook, target)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", target, rec.Code)
		}
	}
}

func TestRandomBook_NullAndPassThrough(t *testing.T) {
	svc := &fakeHomeService{}
	genre, exclude := uuid.New(), uuid.New()
	rec := serve(t, NewHandler(svc).RandomBook, "/api/v1/home/random?genre_id="+genre.String()+"&exclude_id="+exclude.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := decode(t, rec)
	if v, ok := body["book"]; !ok || v != nil {
		t.Fatalf("expected book: null, got %v", body)
	}
	if svc.gotGenre == nil || *svc.gotGenre != genre || svc.gotExclude == nil || *svc.gotExclude != exclude {
		t.Fatal("genre/exclude not passed through")
	}
}

func TestRandomBook_Book(t *testing.T) {
	id := uuid.New()
	svc := &fakeHomeService{random: &catalogdomain.Book{ID: id, Title: "Книга", Slug: "kniga"}}
	body := decode(t, serve(t, NewHandler(svc).RandomBook, "/api/v1/home/random"))
	book := body["book"].(map[string]interface{})
	if book["id"] != id.String() || book["slug"] != "kniga" {
		t.Fatalf("unexpected book %v", book)
	}
	if svc.gotGenre != nil || svc.gotExclude != nil {
		t.Fatal("expected nil filters without query params")
	}

	svc.randomErr = errors.New("db")
	rec := serve(t, NewHandler(svc).RandomBook, "/api/v1/home/random")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func serveAs(t *testing.T, h echo.HandlerFunc, target, userID string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if userID != "" {
		c.Set("user_id", userID)
	}
	if err := h(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	return rec
}

func TestGetMyHome_401WithoutUser(t *testing.T) {
	svc := &fakeHomeService{}
	for _, user := range []string{"", "not-a-uuid"} {
		rec := serveAs(t, NewHandler(svc).GetMyHome, "/api/v1/home/me", user)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("user %q: expected 401, got %d", user, rec.Code)
		}
	}
	if svc.gotUserID != nil {
		t.Fatal("service must not be called without a user")
	}
}

func TestGetMyHome_UsesContextUserAndEmptyShape(t *testing.T) {
	user := uuid.New()
	svc := &fakeHomeService{personal: &service.PersonalHome{Recommendations: []catalogdomain.Book{}}}
	// A user_id query parameter must be ignored.
	rec := serveAs(t, NewHandler(svc).GetMyHome, "/api/v1/home/me?user_id="+uuid.New().String(), user.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.gotUserID == nil || *svc.gotUserID != user {
		t.Fatalf("expected the context user %s, got %v", user, svc.gotUserID)
	}
	body := decode(t, rec)
	for _, k := range []string{"continue_reading", "want_to_read_pick"} {
		if v, ok := body[k]; !ok || v != nil {
			t.Fatalf("expected %s: null, got %v (present=%v)", k, v, ok)
		}
	}
	if arr, ok := body["recommendations"].([]interface{}); !ok || len(arr) != 0 {
		t.Fatalf("expected recommendations: [], got %v", body["recommendations"])
	}
}

func TestGetMyHome_FullShape(t *testing.T) {
	shelf, wtr := uuid.New(), uuid.New()
	book := catalogdomain.Book{ID: uuid.New(), Title: "Книга", Slug: "kniga", RatingsCount: 2}
	svc := &fakeHomeService{personal: &service.PersonalHome{
		ContinueReading: &service.ContinueReading{ShelfID: shelf, Total: 7, Books: []catalogdomain.Book{book}},
		Recommendations: []catalogdomain.Book{book},
		WantToReadPick:  &service.WantToReadPick{ShelfID: wtr, Total: 14},
	}}
	body := decode(t, serveAs(t, NewHandler(svc).GetMyHome, "/api/v1/home/me", uuid.New().String()))

	cr := body["continue_reading"].(map[string]interface{})
	if cr["shelf_id"] != shelf.String() || cr["total"].(float64) != 7 {
		t.Fatalf("unexpected continue_reading %v", cr)
	}
	if b := cr["books"].([]interface{})[0].(map[string]interface{}); b["slug"] != "kniga" || b["ratings_count"].(float64) != 2 {
		t.Fatalf("unexpected book %v", b)
	}
	pick := body["want_to_read_pick"].(map[string]interface{})
	if pick["shelf_id"] != wtr.String() || pick["total"].(float64) != 14 {
		t.Fatalf("unexpected want_to_read_pick %v", pick)
	}
	if v, ok := pick["book"]; !ok || v != nil {
		t.Fatalf("expected book: null, got %v", pick)
	}
	if len(body["recommendations"].([]interface{})) != 1 {
		t.Fatalf("unexpected recommendations %v", body["recommendations"])
	}

	svc.personalErr = errors.New("db")
	if rec := serveAs(t, NewHandler(svc).GetMyHome, "/api/v1/home/me", uuid.New().String()); rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
