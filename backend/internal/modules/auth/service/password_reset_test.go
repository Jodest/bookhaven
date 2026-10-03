package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/pkg/mailer"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Tests for password reset (docs/features/password-reset.md).

const newTestPassword = "new-password-456"

// resetTokenFromMessage extracts the plain token from the reset link in
// msg's text.
func resetTokenFromMessage(t *testing.T, msg mailer.Message) string {
	t.Helper()
	const marker = "http://localhost:3000/reset-password?token="
	i := strings.Index(msg.TextBody, marker)
	if i < 0 {
		t.Fatalf("message text has no reset link: %q", msg.TextBody)
	}
	rest := msg.TextBody[i+len(marker):]
	if len(rest) < 64 {
		t.Fatalf("token in link too short: %q", rest)
	}
	return rest[:64]
}

// forgot requests a reset for email and returns the emailed plain token.
func (e *verificationEnv) forgot(t *testing.T, email string) string {
	t.Helper()
	before := len(e.mailer.messages)
	if err := e.svc.ForgotPassword(context.Background(), email, testIP); err != nil {
		t.Fatalf("forgot password: %v", err)
	}
	if len(e.mailer.messages) != before+1 {
		t.Fatalf("expected one reset email, got %d", len(e.mailer.messages)-before)
	}
	return resetTokenFromMessage(t, e.mailer.messages[len(e.mailer.messages)-1])
}

func (e *verificationEnv) resetTokens(t *testing.T, userID uuid.UUID) []domain.PasswordResetToken {
	t.Helper()
	var rows []domain.PasswordResetToken
	if err := e.db.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		t.Fatalf("list reset tokens: %v", err)
	}
	return rows
}

func (e *verificationEnv) insertResetToken(t *testing.T, userID uuid.UUID, email, plain string, expiresAt time.Time, usedAt *time.Time) {
	t.Helper()
	row := &domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    userID,
		Email:     email,
		TokenHash: hashToken(plain),
		ExpiresAt: expiresAt,
		UsedAt:    usedAt,
		CreatedAt: time.Now(),
	}
	if err := e.db.Create(row).Error; err != nil {
		t.Fatalf("insert reset token: %v", err)
	}
}

func (e *verificationEnv) getUser(t *testing.T, userID uuid.UUID) *usersDomain.User {
	t.Helper()
	u, err := e.users.GetByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	return u
}

func passwordMatches(u *usersDomain.User, password string) bool {
	return u.PasswordHash != nil && bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(password)) == nil
}

func TestForgotPassword_UnknownEmail(t *testing.T) {
	env := setupVerificationEnv(t)

	if err := env.svc.ForgotPassword(context.Background(), "nobody@example.com", testIP); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(env.mailer.messages) != 0 {
		t.Fatal("nothing must be sent for an unknown email")
	}
	if env.resetIPLimiter.calls() != 1 || env.resetEmailLimiter.calls() != 1 {
		t.Fatalf("both limiters must be consulted, got ip=%d email=%d", env.resetIPLimiter.calls(), env.resetEmailLimiter.calls())
	}
	if env.resetIPLimiter.keys[0] != testIP {
		t.Fatalf("ip limiter key = %q, want %q", env.resetIPLimiter.keys[0], testIP)
	}
	if env.ipLimiter.calls() != 0 || env.emailLimiter.calls() != 0 {
		t.Fatal("forgot-password must not use the resend-verification limiters")
	}
}

func TestForgotPassword_VerifiedUser(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.markVerified(t, user.ID)

	plain := env.forgot(t, "anna@example.com")

	msg := env.mailer.messages[len(env.mailer.messages)-1]
	if msg.To != "anna@example.com" {
		t.Errorf("email sent to %q", msg.To)
	}
	if !strings.Contains(msg.Subject, "Сброс пароля") {
		t.Errorf("unexpected subject %q", msg.Subject)
	}
	if !strings.Contains(msg.TextBody, "/reset-password?token="+plain) {
		t.Errorf("text does not contain link: %q", msg.TextBody)
	}

	rows := env.resetTokens(t, user.ID)
	if len(rows) != 1 {
		t.Fatalf("expected 1 reset token row, got %d", len(rows))
	}
	row := rows[0]
	if row.TokenHash != hashToken(plain) {
		t.Fatal("stored hash does not match sha256 of the emailed token")
	}
	if row.Email != user.Email || row.UsedAt != nil {
		t.Fatalf("unexpected token row: %+v", row)
	}
	if d := time.Until(row.ExpiresAt); d < 59*time.Minute || d > time.Hour {
		t.Fatalf("expected expiry ~1h from now, got %v", d)
	}
}

func TestForgotPassword_UnverifiedUserGetsEmail(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")

	env.forgot(t, "anna@example.com")
	if rows := env.resetTokens(t, user.ID); len(rows) != 1 {
		t.Fatalf("expected 1 reset token row, got %d", len(rows))
	}
}

