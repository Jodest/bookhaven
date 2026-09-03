package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	listsDomain "github.com/bookhaven/backend/internal/modules/lists/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	usersService "github.com/bookhaven/backend/internal/modules/users/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type fakeUserService struct {
	users map[uuid.UUID]*usersDomain.User
}

func newFakeUserService() *fakeUserService {
	return &fakeUserService{users: make(map[uuid.UUID]*usersDomain.User)}
}

func (f *fakeUserService) GetByID(ctx context.Context, id uuid.UUID) (*usersDomain.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, usersService.ErrUserNotFound
	}
	copy := *u
	return &copy, nil
}

func (f *fakeUserService) UpdateMe(ctx context.Context, userID uuid.UUID, name, avatarURL, profileType *string) (*usersDomain.User, error) {
	u, ok := f.users[userID]
	if !ok {
		return nil, usersService.ErrUserNotFound
	}
	copy := *u
	if name != nil {
		n := strings.TrimSpace(*name)
		if l := utf8.RuneCountInString(n); l < 2 || l > 255 {
			return nil, usersService.ErrInvalidName
		}
		copy.Name = &n
	}
	if avatarURL != nil {
		a := strings.TrimSpace(*avatarURL)
		if a != "" && !isValidHTTPURL(a) {
			return nil, usersService.ErrInvalidAvatarURL
		}
		if a == "" {
			copy.AvatarURL = nil
		} else {
			copy.AvatarURL = &a
		}
	}
	if profileType != nil {
		pt := *profileType
		switch pt {
		case "reader", "librarian", "bookseller", "other":
			copy.ProfileType = usersDomain.ProfileType(pt)
		default:
			return nil, usersService.ErrInvalidProfileType
		}
	}
	copy.UpdatedAt = time.Now()
	f.users[userID] = &copy
	return &copy, nil
}

func isValidHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

type fakeListService struct {
	lists map[uuid.UUID][]listsDomain.List
}

func newFakeListService() *fakeListService {
	return &fakeListService{lists: make(map[uuid.UUID][]listsDomain.List)}
}

func (f *fakeListService) ListPublicByUser(ctx context.Context, userID uuid.UUID) ([]listsDomain.List, error) {
	return f.lists[userID], nil
}

func setupHandler() (*Handler, *fakeUserService, *fakeListService) {
	userSvc := newFakeUserService()
	listSvc := newFakeListService()
	return NewHandler(userSvc, listSvc), userSvc, listSvc
}

