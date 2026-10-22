package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// counterLimiter is a stateful fake LoginLimiter: Allow reserves a slot until
// limit is reached.
type counterLimiter struct {
	limit      int
	count      map[string]int
	allowCalls int
	releases   []string
	resets     []string
	allowErr   error
	retryAfter time.Duration
}

func newCounterLimiter(limit int) *counterLimiter {
	return &counterLimiter{limit: limit, count: map[string]int{}, retryAfter: 10 * time.Minute}
}

func (l *counterLimiter) Allow(_ context.Context, key string) (bool, time.Duration, error) {
	l.allowCalls++
	if l.allowErr != nil {
		return false, 0, l.allowErr
	}
	if l.count[key] >= l.limit {
		return false, l.retryAfter, nil
	}
	l.count[key]++
	return true, 0, nil
}

func (l *counterLimiter) Release(_ context.Context, key string) error {
	l.releases = append(l.releases, key)
	if l.count[key] > 0 {
		l.count[key]--
	}
	return nil
}

func (l *counterLimiter) Reset(_ context.Context, key string) error {
	l.resets = append(l.resets, key)
	delete(l.count, key)
	return nil
}

func throttleEnv(t *testing.T, ipLimit, emailLimit int) (*verificationEnv, *counterLimiter, *counterLimiter) {
	t.Helper()
	env := setupVerificationEnv(t)
	ipL, emailL := newCounterLimiter(ipLimit), newCounterLimiter(emailLimit)
	env.svc.loginLimiters = LoginLimiters{IP: ipL, Email: emailL}
	return env, ipL, emailL
}

func TestLogin_ThrottlesAfterTenFailures(t *testing.T) {
	env, _, emailL := throttleEnv(t, 100, 10)
	env.register(t, "anna@example.com")
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		if _, _, err := env.svc.Login(ctx, "anna@example.com", "wrong-password", testIP); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: expected ErrInvalidCredentials, got %v", i+1, err)
		}
	}
	_, _, err := env.svc.Login(ctx, "anna@example.com", "password123", testIP)
	var rl *RateLimitError
	if !errors.As(err, &rl) || !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected *RateLimitError, got %v", err)
	}
	if rl.RetryAfter != emailL.retryAfter {
		t.Fatalf("RetryAfter = %v, want %v", rl.RetryAfter, emailL.retryAfter)
	}
}

func TestLogin_LimitedBeforeLookup_SameForUnknownAndExisting(t *testing.T) {
	env, _, emailL := throttleEnv(t, 100, 1)
	env.register(t, "anna@example.com")
	ctx := context.Background()

	for _, email := range []string{"anna@example.com", "ghost@example.com"} {
		if _, _, err := env.svc.Login(ctx, email, "password123", testIP); email == "ghost@example.com" && !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("unknown first attempt: %v", err)
		} else if email == "anna@example.com" && err != nil && !errors.Is(err, ErrEmailNotVerified) {
			t.Fatalf("existing first attempt: %v", err)
		}
	}
	// Exhaust both addresses, then the correct password is not even checked.
	emailL.count[emailRateLimitKey("anna@example.com")] = 1
	emailL.count[emailRateLimitKey("ghost@example.com")] = 1
	for _, email := range []string{"anna@example.com", "ghost@example.com"} {
		_, _, err := env.svc.Login(ctx, email, "password123", testIP)
		if !errors.Is(err, ErrRateLimited) {
			t.Fatalf("%s: expected ErrRateLimited, got %v", email, err)
		}
	}
}