func TestForgotPassword_NoPassword(t *testing.T) {
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

	if err := env.svc.ForgotPassword(context.Background(), "oauth@example.com", testIP); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(env.mailer.messages) != 0 || len(env.resetTokens(t, user.ID)) != 0 {
		t.Fatal("nothing must be sent for an account without password")
	}
}

func TestForgotPassword_ReplacesOldToken(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")

	oldPlain := env.forgot(t, "anna@example.com")
	newPlain := env.forgot(t, "anna@example.com")
	if oldPlain == newPlain {
		t.Fatal("expected a new token")
	}
	if rows := env.resetTokens(t, user.ID); len(rows) != 1 {
		t.Fatalf("expected only the new token row, got %d", len(rows))
	}
	if err := env.svc.ResetPassword(context.Background(), oldPlain, newTestPassword); !errors.Is(err, ErrResetTokenInvalid) {
		t.Fatalf("old link: expected ErrResetTokenInvalid, got %v", err)
	}
	if err := env.svc.ResetPassword(context.Background(), newPlain, newTestPassword); err != nil {
		t.Fatalf("new link: %v", err)
	}
}

func TestForgotPassword_IPRateLimited(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.resetIPLimiter.allowed = false
	env.resetIPLimiter.retryAfter = 42 * time.Second

	err := env.svc.ForgotPassword(context.Background(), "anna@example.com", testIP)
	var rlErr *RateLimitError
	if !errors.Is(err, ErrRateLimited) || !errors.As(err, &rlErr) || rlErr.RetryAfter != 42*time.Second {
		t.Fatalf("expected RateLimitError with 42s, got %v", err)
	}
	if env.resetEmailLimiter.calls() != 0 {
		t.Fatal("email limiter must not be consulted after the ip limit denies")
	}
	if len(env.mailer.messages) != 1 || len(env.resetTokens(t, user.ID)) != 0 {
		t.Fatal("rate-limited request must not create tokens or send email")
	}
}

func TestForgotPassword_EmailRateLimited(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.resetEmailLimiter.allowed = false
	env.resetEmailLimiter.retryAfter = time.Minute

	err := env.svc.ForgotPassword(context.Background(), "anna@example.com", testIP)
	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) || rlErr.RetryAfter != time.Minute {
		t.Fatalf("expected RateLimitError with 1m, got %v", err)
	}
	if len(env.mailer.messages) != 1 || len(env.resetTokens(t, user.ID)) != 0 {
		t.Fatal("rate-limited request must not create tokens or send email")
	}
}

func TestForgotPassword_EmailKeyIsNormalisedHash(t *testing.T) {
	env := setupVerificationEnv(t)

	for _, email := range []string{"Anna@Example.com", "  anna@example.com "} {
		if err := env.svc.ForgotPassword(context.Background(), email, testIP); err != nil {
			t.Fatalf("forgot %q: %v", email, err)
		}
	}
	keys := env.resetEmailLimiter.keys
	if len(keys) != 2 || keys[0] != keys[1] {
		t.Fatalf("expected two equal keys, got %v", keys)
	}
	if keys[0] != hashToken("anna@example.com") {
		t.Fatalf("key = %q, want sha256 of the normalised email", keys[0])
	}
}

func TestForgotPassword_LimiterError(t *testing.T) {
	for _, which := range []string{"ip", "email"} {
		t.Run(which, func(t *testing.T) {
			env := setupVerificationEnv(t)
			env.register(t, "anna@example.com")
			if which == "ip" {
				env.resetIPLimiter.err = errors.New("redis down")
			} else {
				env.resetEmailLimiter.err = errors.New("redis down")
			}

			err := env.svc.ForgotPassword(context.Background(), "anna@example.com", testIP)
			if err == nil || errors.Is(err, ErrRateLimited) {
				t.Fatalf("expected a plain error, got %v", err)
			}
			if len(env.mailer.messages) != 1 {
				t.Fatal("no email must be sent on limiter error")
			}
		})
	}
}

func TestForgotPassword_EnqueueErrorIsSwallowed(t *testing.T) {
	env := setupVerificationEnv(t)
	env.register(t, "anna@example.com")
	env.mailer.err = errors.New("redis down")

	if err := env.svc.ForgotPassword(context.Background(), "anna@example.com", testIP); err != nil {
		t.Fatalf("expected nil (logged) on enqueue failure, got %v", err)
	}
}

