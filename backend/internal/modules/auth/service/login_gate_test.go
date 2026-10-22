package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
)

// Tests for the email verification login gate
// (docs/features/email-verification-login-gate.md).

const testIP = "203.0.113.7"

func (e *verificationEnv) refreshTokenCount(t *testing.T, userID uuid.UUID, activeOnly bool) int64 {
	t.Helper()
	q := e.db.Model(&domain.RefreshToken{}).Where("user_id = ?", userID)
	if activeOnly {
		q = q.Where("revoked_at IS NULL")
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatalf("count refresh tokens: %v", err)
	}
	return n
}

func TestLogin_UnverifiedCorrectPassword(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")

	_, tokens, err := env.svc.Login(context.Background(), "anna@example.com", "password123", "203.0.113.9")
	if !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
	if tokens != nil {
		t.Fatal("no tokens must be returned for an unverified user")
	}
	if n := env.refreshTokenCount(t, user.ID, false); n != 0 {
		t.Fatalf("expected no refresh-token rows, got %d", n)
	}
}

func TestLogin_UnverifiedWrongPassword(t *testing.T) {
	env := setupVerificationEnv(t)
	env.register(t, "anna@example.com")

	if _, _, err := env.svc.Login(context.Background(), "anna@example.com", "wrong-password", "203.0.113.9"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_Verified(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.markVerified(t, user.ID)

	_, tokens, err := env.svc.Login(context.Background(), "anna@example.com", "password123", "203.0.113.9")
	if err != nil || tokens == nil || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("expected tokens, got tokens=%v err=%v", tokens, err)
	}
}

func TestRefreshTokens_UnverifiedRevokesAll(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	// Sessions issued before the login gate existed.
	first, err := env.svc.CreateTokenPair(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("create token pair: %v", err)
	}
	if _, err := env.svc.CreateTokenPair(context.Background(), user.ID); err != nil {
		t.Fatalf("create token pair: %v", err)
	}

	if _, err := env.svc.RefreshTokens(context.Background(), first.RefreshToken); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
	if n := env.refreshTokenCount(t, user.ID, true); n != 0 {
		t.Fatalf("expected all refresh tokens revoked, %d still active", n)
	}
	if n := env.refreshTokenCount(t, user.ID, false); n != 2 {
		t.Fatalf("expected no new refresh-token rows, got %d rows", n)
	}
}

func TestRefreshTokens_VerifiedRotates(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.markVerified(t, user.ID)
	issued, err := env.svc.CreateTokenPair(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("create token pair: %v", err)
	}

	rotated, err := env.svc.RefreshTokens(context.Background(), issued.RefreshToken)
	if err != nil || rotated == nil {
		t.Fatalf("expected rotation, got %v", err)
	}
	if n := env.refreshTokenCount(t, user.ID, true); n != 1 {
		t.Fatalf("expected exactly the new token active, got %d", n)
	}
}

func TestResendVerification_UnknownEmail(t *testing.T) {
	env := setupVerificationEnv(t)

	if err := env.svc.ResendVerification(context.Background(), "nobody@example.com", testIP); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(env.mailer.messages) != 0 {
		t.Fatal("nothing must be sent for an unknown email")
	}
	if env.ipLimiter.calls() != 1 || env.emailLimiter.calls() != 1 {
		t.Fatalf("both limiters must be consulted, got ip=%d email=%d", env.ipLimiter.calls(), env.emailLimiter.calls())
	}
	if env.ipLimiter.keys[0] != testIP {
		t.Fatalf("ip limiter key = %q, want %q", env.ipLimiter.keys[0], testIP)
	}
}

func TestResendVerification_AlreadyVerified(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.markVerified(t, user.ID)

	if err := env.svc.ResendVerification(context.Background(), "anna@example.com", testIP); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(env.mailer.messages) != 1 {
		t.Fatalf("expected only the registration email, got %d", len(env.mailer.messages))
	}
}

func TestResendVerification_NoPassword(t *testing.T) {
	env := setupVerificationEnv(t)
	user := &usersDomain.User{
		ID:          uuid.New(),
		Email:       "oauth@example.com",
		Role:        usersDomain.RoleUser,
		ProfileType: usersDomain.ProfileTypeReader,
	}
	if err := env.users.Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := env.svc.ResendVerification(context.Background(), "oauth@example.com", testIP); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(env.mailer.messages) != 0 {
		t.Fatal("nothing must be sent for an account without password")
	}
}

func TestResendVerification_IPRateLimited(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.ipLimiter.allowed = false
	env.ipLimiter.retryAfter = 42 * time.Second

	err := env.svc.ResendVerification(context.Background(), "anna@example.com", testIP)
	var rlErr *RateLimitError
	if !errors.Is(err, ErrRateLimited) || !errors.As(err, &rlErr) || rlErr.RetryAfter != 42*time.Second {
		t.Fatalf("expected RateLimitError with 42s, got %v", err)
	}
	if env.emailLimiter.calls() != 0 {
		t.Fatal("email limiter must not be consulted after the ip limit denies")
	}
	if len(env.mailer.messages) != 1 || len(env.tokens(t, user.ID)) != 1 {
		t.Fatal("rate-limited resend must not touch tokens or send email")
	}
}

func TestResendVerification_EmailRateLimited(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.emailLimiter.allowed = false
	env.emailLimiter.retryAfter = time.Hour

	err := env.svc.ResendVerification(context.Background(), "anna@example.com", testIP)
	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) || rlErr.RetryAfter != time.Hour {
		t.Fatalf("expected RateLimitError with 1h, got %v", err)
	}
	if len(env.mailer.messages) != 1 || len(env.tokens(t, user.ID)) != 1 {
		t.Fatal("rate-limited resend must not touch tokens or send email")
	}
}