func TestLogin_SuccessResetsEmailAndKeepsIPSlot(t *testing.T) {
	env, ipL, emailL := throttleEnv(t, 100, 10)
	user, _ := env.register(t, "anna@example.com")
	env.markVerified(t, user.ID)
	ctx := context.Background()

	for i := 0; i < 9; i++ {
		_, _, _ = env.svc.Login(ctx, "anna@example.com", "wrong-password", testIP)
	}
	if _, tokens, err := env.svc.Login(ctx, "anna@example.com", "password123", testIP); err != nil || tokens == nil {
		t.Fatalf("expected success, got tokens=%v err=%v", tokens, err)
	}
	key := emailRateLimitKey("anna@example.com")
	if emailL.count[key] != 0 || len(emailL.resets) != 1 || emailL.resets[0] != key {
		t.Fatalf("email counter not reset: count=%d resets=%v", emailL.count[key], emailL.resets)
	}
	if ipL.count[testIP] != 10 || len(ipL.releases) != 0 {
		t.Fatalf("ip slot must stay spent: count=%d releases=%v", ipL.count[testIP], ipL.releases)
	}
}

func TestLogin_UnverifiedCorrectPasswordResetsEmailOnly(t *testing.T) {
	env, ipL, emailL := throttleEnv(t, 100, 10)
	env.register(t, "anna@example.com")

	if _, _, err := env.svc.Login(context.Background(), "anna@example.com", "password123", testIP); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
	if len(emailL.resets) != 1 || len(ipL.releases) != 0 {
		t.Fatalf("expected email reset and no ip release, got resets=%v releases=%v", emailL.resets, ipL.releases)
	}
}

func TestLogin_CorrectPasswordsConsumeIPQuota(t *testing.T) {
	env, _, _ := throttleEnv(t, 6, 10)
	anna, _ := env.register(t, "anna@example.com")
	env.markVerified(t, anna.ID)
	ctx := context.Background()

	// Alternate victim guesses with logins to an own account: the ip counter
	// must still fill up.
	for i := 0; i < 3; i++ {
		_, _, _ = env.svc.Login(ctx, "victim@example.com", "guess", testIP)
		if _, tokens, err := env.svc.Login(ctx, "anna@example.com", "password123", testIP); err != nil || tokens == nil {
			t.Fatalf("own login %d: tokens=%v err=%v", i, tokens, err)
		}
	}
	if _, _, err := env.svc.Login(ctx, "anna@example.com", "password123", testIP); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited from the ip limiter, got %v", err)
	}
}

func TestLogin_IPLimitedDoesNotTouchEmailLimiter(t *testing.T) {
	env, ipL, emailL := throttleEnv(t, 1, 10)
	env.register(t, "anna@example.com")
	ctx := context.Background()

	_, _, _ = env.svc.Login(ctx, "anna@example.com", "wrong-password", testIP)
	callsBefore := emailL.allowCalls
	if _, _, err := env.svc.Login(ctx, "anna@example.com", "wrong-password", testIP); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if emailL.allowCalls != callsBefore {
		t.Fatal("email limiter must not be consulted when the ip is limited")
	}
	_ = ipL
}

func TestLogin_LimiterErrorFailsOpen(t *testing.T) {
	env, ipL, emailL := throttleEnv(t, 100, 10)
	user, _ := env.register(t, "anna@example.com")
	env.markVerified(t, user.ID)
	ipL.allowErr = errors.New("redis down")
	emailL.allowErr = errors.New("redis down")

	if _, tokens, err := env.svc.Login(context.Background(), "anna@example.com", "password123", testIP); err != nil || tokens == nil {
		t.Fatalf("expected login to succeed with limiters down, got tokens=%v err=%v", tokens, err)
	}
}

func TestResetPassword_ResetsLoginEmailCounter(t *testing.T) {
	env, _, emailL := throttleEnv(t, 100, 10)
	user, _ := env.register(t, "Anna@Example.com")
	env.markVerified(t, user.ID)
	plain := env.forgot(t, "Anna@Example.com")

	if err := env.svc.ResetPassword(context.Background(), plain, newTestPassword); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	want := emailRateLimitKey(user.Email)
	if len(emailL.resets) != 1 || emailL.resets[0] != want {
		t.Fatalf("resets = %v, want [%s]", emailL.resets, want)
	}
}