func TestResetPassword_Success(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.markVerified(t, user.ID)
	verifiedAt := env.getUser(t, user.ID).EmailVerifiedAt

	oldSession, err := env.svc.CreateTokenPair(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("create token pair: %v", err)
	}
	if _, err := env.svc.CreateTokenPair(context.Background(), user.ID); err != nil {
		t.Fatalf("create token pair: %v", err)
	}
	plain := env.forgot(t, "anna@example.com")
	// A stray unused token (e.g. issued concurrently) must be removed too.
	env.insertResetToken(t, user.ID, user.Email, plainToken('f'), time.Now().Add(time.Hour), nil)

	if err := env.svc.ResetPassword(context.Background(), plain, newTestPassword); err != nil {
		t.Fatalf("reset password: %v", err)
	}

	got := env.getUser(t, user.ID)
	if !passwordMatches(got, newTestPassword) {
		t.Fatal("new password does not match")
	}
	if passwordMatches(got, "password123") {
		t.Fatal("old password still matches")
	}
	if got.EmailVerifiedAt == nil || !got.EmailVerifiedAt.Equal(*verifiedAt) {
		t.Fatalf("email_verified_at changed: %v -> %v", verifiedAt, got.EmailVerifiedAt)
	}
	rows := env.resetTokens(t, user.ID)
	if len(rows) != 1 || rows[0].TokenHash != hashToken(plain) || rows[0].UsedAt == nil {
		t.Fatalf("expected only the used token row, got %+v", rows)
	}
	if n := env.refreshTokenCount(t, user.ID, true); n != 0 {
		t.Fatalf("expected all refresh tokens revoked, %d still active", n)
	}

	if _, err := env.svc.RefreshTokens(context.Background(), oldSession.RefreshToken); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("old refresh token: expected ErrTokenInvalid, got %v", err)
	}
	if _, _, err := env.svc.Login(context.Background(), "anna@example.com", "password123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password login: expected ErrInvalidCredentials, got %v", err)
	}
	if _, tokens, err := env.svc.Login(context.Background(), "anna@example.com", newTestPassword); err != nil || tokens == nil {
		t.Fatalf("new password login: tokens=%v err=%v", tokens, err)
	}

	// Second click on the same link.
	if err := env.svc.ResetPassword(context.Background(), plain, "another-password"); !errors.Is(err, ErrResetTokenUsed) {
		t.Fatalf("expected ErrResetTokenUsed, got %v", err)
	}
	if !passwordMatches(env.getUser(t, user.ID), newTestPassword) {
		t.Fatal("password must not change on a used token")
	}
}

func TestResetPassword_VerifiesUnverifiedUser(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	plain := env.forgot(t, "anna@example.com")

	if err := env.svc.ResetPassword(context.Background(), plain, newTestPassword); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if !env.isVerified(t, user.ID) {
		t.Fatal("reset must verify the email")
	}
	if _, tokens, err := env.svc.Login(context.Background(), "anna@example.com", newTestPassword); err != nil || tokens == nil {
		t.Fatalf("login after reset: tokens=%v err=%v", tokens, err)
	}
}

func TestResetPassword_Expired(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	plain := plainToken('a')
	env.insertResetToken(t, user.ID, user.Email, plain, time.Now().Add(-time.Minute), nil)

	if err := env.svc.ResetPassword(context.Background(), plain, newTestPassword); !errors.Is(err, ErrResetTokenExpired) {
		t.Fatalf("expected ErrResetTokenExpired, got %v", err)
	}
	got := env.getUser(t, user.ID)
	if !passwordMatches(got, "password123") || got.EmailVerifiedAt != nil {
		t.Fatal("an expired token must not change the user")
	}
}

func TestResetPassword_UsedToken(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	plain := plainToken('d')
	used := time.Now().Add(-time.Minute)
	env.insertResetToken(t, user.ID, user.Email, plain, time.Now().Add(time.Hour), &used)

	if err := env.svc.ResetPassword(context.Background(), plain, newTestPassword); !errors.Is(err, ErrResetTokenUsed) {
		t.Fatalf("expected ErrResetTokenUsed, got %v", err)
	}
}

func TestResetPassword_InvalidTokens(t *testing.T) {
	env := setupVerificationEnv(t)
	env.register(t, "anna@example.com")

	for _, token := range []string{"", "short", strings.Repeat("z", 64), plainToken('b')} {
		if err := env.svc.ResetPassword(context.Background(), token, newTestPassword); !errors.Is(err, ErrResetTokenInvalid) {
			t.Fatalf("token %q: expected ErrResetTokenInvalid, got %v", token, err)
		}
	}
}

func TestResetPassword_EmailMismatch(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	plain := plainToken('c')
	env.insertResetToken(t, user.ID, "old@example.com", plain, time.Now().Add(time.Hour), nil)

	if err := env.svc.ResetPassword(context.Background(), plain, newTestPassword); !errors.Is(err, ErrResetTokenInvalid) {
		t.Fatalf("expected ErrResetTokenInvalid, got %v", err)
	}
	if !passwordMatches(env.getUser(t, user.ID), "password123") {
		t.Fatal("password must not change")
	}
}
