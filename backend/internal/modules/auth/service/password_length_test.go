package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// bcrypt limit: 72 bytes. Cyrillic letters take 2 bytes each.
func TestRegister_PasswordByteLimit(t *testing.T) {
	tooLong := map[string]string{
		"73 ASCII bytes":    strings.Repeat("a", 73),
		"40 Cyrillic runes": strings.Repeat("я", 40),
	}
	for name, pw := range tooLong {
		t.Run("rejects "+name, func(t *testing.T) {
			env := setupVerificationEnv(t)
			user, err := env.svc.Register(context.Background(), "anna@example.com", pw, "Анна")
			if !errors.Is(err, ErrPasswordTooLong) || user != nil {
				t.Fatalf("expected ErrPasswordTooLong, got user=%v err=%v", user, err)
			}
			if u, _ := env.users.GetByEmail(context.Background(), "anna@example.com"); u != nil {
				t.Fatal("no user must be created")
			}
			if len(env.mailer.messages) != 0 {
				t.Fatal("no email must be sent")
			}
		})
	}
	ok := map[string]string{
		"72 ASCII bytes":    strings.Repeat("a", 72),
		"36 Cyrillic runes": strings.Repeat("я", 36),
	}
	for name, pw := range ok {
		t.Run("accepts "+name, func(t *testing.T) {
			env := setupVerificationEnv(t)
			user, err := env.svc.Register(context.Background(), "anna@example.com", pw, "Анна")
			if err != nil {
				t.Fatalf("register: %v", err)
			}
			if !passwordMatches(env.getUser(t, user.ID), pw) {
				t.Fatal("hash does not match")
			}
		})
	}
}

func TestResetPassword_PasswordByteLimit(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	plain := plainToken('e')
	env.insertResetToken(t, user.ID, user.Email, plain, time.Now().Add(time.Hour), nil)

	if err := env.svc.ResetPassword(context.Background(), plain, strings.Repeat("я", 40)); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("expected ErrPasswordTooLong, got %v", err)
	}
	// Checked before the token: even a malformed token reports the password.
	if err := env.svc.ResetPassword(context.Background(), "bad", strings.Repeat("a", 73)); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("expected ErrPasswordTooLong, got %v", err)
	}
	for _, tok := range env.resetTokens(t, user.ID) {
		if tok.UsedAt != nil {
			t.Fatal("token must stay unused")
		}
	}
	if !passwordMatches(env.getUser(t, user.ID), "password123") {
		t.Fatal("old password must still match")
	}

	ok := strings.Repeat("я", 36)
	if err := env.svc.ResetPassword(context.Background(), plain, ok); err != nil {
		t.Fatalf("reset with 36 Cyrillic runes: %v", err)
	}
	if !passwordMatches(env.getUser(t, user.ID), ok) {
		t.Fatal("new password does not match")
	}
}
