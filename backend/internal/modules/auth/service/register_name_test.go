package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRegister_NameRules(t *testing.T) {
	for label, name := range map[string]string{
		"whitespace only": "   ",
		"empty":           "",
		"one char":        " a ",
		"256 runes":       strings.Repeat("я", 256),
	} {
		t.Run("rejects "+label, func(t *testing.T) {
			env := setupVerificationEnv(t)
			user, err := env.svc.Register(context.Background(), "anna@example.com", "password123", name, "")
			if !errors.Is(err, ErrInvalidName) || user != nil {
				t.Fatalf("expected ErrInvalidName, got user=%v err=%v", user, err)
			}
			if u, _ := env.users.GetByEmail(context.Background(), "anna@example.com"); u != nil {
				t.Fatal("no user must be created")
			}
		})
	}

	t.Run("trims stored name", func(t *testing.T) {
		env := setupVerificationEnv(t)
		user, err := env.svc.Register(context.Background(), "anna@example.com", "password123", "  Анна  ", "")
		if err != nil {
			t.Fatalf("register: %v", err)
		}
		if got := env.getUser(t, user.ID).Name; got == nil || *got != "Анна" {
			t.Fatalf("stored name = %v, want Анна", got)
		}
	})

	t.Run("accepts 255 runes", func(t *testing.T) {
		env := setupVerificationEnv(t)
		if _, err := env.svc.Register(context.Background(), "anna@example.com", "password123", strings.Repeat("я", 255), ""); err != nil {
			t.Fatalf("register: %v", err)
		}
	})
}
