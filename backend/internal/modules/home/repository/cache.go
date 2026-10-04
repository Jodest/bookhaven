package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// defaultCacheTimeout bounds every Redis operation so a down Redis never
// holds the home request for the go-redis dial timeout.
const defaultCacheTimeout = 200 * time.Millisecond

// RedisCache is a fail-fast byte cache. It satisfies home service.Cache.
// Keys are chosen by the service (home:v1:<date>).
type RedisCache struct {
	client  *redis.Client
	timeout time.Duration
}

// NewRedisCache creates a RedisCache with a 200ms per-op timeout.
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client, timeout: defaultCacheTimeout}
}

// Get returns the value. A missing key yields ok=false with a nil error.
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	raw, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("home cache get: %w", err)
	}
	return raw, true, nil
}

// Set stores the value with the given TTL.
func (c *RedisCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if err := c.client.Set(ctx, key, val, ttl).Err(); err != nil {
		return fmt.Errorf("home cache set: %w", err)
	}
	return nil
}
