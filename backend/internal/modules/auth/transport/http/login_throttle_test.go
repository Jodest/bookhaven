package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/service"
)

// loginStubLimiter is a service.LoginLimiter returning a fixed Allow answer.
type loginStubLimiter struct {
	lastKey    string
	allowed    bool
	retryAfter time.Duration
}

func (l *loginStubLimiter) Allow(_ context.Context, key string) (bool, time.Duration, error) {
	l.lastKey = key
	return l.allowed, l.retryAfter, nil
}
func (l *loginStubLimiter) Release(context.Context, string) error { return nil }
func (l *loginStubLimiter) Reset(context.Context, string) error   { return nil }

func loginHandlerWithLimiters(t *testing.T, ipL, emailL *loginStubLimiter) *Handler {
	t.Helper()
	env := setupVerificationHandler(t)
	svc := service.NewService(service.Config{
		JWTSecret:      "test-secret",
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         "bookhaven-test",
	}, env.db, env.users, env.mailer, service.ResendLimiters{}, service.ResendLimiters{},
		service.LoginLimiters{IP: ipL, Email: emailL}, nil)
	return NewHandler(svc)
}

func TestLoginHandler_RateLimited(t *testing.T) {
	for name, tc := range map[string]struct{ ipAllowed, emailAllowed bool }{
		"ip limited":    {false, true},
		"email limited": {true, false},
	} {
		t.Run(name, func(t *testing.T) {
			ipL := &loginStubLimiter{allowed: tc.ipAllowed, retryAfter: 41500 * time.Millisecond}
			emailL := &loginStubLimiter{allowed: tc.emailAllowed, retryAfter: 41500 * time.Millisecond}
			h := loginHandlerWithLimiters(t, ipL, emailL)

			c, rec := postJSON("/api/v1/auth/login", `{"email":"anna@example.com","password":"password123"}`)
			if err := h.Login(c); err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("status = %d, want 429 (body %s)", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Retry-After"); got != "42" {
				t.Fatalf("Retry-After = %q, want 42", got)
			}
			body := decodeBody(t, rec)
			if body["code"] != "rate_limited" || body["error"] == nil || body["retry_after_seconds"] != float64(42) {
				t.Fatalf("unexpected body %s", rec.Body.String())
			}
		})
	}
}

func TestLoginHandler_UsesRealIP(t *testing.T) {
	ipL := &loginStubLimiter{allowed: true}
	emailL := &loginStubLimiter{allowed: true}
	h := loginHandlerWithLimiters(t, ipL, emailL)

	c, _ := postJSON("/api/v1/auth/login", `{"email":"anna@example.com","password":"password123"}`)
	c.Echo().IPExtractor = func(r *http.Request) string { return "198.51.100.9" }
	if err := h.Login(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if ipL.lastKey != "198.51.100.9" {
		t.Fatalf("ip limiter key = %q, want the client IP", ipL.lastKey)
	}
}
