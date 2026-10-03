package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// similarCacheKeyPrefix is versioned so a future scoring change can bump
	// the version instead of flushing old keys.
	similarCacheKeyPrefix = "similar:v1:"
	// defaultSimilarCacheTimeout bounds every Redis operation so a down
	// Redis never holds a request for the go-redis dial timeout.
	defaultSimilarCacheTimeout = 200 * time.Millisecond
)

// RedisSimilarCache stores the ranked similar-book ID list per source book.
// It satisfies catalog service.SimilarCache.
type RedisSimilarCache struct {
	client  *redis.Client
	timeout time.Duration
}

// NewRedisSimilarCache creates a RedisSimilarCache with a 200ms per-op timeout.
func NewRedisSimilarCache(client *redis.Client) *RedisSimilarCache {
	return &RedisSimilarCache{client: client, timeout: defaultSimilarCacheTimeout}
}

func similarCacheKey(bookID uuid.UUID) string {
	return similarCacheKeyPrefix + bookID.String()
}

// Get returns the cached ID list. A missing key yields ok=false with a nil
// error; a corrupt value yields ok=false with an error.
func (c *RedisSimilarCache) Get(ctx context.Context, bookID uuid.UUID) ([]uuid.UUID, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	raw, err := c.client.Get(ctx, similarCacheKey(bookID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("similar cache get: %w", err)
	}

	var strs []string
	if err := json.Unmarshal(raw, &strs); err != nil {
		return nil, false, fmt.Errorf("similar cache decode: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(strs))
	for _, s := range strs {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, false, fmt.Errorf("similar cache decode id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, true, nil
}

// Set stores the ID list as a JSON array of UUID strings with the given TTL.
func (c *RedisSimilarCache) Set(ctx context.Context, bookID uuid.UUID, ids []uuid.UUID, ttl time.Duration) error {
	strs := make([]string, 0, len(ids))
	for _, id := range ids {
		strs = append(strs, id.String())
	}
	raw, err := json.Marshal(strs)
	if err != nil {
		return fmt.Errorf("similar cache encode: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if err := c.client.Set(ctx, similarCacheKey(bookID), raw, ttl).Err(); err != nil {
		return fmt.Errorf("similar cache set: %w", err)
	}
	return nil
}
