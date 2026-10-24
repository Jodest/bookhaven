package http

import (
	"net/http"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/service"
)

func registerHandlerWithLimiters(t *testing.T, ipL, emailL *loginStubLimiter) *Handler {
	t.Helper()
	env := setupVerificationHandler(t)
	svc := service.NewService(service.Config{
		JWTSecret:      "test-secret",
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         "bookhaven-test",
	}, env.db, env.users, env.mailer, service.ResendLimiters{}, service.ResendLimiters{},
		service.LoginLimiters{}, service.RegisterLimiters{IP: ipL, Email: emailL}, nil)
	return NewHandler(svc)
}

const registerBody = `{"email":"anna@example.com","password":"password123","name":"Анна"}`

func TestRegisterHandler_RateLimited(t *testing.T) {
	for name, tc := range map[string]struct{ ipAllowed, emailAllowed bool }{
		"ip limited":    {false, true},
		"email limited": {true, false},
	} {
		t.Run(name, func(t *testing.T) {
			ipL := &loginStubLimiter{allowed: tc.ipAllowed, retryAfter: 41500 * time.Millisecond}
			emailL := &loginStubLimiter{allowed: tc.emailAllowed, retryAfter: 41500 * time.Millisecond}
			h := registerHandlerWithLimiters(t, ipL, emailL)

			c, rec := postJSON("/api/v1/auth/register", registerBody)
			if err := h.Register(c); err != nil {
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

func TestRegisterHandler_LimitedResponseSameForExistingAndUnknown(t *testing.T) {
	ipL := &loginStubLimiter{allowed: true}
	emailL := &loginStubLimiter{allowed: true, retryAfter: 90 * time.Second}
	h := registerHandlerWithLimiters(t, ipL, emailL)

	// anna@example.com is seeded by setupVerificationHandler.
	emailL.allowed = false

	var bodies []string
	for _, email := range []string{"anna@example.com", "ghost@example.com"} {
		c, rec := postJSON("/api/v1/auth/register", `{"email":"`+email+`","password":"password123","name":"Анна"}`)
		if err := h.Register(c); err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("%s: status = %d, want 429", email, rec.Code)
		}
		bodies = append(bodies, rec.Body.String())
	}
	if bodies[0] != bodies[1] {
		t.Fatalf("limited responses differ: %q vs %q", bodies[0], bodies[1])
	}
}

func TestRegisterHandler_BadRequestDoesNotCount(t *testing.T) {
	ipL := &loginStubLimiter{allowed: true}
	emailL := &loginStubLimiter{allowed: true}
	h := registerHandlerWithLimiters(t, ipL, emailL)

	for _, body := range []string{
		`{"email":"not-an-email","password":"password123","name":"Анна"}`,
		`{"email":"anna@example.com","password":"password123","name":"А"}`,
	} {
		c, rec := postJSON("/api/v1/auth/register", body)
		if err := h.Register(c); err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
		}
	}
	if ipL.lastKey != "" || emailL.lastKey != "" {
		t.Fatalf("limiters must not be consulted: ip=%q email=%q", ipL.lastKey, emailL.lastKey)
	}
}