func TestResendVerification_EmailKeyIsNormalisedHash(t *testing.T) {
	env := setupVerificationEnv(t)

	for _, email := range []string{"Anna@Example.com", "  anna@example.com "} {
		if err := env.svc.ResendVerification(context.Background(), email, testIP); err != nil {
			t.Fatalf("resend %q: %v", email, err)
		}
	}
	keys := env.emailLimiter.keys
	if len(keys) != 2 || keys[0] != keys[1] {
		t.Fatalf("expected two equal keys, got %v", keys)
	}
	if keys[0] != hashToken("anna@example.com") {
		t.Fatalf("key = %q, want sha256 of the normalised email", keys[0])
	}
	if strings.Contains(keys[0], "@") {
		t.Fatal("plain email must not be used as a key")
	}
}

func TestResendVerification_LimiterError(t *testing.T) {
	for _, which := range []string{"ip", "email"} {
		t.Run(which, func(t *testing.T) {
			env := setupVerificationEnv(t)
			env.register(t, "anna@example.com")
			if which == "ip" {
				env.ipLimiter.err = errors.New("redis down")
			} else {
				env.emailLimiter.err = errors.New("redis down")
			}

			err := env.svc.ResendVerification(context.Background(), "anna@example.com", testIP)
			if err == nil || errors.Is(err, ErrRateLimited) {
				t.Fatalf("expected a plain error, got %v", err)
			}
			if len(env.mailer.messages) != 1 {
				t.Fatal("no email must be sent on limiter error")
			}
		})
	}
}

func TestResendVerification_ReplacesOldToken(t *testing.T) {
	env := setupVerificationEnv(t)
	user, oldPlain := env.register(t, "anna@example.com")

	if err := env.svc.ResendVerification(context.Background(), "anna@example.com", testIP); err != nil {
		t.Fatalf("resend: %v", err)
	}
	if env.ipLimiter.calls() != 1 || env.emailLimiter.calls() != 1 {
		t.Fatalf("expected each limiter to be called once, got ip=%d email=%d", env.ipLimiter.calls(), env.emailLimiter.calls())
	}
	if len(env.mailer.messages) != 2 {
		t.Fatalf("expected 2 emails, got %d", len(env.mailer.messages))
	}
	newPlain := tokenFromMessage(t, env.mailer.messages[1])
	if newPlain == oldPlain {
		t.Fatal("expected a new token")
	}
	if rows := env.tokens(t, user.ID); len(rows) != 1 {
		t.Fatalf("expected only the new token row, got %d", len(rows))
	}

	if _, err := env.svc.VerifyEmail(context.Background(), oldPlain); !errors.Is(err, ErrVerificationTokenInvalid) {
		t.Fatalf("old link: expected ErrVerificationTokenInvalid, got %v", err)
	}
	if already, err := env.svc.VerifyEmail(context.Background(), newPlain); err != nil || already {
		t.Fatalf("new link: expected verified, got already=%v err=%v", already, err)
	}
}

func TestResendVerification_EnqueueErrorIsSwallowed(t *testing.T) {
	env := setupVerificationEnv(t)
	env.register(t, "anna@example.com")
	env.mailer.err = errors.New("redis down")

	if err := env.svc.ResendVerification(context.Background(), "anna@example.com", testIP); err != nil {
		t.Fatalf("expected nil (logged) on enqueue failure, got %v", err)
	}
}
