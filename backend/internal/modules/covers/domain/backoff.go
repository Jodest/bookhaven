// Package domain holds the pure rules of the cover mirror (#489): retry
// backoff and the states a failed copy moves to.
package domain

import "time"

const (
	// MaxAttempts is the number of consecutive transient failures after which
	// a copy becomes failed.
	MaxAttempts = 6
	// FirstBackoff is the pause before the first retry.
	FirstBackoff = 10 * time.Minute
	// MaxBackoff caps the doubling pause.
	MaxBackoff = 24 * time.Hour
	// ExhaustedRetry is the pause before a failed (exhausted) copy is tried
	// once more by the sweep.
	ExhaustedRetry = 7 * 24 * time.Hour
)

// Backoff is the pause after the n-th consecutive transient failure
// (n >= 1): 10m * 2^(n-1), capped at 24h.
func Backoff(n int) time.Duration {
	if n < 1 {
		n = 1
	}
	d := FirstBackoff
	for i := 1; i < n; i++ {
		d *= 2
		if d >= MaxBackoff {
			return MaxBackoff
		}
	}
	return d
}

// AfterTransientFailure returns the state after one more transient failure on
// top of prevAttempts earlier ones: pending with a backoff, or, once
// MaxAttempts is reached, failed with the attempts reset and a 7-day retry.
func AfterTransientFailure(prevAttempts int, now time.Time) (status string, attempts int, next time.Time) {
	n := prevAttempts + 1
	if n >= MaxAttempts {
		return "failed", 0, now.Add(ExhaustedRetry)
	}
	return "pending", n, now.Add(Backoff(n))
}
