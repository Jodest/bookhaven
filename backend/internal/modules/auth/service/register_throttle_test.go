package service

import (
	"context"
	"errors"
	"testing"

	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/modules/users/repository"
)

func registerThrottleEnv(t *testing.T, ipLimit, emailLimit int) (*verificationEnv, *counterLimiter, *counterLimiter) {
	t.Helper()
	env := setupVerificationEnv(t)
	ipL, emailL := newCounterLimiter(ipLimit), newCounterLimiter(emailLimit)
	env.svc.registerLimiters = RegisterLimiters{IP: ipL, Email: emailL}
	return env, ipL, emailL
}

func TestRegister_IPLimitBlocksEleventhRequest(t *testing.T) {
	env, _, _ := registerThrottleEnv(t, 10, 100)
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		email := string(rune('a'+i)) + "@example.com"
		if _, err := env.svc.Register(ctx, email, "password123", "Анна", testIP); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	sent := len(env.mailer.messages)
	_, err := env.svc.Register(ctx, "k@example.com", "password123", "Анна", testIP)
	var rl *RateLimitError
	if !errors.As(err, &rl) || !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected *RateLimitError, got %v", err)
	}
	if _, gerr := env.users.GetByEmail(ctx, "k@example.com"); gerr == nil {
		t.Fatal("user must not be created when limited")
	}
	if len(env.mailer.messages) != sent {
		t.Fatal("no mail must be enqueued when limited")
	}
}

func TestRegister_IPDenyDoesNotTouchEmailLimiter(t *testing.T) {
	env, _, emailL := registerThrottleEnv(t, 0, 5)
	_, err := env.svc.Register(context.Background(), "anna@example.com", "password123", "Анна", testIP)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if emailL.allowCalls != 0 {
		t.Fatalf("email limiter must not be consulted, calls=%d", emailL.allowCalls)
	}
}

func TestRegister_EmailDenyKeepsIPSlot(t *testing.T) {
	env, ipL, _ := registerThrottleEnv(t, 10, 0)
	_, err := env.svc.Register(context.Background(), "anna@example.com", "password123", "Анна", testIP)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if ipL.count[testIP] != 1 || len(ipL.releases) != 0 {
		t.Fatalf("ip slot must stay spent: count=%d releases=%v", ipL.count[testIP], ipL.releases)
	}
}

func TestRegister_ConflictConsumesBothSlots(t *testing.T) {
	env, ipL, emailL := registerThrottleEnv(t, 10, 5)
	ctx := context.Background()
	if _, err := env.svc.Register(ctx, "anna@example.com", "password123", "Анна", testIP); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.Register(ctx, "anna@example.com", "password123", "Анна", testIP); !errors.Is(err, ErrUserExists) {
		t.Fatalf("expected ErrUserExists, got %v", err)
	}
	key := emailRateLimitKey("anna@example.com")
	if ipL.count[testIP] != 2 || emailL.count[key] != 2 || len(ipL.releases)+len(emailL.releases) != 0 {
		t.Fatalf("both slots must stay spent: ip=%d email=%d", ipL.count[testIP], emailL.count[key])
	}
}

func TestRegister_InvalidInputConsumesNothing(t *testing.T) {
	env, ipL, emailL := registerThrottleEnv(t, 10, 5)
	ctx := context.Background()
	if _, err := env.svc.Register(ctx, "anna@example.com", "password123", "А", testIP); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("expected ErrInvalidName, got %v", err)
	}
	long := make([]byte, MaxPasswordBytes+1)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := env.svc.Register(ctx, "anna@example.com", string(long), "Анна", testIP); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("expected ErrPasswordTooLong, got %v", err)
	}
	if ipL.allowCalls != 0 || emailL.allowCalls != 0 {
		t.Fatalf("limiters must not be consulted: ip=%d email=%d", ipL.allowCalls, emailL.allowCalls)
	}
}

type failingCreateRepo struct{ repository.Repository }

func (failingCreateRepo) Create(context.Context, *usersDomain.User) error {
	return errors.New("db down")
}

func TestRegister_InternalErrorReleasesBothSlots(t *testing.T) {
	env, ipL, emailL := registerThrottleEnv(t, 10, 5)
	env.svc.users = failingCreateRepo{env.users}
	_, err := env.svc.Register(context.Background(), "anna@example.com", "password123", "Анна", testIP)
	if err == nil || errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected internal error, got %v", err)
	}
	key := emailRateLimitKey("anna@example.com")
	if ipL.count[testIP] != 0 || emailL.count[key] != 0 || len(ipL.releases) != 1 || len(emailL.releases) != 1 {
		t.Fatalf("both slots must be released: ip=%d email=%d", ipL.count[testIP], emailL.count[key])
	}
}

func TestRegister_LimiterErrorFailsOpen(t *testing.T) {
	env, ipL, emailL := registerThrottleEnv(t, 10, 5)
	ipL.allowErr = errors.New("redis down")
	emailL.allowErr = errors.New("redis down")
	if _, err := env.svc.Register(context.Background(), "anna@example.com", "password123", "Анна", testIP); err != nil {
		t.Fatalf("expected success when limiters fail, got %v", err)
	}
}

func TestRegister_EmailKeyIsNormalised(t *testing.T) {
	env, _, emailL := registerThrottleEnv(t, 10, 5)
	ctx := context.Background()
	_, _ = env.svc.Register(ctx, " User@X.com ", "password123", "Анна", testIP)
	_, _ = env.svc.Register(ctx, "user@x.com", "password123", "Анна", testIP)
	if len(emailL.count) != 1 || emailL.count[emailRateLimitKey("user@x.com")] == 0 {
		t.Fatalf("expected one normalised key, got %v", emailL.count)
	}
}
