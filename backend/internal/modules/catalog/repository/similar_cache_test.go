package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newTestSimilarCache(t *testing.T) (*RedisSimilarCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisSimilarCache(client), mr
}

func TestRedisSimilarCache_RoundTrip(t *testing.T) {
	c, mr := newTestSimilarCache(t)
	ctx := context.Background()
	bookID := uuid.New()
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	if err := c.Set(ctx, bookID, ids, 6*time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !mr.Exists("similar:v1:" + bookID.String()) {
		t.Fatal("expected key similar:v1:<id> to exist")
	}

	got, ok, err := c.Get(ctx, bookID)
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if len(got) != len(ids) {
		t.Fatalf("expected %d ids, got %d", len(ids), len(got))
	}
	for i := range ids {
		if got[i] != ids[i] {
			t.Fatalf("position %d: expected %s, got %s", i, ids[i], got[i])
		}
	}
}

func TestRedisSimilarCache_EmptyListRoundTrip(t *testing.T) {
	c, _ := newTestSimilarCache(t)
	ctx := context.Background()
	bookID := uuid.New()

	if err := c.Set(ctx, bookID, []uuid.UUID{}, time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, ok, err := c.Get(ctx, bookID)
	if err != nil || !ok || len(got) != 0 {
		t.Fatalf("expected cached empty list, got ids=%v ok=%v err=%v", got, ok, err)
	}
}

func TestRedisSimilarCache_MissingKey(t *testing.T) {
	c, _ := newTestSimilarCache(t)
	got, ok, err := c.Get(context.Background(), uuid.New())
	if err != nil || ok || got != nil {
		t.Fatalf("expected miss, got ids=%v ok=%v err=%v", got, ok, err)
	}
}

func TestRedisSimilarCache_TTLSet(t *testing.T) {
	c, mr := newTestSimilarCache(t)
	bookID := uuid.New()
	if err := c.Set(context.Background(), bookID, []uuid.UUID{uuid.New()}, 6*time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	if ttl := mr.TTL("similar:v1:" + bookID.String()); ttl != 6*time.Hour {
		t.Fatalf("expected TTL 6h, got %v", ttl)
	}
}

func TestRedisSimilarCache_CorruptValue(t *testing.T) {
	c, mr := newTestSimilarCache(t)
	bookID := uuid.New()
	key := "similar:v1:" + bookID.String()

	for _, raw := range []string{"not-json", `["not-a-uuid"]`} {
		if err := mr.Set(key, raw); err != nil {
			t.Fatalf("seed: %v", err)
		}
		_, ok, err := c.Get(context.Background(), bookID)
		if err == nil || ok {
			t.Fatalf("value %q: expected error and ok=false, got ok=%v err=%v", raw, ok, err)
		}
	}
}

func TestRedisSimilarCache_ClosedServerErrorsQuickly(t *testing.T) {
	c, mr := newTestSimilarCache(t)
	mr.Close()

	start := time.Now()
	_, ok, err := c.Get(context.Background(), uuid.New())
	if err == nil || ok {
		t.Fatalf("expected error on closed server, got ok=%v err=%v", ok, err)
	}
	if err := c.Set(context.Background(), uuid.New(), []uuid.UUID{uuid.New()}, time.Hour); err == nil {
		t.Fatal("expected Set error on closed server")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("expected fail-fast, took %v", elapsed)
	}
}
