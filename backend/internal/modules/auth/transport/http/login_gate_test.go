package http

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
)

// Handler tests for the email verification login gate
// (docs/features/email-verification-login-gate.md).

const gatePassword = "password123"

// registerUser creates a password user via the service (unverified).
func (e *verificationHandlerEnv) registerUser(t *testing.T, email string) *usersDomain.User {
	t.Helper()
	user, err := e.svc.Register(context.Background(), email, gatePassword, "Пётр", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return user
}

func (e *verificationHandlerEnv) verify(t *testing.T, email string) {
	t.Helper()
	if err := e.db.Exec("UPDATE users SET email_verified_at = ? WHERE email = ?", time.Now(), email).Error; err != nil {
		t.Fatalf("mark verified: %v", err)
	}
}

func TestRegisterHandler_NoSession(t *testing.T) {
	env := setupVerificationHandler(t)
	c, rec := postJSON("/api/v1/auth/register", `{"email":"petr@example.com","password":"password123","name":"Пётр"}`)
	if err := env.h.Register(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["email"] != "petr@example.com" || body["verification_required"] != true {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
	if _, ok := body["access_token"]; ok {
		t.Fatal("register must not return tokens")
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("register must not set cookies")
	}
	if len(env.mailer.messages) != 1 {
		t.Fatalf("expected the verification email, got %d", len(env.mailer.messages))
	}
}

func TestLoginHandler_Gate(t *testing.T) {
	cases := []struct {
		name       string
		verified   bool
		password   string
		wantStatus int
		wantCode   any
		wantCookie bool
	}{
		{name: "unverified, correct password", password: gatePassword, wantStatus: http.StatusForbidden, wantCode: "email_not_verified"},
		{name: "unverified, wrong password", password: "wrong-password", wantStatus: http.StatusUnauthorized, wantCode: nil},
		{name: "verified", verified: true, password: gatePassword, wantStatus: http.StatusOK, wantCode: nil, wantCookie: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupVerificationHandler(t)
			env.registerUser(t, "petr@example.com")
			if tc.verified {
				env.verify(t, "petr@example.com")
			}
			c, rec := postJSON("/api/v1/auth/login", `{"email":"petr@example.com","password":"`+tc.password+`"}`)
			if err := env.h.Login(c); err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			body := decodeBody(t, rec)
			if body["code"] != tc.wantCode {
				t.Fatalf("code = %v, want %v", body["code"], tc.wantCode)
			}
			if got := len(rec.Result().Cookies()) > 0; got != tc.wantCookie {
				t.Fatalf("cookies set = %v, want %v", got, tc.wantCookie)
			}
		})
	}
}

func TestRefreshHandler_UnverifiedClearsCookies(t *testing.T) {
	env := setupVerificationHandler(t)
	user := env.registerUser(t, "petr@example.com")
	tokens, err := env.svc.CreateTokenPair(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("create token pair: %v", err)
	}

	c, rec := postJSON("/api/v1/auth/refresh", `{"refresh_token":"`+tokens.RefreshToken+`"}`)
	if err := env.h.Refresh(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %s)", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["code"] != "email_not_verified" {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
	for _, name := range []string{"access_token", "refresh_token", "has_session"} {
		cookie := findSetCookie(rec, name)
		if cookie == nil || cookie.MaxAge != -1 {
			t.Fatalf("expected %s to be cleared, got %+v", name, cookie)
		}
	}
}

func TestResendVerificationHandler(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, env *verificationHandlerEnv)
		body       string
		wantStatus int
		wantKey    string
		wantValue  string
		wantEmails int
	}{
		{
			name:       "unverified user",
			setup:      func(t *testing.T, env *verificationHandlerEnv) { env.registerUser(t, "petr@example.com") },
			body:       `{"email":"petr@example.com"}`,
			wantStatus: http.StatusAccepted, wantKey: "status", wantValue: "sent", wantEmails: 2,
		},
		{
			name:       "unknown email",
			body:       `{"email":"nobody@example.com"}`,
			wantStatus: http.StatusAccepted, wantKey: "status", wantValue: "sent", wantEmails: 0,
		},
		{
			name: "verified user",
			setup: func(t *testing.T, env *verificationHandlerEnv) {
				env.registerUser(t, "petr@example.com")
				env.verify(t, "petr@example.com")
			},
			body:       `{"email":"petr@example.com"}`,
			wantStatus: http.StatusAccepted, wantKey: "status", wantValue: "sent", wantEmails: 1,
		},
		{
			name: "enqueue failure is not revealed",
			setup: func(t *testing.T, env *verificationHandlerEnv) {
				env.registerUser(t, "petr@example.com")
				env.mailer.err = errors.New("redis down")
			},
			body:       `{"email":"petr@example.com"}`,
			wantStatus: http.StatusAccepted, wantKey: "status", wantValue: "sent", wantEmails: 1,
		},
		{
			name:       "invalid email",
			body:       `{"email":"not-an-email"}`,
			wantStatus: http.StatusBadRequest, wantKey: "code", wantValue: "invalid_email",
		},
		{
			name:       "missing email",
			body:       `{}`,
			wantStatus: http.StatusBadRequest, wantKey: "code", wantValue: "invalid_email",
		},
		{
			name:       "malformed body",
			body:       `{"email":`,
			wantStatus: http.StatusBadRequest, wantKey: "code", wantValue: "invalid_email",
		},
		{
			name: "ip rate limited",
			setup: func(t *testing.T, env *verificationHandlerEnv) {
				env.ipLimiter.allowed = false
				env.ipLimiter.retryAfter = 41500 * time.Millisecond
			},
			body:       `{"email":"petr@example.com"}`,
			wantStatus: http.StatusTooManyRequests, wantKey: "code", wantValue: "rate_limited",
		},
		{
			name: "email rate limited",
			setup: func(t *testing.T, env *verificationHandlerEnv) {
				env.emailLimiter.allowed = false
				env.emailLimiter.retryAfter = 41500 * time.Millisecond
			},
			body:       `{"email":"petr@example.com"}`,
			wantStatus: http.StatusTooManyRequests, wantKey: "code", wantValue: "rate_limited",
		},
		{
			name:       "limiter error",
			setup:      func(t *testing.T, env *verificationHandlerEnv) { env.ipLimiter.err = errors.New("redis down") },
			body:       `{"email":"petr@example.com"}`,
			wantStatus: http.StatusInternalServerError, wantKey: "code", wantValue: "email_send_failed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupVerificationHandler(t)
			if tc.setup != nil {
				tc.setup(t, env)
			}
			// No user_id in the context: the endpoint is public.
			c, rec := postJSON("/api/v1/auth/resend-verification", tc.body)
			if err := env.h.ResendVerification(c); err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			body := decodeBody(t, rec)
			if body[tc.wantKey] != tc.wantValue {
				t.Fatalf("%s = %v, want %s (body %s)", tc.wantKey, body[tc.wantKey], tc.wantValue, rec.Body.String())
			}
			if rec.Code >= 400 && body["error"] == nil {
				t.Fatalf("error responses must keep the error field: %s", rec.Body.String())
			}
			if rec.Code == http.StatusTooManyRequests {
				if got := rec.Header().Get("Retry-After"); got != "42" {
					t.Fatalf("Retry-After = %q, want 42", got)
				}
				if body["retry_after_seconds"] != float64(42) {
					t.Fatalf("retry_after_seconds = %v, want 42", body["retry_after_seconds"])
				}
			}
			if rec.Code == http.StatusAccepted && len(env.mailer.messages) != tc.wantEmails {
				t.Fatalf("emails = %d, want %d", len(env.mailer.messages), tc.wantEmails)
			}
			if rec.Code == http.StatusBadRequest && (env.ipLimiter.calls != 0 || env.emailLimiter.calls != 0) {
				t.Fatal("invalid requests must not consume rate-limit quota")
			}
		})
	}
}

func TestResendVerificationHandler_UsesRealIP(t *testing.T) {
	env := setupVerificationHandler(t)
	c, _ := postJSON("/api/v1/auth/resend-verification", `{"email":"petr@example.com"}`)
	c.Echo().IPExtractor = func(r *http.Request) string { return "198.51.100.9" }
	if err := env.h.ResendVerification(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if env.ipLimiter.lastKey != "198.51.100.9" {
		t.Fatalf("ip limiter key = %q, want the client IP", env.ipLimiter.lastKey)
	}
}
