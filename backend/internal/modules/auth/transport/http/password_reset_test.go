package http

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	authDomain "github.com/bookhaven/backend/internal/modules/auth/domain"
	"github.com/google/uuid"
)

// Handler tests for password reset (docs/features/password-reset.md).

// insertResetToken stores a password reset token for a password user
// registered via registerUser.
func (e *verificationHandlerEnv) insertResetToken(t *testing.T, userID uuid.UUID, email, plain string, expiresAt time.Time, usedAt *time.Time) {
	t.Helper()
	row := &authDomain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    userID,
		Email:     email,
		TokenHash: sha256Hex(plain),
		ExpiresAt: expiresAt,
		UsedAt:    usedAt,
		CreatedAt: time.Now(),
	}
	if err := e.db.Create(row).Error; err != nil {
		t.Fatalf("insert reset token: %v", err)
	}
}

func (e *verificationHandlerEnv) resetTokenUsed(t *testing.T, plain string) bool {
	t.Helper()
	var row authDomain.PasswordResetToken
	if err := e.db.Where("token_hash = ?", sha256Hex(plain)).First(&row).Error; err != nil {
		t.Fatalf("find reset token: %v", err)
	}
	return row.UsedAt != nil
}

func TestForgotPasswordHandler(t *testing.T) {
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
			name:       "known user",
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
			name:       "account without password",
			body:       `{"email":"anna@example.com"}`,
			wantStatus: http.StatusAccepted, wantKey: "status", wantValue: "sent", wantEmails: 0,
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
			name:       "malformed body",
			body:       `{"email":`,
			wantStatus: http.StatusBadRequest, wantKey: "code", wantValue: "invalid_email",
		},
		{
			name: "ip rate limited",
			setup: func(t *testing.T, env *verificationHandlerEnv) {
				env.resetIPLimiter.allowed = false
				env.resetIPLimiter.retryAfter = 41500 * time.Millisecond
			},
			body:       `{"email":"petr@example.com"}`,
			wantStatus: http.StatusTooManyRequests, wantKey: "code", wantValue: "rate_limited",
		},
		{
			name: "email rate limited",
			setup: func(t *testing.T, env *verificationHandlerEnv) {
				env.resetEmailLimiter.allowed = false
				env.resetEmailLimiter.retryAfter = 41500 * time.Millisecond
			},
			body:       `{"email":"petr@example.com"}`,
			wantStatus: http.StatusTooManyRequests, wantKey: "code", wantValue: "rate_limited",
		},
		{
			name:       "limiter error",
			setup:      func(t *testing.T, env *verificationHandlerEnv) { env.resetIPLimiter.err = errors.New("redis down") },
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
			c, rec := postJSON("/api/v1/auth/forgot-password", tc.body)
			if err := env.h.ForgotPassword(c); err != nil {
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
			if rec.Code == http.StatusBadRequest && (env.resetIPLimiter.calls != 0 || env.resetEmailLimiter.calls != 0) {
				t.Fatal("invalid requests must not consume rate-limit quota")
			}
			if env.ipLimiter.calls != 0 || env.emailLimiter.calls != 0 {
				t.Fatal("forgot-password must not use the resend-verification limiters")
			}
		})
	}
}

func TestForgotPasswordHandler_UsesRealIP(t *testing.T) {
	env := setupVerificationHandler(t)
	c, _ := postJSON("/api/v1/auth/forgot-password", `{"email":"petr@example.com"}`)
	c.Echo().IPExtractor = func(r *http.Request) string { return "198.51.100.9" }
	if err := env.h.ForgotPassword(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if env.resetIPLimiter.lastKey != "198.51.100.9" {
		t.Fatalf("ip limiter key = %q, want the client IP", env.resetIPLimiter.lastKey)
	}
}

func TestResetPasswordHandler_Errors(t *testing.T) {
	valid := strings.Repeat("a", 64)
	cases := []struct {
		name       string
		expiresIn  time.Duration
		used       bool
		email      string // token email; "" = the user's email
		body       string
		wantCode   string
		wantUnused bool // token must still be unused afterwards
	}{
		{name: "malformed body", body: `{"token":`, wantCode: "invalid_token"},
		{name: "short password", expiresIn: time.Hour, body: `{"token":"` + valid + `","password":"short"}`, wantCode: "invalid_password", wantUnused: true},
		{name: "missing password", expiresIn: time.Hour, body: `{"token":"` + valid + `"}`, wantCode: "invalid_password", wantUnused: true},
		{name: "malformed token", body: `{"token":"abc","password":"new-password-456"}`, wantCode: "invalid_token"},
		{name: "unknown token", body: `{"token":"` + strings.Repeat("b", 64) + `","password":"new-password-456"}`, wantCode: "invalid_token"},
		{name: "email mismatch", expiresIn: time.Hour, email: "old@example.com", body: `{"token":"` + valid + `","password":"new-password-456"}`, wantCode: "invalid_token", wantUnused: true},
		{name: "expired token", expiresIn: -time.Minute, body: `{"token":"` + valid + `","password":"new-password-456"}`, wantCode: "token_expired", wantUnused: true},
		{name: "used token", expiresIn: time.Hour, used: true, body: `{"token":"` + valid + `","password":"new-password-456"}`, wantCode: "token_used"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupVerificationHandler(t)
			user := env.registerUser(t, "petr@example.com")
			if tc.expiresIn != 0 {
				var usedAt *time.Time
				if tc.used {
					now := time.Now()
					usedAt = &now
				}
				email := tc.email
				if email == "" {
					email = user.Email
				}
				env.insertResetToken(t, user.ID, email, valid, time.Now().Add(tc.expiresIn), usedAt)
			}

			c, rec := postJSON("/api/v1/auth/reset-password", tc.body)
			if err := env.h.ResetPassword(c); err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
			}
			body := decodeBody(t, rec)
			if body["code"] != tc.wantCode || body["error"] == nil {
				t.Fatalf("unexpected body %s, want code %s", rec.Body.String(), tc.wantCode)
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Fatal("failed reset must not touch cookies")
			}
			if tc.wantUnused && env.resetTokenUsed(t, valid) {
				t.Fatal("token must stay unused")
			}
		})
	}
}

func TestResetPasswordHandler_Success(t *testing.T) {
	env := setupVerificationHandler(t)
	user := env.registerUser(t, "petr@example.com")
	valid := strings.Repeat("c", 64)
	env.insertResetToken(t, user.ID, user.Email, valid, time.Now().Add(time.Hour), nil)

	// No user_id in the context: the endpoint is public.
	c, rec := postJSON("/api/v1/auth/reset-password", `{"token":"`+valid+`","password":"new-password-456"}`)
	if err := env.h.ResetPassword(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["status"] != "password_reset" {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
	for _, name := range []string{"access_token", "refresh_token", "has_session"} {
		cookie := findSetCookie(rec, name)
		if cookie == nil || cookie.MaxAge != -1 {
			t.Fatalf("expected %s to be cleared, got %+v", name, cookie)
		}
	}
	if !env.resetTokenUsed(t, valid) {
		t.Fatal("token must be marked used")
	}

	// The new password logs in (the reset also verified the email).
	lc, lrec := postJSON("/api/v1/auth/login", `{"email":"petr@example.com","password":"new-password-456"}`)
	if err := env.h.Login(lc); err != nil {
		t.Fatalf("login handler error: %v", err)
	}
	if lrec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body %s)", lrec.Code, lrec.Body.String())
	}
}
