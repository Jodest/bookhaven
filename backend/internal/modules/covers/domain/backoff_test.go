package domain

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	want := []time.Duration{10 * time.Minute, 20 * time.Minute, 40 * time.Minute, 80 * time.Minute, 160 * time.Minute, 320 * time.Minute, 640 * time.Minute, 1280 * time.Minute}
	for i, w := range want {
		if got := Backoff(i + 1); got != w {
			t.Errorf("Backoff(%d)=%v want %v", i+1, got, w)
		}
	}
	if got := Backoff(9); got != MaxBackoff {
		t.Errorf("Backoff(9)=%v want cap", got)
	}
	if got := Backoff(100); got != MaxBackoff {
		t.Errorf("Backoff(100)=%v want cap", got)
	}
	if got := Backoff(0); got != FirstBackoff {
		t.Errorf("Backoff(0)=%v", got)
	}
}

func TestAfterTransientFailure(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	st, n, next := AfterTransientFailure(0, now)
	if st != "pending" || n != 1 || !next.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("first: %s %d %v", st, n, next)
	}
	st, n, next = AfterTransientFailure(4, now)
	if st != "pending" || n != 5 || !next.Equal(now.Add(160*time.Minute)) {
		t.Fatalf("fifth: %s %d %v", st, n, next)
	}
	st, n, next = AfterTransientFailure(5, now)
	if st != "failed" || n != 0 || !next.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("sixth: %s %d %v", st, n, next)
	}
}
