package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	authDomain "github.com/bookhaven/backend/internal/modules/auth/domain"
	"github.com/labstack/echo/v4"
)

func newAuthTestContext() (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func findSetCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// TestSetAuthCookies_SetsHasSessionAlongsideRefreshToken covers the
// Register/Login/Refresh path: all three handlers call setAuthCookies, so
// exercising the shared helper directly asserts the invariant for all of them.
func TestSetAuthCookies_SetsHasSessionAlongsideRefreshToken(t *testing.T) {
	c, rec := newAuthTestContext()

	setAuthCookies(c, &authDomain.TokenPair{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresIn:    900,
	})

	hasSession := findSetCookie(rec, "has_session")
	if hasSession == nil {
		t.Fatal("expected has_session cookie to be set")
	}
	if hasSession.Value != "1" {
		t.Errorf("expected has_session value %q, got %q", "1", hasSession.Value)
	}
	if !hasSession.HttpOnly {
		t.Error("expected has_session to be HttpOnly")
	}
	if hasSession.Path != "/" {
		t.Errorf("expected has_session path %q, got %q", "/", hasSession.Path)
	}

	refreshToken := findSetCookie(rec, "refresh_token")
	if refreshToken == nil {
		t.Fatal("expected refresh_token cookie to be set")
	}

	if hasSession.MaxAge != refreshToken.MaxAge {
		t.Errorf("expected has_session MaxAge (%d) to match refresh_token MaxAge (%d)", hasSession.MaxAge, refreshToken.MaxAge)
	}
}

// TestSetAuthCookies_NilTokens ensures no cookies are set (and no panic) when
// tokens is nil, matching the existing early-return behavior.
func TestSetAuthCookies_NilTokens(t *testing.T) {
	c, rec := newAuthTestContext()

	setAuthCookies(c, nil)

	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("expected no cookies to be set for nil tokens, got %d", len(rec.Result().Cookies()))
	}
}

// TestClearAuthCookies_ClearsHasSession covers the Logout path.
func TestClearAuthCookies_ClearsHasSession(t *testing.T) {
	c, rec := newAuthTestContext()

	clearAuthCookies(c)

	for _, name := range []string{"access_token", "refresh_token", "has_session"} {
		cookie := findSetCookie(rec, name)
		if cookie == nil {
			t.Fatalf("expected %s cookie to be cleared (Set-Cookie present)", name)
		}
		if cookie.Value != "" {
			t.Errorf("expected %s cleared value to be empty, got %q", name, cookie.Value)
		}
		if cookie.MaxAge != -1 {
			t.Errorf("expected %s cleared MaxAge to be -1, got %d", name, cookie.MaxAge)
		}
	}
}
