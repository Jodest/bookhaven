// Package ratelimit provides a small Redis-backed fixed-window rate limiter
// for low-volume, per-subject actions (e.g. "resend verification email",
// later "request password reset").
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Options configures a RedisLimiter. Two independent fixed windows are
// enforced per key:
//
//   - a minimum interval between allowed calls: <Prefix>:min:<key>
//     (SET NX EX MinInterval);
//   - at most WindowLimit allowed calls per Window: <Prefix>:day:<key>
//     (INCR, EXPIRE Window when the counter becomes 1).
type Options struct {
	Prefix      string        // e.g. "rl:verify-resend"
	MinInterval time.Duration // e.g. time.Minute
	WindowLimit int64         // e.g. 5
	Window      time.Duration // e.g. 24 * time.Hour
}

// RedisLimiter implements the two fixed windows described in Options on top
// of a go-redis client.
type RedisLimiter struct {
	client *redis.Client
	opts   Options
}

// NewRedisLimiter builds a RedisLimiter.
func NewRedisLimiter(client *redis.Client, opts Options) *RedisLimiter {
	return &RedisLimiter{client: client, opts: opts}
}

// Allow records an attempt for key. It returns allowed=false and the time
// until the blocking window expires when the attempt is over a limit. A
// call denied by the minimum interval does not consume the window quota.
// Redis errors are returned as err (callers should fail closed).
func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	minKey := l.opts.Prefix + ":min:" + key
	windowKey := l.opts.Prefix + ":day:" + key

	// An exhausted window quota is reported first, so a caller over the
	// daily limit always learns the real (long) retry-after.
	if l.opts.WindowLimit > 0 {
		count, err := l.client.Get(ctx, windowKey).Int64()
		if err != nil && !errors.Is(err, redis.Nil) {
			return false, 0, fmt.Errorf("ratelimit: get %s: %w", windowKey, err)
		}
		if count >= l.opts.WindowLimit {
			retry, err := l.ttl(ctx, windowKey, l.opts.Window)
			if err != nil {
				return false, 0, err
			}
			return false, retry, nil
		}
	}

	if l.opts.MinInterval > 0 {
		set, err := l.client.SetNX(ctx, minKey, 1, l.opts.MinInterval).Result()
		if err != nil {
			return false, 0, fmt.Errorf("ratelimit: set %s: %w", minKey, err)
		}
		if !set {
			retry, err := l.ttl(ctx, minKey, l.opts.MinInterval)
			if err != nil {
				return false, 0, err
			}
			return false, retry, nil
		}
	}

	if l.opts.WindowLimit > 0 {
		count, err := l.client.Incr(ctx, windowKey).Result()
		if err != nil {
			return false, 0, fmt.Errorf("ratelimit: incr %s: %w", windowKey, err)
		}
		if count == 1 {
			if err := l.client.Expire(ctx, windowKey, l.opts.Window).Err(); err != nil {
				return false, 0, fmt.Errorf("ratelimit: expire %s: %w", windowKey, err)
			}
		}
		if count > l.opts.WindowLimit {
			retry, err := l.ttl(ctx, windowKey, l.opts.Window)
			if err != nil {
				return false, 0, err
			}
			return false, retry, nil
		}
	}

	return true, 0, nil
}

// ttl returns the remaining TTL of key, repairing a key that somehow lost
// its expiry (e.g. a crash between INCR and EXPIRE) so it cannot block
// forever.
func (l *RedisLimiter) ttl(ctx context.Context, key string, fallback time.Duration) (time.Duration, error) {
	ttl, err := l.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("ratelimit: ttl %s: %w", key, err)
	}
	if ttl <= 0 {
		if ttl == -1 {
			_ = l.client.Expire(ctx, key, fallback).Err()
		}
		return fallback, nil
	}
	return ttl, nil
}
