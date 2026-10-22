package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestLimiter(t *testing.T) (*RedisLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisLimiter(client, Options{
		Prefix:      "rl:test",
		MinInterval: time.Minute,
		WindowLimit: 5,
		Window:      24 * time.Hour,
	}), mr
}

func TestRedisLimiter_FirstCallAllowed(t *testing.T) {
	l, mr := newTestLimiter(t)
	allowed, retry, err := l.Allow(context.Background(), "u1")
	if err != nil || !allowed || retry != 0 {
		t.Fatalf("expected allowed, got allowed=%v retry=%v err=%v", allowed, retry, err)
	}
	if !mr.Exists("rl:test:min:u1") || !mr.Exists("rl:test:day:u1") {
		t.Fatal("expected both window keys to be set")
	}
}

func TestRedisLimiter_SecondCallWithinMinuteDenied(t *testing.T) {
	l, _ := newTestLimiter(t)
	ctx := context.Background()
	if allowed, _, err := l.Allow(ctx, "u1"); err != nil || !allowed {
		t.Fatalf("first call: allowed=%v err=%v", allowed, err)
	}
	allowed, retry, err := l.Allow(ctx, "u1")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if allowed {
		t.Fatal("expected second call within a minute to be denied")
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("expected 0 < retry <= 60s, got %v", retry)
	}

	// Another key is independent.
	if allowed, _, err := l.Allow(ctx, "u2"); err != nil || !allowed {
		t.Fatalf("other key: allowed=%v err=%v", allowed, err)
	}
}

func TestRedisLimiter_MinuteDenialDoesNotConsumeDailyQuota(t *testing.T) {
	l, mr := newTestLimiter(t)
	ctx := context.Background()
	_, _, _ = l.Allow(ctx, "u1")
	_, _, _ = l.Allow(ctx, "u1") // denied by the minute window
	got, err := mr.Get("rl:test:day:u1")
	if err != nil {
		t.Fatalf("get day counter: %v", err)
	}
	if got != "1" {
		t.Fatalf("expected daily counter 1, got %s", got)
	}
}

func TestRedisLimiter_SixthCallInDayDenied(t *testing.T) {
	l, mr := newTestLimiter(t)
	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		allowed, _, err := l.Allow(ctx, "u1")
		if err != nil || !allowed {
			t.Fatalf("call %d: expected allowed, got allowed=%v err=%v", i, allowed, err)
		}
		mr.FastForward(61 * time.Second)
	}
	allowed, retry, err := l.Allow(ctx, "u1")
	if err != nil {
		t.Fatalf("6th call: %v", err)
	}
	if allowed {
		t.Fatal("expected 6th call within a day to be denied")
	}
	if retry <= time.Minute || retry > 24*time.Hour {
		t.Fatalf("expected retry to be the remaining daily window, got %v", retry)
	}

	// After the daily window passes, calls are allowed again.
	mr.FastForward(24 * time.Hour)
	if allowed, _, err := l.Allow(ctx, "u1"); err != nil || !allowed {
		t.Fatalf("after window: allowed=%v err=%v", allowed, err)
	}
}

func TestRedisLimiter_RedisErrorReturned(t *testing.T) {
	l, mr := newTestLimiter(t)
	mr.Close()
	if _, _, err := l.Allow(context.Background(), "u1"); err == nil {
		t.Fatal("expected an error when redis is unavailable")
	}
}

func TestRedisLimiter_ReleaseDecrements(t *testing.T) {
	l, mr := newTestLimiter(t)
	ctx := context.Background()
	if _, _, err := l.Allow(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(ctx, "u1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	got, err := mr.Get("rl:test:day:u1")
	if err != nil || got != "0" {
		t.Fatalf("expected counter 0, got %q err=%v", got, err)
	}
	// Already zero: stays zero, never negative.
	if err := l.Release(ctx, "u1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if got, _ := mr.Get("rl:test:day:u1"); got != "0" {
		t.Fatalf("expected counter to stay 0, got %q", got)
	}
}

func TestRedisLimiter_ReleaseMissingKeyIsNoop(t *testing.T) {
	l, mr := newTestLimiter(t)
	if err := l.Release(context.Background(), "ghost"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if mr.Exists("rl:test:day:ghost") {
		t.Fatal("release must not create a key")
	}
}

func TestRedisLimiter_ResetDeletesKeys(t *testing.T) {
	l, mr := newTestLimiter(t)
	ctx := context.Background()
	if _, _, err := l.Allow(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if err := l.Reset(ctx, "u1"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if mr.Exists("rl:test:min:u1") || mr.Exists("rl:test:day:u1") {
		t.Fatal("expected both keys deleted")
	}
}