func newContext(method, target string, body []byte) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestGetUser_PublicProfile(t *testing.T) {
	h, userSvc, listSvc := setupHandler()
	name := "Alice"
	user := &usersDomain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: usersDomain.ProfileTypeReader, CreatedAt: time.Now()}
	userSvc.users[user.ID] = user

	listSvc.lists[user.ID] = []listsDomain.List{
		{ID: uuid.New(), UserID: user.ID, Title: "Публичный", Description: "Описание", IsPublic: true, BooksCount: 3, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	c, rec := newContext(http.MethodGet, "/api/v1/users/"+user.ID.String(), nil)
	c.SetPath("/api/v1/users/:id")
	c.SetParamNames("id")
	c.SetParamValues(user.ID.String())

	if err := h.GetUser(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body publicProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.User.ID != user.ID.String() {
		t.Fatal("expected user id in response")
	}
	if len(body.Lists) != 1 {
		t.Fatalf("expected 1 public list, got %d", len(body.Lists))
	}
}

func TestGetUser_MissingID(t *testing.T) {
	h, _, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/users/"+uuid.New().String(), nil)
	c.SetPath("/api/v1/users/:id")
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())

	if err := h.GetUser(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetUser_InvalidID(t *testing.T) {
	h, _, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/users/bad-id", nil)
	c.SetPath("/api/v1/users/:id")
	c.SetParamNames("id")
	c.SetParamValues("bad-id")

	if err := h.GetUser(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetUser_PrivacyNoEmail(t *testing.T) {
	h, userSvc, _ := setupHandler()
	name := "Alice"
	user := &usersDomain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: usersDomain.ProfileTypeReader, CreatedAt: time.Now()}
	userSvc.users[user.ID] = user

	c, rec := newContext(http.MethodGet, "/api/v1/users/"+user.ID.String(), nil)
	c.SetPath("/api/v1/users/:id")
	c.SetParamNames("id")
	c.SetParamValues(user.ID.String())

	if err := h.GetUser(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	var userObj map[string]interface{}
	if err := json.Unmarshal(raw["user"], &userObj); err != nil {
		t.Fatalf("failed to decode user: %v", err)
	}
	if _, ok := userObj["email"]; ok {
		t.Fatal("public profile must not expose email")
	}
	if _, ok := userObj["role"]; ok {
		t.Fatal("public profile must not expose role")
	}
	if userObj["name"] != "Alice" {
		t.Fatalf("expected name Alice, got %v", userObj["name"])
	}
}

func TestUpdateMe_RequiresAuth(t *testing.T) {
	h, _, _ := setupHandler()
	c, rec := newContext(http.MethodPatch, "/api/v1/users/me", []byte(`{"name":"New"}`))

	if err := h.UpdateMe(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestUpdateMe_OwnerUpdate(t *testing.T) {
	h, userSvc, _ := setupHandler()
	name := "Alice"
	user := &usersDomain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: usersDomain.ProfileTypeReader}
	userSvc.users[user.ID] = user

	c, rec := newContext(http.MethodPatch, "/api/v1/users/me", []byte(`{"name":"New Name","avatar_url":"https://example.com/a.png","profile_type":"other"}`))
	c.Set("user_id", user.ID.String())

	if err := h.UpdateMe(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body ownerUserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Name == nil || *body.Name != "New Name" {
		t.Fatalf("expected name New Name, got %v", body.Name)
	}
	if body.AvatarURL == nil || *body.AvatarURL != "https://example.com/a.png" {
		t.Fatal("expected avatar to be updated")
	}
	if body.ProfileType != "other" {
		t.Fatalf("expected profile type other, got %s", body.ProfileType)
	}
	if body.Email != user.Email {
		t.Fatal("owner response should include email")
	}
}

func TestUpdateMe_ClearAvatarWithNull(t *testing.T) {
	h, userSvc, _ := setupHandler()
	name := "Alice"
	avatar := "https://example.com/avatar.jpg"
	user := &usersDomain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, AvatarURL: &avatar, ProfileType: usersDomain.ProfileTypeReader}
	userSvc.users[user.ID] = user

	c, rec := newContext(http.MethodPatch, "/api/v1/users/me", []byte(`{"avatar_url":null}`))
	c.Set("user_id", user.ID.String())

	if err := h.UpdateMe(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if userSvc.users[user.ID].AvatarURL != nil {
		t.Fatalf("expected avatar to be cleared, got %v", *userSvc.users[user.ID].AvatarURL)
	}
}

func TestUpdateMe_OmittedAvatarUnchanged(t *testing.T) {
	h, userSvc, _ := setupHandler()
	name := "Alice"
	avatar := "https://example.com/avatar.jpg"
	user := &usersDomain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, AvatarURL: &avatar, ProfileType: usersDomain.ProfileTypeReader}
	userSvc.users[user.ID] = user

	c, rec := newContext(http.MethodPatch, "/api/v1/users/me", []byte(`{"name":"New Name"}`))
	c.Set("user_id", user.ID.String())

	if err := h.UpdateMe(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if userSvc.users[user.ID].AvatarURL == nil || *userSvc.users[user.ID].AvatarURL != avatar {
		t.Fatal("expected avatar to remain unchanged when omitted")
	}
}

func TestUpdateMe_ValidationError(t *testing.T) {
	h, userSvc, _ := setupHandler()
	name := "Alice"
	user := &usersDomain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: usersDomain.ProfileTypeReader}
	userSvc.users[user.ID] = user

	c, rec := newContext(http.MethodPatch, "/api/v1/users/me", []byte(`{"name":"a","avatar_url":"not-url","profile_type":"bot"}`))
	c.Set("user_id", user.ID.String())

	if err := h.UpdateMe(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateMe_IDORAttempt(t *testing.T) {
	h, userSvc, _ := setupHandler()
	ownerID := uuid.New()
	attackerID := uuid.New()
	name := "Alice"
	userSvc.users[ownerID] = &usersDomain.User{ID: ownerID, Email: "alice@example.com", Name: &name, ProfileType: usersDomain.ProfileTypeReader}
	attackerName := "Bob"
	userSvc.users[attackerID] = &usersDomain.User{ID: attackerID, Email: "bob@example.com", Name: &attackerName, ProfileType: usersDomain.ProfileTypeReader}

	// Handler only uses user_id from context, so sending attacker token can only update attacker.
	c, rec := newContext(http.MethodPatch, "/api/v1/users/me", []byte(`{"name":"Hacked"}`))
	c.Set("user_id", attackerID.String())

	if err := h.UpdateMe(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if *userSvc.users[ownerID].Name != "Alice" {
		t.Fatal("owner profile must not be modified by another user")
	}
}
